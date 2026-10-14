package ws

import (
	"context"
	"github.com/gorilla/websocket"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

func TestMatchingSessionsAreReadOnlyAndNeverKeepPlayerActive(t *testing.T) {
	ms := &managedSession{sess: &session{}, meta: &handshakeMeta{SessionID: "test", Codec: "h264", Width: 96, Height: 64}, refs: 1, subs: map[chan sharedVideoFrame]struct{}{}}
	replay := &managedSession{sess: &session{}, refs: 1, replay: true}
	h := &Hub{sessions: map[string]*managedSession{"phone": ms, "replay": replay}}
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { h.ServeMatchingVisionWS(w, r.WithContext(ctx)) }))
	defer server.Close()
	peer, _, err := websocket.DefaultDialer.Dial("ws"+strings.TrimPrefix(server.URL, "http")+"?serial=phone", nil)
	if err != nil {
		t.Fatal(err)
	}
	defer peer.Close()
	peer.SetReadDeadline(time.Now().Add(2 * time.Second))
	var hello map[string]any
	if err = peer.ReadJSON(&hello); err != nil {
		t.Fatal(err)
	}
	if hello["type"] != "hello" {
		t.Fatalf("unexpected hello: %v", hello)
	}
	if err = peer.WriteJSON(map[string]any{"type": "action.request", "request_id": "forbidden", "action": "tap", "x": 1, "y": 1, "protocol_version": "1"}); err != nil {
		t.Fatal(err)
	}
	var result map[string]any
	if err = peer.ReadJSON(&result); err != nil {
		t.Fatal(err)
	}
	if result["error_code"] != "read_only" || result["accepted"] != false || result["executed"] != false {
		t.Fatalf("action accepted: %v", result)
	}
	if devices := h.MatchingDevices(); len(devices) != 1 || devices[0] != "phone" {
		t.Fatalf("wrong devices: %v", devices)
	}
	ms.mu.Lock()
	ms.refs--
	ms.mu.Unlock() // The player closes; matching still holds its own reference.
	if devices := h.MatchingDevices(); len(devices) != 0 {
		t.Fatalf("matching kept phone active: %v", devices)
	}
	if _, _, _, err = h.acquireMatchingSession("phone"); err == nil {
		t.Fatal("reopened closed player")
	}
	if _, _, _, err = h.acquireMatchingSession("missing"); err == nil {
		t.Fatal("opened absent device")
	}
	// Restore a player reference so release does not need a real scrcpy process.
	ms.mu.Lock()
	ms.refs++
	ms.mu.Unlock()
	cancel()
	if _, _, err = peer.ReadMessage(); err == nil {
		t.Fatal("bridge cancellation did not close socket")
	}
	deadline := time.Now().Add(time.Second)
	for {
		ms.mu.Lock()
		refs := ms.matchingRefs
		ms.mu.Unlock()
		if refs == 0 {
			break
		}
		if time.Now().After(deadline) {
			t.Fatal("matching reference leaked")
		}
		time.Sleep(time.Millisecond)
	}
}
