package updater

import (
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"sync"
	"time"
)

// Service 整合 Checker、Downloader 与 Applier
type Service struct {
	checker    *Checker
	downloader *Downloader
	applier    *Applier
	upgradeMu  sync.Mutex
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
	}, nil
}

// RegisterRoutes 注册自动更新相关 HTTP 接口
func RegisterRoutes(mux *http.ServeMux, s *Service) {
	mux.HandleFunc("/api/version/check", s.handleCheck)
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

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(result)
}

func (s *Service) handleStatus(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method Not Allowed", http.StatusMethodNotAllowed)
		return
	}
	progress := s.downloader.GetProgress()
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

	var req UpgradeRequest
	if r.Body != nil {
		_ = json.NewDecoder(r.Body).Decode(&req)
	}

	// 如果没有传 download_url，主动检查一次并使用匹配的 asset
	if req.DownloadURL == "" {
		check, err := s.checker.CheckUpdate()
		if err != nil {
			http.Error(w, fmt.Sprintf("failed to get release info: %v", err), http.StatusBadRequest)
			return
		}
		if check.DownloadURL == "" {
			http.Error(w, "no suitable binary asset found for current OS/Arch", http.StatusBadRequest)
			return
		}
		req.DownloadURL = check.DownloadURL
		req.AssetSize = check.AssetSize
	}

	// 启动后台下载任务
	go func() {
		s.upgradeMu.Lock()
		defer s.upgradeMu.Unlock()

		tempDir := os.TempDir()
		destFile := filepath.Join(tempDir, fmt.Sprintf("mywebscrcpy-update-%d", time.Now().UnixNano()))

		err := s.downloader.DownloadAsset(req.DownloadURL, req.AssetSize, destFile)
		if err != nil {
			return
		}

		// 下载完成，进入 Applying 阶段
		s.downloader.notify(ProgressInfo{
			State:      StateApplying,
			Percent:    100,
			Downloaded: req.AssetSize,
			Total:      req.AssetSize,
			Speed:      "0 KB/s",
			TargetFile: destFile,
		})

		// 应用更新
		if err := s.applier.ApplyUpdate(destFile); err != nil {
			s.downloader.notify(ProgressInfo{
				State: StateError,
				Error: fmt.Sprintf("apply update failed: %v", err),
			})
			return
		}

		// 应用成功
		s.downloader.notify(ProgressInfo{
			State:      StateSuccess,
			Percent:    100,
			Downloaded: req.AssetSize,
			Total:      req.AssetSize,
			Speed:      "0 KB/s",
		})

		// 触发重启
		_ = s.applier.RestartProcess()
	}()

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]any{
		"status":  "started",
		"message": "upgrade task initiated",
	})
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
