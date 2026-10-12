package ws

import "testing"

func TestClosedHubRejectsNewSessions(t *testing.T) {
	h := &Hub{}
	h.Close()
	h.Close()
	if _, _, _, err := h.acquireSession("device"); err == nil {
		t.Fatal("closed hub accepted new session")
	}
}
