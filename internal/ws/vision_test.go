package ws

import (
	"testing"
	"time"

	"mywebscrcpy/internal/scrcpy"
)

func TestNormalizeVisionFPS(t *testing.T) {
	if normalizeVisionFPS(-1) != 0 || normalizeVisionFPS(0) != 0 || normalizeVisionFPS(5) != 5 || normalizeVisionFPS(99) != 30 {
		t.Fatalf("unexpected max_fps normalization")
	}
}

func TestVisionFPSKeepsDecoderPacketsAndSamplesDelta(t *testing.T) {
	now := time.Unix(100, 0)
	last := now.Add(-50 * time.Millisecond)
	if !shouldEmitVisionFrame(byte(scrcpy.FrameConfig), 5, last, now) {
		t.Fatal("config frame must always be emitted")
	}
	if !shouldEmitVisionFrame(byte(scrcpy.FrameKey), 5, last, now) {
		t.Fatal("key frame must always be emitted")
	}
	if shouldEmitVisionFrame(byte(scrcpy.FrameDelta), 5, last, now) {
		t.Fatal("delta inside max_fps interval must be sampled")
	}
	if !shouldEmitVisionFrame(byte(scrcpy.FrameDelta), 5, last, now.Add(200*time.Millisecond)) {
		t.Fatal("delta at max_fps interval must be emitted")
	}
}

func TestSupportedVisionProtocol(t *testing.T) {
	for _, protocol := range []string{"", "1"} {
		if !supportedVisionProtocol(protocol) {
			t.Fatalf("protocol %q should be supported", protocol)
		}
	}
	if supportedVisionProtocol("2") {
		t.Fatal("future protocol must not be silently accepted")
	}
}
