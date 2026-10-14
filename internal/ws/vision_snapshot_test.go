package ws

import (
	"testing"

	"mywebscrcpy/internal/scrcpy"
)

func TestVisionSnapshotIncludesDeltasToCurrentStaticImage(t *testing.T) {
	ms := &managedSession{subs: make(map[chan sharedVideoFrame]struct{})}
	for _, frame := range []*scrcpy.Frame{
		{Kind: scrcpy.FrameConfig, Payload: []byte("config")},
		{Kind: scrcpy.FrameKey, Payload: []byte("key")},
		{Kind: scrcpy.FrameDelta, Payload: []byte("delta-1")},
		{Kind: scrcpy.FrameDelta, Payload: []byte("delta-2")},
	} {
		cacheAndBroadcast(ms, frame, encodeFrame(frame))
	}
	ch, cancel := (&Hub{}).subscribeVisionSnapshot(ms)
	defer cancel()
	for i, want := range []string{"config", "key", "delta-1", "delta-2"} {
		frame := <-ch
		if string(frame.data[9:]) != want || !frame.replayed || frame.snapshotComplete != (i == 3) {
			t.Fatalf("snapshot packet %d = %+v, want %s", i, frame, want)
		}
	}
	// The existing browser subscription still receives only decoder headers.
	browser, closeBrowser := (&Hub{}).subscribeShared(ms)
	defer closeBrowser()
	if len(browser) != 2 {
		t.Fatal("browser bootstrap changed")
	}
	newFrame := &scrcpy.Frame{Kind: scrcpy.FrameDelta, Payload: []byte("live")}
	cacheAndBroadcast(ms, newFrame, encodeFrame(newFrame))
	if frame := <-ch; frame.replayed || frame.snapshotComplete || string(frame.data[9:]) != "live" {
		t.Fatalf("new live frame = %+v", frame)
	}
}

func TestVisionSnapshotInvalidatesOnRotationConfigurationAndOverflow(t *testing.T) {
	for _, kind := range []scrcpy.FrameKind{scrcpy.FrameSession, scrcpy.FrameConfig} {
		ms := &managedSession{subs: make(map[chan sharedVideoFrame]struct{})}
		key := &scrcpy.Frame{Kind: scrcpy.FrameKey, Payload: []byte("key")}
		cacheAndBroadcast(ms, key, encodeFrame(key))
		reset := &scrcpy.Frame{Kind: kind}
		cacheAndBroadcast(ms, reset, encodeFrame(reset))
		if len(ms.visionSnapshot) != 0 || ms.visionBytes != 0 {
			t.Fatal("reset retained old snapshot")
		}
	}
	ms := &managedSession{subs: make(map[chan sharedVideoFrame]struct{})}
	key := &scrcpy.Frame{Kind: scrcpy.FrameKey, Payload: []byte("key")}
	cacheAndBroadcast(ms, key, encodeFrame(key))
	delta := &scrcpy.Frame{Kind: scrcpy.FrameDelta, Payload: []byte("delta")}
	for i := 0; i < maxVisionSnapshotPackets; i++ {
		cacheAndBroadcast(ms, delta, encodeFrame(delta))
	}
	if len(ms.visionSnapshot) != 0 || ms.visionBytes != 0 {
		t.Fatal("overflow retained an incomplete decoder chain")
	}
	cacheAndBroadcast(ms, delta, encodeFrame(delta))
	if len(ms.visionSnapshot) != 0 {
		t.Fatal("delta started a cache without a key frame")
	}
	cacheAndBroadcast(ms, key, encodeFrame(key))
	if len(ms.visionSnapshot) != 1 {
		t.Fatal("new key did not rebuild snapshot")
	}
	large := &scrcpy.Frame{Kind: scrcpy.FrameDelta, Payload: make([]byte, maxVisionSnapshotBytes)}
	cacheAndBroadcast(ms, large, encodeFrame(large))
	if ms.visionBytes != 0 || len(ms.visionSnapshot) != 0 {
		t.Fatal("byte limit did not discard snapshot")
	}
}
