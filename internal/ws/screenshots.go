package ws

import (
	"bytes"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"image/png"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"
)

const (
	maxScreenshotBytes      = 20 << 20 // 20MB
	defaultScreenshotsQuota = int64(2 << 30)
)

var pngMagic = []byte{0x89, 0x50, 0x4E, 0x47, 0x0D, 0x0A, 0x1A, 0x0A}

type screenshotEntry struct {
	ID        string    `json:"screenshot_id"`
	Serial    string    `json:"serial"`
	CreatedAt time.Time `json:"created_at"`
	Width     int       `json:"width"`
	Height    int       `json:"height"`
	Bytes     int64     `json:"bytes"`
	Path      string    `json:"-"`
}

type screenshotManager struct {
	dir   string
	quota int64

	mu      sync.Mutex
	entries map[string]*screenshotEntry
}

var screenshotManagers sync.Map // *Hub -> *screenshotManager

func screenshotStorageDir() string {
	if dir := strings.TrimSpace(os.Getenv("SCREENSHOTS_DIR")); dir != "" {
		return dir
	}
	if cacheDir, err := os.UserCacheDir(); err == nil && cacheDir != "" {
		return filepath.Join(cacheDir, "mywebscrcpy", "screenshots")
	}
	return filepath.Join(os.TempDir(), "mywebscrcpy-screenshots")
}

func sanitizeSerial(serial string) string {
	var b strings.Builder
	for _, r := range serial {
		if (r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z') || (r >= '0' && r <= '9') || r == '-' || r == '_' || r == '.' {
			b.WriteRune(r)
		} else {
			b.WriteRune('_')
		}
	}
	s := b.String()
	if s == "" {
		return "unknown"
	}
	if len(s) > 64 {
		return s[:64]
	}
	return s
}

func newScreenshotID() string {
	var b [12]byte
	if _, err := rand.Read(b[:]); err == nil {
		return "shot_" + hex.EncodeToString(b[:])
	}
	return fmt.Sprintf("shot_%d", time.Now().UnixNano())
}

func newScreenshotManager(dir string, quota int64) *screenshotManager {
	m := &screenshotManager{dir: dir, quota: quota, entries: make(map[string]*screenshotEntry)}
	m.scan()
	return m
}

func (m *screenshotManager) scan() {
	entries, err := os.ReadDir(m.dir)
	if err != nil {
		return
	}
	for _, e := range entries {
		name := e.Name()
		if !strings.HasSuffix(name, ".png") || e.IsDir() {
			continue
		}
		id := strings.TrimSuffix(filepath.Base(name), ".png")
		// Filename is <safeSerial>__<id>.png, recover id part.
		if idx := strings.LastIndex(id, "__"); idx >= 0 {
			id = id[idx+2:]
		}
		if !strings.HasPrefix(id, "shot_") {
			continue
		}
		full := filepath.Join(m.dir, name)
		metaPath := filepath.Join(m.dir, id+".json")
		var ent screenshotEntry
		if raw, err := os.ReadFile(metaPath); err == nil {
			if json.Unmarshal(raw, &ent) == nil && ent.ID == id {
				ent.Path = full
				if info, err := os.Stat(full); err == nil {
					ent.Bytes = info.Size()
				}
				m.entries[ent.ID] = &ent
				continue
			}
		}
		// Fallback without sidecar: best-effort rebuild.
		info, err := e.Info()
		if err != nil {
			continue
		}
		width, height := 0, 0
		if f, err := os.Open(full); err == nil {
			if cfg, err := png.DecodeConfig(f); err == nil {
				width, height = cfg.Width, cfg.Height
			}
			f.Close()
		}
		created := info.ModTime().UTC()
		m.entries[id] = &screenshotEntry{ID: id, CreatedAt: created, Width: width, Height: height, Bytes: info.Size(), Path: full}
	}
}

func (m *screenshotManager) get(id, serial string) (*screenshotEntry, error) {
	m.mu.Lock()
	ent := m.entries[id]
	m.mu.Unlock()
	if ent == nil {
		return nil, recordingNotFound("screenshot not found")
	}
	if serial == "" || ent.Serial != serial {
		return nil, badRecordingRequest("serial does not match screenshot")
	}
	return ent, nil
}

func (m *screenshotManager) list(serial string) ([]screenshotEntry, error) {
	if serial == "" {
		return nil, badRecordingRequest("missing serial")
	}
	m.mu.Lock()
	out := make([]screenshotEntry, 0)
	for _, ent := range m.entries {
		if ent.Serial == serial {
			out = append(out, *ent)
		}
	}
	m.mu.Unlock()
	sort.Slice(out, func(i, j int) bool { return out[i].CreatedAt.After(out[j].CreatedAt) })
	return out, nil
}

func (m *screenshotManager) save(serial string, data []byte) (*screenshotEntry, error) {
	if serial == "" {
		return nil, badRecordingRequest("missing serial")
	}
	if len(data) == 0 || len(data) < len(pngMagic) || !bytes.Equal(data[:8], pngMagic) {
		return nil, badRecordingRequest("invalid png data")
	}
	cfg, err := png.DecodeConfig(bytes.NewReader(data))
	if err != nil {
		return nil, badRecordingRequest("invalid png data")
	}
	if int64(len(data)) > m.quota {
		return nil, &recordingHTTPError{status: http.StatusRequestEntityTooLarge, code: "screenshot_quota_exceeded", msg: "截图文件过大，超出截图存储配额"}
	}
	if err := os.MkdirAll(m.dir, 0o750); err != nil {
		return nil, recordingServiceUnavailable("screenshot directory unavailable: " + err.Error())
	}
	used, err := directorySize(m.dir)
	if err != nil {
		used = 0
	}
	if used+int64(len(data)) > m.quota {
		return nil, &recordingHTTPError{status: http.StatusInsufficientStorage, code: "screenshot_quota_exceeded", msg: "截图存储空间不足，已超出配额，请先删除旧截图"}
	}
	id := newScreenshotID()
	now := time.Now().UTC()
	name := sanitizeSerial(serial) + "__" + id + ".png"
	path := filepath.Join(m.dir, name)
	if err := os.WriteFile(path, data, 0o600); err != nil {
		return nil, recordingServiceUnavailable("screenshot save failed: " + err.Error())
	}
	ent := &screenshotEntry{ID: id, Serial: serial, CreatedAt: now, Width: cfg.Width, Height: cfg.Height, Bytes: int64(len(data)), Path: path}
	meta, _ := json.Marshal(ent)
	_ = os.WriteFile(filepath.Join(m.dir, id+".json"), meta, 0o600)
	m.mu.Lock()
	m.entries[id] = ent
	m.mu.Unlock()
	return ent, nil
}

func (m *screenshotManager) delete(id, serial string) error {
	ent, err := m.get(id, serial)
	if err != nil {
		return err
	}
	_ = os.Remove(ent.Path)
	_ = os.Remove(filepath.Join(m.dir, ent.ID+".json"))
	m.mu.Lock()
	if m.entries[id] == ent {
		delete(m.entries, id)
	}
	m.mu.Unlock()
	return nil
}

func screenshotManagerFor(h *Hub) *screenshotManager {
	if v, ok := screenshotManagers.Load(h); ok {
		if m, ok := v.(*screenshotManager); ok {
			return m
		}
	}
	dir := screenshotStorageDir()
	quota := envPositiveInt64("SCREENSHOTS_MAX_BYTES", defaultScreenshotsQuota)
	m := newScreenshotManager(dir, quota)
	actual, _ := screenshotManagers.LoadOrStore(h, m)
	return actual.(*screenshotManager)
}

// RegisterScreenshotRoutes registers screenshot capture storage routes.
func (h *Hub) RegisterScreenshotRoutes(mux *http.ServeMux) {
	screenshotManagerFor(h)
	mux.HandleFunc("POST /api/screenshots", h.uploadScreenshot)
	mux.HandleFunc("GET /api/screenshots", h.listScreenshots)
	mux.HandleFunc("GET /api/screenshots/{screenshot_id}", h.getScreenshot)
	mux.HandleFunc("DELETE /api/screenshots/{screenshot_id}", h.deleteScreenshot)
}

func (h *Hub) uploadScreenshot(w http.ResponseWriter, r *http.Request) {
	serial := r.URL.Query().Get("serial")
	m := screenshotManagerFor(h)
	data, err := io.ReadAll(io.LimitReader(r.Body, maxScreenshotBytes+1))
	if err != nil {
		writeRecordingError(w, badRecordingRequest("read screenshot body failed"))
		return
	}
	if int64(len(data)) > maxScreenshotBytes {
		writeRecordingError(w, &recordingHTTPError{status: http.StatusRequestEntityTooLarge, code: "screenshot_too_large", msg: "截图文件过大，最大支持 20MB"})
		return
	}
	ent, err := m.save(serial, data)
	if err != nil {
		writeRecordingError(w, err)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	_ = json.NewEncoder(w).Encode(ent)
}

func (h *Hub) listScreenshots(w http.ResponseWriter, r *http.Request) {
	m := screenshotManagerFor(h)
	entries, err := m.list(r.URL.Query().Get("serial"))
	if err != nil {
		writeRecordingError(w, err)
		return
	}
	if entries == nil {
		entries = []screenshotEntry{}
	}
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]any{"screenshots": entries})
}

func (h *Hub) getScreenshot(w http.ResponseWriter, r *http.Request) {
	m := screenshotManagerFor(h)
	ent, err := m.get(r.PathValue("screenshot_id"), r.URL.Query().Get("serial"))
	if err != nil {
		writeRecordingError(w, err)
		return
	}
	f, err := os.Open(ent.Path)
	if err != nil {
		writeRecordingError(w, recordingNotFound("screenshot not found"))
		return
	}
	defer f.Close()
	info, err := f.Stat()
	if err != nil {
		writeRecordingError(w, err)
		return
	}
	w.Header().Set("Content-Type", "image/png")
	if r.URL.Query().Get("download") == "1" {
		w.Header().Set("Content-Disposition", fmt.Sprintf("attachment; filename=%q", ent.ID+".png"))
	}
	http.ServeContent(w, r, ent.ID+".png", info.ModTime(), f)
}

func (h *Hub) deleteScreenshot(w http.ResponseWriter, r *http.Request) {
	m := screenshotManagerFor(h)
	if err := m.delete(r.PathValue("screenshot_id"), r.URL.Query().Get("serial")); err != nil {
		writeRecordingError(w, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}
