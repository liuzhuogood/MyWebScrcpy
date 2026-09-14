package ws

import (
	"testing"

	"mywebscrcpy/internal/vision"
)

func TestResultSubscriberReceivesCurrentSessionState(t *testing.T) {
	h := NewHub("adb", "server.jar")
	empty := vision.Message{Type: "detection.result", SessionID: "session-1", Objects: []vision.Object{}}
	h.publishResult("device-1", empty)

	matching, unsubscribeMatching := h.subscribeResults("device-1", "session-1")
	defer unsubscribeMatching()
	select {
	case got := <-matching:
		if got.SessionID != empty.SessionID || got.Objects == nil || len(got.Objects) != 0 {
			t.Fatalf("unexpected replayed result: %#v", got)
		}
	default:
		t.Fatal("current session did not receive latest empty result")
	}

	otherSession, unsubscribeOther := h.subscribeResults("device-1", "session-2")
	defer unsubscribeOther()
	select {
	case got := <-otherSession:
		t.Fatalf("different session received stale result: %#v", got)
	default:
	}
}

func TestResultSubscriberIgnoresOtherSessionUpdates(t *testing.T) {
	h := NewHub("adb", "server.jar")
	current, unsubscribe := h.subscribeResults("device-1", "session-1")
	defer unsubscribe()
	h.publishResult("device-1", vision.Message{Type: "detection.result", SessionID: "session-2"})
	select {
	case got := <-current:
		t.Fatalf("subscriber received another session result: %#v", got)
	default:
	}
}

func TestResultSubscriberReceivesDeviceLevelBroadcast(t *testing.T) {
	h := NewHub("adb", "server.jar")
	// Subscriber with active session ID
	sub, unsubscribe := h.subscribeResults("device-1", "browser-session-123")
	defer unsubscribe()

	// Device-level broadcast (msg.SessionID == "")
	deviceMsg := vision.Message{
		Type:     "detection.result",
		DeviceID: "device-1",
		Objects: []vision.Object{
			{Label: "login_button", Confidence: 0.95, X: 0.1, Y: 0.2, W: 0.3, H: 0.4},
		},
	}
	h.publishResult("device-1", deviceMsg)

	select {
	case got := <-sub:
		if len(got.Objects) != 1 || got.Objects[0].Label != "login_button" {
			t.Fatalf("subscriber did not receive expected device-level broadcast: %#v", got)
		}
	default:
		t.Fatal("subscriber with session_id failed to receive device-level broadcast")
	}

	// Late-joining subscriber with session_id should receive the replayed device-level lastResult
	lateSub, lateUnsub := h.subscribeResults("device-1", "browser-session-456")
	defer lateUnsub()

	select {
	case got := <-lateSub:
		if len(got.Objects) != 1 || got.Objects[0].Label != "login_button" {
			t.Fatalf("late subscriber did not receive replayed device-level broadcast: %#v", got)
		}
	default:
		t.Fatal("late subscriber with session_id failed to receive replayed device-level broadcast")
	}
}

