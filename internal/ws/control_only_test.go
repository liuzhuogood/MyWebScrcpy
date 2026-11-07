package ws

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/gorilla/websocket"
)

func TestControlOnlySendsRealDimensionsWithoutMediaSubscriptions(t *testing.T) {
	ms := &managedSession{meta: &handshakeMeta{Serial: "device"}, width: 720, height: 1600}
	h := &Hub{sessions: map[string]*managedSession{"device": ms}}
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		conn, err := upgrader.Upgrade(w, r, nil)
		if err != nil {
			return
		}
		defer conn.Close()
		done := make(chan struct{})
		go func() {
			defer close(done)
			for {
				if _, _, err := conn.ReadMessage(); err != nil {
					return
				}
			}
		}()
		h.pumpSharedControl(conn, ms, done)
	}))
	defer server.Close()
	conn, _, err := websocket.DefaultDialer.Dial("ws"+strings.TrimPrefix(server.URL, "http"), nil)
	if err != nil {
		t.Fatal(err)
	}
	defer conn.Close()
	conn.SetReadDeadline(time.Now().Add(4 * time.Second))
	var meta struct {
		Type   string `json:"type"`
		Width  uint32 `json:"width"`
		Height uint32 `json:"height"`
	}
	if err := conn.ReadJSON(&meta); err != nil {
		t.Fatal(err)
	}
	if meta.Type != "control.meta" || meta.Width != 720 || meta.Height != 1600 {
		t.Fatalf("unexpected metadata: %+v", meta)
	}
	ms.mu.Lock()
	hasMedia := len(ms.subs) != 0 || len(ms.audioSubs) != 0 || len(ms.clipboardSubs) != 0
	ms.mu.Unlock()
	if hasMedia {
		t.Fatal("control-only client subscribed to media")
	}
	// 与 readSharedSession 的流终止处理一致：移除旧会话。
	h.sessionMu.Lock()
	delete(h.sessions, "device")
	h.sessionMu.Unlock()
	conn.SetReadDeadline(time.Now().Add(4 * time.Second))
	var stopped map[string]string
	if err := conn.ReadJSON(&stopped); err != nil || stopped["type"] != "disconnected" {
		t.Fatalf("old session did not disconnect: %v, %v", stopped, err)
	}
	if _, _, err := conn.ReadMessage(); err == nil {
		t.Fatal("old session kept the control-only socket alive")
	}
}
