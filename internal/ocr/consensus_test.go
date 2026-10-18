package ocr

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"github.com/gorilla/websocket"
	"image"
	"image/png"
	"io"
	"math"
	"mime/multipart"
	"mywebscrcpy/internal/vision"
	"mywebscrcpy/internal/ws"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
	"time"
)

func consensusPNG() []byte {
	var b bytes.Buffer
	_ = png.Encode(&b, image.NewRGBA(image.Rect(0, 0, 100, 200)))
	return b.Bytes()
}
func consensusPayload(text string, x float64) string {
	return fmt.Sprintf(`{"ocr_result":%q,"ocr_boxes":[{"text":%q,"confidence":0.9,"x":%v,"y":20,"w":30,"h":10}]}`, text, text, x)
}

func TestParallelActuallyOverlapsAndDeduplicates(t *testing.T) {
	entered := make(chan struct{}, 3)
	release := make(chan struct{})
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		entered <- struct{}{}
		select {
		case <-release:
		case <-r.Context().Done():
			return
		}
		_, _ = w.Write([]byte(consensusPayload("文字", 10)))
	}))
	defer server.Close()
	for _, key := range []string{"INDRAOCR_URL", "MACOCR_URL", "PADDLEOCR_URL"} {
		t.Setenv(key, server.URL)
	}
	done := make(chan *DetectResponse, 1)
	errs := make(chan error, 1)
	go func() {
		res, err := NewService(nil, nil).Detect(t.Context(), consensusPNG(), DetectRequest{Engine: "parallel"})
		done <- res
		errs <- err
	}()
	for range 3 {
		select {
		case <-entered:
		case <-time.After(time.Second):
			close(release)
			t.Fatal("engines were not concurrent")
		}
	}
	close(release)
	res := <-done
	if err := <-errs; err != nil {
		t.Fatal(err)
	}
	if res.Engine != "parallel" || len(res.Boxes) != 1 || len(res.EnginesUsed) != 3 {
		t.Fatalf("bad fused result: %+v", res)
	}
}

func TestParallelPartialAndAllFailure(t *testing.T) {
	good := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { _, _ = w.Write([]byte(consensusPayload("文字", 10))) }))
	defer good.Close()
	bad := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { http.Error(w, "offline", 503) }))
	defer bad.Close()
	t.Setenv("INDRAOCR_URL", good.URL)
	t.Setenv("MACOCR_URL", bad.URL)
	t.Setenv("PADDLEOCR_URL", bad.URL)
	svc := NewService(nil, nil)
	res, err := svc.Detect(t.Context(), consensusPNG(), DetectRequest{Engine: "parallel"})
	if err != nil || len(res.Boxes) != 1 || len(res.FallbackReason) != 2 {
		t.Fatalf("res=%+v err=%v", res, err)
	}
	t.Setenv("INDRAOCR_URL", bad.URL)
	if _, err = svc.Detect(t.Context(), consensusPNG(), DetectRequest{Engine: "parallel"}); err == nil {
		t.Fatal("all failed accepted")
	}
}

func TestStabilizationSamplesFreshFramesAndCancellation(t *testing.T) {
	var captures atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(consensusPayload("稳定", float64(10+captures.Load()%3))))
	}))
	defer server.Close()
	t.Setenv("INDRAOCR_URL", server.URL)
	svc := NewService(nil, nil)
	svc.captureScreen = func(ctx context.Context, serial string) ([]byte, error) { captures.Add(1); return consensusPNG(), nil }
	started := time.Now()
	res, err := svc.DetectDevice(t.Context(), DetectRequest{Serial: "phone", StabilizationMS: 160})
	if err != nil || res.Samples < 2 || captures.Load() != int32(res.Samples) || len(res.Boxes) != 1 || time.Since(started) < 160*time.Millisecond {
		t.Fatalf("res=%+v captures=%v err=%v", res, captures.Load(), err)
	}
	ctx, cancel := context.WithCancel(t.Context())
	cancel()
	before := captures.Load()
	if _, err = svc.DetectDevice(ctx, DetectRequest{Serial: "phone", StabilizationMS: 100}); err == nil || captures.Load() != before {
		t.Fatal("cancelled request captured")
	}
}

func TestStabilizationParameterHTTP(t *testing.T) {
	svc := NewService(nil, nil)
	var captures atomic.Int32
	svc.captureScreen = func(context.Context, string) ([]byte, error) { captures.Add(1); return consensusPNG(), nil }
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(`{"ocr_result":"","ocr_boxes":[]}`))
	}))
	defer server.Close()
	t.Setenv("INDRAOCR_URL", server.URL)
	mux := http.NewServeMux()
	svc.RegisterRoutes(mux)
	for _, value := range []string{"-1", "5001", "1.5", "true", "null", "\"500\""} {
		before := captures.Load()
		w := httptest.NewRecorder()
		r := httptest.NewRequest("POST", "/api/vision/ocr", strings.NewReader(`{"serial":"phone","stabilization_ms":`+value+`}`))
		r.Header.Set("Content-Type", "application/json")
		mux.ServeHTTP(w, r)
		if w.Code != 400 || captures.Load() != before {
			t.Errorf("%s: status=%d body=%s", value, w.Code, w.Body.String())
		}
	}
	w := httptest.NewRecorder()
	r := httptest.NewRequest("POST", "/api/vision/ocr", strings.NewReader(`{"serial":"phone","stabilization_ms":100}`))
	r.Header.Set("Content-Type", "application/json")
	mux.ServeHTTP(w, r)
	var res DetectResponse
	_ = json.Unmarshal(w.Body.Bytes(), &res)
	if w.Code != 200 || res.Samples < 2 {
		t.Fatalf("status=%d body=%s", w.Code, w.Body.String())
	}
}

func TestFuseSamplesRejectsOldPageAndDuplicateVotes(t *testing.T) {
	b := func(text string, x float64) Box { return Box{Text: text, X: x, Y: .1, W: .2, H: .1, Confidence: .9} }
	samples := []*DetectResponse{{Boxes: []Box{b("旧页面", .1), b("消失", .6)}}, {Boxes: []Box{b("旧页面", .11)}}, {Boxes: []Box{b("新页面", .1)}}}
	res := fuseOCRResults(samples, true)
	if len(res.Boxes) != 1 || res.Boxes[0].Text != "新页面" {
		t.Fatalf("old page survived: %+v", res)
	}
}

func TestParallelCancellationDrainsWorkers(t *testing.T) {
	var active atomic.Int32
	entered := make(chan struct{}, 3)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = io.Copy(io.Discard, r.Body)
		active.Add(1)
		defer active.Add(-1)
		entered <- struct{}{}
		<-r.Context().Done()
	}))
	defer server.Close()
	for _, key := range []string{"INDRAOCR_URL", "MACOCR_URL", "PADDLEOCR_URL"} {
		t.Setenv(key, server.URL)
	}
	ctx, cancel := context.WithCancel(t.Context())
	defer cancel()
	done := make(chan error, 1)
	go func() {
		_, err := NewService(nil, nil).Detect(ctx, consensusPNG(), DetectRequest{Engine: "parallel"})
		done <- err
	}()
	for range 3 {
		select {
		case <-entered:
		case <-time.After(time.Second):
			t.Fatal("engine not started")
		}
	}
	cancel()
	select {
	case err := <-done:
		if !errors.Is(err, context.Canceled) {
			t.Fatalf("wrong cancellation %v", err)
		}
	case <-time.After(time.Second):
		t.Fatal("cancellation did not drain workers")
	}
	deadline := time.Now().Add(time.Second)
	for active.Load() != 0 && time.Now().Before(deadline) {
		time.Sleep(time.Millisecond)
	}
	if active.Load() != 0 {
		t.Fatalf("workers still active %d", active.Load())
	}
}

func TestDeviceWindowRejectsDimensionChangeAndLimitsCapacity(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { _, _ = w.Write([]byte(consensusPayload("Text", 10))) }))
	defer server.Close()
	t.Setenv("INDRAOCR_URL", server.URL)
	svc := NewService(nil, nil)
	calls := 0
	svc.captureScreen = func(context.Context, string) ([]byte, error) {
		calls++
		if calls == 1 {
			return consensusPNG(), nil
		}
		var b bytes.Buffer
		_ = png.Encode(&b, image.NewRGBA(image.Rect(0, 0, 200, 100)))
		return b.Bytes(), nil
	}
	if _, err := svc.DetectDevice(t.Context(), DetectRequest{Serial: "phone", StabilizationMS: 150}); err == nil || !strings.Contains(err.Error(), "尺寸") {
		t.Fatalf("dimension change accepted: %v", err)
	}
	first, _ := svc.acquireDetection()
	second, _ := svc.acquireDetection()
	defer first()
	defer second()
	if _, err := svc.DetectDevice(t.Context(), DetectRequest{Serial: "phone"}); !errors.Is(err, errOCRBusy) {
		t.Fatalf("no capacity limit: %v", err)
	}
}

func TestUploadedStabilizationAndMultipartValidation(t *testing.T) {
	svc := NewService(nil, nil)
	mux := http.NewServeMux()
	svc.RegisterRoutes(mux)
	for _, v := range []string{"-1", "5001", "1.5", "true", "", "500"} {
		var body bytes.Buffer
		writer := multipart.NewWriter(&body)
		_ = writer.WriteField("stabilization_ms", v)
		if v == "500" {
			part, _ := writer.CreateFormFile("file", "frame.png")
			_, _ = part.Write(consensusPNG())
			_ = writer.WriteField("serial", "phone")
		}
		_ = writer.Close()
		w := httptest.NewRecorder()
		r := httptest.NewRequest("POST", "/api/vision/ocr", &body)
		r.Header.Set("Content-Type", writer.FormDataContentType())
		mux.ServeHTTP(w, r)
		if w.Code != 400 {
			t.Errorf("%q accepted: %d %s", v, w.Code, w.Body.String())
		}
	}
}

func TestDeviceROIAndKeywordFilteringAfterVoting(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		file, _, err := r.FormFile("file")
		if err != nil {
			t.Error(err)
			return
		}
		defer file.Close()
		config, _, err := image.DecodeConfig(file)
		if err != nil || config.Width != 50 || config.Height != 100 {
			t.Errorf("crop: %+v %v", config, err)
		}
		_, _ = w.Write([]byte(consensusPayload("target", 10)))
	}))
	defer server.Close()
	t.Setenv("INDRAOCR_URL", server.URL)
	svc := NewService(nil, nil)
	svc.captureScreen = func(context.Context, string) ([]byte, error) { return consensusPNG(), nil }
	res, err := svc.DetectDevice(t.Context(), DetectRequest{Serial: "phone", BBox: []float64{.2, .25, .5, .5}, StabilizationMS: 100, Keywords: []string{"target"}})
	if err != nil || len(res.Boxes) != 1 || math.Abs(res.Boxes[0].X-.3) > .00001 {
		t.Fatalf("bad ROI mapping: %+v %v", res, err)
	}
}

func TestParallelSlowEngineDoesNotDiscardSuccess(t *testing.T) {
	good := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { _, _ = w.Write([]byte(consensusPayload("Text", 10))) }))
	defer good.Close()
	slow := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { _, _ = io.Copy(io.Discard, r.Body); <-r.Context().Done() }))
	defer slow.Close()
	t.Setenv("INDRAOCR_URL", good.URL)
	t.Setenv("MACOCR_URL", slow.URL)
	t.Setenv("PADDLEOCR_URL", good.URL)
	svc := NewService(nil, nil)
	svc.parallelEngineTimeout = 30 * time.Millisecond
	res, err := svc.Detect(t.Context(), consensusPNG(), DetectRequest{Engine: "parallel"})
	if err != nil || res == nil || len(res.EnginesUsed) != 2 || len(res.FallbackReason) != 1 {
		t.Fatalf("lost successful engines: %+v %v", res, err)
	}
}

func TestDeviceDeadlineReturnsGatewayTimeout(t *testing.T) {
	svc := NewService(nil, nil)
	svc.captureScreen = func(ctx context.Context, _ string) ([]byte, error) { <-ctx.Done(); return nil, ctx.Err() }
	ctx, cancel := context.WithTimeout(t.Context(), 20*time.Millisecond)
	defer cancel()
	req := httptest.NewRequest("POST", "/api/vision/ocr", strings.NewReader(`{"serial":"phone","stabilization_ms":500}`)).WithContext(ctx)
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	mux := http.NewServeMux()
	svc.RegisterRoutes(mux)
	mux.ServeHTTP(w, req)
	if w.Code != 504 {
		t.Fatalf("timeout returned %d: %s", w.Code, w.Body.String())
	}
}

func TestLateMarkCannotOverwriteNewerCompletedRequest(t *testing.T) {
	hub := ws.NewHub("adb", "")
	svc := NewService(nil, hub)
	req := DetectRequest{Serial: "phone", Mark: true}
	old, finishOld := svc.markRequest(req)
	defer finishOld()
	latest, finishLatest := svc.markRequest(req)
	latest(req, &DetectResponse{Boxes: []Box{{Text: "new", W: .1, H: .1}}})
	finishLatest()
	old(req, &DetectResponse{Boxes: []Box{{Text: "old", W: .1, H: .1}}})
	server := httptest.NewServer(http.HandlerFunc(hub.ServeVisionResults))
	defer server.Close()
	conn, _, err := websocket.DefaultDialer.Dial("ws"+strings.TrimPrefix(server.URL, "http")+"?serial=phone", nil)
	if err != nil {
		t.Fatal(err)
	}
	defer func() {
		conn.Close()
		hub.PublishVisionResult("phone", vision.Message{})
		hub.PublishVisionResult("phone", vision.Message{})
	}()
	conn.SetReadDeadline(time.Now().Add(time.Second))
	var msg vision.Message
	if err := conn.ReadJSON(&msg); err != nil {
		t.Fatal(err)
	}
	if len(msg.Objects) != 1 || msg.Objects[0].Label != "new" {
		t.Fatalf("stale mark published: %+v", msg)
	}
}
