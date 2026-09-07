package framehub

import "testing"

func TestHubDropsOldFramesForSlowSubscriber(t *testing.T) {
	h := New(1)
	_, ch, cancel := h.Subscribe()
	defer cancel()
	h.Publish(Frame{FrameID: 1, Payload: []byte("a")})
	h.Publish(Frame{FrameID: 2, Payload: []byte("b")})
	f := <-ch
	if f.FrameID != 2 {
		t.Fatalf("got frame %d, want latest frame 2", f.FrameID)
	}
}
