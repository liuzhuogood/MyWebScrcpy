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

	"mywebscrcpy/internal/ocr"
	"mywebscrcpy/internal/uipage"
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
	want := "-s phone get-state\n"
	if string(commands) != want {
		t.Fatalf("unexpected ADB workflow (screencap should be gone): %s", commands)
	}
}
