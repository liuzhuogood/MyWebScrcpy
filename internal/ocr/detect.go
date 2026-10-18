package ocr

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"image"
	"image/draw"
	"image/png"
	"log"
	"math"
	"strings"
	"time"

	"mywebscrcpy/internal/dependencies"
	"mywebscrcpy/internal/vision"
)

const MaxStabilizationMS int64 = 5000

var errOCRBusy = errors.New("OCR 并发请求已满，请稍后重试")

func validateDetectRequest(req DetectRequest) error {
	if req.StabilizationMS < 0 || req.StabilizationMS > MaxStabilizationMS {
		return errors.New("stabilization_ms 必须是 0 到 5000 的整数毫秒")
	}
	if len(req.BBox) != 0 {
		if len(req.BBox) != 4 {
			return errors.New("bbox 必须包含四个归一化坐标")
		}
		for _, v := range req.BBox {
			if math.IsNaN(v) || math.IsInf(v, 0) || v < 0 || v > 1 {
				return errors.New("bbox 坐标必须在 0 到 1 之间")
			}
		}
		if req.BBox[2] <= 0 || req.BBox[3] <= 0 || req.BBox[0]+req.BBox[2] > 1 || req.BBox[1]+req.BBox[3] > 1 {
			return errors.New("bbox 超出画面或尺寸无效")
		}
	}
	return nil
}
func (s *Service) effectiveEngine(req DetectRequest) string {
	if req.Engine != "" {
		return req.Engine
	}
	return s.getConfig("OCR_ENGINE", "auto")
}
func (s *Service) acquireDetection() (func(), error) {
	select {
	case s.detectSlots <- struct{}{}:
		return func() { <-s.detectSlots }, nil
	default:
		return nil, errOCRBusy
	}
}

// Detect keeps uploaded-image recognition separate from live temporal sampling.
func (s *Service) Detect(ctx context.Context, img []byte, req DetectRequest) (*DetectResponse, error) {
	if err := validateDetectRequest(req); err != nil {
		return nil, err
	}
	if req.StabilizationMS > 0 {
		return nil, errors.New("stabilization_ms 大于 0 时请仅提供 serial，由服务端连续截图")
	}
	req.Engine = s.effectiveEngine(req)
	if req.Engine != "parallel" {
		publish, finishMark := s.markRequest(req)
		defer finishMark()
		singleReq := req
		singleReq.Mark = false
		res, err := s.detectSingle(ctx, img, singleReq, false)
		if err == nil && ctx.Err() != nil {
			return nil, ctx.Err()
		}
		if err == nil {
			res.Samples = 1
			res.EnginesUsed = []string{res.Engine}
			if len(res.Boxes) > 0 {
				publish(req, res)
			}
		}
		return res, err
	}
	release, err := s.acquireDetection()
	if err != nil {
		return nil, err
	}
	defer release()
	ctx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()
	started := time.Now()
	mark := req.Mark
	publish, finishMark := s.markRequest(req)
	defer finishMark()
	req.Mark = false
	res, err := s.detectParallel(ctx, img, req)
	if err != nil {
		return nil, err
	}
	if err = ctx.Err(); err != nil {
		return nil, err
	}
	req.Mark = mark
	res.ElapsedMS = time.Since(started).Milliseconds()
	filterConsensus(req, res)
	publish(req, res)
	return res, nil
}
func (s *Service) recognize(ctx context.Context, img []byte, req DetectRequest) (*DetectResponse, error) {
	if req.Engine == "parallel" {
		return s.detectParallel(ctx, img, req)
	}
	res, err := s.detectSingle(ctx, img, req, false)
	if err == nil {
		res.Samples = 1
		res.EnginesUsed = []string{res.Engine}
	}
	return res, err
}
func (s *Service) detectParallel(ctx context.Context, img []byte, req DetectRequest) (*DetectResponse, error) {
	type answer struct {
		res    *DetectResponse
		err    error
		engine string
		index  int
	}
	engines := []string{"indraocr", "macocr", "paddleocr"}
	ch := make(chan answer, len(engines))
	for i, engine := range engines {
		go func(i int, engine string) {
			r := req
			r.Engine = engine
			r.Keywords = nil
			r.Mark = false
			engineCtx, cancel := context.WithTimeout(ctx, s.parallelEngineTimeout)
			defer cancel()
			res, err := s.detectSingle(engineCtx, img, r, true)
			ch <- answer{res, err, engine, i}
		}(i, engine)
	}
	answers := make([]answer, 3)
	for range engines {
		a := <-ch
		answers[a.index] = a
	}
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	results := []*DetectResponse{}
	used := []string{}
	failures := []string{}
	for _, a := range answers {
		if a.err != nil {
			failures = append(failures, fmt.Sprintf("%s: %v", a.engine, a.err))
			continue
		}
		results = append(results, a.res)
		used = append(used, a.engine)
	}
	if len(results) == 0 {
		return nil, fmt.Errorf("所有 OCR 服务均不可用 (%s)", strings.Join(failures, "; "))
	}
	res := fuseOCRResults(results, false)
	res.Engine = "parallel"
	res.Samples = 1
	res.EnginesUsed = used
	res.FallbackReason = failures
	if len(failures) > 0 {
		log.Printf("[OCR] 并行识别部分失败 engines=%v failures=%d", used, len(failures))
	}
	return res, nil
}

func captureDeviceScreen(ctx context.Context, serial string) ([]byte, error) {
	cmd := dependencies.CommandContext(ctx, dependencies.ManagedADB, "-s", serial, "exec-out", "screencap", "-p")
	out, err := cmd.Output()
	if err != nil {
		if ctx.Err() != nil {
			return nil, ctx.Err()
		}
		return nil, fmt.Errorf("获取设备截图失败: %w", err)
	}
	return out, nil
}

func cropDeviceFrame(data []byte, bbox []float64) ([]byte, image.Point, error) {
	img, _, err := image.Decode(bytes.NewReader(data))
	if err != nil {
		return nil, image.Point{}, fmt.Errorf("设备截图无效: %w", err)
	}
	size := img.Bounds().Size()
	if len(bbox) == 0 {
		return data, size, nil
	}
	x, y := int(bbox[0]*float64(size.X)), int(bbox[1]*float64(size.Y))
	right, bottom := int(math.Ceil((bbox[0]+bbox[2])*float64(size.X))), int(math.Ceil((bbox[1]+bbox[3])*float64(size.Y)))
	if right <= x || bottom <= y {
		return nil, size, errors.New("bbox 无有效像素")
	}
	crop := image.NewRGBA(image.Rect(0, 0, right-x, bottom-y))
	draw.Draw(crop, crop.Bounds(), img, image.Pt(x, y), draw.Src)
	var out bytes.Buffer
	if err := png.Encode(&out, crop); err != nil {
		return nil, size, err
	}
	return out.Bytes(), size, nil
}

// DetectDevice takes fresh screenshots within the sampling window. No request
// starts after that window; in-flight work has a separate 30-second total budget.
func (s *Service) DetectDevice(ctx context.Context, req DetectRequest) (*DetectResponse, error) {
	if err := validateDetectRequest(req); err != nil {
		return nil, err
	}
	if strings.TrimSpace(req.Serial) == "" {
		return nil, errors.New("必须提供 serial")
	}
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	release, err := s.acquireDetection()
	if err != nil {
		return nil, err
	}
	defer release()
	ctx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()
	started := time.Now()
	until := started.Add(time.Duration(req.StabilizationMS) * time.Millisecond)
	mark := req.Mark
	publish, finishMark := s.markRequest(req)
	defer finishMark()
	keywords := req.Keywords
	if req.StabilizationMS > 0 {
		req.Keywords = nil
	}
	req.Mark = false
	req.Engine = s.effectiveEngine(req)
	samples := []*DetectResponse{}
	used := []string{}
	failures := []string{}
	var size image.Point
	for len(samples) < 101 {
		if err := ctx.Err(); err != nil {
			return nil, err
		}
		if len(samples) > 0 && !time.Now().Before(until) {
			break
		}
		frameStarted := time.Now()
		data, err := s.captureScreen(ctx, req.Serial)
		if err != nil {
			return nil, err
		}
		data, currentSize, err := cropDeviceFrame(data, req.BBox)
		if err != nil {
			return nil, err
		}
		if len(samples) > 0 && currentSize != size {
			return nil, errors.New("采样期间设备尺寸或方向变化，请重新识别")
		}
		size = currentSize
		sampleReq := req
		if len(req.BBox) == 4 {
			x, y := int(req.BBox[0]*float64(size.X)), int(req.BBox[1]*float64(size.Y))
			right, bottom := int(math.Ceil((req.BBox[0]+req.BBox[2])*float64(size.X))), int(math.Ceil((req.BBox[1]+req.BBox[3])*float64(size.Y)))
			sampleReq.BBox = []float64{float64(x) / float64(size.X), float64(y) / float64(size.Y), float64(right-x) / float64(size.X), float64(bottom-y) / float64(size.Y)}
		}
		sample, err := s.recognize(ctx, data, sampleReq)
		if err != nil {
			if ctx.Err() != nil {
				return nil, ctx.Err()
			}
			return nil, err
		}
		samples = append(samples, sample)
		for _, engine := range sample.EnginesUsed {
			if !containsString(used, engine) {
				used = append(used, engine)
			}
		}
		for _, failure := range sample.FallbackReason {
			if !containsString(failures, failure) {
				failures = append(failures, failure)
			}
		}
		if req.StabilizationMS == 0 {
			break
		}
		wait := time.Until(frameStarted.Add(50 * time.Millisecond))
		remaining := time.Until(until)
		if wait > remaining {
			wait = remaining
		}
		if wait > 0 {
			timer := time.NewTimer(wait)
			select {
			case <-ctx.Done():
				timer.Stop()
				return nil, ctx.Err()
			case <-timer.C:
			}
		}
	}
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	var res *DetectResponse
	if req.StabilizationMS == 0 {
		res = samples[0]
	} else {
		res = fuseOCRResults(samples, true)
		res.Engine = samples[len(samples)-1].Engine
	}
	res.Samples = len(samples)
	res.EnginesUsed = used
	res.FallbackReason = failures
	res.ElapsedMS = time.Since(started).Milliseconds()
	req.Mark = mark
	req.Keywords = keywords
	filterConsensus(req, res)
	publish(req, res)
	log.Printf("[OCR] 设备识别完成 serial=%q engine=%s stabilization_ms=%d samples=%d elapsed_ms=%d", req.Serial, req.Engine, req.StabilizationMS, res.Samples, res.ElapsedMS)
	return res, nil
}
func containsString(values []string, value string) bool {
	for _, v := range values {
		if v == value {
			return true
		}
	}
	return false
}
func (s *Service) publishDetection(req DetectRequest, res *DetectResponse) {
	if !req.Mark || req.Serial == "" || s.hub == nil {
		return
	}
	duration := req.MarkDurationMS
	if duration <= 0 {
		duration = 3000
	}
	objects := make([]vision.Object, 0, len(res.Boxes))
	for _, b := range res.Boxes {
		objects = append(objects, vision.Object{Label: b.Text, Confidence: b.Confidence, X: b.X, Y: b.Y, W: b.W, H: b.H})
	}
	s.hub.PublishVisionResult(req.Serial, vision.Message{Type: "detection.result", DeviceID: req.Serial, Timestamp: time.Now().UnixMilli(), ExpiresMS: duration, Objects: objects})
}

func filterConsensus(req DetectRequest, res *DetectResponse) {
	if len(req.Keywords) == 0 {
		return
	}
	boxes := make([]Box, 0, len(res.Boxes))
	for _, b := range res.Boxes {
		for _, keyword := range req.Keywords {
			if strings.Contains(strings.ToLower(b.Text), strings.ToLower(keyword)) {
				boxes = append(boxes, b)
				break
			}
		}
	}
	res.Boxes = boxes
	res.Matched = len(boxes) > 0
	res.Status = "not_found"
	if res.Matched {
		res.Status = "matched"
	}
}

// Keep only live marking requests. A newer request supersedes the older one;
// finishing the newer request does not authorize a late older publication.
func (s *Service) markRequest(req DetectRequest) (func(DetectRequest, *DetectResponse), func()) {
	if !req.Mark || req.Serial == "" || s.hub == nil {
		return func(DetectRequest, *DetectResponse) {}, func() {}
	}
	s.markMu.Lock()
	if s.markRequests == nil {
		s.markRequests = make(map[string]uint64)
	}
	s.markRevision++
	revision := s.markRevision
	s.markRequests[req.Serial] = revision
	s.markMu.Unlock()
	publish := func(req DetectRequest, res *DetectResponse) {
		s.markMu.Lock()
		defer s.markMu.Unlock()
		if s.markRequests[req.Serial] == revision {
			s.publishDetection(req, res)
		}
	}
	finish := func() {
		s.markMu.Lock()
		defer s.markMu.Unlock()
		if s.markRequests[req.Serial] == revision {
			delete(s.markRequests, req.Serial)
		}
	}
	return publish, finish
}
