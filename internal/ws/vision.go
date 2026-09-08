package ws

import (
	"context"
	"encoding/binary"
	"encoding/json"
	"net/http"
	"sync"
	"time"

	"github.com/gorilla/websocket"
	"mywebscrcpy/internal/action"
	debuglog "mywebscrcpy/internal/debug"
	"mywebscrcpy/internal/scrcpy"
	"mywebscrcpy/internal/vision"
)

// ServeVisionWS exposes a bidirectional vision stream. Video messages reuse the
// existing scrcpy frame envelope; Python returns JSON detection/action messages.
func (h *Hub) ServeVisionWS(w http.ResponseWriter, r *http.Request) {
	serial := r.URL.Query().Get("serial")
	if serial == "" {
		http.Error(w, "missing serial", http.StatusBadRequest)
		return
	}
	c, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		return
	}
	defer c.Close()
	var writeMu sync.Mutex
	var optionsMu sync.RWMutex
	// 客户端会在收到 hello 后发送自己的 max_fps。握手完成前不能按“不限帧”
	// 推送，否则高码率设备可能先把 WebSocket 写缓冲塞满，触发写超时并断开。
	maxFPS := 5
	writeJSON := func(v interface{}) error {
		writeMu.Lock()
		defer writeMu.Unlock()
		// 二进制帧的 deadline 不能沿用到下一次 JSON 帧头；静止画面时两帧
		// 之间可能超过 3 秒，否则下一次写会立即命中已过期的 deadline。
		c.SetWriteDeadline(time.Now().Add(3 * time.Second))
		return c.WriteJSON(v)
	}
	writeBinary := func(b []byte) error {
		writeMu.Lock()
		defer writeMu.Unlock()
		c.SetWriteDeadline(time.Now().Add(3 * time.Second))
		return c.WriteMessage(websocket.BinaryMessage, b)
	}
	c.SetReadLimit(1 << 20)
	ms, meta, cancelSession, err := h.acquireSession(serial)
	if err != nil {
		_ = writeJSON(map[string]interface{}{"type": "error", "message": err.Error()})
		return
	}
	defer cancelSession()
	width, height := ms.sess.conn.Size()
	_ = writeJSON(map[string]interface{}{"type": "hello", "protocol_version": "1", "device_id": serial, "session_id": meta.SessionID, "format": "scrcpy-frame", "codec": meta.Codec, "width": meta.Width, "height": meta.Height})
	done := make(chan struct{})
	var stopOnce sync.Once
	stop := func() { stopOnce.Do(func() { close(done) }) }
	defer stop()
	go func() {
		defer stop()
		for {
			_, data, e := c.ReadMessage()
			if e != nil {
				return
			}
			if len(data) == 0 {
				continue
			}
			var msg vision.Message
			if json.Unmarshal(data, &msg) != nil {
				_ = writeJSON(map[string]string{"type": "error", "code": "invalid_json"})
				continue
			}
			if !supportedVisionProtocol(msg.Protocol) {
				h.recordEvent(debuglog.Event{Type: "protocol.rejected", DeviceID: serial, SessionID: meta.SessionID, Message: "unsupported vision protocol", Fields: map[string]interface{}{"protocol_version": msg.Protocol}})
				_ = writeJSON(map[string]string{"type": "error", "code": "unsupported_protocol"})
				continue
			}
			if msg.DeviceID != "" && msg.DeviceID != serial {
				_ = writeJSON(map[string]string{"type": "error", "code": "device_mismatch"})
				continue
			}
			if msg.SessionID != "" && msg.SessionID != meta.SessionID {
				_ = writeJSON(map[string]string{"type": "error", "code": "session_mismatch"})
				continue
			}
			msg.DeviceID = serial
			msg.SessionID = meta.SessionID
			if msg.Type == "hello" {
				optionsMu.Lock()
				maxFPS = normalizeVisionFPS(msg.MaxFPS)
				optionsMu.Unlock()
				_ = writeJSON(map[string]interface{}{"type": "hello.ack", "device_id": serial, "session_id": meta.SessionID, "max_fps": maxFPS})
			} else if msg.Type == "heartbeat" {
				_ = writeJSON(map[string]interface{}{"type": "heartbeat", "device_id": serial, "session_id": meta.SessionID, "timestamp": time.Now().UnixMilli()})
			} else if msg.Type == "detection.result" {
				if vision.IsExpired(msg, time.Now()) {
					h.recordEvent(debuglog.Event{Type: "detection.expired", DeviceID: serial, SessionID: meta.SessionID, Message: "result exceeded expires_ms"})
					_ = writeJSON(map[string]string{"type": "error", "code": "expired_result"})
					continue
				}
				if e := vision.ValidateObjects(msg.Objects); e != nil {
					h.recordEvent(debuglog.Event{Type: "detection.rejected", DeviceID: serial, SessionID: meta.SessionID, Message: e.Error()})
					_ = writeJSON(map[string]string{"type": "error", "code": "invalid_detection", "detail": e.Error()})
					continue
				}
				h.publishResult(serial, msg)
			} else if msg.Type == "action.request" {
				result := <-ms.actions.Submit(context.Background(), action.Request{RequestID: msg.RequestID, DeviceID: serial, Source: "vision", Action: msg.Action, X: msg.X, Y: msg.Y, X2: msg.X2, Y2: msg.Y2, Text: msg.Text, Keycode: msg.Keycode, FrameID: msg.FrameID, ExpiresMS: msg.ExpiresMS})
				h.recordEvent(debuglog.Event{Type: "action.result", DeviceID: serial, SessionID: meta.SessionID, RequestID: result.RequestID, Fields: map[string]interface{}{"accepted": result.Accepted, "executed": result.Executed, "error_code": result.ErrorCode}})
				_ = writeJSON(map[string]interface{}{"type": "action.result", "request_id": result.RequestID, "device_id": serial, "session_id": meta.SessionID, "accepted": result.Accepted, "executed": result.Executed, "error_code": result.ErrorCode})
			}
		}
	}()
	frames, cancelFrames := h.subscribeShared(ms)
	defer cancelFrames()
	var frameID uint64
	var lastSent time.Time
	for {
		select {
		case <-done:
			return
		default:
		}
		frame, ok := <-frames
		if !ok {
			return
		}
		data := frame.data
		frameID++
		kind := data[0]
		pts := binary.BigEndian.Uint64(data[1:9])
		if kind == byte(scrcpy.FrameSession) && len(data) >= 17 {
			width = binary.BigEndian.Uint32(data[9:13])
			height = binary.BigEndian.Uint32(data[13:17])
		}
		optionsMu.RLock()
		limit := maxFPS
		optionsMu.RUnlock()
		now := time.Now()
		if !shouldEmitVisionFrame(kind, limit, lastSent, now) {
			continue
		}
		lastSent = now
		if e := writeJSON(map[string]interface{}{"type": "frame", "device_id": serial, "session_id": meta.SessionID, "frame_id": frameID, "pts": pts, "timestamp": time.Now().UnixMilli(), "width": width, "height": height, "kind": kind, "codec": meta.Codec, "replayed": frame.replayed}); e != nil {
			h.recordEvent(debuglog.Event{Type: "vision.write_error", DeviceID: serial, SessionID: meta.SessionID, Message: e.Error(), Fields: map[string]interface{}{"stage": "frame_metadata"}})
			return
		}
		if e := writeBinary(data); e != nil {
			h.recordEvent(debuglog.Event{Type: "vision.write_error", DeviceID: serial, SessionID: meta.SessionID, Message: e.Error(), Fields: map[string]interface{}{"stage": "frame_binary", "kind": kind, "bytes": len(data)}})
			return
		}
	}
}

// Empty protocol_version remains accepted for backwards compatibility. Clients
// that send a version explicitly must use the currently supported protocol.
func supportedVisionProtocol(protocol string) bool {
	return protocol == "" || protocol == "1"
}

func normalizeVisionFPS(maxFPS int) int {
	if maxFPS < 0 {
		return 0
	}
	if maxFPS > 30 {
		return 30
	}
	return maxFPS
}

func shouldEmitVisionFrame(kind byte, maxFPS int, lastSent, now time.Time) bool {
	// Configuration, key and session packets must not be sampled out: they are
	// required to initialize/reinitialize the decoder after reconnect/rotation.
	if kind != byte(scrcpy.FrameDelta) || maxFPS <= 0 || lastSent.IsZero() {
		return true
	}
	return now.Sub(lastSent) >= time.Second/time.Duration(maxFPS)
}

func (h *Hub) publishResult(serial string, msg vision.Message) {
	h.resultMu.Lock()
	defer h.resultMu.Unlock()
	for ch := range h.results[serial] {
		select {
		case ch <- msg:
		default:
		}
	}
}

// ServeVisionResults streams detection results to a browser overlay.
func (h *Hub) ServeVisionResults(w http.ResponseWriter, r *http.Request) {
	serial := r.URL.Query().Get("serial")
	if serial == "" {
		http.Error(w, "missing serial", 400)
		return
	}
	c, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		return
	}
	defer c.Close()
	ch := make(chan vision.Message, 8)
	h.resultMu.Lock()
	if h.results[serial] == nil {
		h.results[serial] = make(map[chan vision.Message]struct{})
	}
	h.results[serial][ch] = struct{}{}
	h.resultMu.Unlock()
	defer func() { h.resultMu.Lock(); delete(h.results[serial], ch); h.resultMu.Unlock() }()
	for msg := range ch {
		c.SetWriteDeadline(time.Now().Add(3 * time.Second))
		if err := c.WriteJSON(msg); err != nil {
			return
		}
	}
}
