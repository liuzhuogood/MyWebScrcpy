package ws

import "mywebscrcpy/internal/scrcpy"

// Keep a bounded complete decoder chain, not an old key frame presented as the
// current image. On overflow wait for the next key frame to rebuild the cache.
const maxVisionSnapshotBytes = 16 << 20
const maxVisionSnapshotPackets = 512

// Caller holds ms.mu.
func cacheVisionSnapshot(ms *managedSession, frame *scrcpy.Frame, data []byte) {
	switch frame.Kind {
	case scrcpy.FrameConfig, scrcpy.FrameSession:
		ms.visionSnapshot, ms.visionBytes = nil, 0
		return
	case scrcpy.FrameKey:
		ms.visionSnapshot, ms.visionBytes = nil, 0
	case scrcpy.FrameDelta:
		if len(ms.visionSnapshot) == 0 {
			return
		}
	default:
		return
	}
	if ms.visionBytes+len(data) > maxVisionSnapshotBytes || len(ms.visionSnapshot) >= maxVisionSnapshotPackets {
		ms.visionSnapshot, ms.visionBytes = nil, 0
		return
	}
	// data is the immutable packet owned by the live/replay broadcaster.
	ms.visionSnapshot = append(ms.visionSnapshot, data)
	ms.visionBytes += len(data)
}

func (h *Hub) subscribeVisionSnapshot(ms *managedSession) (<-chan sharedVideoFrame, func()) {
	return h.subscribeSnapshot(ms, true)
}

// Browser reconnects need the current complete image, not just an old IDR.
// Retain Vision's explicit non-current bootstrap fallback for its consumers.
func (h *Hub) subscribeBrowserVideo(ms *managedSession) (<-chan sharedVideoFrame, func()) {
	return h.subscribeSnapshot(ms, false)
}

// Register the snapshot and capture its dimensions in one critical section.
// Any rotation after this point is queued, even while the handshake is written.
func (h *Hub) subscribeBrowserVideoWithMeta(ms *managedSession) (handshakeMeta, <-chan sharedVideoFrame, func()) {
	ms.mu.Lock()
	defer ms.mu.Unlock()
	meta := currentHandshakeMetaLocked(ms)
	frames, cancel := h.subscribeSnapshotLocked(ms, false)
	return meta, frames, cancel
}

// Keep both Vision bootstrap modes atomic with their announced dimensions.
func (h *Hub) subscribeVisionWithMeta(ms *managedSession, currentFrame bool) (handshakeMeta, <-chan sharedVideoFrame, func()) {
	ms.mu.Lock()
	defer ms.mu.Unlock()
	meta := currentHandshakeMetaLocked(ms)
	var frames <-chan sharedVideoFrame
	var cancel func()
	if currentFrame {
		frames, cancel = h.subscribeSnapshotLocked(ms, true)
	} else {
		frames, cancel = h.subscribeSharedLocked(ms)
	}
	return meta, frames, cancel
}

func (h *Hub) subscribeSnapshot(ms *managedSession, allowOldBootstrap bool) (<-chan sharedVideoFrame, func()) {
	ms.mu.Lock()
	defer ms.mu.Unlock()
	return h.subscribeSnapshotLocked(ms, allowOldBootstrap)
}

// Caller holds ms.mu. The cancellation callback acquires it independently.
func (h *Hub) subscribeSnapshotLocked(ms *managedSession, allowOldBootstrap bool) (<-chan sharedVideoFrame, func()) {
	ch := make(chan sharedVideoFrame, sharedVideoQueueSize+len(ms.visionSnapshot)+1)
	ms.subs[ch] = struct{}{}
	if !allowOldBootstrap {
		if ms.browserSubs == nil {
			ms.browserSubs = make(map[chan sharedVideoFrame]struct{})
		}
		ms.browserSubs[ch] = struct{}{}
	}
	if ms.lastConfig != nil {
		ch <- sharedVideoFrame{data: ms.lastConfig, replayed: true}
	}
	if allowOldBootstrap && len(ms.visionSnapshot) == 0 && ms.lastKey != nil {
		// Preserve decoder bootstrap when no complete snapshot is available.
		// This packet is deliberately not marked as the current image.
		ch <- sharedVideoFrame{data: ms.lastKey, replayed: true}
	}
	for index, data := range ms.visionSnapshot {
		ch <- sharedVideoFrame{data: data, replayed: true, snapshotComplete: index == len(ms.visionSnapshot)-1}
	}
	return ch, func() {
		ms.mu.Lock()
		if _, ok := ms.subs[ch]; ok {
			delete(ms.subs, ch)
			close(ch)
		}
		delete(ms.browserSubs, ch)
		ms.mu.Unlock()
	}
}
