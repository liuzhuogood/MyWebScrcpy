package ws

import (
	"encoding/binary"
	"testing"

	"mywebscrcpy/internal/scrcpy"
)

func TestEncodeFrameUsesStableEnvelope(t *testing.T) {
	f := &scrcpy.Frame{Kind: scrcpy.FrameKey, PTS: 42, Payload: []byte{1, 2, 3}}
	b := encodeFrame(f)
	if len(b) != 12 || b[0] != byte(scrcpy.FrameKey) || binary.BigEndian.Uint64(b[1:9]) != 42 {
		t.Fatalf("unexpected key envelope: %v", b)
	}
	if string(b[9:]) != "\x01\x02\x03" {
		t.Fatalf("unexpected payload: %v", b[9:])
	}

	s := encodeFrame(&scrcpy.Frame{Kind: scrcpy.FrameSession, Width: 448, Height: 1024})
	if len(s) != 17 || binary.BigEndian.Uint32(s[9:13]) != 448 || binary.BigEndian.Uint32(s[13:17]) != 1024 {
		t.Fatalf("unexpected session envelope: %v", s)
	}
}

func TestSessionFrameInvalidatesCachedHeaders(t *testing.T) {
	ms := &managedSession{subs: make(map[chan sharedVideoFrame]struct{})}
	cacheAndBroadcast(ms, &scrcpy.Frame{Kind: scrcpy.FrameConfig, Payload: []byte("config")}, []byte("config"))
	cacheAndBroadcast(ms, &scrcpy.Frame{Kind: scrcpy.FrameKey, Payload: []byte("key")}, []byte("key"))
	cacheAndBroadcast(ms, &scrcpy.Frame{Kind: scrcpy.FrameSession, Width: 1024, Height: 448}, encodeFrame(&scrcpy.Frame{Kind: scrcpy.FrameSession, Width: 1024, Height: 448}))
	ms.mu.Lock()
	defer ms.mu.Unlock()
	if ms.lastConfig != nil || ms.lastKey != nil || ms.width != 1024 || ms.height != 448 {
		t.Fatalf("stale headers or dimensions remain: config=%q key=%q size=%dx%d", ms.lastConfig, ms.lastKey, ms.width, ms.height)
	}
}

func TestSubscribeSharedReplaysLatestHeaders(t *testing.T) {
	ms := &managedSession{subs: make(map[chan sharedVideoFrame]struct{}), lastConfig: []byte("config"), lastKey: []byte("key")}
	ch, cancel := (&Hub{}).subscribeShared(ms)
	defer cancel()
	if frame := <-ch; string(frame.data) != "config" || !frame.replayed {
		t.Fatalf("first cached frame = %#v", frame)
	}
	if frame := <-ch; string(frame.data) != "key" || !frame.replayed {
		t.Fatalf("second cached frame = %#v", frame)
	}
}

func TestSubscribeSharedKeepsHeadersWhileFramesArrive(t *testing.T) {
	ms := &managedSession{subs: make(map[chan sharedVideoFrame]struct{}), lastConfig: []byte("config"), lastKey: []byte("key")}
	ch, cancel := (&Hub{}).subscribeShared(ms)
	defer cancel()
	for i := byte(0); i < 20; i++ {
		f := &scrcpy.Frame{Kind: scrcpy.FrameDelta, Payload: []byte{i}}
		cacheAndBroadcast(ms, f, encodeFrame(f))
	}
	if frame := <-ch; string(frame.data) != "config" || !frame.replayed {
		t.Fatalf("config frame was displaced: %#v", frame)
	}
	if frame := <-ch; string(frame.data) != "key" || !frame.replayed {
		t.Fatalf("key frame was displaced: %#v", frame)
	}
}

func TestCacheAndBroadcastDropsOldFramesForSlowSubscriber(t *testing.T) {
	ms := &managedSession{subs: make(map[chan sharedVideoFrame]struct{})}
	ch, cancel := (&Hub{}).subscribeShared(ms)
	defer cancel()
	for i := byte(0); i < 100; i++ {
		f := &scrcpy.Frame{Kind: scrcpy.FrameDelta, Payload: []byte{i}}
		cacheAndBroadcast(ms, f, encodeFrame(f))
	}
	ms.mu.Lock()
	dropped := ms.dropped
	ms.mu.Unlock()
	if dropped == 0 {
		t.Fatal("slow subscriber did not record dropped frames")
	}
	var last sharedVideoFrame
	for {
		select {
		case last = <-ch:
		default:
			if len(last.data) == 0 || last.data[9] != 99 || last.replayed {
				t.Fatalf("slow subscriber did not retain latest frame: %v", last)
			}
			return
		}
	}
}

func TestNewSessionIDIsNonEmptyAndUnique(t *testing.T) {
	a, b := newSessionID(), newSessionID()
	if a == "" || b == "" || a == b {
		t.Fatalf("invalid session ids: %q %q", a, b)
	}
}
