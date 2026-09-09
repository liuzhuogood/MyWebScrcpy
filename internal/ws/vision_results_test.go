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
