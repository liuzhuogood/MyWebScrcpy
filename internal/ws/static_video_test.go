package ws

import (
	"mywebscrcpy/internal/scrcpy"
	"testing"
)

func TestBrowserSnapshotReachesLatestStaticFrame(t *testing.T) {
	ms := &managedSession{subs: make(map[chan sharedVideoFrame]struct{})}
	for i, kind := range []scrcpy.FrameKind{scrcpy.FrameConfig, scrcpy.FrameKey, scrcpy.FrameDelta, scrcpy.FrameDelta} {
		frame := &scrcpy.Frame{Kind: kind, PTS: uint64(i * 1000000), Payload: []byte{byte(i)}}
		cacheAndBroadcast(ms, frame, encodeFrame(frame))
	}
	ch, cancel := (&Hub{}).subscribeBrowserVideo(ms)
	defer cancel()
	if len(ch) != 4 {
		t.Fatalf("late browser received %d packets; need config plus complete key/delta chain", len(ch))
	}
	for i := 0; i < 4; i++ {
		frame := <-ch
		if frame.data[9] != byte(i) {
			t.Fatalf("packet %d = %v", i, frame.data)
		}
	}
}
func TestBrowserDoesNotPresentOldKeyAfterSnapshotOverflow(t *testing.T) {
	ms := &managedSession{subs: make(map[chan sharedVideoFrame]struct{})}
	config := &scrcpy.Frame{Kind: scrcpy.FrameConfig, Payload: []byte{0}}
	key := &scrcpy.Frame{Kind: scrcpy.FrameKey, Payload: []byte{1}}
	cacheAndBroadcast(ms, config, encodeFrame(config))
	cacheAndBroadcast(ms, key, encodeFrame(key))
	for i := 0; i < maxVisionSnapshotPackets; i++ {
		frame := &scrcpy.Frame{Kind: scrcpy.FrameDelta, Payload: []byte{2}}
		cacheAndBroadcast(ms, frame, encodeFrame(frame))
	}
	ch, cancel := (&Hub{}).subscribeBrowserVideo(ms)
	defer cancel()
	if len(ch) != 1 {
		t.Fatalf("incomplete chain must wait for a new key, got %d packets", len(ch))
	}
	if frame := <-ch; frame.data[0] != byte(scrcpy.FrameConfig) {
		t.Fatal("expected only decoder config")
	}
	// Keep the existing Vision bootstrap fallback contract intact.
	vision, stop := (&Hub{}).subscribeVisionSnapshot(ms)
	defer stop()
	if len(vision) != 2 {
		t.Fatal("Vision fallback changed")
	}
}

func TestReplayBrowserSnapshotIncludesPausedLatestImage(t *testing.T) {
	ms := &managedSession{subs: make(map[chan sharedVideoFrame]struct{})}
	for i, kind := range []scrcpy.FrameKind{scrcpy.FrameConfig, scrcpy.FrameKey, scrcpy.FrameDelta} {
		frame := &scrcpy.Frame{Kind: kind, PTS: uint64(i), Payload: []byte{byte(i)}}
		broadcastReplayFrame(ms, frame, encodeFrame(frame))
	}
	ch, cancel := (&Hub{}).subscribeBrowserVideo(ms)
	defer cancel()
	if len(ch) != 3 {
		t.Fatalf("paused replay bootstrap has %d packets, want config/key/latest delta", len(ch))
	}
}
