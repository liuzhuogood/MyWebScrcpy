package vision

import (
	"testing"
	"time"
)

func TestIsExpired(t *testing.T) {
	now := time.UnixMilli(10_000)
	if IsExpired(Message{Timestamp: 9_500, ExpiresMS: 600}, now) {
		t.Fatal("result inside expiry window was rejected")
	}
	if !IsExpired(Message{Timestamp: 9_000, ExpiresMS: 500}, now) {
		t.Fatal("stale result was not expired")
	}
	if IsExpired(Message{Timestamp: 0, ExpiresMS: 1}, now) {
		t.Fatal("missing timestamp should not be expired")
	}
}
