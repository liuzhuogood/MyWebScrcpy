package ws

import (
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"mywebscrcpy/internal/scrcpy"
)

func TestMP4RecordingMuxerWritesAACTrack(t *testing.T) {
	ffmpeg, err := exec.LookPath("ffmpeg")
	if err != nil {
		t.Skip("ffmpeg is required for the audio MP4 integration test")
	}
	ffprobe, err := exec.LookPath("ffprobe")
	if err != nil {
		t.Skip("ffprobe is required for the audio MP4 integration test")
	}
	video := h264Fixture(t, ffmpeg)
	config, key := splitH264ConfigAndKey(t, video)
	audio := aacLCFrames(t, ffmpeg)
	path := filepath.Join(t.TempDir(), "recording.mp4")
	muxer, err := newMP4RecordingMuxer(path, true)
	if err != nil {
		t.Fatalf("new muxer: %v", err)
	}
	now := time.Now()
	if err := muxer.Write(scrcpy.FrameConfig, 0, makeAVCC(config...), now); err != nil {
		t.Fatalf("write video config: %v", err)
	}
	// AAC-LC, 48 kHz, stereo AudioSpecificConfig.
	if err := muxer.WriteAudio(true, 0, []byte{0x11, 0x90}, now); err != nil {
		t.Fatalf("write audio config: %v", err)
	}
	if err := muxer.Write(scrcpy.FrameKey, 0, makeAVCC(key...), now); err != nil {
		t.Fatalf("write video key: %v", err)
	}
	if err := muxer.WriteAudio(false, 0, audio[0], now); err != nil {
		t.Fatalf("write first audio frame: %v", err)
	}
	if err := muxer.Write(scrcpy.FrameKey, 33_333, makeAVCC(key...), now.Add(33*time.Millisecond)); err != nil {
		t.Fatalf("write video frame: %v", err)
	}
	if err := muxer.WriteAudio(false, 21_333, audio[1], now.Add(21*time.Millisecond)); err != nil {
		t.Fatalf("write second audio frame: %v", err)
	}
	if err := muxer.Close(now.Add(60 * time.Millisecond)); err != nil {
		t.Fatalf("close muxer: %v", err)
	}
	out, err := exec.Command(ffprobe, "-v", "error", "-show_entries", "stream=codec_name,codec_type", "-of", "default=noprint_wrappers=1", path).Output()
	if err != nil {
		t.Fatalf("probe MP4: %v", err)
	}
	if !strings.Contains(string(out), "codec_name=h264") || !strings.Contains(string(out), "codec_name=aac") {
		t.Fatalf("MP4 missing expected streams: %s", out)
	}
}

func aacLCFrames(t *testing.T, ffmpeg string) [][]byte {
	t.Helper()
	out, err := exec.Command(ffmpeg, "-hide_banner", "-loglevel", "error", "-f", "lavfi", "-i", "anullsrc=r=48000:cl=stereo", "-t", "0.1", "-c:a", "aac", "-profile:a", "aac_low", "-f", "adts", "pipe:1").Output()
	if err != nil {
		t.Fatalf("make AAC fixture: %v", err)
	}
	var frames [][]byte
	for len(out) >= 7 {
		if out[0] != 0xff || out[1]&0xf6 != 0xf0 {
			t.Fatalf("invalid ADTS header: %x", out[:min(len(out), 7)])
		}
		headerLen := 9
		if out[1]&1 != 0 {
			headerLen = 7
		}
		frameLen := int(out[3]&3)<<11 | int(out[4])<<3 | int(out[5])>>5
		if frameLen < headerLen || frameLen > len(out) {
			t.Fatalf("invalid ADTS frame length %d", frameLen)
		}
		frames = append(frames, append([]byte(nil), out[headerLen:frameLen]...))
		out = out[frameLen:]
	}
	if len(frames) < 2 {
		t.Fatalf("AAC fixture produced %d frames", len(frames))
	}
	return frames
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}
