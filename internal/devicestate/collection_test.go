package devicestate

import (
	"bytes"
	"context"
	"encoding/json"
	"image/color"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"

	"mywebscrcpy/internal/ocr"
	"mywebscrcpy/internal/uipage"
	"mywebscrcpy/internal/vision"
	"mywebscrcpy/internal/visionframes"
)

func TestCollectStateWithoutUIDump(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("fixture uses a POSIX shell executable")
	}
	dir := t.TempDir()
	logPath := filepath.Join(dir, "commands.log")
	quote := func(path string) string { return "'" + strings.ReplaceAll(path, "'", "'\\''") + "'" }
	adb := filepath.Join(dir, "adb")
	script := "#!/bin/sh\nprintf '%s\\n' \"$*\" >> " + quote(logPath) + "\n" +
		"case \"$*\" in\n" +
		"  '-s phone shell input keyevent 224') exit 0;;\n" +
		"  '-s phone shell wm dismiss-keyguard') exit 0;;\n" +
		"  '-s phone get-state') printf 'device\\n';;\n" +
		"  *) exit 1;;\nesac\n"
	if err := os.WriteFile(adb, []byte(script), 0700); err != nil {
		t.Fatal(err)
	}
	pageReads := 0
	page := uipage.NewWithRunner(func(ctx context.Context, args ...string) ([]byte, error) {
		if strings.Join(args[2:], " ") == "shell dumpsys window" {
			pageReads++
			return []byte("mCurrentFocus=Window{123 u0 com.example/.MainActivity}"), nil
		}
		return nil, nil
	})
	ocrCalls := 0
	ocrServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		ocrCalls++
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"ocr_result":"fixture text","ocr_boxes":[]}`))
	}))
	defer ocrServer.Close()
	t.Setenv("INDRAOCR_URL", ocrServer.URL)
	frames := visionframes.NewStore()
	if _, err := frames.Put("phone", createTestJPEG(4, 6, color.White)); err != nil {
		t.Fatal(err)
	}
	svc := NewService(adb, page, ocr.NewService(nil, nil), nil, nil, frames)
	state, err := svc.CollectState(t.Context(), "phone", nil)
	if err != nil {
		t.Fatal(err)
	}
	if !state.ContextStable || pageReads != 2 {
		t.Fatalf("pre/post page validation missing: stable=%v reads=%d", state.ContextStable, pageReads)
	}
	if state.Screenshot.Width != 4 || state.Screenshot.Height != 6 || !strings.HasPrefix(state.Screenshot.DataBase64, "data:image/jpeg;base64,") {
		t.Fatalf("screenshot not collected from stream frame: %+v", state.Screenshot)
	}
	if ocrCalls != 1 || state.OCR.Engine != "indraocr" || state.OCR.Text != "fixture text" {
		t.Fatalf("OCR not preserved: calls=%d result=%+v", ocrCalls, state.OCR)
	}
	if len(state.PartialFailures) != 0 {
		t.Fatalf("unexpected collection failures: %v", state.PartialFailures)
	}
	if _, exists := state.Sources["ui_xml"]; exists {
		t.Fatal("removed XML source still present")
	}
	data, err := json.Marshal(state)
	if err != nil {
		t.Fatal(err)
	}
	if bytes.Contains(data, []byte(`"ui_xml`)) {
		t.Fatal("removed XML fields still in response")
	}
	commands, err := os.ReadFile(logPath)
	if err != nil {
		t.Fatal(err)
	}
	want := "-s phone shell input keyevent 224\n-s phone shell wm dismiss-keyguard\n-s phone get-state\n"
	if string(commands) != want {
		t.Fatalf("unexpected ADB workflow: %s", commands)
	}
}

type mockStreamKeeper struct {
	frame   visionframes.Frame
	err     error
	touched bool
}

func (m *mockStreamKeeper) EnsureFrame(ctx context.Context, serial string, maxWait time.Duration) (visionframes.Frame, error) {
	m.touched = true
	return m.frame, m.err
}

func (m *mockStreamKeeper) Touch(serial string, ttl time.Duration) error {
	m.touched = true
	return nil
}

func (m *mockStreamKeeper) IsActive(serial string) bool {
	return true
}

func TestCollectStateWithStreamKeeper(t *testing.T) {
	dir := t.TempDir()
	adb := filepath.Join(dir, "adb")
	script := "#!/bin/sh\n" +
		"case \"$*\" in\n" +
		"  '-s phone shell input keyevent 224') exit 0;;\n" +
		"  '-s phone shell wm dismiss-keyguard') exit 0;;\n" +
		"  '-s phone get-state') printf 'device\\n';;\n" +
		"  *) exit 1;;\nesac\n"
	if err := os.WriteFile(adb, []byte(script), 0700); err != nil {
		t.Fatal(err)
	}

	page := uipage.NewWithRunner(func(ctx context.Context, args ...string) ([]byte, error) {
		return []byte("mCurrentFocus=Window{123 u0 com.example/.MainActivity}"), nil
	})
	emptyFrames := visionframes.NewStore()
	svc := NewService(adb, page, nil, nil, nil, emptyFrames)

	testJPEG := createTestJPEG(8, 12, color.White)
	mockKeeper := &mockStreamKeeper{
		frame: visionframes.Frame{
			JPEG:   testJPEG,
			Width:  8,
			Height: 12,
		},
	}
	svc.SetStreamKeeper(mockKeeper)

	state, err := svc.CollectState(t.Context(), "phone", nil)
	if err != nil {
		t.Fatalf("CollectState with StreamKeeper failed: %v", err)
	}
	if state.Screenshot.Width != 8 || state.Screenshot.Height != 12 {
		t.Fatalf("unexpected screenshot dimensions from StreamKeeper: %+v", state.Screenshot)
	}
	if state.Sources["screenshot"].Status != "success" {
		t.Fatalf("expected screenshot source success, got: %+v", state.Sources["screenshot"])
	}
	if !mockKeeper.touched {
		t.Fatalf("expected StreamKeeper to be touched")
	}
}

type mockVisionHub struct {
	msg vision.Message
	ok  bool
}

func (m *mockVisionHub) LatestVisionResult(serial string) (vision.Message, bool) {
	return m.msg, m.ok
}

func TestCollectStateWithStreamTemplateMatching(t *testing.T) {
	frames := visionframes.NewStore()
	testJPEG := createTestJPEG(100, 200, color.White)
	_, _ = frames.Put("phone", testJPEG)

	svc := NewService("", nil, nil, nil, nil, frames)
	hub := &mockVisionHub{
		msg: vision.Message{
			Objects: []vision.Object{
				{
					Label:      "login_btn",
					Confidence: 0.96,
					X:          0.1,
					Y:          0.2,
					W:          0.4,
					H:          0.3,
				},
			},
		},
		ok: true,
	}
	svc.SetVisionHub(hub)

	state, err := svc.CollectState(t.Context(), "phone", nil)
	if err != nil {
		t.Fatalf("CollectState failed: %v", err)
	}
	if len(state.TemplateMatches) != 1 {
		t.Fatalf("expected 1 template match from stream, got: %d", len(state.TemplateMatches))
	}
	m := state.TemplateMatches[0]
	if m.TemplateID != "login_btn" || m.Confidence != 0.96 {
		t.Fatalf("unexpected match item: %+v", m)
	}
	if m.PixelX != 10 || m.PixelY != 40 || m.PixelW != 40 || m.PixelH != 60 {
		t.Fatalf("unexpected pixel coordinates: %+v", m)
	}
	if state.Sources["templates"].Status != "success" {
		t.Fatalf("expected templates status success, got: %+v", state.Sources["templates"])
	}
}

func TestCollectStateSkipOCR(t *testing.T) {
	frames := visionframes.NewStore()
	testJPEG := createTestJPEG(10, 10, color.White)
	_, _ = frames.Put("phone", testJPEG)

	ocrCalled := false
	ocrServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		ocrCalled = true
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"ocr_result":"text","ocr_boxes":[]}`))
	}))
	defer ocrServer.Close()
	t.Setenv("INDRAOCR_URL", ocrServer.URL)

	svc := NewService("", nil, ocr.NewService(nil, nil), nil, nil, frames)
	state, err := svc.CollectStateWithOptions(t.Context(), "phone", CollectOptions{
		IncludeOCR: false,
	})
	if err != nil {
		t.Fatalf("CollectStateWithOptions failed: %v", err)
	}
	if ocrCalled {
		t.Fatalf("expected OCR to NOT be called when IncludeOCR is false")
	}
	if state.Sources["ocr"].Status != "skipped" {
		t.Fatalf("expected ocr source status to be skipped, got: %+v", state.Sources["ocr"])
	}
}

