package taskmanager

import (
	"encoding/base64"
	"encoding/json"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/gorilla/websocket"
	"mywebscrcpy/internal/dependencies"
)

func TestTaskBridgeScopeAndSocketRevocation(t *testing.T) {
	_, m := artifactManager(t)
	disconnected := make(chan struct{}, 1)
	m.TemplateReadable = func(id, serial string) bool { return id == "allowed" && serial == "device-A" }
	mux := http.NewServeMux()
	mux.HandleFunc("GET /api/templates/{id}", func(w http.ResponseWriter, r *http.Request) {
		json.NewEncoder(w).Encode(map[string]string{"id": r.PathValue("id")})
	})
	mux.HandleFunc("GET /api/vision/stream", func(w http.ResponseWriter, r *http.Request) {
		c, err := (&websocket.Upgrader{}).Upgrade(w, r, nil)
		if err != nil {
			return
		}
		defer c.Close()
		for {
			if _, _, err := c.ReadMessage(); err != nil {
				disconnected <- struct{}{}
				return
			}
		}
	})
	if err := m.ConfigureAPI(mux); err != nil {
		t.Fatal(err)
	}
	task, err := m.StartTask(0, "scope", "device-A", 30, "import time\ntime.sleep(30)")
	if err != nil {
		t.Fatal(err)
	}
	m.mu.Lock()
	token := m.procs[task.ID].apiToken
	m.mu.Unlock()
	for _, test := range []struct {
		path, method string
		status       int
	}{
		{"/api/templates/allowed?serial=device-A", "GET", 200},
		{"/api/templates/other?serial=device-A", "GET", 403},
		{"/api/templates/allowed?serial=device-B", "GET", 401},
		{"/api/templates/allowed?serial=device-A", "DELETE", 404},
		{"/api/templates/export?serial=device-A", "GET", 404},
		{"/api/devices?serial=device-A", "GET", 404},
	} {
		request, _ := http.NewRequest(test.method, m.apiURL+test.path, nil)
		request.Header.Set("Authorization", "Bearer "+token)
		response, err := http.DefaultClient.Do(request)
		if err != nil {
			t.Fatal(err)
		}
		response.Body.Close()
		if response.StatusCode != test.status {
			t.Fatalf("%s: %d", test.path, response.StatusCode)
		}
	}
	c, _, err := websocket.DefaultDialer.Dial("ws"+strings.TrimPrefix(m.apiURL, "http")+"/api/vision/stream?serial=device-A", http.Header{"Authorization": []string{"Bearer " + token}})
	if err != nil {
		t.Fatal(err)
	}
	defer c.Close()
	if err = m.KillTask(task.ID); err != nil {
		t.Fatal(err)
	}
	select {
	case <-disconnected:
	case <-time.After(3 * time.Second):
		t.Fatal("hijacked task socket survived job termination")
	}
}

func TestInstalledSDKVisionThroughTaskBridge(t *testing.T) {
	if os.Getenv("MYWEBSCRCPY_SDK_VISION_TEST") != "1" {
		t.Skip("set MYWEBSCRCPY_SDK_VISION_TEST=1 with vision dependencies")
	}
	python, err := dependencies.PythonPath()
	if err != nil {
		t.Fatal(err)
	}
	command := exec.Command(python, "-c", `import json,base64,cv2; from test_vision import assets; p,i,c,b=assets(); _,png=cv2.imencode('.png',p); print(json.dumps({k:base64.b64encode(v).decode() for k,v in {'config':c,'packet':b,'png':png.tobytes()}.items()}))`)
	directory, _ := filepath.Abs("../../packages/python-sdk/tests")
	command.Env = append(os.Environ(), "PYTHONPATH="+directory)
	output, err := command.Output()
	if err != nil {
		t.Fatal(err)
	}
	var encoded map[string]string
	if err = json.Unmarshal(output, &encoded); err != nil {
		t.Fatal(err)
	}
	blobs := map[string][]byte{}
	for key, value := range encoded {
		blobs[key], err = base64.StdEncoding.DecodeString(value)
		if err != nil {
			t.Fatal(err)
		}
	}
	_, m := artifactManager(t)
	m.TemplateReadable = func(id, serial string) bool { return id == "target" && serial == "device-A" }
	var actions atomic.Int32
	mux := http.NewServeMux()
	mux.HandleFunc("GET /api/templates/target", func(w http.ResponseWriter, r *http.Request) {
		json.NewEncoder(w).Encode(map[string]any{"id": "target", "name": "target", "serial": "global", "threshold": .95, "method": "TM_CCOEFF_NORMED", "grayscale": false, "enabled": true, "width": 15, "height": 12, "image_count": 1, "created_at": "2026-10-13T00:00:00Z", "updated_at": "2026-10-13T00:00:00Z"})
	})
	mux.HandleFunc("GET /api/templates/target/image", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "image/png")
		w.Write(blobs["png"])
	})
	mux.HandleFunc("GET /api/vision/stream", func(w http.ResponseWriter, r *http.Request) {
		c, err := (&websocket.Upgrader{}).Upgrade(w, r, nil)
		if err != nil {
			return
		}
		defer c.Close()
		hello := map[string]any{"type": "hello", "protocol_version": "1", "device_id": "device-A", "session_id": "session", "format": "scrcpy-frame", "codec": "h264", "width": 96, "height": 64}
		if c.WriteJSON(hello) != nil {
			return
		}
		var message map[string]any
		if c.ReadJSON(&message) != nil {
			return
		}
		if message["max_fps"] != float64(0) {
			t.Error("SDK did not request complete encoded stream")
		}
		var frameID uint64
		packet := func(kind byte, data []byte) error {
			frameID++
			pts := frameID * 100000
			if err := c.WriteJSON(map[string]any{"type": "frame", "device_id": "device-A", "session_id": "session", "frame_id": frameID, "pts": pts, "timestamp": time.Now().UnixMilli(), "width": 96, "height": 64, "kind": kind, "codec": "h264", "replayed": false}); err != nil {
				return err
			}
			envelope := make([]byte, 9)
			envelope[0] = kind
			for i := 0; i < 8; i++ {
				envelope[8-i] = byte(pts >> (i * 8))
			}
			return c.WriteMessage(websocket.BinaryMessage, append(envelope, data...))
		}
		if packet(0, blobs["config"]) != nil {
			return
		}
		ticker := time.NewTicker(40 * time.Millisecond)
		defer ticker.Stop()
		requests := make(chan map[string]any, 1)
		done := make(chan struct{})
		go func() {
			defer close(done)
			for {
				var request map[string]any
				if c.ReadJSON(&request) != nil {
					return
				}
				select {
				case requests <- request:
				case <-r.Context().Done():
					return
				}
			}
		}()
		for {
			select {
			case <-done:
				return
			case <-ticker.C:
				if packet(1, blobs["packet"]) != nil {
					return
				}
			case request := <-requests:
				if request["type"] == "action.request" {
					actions.Add(1)
					if request["action"] != "tap" || request["frame_id"] == nil {
						t.Error("invalid matched tap")
					}
					if c.WriteJSON(map[string]any{"type": "action.result", "device_id": "device-A", "session_id": "session", "request_id": request["request_id"], "accepted": true, "executed": true, "error_code": ""}) != nil {
						return
					}
				}
			}
		}
	})
	if err = m.ConfigureAPI(mux); err != nil {
		t.Fatal(err)
	}
	source, err := os.ReadFile("../../.pi/skills/markdown-to-python/assets/vision-example.py")
	if err != nil {
		t.Fatal(err)
	}
	code := strings.ReplaceAll(string(source), "CONFIRMED_TEMPLATE_ID", "target")
	task, err := m.StartTask(0, "SDK Vision", "device-A", 15, code)
	if err != nil {
		t.Fatal(err)
	}
	artifactDone(t, m, task.ID)
	record, _ := m.database.GetTaskByID(task.ID)
	if record.Status != "completed" || record.ExitCode != 0 || actions.Load() != 1 {
		t.Fatalf("record=%+v actions=%d", record, actions.Load())
	}
}
