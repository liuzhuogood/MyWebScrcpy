package ws

import (
	"bytes"
	"encoding/binary"
	"encoding/hex"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"mywebscrcpy/internal/action"
	"mywebscrcpy/internal/scrcpy"
)

// Real 64x64 baseline H.264 NALUs captured from libx264 (ultrafast).
var (
	replayTestSPS = mustHex("6742c00ada109b0110000003001000000300a8f1226a")
	replayTestPPS = mustHex("68ce0fc8")
	replayTestIDR = mustHex("6588843a2628000902c9c9c9d75d75d75d75d75d75e0")
	replayTestP1  = mustHex("419a2014a08c")
	replayTestP2  = mustHex("419a4014a08c")
)

func mustHex(s string) []byte {
	b, err := hex.DecodeString(s)
	if err != nil {
		panic(err)
	}
	return b
}

func annexBFrame(nalus ...[]byte) []byte {
	var out []byte
	for _, nalu := range nalus {
		out = append(out, 0, 0, 0, 1)
		out = append(out, nalu...)
	}
	return out
}

func buildReplayFixtureMP4(t *testing.T, path string) {
	t.Helper()
	muxer, err := newMP4RecordingMuxer(path, false)
	if err != nil {
		t.Fatalf("new muxer: %v", err)
	}
	now := time.Now()
	config := annexBFrame(replayTestSPS, replayTestPPS)
	key := annexBFrame(replayTestIDR)
	if err := muxer.Write(scrcpy.FrameConfig, 0, config, now); err != nil {
		t.Fatalf("write config: %v", err)
	}
	for i, p := range [][]byte{key, annexBFrame(replayTestP1), annexBFrame(replayTestP2)} {
		kind := scrcpy.FrameKey
		if i > 0 {
			kind = scrcpy.FrameDelta
		}
		if err := muxer.Write(kind, uint64(1000*(i+1)), p, now); err != nil {
			t.Fatalf("write sample %d: %v", i, err)
		}
	}
	if err := muxer.Close(now.Add(4 * time.Millisecond)); err != nil {
		t.Fatalf("close muxer: %v", err)
	}
}

func TestDemuxRecordingRoundTrip(t *testing.T) {
	path := filepath.Join(t.TempDir(), "replay.mp4")
	buildReplayFixtureMP4(t, path)
	media, err := demuxRecordingMP4(path)
	if err != nil {
		t.Fatalf("demux: %v", err)
	}
	if media.width != 64 || media.height != 64 {
		t.Fatalf("resolution = %dx%d, want 64x64", media.width, media.height)
	}
	if got := splitAnnexBNALUs(media.config); len(got) != 2 || !bytes.Equal(got[0], replayTestSPS) || !bytes.Equal(got[1], replayTestPPS) {
		t.Fatalf("config SPS/PPS mismatch: %d nalus", len(got))
	}
	if len(media.samples) != 3 {
		t.Fatalf("samples = %d, want 3", len(media.samples))
	}
	wantKinds := []bool{true, false, false}
	wantPayloads := [][]byte{annexBFrame(replayTestIDR), annexBFrame(replayTestP1), annexBFrame(replayTestP2)}
	var prev uint64
	for i, s := range media.samples {
		if s.key != wantKinds[i] {
			t.Fatalf("sample %d key = %v, want %v", i, s.key, wantKinds[i])
		}
		if i > 0 && s.pts <= prev {
			t.Fatalf("PTS not monotonic: sample %d pts=%d prev=%d", i, s.pts, prev)
		}
		prev = s.pts
		if !bytes.Equal(s.data, wantPayloads[i]) {
			t.Fatalf("sample %d payload mismatch: got %x", i, s.data)
		}
	}
	// The muxer stores decodeTime accumulated from zero, so replay PTS are
	// relative (0-based) rather than the absolute scrcpy PTS.
	if media.samples[0].pts != 0 || media.samples[1].pts != 1000 || media.samples[2].pts != 2000 {
		t.Fatalf("PTS values = %d,%d,%d, want 0,1000,2000",
			media.samples[0].pts, media.samples[1].pts, media.samples[2].pts)
	}
}

func TestDemuxRejectsNonH264(t *testing.T) {
	if _, err := demuxRecordingMP4(filepath.Join(t.TempDir(), "missing.mp4")); !os.IsNotExist(err) {
		t.Fatalf("missing file err = %v", err)
	}
	bad := filepath.Join(t.TempDir(), "bad.mp4")
	if err := os.WriteFile(bad, []byte("not an mp4"), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := demuxRecordingMP4(bad); err == nil {
		t.Fatal("corrupt file must fail demux")
	}
}

func TestSPSResolution(t *testing.T) {
	w, h, err := h264ResolutionFromSPS(replayTestSPS)
	if err != nil || w != 64 || h != 64 {
		t.Fatalf("sps resolution = %dx%d err=%v", w, h, err)
	}
	if _, _, err := h264ResolutionFromSPS([]byte{0x67}); err == nil {
		t.Fatal("truncated sps must fail")
	}
}

func replayTestHub(t *testing.T, mp4Path string, status recordingStatus) (*Hub, *http.ServeMux) {
	t.Helper()
	h := &Hub{}
	h.recordings = &recordingManager{hub: h, dir: t.TempDir(), entries: make(map[string]*recording), bySerial: make(map[string]string)}
	rec := &recording{id: "rec_replay", serial: "phone-a", status: status, startedAt: time.Now(), path: mp4Path, stop: make(chan struct{})}
	if status == recordingComplete {
		buildReplayFixtureMP4(t, mp4Path)
	}
	h.recordings.entries[rec.id] = rec
	mux := http.NewServeMux()
	h.RegisterReplayRoutes(mux)
	return h, mux
}

func serveReplay(t *testing.T, mux *http.ServeMux, method, target, body string) *httptest.ResponseRecorder {
	t.Helper()
	r := httptest.NewRequest(method, target, strings.NewReader(body))
	w := httptest.NewRecorder()
	mux.ServeHTTP(w, r)
	return w
}

func waitForReplayGone(t *testing.T, mux *http.ServeMux) {
	t.Helper()
	deadline := time.Now().Add(5 * time.Second)
	for time.Now().Before(deadline) {
		w := serveReplay(t, mux, http.MethodGet, "/api/replay?serial=phone-a", "")
		if strings.Contains(w.Body.String(), `"replay":null`) {
			return
		}
		time.Sleep(10 * time.Millisecond)
	}
	t.Fatal("replay did not finish")
}

func TestReplayAPIStateMachine(t *testing.T) {
	dir := t.TempDir()
	mp4Path := filepath.Join(dir, "rec_replay.mp4")
	h, mux := replayTestHub(t, mp4Path, recordingComplete)

	if w := serveReplay(t, mux, http.MethodPost, "/api/replay/start", `{"recording_id":"rec_replay"}`); w.Code != http.StatusBadRequest {
		t.Fatalf("missing serial status=%d", w.Code)
	}
	if w := serveReplay(t, mux, http.MethodGet, "/api/replay", ""); w.Code != http.StatusBadRequest {
		t.Fatalf("missing serial get status=%d", w.Code)
	}
	if w := serveReplay(t, mux, http.MethodGet, "/api/replay?serial=phone-a", ""); w.Code != http.StatusOK || !strings.Contains(w.Body.String(), `"replay":null`) {
		t.Fatalf("initial state must be LIVE(null): status=%d body=%s", w.Code, w.Body.String())
	}
	if w := serveReplay(t, mux, http.MethodPost, "/api/replay/start?serial=phone-a", `{"recording_id":"nope"}`); w.Code != http.StatusNotFound {
		t.Fatalf("unknown recording status=%d", w.Code)
	}
	start := serveReplay(t, mux, http.MethodPost, "/api/replay/start?serial=phone-a", `{"recording_id":"rec_replay"}`)
	if start.Code != http.StatusAccepted || !strings.Contains(start.Body.String(), `"status":"replaying"`) {
		t.Fatalf("start status=%d body=%s", start.Code, start.Body.String())
	}
	if w := serveReplay(t, mux, http.MethodPost, "/api/replay/start?serial=phone-a", `{"recording_id":"rec_replay"}`); w.Code != http.StatusConflict {
		t.Fatalf("second start status=%d, want 409", w.Code)
	}
	get := serveReplay(t, mux, http.MethodGet, "/api/replay?serial=phone-a", "")
	if get.Code != http.StatusOK || !strings.Contains(get.Body.String(), "rec_replay") {
		t.Fatalf("get status=%d body=%s", get.Code, get.Body.String())
	}

	// While REPLAYING, acquireSession must serve the replay without touching adb.
	ms, meta, release, err := h.acquireSession("phone-a")
	if err != nil {
		t.Fatalf("acquire during replay: %v", err)
	}
	if meta.Codec != "h264" || meta.Width != 64 || meta.Height != 64 {
		t.Fatalf("replay meta = %+v", meta)
	}
	ch, cancel := h.subscribeShared(ms)
	first := <-ch
	second := <-ch
	cancel()
	release()
	if first.data[0] != byte(scrcpy.FrameConfig) || !first.replayed || second.data[0] != byte(scrcpy.FrameKey) || !second.replayed {
		t.Fatalf("replay bootstrap must be replayed config/key: %v %v", first.data[0], second.data[0])
	}

	if w := serveReplay(t, mux, http.MethodPost, "/api/replay/stop?serial=phone-a", ""); w.Code != http.StatusAccepted {
		t.Fatalf("stop status=%d body=%s", w.Code, w.Body.String())
	}
	waitForReplayGone(t, mux)
	if w := serveReplay(t, mux, http.MethodPost, "/api/replay/stop?serial=phone-a", ""); w.Code != http.StatusNotFound {
		t.Fatalf("stop without replay status=%d, want 404", w.Code)
	}
}

func TestReplayAutoFinishes(t *testing.T) {
	dir := t.TempDir()
	mp4Path := filepath.Join(dir, "rec_replay.mp4")
	_, mux := replayTestHub(t, mp4Path, recordingComplete)
	if w := serveReplay(t, mux, http.MethodPost, "/api/replay/start?serial=phone-a", `{"recording_id":"rec_replay"}`); w.Code != http.StatusAccepted {
		t.Fatalf("start status=%d body=%s", w.Code, w.Body.String())
	}
	// 3 samples paced over ~2ms of PTS must drain on their own.
	waitForReplayGone(t, mux)
}

func TestReplayRejectsIncompleteRecording(t *testing.T) {
	dir := t.TempDir()
	_, mux := replayTestHub(t, filepath.Join(dir, "rec_active.mp4"), recordingActive)
	if w := serveReplay(t, mux, http.MethodPost, "/api/replay/start?serial=phone-a", `{"recording_id":"rec_replay"}`); w.Code != http.StatusConflict {
		t.Fatalf("incomplete recording status=%d, want 409", w.Code)
	}
}

func TestReplayLoopFieldDefaultsFalse(t *testing.T) {
	dir := t.TempDir()
	mp4Path := filepath.Join(dir, "rec_replay.mp4")
	_, mux := replayTestHub(t, mp4Path, recordingComplete)

	start := serveReplay(t, mux, http.MethodPost, "/api/replay/start?serial=phone-a", `{"recording_id":"rec_replay"}`)
	if start.Code != http.StatusAccepted {
		t.Fatalf("start status=%d body=%s", start.Code, start.Body.String())
	}
	if !strings.Contains(start.Body.String(), `"loop":false`) {
		t.Fatalf("missing loop:false in start body: %s", start.Body.String())
	}
	get := serveReplay(t, mux, http.MethodGet, "/api/replay?serial=phone-a", "")
	if !strings.Contains(get.Body.String(), `"loop":false`) {
		t.Fatalf("missing loop:false in get body: %s", get.Body.String())
	}
	if w := serveReplay(t, mux, http.MethodPost, "/api/replay/stop?serial=phone-a", ""); w.Code != http.StatusAccepted {
		t.Fatalf("stop status=%d", w.Code)
	}
	waitForReplayGone(t, mux)
}

func TestReplayLoopSecondPass(t *testing.T) {
	path := filepath.Join(t.TempDir(), "replay.mp4")
	buildReplayFixtureMP4(t, path)
	media, err := demuxRecordingMP4(path)
	if err != nil {
		t.Fatal(err)
	}
	h := &Hub{}
	h.replays = newReplayManager(h)
	meta := &handshakeMeta{Type: "meta", Codec: "h264", Width: media.width, Height: media.height, Serial: "phone-a", SessionID: "test-loop"}
	ms := &managedSession{replay: true, meta: meta, refs: 1, subs: make(map[chan sharedVideoFrame]struct{}), audioSubs: make(map[chan sharedAudioPacket]struct{}), width: media.width, height: media.height}
	ms.actions = action.New(deviceActionExecutor{ms: ms}, 64)
	rp := &replay{id: "rp_loop", serial: "phone-a", recordingID: "rec_replay", startedAt: time.Now(), loop: true, ms: ms, stop: make(chan struct{}), done: make(chan struct{})}
	h.replays.bySerial["phone-a"] = rp
	ch, cancel := h.subscribeShared(ms)
	defer cancel()
	go h.replays.run(rp, media)

	sessions := 0
	configs := 0
	var lastPTS uint64
	seenSecondPass := false
	timeout := time.After(5 * time.Second)
	for {
		select {
		case f, ok := <-ch:
			if !ok {
				t.Fatal("loop replay channel closed before second pass")
			}
			if !f.replayed {
				t.Fatal("loop frame missing replayed flag")
			}
			switch f.data[0] {
			case byte(scrcpy.FrameSession):
				sessions++
				if sessions >= 2 {
					seenSecondPass = true
				}
			case byte(scrcpy.FrameConfig):
				configs++
			case byte(scrcpy.FrameKey), byte(scrcpy.FrameDelta):
				pts := binary.BigEndian.Uint64(f.data[1:9])
				if pts < lastPTS {
					// PTS must never go backwards, including across the loop seam.
					t.Fatalf("PTS went backwards across loop: %d < %d", pts, lastPTS)
				}
				lastPTS = pts
			}
			if seenSecondPass && configs >= 2 && rp.completedLoops() >= 1 {
				v := rp.view()
				if !v.Loop || v.LoopCount < 1 {
					t.Fatalf("view = %+v, want loop=true loop_count>=1", v)
				}
				rp.stopOnce.Do(func() { close(rp.stop) })
				<-rp.done
				return
			}
		case <-timeout:
			t.Fatalf("second pass not seen: sessions=%d configs=%d loopCount=%d", sessions, configs, rp.completedLoops())
		}
	}
}

func TestReplayLoopStaysAliveUntilStop(t *testing.T) {
	dir := t.TempDir()
	mp4Path := filepath.Join(dir, "rec_replay.mp4")
	_, mux := replayTestHub(t, mp4Path, recordingComplete)
	start := serveReplay(t, mux, http.MethodPost, "/api/replay/start?serial=phone-a", `{"recording_id":"rec_replay","loop":true}`)
	if start.Code != http.StatusAccepted || !strings.Contains(start.Body.String(), `"loop":true`) {
		t.Fatalf("loop start status=%d body=%s", start.Code, start.Body.String())
	}
	// Fixture drains in ~2ms; loop must still be active well after that.
	time.Sleep(50 * time.Millisecond)
	get := serveReplay(t, mux, http.MethodGet, "/api/replay?serial=phone-a", "")
	if get.Code != http.StatusOK || !strings.Contains(get.Body.String(), `"loop":true`) {
		t.Fatalf("loop get status=%d body=%s", get.Code, get.Body.String())
	}
	if !strings.Contains(get.Body.String(), `"loop_count"`) {
		t.Fatalf("get should carry loop_count: %s", get.Body.String())
	}
	if w := serveReplay(t, mux, http.MethodPost, "/api/replay/stop?serial=phone-a", ""); w.Code != http.StatusAccepted {
		t.Fatalf("loop stop status=%d body=%s", w.Code, w.Body.String())
	}
	waitForReplayGone(t, mux)
	if w := serveReplay(t, mux, http.MethodPost, "/api/replay/stop?serial=phone-a", ""); w.Code != http.StatusNotFound {
		t.Fatalf("second stop status=%d, want 404", w.Code)
	}
}
func TestReplayedFramesFanOut(t *testing.T) {
	path := filepath.Join(t.TempDir(), "replay.mp4")
	buildReplayFixtureMP4(t, path)
	media, err := demuxRecordingMP4(path)
	if err != nil {
		t.Fatal(err)
	}
	h := &Hub{}
	h.replays = newReplayManager(h)
	meta := &handshakeMeta{Type: "meta", Codec: "h264", Width: media.width, Height: media.height, Serial: "phone-a", SessionID: "test"}
	ms := &managedSession{replay: true, meta: meta, refs: 1, subs: make(map[chan sharedVideoFrame]struct{}), audioSubs: make(map[chan sharedAudioPacket]struct{}), width: media.width, height: media.height}
	ms.actions = action.New(deviceActionExecutor{ms: ms}, 64)
	rp := &replay{id: "rp_test", serial: "phone-a", recordingID: "rec_replay", startedAt: time.Now(), ms: ms, stop: make(chan struct{}), done: make(chan struct{})}
	h.replays.bySerial["phone-a"] = rp
	ch, cancel := h.subscribeShared(ms)
	defer cancel()
	go h.replays.run(rp, media)
	<-rp.done
	var kinds []byte
	var lastPTS uint64
	frames := 0
	timeout := time.After(5 * time.Second)
	for {
		select {
		case f, ok := <-ch:
			if !ok {
				if frames < 5 { // session + config + 3 samples
					t.Fatalf("only %d frames before close", frames)
				}
				if kinds[0] != byte(scrcpy.FrameSession) || kinds[1] != byte(scrcpy.FrameConfig) || kinds[2] != byte(scrcpy.FrameKey) {
					t.Fatalf("frame order = %v", kinds)
				}
				return
			}
			if !f.replayed {
				t.Fatalf("frame %d missing replayed flag", frames)
			}
			kinds = append(kinds, f.data[0])
			if f.data[0] == byte(scrcpy.FrameKey) || f.data[0] == byte(scrcpy.FrameDelta) {
				pts := binary.BigEndian.Uint64(f.data[1:9])
				if frames > 3 && pts < lastPTS {
					t.Fatalf("PTS went backwards: %d < %d", pts, lastPTS)
				}
				lastPTS = pts
			}
			frames++
		case <-timeout:
			t.Fatal("replay fan-out timed out")
		}
	}
}
