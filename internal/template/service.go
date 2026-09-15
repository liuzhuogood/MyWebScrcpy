package template

import (
	"context"
	"errors"
	"fmt"
	"image"
	_ "image/jpeg"
	_ "image/png"
	"log"
	"math/rand"
	"strings"
	"sync"
	"time"

	"mywebscrcpy/internal/action"
	"mywebscrcpy/internal/vision"
	"mywebscrcpy/internal/ws"
)

const (
	defaultMatchInterval = 500 * time.Millisecond
	defaultScreenTimeout = 3 * time.Second
)

// VisionPublisher publishes vision messages to connected clients.
type VisionPublisher interface {
	PublishVisionResult(serial string, msg vision.Message)
}

// ActionSubmitter submits actions to a device session.
type ActionSubmitter interface {
	SubmitAction(ctx context.Context, serial string, req action.Request) (action.Result, error)
}

// deviceWorker maintains the background matching state for a specific device.
type deviceWorker struct {
	serial        string
	enabled       bool
	cancel        context.CancelFunc
	done          chan struct{}
	mu            sync.RWMutex
	lastMatches   []MatchResult
	lastMatchTime time.Time
	latestFrame   image.Image
}

// Service orchestrates template management, screen capture, matching workers, and vision publishing.
type Service struct {
	adbPath      string
	storage      *Storage
	matcher      *Matcher
	publisher    VisionPublisher
	submitter    ActionSubmitter
	interval     time.Duration
	autoMatching bool

	mu      sync.RWMutex
	workers map[string]*deviceWorker
	closed  bool

	captureScreenFn func(ctx context.Context, serial string) (image.Image, error)

	pyMatcher     *PyMatcher
	pyStarted     bool
	pyError       string
	pyMu          sync.Mutex
	pyUnavailOnce sync.Once
}

// NewService creates a new template Service instance.
func NewService(adbPath string, storage *Storage, publisher VisionPublisher) *Service {
	if adbPath == "" {
		adbPath = "adb"
	}
	var submitter ActionSubmitter
	if sub, ok := publisher.(ActionSubmitter); ok {
		submitter = sub
	}
	s := &Service{
		adbPath:      adbPath,
		storage:      storage,
		matcher:      NewMatcher(),
		publisher:    publisher,
		submitter:    submitter,
		interval:     defaultMatchInterval,
		autoMatching: true,
		workers:      make(map[string]*deviceWorker),
	}
	if storage != nil {
		storage.OnChange(func(serials []string) {
			for _, ser := range serials {
				s.TriggerRedetect(ser)
			}
		})
		s.pyMatcher = NewPyMatcher(storage.BaseDir(), s.applyPythonResult)
	}
	return s
}

// ConsumeVideoFrame receives H.264 directly from the in-process scrcpy hub and
// forwards the raw encoded frame to the Python matching process (which decodes
// H264 with PyAV, including CABAC, and runs OpenCV template matching). If the
// Python matcher is not available, live matching is disabled rather than
// silently falling back to the built-in decoder.
func (s *Service) ConsumeVideoFrame(frame ws.VideoFrame) {
	if frame.Kind != 0 && frame.Kind != 1 && frame.Kind != 2 {
		return
	}
	if s.pyMatcher == nil {
		return
	}
	if !s.PythonMatchingEnabled() {
		s.notePyUnavailable()
		return
	}
	// 仅当该设备的模板匹配开关为开启时才转发帧，避免无谓解码与资源占用。
	if !s.deviceMatchingActive(frame.DeviceID) {
		return
	}
	s.pyMatcher.SubmitFrame(frame.DeviceID, uint8(frame.Kind), frame.FrameID, frame.Payload)
}

// deviceMatchingActive reports whether frames for serial should be forwarded to
// the Python matcher. A manually disabled worker (switch off) is honored and
// never re-enabled by incoming frames.
func (s *Service) deviceMatchingActive(serial string) bool {
	s.mu.RLock()
	w, exists := s.workers[serial]
	auto := s.autoMatching
	s.mu.RUnlock()
	if exists {
		return w.enabled
	}
	return auto
}

// notePyUnavailable logs a clear, one-time explanation when the Python matcher
// is required but unavailable, so users know why live matching is off.
func (s *Service) notePyUnavailable() {
	s.pyUnavailOnce.Do(func() {
		log.Printf("模板匹配未启动：未检测到可用的 Python 匹配环境（需要 python3 + PyAV(av) + OpenCV）。")
		log.Printf("请安装依赖，或通过环境变量 MYWEBSCRCPY_PYTHON 指定 Python 解释器。详见 docs/python-matching.md。")
	})
}

// SetAutoMatching configures whether device matching is enabled automatically on first status inquiry.
func (s *Service) SetAutoMatching(enabled bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.autoMatching = enabled
}

// AutoMatching returns whether autoMatching is enabled.
func (s *Service) AutoMatching() bool {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.autoMatching
}

// StartPythonMatching spawns the Python matching subprocess. On failure the
// service keeps using the built-in Go matching path.
func (s *Service) StartPythonMatching() error {
	if s.pyMatcher == nil {
		err := errors.New("python matcher not configured")
		s.setPyError(err)
		return err
	}
	s.pyMu.Lock()
	defer s.pyMu.Unlock()
	if s.pyStarted {
		s.pyError = ""
		return nil
	}
	if err := s.pyMatcher.Start(); err != nil {
		s.pyError = err.Error()
		return err
	}
	s.pyStarted = true
	s.pyError = ""
	return nil
}

// setPyError records the Python matcher error under the pyMu lock.
func (s *Service) setPyError(err error) {
	s.pyMu.Lock()
	defer s.pyMu.Unlock()
	if err != nil {
		s.pyError = err.Error()
	} else {
		s.pyError = ""
	}
}

// PythonMatchingEnabled reports whether the Python matcher is running.
func (s *Service) PythonMatchingEnabled() bool {
	s.pyMu.Lock()
	defer s.pyMu.Unlock()
	return s.pyStarted
}

// PythonMatchingStatus returns (ready, errorReason). ready is true when the
// Python matcher is running; otherwise errorReason explains what is missing.
func (s *Service) PythonMatchingStatus() (bool, string) {
	s.pyMu.Lock()
	defer s.pyMu.Unlock()
	return s.pyStarted, s.pyError
}

// applyPythonResult applies a matching result returned by the Python process to
// the device worker and broadcasts it to connected clients.
func (s *Service) applyPythonResult(serial string, frameID uint64, matches []MatchResult) {
	s.mu.RLock()
	w, exists := s.workers[serial]
	auto := s.autoMatching
	s.mu.RUnlock()
	// 仅在 worker 不存在且自动匹配开启时创建；用户手动关闭（enabled=false）的不覆盖。
	if !exists && auto {
		_ = s.SetDeviceMatching(serial, true)
		s.mu.RLock()
		w = s.workers[serial]
		exists = w != nil
		s.mu.RUnlock()
	}
	if !exists || !w.enabled {
		return
	}
	w.mu.Lock()
	w.lastMatches = matches
	w.lastMatchTime = time.Now()
	w.mu.Unlock()
	s.BroadcastMatches(serial, matches)
}

// SetActionSubmitter injects an ActionSubmitter instance into Service.
func (s *Service) SetActionSubmitter(sub ActionSubmitter) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.submitter = sub
}

// Storage returns the underlying template storage.
func (s *Service) Storage() *Storage {
	return s.storage
}

// SetInterval configures the worker polling interval.
func (s *Service) SetInterval(d time.Duration) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if d > 0 {
		s.interval = d
	}
}

// SetCaptureScreenFn sets an override screen capture function (useful for tests).
func (s *Service) SetCaptureScreenFn(fn func(ctx context.Context, serial string) (image.Image, error)) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.captureScreenFn = fn
}

// CaptureScreen acquires a screenshot from the specified device.
func (s *Service) CaptureScreen(ctx context.Context, serial string) (image.Image, error) {
	s.mu.RLock()
	fn := s.captureScreenFn
	s.mu.RUnlock()

	if fn != nil {
		return fn(ctx, serial)
	}

	return s.captureScreenDefault(ctx, serial)
}

// ProcessLiveFrame accepts a live frame from the device video stream, runs pure Go matching, and broadcasts results.
func (s *Service) ProcessLiveFrame(serial string, img image.Image) ([]MatchResult, error) {
	if img == nil {
		return nil, ErrInvalidImage
	}
	serial = strings.TrimSpace(serial)
	if serial == "" {
		return nil, errors.New("serial is required")
	}

	s.mu.RLock()
	w, exists := s.workers[serial]
	autoMatch := s.autoMatching
	s.mu.RUnlock()

	if (!exists || !w.enabled) && autoMatch {
		_ = s.SetDeviceMatching(serial, true)
		s.mu.RLock()
		w = s.workers[serial]
		exists = w != nil
		s.mu.RUnlock()
	}

	if !exists || !w.enabled {
		return []MatchResult{}, nil
	}

	w.mu.Lock()
	w.latestFrame = img
	w.mu.Unlock()

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	matches, err := s.Detect(ctx, serial, img)
	if err != nil {
		return nil, err
	}

	w.mu.Lock()
	w.lastMatches = matches
	w.lastMatchTime = time.Now()
	w.mu.Unlock()

	s.BroadcastMatches(serial, matches)
	return matches, nil
}

// GetLatestFrame returns the latest live frame received for a device.
func (s *Service) GetLatestFrame(serial string) image.Image {
	s.mu.RLock()
	w, exists := s.workers[serial]
	s.mu.RUnlock()

	if !exists || w == nil {
		return nil
	}

	w.mu.RLock()
	defer w.mu.RUnlock()
	return w.latestFrame
}

// TriggerRedetectAll triggers immediate re-detection across all active device workers.
func (s *Service) TriggerRedetectAll() {
	s.TriggerRedetect("")
}

// TriggerRedetect triggers immediate re-detection for the specified serial using its latest cached frame.
// If serial is "global" or empty, all active device workers are triggered.
func (s *Service) TriggerRedetect(serial string) {
	norm := strings.TrimSpace(serial)
	var targets []*deviceWorker

	s.mu.RLock()
	if norm == "" || norm == GlobalSerial || strings.EqualFold(norm, "global") {
		for _, w := range s.workers {
			if w != nil && w.enabled {
				targets = append(targets, w)
			}
		}
	} else {
		if w, ok := s.workers[norm]; ok && w != nil && w.enabled {
			targets = append(targets, w)
		} else if w, ok := s.workers[NormalizeSerial(norm)]; ok && w != nil && w.enabled {
			targets = append(targets, w)
		}
	}
	s.mu.RUnlock()

	for _, w := range targets {
		s.redetectWorker(w)
	}
}

func (s *Service) redetectWorker(w *deviceWorker) {
	if w == nil || !w.enabled {
		return
	}
	w.mu.RLock()
	frame := w.latestFrame
	targetSerial := w.serial
	w.mu.RUnlock()

	if frame == nil {
		return
	}

	go func() {
		ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
		defer cancel()

		matches, err := s.Detect(ctx, targetSerial, frame)
		if err != nil {
			return
		}

		w.mu.Lock()
		w.lastMatches = matches
		w.lastMatchTime = time.Now()
		w.mu.Unlock()

		s.BroadcastMatches(targetSerial, matches)
	}()
}

// captureScreenDefault returns an error indicating adb screencap has been replaced by the real-time video stream engine.
func (s *Service) captureScreenDefault(ctx context.Context, serial string) (image.Image, error) {
	return nil, errors.New("adb screencap disabled: please use real-time stream engine")
}

// SetDeviceMatching enables or disables background template matching for a device.
func (s *Service) SetDeviceMatching(serial string, enabled bool) error {
	serial = strings.TrimSpace(serial)
	if serial == "" {
		return errors.New("serial is required")
	}

	s.mu.Lock()
	if s.closed {
		s.mu.Unlock()
		return errors.New("service is stopped")
	}

	w, exists := s.workers[serial]
	if !enabled {
		if !exists || !w.enabled {
			s.mu.Unlock()
			return nil
		}
		w.mu.Lock()
		w.enabled = false
		if w.cancel != nil {
			w.cancel()
		}
		w.mu.Unlock()
		s.mu.Unlock()

		if w.done != nil {
			<-w.done
		}

		s.broadcastClear(serial)
		return nil
	}

	// enabled == true
	if exists && w.enabled {
		s.mu.Unlock()
		return nil
	}

	ctx, cancel := context.WithCancel(context.Background())
	worker := &deviceWorker{
		serial:      serial,
		enabled:     true,
		cancel:      cancel,
		done:        make(chan struct{}),
		lastMatches: []MatchResult{},
	}
	s.workers[serial] = worker
	hasMockCapture := s.captureScreenFn != nil
	s.mu.Unlock()

	if hasMockCapture {
		go s.runWorker(ctx, worker)
	} else {
		close(worker.done)
	}

	return nil
}

// runWorker runs the periodic template matching loop for a device.
func (s *Service) runWorker(ctx context.Context, w *deviceWorker) {
	defer close(w.done)

	s.mu.RLock()
	interval := s.interval
	s.mu.RUnlock()

	if interval <= 0 {
		interval = defaultMatchInterval
	}

	// Run initial iteration immediately
	s.matchIteration(ctx, w)

	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			s.matchIteration(ctx, w)
		}
	}
}

// matchIteration captures a single frame, runs matching, and broadcasts results.
func (s *Service) matchIteration(ctx context.Context, w *deviceWorker) {
	screenImg, err := s.CaptureScreen(ctx, w.serial)
	if err != nil {
		return
	}

	matches, err := s.Detect(ctx, w.serial, screenImg)
	if err != nil {
		return
	}

	w.mu.Lock()
	w.lastMatches = matches
	w.lastMatchTime = time.Now()
	w.mu.Unlock()

	s.BroadcastMatches(w.serial, matches)
}

// BroadcastMatches converts MatchResults to vision objects and broadcasts to the screen overlay.
func (s *Service) BroadcastMatches(serial string, matches []MatchResult) {
	if len(matches) > 0 {
		objects := make([]vision.Object, len(matches))
		for i, m := range matches {
			objects[i] = vision.Object{
				Label:      m.Name,
				Confidence: m.Score,
				X:          m.X,
				Y:          m.Y,
				W:          m.W,
				H:          m.H,
			}
		}
		s.broadcastResult(serial, objects)
	} else {
		s.broadcastClear(serial)
	}
}

// broadcastResult sends detected objects to the publisher.
func (s *Service) broadcastResult(serial string, objects []vision.Object) {
	s.broadcastResultWithFrame(serial, objects, 0, time.Now())
}

func (s *Service) broadcastResultWithFrame(serial string, objects []vision.Object, frameID uint64, capturedAt time.Time) {
	if s.publisher == nil {
		return
	}
	if objects == nil {
		objects = []vision.Object{}
	}
	msg := vision.Message{
		Type:      "detection.result",
		DeviceID:  serial,
		SessionID: "",
		FrameID:   frameID,
		Timestamp: capturedAt.UnixMilli(),
		Objects:   objects,
	}
	s.publisher.PublishVisionResult(serial, msg)
}

// broadcastClear sends an empty object list to clear overlays on frontend.
func (s *Service) broadcastClear(serial string) {
	s.broadcastResult(serial, []vision.Object{})
}

// Detect runs detection on an image using all active templates applicable to the serial.
func (s *Service) Detect(ctx context.Context, serial string, img image.Image) ([]MatchResult, error) {
	return s.DetectWithOptions(ctx, serial, img, nil, MatchOptions{})
}

// DetectWithOptions runs detection on an image with specific template IDs and matching options.
func (s *Service) DetectWithOptions(ctx context.Context, serial string, img image.Image, templateIDs []string, opts MatchOptions) ([]MatchResult, error) {
	if img == nil {
		img = s.GetLatestFrame(serial)
		if img == nil {
			return nil, ErrInvalidImage
		}
	}
	if s.storage == nil {
		return []MatchResult{}, nil
	}

	var targets []*Template
	if len(templateIDs) > 0 {
		for _, id := range templateIDs {
			t, err := s.storage.GetTemplate(id)
			if err != nil {
				continue
			}
			if t.Enabled {
				targets = append(targets, t)
			}
		}
	} else {
		all := s.storage.ListTemplates(serial)
		for _, t := range all {
			if t.Enabled {
				targets = append(targets, t)
			}
		}
	}

	if len(targets) == 0 {
		return []MatchResult{}, nil
	}

	sc, err := s.matcher.PrepareScene(img)
	if err != nil {
		return nil, err
	}

	var allMatches []MatchResult
	for _, t := range targets {
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		default:
		}

		images, err := s.storage.GetTemplateImages(t.ID)
		if err != nil {
			continue
		}
		mOpts := opts
		if mOpts.MinScore <= 0 {
			mOpts.MinScore = t.Threshold
		}
		var best []MatchResult
		for _, tmplImg := range images {
			matches, matchErr := s.matcher.MatchWithScene(sc, img, t, tmplImg, mOpts)
			if matchErr == nil && len(matches) > 0 && (len(best) == 0 || matches[0].Score > best[0].Score) {
				best = matches
			}
		}
		allMatches = append(allMatches, best...)
	}

	if allMatches == nil {
		allMatches = []MatchResult{}
	}
	return allMatches, nil
}

// GetDeviceStatus returns the current matching status for a device.
func (s *Service) GetDeviceStatus(serial string) DeviceStatus {
	s.mu.RLock()
	w, exists := s.workers[serial]
	auto := s.autoMatching
	s.mu.RUnlock()

	if !exists && auto {
		_ = s.SetDeviceMatching(serial, true)
		s.mu.RLock()
		w = s.workers[serial]
		exists = true
		s.mu.RUnlock()
	}

	status := DeviceStatus{
		Serial:      serial,
		LastMatches: []MatchResult{},
	}

	ready, pyErr := s.PythonMatchingStatus()
	status.PythonReady = ready
	status.PythonError = pyErr

	if s.storage != nil {
		tmpls := s.storage.ListTemplates(serial)
		count := 0
		for _, t := range tmpls {
			if t.Enabled {
				count++
			}
		}
		status.ActiveTemplates = count
	}

	if exists && w != nil {
		w.mu.RLock()
		status.Enabled = w.enabled
		status.LastMatchTime = w.lastMatchTime
		if len(w.lastMatches) > 0 {
			status.LastMatches = make([]MatchResult, len(w.lastMatches))
			copy(status.LastMatches, w.lastMatches)
		}
		w.mu.RUnlock()
	}

	return status
}

// GetLatestMatches returns the latest match results for a device.
func (s *Service) GetLatestMatches(serial string) []MatchResult {
	s.mu.RLock()
	w, exists := s.workers[serial]
	s.mu.RUnlock()

	if !exists || w == nil {
		return []MatchResult{}
	}

	w.mu.RLock()
	defer w.mu.RUnlock()

	if len(w.lastMatches) == 0 {
		return []MatchResult{}
	}

	res := make([]MatchResult, len(w.lastMatches))
	copy(res, w.lastMatches)
	return res
}

// Stop stops all background workers and cleans up resources.
func (s *Service) Stop() {
	s.mu.Lock()
	if s.closed {
		s.mu.Unlock()
		return
	}
	s.closed = true
	workers := make([]*deviceWorker, 0, len(s.workers))
	for _, w := range s.workers {
		workers = append(workers, w)
	}
	s.mu.Unlock()

	if s.pyMatcher != nil {
		s.pyMatcher.Close()
	}

	for _, w := range workers {
		w.mu.Lock()
		if w.enabled {
			w.enabled = false
			if w.cancel != nil {
				w.cancel()
			}
		}
		w.mu.Unlock()

		if w.done != nil {
			<-w.done
		}

		s.broadcastClear(w.serial)
	}

	if s.storage != nil {
		_ = s.storage.Close()
	}
}

// ClickTemplate finds a template by name or ID, matches it on the device screen,
// and triggers a click on its center (or with bounded random offset if requested).
// If the template does not match on screen, it returns ErrTemplateNotMatched.
func (s *Service) ClickTemplate(ctx context.Context, serial, nameOrID string, opts ClickOptions) (*ClickResult, error) {
	if serial == "" {
		return nil, errors.New("missing serial")
	}
	if nameOrID == "" {
		return nil, errors.New("missing template name or id")
	}

	tmpl, err := s.storage.FindTemplate(serial, nameOrID)
	if err != nil {
		return nil, err
	}

	// 1. Search for a matching target on screen
	var bestMatch *MatchResult

	// Check latest cached matches first
	latest := s.GetLatestMatches(serial)
	for _, m := range latest {
		if m.TemplateID == tmpl.ID && m.Score >= tmpl.Threshold {
			if bestMatch == nil || m.Score > bestMatch.Score {
				cp := m
				bestMatch = &cp
			}
		}
	}

	// If not found in cache or no background worker running, perform a direct screenshot + match
	if bestMatch == nil {
		img, err := s.CaptureScreen(ctx, serial)
		if err != nil {
			return nil, fmt.Errorf("capture screen failed: %w", err)
		}
		matches, err := s.DetectWithOptions(ctx, serial, img, []string{tmpl.ID}, MatchOptions{MinScore: tmpl.Threshold})
		if err != nil {
			return nil, fmt.Errorf("detection failed: %w", err)
		}
		for _, m := range matches {
			if bestMatch == nil || m.Score > bestMatch.Score {
				cp := m
				bestMatch = &cp
			}
		}
	}

	if bestMatch == nil {
		return nil, ErrTemplateNotMatched
	}

	// 2. Compute center point
	targetX := bestMatch.X + bestMatch.W/2.0
	targetY := bestMatch.Y + bestMatch.H/2.0

	// 3. Apply random bounded offset if requested (staying strictly within the matched box)
	if opts.RandomOffset {
		jitterX := (rand.Float64() - 0.5) * bestMatch.W * 0.4
		jitterY := (rand.Float64() - 0.5) * bestMatch.H * 0.4
		targetX += jitterX
		targetY += jitterY
		if targetX < bestMatch.X {
			targetX = bestMatch.X
		} else if targetX > bestMatch.X+bestMatch.W {
			targetX = bestMatch.X + bestMatch.W
		}
		if targetY < bestMatch.Y {
			targetY = bestMatch.Y
		} else if targetY > bestMatch.Y+bestMatch.H {
			targetY = bestMatch.Y + bestMatch.H
		}
	}

	// Clamp to normalized [0.0, 1.0]
	if targetX < 0 {
		targetX = 0
	} else if targetX > 1 {
		targetX = 1
	}
	if targetY < 0 {
		targetY = 0
	} else if targetY > 1 {
		targetY = 1
	}

	mode := opts.Mode
	if mode == "" {
		mode = "sdk"
	}

	// 4. Submit click action
	s.mu.RLock()
	sub := s.submitter
	s.mu.RUnlock()

	executed := false
	if sub != nil {
		req := action.Request{
			DeviceID:   serial,
			Source:     "template_click",
			Action:     "tap",
			X:          targetX,
			Y:          targetY,
			Mode:       mode,
			Humanize:   opts.Humanize,
			DurationMS: opts.DurationMS,
		}
		res, err := sub.SubmitAction(ctx, serial, req)
		if err != nil {
			return nil, fmt.Errorf("action execution failed: %w", err)
		}
		executed = res.Executed
	}

	return &ClickResult{
		OK:           true,
		TemplateID:   tmpl.ID,
		TemplateName: tmpl.Name,
		Score:        bestMatch.Score,
		X:            targetX,
		Y:            targetY,
		RandomOffset: opts.RandomOffset,
		Mode:         mode,
		Executed:     executed,
	}, nil
}
