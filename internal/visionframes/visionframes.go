// Package visionframes 缓存浏览器投屏解码后的最新画面帧，供设备总状态等
// 无解码能力的 Go 侧消费方按 serial 取用，替代 adb screencap 截图。
package visionframes

import (
	"bytes"
	"encoding/json"
	"image/jpeg"
	"log"
	"net/http"
	"sync"
	"time"
)

const (
	// MaxFrameBytes 单帧 JPEG 上限，448×1024 质量帧远小于该值，超限视为异常请求。
	MaxFrameBytes = 2 << 20
	// maxStoredFrames 防 serial 无限增长的上限。
	maxStoredFrames = 32
)

// Frame 一帧已解码画面（JPEG 编码）。
type Frame struct {
	JPEG      []byte    `json:"-"`
	Width     int       `json:"width"`
	Height    int       `json:"height"`
	Timestamp time.Time `json:"timestamp"`
}

// Store 按 serial 保留最新一帧；多路投屏（主面板/悬浮窗）同传时后到覆盖。
type Store struct {
	mu     sync.RWMutex
	latest map[string]Frame
	seen   map[string]bool
}

func NewStore() *Store {
	return &Store{latest: make(map[string]Frame), seen: make(map[string]bool)}
}

// Put 存入一帧；分辨率从 JPEG 头解析，解析失败返回错误不入库。
func (s *Store) Put(serial string, jpegBytes []byte) (Frame, error) {
	cfg, err := jpeg.DecodeConfig(bytes.NewReader(jpegBytes))
	if err != nil {
		return Frame{}, err
	}
	f := Frame{
		JPEG:      append([]byte(nil), jpegBytes...),
		Width:     cfg.Width,
		Height:    cfg.Height,
		Timestamp: time.Now(),
	}
	s.mu.Lock()
	if !s.seen[serial] {
		s.seen[serial] = true
		log.Printf("[VisionFrames] 首帧上传 serial=%s %dx%d", serial, f.Width, f.Height)
	}
	if len(s.latest) >= maxStoredFrames {
		delete(s.latest, s.oldestLocked())
	}
	s.latest[serial] = f
	s.mu.Unlock()
	return f, nil
}

func (s *Store) oldestLocked() string {
	var oldestSerial string
	var oldest time.Time
	for serial, f := range s.latest {
		if oldestSerial == "" || f.Timestamp.Before(oldest) {
			oldestSerial, oldest = serial, f.Timestamp
		}
	}
	return oldestSerial
}

// Latest 返回 serial 在 maxAge 内的最新帧；过期或不存在返回 false。
func (s *Store) Latest(serial string, maxAge time.Duration) (Frame, bool) {
	s.mu.RLock()
	f, ok := s.latest[serial]
	s.mu.RUnlock()
	if !ok || time.Since(f.Timestamp) > maxAge {
		return Frame{}, false
	}
	return f, true
}

// Handler 返回上传端点，供匹配服务 bridge 等内部调用方复用。
func (s *Store) Handler() http.Handler {
	return http.HandlerFunc(s.handleUpload)
}

// RegisterRoutes 挂载浏览器帧上传端点。
func (s *Store) RegisterRoutes(mux *http.ServeMux) {
	mux.HandleFunc("POST /api/vision/frame", s.handleUpload)
}

func (s *Store) handleUpload(w http.ResponseWriter, r *http.Request) {
	serial := r.URL.Query().Get("serial")
	if serial == "" {
		http.Error(w, "missing serial", http.StatusBadRequest)
		return
	}
	body := http.MaxBytesReader(w, r.Body, MaxFrameBytes)
	buf := new(bytes.Buffer)
	if _, err := buf.ReadFrom(body); err != nil {
		http.Error(w, "读取帧失败: "+err.Error(), http.StatusBadRequest)
		return
	}
	f, err := s.Put(serial, buf.Bytes())
	if err != nil {
		http.Error(w, "帧解码失败: "+err.Error(), http.StatusBadRequest)
		return
	}
	writeJSON(w, http.StatusOK, f)
}

func writeJSON(w http.ResponseWriter, status int, value any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(value)
}
