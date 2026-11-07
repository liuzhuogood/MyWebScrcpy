package ws

import (
	"encoding/binary"
	"time"

	debuglog "mywebscrcpy/internal/debug"
	"mywebscrcpy/internal/scrcpy"
)

// Caller holds ms.mu. Only browser subscriptions can restart from a newer GOP;
// recording and Vision retain their own delivery contracts.
func rebuildBrowserVideoQueue(ms *managedSession, ch chan sharedVideoFrame) {
	var rotation, config *sharedVideoFrame
	drained := 0
	for len(ch) > 0 {
		select {
		case frame := <-ch:
			drained++
			if len(frame.data) > 0 {
				f := frame
				switch scrcpy.FrameKind(frame.data[0]) {
				case scrcpy.FrameSession:
					rotation, config = &f, nil
				case scrcpy.FrameConfig:
					config = &f
				}
			}
		default:
		}
	}
	needed := len(ms.visionSnapshot)
	if config != nil {
		needed++
	}
	if rotation != nil {
		needed++
	}
	if len(ms.visionSnapshot) == 0 || needed > cap(ch) {
		// A replacement subscription can size its queue for the complete current
		// GOP, or wait for the next key if the bounded cache is unavailable. Never
		// keep an arbitrary delta suffix: it lacks decoder references.
		delete(ms.subs, ch)
		delete(ms.browserSubs, ch)
		close(ch)
		ms.dropped += uint64(drained + 1)
		recordBrowserVideoEvent(ms, "browser.video_reconnect", drained+1, needed)
		return
	}
	if rotation != nil {
		ch <- *rotation
	}
	// Only configuration still pending in this connection's queue is replayed.
	// Re-sending an already delivered config would reset the browser's PTS floor
	// and allow an old key frame in this same epoch to flash on screen.
	if config != nil {
		ch <- *config
	}
	for _, data := range ms.visionSnapshot {
		ch <- sharedVideoFrame{data: data}
	}
	ms.dropped += uint64(drained)
	recordBrowserVideoEvent(ms, "browser.video_catchup", drained, len(ms.visionSnapshot))
}

// newestBrowserVideoBatch keeps the latest independently decodable chain in a
// queued burst. Configuration and rotation before that key frame must survive.
// With no newer key, every delta remains necessary and is delivered unchanged.
func newestBrowserVideoBatch(frames []sharedVideoFrame) []sharedVideoFrame {
	key := -1
	for i, frame := range frames {
		if len(frame.data) > 0 && frame.data[0] == byte(scrcpy.FrameKey) {
			key = i
		}
	}
	if key <= 0 {
		return frames
	}
	config, rotation := -1, -1
	for i := 0; i < key; i++ {
		if len(frames[i].data) == 0 {
			continue
		}
		switch scrcpy.FrameKind(frames[i].data[0]) {
		case scrcpy.FrameSession:
			rotation = i
			config = -1
		case scrcpy.FrameConfig:
			config = i
		}
	}
	prefix := 0
	if rotation >= 0 {
		frames[prefix] = frames[rotation]
		prefix++
	}
	if config >= 0 {
		frames[prefix] = frames[config]
		prefix++
	}
	end := prefix + copy(frames[prefix:], frames[key:])
	clear(frames[end:])
	return frames[:end]
}

func (h *Hub) consumeSharedVideoFrame(serial string, ms *managedSession, f *scrcpy.Frame, capturedAt time.Time) {
	h.frameConsumerMu.RLock()
	consumer := h.frameConsumer
	h.frameConsumerMu.RUnlock()
	if consumer == nil {
		return
	}
	ms.mu.Lock()
	frameID, width, height := ms.frames, ms.width, ms.height
	ms.mu.Unlock()
	consumer.ConsumeVideoFrame(VideoFrame{DeviceID: serial, SessionID: ms.meta.SessionID, FrameID: frameID, PTS: f.PTS, CapturedAt: capturedAt, Kind: f.Kind, Width: width, Height: height, Payload: append([]byte(nil), f.Payload...)})
}

// The existing bounded event ring avoids synchronous process logging on the
// shared video reader's hot path. Caller holds ms.mu; no network I/O is involved.
func recordBrowserVideoEvent(ms *managedSession, kind string, discarded, retained int) {
	if ms.h == nil || ms.meta == nil {
		return
	}
	ms.h.recordEvent(debuglog.Event{Type: kind, DeviceID: ms.meta.Serial, SessionID: ms.meta.SessionID, Fields: map[string]interface{}{"discarded_packets": discarded, "retained_packets": retained}})
}

// Initial metadata remains immutable because other session users retain its
// pointer. Reconnects must announce the current orientation, without injecting
// a synthetic session frame that would reset the browser's presentation floor.
func currentHandshakeMeta(ms *managedSession) handshakeMeta {
	ms.mu.Lock()
	defer ms.mu.Unlock()
	return currentHandshakeMetaLocked(ms)
}

func currentHandshakeMetaLocked(ms *managedSession) handshakeMeta {
	meta := *ms.meta
	meta.Width, meta.Height = ms.width, ms.height
	return meta
}

// Record at most once per second for each connection; ordinary frames add no
// event allocation. Source PTS permits correlation with input and receive logs.
func (h *Hub) recordBrowserVideoWrite(ms *managedSession, packet sharedVideoFrame, elapsed time.Duration, queued int, finished, last time.Time) time.Time {
	if elapsed <= 100*time.Millisecond || (!last.IsZero() && finished.Sub(last) < time.Second) {
		return last
	}
	h.recordEvent(debuglog.Event{At: finished, Type: "browser.video_slow", DeviceID: ms.meta.Serial, SessionID: ms.meta.SessionID, Fields: map[string]interface{}{
		"write_ms": elapsed.Milliseconds(), "source_pts_us": binary.BigEndian.Uint64(packet.data[1:9]), "bytes": len(packet.data), "queued_packets": queued,
	}})
	return finished
}
