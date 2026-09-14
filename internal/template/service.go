package template

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"image"
	"image/png"
	"math/rand"
	"os/exec"
	"strings"
	"sync"
	"time"

	"mywebscrcpy/internal/action"
	"mywebscrcpy/internal/vision"
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
}

// Service orchestrates template management, screen capture, matching workers, and vision publishing.
type Service struct {
	adbPath   string
	storage   *Storage
	matcher   *Matcher
	publisher VisionPublisher
	submitter ActionSubmitter
	interval  time.Duration

	mu      sync.RWMutex
	workers map[string]*deviceWorker
	closed  bool

	captureScreenFn func(ctx context.Context, serial string) (image.Image, error)
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
	return &Service{
		adbPath:   adbPath,
		storage:   storage,
		matcher:   NewMatcher(),
		publisher: publisher,
		submitter: submitter,
		interval:  defaultMatchInterval,
		workers:   make(map[string]*deviceWorker),
	}
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

// captureScreenDefault executes adb screencap to get a PNG image.
func (s *Service) captureScreenDefault(ctx context.Context, serial string) (image.Image, error) {
	var lastErr error
	for attempt := 0; attempt < 2; attempt++ {
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		default:
		}

		capCtx, capCancel := context.WithTimeout(ctx, defaultScreenTimeout)
		cmd := exec.CommandContext(capCtx, s.adbPath, "-s", serial, "exec-out", "screencap", "-p")
		var out bytes.Buffer
		cmd.Stdout = &out
		err := cmd.Run()
		capCancel()

		if err != nil {
			lastErr = err
			time.Sleep(50 * time.Millisecond)
			continue
		}

		data := out.Bytes()
		if len(data) == 0 {
			lastErr = errors.New("screencap returned empty output")
			continue
		}

		img, err := png.Decode(bytes.NewReader(data))
		if err != nil {
			lastErr = fmt.Errorf("failed to decode screencap png: %w", err)
			continue
		}

		return img, nil
	}

	return nil, fmt.Errorf("captureScreen failed after retries: %w", lastErr)
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
	s.mu.Unlock()

	go s.runWorker(ctx, worker)
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
		Timestamp: time.Now().UnixMilli(),
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
		return nil, ErrInvalidImage
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

		tmplImg, err := s.storage.GetTemplateImage(t.ID)
		if err != nil {
			continue
		}

		mOpts := opts
		if mOpts.MinScore <= 0 {
			mOpts.MinScore = t.Threshold
		}

		matches, err := s.matcher.MatchWithScene(sc, img, t, tmplImg, mOpts)
		if err != nil {
			continue
		}
		allMatches = append(allMatches, matches...)
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
	s.mu.RUnlock()

	status := DeviceStatus{
		Serial:      serial,
		LastMatches: []MatchResult{},
	}

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

