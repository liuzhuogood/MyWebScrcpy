package matchingservice

import (
	"context"
	"encoding/binary"
	"encoding/json"
	"github.com/gorilla/websocket"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

// MYWEBSCRCPY_MATCHING_TEST_PYTHON opts into real decoding/matching with installed vision dependencies.
func TestRealPythonWorkers(t *testing.T) {
	python := os.Getenv("MYWEBSCRCPY_MATCHING_TEST_PYTHON")
	if python == "" {
		t.Skip("set MYWEBSCRCPY_MATCHING_TEST_PYTHON for full SDK integration")
	}
	dir := t.TempDir()
	generator := `import sys
from pathlib import Path
sys.path.insert(0,sys.argv[1])
from test_vision import assets
import cv2
pattern,scene,config,packet=assets()
d=Path(sys.argv[2])
for name,data in [('config',config),('packet',packet),('image',cv2.imencode('.png',pattern)[1].tobytes())]:
 (d/name).write_bytes(data)
`
	cmd := exec.Command(python, "-c", generator, filepath.Join("..", "..", "packages", "python-sdk", "tests"), dir)
	cmd.Env = append(os.Environ(), "PYTHONPATH="+filepath.Join("..", "..", "packages", "python-sdk", "src"))
	if output, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("fixture: %v %s", err, output)
	}
	config, _ := os.ReadFile(filepath.Join(dir, "config"))
	packet, _ := os.ReadFile(filepath.Join(dir, "packet"))
	png, _ := os.ReadFile(filepath.Join(dir, "image"))
	var mu sync.Mutex
	active := []string{"phone-a", "phone-b", "broken"}
	published := map[string]int{}
	var connections atomic.Int32
	var actions atomic.Int32
	var static atomic.Bool
	devices := func() []string { mu.Lock(); defer mu.Unlock(); return append([]string{}, active...) }
	api := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Query().Get("serial") == "broken" {
			writeJSON(w, 500, map[string]string{"error": "fixture_failure"})
			return
		}
		asset := map[string]any{"id": "target", "name": "目标", "serial": "global", "threshold": .95, "method": "TM_CCOEFF_NORMED", "grayscale": false, "scales": []float64{1}, "enabled": true, "width": 15, "height": 12, "image_count": 1, "created_at": "2026-10-14T00:00:00Z", "updated_at": "2026-10-14T00:00:00Z"}
		if r.URL.Path == "/api/templates/target/image" {
			w.Header().Set("Content-Type", "image/png")
			w.Write(png)
		} else if r.URL.Path == "/api/templates" {
			writeJSON(w, 200, map[string]any{"templates": []any{asset}, "total": 1})
		} else {
			writeJSON(w, 200, asset)
		}
	})
	vision := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		up := websocket.Upgrader{}
		peer, err := up.Upgrade(w, r, nil)
		if err != nil {
			return
		}
		connections.Add(1)
		defer connections.Add(-1)
		defer peer.Close()
		serial := r.URL.Query().Get("serial")
		sessionID := "session-" + serial
		peer.WriteJSON(map[string]any{"type": "hello", "protocol_version": "1", "device_id": serial, "session_id": sessionID, "format": "scrcpy-frame", "codec": "h264", "width": 96, "height": 64})
		var hello map[string]any
		if peer.ReadJSON(&hello) != nil {
			return
		}
		peer.WriteJSON(map[string]any{"type": "hello.ack", "device_id": serial, "session_id": sessionID, "max_fps": 0})
		done := make(chan struct{})
		go func() {
			defer close(done)
			for {
				_, data, err := peer.ReadMessage()
				if err != nil {
					return
				}
				var v struct {
					Type    string `json:"type"`
					Objects []any  `json:"objects"`
				}
				if json.Unmarshal(data, &v) != nil {
					return
				}
				if v.Type == "action.request" {
					actions.Add(1)
				}
				if v.Type == "detection.result" && len(v.Objects) > 0 {
					mu.Lock()
					published[serial]++
					mu.Unlock()
				}
			}
		}()
		var id uint64
		send := func(kind byte, payload []byte) bool {
			id++
			pts := id * 100000
			envelope := make([]byte, 9+len(payload))
			envelope[0] = kind
			binary.BigEndian.PutUint64(envelope[1:9], pts)
			copy(envelope[9:], payload)
			if peer.WriteJSON(map[string]any{"type": "frame", "device_id": serial, "session_id": sessionID, "frame_id": id, "pts": pts, "timestamp": time.Now().UnixMilli(), "width": 96, "height": 64, "kind": kind, "codec": "h264", "replayed": false}) != nil {
				return false
			}
			return peer.WriteMessage(websocket.BinaryMessage, envelope) == nil
		}
		if !send(0, config) {
			return
		}
		ticker := time.NewTicker(100 * time.Millisecond)
		defer ticker.Stop()
		for {
			select {
			case <-done:
				return
			case <-r.Context().Done():
				return
			case <-ticker.C:
				if serial == "phone-b" && static.Load() {
					continue
				}
				if !send(1, packet) {
					return
				}
			}
		}
	})
	m := New(Options{Python: func() (string, error) { return python, nil }, Devices: devices, API: api, Vision: vision, TemplateReadable: func(id, serial string) bool { return id == "target" }})
	defer m.Close()
	if err := m.Start(context.Background()); err != nil {
		t.Fatal(err)
	}
	pid := m.Status().PID
	await := func(assertion func() bool) {
		t.Helper()
		deadline := time.Now().Add(12 * time.Second)
		for !assertion() {
			if time.Now().After(deadline) {
				t.Fatalf("timed out: %+v connections=%d", m.Status(), connections.Load())
			}
			time.Sleep(30 * time.Millisecond)
		}
	}
	await(func() bool { mu.Lock(); defer mu.Unlock(); return published["phone-a"] > 0 && published["phone-b"] > 0 })
	await(func() bool {
		for _, d := range m.Status().Devices {
			if d.Serial == "broken" {
				return d.State == "failed"
			}
		}
		return false
	})
	if m.Status().PID != pid {
		t.Fatal("started per-device process")
	}
	mu.Lock()
	active = []string{"phone-b"}
	mu.Unlock()
	await(func() bool {
		d := m.Status().Devices
		return len(d) == 1 && d[0].Serial == "phone-b" && connections.Load() == 1
	})
	static.Store(true)
	await(func() bool {
		d := m.Status().Devices
		return len(d) == 1 && d[0].State == "waiting" && connections.Load() == 1
	})
	if m.Status().PID != pid {
		t.Fatal("static screen restarted process")
	}
	if err := m.Stop(); err != nil {
		t.Fatal(err)
	}
	await(func() bool { return connections.Load() == 0 })
	if actions.Load() != 0 {
		t.Fatal("automatic matcher sent device actions")
	}
	if m.Status().PID != 0 {
		t.Fatal("Python process leaked")
	}
}

func TestRealPythonMissingDependencies(t *testing.T) {
	python := os.Getenv("MYWEBSCRCPY_MATCHING_TEST_PYTHON")
	if python == "" {
		t.Skip("set MYWEBSCRCPY_MATCHING_TEST_PYTHON for interpreter tests")
	}
	dir := filepath.Join(t.TempDir(), "isolated")
	if output, err := exec.Command(python, "-m", "venv", "--without-pip", dir).CombinedOutput(); err != nil {
		t.Fatalf("venv: %v %s", err, output)
	}
	path := filepath.Join(dir, "bin", "python")
	if runtime.GOOS == "windows" {
		path = filepath.Join(dir, "Scripts", "python.exe")
	}
	m := testManager(path)
	defer m.Close()
	env, err := m.Check(context.Background())
	if err != nil || env.Ready || len(env.Dependencies) != 5 {
		t.Fatalf("unexpected detection: %+v %v", env, err)
	}
	for _, d := range env.Dependencies {
		if d.Available || d.Error == "" {
			t.Fatalf("missing import not detected: %+v", d)
		}
	}
	if err = m.Start(context.Background()); err == nil || m.Status().PID != 0 {
		t.Fatal("started without actual Python dependencies")
	}
}
