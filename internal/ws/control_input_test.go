package ws

import (
	"bytes"
	"context"
	"encoding/binary"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"strings"

	"github.com/gorilla/websocket"
	"mywebscrcpy/internal/action"
	"testing"
	"time"

	"mywebscrcpy/internal/scrcpy"
)

func TestControlInputCleanupKeepsNewOwner(t *testing.T) {
	var state controlInputOwners
	var writes [][]byte
	write := func(_ context.Context, b []byte) error {
		writes = append(writes, append([]byte(nil), b...))
		return nil
	}
	down := scrcpy.TouchEvent(scrcpy.ActionDown, scrcpy.PointerIDMouse, 20, 30, 100, 200, 1, 1, 1)
	key := scrcpy.KeyCodeEvent(scrcpy.KeyActionDown, 4, 0, 0)
	for _, owner := range []string{"old", "new"} {
		if err := state.write(context.Background(), owner, down, write); err != nil {
			t.Fatal(err)
		}
		if err := state.write(context.Background(), owner, key, write); err != nil {
			t.Fatal(err)
		}
	}
	writes = nil
	if err := state.release(context.Background(), "old", false, write); err != nil {
		t.Fatal(err)
	}
	if len(writes) != 0 {
		t.Fatalf("old disconnect released new client's inputs: %v", writes)
	}
	if err := state.release(context.Background(), "new", false, write); err != nil {
		t.Fatal(err)
	}
	if len(writes) != 2 {
		t.Fatalf("want touch and key releases, got %v", writes)
	}
	for _, b := range writes {
		if b[1] != 1 {
			t.Fatalf("not release: %v", b)
		}
	}
}

func TestControlInputIgnoresLateMoveAndReleaseFromOldOwner(t *testing.T) {
	var state controlInputOwners
	var writes [][]byte
	write := func(_ context.Context, b []byte) error {
		writes = append(writes, append([]byte(nil), b...))
		return nil
	}
	down := scrcpy.TouchEvent(scrcpy.ActionDown, 7, 20, 30, 100, 200, 1, 1, 1)
	if err := state.write(context.Background(), "old", down, write); err != nil {
		t.Fatal(err)
	}
	if err := state.write(context.Background(), "new", down, write); err != nil {
		t.Fatal(err)
	}
	writes = nil
	for _, action := range []uint8{scrcpy.ActionMove, scrcpy.ActionUp} {
		raw := scrcpy.TouchEvent(action, 7, 80, 90, 100, 200, 1, 1, 1)
		if err := state.write(context.Background(), "old", raw, write); err != nil {
			t.Fatal(err)
		}
	}
	if len(writes) != 0 {
		t.Fatalf("old input affected new owner's gesture: %v", writes)
	}
	if err := state.release(context.Background(), "new", false, write); err != nil {
		t.Fatal(err)
	}
	if len(writes) != 1 || binary.BigEndian.Uint32(writes[0][10:14]) != 20 {
		t.Fatalf("new owner's position changed by stale move: %v", writes)
	}
}

func TestControlInputCleanupUsesLastPositionAndClearsButtons(t *testing.T) {
	var state controlInputOwners
	var last []byte
	write := func(_ context.Context, b []byte) error { last = append([]byte(nil), b...); return nil }
	for _, action := range []uint8{scrcpy.ActionDown, scrcpy.ActionMove} {
		b := scrcpy.TouchEvent(action, 9, 40, 70, 100, 200, 1, 1, 1)
		if err := state.write(context.Background(), "owner", b, write); err != nil {
			t.Fatal(err)
		}
	}
	if err := state.release(context.Background(), "owner", false, write); err != nil {
		t.Fatal(err)
	}
	want := scrcpy.TouchEvent(scrcpy.ActionUp, 9, 40, 70, 100, 200, 0, 0, 0)
	if !bytes.Equal(last, want) {
		t.Fatalf("cleanup=%v want=%v", last, want)
	}
	last = nil
	if err := state.release(context.Background(), "owner", false, write); err != nil {
		t.Fatal(err)
	}
	if last != nil {
		t.Fatal("cleanup did not clear owned state")
	}
}

func TestControlInputCanceledRequestDoesNotRepress(t *testing.T) {
	var state controlInputOwners
	called := false
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	err := state.write(ctx, "owner", scrcpy.KeyCodeEvent(0, 4, 0, 0), func(context.Context, []byte) error { called = true; return nil })
	if err != context.Canceled || called {
		t.Fatalf("canceled request was written: %v %v", err, called)
	}
}

func TestControlInputOwnershipIndexTracksTakeoverAndRelease(t *testing.T) {
	var state controlInputOwners
	write := func(context.Context, []byte) error { return nil }
	down := scrcpy.TouchEvent(scrcpy.ActionDown, 1, 1, 1, 10, 10, 1, 1, 1)
	key := scrcpy.KeyCodeEvent(scrcpy.KeyActionDown, 4, 0, 0)
	for _, raw := range [][]byte{down, key} {
		if err := state.write(context.Background(), "old", raw, write); err != nil {
			t.Fatal(err)
		}
	}
	if !state.hasOwner("old") || state.hasOwner("new") {
		t.Fatal("ownership missing")
	}
	for _, raw := range [][]byte{down, key} {
		if err := state.write(context.Background(), "new", raw, write); err != nil {
			t.Fatal(err)
		}
	}
	if state.hasOwner("old") || !state.hasOwner("new") {
		t.Fatal("takeover retained old ownership")
	}
	if err := state.release(context.Background(), "new", false, write); err != nil {
		t.Fatal(err)
	}
	if state.hasOwner("new") {
		t.Fatal("release retained ownership")
	}
}

func TestIdleBrowserCleanupDoesNotWaitForOtherGesture(t *testing.T) {
	ms := &managedSession{sess: &session{conn: &scrcpy.Connection{}}}
	ms.controlMu.Lock()
	defer ms.controlMu.Unlock()
	done := make(chan struct{})
	go func() { ms.releaseBrowserControl("idle"); close(done) }()
	select {
	case <-done:
	case <-time.After(100 * time.Millisecond):
		t.Fatal("idle connection cleanup waited for unrelated gesture")
	}
}

type cancelingControlExecutor struct {
	started  chan struct{}
	canceled chan struct{}
}

func (e *cancelingControlExecutor) Execute(ctx context.Context, _ action.Request) error {
	close(e.started)
	<-ctx.Done()
	close(e.canceled)
	return ctx.Err()
}

func TestPumpControlDisconnectCancelsRunningInput(t *testing.T) {
	exec := &cancelingControlExecutor{started: make(chan struct{}), canceled: make(chan struct{})}
	ms := &managedSession{meta: &handshakeMeta{Serial: "device", SessionID: "session"}, actions: action.New(exec, 4)}
	defer ms.actions.Close()
	h := &Hub{}
	finished := make(chan struct{})
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		c, err := upgrader.Upgrade(w, r, nil)
		if err != nil {
			return
		}
		h.pumpControl(c, ms)
		close(finished)
	}))
	defer server.Close()
	c, _, err := websocket.DefaultDialer.Dial("ws"+strings.TrimPrefix(server.URL, "http"), nil)
	if err != nil {
		t.Fatal(err)
	}
	defer c.Close()
	if err = c.WriteMessage(websocket.BinaryMessage, scrcpy.TouchEvent(scrcpy.ActionDown, 1, 1, 1, 10, 10, 1, 1, 1)); err != nil {
		t.Fatal(err)
	}
	select {
	case <-exec.started:
	case <-time.After(time.Second):
		t.Fatal("input did not reach executor")
	}
	c.Close()
	select {
	case <-exec.canceled:
	case <-time.After(time.Second):
		t.Fatal("disconnect was not read while action was blocked")
	}
	select {
	case <-finished:
	case <-time.After(time.Second):
		t.Fatal("control pump leaked after disconnect")
	}
}

type recordingControlExecutor struct{ requests chan action.Request }

func (e recordingControlExecutor) Execute(_ context.Context, r action.Request) error {
	e.requests <- r
	return nil
}
func TestPumpControlPreservesInputOrderAndOwner(t *testing.T) {
	requests := make(chan action.Request, 3)
	ms := &managedSession{meta: &handshakeMeta{Serial: "device", SessionID: "session"}, actions: action.New(recordingControlExecutor{requests}, 4)}
	defer ms.actions.Close()
	h := &Hub{}
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		c, err := upgrader.Upgrade(w, r, nil)
		if err != nil {
			return
		}
		h.pumpControl(c, ms)
	}))
	defer server.Close()
	c, _, err := websocket.DefaultDialer.Dial("ws"+strings.TrimPrefix(server.URL, "http"), nil)
	if err != nil {
		t.Fatal(err)
	}
	defer c.Close()
	actions := []uint8{scrcpy.ActionDown, scrcpy.ActionMove, scrcpy.ActionUp}
	for _, a := range actions {
		if err = c.WriteMessage(websocket.BinaryMessage, scrcpy.TouchEvent(a, 1, 1, 1, 10, 10, 1, 1, 1)); err != nil {
			t.Fatal(err)
		}
	}
	owner := ""
	for _, a := range actions {
		select {
		case r := <-requests:
			if r.Raw[1] != a || r.ControlOwner == "" {
				t.Fatalf("invalid raw order/owner: %+v", r)
			}
			if owner != "" && r.ControlOwner != owner {
				t.Fatal("owner changed within one connection")
			}
			owner = r.ControlOwner
		case <-time.After(time.Second):
			t.Fatal("missing ordered input")
		}
	}
}

// The fixture speaks the existing scrcpy v4 forward handshake and collects
// actual control socket bytes. No ADB process or device is involved.
func controlSocketFixture(t *testing.T) (*scrcpy.Connection, <-chan []byte) {
	t.Helper()
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { listener.Close() })
	packets := make(chan []byte, 16)
	go func() {
		defer close(packets)
		video, err := listener.Accept()
		if err != nil {
			return
		}
		defer video.Close()
		if _, err = video.Write([]byte{0}); err != nil {
			return
		}
		ctrl, err := listener.Accept()
		if err != nil {
			return
		}
		defer ctrl.Close()
		header := make([]byte, 16)
		binary.BigEndian.PutUint32(header[:4], scrcpy.CodecIDH264)
		binary.BigEndian.PutUint64(header[4:12], scrcpy.FlagSessionPacket)
		binary.BigEndian.PutUint32(header[8:12], 100)
		binary.BigEndian.PutUint32(header[12:16], 200)
		if _, err = video.Write(header); err != nil {
			return
		}
		for {
			var typ [1]byte
			if _, err = io.ReadFull(ctrl, typ[:]); err != nil {
				return
			}
			size := 0
			switch typ[0] {
			case scrcpy.TypeInjectTouchEvent:
				size = 32
			case scrcpy.TypeInjectKeyCode:
				size = 14
			case scrcpy.TypeBackOrScreenOn:
				size = 2
			default:
				return
			}
			packet := make([]byte, size)
			packet[0] = typ[0]
			if _, err = io.ReadFull(ctrl, packet[1:]); err != nil {
				return
			}
			packets <- packet
		}
	}()
	conn, err := scrcpy.Dial(listener.Addr().(*net.TCPAddr).Port, false, true, time.Second)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(conn.Close)
	return conn, packets
}

func TestPumpControlDisconnectReleasesActualHeldInputs(t *testing.T) {
	device, packets := controlSocketFixture(t)
	ms := &managedSession{meta: &handshakeMeta{Serial: "device", SessionID: "session"}, sess: &session{conn: device}}
	ms.actions = action.New(deviceActionExecutor{ms: ms}, 8)
	defer ms.actions.Close()
	h := &Hub{}
	finished := make(chan struct{})
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		c, err := upgrader.Upgrade(w, r, nil)
		if err != nil {
			return
		}
		h.pumpControl(c, ms)
		close(finished)
	}))
	defer server.Close()
	c, _, err := websocket.DefaultDialer.Dial("ws"+strings.TrimPrefix(server.URL, "http"), nil)
	if err != nil {
		t.Fatal(err)
	}
	defer c.Close()
	down := scrcpy.TouchEvent(scrcpy.ActionDown, 42, 25, 60, 100, 200, 1, 1, 1)
	key := scrcpy.KeyCodeEvent(scrcpy.KeyActionDown, 4, 0, 0)
	for _, b := range [][]byte{down, key} {
		if err = c.WriteMessage(websocket.BinaryMessage, b); err != nil {
			t.Fatal(err)
		}
		select {
		case got := <-packets:
			if !bytes.Equal(got, b) {
				t.Fatalf("socket input=%v want=%v", got, b)
			}
		case <-time.After(time.Second):
			t.Fatal("input not written to device socket")
		}
	}
	c.Close()
	for i := 0; i < 2; i++ {
		select {
		case b := <-packets:
			if len(b) < 2 || b[1] != 1 {
				t.Fatalf("disconnect did not release held input: %v", b)
			}
			if b[0] == scrcpy.TypeInjectTouchEvent && (binary.BigEndian.Uint64(b[2:10]) != 42 || binary.BigEndian.Uint32(b[10:14]) != 25 || binary.BigEndian.Uint16(b[22:24]) != 0) {
				t.Fatalf("invalid pointer release: %v", b)
			}
		case <-time.After(time.Second):
			t.Fatal("disconnect left input pressed on actual control socket")
		}
	}
	select {
	case <-finished:
	case <-time.After(time.Second):
		t.Fatal("pump did not finish cleanup")
	}
}

func TestPumpControlIdleDisconnectLeavesOtherGestureAlive(t *testing.T) {
	device, _ := controlSocketFixture(t)
	ms := &managedSession{meta: &handshakeMeta{Serial: "device", SessionID: "session"}, sess: &session{conn: device}}
	ms.actions = action.New(deviceActionExecutor{ms: ms}, 8)
	defer ms.actions.Close()
	ms.controlMu.Lock()
	defer ms.controlMu.Unlock()
	h := &Hub{}
	finished := make(chan struct{})
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		c, err := upgrader.Upgrade(w, r, nil)
		if err != nil {
			return
		}
		h.pumpControl(c, ms)
		close(finished)
	}))
	defer server.Close()
	c, _, err := websocket.DefaultDialer.Dial("ws"+strings.TrimPrefix(server.URL, "http"), nil)
	if err != nil {
		t.Fatal(err)
	}
	c.Close()
	select {
	case <-finished:
	case <-time.After(time.Second):
		t.Fatal("idle browser blocked on unrelated gesture's mutex")
	}
	// The long gesture still owns controlMu, but the idle disconnect must leave
	// its shared control socket intact.
	if err = device.WriteControl(scrcpy.KeyCodeEvent(scrcpy.KeyActionUp, 4, 0, 0)); err != nil {
		t.Fatalf("idle disconnect closed another client's session: %v", err)
	}
}
