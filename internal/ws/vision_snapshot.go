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
	ms.visionSnapshot = append(ms.visionSnapshot, append([]byte(nil), data...))
	ms.visionBytes += len(data)
}

func (h *Hub) subscribeVisionSnapshot(ms *managedSession) (<-chan sharedVideoFrame, func()) {
	ms.mu.Lock()
	ch := make(chan sharedVideoFrame, sharedVideoQueueSize+len(ms.visionSnapshot)+1)
	ms.subs[ch] = struct{}{}
	if ms.lastConfig != nil {
		ch <- sharedVideoFrame{data: append([]byte(nil), ms.lastConfig...), replayed: true}
	}
	if len(ms.visionSnapshot) == 0 && ms.lastKey != nil {
		// Preserve decoder bootstrap when no complete snapshot is available.
		// This packet is deliberately not marked as the current image.
		ch <- sharedVideoFrame{data: append([]byte(nil), ms.lastKey...), replayed: true}
	}
	for index, data := range ms.visionSnapshot {
		ch <- sharedVideoFrame{data: append([]byte(nil), data...), replayed: true, snapshotComplete: index == len(ms.visionSnapshot)-1}
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
