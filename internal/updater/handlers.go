package updater

import (
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"sync"
)

// Service 整合 Checker、Downloader 与 Applier
type Service struct {
	checker    *Checker
	downloader *Downloader
	applier    *Applier
	upgradeMu  sync.Mutex
	policy     Policy
	prepare    func() error
	resume     func()
	stop       func()
	version    string
}

// NewService 创建 Updater 服务
func NewService(currentVersion string) (*Service, error) {
	checker := NewChecker(currentVersion)
	downloader := NewDownloader()
	applier, err := NewApplier()
	if err != nil {
		return nil, err
	}

	return &Service{
		checker:    checker,
		downloader: downloader,
		applier:    applier,
		policy:     DetectPolicy(false),
		version:    currentVersion,
	}, nil
}

// RegisterRoutes 注册自动更新相关 HTTP 接口
func RegisterRoutes(mux *http.ServeMux, s *Service) {
	mux.HandleFunc("/api/version/check", s.handleCheck)
	mux.HandleFunc("GET /api/version/health", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]string{"version": s.version})
	})
	mux.HandleFunc("/api/version/upgrade", s.handleUpgrade)
	mux.HandleFunc("/api/version/upgrade-stream", s.handleUpgradeStream)
	mux.HandleFunc("/api/version/status", s.handleStatus)
}

func (s *Service) handleCheck(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method Not Allowed", http.StatusMethodNotAllowed)
		return
	}

	result, err := s.checker.CheckUpdate()
	if err != nil {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusInternalServerError)
		_ = json.NewEncoder(w).Encode(map[string]any{
			"error":           err.Error(),
			"current_version": s.checker.GetCurrentVersion(),
		})
		return
	}

	result.Policy = s.policy
	result.CanUpgrade = s.policy.Method == "binary" && result.HasUpdate && result.DownloadURL != "" && validDigest(result.Digest)
	if s.policy.Method == "binary" && result.HasUpdate && !result.CanUpgrade {
		result.Reason = "当前版本缺少可验证的更新文件"
	}
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(result)
}

func (s *Service) handleStatus(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method Not Allowed", http.StatusMethodNotAllowed)
		return
	}
	progress := s.downloader.GetProgress()
	if progress.State == StateIdle {
		if bytes, err := os.ReadFile(filepath.Join("data", "updater-status.json")); err == nil {
			_ = json.Unmarshal(bytes, &progress)
		}
	}
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(progress)
}

type UpgradeRequest struct {
	DownloadURL string `json:"download_url"`
	AssetSize   int64  `json:"asset_size"`
	Restart     bool   `json:"restart"`
}

func (s *Service) handleUpgrade(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method Not Allowed", http.StatusMethodNotAllowed)
		return
	}

	if receipt := os.Getenv("MYWEBSCRCPY_UPDATE_RECEIPT"); receipt != "" && !fileExists(receipt+".confirmed") {
		http.Error(w, "新版启动验证尚未完成", http.StatusConflict)
		return
	}
	if s.policy.Method != "binary" {
		http.Error(w, s.policy.Reason, http.StatusConflict)
		return
	}
	// Never accept a download URL from a client: select only the configured repository release.
	if !s.upgradeMu.TryLock() {
		http.Error(w, "更新正在进行", http.StatusConflict)
		return
	}
	check, err := s.checker.CheckUpdate()
	if err != nil || check == nil || !check.HasUpdate || check.DownloadURL == "" || !validDigest(check.Digest) {
		s.upgradeMu.Unlock()
		http.Error(w, "没有可验证的新版更新文件", http.StatusBadRequest)
		return
	}
	if s.prepare == nil || s.stop == nil {
		s.upgradeMu.Unlock()
		http.Error(w, "当前启动方式不支持自动更新", http.StatusConflict)
		return
	}
	_ = os.Remove(filepath.Join("data", "updater-status.json"))
	go func() {
		defer s.upgradeMu.Unlock()
		if err := s.stageUpgrade(check); err != nil {
			s.downloader.notify(ProgressInfo{State: StateError, Error: err.Error()})
		}
	}()
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusAccepted)
	_ = json.NewEncoder(w).Encode(map[string]string{"status": "started"})
}

// ConfigureRuntime connects the updater to the application's orderly shutdown.
func (s *Service) ConfigureRuntime(policy Policy, prepare func() error, stop func(), resume func()) {
	s.policy, s.prepare, s.stop, s.resume = policy, prepare, stop, resume
}

func (s *Service) handleUpgradeStream(w http.ResponseWriter, r *http.Request) {
	flusher, ok := w.(http.Flusher)
	if !ok {
		http.Error(w, "Streaming unsupported", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")
	w.Header().Set("X-Accel-Buffering", "no")

	ch := s.downloader.Subscribe()
	defer s.downloader.Unsubscribe(ch)

	notify := r.Context().Done()

	for {
		select {
		case <-notify:
			return
		case p, ok := <-ch:
			if !ok {
				return
			}
			data, _ := json.Marshal(p)
			_, _ = fmt.Fprintf(w, "data: %s\n\n", data)
			flusher.Flush()

			if p.State == StateSuccess || p.State == StateError {
				return
			}
		}
	}
}
