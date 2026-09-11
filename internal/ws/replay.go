package ws

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"sort"
	"sync"
	"time"

	"github.com/Eyevinn/mp4ff/mp4"
	"mywebscrcpy/internal/action"
	"mywebscrcpy/internal/scrcpy"
)

// replayMedia is a demuxed recording: H.264 only, PTS in microseconds.
type replaySample struct {
	key  bool
	pts  uint64 // µs, matches scrcpy PTS timescale
	data []byte // Annex-B
}

type replayMedia struct {
	width, height uint32
	config        []byte // Annex-B SPS/PPS
	samples       []replaySample
}

type replay struct {
	id          string
	serial      string
	recordingID string
	startedAt   time.Time
	loop        bool
	loopMu      sync.Mutex
	loopCount   int // completed passes, from 0
	ms          *managedSession
	stop        chan struct{}
	stopOnce    sync.Once
	done        chan struct{}

	paused      bool
	pauseMu     sync.Mutex
	pauseAccum  time.Duration // 已完成暂停的累计墙钟时长
	pausedSince time.Time     // 当前暂停的起点；未暂停时为零值
	pauseNotify chan struct{} // 每次暂停状态变化时投递一次，用于唤醒节拍等待

	media  *replayMedia
	seek   chan int // 缓冲1：待跳转的样本下标
	seekMu sync.Mutex
}

func (r *replay) isLoop() bool { return r.loop }

func (r *replay) completedLoops() int {
	r.loopMu.Lock()
	defer r.loopMu.Unlock()
	return r.loopCount
}

func (r *replay) incLoopCount() int {
	r.loopMu.Lock()
	defer r.loopMu.Unlock()
	r.loopCount++
	return r.loopCount
}

func (r *replay) isPaused() bool {
	r.pauseMu.Lock()
	defer r.pauseMu.Unlock()
	return r.paused
}

// pausedDuration 返回已完成暂停的累计时长（当前正在进行的暂停不计入）
func (r *replay) pausedDuration() time.Duration {
	r.pauseMu.Lock()
	defer r.pauseMu.Unlock()
	return r.pauseAccum
}

// setPaused 设置暂停状态；返回是否发生了状态变化。
func (r *replay) setPaused(p bool) bool {
	r.pauseMu.Lock()
	if r.paused == p {
		r.pauseMu.Unlock()
		return false
	}
	r.paused = p
	if p {
		r.pausedSince = time.Now()
	} else {
		if !r.pausedSince.IsZero() {
			r.pauseAccum += time.Since(r.pausedSince)
		}
		r.pausedSince = time.Time{}
	}
	r.pauseMu.Unlock()
	select {
	case r.pauseNotify <- struct{}{}:
	default:
	}
	return true
}

// waitResume 阻塞直到恢复或停止；返回 false 表示应退出整个 run 循环（停止）。
func (r *replay) waitResume() bool {
	for r.isPaused() {
		select {
		case <-r.stop:
			return false
		case <-r.pauseNotify:
		}
	}
	return true
}

// resetPausedAccum 节拍重校准后重置暂停累计
func (r *replay) resetPausedAccum() {
	r.pauseMu.Lock()
	r.pauseAccum = 0
	if r.paused {
		r.pausedSince = time.Now()
	}
	r.pauseMu.Unlock()
}

// seekToElapsed 按毫秒跳转，落到关键帧后投递
func (r *replay) seekToElapsed(ms int64) {
	r.seekMu.Lock()
	defer r.seekMu.Unlock()
	if r.media == nil || len(r.media.samples) == 0 {
		return
	}
	samples := r.media.samples
	first := samples[0].pts
	last := samples[len(samples)-1].pts
	span := uint64(0)
	if last >= first {
		span = last - first
	}
	var targetPTS uint64
	switch {
	case ms <= 0:
		targetPTS = first
	case span == 0 || uint64(ms*1000) >= span:
		targetPTS = last
	default:
		targetPTS = first + uint64(ms*1000)
	}
	idx := sort.Search(len(samples), func(i int) bool { return samples[i].pts >= targetPTS })
	if idx >= len(samples) {
		idx = len(samples) - 1
	}
	// 回退到最近的关键帧，保证 reframe 后能立即解码
	for idx > 0 && !samples[idx].key {
		idx--
	}
	select {
	case r.seek <- idx:
	default:
	}
}

type replayView struct {
	ReplayID    string    `json:"replay_id"`
	Serial      string    `json:"serial"`
	RecordingID string    `json:"recording_id"`
	Status      string    `json:"status"`
	StartedAt   time.Time `json:"started_at"`
	Loop        bool      `json:"loop"`
	LoopCount   int       `json:"loop_count"`
	Paused      bool      `json:"paused"`
}

func (r *replay) view() replayView {
	return replayView{ReplayID: r.id, Serial: r.serial, RecordingID: r.recordingID, Status: "replaying", StartedAt: r.startedAt.UTC(), Loop: r.loop, LoopCount: r.completedLoops(), Paused: r.isPaused()}
}

type replayManager struct {
	hub *Hub
	mu  sync.Mutex
	// bySerial enforces at most one active replay per serial. Presence of an
	// entry means the serial is in REPLAYING state; absence means LIVE.
	bySerial map[string]*replay
}

func newReplayManager(h *Hub) *replayManager {
	return &replayManager{hub: h, bySerial: make(map[string]*replay)}
}

func (h *Hub) replayMgr() *replayManager {
	if h.replays == nil {
		h.replays = newReplayManager(h)
	}
	return h.replays
}

// acquire serves Hub.acquireSession while a replay is active: callers share
// the replay's managedSession and get replayed=true bootstrap frames.
func (m *replayManager) acquire(serial string) (*managedSession, *handshakeMeta, func(), bool) {
	m.mu.Lock()
	defer m.mu.Unlock()
	rp := m.bySerial[serial]
	if rp == nil {
		return nil, nil, nil, false
	}
	ms := rp.ms
	ms.mu.Lock()
	ms.refs++
	ms.mu.Unlock()
	return ms, ms.meta, func() { m.hub.releaseSession(rp.serial, ms) }, true
}

func (m *replayManager) get(serial string) *replay {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.bySerial[serial]
}

func (m *replayManager) start(serial, recordingID string, loop bool) (*replay, error) {
	if serial == "" {
		return nil, &recordingHTTPError{status: http.StatusBadRequest, code: "invalid_request", msg: "missing serial"}
	}
	if recordingID == "" {
		return nil, &recordingHTTPError{status: http.StatusBadRequest, code: "invalid_request", msg: "recording_id is required"}
	}
	if m.hub.recordings == nil {
		return nil, &recordingHTTPError{status: http.StatusServiceUnavailable, code: "recording_unavailable", msg: "recording service unavailable"}
	}
	m.mu.Lock()
	if _, exists := m.bySerial[serial]; exists {
		m.mu.Unlock()
		return nil, &recordingHTTPError{status: http.StatusConflict, code: "replay_conflict", msg: "replay already active for this device"}
	}
	m.mu.Unlock()

	rec, err := m.hub.recordings.get(recordingID, serial)
	if err != nil {
		return nil, err
	}
	rec.mu.Lock()
	status, path := rec.status, rec.path
	rec.mu.Unlock()
	if status != recordingComplete {
		return nil, &recordingHTTPError{status: http.StatusConflict, code: "recording_not_completed", msg: "recording is not ready for replay"}
	}
	media, err := demuxRecordingMP4(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, &recordingHTTPError{status: http.StatusGone, code: "recording_expired", msg: "recording file is unavailable"}
		}
		return nil, &recordingHTTPError{status: http.StatusBadRequest, code: "invalid_request", msg: "recording cannot be replayed: " + err.Error()}
	}

	// Release the live scrcpy session so existing /ws and vision subscribers
	// disconnect; their reconnect lands on the replay session below.
	m.hub.closeLiveSession(serial)

	meta := &handshakeMeta{Type: "meta", Codec: "h264", Width: media.width, Height: media.height, Serial: serial, SessionID: newSessionID()}
	ms := &managedSession{replay: true, meta: meta, refs: 1, subs: make(map[chan sharedVideoFrame]struct{}), audioSubs: make(map[chan sharedAudioPacket]struct{}), width: media.width, height: media.height}
	ms.actions = action.New(deviceActionExecutor{ms: ms}, 64)
	rp := &replay{id: newRecordingID(), serial: serial, recordingID: recordingID, startedAt: time.Now(), loop: loop, ms: ms, stop: make(chan struct{}), done: make(chan struct{}), pauseNotify: make(chan struct{}, 1), media: media, seek: make(chan int, 1)}

	m.mu.Lock()
	if _, exists := m.bySerial[serial]; exists {
		m.mu.Unlock()
		return nil, &recordingHTTPError{status: http.StatusConflict, code: "replay_conflict", msg: "replay already active for this device"}
	}
	m.bySerial[serial] = rp
	m.mu.Unlock()
	go m.run(rp, media)
	return rp, nil
}

func (m *replayManager) stop(serial string) (*replay, error) {
	if serial == "" {
		return nil, &recordingHTTPError{status: http.StatusBadRequest, code: "invalid_request", msg: "missing serial"}
	}
	m.mu.Lock()
	rp := m.bySerial[serial]
	m.mu.Unlock()
	if rp == nil {
		return nil, &recordingHTTPError{status: http.StatusNotFound, code: "replay_not_found", msg: "no active replay for this device"}
	}
	rp.stopOnce.Do(func() { close(rp.stop) })
	return rp, nil
}

// run replays samples at 1x wall-clock pacing, then finishes so the next
// acquireSession rebuilds a live session. With loop=true it seamlessly
// restarts from the head (session/config reframed, PTS offset forward and
// wall-clock rebased so subscriber decoders continue) until stop.
func (m *replayManager) run(rp *replay, media *replayMedia) {
	defer m.finish(rp)
	ms := rp.ms
	sessionFrame := &scrcpy.Frame{Kind: scrcpy.FrameSession, Width: media.width, Height: media.height}
	configFrame := &scrcpy.Frame{Kind: scrcpy.FrameConfig, Payload: media.config}
	if len(media.samples) == 0 {
		if !rp.isLoop() {
			return
		}
		<-rp.stop
		return
	}
	// Gap between the last two samples keeps pacing/PTS monotonic across
	// loop boundaries. Single-sample media falls back to 40ms.
	var gap uint64 = 40_000
	if n := len(media.samples); n > 1 {
		if g := media.samples[n-1].pts - media.samples[n-2].pts; g > 0 {
			gap = g
		} else {
			gap = 1000
		}
	}
	firstPTS := media.samples[0].pts
	lastPTS := media.samples[len(media.samples)-1].pts
	span := uint64(0)
	if lastPTS >= firstPTS {
		span = lastPTS - firstPTS
	}
	var ptsOffset uint64
	startIdx := 0
reframe:
	for {
		if rp.isPaused() {
			if !rp.waitResume() {
				return
			}
		}
		select {
		case <-rp.stop:
			return
		default:
		}
		broadcastReplayFrame(ms, sessionFrame, encodeFrame(sessionFrame))
		broadcastReplayFrame(ms, configFrame, encodeFrame(configFrame))
		t0 := media.samples[startIdx].pts
		start := time.Now()
		rp.resetPausedAccum()
		for i := startIdx; i < len(media.samples); i++ {
			s := media.samples[i]
			select {
			case idx := <-rp.seek:
				startIdx = idx
				continue reframe
			default:
			}
			if i > 0 && s.pts >= t0 {
				for {
					wait := time.Duration(s.pts-t0)*time.Microsecond - (time.Since(start) - rp.pausedDuration())
					if wait <= 0 {
						break
					}
					if rp.isPaused() {
						if !rp.waitResume() {
							return
						}
						continue
					}
					select {
					case <-rp.stop:
						return
					case idx := <-rp.seek:
						startIdx = idx
						continue reframe
					case <-rp.pauseNotify:
					case <-time.After(wait):
					}
				}
			}
			if rp.isPaused() {
				if !rp.waitResume() {
					return
				}
			}
			select {
			case <-rp.stop:
				return
			default:
			}
			kind := scrcpy.FrameDelta
			if s.key {
				kind = scrcpy.FrameKey
			}
			f := &scrcpy.Frame{Kind: kind, PTS: s.pts + ptsOffset, Payload: s.data}
			broadcastReplayFrame(ms, f, encodeFrame(f))
		}
		if !rp.isLoop() {
			return
		}
		rp.incLoopCount()
		ptsOffset += span + gap
		startIdx = 0
	}
}

func (m *replayManager) finish(rp *replay) {
	m.mu.Lock()
	if m.bySerial[rp.serial] == rp {
		delete(m.bySerial, rp.serial)
	}
	m.mu.Unlock()
	ms := rp.ms
	ms.mu.Lock()
	for ch := range ms.subs {
		close(ch)
	}
	ms.subs = make(map[chan sharedVideoFrame]struct{})
	ms.mu.Unlock()
	ms.actions.Close()
	close(rp.done)
}

// broadcastReplayFrame mirrors cacheAndBroadcast but marks every frame
// replayed so vision/record consumers can tell replay traffic from live.
func broadcastReplayFrame(ms *managedSession, f *scrcpy.Frame, buf []byte) {
	ms.mu.Lock()
	defer ms.mu.Unlock()
	ms.frames++
	ms.lastFrame = time.Now()
	if f.Kind == scrcpy.FrameSession {
		ms.width, ms.height = f.Width, f.Height
		ms.lastConfig = nil
		ms.lastKey = nil
	}
	if f.Kind == scrcpy.FrameConfig {
		ms.lastConfig = append([]byte(nil), buf...)
	}
	if f.Kind == scrcpy.FrameKey {
		ms.lastKey = append([]byte(nil), buf...)
	}
	for ch := range ms.subs {
		select {
		case ch <- sharedVideoFrame{data: append([]byte(nil), buf...), replayed: true}:
		default:
			ms.dropped++
			select {
			case <-ch:
			default:
			}
			select {
			case ch <- sharedVideoFrame{data: append([]byte(nil), buf...), replayed: true}:
			default:
			}
		}
	}
}

// closeLiveSession drops the real scrcpy session for serial immediately.
// Existing subscribers see their channels close and reconnect onto the replay.
func (h *Hub) closeLiveSession(serial string) {
	h.sessionMu.Lock()
	ms := h.sessions[serial]
	if ms != nil {
		delete(h.sessions, serial)
	}
	h.sessionMu.Unlock()
	if ms == nil {
		return
	}
	ms.mu.Lock()
	for ch := range ms.subs {
		close(ch)
	}
	ms.subs = make(map[chan sharedVideoFrame]struct{})
	for ch := range ms.audioSubs {
		close(ch)
	}
	ms.audioSubs = make(map[chan sharedAudioPacket]struct{})
	ms.mu.Unlock()
	ms.actions.Close()
	ms.sess.close()
}

// demuxRecordingMP4 reads the muxer's fragmented MP4 back into shared-frame
// payloads: avcC supplies SPS/PPS, per-sample sync flags split key/delta and
// decodeTime (timescale 1e6) is the scrcpy PTS.
func demuxRecordingMP4(path string) (*replayMedia, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	file, err := mp4.DecodeFile(f)
	if err != nil {
		return nil, fmt.Errorf("decode mp4: %w", err)
	}
	if file.Init == nil || file.Init.Moov == nil {
		return nil, errors.New("missing mp4 init segment")
	}
	var videoTrak *mp4.TrakBox
	for _, trak := range file.Init.Moov.Traks {
		if trak.Mdia == nil || trak.Mdia.Hdlr == nil || trak.Mdia.Mdhd == nil || trak.Mdia.Minf == nil || trak.Mdia.Minf.Stbl == nil || trak.Mdia.Minf.Stbl.Stsd == nil {
			continue
		}
		if trak.Mdia.Hdlr.HandlerType != "vide" {
			continue
		}
		videoTrak = trak
		break
	}
	if videoTrak == nil {
		return nil, errors.New("no video track")
	}
	stsd := videoTrak.Mdia.Minf.Stbl.Stsd
	if stsd.AvcX == nil || stsd.AvcX.AvcC == nil {
		return nil, errors.New("recording is not h264")
	}
	avcC := stsd.AvcX.AvcC
	if len(avcC.SPSnalus) == 0 || len(avcC.PPSnalus) == 0 {
		return nil, errors.New("h264 config lacks SPS or PPS")
	}
	var config []byte
	for _, nalu := range append(append([][]byte{}, avcC.SPSnalus...), avcC.PPSnalus...) {
		config = append(config, 0, 0, 0, 1)
		config = append(config, nalu...)
	}
	width, height, err := h264ResolutionFromSPS(avcC.SPSnalus[0])
	if err != nil {
		width, height = uint32(stsd.AvcX.Width), uint32(stsd.AvcX.Height)
		if width == 0 || height == 0 {
			return nil, fmt.Errorf("resolve resolution: %w", err)
		}
	}
	timescale := uint64(videoTrak.Mdia.Mdhd.Timescale)
	if timescale == 0 {
		return nil, errors.New("invalid video timescale")
	}
	if videoTrak.Tkhd == nil {
		return nil, errors.New("missing video track header")
	}
	trackID := videoTrak.Tkhd.TrackID
	var samples []replaySample
	for _, seg := range file.Segments {
		for _, frag := range seg.Fragments {
			full, err := frag.GetFullSamples(&mp4.TrexBox{TrackID: trackID})
			if err != nil {
				return nil, fmt.Errorf("read mp4 samples: %w", err)
			}
			for _, s := range full {
				annexB, err := avccToAnnexB(s.Data)
				if err != nil {
					return nil, fmt.Errorf("invalid h264 sample: %w", err)
				}
				samples = append(samples, replaySample{key: mp4.IsSyncSampleFlags(s.Flags), pts: s.DecodeTime * 1_000_000 / timescale, data: annexB})
			}
		}
	}
	if len(samples) == 0 {
		return nil, errors.New("no video samples")
	}
	return &replayMedia{width: width, height: height, config: config, samples: samples}, nil
}

// h264ResolutionFromSPS parses an SPS NALU (without start code) for
// pic_width/height in map units, honoring frame cropping.
func h264ResolutionFromSPS(sps []byte) (uint32, uint32, error) {
	if len(sps) < 2 {
		return 0, 0, errors.New("sps too short")
	}
	br := &spsBitReader{data: unescapeRBSP(sps[1:])}
	readBits := func(n int) (uint, error) {
		var v uint
		for i := 0; i < n; i++ {
			b, err := br.readBit()
			if err != nil {
				return 0, err
			}
			v = v<<1 | uint(b)
		}
		return v, nil
	}
	readUE := func() (uint, error) {
		zeros := 0
		for {
			b, err := br.readBit()
			if err != nil {
				return 0, err
			}
			if b == 1 {
				break
			}
			zeros++
			if zeros > 31 {
				return 0, errors.New("invalid exp-golomb code")
			}
		}
		if zeros == 0 {
			return 0, nil
		}
		rest, err := readBits(zeros)
		if err != nil {
			return 0, err
		}
		return (1<<zeros - 1) + rest, nil
	}
	profile, err := readBits(8)
	if err != nil {
		return 0, 0, err
	}
	if _, err := readBits(16); err != nil { // constraints + level
		return 0, 0, err
	}
	if _, err := readUE(); err != nil { // sps id
		return 0, 0, err
	}
	chroma := uint(1)
	if profile == 100 || profile == 110 || profile == 122 || profile == 244 || profile == 44 || profile == 83 || profile == 86 || profile == 118 || profile == 128 {
		chroma, err = readUE()
		if err != nil {
			return 0, 0, err
		}
		if chroma == 3 {
			if _, err := readBits(1); err != nil {
				return 0, 0, err
			}
		}
		if _, err := readUE(); err != nil { // bit depth luma
			return 0, 0, err
		}
		if _, err := readUE(); err != nil { // bit depth chroma
			return 0, 0, err
		}
		if _, err := readBits(1); err != nil { // qpprime zero transform bypass
			return 0, 0, err
		}
		seqScaling, err := readBits(1)
		if err != nil {
			return 0, 0, err
		}
		if seqScaling == 1 {
			lists := 8
			if chroma == 3 {
				lists = 12
			}
			for i := 0; i < lists; i++ {
				flag, err := readBits(1)
				if err != nil {
					return 0, 0, err
				}
				if flag == 1 {
					size := 16
					if i >= 6 {
						size = 64
					}
					last, next := 8, 8
					for j := 0; j < size; j++ {
						if next != 0 {
							delta, err := readUE()
							if err != nil {
								return 0, 0, err
							}
							next = (last + int(delta) + 256) % 256
						}
						if next != 0 {
							last = next
						}
					}
				}
			}
		}
	}
	if _, err := readUE(); err != nil { // log2 max frame num
		return 0, 0, err
	}
	pocType, err := readUE()
	if err != nil {
		return 0, 0, err
	}
	if pocType == 0 {
		if _, err := readUE(); err != nil {
			return 0, 0, err
		}
	} else if pocType == 1 {
		if _, err := readBits(1); err != nil {
			return 0, 0, err
		}
		if _, err := readUE(); err != nil {
			return 0, 0, err
		}
		if _, err := readUE(); err != nil {
			return 0, 0, err
		}
		n, err := readUE()
		if err != nil {
			return 0, 0, err
		}
		for i := uint(0); i < n; i++ {
			if _, err := readUE(); err != nil {
				return 0, 0, err
			}
		}
	}
	if _, err := readUE(); err != nil { // num ref frames
		return 0, 0, err
	}
	if _, err := readBits(1); err != nil { // gaps flag
		return 0, 0, err
	}
	wMinus1, err := readUE()
	if err != nil {
		return 0, 0, err
	}
	hMinus1, err := readUE()
	if err != nil {
		return 0, 0, err
	}
	frameOnly, err := readBits(1)
	if err != nil {
		return 0, 0, err
	}
	if frameOnly == 0 {
		if _, err := readBits(1); err != nil {
			return 0, 0, err
		}
	}
	if _, err := readBits(1); err != nil { // direct 8x8 inference
		return 0, 0, err
	}
	cropFlag, err := readBits(1)
	if err != nil {
		return 0, 0, err
	}
	var cropLeft, cropRight, cropTop, cropBottom uint
	if cropFlag == 1 {
		if cropLeft, err = readUE(); err != nil {
			return 0, 0, err
		}
		if cropRight, err = readUE(); err != nil {
			return 0, 0, err
		}
		if cropTop, err = readUE(); err != nil {
			return 0, 0, err
		}
		if cropBottom, err = readUE(); err != nil {
			return 0, 0, err
		}
	}
	width := (wMinus1 + 1) * 16
	height := (hMinus1 + 1) * 16 * (2 - frameOnly)
	if chroma == 0 {
		width -= cropLeft + cropRight
		height -= (cropTop + cropBottom) * (2 - frameOnly)
	} else {
		subX, subY := uint(2), uint(2)
		if chroma == 2 {
			subY = 1
		}
		width -= (cropLeft + cropRight) * subX
		height -= (cropTop + cropBottom) * subY * (2 - frameOnly)
	}
	if width == 0 || height == 0 {
		return 0, 0, errors.New("invalid sps dimensions")
	}
	return uint32(width), uint32(height), nil
}

type spsBitReader struct {
	data []byte
	pos  int // bit position
}

func (b *spsBitReader) readBit() (uint, error) {
	if b.pos/8 >= len(b.data) {
		return 0, errors.New("sps truncated")
	}
	v := (b.data[b.pos/8] >> (7 - b.pos%8)) & 1
	b.pos++
	return uint(v), nil
}

func unescapeRBSP(data []byte) []byte {
	out := make([]byte, 0, len(data))
	for i := 0; i < len(data); i++ {
		if i+2 < len(data) && data[i] == 0 && data[i+1] == 0 && data[i+2] == 3 {
			out = append(out, 0, 0)
			i += 2
			continue
		}
		out = append(out, data[i])
	}
	return out
}

func (h *Hub) RegisterReplayRoutes(mux *http.ServeMux) {
	mux.HandleFunc("POST /api/replay/start", h.startReplay)
	mux.HandleFunc("POST /api/replay/stop", h.stopReplay)
	mux.HandleFunc("POST /api/replay/pause", h.pauseReplay)
	mux.HandleFunc("POST /api/replay/seek", h.seekReplay)
	mux.HandleFunc("GET /api/replay", h.getReplay)
}

func (h *Hub) startReplay(w http.ResponseWriter, r *http.Request) {
	serial := r.URL.Query().Get("serial")
	var body struct {
		RecordingID string `json:"recording_id"`
		Loop        *bool  `json:"loop"`
	}
	decoder := json.NewDecoder(io.LimitReader(r.Body, 4096))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&body); err != nil {
		writeRecordingError(w, &recordingHTTPError{status: http.StatusBadRequest, code: "invalid_request", msg: "recording_id is required"})
		return
	}
	loop := body.Loop != nil && *body.Loop
	rp, err := h.replayMgr().start(serial, body.RecordingID, loop)
	if err != nil {
		log.Printf("[replay] start failed serial=%s recording=%s: %v", serial, body.RecordingID, err)
		writeRecordingError(w, err)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusAccepted)
	_ = json.NewEncoder(w).Encode(rp.view())
}

func (h *Hub) stopReplay(w http.ResponseWriter, r *http.Request) {
	serial := r.URL.Query().Get("serial")
	rp, err := h.replayMgr().stop(serial)
	if err != nil {
		writeRecordingError(w, err)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusAccepted)
	_ = json.NewEncoder(w).Encode(rp.view())
}

func (h *Hub) pauseReplay(w http.ResponseWriter, r *http.Request) {
	serial := r.URL.Query().Get("serial")
	var body struct {
		Paused *bool `json:"paused"`
	}
	decoder := json.NewDecoder(io.LimitReader(r.Body, 4096))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&body); err != nil {
		writeRecordingError(w, &recordingHTTPError{status: http.StatusBadRequest, code: "invalid_request", msg: "paused is required"})
		return
	}
	rp := h.replayMgr().get(serial)
	if rp == nil {
		writeRecordingError(w, &recordingHTTPError{status: http.StatusNotFound, code: "replay_not_found", msg: "no active replay for this device"})
		return
	}
	if body.Paused != nil {
		rp.setPaused(*body.Paused)
	}
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusAccepted)
	_ = json.NewEncoder(w).Encode(rp.view())
}

func (h *Hub) seekReplay(w http.ResponseWriter, r *http.Request) {
	serial := r.URL.Query().Get("serial")
	var body struct {
		PositionMS *int64 `json:"position_ms"`
	}
	decoder := json.NewDecoder(io.LimitReader(r.Body, 4096))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&body); err != nil {
		writeRecordingError(w, &recordingHTTPError{status: http.StatusBadRequest, code: "invalid_request", msg: "position_ms is required"})
		return
	}
	rp := h.replayMgr().get(serial)
	if rp == nil {
		writeRecordingError(w, &recordingHTTPError{status: http.StatusNotFound, code: "replay_not_found", msg: "no active replay for this device"})
		return
	}
	if body.PositionMS == nil {
		writeRecordingError(w, &recordingHTTPError{status: http.StatusBadRequest, code: "invalid_request", msg: "position_ms is required"})
		return
	}
	// seek 语义：跳到该位置并继续播放（若暂停则解除暂停），保证 waitResume 阻塞的 goroutine 能醒来消费 seek
	rp.setPaused(false)
	rp.seekToElapsed(*body.PositionMS)
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusAccepted)
	_ = json.NewEncoder(w).Encode(rp.view())
}

func (h *Hub) getReplay(w http.ResponseWriter, r *http.Request) {
	serial := r.URL.Query().Get("serial")
	if serial == "" {
		writeRecordingError(w, &recordingHTTPError{status: http.StatusBadRequest, code: "invalid_request", msg: "missing serial"})
		return
	}
	var view *replayView
	if rp := h.replayMgr().get(serial); rp != nil {
		v := rp.view()
		view = &v
	}
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]any{"replay": view})
}
