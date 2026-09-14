package ws

import (
	"context"
	"strings"
	"sync"
	"testing"
	"time"

	"mywebscrcpy/internal/action"
	"mywebscrcpy/internal/adbcommand"
	"mywebscrcpy/internal/devicegate"
)

type mockADBManager struct {
	mu       sync.Mutex
	commands [][]string
	err      error
}

func (m *mockADBManager) ExecuteDirect(ctx context.Context, serial string, args ...string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	cmd := append([]string{serial}, args...)
	m.commands = append(m.commands, cmd)
	return m.err
}

func (m *mockADBManager) LastCommand() []string {
	m.mu.Lock()
	defer m.mu.Unlock()
	if len(m.commands) == 0 {
		return nil
	}
	return m.commands[len(m.commands)-1]
}

func TestExecuteADBTap(t *testing.T) {
	gate := devicegate.New(1)

	// Create manager wrapping ExecuteDirect behavior
	mgr := adbcommand.New("adb", func(string) *devicegate.Gate { return gate })

	h := &Hub{
		commands:  mgr,
		touchSubs: make(map[string]map[chan TouchEventMessage]string),
	}

	touchCh, cancelTouch := h.subscribeTouch("dev1", "sess1")
	defer cancelTouch()

	ms := &managedSession{
		meta:   &handshakeMeta{Serial: "dev1", SessionID: "sess1"},
		width:  1000,
		height: 2000,
	}

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	execWithMock := deviceActionExecutor{
		ms:    ms,
		gate:  gate,
		touch: touchPublisher{h: h, serial: "dev1", sessionID: "sess1"},
	}

	// 1. Without commands injected, expect adb_manager_unavailable
	err := execWithMock.executeADB(ctx, action.Request{
		DeviceID: "dev1",
		Action:   "tap",
		X:        0.5,
		Y:        0.5,
		Mode:     "adb",
		Humanize: false,
	})
	if err == nil || !strings.Contains(err.Error(), "adb_manager_unavailable") {
		t.Fatalf("expected adb_manager_unavailable, got: %v", err)
	}

	// Now inject test manager with "echo" as mock binary
	testMgr := adbcommand.New("echo", func(string) *devicegate.Gate { return gate })
	execWithMock.commands = testMgr

	// Test tap
	err = execWithMock.executeADB(ctx, action.Request{
		DeviceID: "dev1",
		Action:   "tap",
		X:        0.25,
		Y:        0.75,
		Mode:     "adb",
		Humanize: false,
	})
	if err != nil {
		t.Fatalf("executeADB tap failed: %v", err)
	}

	// Check touch visualization received the event
	select {
	case ev := <-touchCh:
		if ev.Kind != "tap" || ev.X != 0.25 || ev.Y != 0.75 {
			t.Fatalf("unexpected touch event: %+v", ev)
		}
	case <-time.After(time.Second):
		t.Fatal("timeout waiting for touch.event")
	}

	// 2. Humanized ADB tap (humanize=true)
	err = execWithMock.executeADB(ctx, action.Request{
		DeviceID: "dev1",
		Action:   "tap",
		X:        0.5,
		Y:        0.5,
		Mode:     "adb",
		Humanize: true,
	})
	if err != nil {
		t.Fatalf("executeADB humanized tap failed: %v", err)
	}

	select {
	case ev := <-touchCh:
		if ev.Kind != "tap" {
			t.Fatalf("unexpected touch event: %+v", ev)
		}
	case <-time.After(time.Second):
		t.Fatal("timeout waiting for humanized touch.event")
	}

	// 3. ADB swipe
	err = execWithMock.executeADB(ctx, action.Request{
		DeviceID:   "dev1",
		Action:     "swipe",
		X:          0.1,
		Y:          0.2,
		X2:         0.8,
		Y2:         0.9,
		Mode:       "adb",
		DurationMS: 400,
	})
	if err != nil {
		t.Fatalf("executeADB swipe failed: %v", err)
	}

	select {
	case ev := <-touchCh:
		if ev.Kind != "swipe" || ev.X != 0.1 || ev.Y != 0.2 || ev.X2 != 0.8 || ev.Y2 != 0.9 {
			t.Fatalf("unexpected swipe touch event: %+v", ev)
		}
	case <-time.After(time.Second):
		t.Fatal("timeout waiting for swipe touch.event")
	}
}
