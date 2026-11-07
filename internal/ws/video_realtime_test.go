package ws

import (
	"encoding/binary"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/gorilla/websocket"
	"mywebscrcpy/internal/action"
	"mywebscrcpy/internal/scrcpy"
)

func publishVideoTest(ms *managedSession, kind scrcpy.FrameKind, payload string) {
	f := &scrcpy.Frame{Kind: kind, Payload: []byte(payload), Width: 448, Height: 1024}
	cacheAndBroadcast(ms, f, encodeFrame(f))
}

func drainVideoTest(ch <-chan sharedVideoFrame) []sharedVideoFrame {
	var frames []sharedVideoFrame
	for {
		select {
		case f, ok := <-ch:
			if !ok {
				return frames
			}
			frames = append(frames, f)
		default:
			return frames
		}
	}
}

func TestBrowserOverflowRetainsCompleteLatestStaticImage(t *testing.T) {
	ms := &managedSession{subs: make(map[chan sharedVideoFrame]struct{})}
	publishVideoTest(ms, scrcpy.FrameConfig, "config")
	publishVideoTest(ms, scrcpy.FrameKey, "old-key")
	ch, cancel := (&Hub{}).subscribeBrowserVideo(ms)
	defer cancel()
	for i := 0; i < 60; i++ {
		publishVideoTest(ms, scrcpy.FrameDelta, "old-delta")
	}
	publishVideoTest(ms, scrcpy.FrameKey, "new-key")
	for _, p := range []string{"new-1", "new-2", "new-3", "final-static"} {
		publishVideoTest(ms, scrcpy.FrameDelta, p)
	}
	frames := drainVideoTest(ch)
	want := []string{"config", "new-key", "new-1", "new-2", "new-3", "final-static"}
	if len(frames) != len(want) {
		t.Fatalf("overflow queued %d packets, want complete latest chain %v", len(frames), want)
	}
	for i, f := range frames {
		if string(f.data[9:]) != want[i] {
			t.Fatalf("packet %d=%q, want %q", i, f.data[9:], want[i])
		}
	}
}

func TestBrowserOverflowPreservesPendingRotation(t *testing.T) {
	ms := &managedSession{subs: make(map[chan sharedVideoFrame]struct{})}
	publishVideoTest(ms, scrcpy.FrameConfig, "old-config")
	publishVideoTest(ms, scrcpy.FrameKey, "old-key")
	ch, cancel := (&Hub{}).subscribeBrowserVideo(ms)
	defer cancel()
	for i := 0; i < 60; i++ {
		publishVideoTest(ms, scrcpy.FrameDelta, "old-delta")
	}
	publishVideoTest(ms, scrcpy.FrameSession, "")
	publishVideoTest(ms, scrcpy.FrameConfig, "new-config")
	publishVideoTest(ms, scrcpy.FrameKey, "new-key")
	publishVideoTest(ms, scrcpy.FrameDelta, "new-1")
	publishVideoTest(ms, scrcpy.FrameDelta, "final-static")
	frames := drainVideoTest(ch)
	if len(frames) != 5 || frames[0].data[0] != byte(scrcpy.FrameSession) {
		t.Fatalf("rotation not retained: %+v", frames)
	}
	for i, p := range []string{"new-config", "new-key", "new-1", "final-static"} {
		if string(frames[i+1].data[9:]) != p {
			t.Fatalf("packet %d=%q want %q", i+1, frames[i+1].data[9:], p)
		}
	}
}

func TestBrowserOverflowReconnectsInsteadOfBreakingLongDependencyChain(t *testing.T) {
	ms := &managedSession{subs: make(map[chan sharedVideoFrame]struct{})}
	publishVideoTest(ms, scrcpy.FrameConfig, "config")
	publishVideoTest(ms, scrcpy.FrameKey, "key")
	ch, cancel := (&Hub{}).subscribeBrowserVideo(ms)
	defer cancel()
	for i := 0; i < 65; i++ {
		publishVideoTest(ms, scrcpy.FrameDelta, "delta")
	}
	select {
	case _, ok := <-ch:
		if ok {
			t.Fatal("oversized dependency chain must reconnect, not return a broken delta prefix")
		}
	default:
		t.Fatal("overflowed browser was left waiting")
	}
	// The replacement subscription can still reconstruct the exact static image.
	next, stop := (&Hub{}).subscribeBrowserVideo(ms)
	defer stop()
	frames := drainVideoTest(next)
	if len(frames) != 67 {
		t.Fatalf("reconnect received %d packets, want config + key + all 65 deltas", len(frames))
	}
}

func TestVideoFanoutOwnsInputOnce(t *testing.T) {
	ms := &managedSession{subs: make(map[chan sharedVideoFrame]struct{})}
	a, stopA := (&Hub{}).subscribeShared(ms)
	defer stopA()
	b, stopB := (&Hub{}).subscribeShared(ms)
	defer stopB()
	f := &scrcpy.Frame{Kind: scrcpy.FrameKey, Payload: []byte("key")}
	buf := encodeFrame(f)
	cacheAndBroadcast(ms, f, buf)
	buf[9] = 'x'
	for _, ch := range []<-chan sharedVideoFrame{a, b} {
		if got := string((<-ch).data[9:]); got != "key" {
			t.Fatalf("caller mutated owned packet: %q", got)
		}
	}
	if got := string(ms.visionSnapshot[0][9:]); got != "key" {
		t.Fatalf("caller mutated cache: %q", got)
	}
}

func TestBrowserBurstSkipsOnlyBeforeLatestIndependentChain(t *testing.T) {
	tests := []struct {
		name  string
		kinds []scrcpy.FrameKind
		want  []int
	}{
		{"same GOP", []scrcpy.FrameKind{scrcpy.FrameKey, scrcpy.FrameDelta, scrcpy.FrameDelta}, []int{0, 1, 2}},
		{"deltas without new key", []scrcpy.FrameKind{scrcpy.FrameDelta, scrcpy.FrameDelta}, []int{0, 1}},
		{"new GOP", []scrcpy.FrameKind{scrcpy.FrameConfig, scrcpy.FrameKey, scrcpy.FrameDelta, scrcpy.FrameKey, scrcpy.FrameDelta}, []int{0, 3, 4}},
		{"rotated GOP", []scrcpy.FrameKind{scrcpy.FrameConfig, scrcpy.FrameKey, scrcpy.FrameDelta, scrcpy.FrameSession, scrcpy.FrameConfig, scrcpy.FrameKey, scrcpy.FrameDelta}, []int{3, 4, 5, 6}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var batch []sharedVideoFrame
			for i, kind := range tt.kinds {
				batch = append(batch, sharedVideoFrame{data: []byte{byte(kind), byte(i)}})
			}
			got := newestBrowserVideoBatch(batch)
			if len(got) != len(tt.want) {
				t.Fatalf("got %d packets, want %d", len(got), len(tt.want))
			}
			for i, want := range tt.want {
				if int(got[i].data[1]) != want {
					t.Fatalf("packet %d=%d want %d", i, got[i].data[1], want)
				}
			}
		})
	}
}

func BenchmarkVideoFanout(b *testing.B) {
	for _, count := range []int{1, 2, 8} {
		b.Run(fmt.Sprintf("subscribers-%d", count), func(b *testing.B) {
			ms := &managedSession{subs: make(map[chan sharedVideoFrame]struct{})}
			var channels []<-chan sharedVideoFrame
			for i := 0; i < count; i++ {
				ch, cancel := (&Hub{}).subscribeShared(ms)
				defer cancel()
				channels = append(channels, ch)
			}
			f := &scrcpy.Frame{Kind: scrcpy.FrameKey, Payload: make([]byte, 128*1024)}
			encoded := encodeFrame(f)
			b.SetBytes(int64(len(encoded)))
			b.ReportAllocs()
			b.ResetTimer()
			for i := 0; i < b.N; i++ {
				cacheAndBroadcast(ms, f, encoded)
				for _, ch := range channels {
					<-ch
				}
			}
		})
	}
}

func TestBrowserCatchupDoesNotResetAlreadyConfiguredDecoder(t *testing.T) {
	ms := &managedSession{subs: make(map[chan sharedVideoFrame]struct{})}
	publishVideoTest(ms, scrcpy.FrameConfig, "config")
	publishVideoTest(ms, scrcpy.FrameKey, "old-key")
	ch, cancel := (&Hub{}).subscribeBrowserVideo(ms)
	defer cancel()
	// These have already reached the decoder. Repeating the configuration would
	// reset its same-WebSocket presentation floor and expose old replayed frames.
	<-ch
	<-ch
	for i := 0; i < 60; i++ {
		publishVideoTest(ms, scrcpy.FrameDelta, "old-delta")
	}
	publishVideoTest(ms, scrcpy.FrameKey, "new-key")
	for i := 0; i < 6; i++ {
		publishVideoTest(ms, scrcpy.FrameDelta, "new-delta")
	}
	frames := drainVideoTest(ch)
	if len(frames) != 7 || frames[0].data[0] != byte(scrcpy.FrameKey) {
		t.Fatalf("catchup reset the configured decoder: %d packets, first kind %d", len(frames), frames[0].data[0])
	}
	for _, f := range frames {
		if f.data[0] == byte(scrcpy.FrameConfig) {
			t.Fatal("same-epoch catchup replayed an already delivered configuration")
		}
	}
}

func TestBrowserReconnectAfterRotationUsesCurrentDimensionsWithoutNewEpoch(t *testing.T) {
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
	server := httptest.NewServer(http.HandlerFunc(h.ServeWS))
	defer server.Close()
	for i := 0; i < 2; i++ {
		conn, _, err := websocket.DefaultDialer.Dial("ws"+strings.TrimPrefix(server.URL, "http")+"?serial=rotated-device", nil)
		if err != nil {
			t.Fatal(err)
		}
		conn.SetReadDeadline(time.Now().Add(time.Second))
		var meta handshakeMeta
		if err := conn.ReadJSON(&meta); err != nil {
			conn.Close()
			t.Fatal(err)
		}
		if meta.Width != 1024 || meta.Height != 448 || meta.SessionID != "same-session" {
			conn.Close()
			t.Fatalf("reconnect announced stale dimensions or a new epoch: %+v", meta)
		}
		// A synthetic rotation would clear the same-session presentation floor.
		// Dimensions belong in meta, followed directly by the valid GOP snapshot.
		for _, kind := range []scrcpy.FrameKind{scrcpy.FrameConfig, scrcpy.FrameKey, scrcpy.FrameDelta} {
			typ, data, err := conn.ReadMessage()
			if err != nil {
				conn.Close()
				t.Fatal(err)
			}
			if typ != websocket.BinaryMessage || len(data) == 0 || data[0] != byte(kind) {
				conn.Close()
				t.Fatalf("snapshot introduced a reset or lost dependencies: %v", data)
			}
		}
		conn.Close()
	}
	if ms.meta.Width != 448 || ms.meta.Height != 1024 {
		t.Fatal("handshake update mutated shared initial metadata")
	}
}

func TestBrowserHandshakeSubscriptionCannotMissConcurrentRotation(t *testing.T) {
	for i := 0; i < 200; i++ {
		ms := &managedSession{meta: &handshakeMeta{Type: "meta", Codec: "h264", SessionID: "same-session", Width: 448, Height: 1024}, width: 448, height: 1024, subs: make(map[chan sharedVideoFrame]struct{})}
		publishVideoTest(ms, scrcpy.FrameConfig, "portrait-config")
		publishVideoTest(ms, scrcpy.FrameKey, "portrait-key")
		start := make(chan struct{})
		var meta handshakeMeta
		var frames <-chan sharedVideoFrame
		var cancel func()
		var wg sync.WaitGroup
		wg.Add(2)
		go func() { defer wg.Done(); <-start; meta, frames, cancel = (&Hub{}).subscribeBrowserVideoWithMeta(ms) }()
		go func() {
			defer wg.Done()
			<-start
			for _, f := range []*scrcpy.Frame{
				{Kind: scrcpy.FrameSession, Width: 1024, Height: 448},
				{Kind: scrcpy.FrameConfig, Payload: []byte("landscape-config")},
				{Kind: scrcpy.FrameKey, Payload: []byte("landscape-key")},
				{Kind: scrcpy.FrameDelta, Payload: []byte("landscape-static")},
			} {
				cacheAndBroadcast(ms, f, encodeFrame(f))
			}
		}()
		close(start)
		wg.Wait()
		width, height := meta.Width, meta.Height
		packets := drainVideoTest(frames)
		for _, f := range packets {
			if f.data[0] == byte(scrcpy.FrameSession) {
				width = binary.BigEndian.Uint32(f.data[9:13])
				height = binary.BigEndian.Uint32(f.data[13:17])
			}
		}
		cancel()
		if width != 1024 || height != 448 {
			t.Fatalf("rotation fell between handshake and subscription: iteration=%d meta=%+v", i, meta)
		}
		if len(packets) == 0 || string(packets[len(packets)-1].data[9:]) != "landscape-static" {
			t.Fatal("atomic subscription lost latest static frame")
		}
	}
}

func TestBrowserSlowVideoWriteTelemetryIsBoundedAndCorrelatesSource(t *testing.T) {
	h := NewHub("adb", "")
	defer h.Close()
	ms := &managedSession{meta: &handshakeMeta{Serial: "phone", SessionID: "session"}}
	f := &scrcpy.Frame{Kind: scrcpy.FrameDelta, PTS: 123456, Payload: []byte("packet")}
	packet := sharedVideoFrame{data: encodeFrame(f)}
	at := time.Unix(100, 0)
	var last time.Time
	last = h.recordBrowserVideoWrite(ms, packet, 90*time.Millisecond, 4, at, last)
	last = h.recordBrowserVideoWrite(ms, packet, 110*time.Millisecond, 5, at.Add(200*time.Millisecond), last)
	last = h.recordBrowserVideoWrite(ms, packet, 200*time.Millisecond, 6, at.Add(900*time.Millisecond), last)
	last = h.recordBrowserVideoWrite(ms, packet, 120*time.Millisecond, 7, at.Add(1300*time.Millisecond), last)
	events := h.events.Snapshot("phone", 10)
	if len(events) != 2 {
		t.Fatalf("slow writes were not rate limited to one event per second: %d", len(events))
	}
	newest := events[0]
	if newest.Type != "browser.video_slow" || newest.SessionID != "session" || newest.Fields["write_ms"] != int64(120) || newest.Fields["source_pts_us"] != uint64(123456) || newest.Fields["bytes"] != len(packet.data) || newest.Fields["queued_packets"] != 7 {
		t.Fatalf("incorrect source/write correlation: %+v", newest)
	}
}
