package ws

import (
	"bytes"
	"encoding/binary"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
	"time"

	"mywebscrcpy/internal/scrcpy"
)

func TestAVCCToAnnexB(t *testing.T) {
	avcc := makeAVCC([]byte{0x67, 1, 2}, []byte{0x68, 3})
	got, err := avccToAnnexB(avcc)
	if err != nil {
		t.Fatalf("avccToAnnexB: %v", err)
	}
	want := []byte{0, 0, 0, 1, 0x67, 1, 2, 0, 0, 0, 1, 0x68, 3}
	if !bytes.Equal(got, want) {
		t.Fatalf("got %x, want %x", got, want)
	}
	if _, err := avccToAnnexB([]byte{0, 0, 0, 5, 1}); err == nil {
		t.Fatal("truncated AVCC must fail")
	}
	annexB := []byte{0, 0, 0, 1, 0x67, 1}
	got, err = avccToAnnexB(annexB)
	if err != nil || !bytes.Equal(got, annexB) {
		t.Fatalf("annex-b passthrough: got=%x err=%v", got, err)
	}
}

func TestRecordingStateAndSerialBoundary(t *testing.T) {
	m := &recordingManager{entries: make(map[string]*recording), bySerial: make(map[string]string)}
	rec := &recording{id: "rec_test", serial: "phone-a", status: recordingActive, stop: make(chan struct{})}
	m.entries[rec.id] = rec
	m.bySerial[rec.serial] = rec.id
	if _, err := m.get(rec.id, "phone-b"); err == nil {
		t.Fatal("other serial must not read recording")
	}
	stopped, err := m.stopRecording(rec.id, "phone-a")
	if err != nil || stopped.view().Status != recordingStopping {
		t.Fatalf("stop failed: rec=%+v err=%v", stopped.view(), err)
	}
	if _, err := m.stopRecording(rec.id, "phone-a"); err != nil {
		t.Fatalf("stop must be idempotent: %v", err)
	}
}

func TestRecordingStorageDir(t *testing.T) {
	t.Setenv("RECORDINGS_DIR", "/tmp/custom-recordings")
	if got := recordingStorageDir(); got != "/tmp/custom-recordings" {
		t.Fatalf("configured directory = %q", got)
	}

	t.Setenv("RECORDINGS_DIR", "")
	got := recordingStorageDir()
	if !filepath.IsAbs(got) || filepath.Base(got) != "recordings" {
		t.Fatalf("default directory must be an absolute recordings directory, got %q", got)
	}
}

func TestRecordingHTTPStatusBoundaries(t *testing.T) {
	h := &Hub{}
	h.recordings = &recordingManager{hub: h, entries: make(map[string]*recording), bySerial: make(map[string]string)}
	active := &recording{id: "rec_active", serial: "phone-a", status: recordingActive, stop: make(chan struct{})}
	expired := &recording{id: "rec_expired", serial: "phone-a", status: recordingExpired, stop: make(chan struct{})}
	h.recordings.entries[active.id] = active
	h.recordings.entries[expired.id] = expired
	mux := http.NewServeMux()
	h.RegisterRecordingRoutes(mux)

	for _, tc := range []struct {
		name, method, target, body string
		want                       int
	}{
		{"missing duration", http.MethodPost, "/api/recordings?serial=phone-a", `{}`, http.StatusBadRequest},
		{"wrong serial", http.MethodGet, "/api/recordings/rec_active?serial=phone-b", "", http.StatusBadRequest},
		{"not ready download", http.MethodGet, "/api/recordings/rec_active/download?serial=phone-a", "", http.StatusConflict},
		{"expired download", http.MethodGet, "/api/recordings/rec_expired/download?serial=phone-a", "", http.StatusGone},
		{"unknown recording", http.MethodGet, "/api/recordings/nope?serial=phone-a", "", http.StatusNotFound},
	} {
		t.Run(tc.name, func(t *testing.T) {
			r := httptest.NewRequest(tc.method, tc.target, strings.NewReader(tc.body))
			w := httptest.NewRecorder()
			mux.ServeHTTP(w, r)
			if w.Code != tc.want {
				t.Fatalf("status=%d want=%d body=%s", w.Code, tc.want, w.Body.String())
			}
		})
	}
}

func TestRecordingListAndDelete(t *testing.T) {
	dir := t.TempDir()
	h := &Hub{}
	h.recordings = &recordingManager{hub: h, dir: dir, entries: make(map[string]*recording), bySerial: make(map[string]string)}
	completed := &recording{id: "rec_done", serial: "phone-a", status: recordingComplete, startedAt: time.Now().Add(-time.Minute), endedAt: time.Now(), path: filepath.Join(dir, "rec_done.mp4")}
	if err := os.WriteFile(completed.path, []byte("mp4"), 0o600); err != nil {
		t.Fatal(err)
	}
	active := &recording{id: "rec_active", serial: "phone-a", status: recordingActive, startedAt: time.Now(), stop: make(chan struct{})}
	h.recordings.entries[completed.id] = completed
	h.recordings.entries[active.id] = active
	mux := http.NewServeMux()
	h.RegisterRecordingRoutes(mux)

	list := httptest.NewRecorder()
	mux.ServeHTTP(list, httptest.NewRequest(http.MethodGet, "/api/recordings?serial=phone-a", nil))
	if list.Code != http.StatusOK || !strings.Contains(list.Body.String(), "rec_done") {
		t.Fatalf("list status=%d body=%s", list.Code, list.Body.String())
	}
	deleteDone := httptest.NewRecorder()
	mux.ServeHTTP(deleteDone, httptest.NewRequest(http.MethodDelete, "/api/recordings/rec_done?serial=phone-a", nil))
	if deleteDone.Code != http.StatusNoContent {
		t.Fatalf("delete status=%d body=%s", deleteDone.Code, deleteDone.Body.String())
	}
	if _, err := os.Stat(completed.path); !os.IsNotExist(err) {
		t.Fatalf("completed file remains: %v", err)
	}
	deleteActive := httptest.NewRecorder()
	mux.ServeHTTP(deleteActive, httptest.NewRequest(http.MethodDelete, "/api/recordings/rec_active?serial=phone-a", nil))
	if deleteActive.Code != http.StatusConflict {
		t.Fatalf("active delete status=%d body=%s", deleteActive.Code, deleteActive.Body.String())
	}
}

func TestDownloadLatestRecording(t *testing.T) {
	dir := t.TempDir()
	h := &Hub{}
	h.recordings = &recordingManager{hub: h, dir: dir, entries: make(map[string]*recording), bySerial: make(map[string]string)}
	older := &recording{id: "rec_old", serial: "phone-a", status: recordingComplete, startedAt: time.Now().Add(-2 * time.Minute), endedAt: time.Now().Add(-time.Minute), path: filepath.Join(dir, "rec_old.mp4")}
	latest := &recording{id: "rec_new", serial: "phone-a", status: recordingComplete, startedAt: time.Now().Add(-time.Minute), endedAt: time.Now(), path: filepath.Join(dir, "rec_new.mp4")}
	for _, rec := range []*recording{older, latest} {
		if err := os.WriteFile(rec.path, []byte(rec.id), 0o600); err != nil {
			t.Fatal(err)
		}
		h.recordings.entries[rec.id] = rec
	}
	mux := http.NewServeMux()
	h.RegisterRecordingRoutes(mux)
	for _, target := range []string{"/api/recordings/download?serial=phone-a", "/api/recordings/download?serial=phone-a&recording_id=rec_old"} {
		w := httptest.NewRecorder()
		mux.ServeHTTP(w, httptest.NewRequest(http.MethodGet, target, nil))
		if w.Code != http.StatusOK {
			t.Fatalf("download %s status=%d body=%s", target, w.Code, w.Body.String())
		}
	}
}

func TestRecordingRunProducesPlayableMP4(t *testing.T) {
	ffmpeg, err := exec.LookPath("ffmpeg")
	if err != nil {
		t.Skip("ffmpeg is required for the MP4 integration test")
	}
	ffprobe, err := exec.LookPath("ffprobe")
	if err != nil {
		t.Skip("ffprobe is required for the MP4 integration test")
	}
	stream := h264Fixture(t, ffmpeg)
	config, key := splitH264ConfigAndKey(t, stream)
	dir := t.TempDir()
	h := &Hub{}
	m := &recordingManager{hub: h, dir: dir, quota: 64 << 20, retention: time.Hour, entries: make(map[string]*recording), bySerial: make(map[string]string)}
	ms := &managedSession{subs: make(map[chan sharedVideoFrame]struct{})}
	rec := &recording{id: "rec_test", serial: "phone-a", status: recordingActive, startedAt: time.Now(), maxDuration: time.Minute, path: filepath.Join(dir, "rec_test.mp4"), partialPath: filepath.Join(dir, "rec_test.mp4.partial"), stop: make(chan struct{})}
	m.entries[rec.id] = rec
	m.bySerial[rec.serial] = rec.id
	go m.run(rec, ms, func() {})
	waitForSubscriber(t, ms)
	cacheAndBroadcast(ms, &scrcpy.Frame{Kind: scrcpy.FrameConfig, Payload: makeAVCC(config...)}, encodeFrame(&scrcpy.Frame{Kind: scrcpy.FrameConfig, Payload: makeAVCC(config...)}))
	cacheAndBroadcast(ms, &scrcpy.Frame{Kind: scrcpy.FrameKey, PTS: 1, Payload: makeAVCC(key...)}, encodeFrame(&scrcpy.Frame{Kind: scrcpy.FrameKey, PTS: 1, Payload: makeAVCC(key...)}))
	time.Sleep(100 * time.Millisecond) // Let the recorder consume its key frame before requesting stop.
	rec.stopOnce.Do(func() { close(rec.stop) })
	waitForRecording(t, rec, recordingComplete)
	if rec.view().BytesWritten <= 0 {
		t.Fatalf("recording did not report bytes: %+v", rec.view())
	}
	probe := exec.Command(ffprobe, "-v", "error", "-show_entries", "format=format_name", "-of", "default=noprint_wrappers=1:nokey=1", rec.path)
	out, err := probe.Output()
	if err != nil || !strings.Contains(string(out), "mov") {
		t.Fatalf("output is not a playable MP4: out=%q err=%v", out, err)
	}
	if _, err := os.Stat(rec.partialPath); !os.IsNotExist(err) {
		t.Fatalf("partial output remains: %v", err)
	}
}

func TestRecordingAutoStopFinalizesMP4(t *testing.T) {
	ffmpeg, err := exec.LookPath("ffmpeg")
	if err != nil {
		t.Skip("ffmpeg is required for the MP4 integration test")
	}
	stream := h264Fixture(t, ffmpeg)
	config, key := splitH264ConfigAndKey(t, stream)
	dir := t.TempDir()
	h := &Hub{}
	m := &recordingManager{hub: h, dir: dir, quota: 64 << 20, retention: time.Hour, entries: make(map[string]*recording), bySerial: make(map[string]string)}
	ms := &managedSession{subs: make(map[chan sharedVideoFrame]struct{})}
	rec := &recording{id: "rec_timer", serial: "phone-a", status: recordingActive, startedAt: time.Now(), maxDuration: 120 * time.Millisecond, path: filepath.Join(dir, "rec_timer.mp4"), partialPath: filepath.Join(dir, "rec_timer.mp4.partial"), stop: make(chan struct{})}
	m.entries[rec.id] = rec
	m.bySerial[rec.serial] = rec.id
	go m.run(rec, ms, func() {})
	waitForSubscriber(t, ms)
	cacheAndBroadcast(ms, &scrcpy.Frame{Kind: scrcpy.FrameConfig, Payload: makeAVCC(config...)}, encodeFrame(&scrcpy.Frame{Kind: scrcpy.FrameConfig, Payload: makeAVCC(config...)}))
	cacheAndBroadcast(ms, &scrcpy.Frame{Kind: scrcpy.FrameKey, PTS: 1, Payload: makeAVCC(key...)}, encodeFrame(&scrcpy.Frame{Kind: scrcpy.FrameKey, PTS: 1, Payload: makeAVCC(key...)}))
	waitForRecording(t, rec, recordingComplete)
	if _, err := os.Stat(rec.path); err != nil {
		t.Fatalf("automatic stop did not publish MP4: %v", err)
	}
}

func TestRecordingRunRebasesReplayedFrameTimestamp(t *testing.T) {
	ffmpeg, err := exec.LookPath("ffmpeg")
	if err != nil {
		t.Skip("ffmpeg is required for the MP4 integration test")
	}
	ffprobe, err := exec.LookPath("ffprobe")
	if err != nil {
		t.Skip("ffprobe is required for the MP4 integration test")
	}
	stream := h264Fixture(t, ffmpeg)
	config, key := splitH264ConfigAndKey(t, stream)
	configFrame := &scrcpy.Frame{Kind: scrcpy.FrameConfig, Payload: makeAVCC(config...)}
	replayedKey := &scrcpy.Frame{Kind: scrcpy.FrameKey, PTS: 1, Payload: makeAVCC(key...)}
	dir := t.TempDir()
	h := &Hub{}
	m := &recordingManager{hub: h, dir: dir, quota: 64 << 20, retention: time.Hour, entries: make(map[string]*recording), bySerial: make(map[string]string)}
	ms := &managedSession{subs: make(map[chan sharedVideoFrame]struct{}), lastConfig: encodeFrame(configFrame), lastKey: encodeFrame(replayedKey)}
	rec := &recording{id: "rec_replayed", serial: "phone-a", status: recordingActive, startedAt: time.Now(), maxDuration: time.Minute, path: filepath.Join(dir, "rec_replayed.mp4"), partialPath: filepath.Join(dir, "rec_replayed.mp4.partial"), stop: make(chan struct{})}
	m.entries[rec.id] = rec
	m.bySerial[rec.serial] = rec.id
	go m.run(rec, ms, func() {})
	waitForSubscriber(t, ms)

	// The cached key is 327 seconds old, matching the reported production bug.
	cacheAndBroadcast(ms, &scrcpy.Frame{Kind: scrcpy.FrameKey, PTS: 327_000_000, Payload: makeAVCC(key...)}, encodeFrame(&scrcpy.Frame{Kind: scrcpy.FrameKey, PTS: 327_000_000, Payload: makeAVCC(key...)}))
	time.Sleep(100 * time.Millisecond)
	rec.stopOnce.Do(func() { close(rec.stop) })
	waitForRecording(t, rec, recordingComplete)

	out, err := exec.Command(ffprobe, "-v", "error", "-show_entries", "format=duration", "-of", "default=noprint_wrappers=1:nokey=1", rec.path).Output()
	if err != nil {
		t.Fatalf("probe recording: %v", err)
	}
	duration, err := strconv.ParseFloat(strings.TrimSpace(string(out)), 64)
	if err != nil {
		t.Fatalf("parse duration %q: %v", out, err)
	}
	if duration > 1 {
		t.Fatalf("replayed timestamp inflated recording duration to %.3fs", duration)
	}
}

func makeAVCC(nalus ...[]byte) []byte {
	var out []byte
	for _, nalu := range nalus {
		var size [4]byte
		binary.BigEndian.PutUint32(size[:], uint32(len(nalu)))
		out = append(out, size[:]...)
		out = append(out, nalu...)
	}
	return out
}

func h264Fixture(t *testing.T, ffmpeg string) []byte {
	t.Helper()
	cmd := exec.Command(ffmpeg, "-hide_banner", "-loglevel", "error", "-f", "lavfi", "-i", "color=c=black:s=32x32:r=5", "-frames:v", "1", "-c:v", "libx264", "-preset", "ultrafast", "-tune", "zerolatency", "-f", "h264", "pipe:1")
	out, err := cmd.Output()
	if err != nil {
		t.Fatalf("make h264 fixture: %v", err)
	}
	return out
}

func splitH264ConfigAndKey(t *testing.T, data []byte) (config, key [][]byte) {
	t.Helper()
	for _, nalu := range splitAnnexB(data) {
		if len(nalu) == 0 {
			continue
		}
		switch nalu[0] & 0x1f {
		case 7, 8:
			config = append(config, nalu)
		case 5:
			key = append(key, nalu)
		}
	}
	if len(config) < 2 || len(key) == 0 {
		t.Fatalf("fixture lacks SPS/PPS/IDR: config=%d key=%d", len(config), len(key))
	}
	return config, key
}

func splitAnnexB(data []byte) [][]byte {
	var result [][]byte
	start := -1
	for i := 0; i+3 < len(data); i++ {
		length := 0
		if data[i] == 0 && data[i+1] == 0 && data[i+2] == 1 {
			length = 3
		} else if data[i] == 0 && data[i+1] == 0 && data[i+2] == 0 && data[i+3] == 1 {
			length = 4
		}
		if length == 0 {
			continue
		}
		if start >= 0 && start < i {
			result = append(result, append([]byte(nil), data[start:i]...))
		}
		start = i + length
		i += length - 1
	}
	if start >= 0 && start < len(data) {
		result = append(result, append([]byte(nil), data[start:]...))
	}
	return result
}

func waitForSubscriber(t *testing.T, ms *managedSession) {
	t.Helper()
	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		ms.mu.Lock()
		n := len(ms.subs)
		ms.mu.Unlock()
		if n == 1 {
			return
		}
		time.Sleep(time.Millisecond)
	}
	t.Fatal("recording never subscribed to shared video")
}

func waitForRecording(t *testing.T, rec *recording, want recordingStatus) {
	t.Helper()
	deadline := time.Now().Add(5 * time.Second)
	for time.Now().Before(deadline) {
		if rec.view().Status == want {
			return
		}
		time.Sleep(10 * time.Millisecond)
	}
	t.Fatalf("recording status=%s want %s, details=%+v", rec.view().Status, want, rec.view())
}
