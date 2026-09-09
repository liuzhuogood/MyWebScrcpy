package ws

import (
	"context"
	"encoding/binary"
	"errors"
	"log"
	"sync"
	"time"

	"github.com/gorilla/websocket"
	"mywebscrcpy/internal/action"
	debuglog "mywebscrcpy/internal/debug"
	"mywebscrcpy/internal/devicegate"
	"mywebscrcpy/internal/scrcpy"
)

type managedSession struct {
	sess       *session
	meta       *handshakeMeta
	mu         sync.Mutex
	refs       int
	subs       map[chan sharedVideoFrame]struct{}
	controlMu  sync.Mutex
	closeTimer *time.Timer
	lastConfig []byte
	lastKey    []byte
	actions    *action.Queue
	width      uint32
	height     uint32
	frames     uint64
	dropped    uint64
	lastFrame  time.Time
}

const sharedVideoQueueSize = 64

// sharedVideoFrame keeps the cached decoder bootstrap separate from live source
// packets. Consumers may need a replayed config/key to initialize a decoder,
// but must not mistake it for a frame captured after they subscribed.
type sharedVideoFrame struct {
	data     []byte
	replayed bool
}

func (h *Hub) acquireSession(serial string) (*managedSession, *handshakeMeta, func(), error) {
	h.sessionMu.Lock()
	if ms := h.sessions[serial]; ms != nil {
		ms.mu.Lock()
		ms.refs++
		if ms.closeTimer != nil {
			ms.closeTimer.Stop()
			ms.closeTimer = nil
		}
		ms.mu.Unlock()
		h.sessionMu.Unlock()
		return ms, ms.meta, func() { h.releaseSession(serial, ms) }, nil
	}
	// 保持注册表锁直到创建完成，避免同一 serial 并发 push/start 两套 scrcpy。
	sess, meta, err := h.startSession(serial)
	if err != nil {
		h.recordEvent(debuglog.Event{Type: "session.start_failed", DeviceID: serial, Message: err.Error()})
		h.sessionMu.Unlock()
		return nil, nil, nil, err
	}
	ms := &managedSession{sess: sess, meta: meta, refs: 1, subs: make(map[chan sharedVideoFrame]struct{}), width: meta.Width, height: meta.Height}
	ms.actions = action.New(deviceActionExecutor{ms: ms, gate: h.gateFor(serial)}, 64)
	h.sessions[serial] = ms
	h.sessionMu.Unlock()
	h.recordEvent(debuglog.Event{Type: "session.started", DeviceID: serial, SessionID: meta.SessionID, Fields: map[string]interface{}{"codec": meta.Codec, "width": meta.Width, "height": meta.Height}})
	go h.readSharedSession(serial, ms)
	return ms, meta, func() { h.releaseSession(serial, ms) }, nil
}

func (h *Hub) releaseSession(serial string, ms *managedSession) {
	ms.mu.Lock()
	ms.refs--
	refs := ms.refs
	if refs == 0 {
		ms.closeTimer = time.AfterFunc(10*time.Second, func() {
			h.sessionMu.Lock()
			ms.mu.Lock()
			if ms.refs != 0 {
				ms.mu.Unlock()
				h.sessionMu.Unlock()
				return
			}
			if h.sessions[serial] == ms {
				delete(h.sessions, serial)
			}
			ms.mu.Unlock()
			h.sessionMu.Unlock()
			ms.actions.Close()
			ms.sess.close()
		})
	}
	ms.mu.Unlock()
	_ = refs
}

func (h *Hub) subscribeShared(ms *managedSession) (<-chan sharedVideoFrame, func()) {
	// 新订阅者先接收缓存的 config/key，再接收增量帧；留出足够空间，避免
	// 浏览器刚刷新、解码器尚未初始化时把关键帧挤出队列。
	ch := make(chan sharedVideoFrame, sharedVideoQueueSize)
	ms.mu.Lock()
	ms.subs[ch] = struct{}{}
	if ms.lastConfig != nil {
		ch <- sharedVideoFrame{data: append([]byte(nil), ms.lastConfig...), replayed: true}
	}
	if ms.lastKey != nil {
		ch <- sharedVideoFrame{data: append([]byte(nil), ms.lastKey...), replayed: true}
	}
	ms.mu.Unlock()
	return ch, func() {
		ms.mu.Lock()
		if _, ok := ms.subs[ch]; ok {
			delete(ms.subs, ch)
			close(ch)
		}
		ms.mu.Unlock()
	}
}

func (h *Hub) readSharedSession(serial string, ms *managedSession) {
	for {
		f, err := ms.sess.conn.ReadFrame()
		if err != nil {
			log.Printf("[ws] shared video ended serial=%s: %v", serial, err)
			h.recordEvent(debuglog.Event{Type: "session.video_end", DeviceID: serial, SessionID: ms.meta.SessionID, Message: err.Error()})
			h.sessionMu.Lock()
			if h.sessions[serial] == ms {
				delete(h.sessions, serial)
			}
			h.sessionMu.Unlock()
			ms.mu.Lock()
			for ch := range ms.subs {
				close(ch)
			}
			ms.subs = make(map[chan sharedVideoFrame]struct{})
			ms.mu.Unlock()
			ms.actions.Close()
			ms.sess.close()
			return
		}
		buf := encodeFrame(f)
		cacheAndBroadcast(ms, f, buf)
	}
}

func cacheAndBroadcast(ms *managedSession, f *scrcpy.Frame, buf []byte) {
	ms.mu.Lock()
	defer ms.mu.Unlock()
	ms.frames++
	ms.lastFrame = time.Now()
	if f.Kind == scrcpy.FrameSession {
		ms.width, ms.height = f.Width, f.Height
		// A session frame marks a new encoder configuration (usually rotation).
		// Do not replay headers from the previous stream to a late subscriber.
		ms.lastConfig = nil
		ms.lastKey = nil
	}
	if f.Kind == scrcpy.FrameConfig {
		ms.lastConfig = append([]byte(nil), buf...)
	}
	if f.Kind == scrcpy.FrameKey {
		ms.lastKey = append([]byte(nil), buf...)
	}
	for ch := range ms.subs {
		select {
		case ch <- sharedVideoFrame{data: append([]byte(nil), buf...)}:
		default:
			ms.dropped++
			select {
			case <-ch:
			default:
			}
			select {
			case ch <- sharedVideoFrame{data: append([]byte(nil), buf...)}:
			default:
			}
		}
	}
}

type sessionStats struct {
	DeviceID      string `json:"device_id"`
	SessionID     string `json:"session_id,omitempty"`
	Subscribers   int    `json:"subscribers"`
	Frames        uint64 `json:"frames"`
	DroppedFrames uint64 `json:"dropped_frames"`
	QueueLength   int    `json:"queue_length"`
	LastFrameUnix int64  `json:"last_frame_unix_ms,omitempty"`
}

func (h *Hub) stats(serial string) (sessionStats, bool) {
	h.sessionMu.Lock()
	ms := h.sessions[serial]
	h.sessionMu.Unlock()
	if ms == nil {
		return sessionStats{DeviceID: serial}, false
	}
	ms.mu.Lock()
	defer ms.mu.Unlock()
	s := sessionStats{DeviceID: serial, SessionID: ms.meta.SessionID, Subscribers: len(ms.subs), Frames: ms.frames, DroppedFrames: ms.dropped, QueueLength: ms.actions.Len()}
	if !ms.lastFrame.IsZero() {
		s.LastFrameUnix = ms.lastFrame.UnixMilli()
	}
	return s, true
}

func encodeFrame(f *scrcpy.Frame) []byte {
	buf := make([]byte, 9+len(f.Payload))
	buf[0] = byte(f.Kind)
	binary.BigEndian.PutUint64(buf[1:9], f.PTS)
	if f.Kind == scrcpy.FrameSession {
		buf = make([]byte, 17)
		buf[0] = byte(f.Kind)
		binary.BigEndian.PutUint64(buf[1:9], f.PTS)
		binary.BigEndian.PutUint32(buf[9:13], f.Width)
		binary.BigEndian.PutUint32(buf[13:17], f.Height)
	} else {
		copy(buf[9:], f.Payload)
	}
	return buf
}

func (h *Hub) pumpSharedVideo(c *websocket.Conn, ms *managedSession, done <-chan struct{}) {
	ch, cancel := h.subscribeShared(ms)
	defer cancel()
	heartbeat := time.NewTicker(2 * time.Second)
	defer heartbeat.Stop()
	for {
		select {
		case <-done:
			return
		case <-heartbeat.C:
			c.SetWriteDeadline(timeNow())
			if err := c.WriteJSON(map[string]interface{}{"type": "stream.heartbeat", "session_id": ms.meta.SessionID, "timestamp": time.Now().UnixMilli()}); err != nil {
				_ = c.Close()
				return
			}
		case frame, ok := <-ch:
			if !ok {
				// End the browser socket as well as the video pump. Otherwise the
				// control reader can remain blocked forever and the UI cannot retry.
				_ = c.WriteJSON(map[string]interface{}{"type": "disconnected"})
				_ = c.Close()
				return
			}
			c.SetWriteDeadline(timeNow())
			if err := c.WriteMessage(websocket.BinaryMessage, frame.data); err != nil {
				_ = c.Close()
				return
			}
		}
	}
}

func timeNow() time.Time { return time.Now().Add(3 * time.Second) }

type deviceActionExecutor struct {
	ms   *managedSession
	gate *devicegate.Gate
}

func (e deviceActionExecutor) Execute(ctx context.Context, r action.Request) error {
	if e.gate != nil {
		return e.gate.Run(ctx, false, func(ctx context.Context) error { return e.execute(ctx, r) })
	}
	return e.execute(ctx, r)
}

func (e deviceActionExecutor) execute(ctx context.Context, r action.Request) error {
	select {
	case <-ctx.Done():
		return ctx.Err()
	default:
	}
	e.ms.controlMu.Lock()
	defer e.ms.controlMu.Unlock()
	if r.Action == "raw" {
		return e.ms.sess.conn.WriteControl(r.Raw)
	}
	e.ms.mu.Lock()
	width, height := e.ms.width, e.ms.height
	e.ms.mu.Unlock()
	if r.Action == "tap" {
		x, y := int32(r.X*float64(width)), int32(r.Y*float64(height))
		w, h := uint16(width), uint16(height)
		if err := e.ms.sess.conn.WriteControl(scrcpy.TouchEvent(scrcpy.ActionDown, scrcpy.PointerIDMouse, x, y, w, h, 1, scrcpy.ButtonPrimary, scrcpy.ButtonPrimary)); err != nil {
			return err
		}
		return e.ms.sess.conn.WriteControl(scrcpy.TouchEvent(scrcpy.ActionUp, scrcpy.PointerIDMouse, x, y, w, h, 0, 0, 0))
	}
	if r.Action == "swipe" {
		x1, y1 := int32(r.X*float64(width)), int32(r.Y*float64(height))
		x2, y2 := int32(r.X2*float64(width)), int32(r.Y2*float64(height))
		w, h := uint16(width), uint16(height)
		if err := e.ms.sess.conn.WriteControl(scrcpy.TouchEvent(scrcpy.ActionDown, scrcpy.PointerIDMouse, x1, y1, w, h, 1, scrcpy.ButtonPrimary, scrcpy.ButtonPrimary)); err != nil {
			return err
		}
		if err := e.ms.sess.conn.WriteControl(scrcpy.TouchEvent(scrcpy.ActionMove, scrcpy.PointerIDMouse, x2, y2, w, h, 1, scrcpy.ButtonPrimary, scrcpy.ButtonPrimary)); err != nil {
			return err
		}
		return e.ms.sess.conn.WriteControl(scrcpy.TouchEvent(scrcpy.ActionUp, scrcpy.PointerIDMouse, x2, y2, w, h, 0, 0, 0))
	}
	if r.Action == "key" {
		if err := e.ms.sess.conn.WriteControl(scrcpy.KeyCodeEvent(scrcpy.KeyActionDown, r.Keycode, 0, r.MetaState)); err != nil {
			return err
		}
		return e.ms.sess.conn.WriteControl(scrcpy.KeyCodeEvent(scrcpy.KeyActionUp, r.Keycode, 0, r.MetaState))
	}
	if r.Action == "text" {
		return e.ms.sess.conn.WriteControl(scrcpy.TextEvent(r.Text))
	}
	return errors.New("action_not_implemented")
}
