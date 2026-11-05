package ws

import (
	"encoding/json"
	"net"
	"net/http"
	"os"
	"sync"
	"testing"
	"time"

	"mywebscrcpy/internal/scrcpy"
)

// Opt-in real-WebSocket fixture. It never starts ADB or a device session and
// deliberately uses the production browser pump, cache and encoded envelope.
func TestStaticVideoBrowserFixture(t *testing.T) {
	path := os.Getenv("MYWEBSCRCPY_STATIC_FRAMES")
	if path == "" {
		t.Skip("opt-in browser fixture")
	}
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	var frames []scrcpy.Frame
	if err := json.Unmarshal(data, &frames); err != nil {
		t.Fatal(err)
	}
	ms := &managedSession{meta: &handshakeMeta{Type: "meta", Codec: "h264", Width: 160, Height: 96, Serial: "static-fixture", SessionID: "static-test-session"}, subs: make(map[chan sharedVideoFrame]struct{}), clipboardSubs: make(map[chan string]struct{})}
	h := &Hub{}
	var mu sync.Mutex
	connections := 0
	sockets := map[interface{ Close() error }]bool{}
	var start sync.Once
	finished := make(chan struct{})
	stopFixture := make(chan struct{})
	var finishOnce sync.Once
	mux := http.NewServeMux()
	mux.HandleFunc("POST /api/static-video/finish", func(w http.ResponseWriter, r *http.Request) {
		finishOnce.Do(func() { close(stopFixture) })
		w.WriteHeader(204)
	})
	mux.HandleFunc("GET /ws", func(w http.ResponseWriter, r *http.Request) {
		c, err := upgrader.Upgrade(w, r, nil)
		if err != nil {
			return
		}
		defer c.Close()
		mu.Lock()
		connections++
		sockets[c] = true
		mu.Unlock()
		defer func() { mu.Lock(); delete(sockets, c); mu.Unlock() }()
		if err := c.WriteJSON(ms.meta); err != nil {
			return
		}
		done := make(chan struct{})
		go func() {
			defer close(done)
			for {
				if _, _, err := c.ReadMessage(); err != nil {
					return
				}
			}
		}()
		h.pumpSharedVideo(c, ms, done)
	})
	mux.HandleFunc("POST /api/static-video/start", func(w http.ResponseWriter, r *http.Request) {
		start.Do(func() {
			go func() {
				for _, frame := range frames {
					value := frame
					cacheAndBroadcast(ms, &value, encodeFrame(&value))
					if value.Kind != scrcpy.FrameConfig {
						time.Sleep(200 * time.Millisecond)
					}
				}
				close(finished)
			}()
		})
		w.WriteHeader(204)
	})
	mux.HandleFunc("POST /api/static-video/drop", func(w http.ResponseWriter, r *http.Request) {
		mu.Lock()
		for c := range sockets {
			c.Close()
		}
		mu.Unlock()
		w.WriteHeader(204)
	})
	mux.HandleFunc("GET /api/static-video/status", func(w http.ResponseWriter, r *http.Request) {
		mu.Lock()
		n := connections
		mu.Unlock()
		done := false
		select {
		case <-finished:
			done = true
		default:
		}
		json.NewEncoder(w).Encode(map[string]interface{}{"connections": n, "finished": done})
	})
	mux.HandleFunc("/api/adb/commands", func(w http.ResponseWriter, r *http.Request) {
		json.NewEncoder(w).Encode(map[string]interface{}{"status": "completed", "exit_code": 0})
	})
	mux.HandleFunc("/api/screen-state", func(w http.ResponseWriter, r *http.Request) {
		json.NewEncoder(w).Encode(map[string]interface{}{"screenOn": true, "screenLocked": false})
	})
	mux.HandleFunc("/api/vision/frame", func(w http.ResponseWriter, r *http.Request) { w.WriteHeader(204) })
	mux.HandleFunc("/api/devices", func(w http.ResponseWriter, r *http.Request) { w.Write([]byte("[]")) })
	listener, err := net.Listen("tcp", "127.0.0.1:21992")
	if err != nil {
		t.Fatal(err)
	}
	server := &http.Server{Handler: mux, ReadHeaderTimeout: time.Second}
	defer server.Close()
	t.Log("static-video fixture ready at http://127.0.0.1:21992")
	go server.Serve(listener)
	// Bounded own task; /api/static-video/finish shuts down the fixture early.
	select {
	case <-stopFixture:
	case <-time.After(10 * time.Minute):
	}
	mu.Lock()
	for c := range sockets {
		c.Close()
	}
	mu.Unlock()
}
