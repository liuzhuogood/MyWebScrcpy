package ocr

import (
	"bytes"
	"context"
	"encoding/json"
	"image"
	"image/png"
	"net"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func TestIndraPriorityAndFallback(t *testing.T) {
	for _, test := range []struct {
		name        string
		indraStatus int
		engine      string
		want        string
		calls       string
	}{
		{"first", 200, "auto", "indraocr", "i"}, {"fallback", 503, "auto", "macocr", "im"},
		{"explicit-mac", 200, "macocr", "macocr", "m"}, {"explicit-paddle", 200, "paddleocr", "paddleocr", "p"},
		{"explicit-indra-fails", 503, "indraocr", "", "i"},
	} {
		t.Run(test.name, func(t *testing.T) {
			calls := ""
			makeServer := func(id string, status int) *httptest.Server {
				return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
					calls += id
					w.WriteHeader(status)
					_ = json.NewEncoder(w).Encode(map[string]any{"ocr_result": "Text", "ocr_boxes": []map[string]any{{"text": "Text", "confidence": .9, "x": .1, "y": .2, "w": .3, "h": .4}}})
				}))
			}
			i, m, p := makeServer("i", test.indraStatus), makeServer("m", 200), makeServer("p", 200)
			defer i.Close()
			defer m.Close()
			defer p.Close()
			t.Setenv("INDRAOCR_URL", i.URL)
			t.Setenv("MACOCR_URL", m.URL)
			t.Setenv("PADDLEOCR_URL", p.URL)
			var imageData bytes.Buffer
			_ = png.Encode(&imageData, image.NewRGBA(image.Rect(0, 0, 100, 200)))
			result, err := NewService(nil, nil).Detect(context.Background(), imageData.Bytes(), DetectRequest{Engine: test.engine})
			if test.want == "" {
				if err == nil {
					t.Fatal("expected explicit engine error")
				}
			} else {
				if err != nil || result.Engine != test.want {
					t.Fatalf("result=%+v error=%v", result, err)
				}
				if test.want == "indraocr" && result.Boxes[0].X != .001 {
					t.Fatalf("Indra subpixel box treated as normalized: %+v", result.Boxes[0])
				}
			}
			if calls != test.calls {
				t.Fatalf("calls=%q want=%q", calls, test.calls)
			}
		})
	}
}

func TestIndraEmptyResultDoesNotFallback(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(`{"ocr_result":"","ocr_boxes":[]}`))
	}))
	defer server.Close()
	t.Setenv("INDRAOCR_URL", server.URL)
	result, err := NewService(nil, nil).Detect(context.Background(), nil, DetectRequest{Engine: "auto"})
	if err != nil || result.Engine != "indraocr" || result.Matched || len(result.Boxes) != 0 {
		t.Fatalf("result=%+v error=%v", result, err)
	}
}

func TestIndraInvalidResponseRejected(t *testing.T) {
	for _, payload := range []string{`{}`, `{"ocr_result":"x","ocr_boxes":[{"text":"x"}]}`} {
		var result map[string]any
		_ = json.Unmarshal([]byte(payload), &result)
		if validateIndraResult(result) == nil {
			t.Fatal("malformed response accepted")
		}
	}
}

func TestIndraPreparationCanBeCancelledIndependently(t *testing.T) {
	runtime := &preparingRuntime{entered: make(chan struct{})}
	service := NewManagedService(nil, nil, runtime, nil)
	done := make(chan error, 1)
	go func() { done <- service.StartIndra(context.Background()) }()
	<-runtime.entered
	state := service.Status()
	if state.IndraStatus != "installing" || state.PaddleStatus == "installing" {
		t.Fatalf("bad independent state: %+v", state)
	}
	if err := service.StopIndra(); err != nil {
		t.Fatal(err)
	}
	select {
	case err := <-done:
		if err == nil {
			t.Fatal("cancelled startup succeeded")
		}
	case <-time.After(time.Second):
		t.Fatal("cancel did not complete")
	}
}

func TestIndraReadinessRejectsUnrelatedHealthyListener(t *testing.T) {
	foreign := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(`{"ready":true,"engine":"indraocr","pid":1}`))
	}))
	defer foreign.Close()
	port := foreign.Listener.Addr().(*net.TCPAddr).Port
	shim := createMockPythonShim(t, t.TempDir(), "python-hang")
	service := NewService(nil, nil).indra
	service.SetPaddlePort(port)
	defer service.StopPaddle()
	ctx, cancel := context.WithTimeout(context.Background(), 250*time.Millisecond)
	defer cancel()
	if err := service.startPaddleProcess(ctx, shim, "unused.py", func() {}); err == nil {
		t.Fatal("accepted another process's health response")
	}
}
