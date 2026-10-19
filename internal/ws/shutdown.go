package ws

import (
	"context"
	"log"
	"time"
)

// Close is used by the desktop host after HTTP shutdown. Existing standalone
// service lifecycle is unchanged. Only sessions in this Hub are stopped.
func (h *Hub) Close() {
	h.sessionMu.Lock()
	h.closed = true
	serials := make([]string, 0, len(h.sessions))
	for serial := range h.sessions {
		serials = append(serials, serial)
	}
	calls := make([]*inflightSession, 0, len(h.inflight))
	for _, call := range h.inflight {
		calls = append(calls, call)
	}
	h.sessionMu.Unlock()
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	// Finish active MP4 files before closing their shared video streams.
	if h.recordings != nil {
		h.recordings.mu.Lock()
		active := make([]*recording, 0, len(h.recordings.entries))
		for _, rec := range h.recordings.entries {
			active = append(active, rec)
		}
		h.recordings.mu.Unlock()
		for _, rec := range active {
			_, _ = h.recordings.stopRecording(rec.id, rec.serial)
		}
		for _, rec := range active {
			for {
				rec.mu.Lock()
				status := rec.status
				rec.mu.Unlock()
				if status != recordingActive && status != recordingStopping {
					break
				}
				select {
				case <-ctx.Done():
					log.Printf("[Desktop] 录制收尾超时 id=%s", rec.id)
					goto nextRecording
				case <-time.After(20 * time.Millisecond):
				}
			}
		nextRecording:
		}
	}
	if h.replays != nil {
		h.replays.mu.Lock()
		replays := make([]*replay, 0, len(h.replays.bySerial))
		for _, rp := range h.replays.bySerial {
			replays = append(replays, rp)
		}
		h.replays.mu.Unlock()
		for _, rp := range replays {
			_, _ = h.replays.stop(rp.serial)
		}
	}
	for _, serial := range serials {
		h.closeLiveSession(serial)
	}
	h.sessionMu.Lock()
	sk := h.streamKeeper
	h.sessionMu.Unlock()
	if sk != nil {
		sk.Close()
	}
	for _, call := range calls {
		select {
		case <-call.done:
		case <-ctx.Done():
			log.Print("[Desktop] 投屏启动退出等待超时")
			return
		}
	}
}
