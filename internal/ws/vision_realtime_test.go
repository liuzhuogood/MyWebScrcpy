package ws

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/gorilla/websocket"
	"mywebscrcpy/internal/action"
	"mywebscrcpy/internal/scrcpy"
)

func TestVisionReconnectAfterRotationKeepsBootstrapAndReadOnlyContracts(t *testing.T) {
	for _, current := range []bool{false, true} {
		for _, readOnly := range []bool{false, true} {
			t.Run(fmt.Sprintf("current-%t-readonly-%t", current, readOnly), func(t *testing.T) {
				h := NewHub("adb", "")
				ms := &managedSession{h: h, sess: &session{}, meta: &handshakeMeta{Type: "meta", Codec: "h264", Serial: "rotated-device", SessionID: "same-session", Width: 448, Height: 1024}, refs: 1, width: 448, height: 1024, subs: make(map[chan sharedVideoFrame]struct{}), audioSubs: make(map[chan sharedAudioPacket]struct{}), clipboardSubs: make(map[chan string]struct{})}
				ms.actions = action.New(deviceActionExecutor{ms: ms}, 1)
				h.sessions["rotated-device"] = ms
				defer h.Close()
				for _, f := range []*scrcpy.Frame{
					{Kind: scrcpy.FrameSession, Width: 1024, Height: 448},
					{Kind: scrcpy.FrameConfig, Payload: []byte("rotated-config")},
					{Kind: scrcpy.FrameKey, PTS: 100, Payload: []byte("rotated-key")},
					{Kind: scrcpy.FrameDelta, PTS: 200, Payload: []byte("final-static")},
				} {
					cacheAndBroadcast(ms, f, encodeFrame(f))
				}
				handler := h.ServeVisionWS
				if readOnly {
					handler = h.ServeMatchingVisionWS
				}
				server := httptest.NewServer(http.HandlerFunc(handler))
				defer server.Close()
				conn, _, err := websocket.DefaultDialer.Dial("ws"+strings.TrimPrefix(server.URL, "http")+"?serial=rotated-device&current_frame="+fmt.Sprint(current), nil)
				if err != nil {
					t.Fatal(err)
				}
				defer conn.Close()
				conn.SetReadDeadline(time.Now().Add(2 * time.Second))
				var hello struct {
					Type      string `json:"type"`
					Width     uint32 `json:"width"`
					Height    uint32 `json:"height"`
					SessionID string `json:"session_id"`
				}
				if err := conn.ReadJSON(&hello); err != nil {
					t.Fatal(err)
				}
				if hello.Type != "hello" || hello.Width != 1024 || hello.Height != 448 || hello.SessionID != "same-session" {
					t.Fatalf("Vision announced stale rotated dimensions: %+v", hello)
				}
				kinds := []scrcpy.FrameKind{scrcpy.FrameConfig, scrcpy.FrameKey}
				if current {
					kinds = append(kinds, scrcpy.FrameDelta)
				}
				for i, kind := range kinds {
					var header struct {
						Type     string `json:"type"`
						Kind     byte   `json:"kind"`
						Width    uint32 `json:"width"`
						Height   uint32 `json:"height"`
						Replayed bool   `json:"replayed"`
						Complete bool   `json:"snapshot_complete"`
					}
					if err := conn.ReadJSON(&header); err != nil {
						t.Fatal(err)
					}
					if header.Type != "frame" || header.Kind != byte(kind) || header.Width != 1024 || header.Height != 448 || !header.Replayed || header.Complete != (current && i == len(kinds)-1) {
						t.Fatalf("incorrect bootstrap metadata: %+v", header)
					}
					typ, data, err := conn.ReadMessage()
					if err != nil {
						t.Fatal(err)
					}
					if typ != websocket.BinaryMessage || len(data) < 9 || data[0] != byte(kind) {
						t.Fatalf("invalid bootstrap binary: %v", data)
					}
				}
				if err := conn.WriteJSON(map[string]interface{}{"type": "hello", "protocol_version": "1", "device_id": "rotated-device", "session_id": "same-session", "max_fps": 0}); err != nil {
					t.Fatal(err)
				}
				var ack struct {
					Type   string `json:"type"`
					MaxFPS int    `json:"max_fps"`
				}
				if err := conn.ReadJSON(&ack); err != nil || ack.Type != "hello.ack" || ack.MaxFPS != 0 {
					t.Fatalf("max_fps contract changed: %+v %v", ack, err)
				}
				if readOnly {
					conn.WriteJSON(map[string]interface{}{"type": "action.request", "protocol_version": "1", "request_id": "not-allowed", "action": "key", "keycode": 3})
					var denied struct {
						Type      string `json:"type"`
						Accepted  bool   `json:"accepted"`
						Executed  bool   `json:"executed"`
						ErrorCode string `json:"error_code"`
					}
					if err := conn.ReadJSON(&denied); err != nil || denied.Type != "action.result" || denied.Accepted || denied.Executed || denied.ErrorCode != "read_only" {
						t.Fatalf("read-only action policy changed: %+v %v", denied, err)
					}
				}
				// Full packet delivery after hello.ack remains valid for both bootstrap modes.
				f := &scrcpy.Frame{Kind: scrcpy.FrameDelta, PTS: 300, Payload: []byte("live")}
				cacheAndBroadcast(ms, f, encodeFrame(f))
				var live map[string]json.RawMessage
				if err := conn.ReadJSON(&live); err != nil {
					t.Fatal(err)
				}
				if string(live["replayed"]) != "false" || string(live["width"]) != "1024" || string(live["height"]) != "448" {
					t.Fatalf("bad live dimensions/metadata: %v", live)
				}
				if typ, data, err := conn.ReadMessage(); err != nil || typ != websocket.BinaryMessage || string(data[9:]) != "live" {
					t.Fatalf("lost live packet: %v %v", data, err)
				}
				if ms.meta.Width != 448 || ms.meta.Height != 1024 {
					t.Fatal("Vision mutated shared initial metadata")
				}
			})
		}
	}
}
