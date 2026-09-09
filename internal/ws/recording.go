package ws

import (
	"crypto/rand"
	"encoding/binary"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"

	"mywebscrcpy/internal/scrcpy"
)

const (
	minRecordingDuration      = time.Minute
	maxRecordingDuration      = 8 * time.Hour
	defaultRecordingQuota     = int64(10 << 30) // 10 GiB
	defaultRecordingRetention = 7 * 24 * time.Hour
)

type recordingStatus string

const (
	recordingActive   recordingStatus = "recording"
	recordingStopping recordingStatus = "stopping"
	recordingComplete recordingStatus = "completed"
	recordingFailed   recordingStatus = "failed"
	recordingExpired  recordingStatus = "expired"
)

type recording struct {
	mu            sync.Mutex
	id            string
	serial        string
	status        recordingStatus
	startedAt     time.Time
	endedAt       time.Time
	maxDuration   time.Duration
	bytesWritten  int64
	failureReason string
	path          string
	partialPath   string
	stop          chan struct{}
	stopOnce      sync.Once
}

type recordingView struct {
	RecordingID   string          `json:"recording_id"`
	Serial        string          `json:"serial"`
	Status        recordingStatus `json:"status"`
	StartedAt     time.Time       `json:"started_at"`
	EndedAt       *time.Time      `json:"ended_at,omitempty"`
	MaxDurationMS int64           `json:"max_duration_ms"`
	BytesWritten  int64           `json:"bytes_written"`
	FailureReason string          `json:"failure_reason,omitempty"`
}

func (r *recording) view() recordingView {
	r.mu.Lock()
	defer r.mu.Unlock()
	v := recordingView{RecordingID: r.id, Serial: r.serial, Status: r.status, StartedAt: r.startedAt.UTC(), MaxDurationMS: r.maxDuration.Milliseconds(), BytesWritten: r.bytesWritten, FailureReason: r.failureReason}
	if !r.endedAt.IsZero() {
		ended := r.endedAt.UTC()
		v.EndedAt = &ended
	}
	return v
}

type recordingManager struct {
	hub       *Hub
	dir       string
	quota     int64
	retention time.Duration

	mu       sync.Mutex
	entries  map[string]*recording
	bySerial map[string]string
}

func newRecordingManager(h *Hub) *recordingManager {
	dir := recordingStorageDir()
	quota := envPositiveInt64("RECORDINGS_MAX_BYTES", defaultRecordingQuota)
	retention := time.Duration(envPositiveInt64("RECORDINGS_RETENTION_HOURS", int64(defaultRecordingRetention/time.Hour))) * time.Hour
	m := &recordingManager{hub: h, dir: dir, quota: quota, retention: retention, entries: make(map[string]*recording), bySerial: make(map[string]string)}
	m.removeIncompleteFiles()
	go m.cleanupLoop()
	return m
}

// recordingStorageDir keeps the default recording location writable when the
// service is launched by a supervisor without a working directory.
func recordingStorageDir() string {
	if dir := strings.TrimSpace(os.Getenv("RECORDINGS_DIR")); dir != "" {
		return dir
	}
	if cacheDir, err := os.UserCacheDir(); err == nil && cacheDir != "" {
		return filepath.Join(cacheDir, "mywebscrcpy", "recordings")
	}
	return filepath.Join(os.TempDir(), "mywebscrcpy-recordings")
}

func envPositiveInt64(name string, fallback int64) int64 {
	v, err := strconv.ParseInt(os.Getenv(name), 10, 64)
	if err != nil || v <= 0 {
		return fallback
	}
	return v
}

func newRecordingID() string {
	var b [12]byte
	if _, err := rand.Read(b[:]); err == nil {
		return "rec_" + hex.EncodeToString(b[:])
	}
	return fmt.Sprintf("rec_%d", time.Now().UnixNano())
}

func (m *recordingManager) start(serial string, duration time.Duration) (*recording, error) {
	if serial == "" {
		return nil, badRecordingRequest("missing serial")
	}
	if duration < minRecordingDuration || duration > maxRecordingDuration {
		return nil, badRecordingRequest("max_duration_ms must be between 60000 and 28800000")
	}
	if err := os.MkdirAll(m.dir, 0o750); err != nil {
		return nil, recordingServiceUnavailable("recording directory unavailable: " + err.Error())
	}
	m.cleanupExpired(time.Now())

	rec := &recording{id: newRecordingID(), serial: serial, status: recordingActive, startedAt: time.Now(), maxDuration: duration, stop: make(chan struct{})}
	m.mu.Lock()
	if _, exists := m.bySerial[serial]; exists {
		m.mu.Unlock()
		return nil, recordingConflict("recording already active for this device")
	}
	m.entries[rec.id] = rec
	m.bySerial[serial] = rec.id
	m.mu.Unlock()

	ms, meta, release, err := m.hub.acquireSession(serial)
	if err != nil {
		m.finish(rec, recordingFailed, "session unavailable: "+err.Error(), false)
		return nil, recordingServiceUnavailable("device session unavailable")
	}
	if meta.Codec != "h264" {
		release()
		m.finish(rec, recordingFailed, "unsupported codec: "+meta.Codec, false)
		return nil, recordingServiceUnavailable("recording currently supports h264 shared streams")
	}
	rec.path = filepath.Join(m.dir, rec.id+".mp4")
	rec.partialPath = rec.path + ".partial"
	go m.run(rec, ms, release)
	return rec, nil
}

func (m *recordingManager) run(rec *recording, ms *managedSession, release func()) {
	defer release()
	frames, cancelFrames := m.hub.subscribeShared(ms)
	defer cancelFrames()
	muxer, err := newMP4RecordingMuxer(rec.partialPath)
	if err != nil {
		m.finish(rec, recordingFailed, "mp4 muxer start failed: "+err.Error(), false)
		return
	}

	timer := time.NewTimer(rec.maxDuration)
	defer timer.Stop()
	gotKey := false
	var runErr error
	for runErr == nil {
		select {
		case <-rec.stop:
			runErr = nil
			goto finalize
		case <-timer.C:
			goto finalize
		case frame, ok := <-frames:
			if !ok {
				runErr = errors.New("shared video session ended")
				break
			}
			if len(frame.data) < 9 {
				continue
			}
			kind := scrcpy.FrameKind(frame.data[0])
			if kind == scrcpy.FrameSession {
				if gotKey {
					runErr = errors.New("video stream reconfigured during recording")
				}
				continue
			}
			if kind != scrcpy.FrameConfig && kind != scrcpy.FrameKey && kind != scrcpy.FrameDelta {
				continue
			}
			if kind == scrcpy.FrameDelta && !gotKey {
				continue
			}
			pts := binary.BigEndian.Uint64(frame.data[1:9])
			if err := muxer.Write(kind, pts, frame.data[9:], time.Now()); err != nil {
				runErr = fmt.Errorf("mp4 muxer write failed: %w", err)
				break
			}
			if kind == scrcpy.FrameKey {
				gotKey = true
			}
			if err := m.checkQuota(rec); err != nil {
				runErr = err
			}
		}
	}

finalize:
	closeErr := muxer.Close(time.Now())
	if runErr != nil {
		m.finish(rec, recordingFailed, runErr.Error(), false)
		return
	}
	if closeErr != nil {
		m.finish(rec, recordingFailed, "mp4 muxer failed: "+closeErr.Error(), false)
		return
	}
	info, err := os.Stat(rec.partialPath)
	if err != nil || info.Size() == 0 {
		m.finish(rec, recordingFailed, "mp4 muxer did not create an output file", false)
		return
	}
	if err := m.checkQuota(rec); err != nil {
		m.finish(rec, recordingFailed, err.Error(), false)
		return
	}
	if err := os.Rename(rec.partialPath, rec.path); err != nil {
		m.finish(rec, recordingFailed, "publish recording failed: "+err.Error(), false)
		return
	}
	m.finish(rec, recordingComplete, "", true)
}

func (m *recordingManager) checkQuota(rec *recording) error {
	used, err := directorySize(m.dir)
	if err != nil {
		return fmt.Errorf("recording storage unavailable: %w", err)
	}
	if used > m.quota {
		return errors.New("recording storage quota exceeded")
	}
	if info, err := os.Stat(rec.partialPath); err == nil {
		rec.mu.Lock()
		rec.bytesWritten = info.Size()
		rec.mu.Unlock()
	}
	return nil
}

func directorySize(dir string) (int64, error) {
	var size int64
	err := filepath.WalkDir(dir, func(_ string, entry os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if entry.Type().IsRegular() {
			info, err := entry.Info()
			if err != nil {
				return err
			}
			size += info.Size()
		}
		return nil
	})
	return size, err
}

func (m *recordingManager) finish(rec *recording, status recordingStatus, reason string, keepFile bool) {
	if !keepFile {
		_ = os.Remove(rec.partialPath)
		_ = os.Remove(rec.path)
	}
	rec.mu.Lock()
	rec.status = status
	rec.failureReason = reason
	rec.endedAt = time.Now()
	rec.mu.Unlock()
	m.mu.Lock()
	if m.bySerial[rec.serial] == rec.id {
		delete(m.bySerial, rec.serial)
	}
	m.mu.Unlock()
}

func (m *recordingManager) stopRecording(id, serial string) (*recording, error) {
	rec, err := m.get(id, serial)
	if err != nil {
		return nil, err
	}
	rec.mu.Lock()
	if rec.status == recordingActive {
		rec.status = recordingStopping
		rec.stopOnce.Do(func() { close(rec.stop) })
	}
	rec.mu.Unlock()
	return rec, nil
}

func (m *recordingManager) get(id, serial string) (*recording, error) {
	m.mu.Lock()
	rec := m.entries[id]
	m.mu.Unlock()
	if rec == nil {
		return nil, recordingNotFound("recording not found")
	}
	if serial == "" || rec.serial != serial {
		return nil, badRecordingRequest("serial does not match recording")
	}
	return rec, nil
}

func (m *recordingManager) list(serial string) ([]recordingView, error) {
	if serial == "" {
		return nil, badRecordingRequest("missing serial")
	}
	m.mu.Lock()
	entries := make([]*recording, 0, len(m.entries))
	for _, rec := range m.entries {
		if rec.serial == serial {
			entries = append(entries, rec)
		}
	}
	m.mu.Unlock()
	views := make([]recordingView, 0, len(entries))
	for _, rec := range entries {
		views = append(views, rec.view())
	}
	sort.Slice(views, func(i, j int) bool { return views[i].StartedAt.After(views[j].StartedAt) })
	return views, nil
}

func (m *recordingManager) deleteRecording(id, serial string) error {
	rec, err := m.get(id, serial)
	if err != nil {
		return err
	}
	rec.mu.Lock()
	status, path := rec.status, rec.path
	rec.mu.Unlock()
	if status == recordingActive || status == recordingStopping {
		return recordingConflict("recording is still active")
	}
	if path != "" {
		if err := os.Remove(path); err != nil && !os.IsNotExist(err) {
			return recordingServiceUnavailable("recording file cannot be deleted")
		}
	}
	m.mu.Lock()
	if m.entries[id] == rec {
		delete(m.entries, id)
	}
	m.mu.Unlock()
	return nil
}

func (m *recordingManager) latestCompleted(serial string) (*recording, error) {
	entries, err := m.list(serial)
	if err != nil {
		return nil, err
	}
	for _, entry := range entries {
		if entry.Status == recordingComplete {
			return m.get(entry.RecordingID, serial)
		}
	}
	return nil, recordingNotFound("no completed recording found")
}

func (m *recordingManager) cleanupExpired(now time.Time) {
	m.mu.Lock()
	entries := make([]*recording, 0, len(m.entries))
	for _, rec := range m.entries {
		entries = append(entries, rec)
	}
	m.mu.Unlock()
	for _, rec := range entries {
		rec.mu.Lock()
		expire := rec.status == recordingComplete && !rec.endedAt.IsZero() && now.Sub(rec.endedAt) >= m.retention
		path := rec.path
		if expire {
			rec.status = recordingExpired
		}
		rec.mu.Unlock()
		if expire {
			_ = os.Remove(path)
		}
	}
}

func (m *recordingManager) cleanupLoop() {
	ticker := time.NewTicker(time.Hour)
	defer ticker.Stop()
	for now := range ticker.C {
		m.cleanupExpired(now)
	}
}

func (m *recordingManager) removeIncompleteFiles() {
	entries, err := os.ReadDir(m.dir)
	if err != nil {
		return
	}
	for _, entry := range entries {
		if entry.Type().IsRegular() && strings.HasSuffix(entry.Name(), ".mp4.partial") {
			_ = os.Remove(filepath.Join(m.dir, entry.Name()))
		}
	}
}

type recordingHTTPError struct {
	status int
	code   string
	msg    string
}

func (e *recordingHTTPError) Error() string { return e.msg }

func badRecordingRequest(msg string) error {
	return &recordingHTTPError{http.StatusBadRequest, "invalid_request", msg}
}
func recordingConflict(msg string) error {
	return &recordingHTTPError{http.StatusConflict, "recording_conflict", msg}
}
func recordingNotFound(msg string) error {
	return &recordingHTTPError{http.StatusNotFound, "recording_not_found", msg}
}
func recordingServiceUnavailable(msg string) error {
	return &recordingHTTPError{http.StatusServiceUnavailable, "recording_unavailable", msg}
}

func writeRecordingError(w http.ResponseWriter, err error) {
	var httpErr *recordingHTTPError
	if !errors.As(err, &httpErr) {
		httpErr = &recordingHTTPError{status: http.StatusInternalServerError, code: "recording_failed", msg: "recording request failed"}
	}
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(httpErr.status)
	_ = json.NewEncoder(w).Encode(map[string]string{"error": httpErr.code, "message": httpErr.msg})
}

func (h *Hub) RegisterRecordingRoutes(mux *http.ServeMux) {
	mux.HandleFunc("POST /api/recordings", h.startRecording)
	mux.HandleFunc("GET /api/recordings", h.listRecordings)
	mux.HandleFunc("GET /api/recordings/{recording_id}", h.getRecording)
	mux.HandleFunc("POST /api/recordings/{recording_id}/stop", h.stopRecording)
	mux.HandleFunc("GET /api/recordings/{recording_id}/download", h.downloadRecording)
	mux.HandleFunc("GET /api/recordings/download", h.downloadLatestRecording)
	mux.HandleFunc("DELETE /api/recordings/{recording_id}", h.deleteRecording)
}

func (h *Hub) startRecording(w http.ResponseWriter, r *http.Request) {
	serial := r.URL.Query().Get("serial")
	var body struct {
		MaxDurationMS *int64 `json:"max_duration_ms"`
	}
	decoder := json.NewDecoder(io.LimitReader(r.Body, 4096))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&body); err != nil || body.MaxDurationMS == nil {
		writeRecordingError(w, badRecordingRequest("max_duration_ms is required"))
		return
	}
	rec, err := h.recordings.start(serial, time.Duration(*body.MaxDurationMS)*time.Millisecond)
	if err != nil {
		log.Printf("[recording] start failed serial=%s: %v", serial, err)
		writeRecordingError(w, err)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusAccepted)
	_ = json.NewEncoder(w).Encode(rec.view())
}

func (h *Hub) getRecording(w http.ResponseWriter, r *http.Request) {
	rec, err := h.recordings.get(r.PathValue("recording_id"), r.URL.Query().Get("serial"))
	if err != nil {
		writeRecordingError(w, err)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(rec.view())
}

func (h *Hub) listRecordings(w http.ResponseWriter, r *http.Request) {
	entries, err := h.recordings.list(r.URL.Query().Get("serial"))
	if err != nil {
		writeRecordingError(w, err)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]any{"recordings": entries})
}

func (h *Hub) stopRecording(w http.ResponseWriter, r *http.Request) {
	rec, err := h.recordings.stopRecording(r.PathValue("recording_id"), r.URL.Query().Get("serial"))
	if err != nil {
		writeRecordingError(w, err)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusAccepted)
	_ = json.NewEncoder(w).Encode(rec.view())
}

func (h *Hub) downloadRecording(w http.ResponseWriter, r *http.Request) {
	rec, err := h.recordings.get(r.PathValue("recording_id"), r.URL.Query().Get("serial"))
	if err != nil {
		writeRecordingError(w, err)
		return
	}
	h.serveRecordingDownload(w, r, rec)
}

func (h *Hub) downloadLatestRecording(w http.ResponseWriter, r *http.Request) {
	serial := r.URL.Query().Get("serial")
	id := r.URL.Query().Get("recording_id")
	var (
		rec *recording
		err error
	)
	if id != "" {
		rec, err = h.recordings.get(id, serial)
	} else {
		rec, err = h.recordings.latestCompleted(serial)
	}
	if err != nil {
		writeRecordingError(w, err)
		return
	}
	h.serveRecordingDownload(w, r, rec)
}

func (h *Hub) serveRecordingDownload(w http.ResponseWriter, r *http.Request, rec *recording) {
	rec.mu.Lock()
	status, path, ended := rec.status, rec.path, rec.endedAt
	rec.mu.Unlock()
	if status == recordingExpired {
		writeRecordingError(w, &recordingHTTPError{status: http.StatusGone, code: "recording_expired", msg: "recording has expired"})
		return
	}
	if status != recordingComplete {
		writeRecordingError(w, &recordingHTTPError{status: http.StatusConflict, code: "recording_not_completed", msg: "recording is not ready for download"})
		return
	}
	f, err := os.Open(path)
	if err != nil {
		writeRecordingError(w, &recordingHTTPError{status: http.StatusGone, code: "recording_expired", msg: "recording file is unavailable"})
		return
	}
	defer f.Close()
	if _, err := f.Stat(); err != nil {
		writeRecordingError(w, err)
		return
	}
	w.Header().Set("Content-Type", "video/mp4")
	w.Header().Set("Content-Disposition", fmt.Sprintf("attachment; filename=%q", rec.id+".mp4"))
	http.ServeContent(w, r, rec.id+".mp4", ended, f)
}

func (h *Hub) deleteRecording(w http.ResponseWriter, r *http.Request) {
	if err := h.recordings.deleteRecording(r.PathValue("recording_id"), r.URL.Query().Get("serial")); err != nil {
		writeRecordingError(w, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// avccToAnnexB normalizes either 4-byte-length-prefixed NAL units or Annex-B
// input. ffmpeg's raw H.264 demuxer accepts Annex-B only.
func avccToAnnexB(data []byte) ([]byte, error) {
	if len(data) == 0 {
		return nil, nil
	}
	if isAnnexB(data) {
		return append([]byte(nil), data...), nil
	}
	var out []byte
	for len(data) > 0 {
		if len(data) < 4 {
			return nil, errors.New("truncated nal length")
		}
		n := int(binary.BigEndian.Uint32(data[:4]))
		data = data[4:]
		if n <= 0 || n > len(data) {
			return nil, errors.New("invalid nal length")
		}
		out = append(out, 0, 0, 0, 1)
		out = append(out, data[:n]...)
		data = data[n:]
	}
	return out, nil
}

func isAnnexB(data []byte) bool {
	return len(data) >= 4 && data[0] == 0 && data[1] == 0 && (data[2] == 1 || (data[2] == 0 && data[3] == 1))
}
