package updater

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"sync"
	"sync/atomic"
	"time"
)

// ProgressState 表示当前的更新状态
type ProgressState string

const (
	StateIdle        ProgressState = "idle"
	StateDownloading ProgressState = "downloading"
	StateVerifying   ProgressState = "verifying"
	StateApplying    ProgressState = "applying"
	StateSuccess     ProgressState = "success"
	StateError       ProgressState = "error"
)

// ProgressInfo 下载与更新进度快照
type ProgressInfo struct {
	State       ProgressState `json:"state"`
	Percent     float64       `json:"percent"`
	Downloaded  int64         `json:"downloaded"`
	Total       int64         `json:"total"`
	Speed       string        `json:"speed"`
	Error       string        `json:"error,omitempty"`
	TargetFile  string        `json:"target_file,omitempty"`
	UpdatedAt   time.Time     `json:"updated_at"`
}

// Downloader 负责流式下载二进制并计算进度
type Downloader struct {
	httpClient *http.Client
	stateMu    sync.RWMutex
	progress   ProgressInfo
	listeners  map[chan ProgressInfo]struct{}
	listenMu   sync.Mutex
	cancelFn   context.CancelFunc
}

// NewDownloader 创建 Downloader 实例
func NewDownloader() *Downloader {
	return &Downloader{
		httpClient: &http.Client{
			// 下载大文件时不宜过短，设置更灵活的传输
			Timeout: 15 * time.Minute,
		},
		progress: ProgressInfo{
			State: StateIdle,
		},
		listeners: make(map[chan ProgressInfo]struct{}),
	}
}

// GetProgress 获取当前进度
func (d *Downloader) GetProgress() ProgressInfo {
	d.stateMu.RLock()
	defer d.stateMu.RUnlock()
	return d.progress
}

// Subscribe 监听进度更新
func (d *Downloader) Subscribe() chan ProgressInfo {
	ch := make(chan ProgressInfo, 10)
	d.listenMu.Lock()
	d.listeners[ch] = struct{}{}
	// 立即发送当前状态
	d.stateMu.RLock()
	current := d.progress
	d.stateMu.RUnlock()
	ch <- current
	d.listenMu.Unlock()
	return ch
}

// Unsubscribe 取消监听
func (d *Downloader) Unsubscribe(ch chan ProgressInfo) {
	d.listenMu.Lock()
	delete(d.listeners, ch)
	close(ch)
	d.listenMu.Unlock()
}

func (d *Downloader) notify(p ProgressInfo) {
	p.UpdatedAt = time.Now()
	d.stateMu.Lock()
	d.progress = p
	d.stateMu.Unlock()

	d.listenMu.Lock()
	defer d.listenMu.Unlock()
	for ch := range d.listeners {
		select {
		case ch <- p:
		default:
		}
	}
}

// Cancel 取消当前下载
func (d *Downloader) Cancel() {
	if d.cancelFn != nil {
		d.cancelFn()
	}
}

// DownloadAsset 开始下载指定 URL 的 asset 到目标临时文件
func (d *Downloader) DownloadAsset(downloadURL string, totalSize int64, destPath string) error {
	d.stateMu.Lock()
	if d.progress.State == StateDownloading || d.progress.State == StateVerifying || d.progress.State == StateApplying {
		d.stateMu.Unlock()
		return fmt.Errorf("update already in progress")
	}
	d.stateMu.Unlock()

	ctx, cancel := context.WithCancel(context.Background())
	d.cancelFn = cancel

	d.notify(ProgressInfo{
		State:      StateDownloading,
		Percent:    0,
		Downloaded: 0,
		Total:      totalSize,
		Speed:      "0 KB/s",
		TargetFile: destPath,
	})

	req, err := http.NewRequestWithContext(ctx, "GET", downloadURL, nil)
	if err != nil {
		d.notify(ProgressInfo{State: StateError, Error: err.Error()})
		return err
	}
	req.Header.Set("User-Agent", "MyWebScrcpy-Updater")

	resp, err := d.httpClient.Do(req)
	if err != nil {
		d.notify(ProgressInfo{State: StateError, Error: err.Error()})
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		err := fmt.Errorf("download returned HTTP %d", resp.StatusCode)
		d.notify(ProgressInfo{State: StateError, Error: err.Error()})
		return err
	}

	if totalSize <= 0 && resp.ContentLength > 0 {
		totalSize = resp.ContentLength
	}

	if err := os.MkdirAll(filepath.Dir(destPath), 0755); err != nil {
		d.notify(ProgressInfo{State: StateError, Error: err.Error()})
		return err
	}

	out, err := os.OpenFile(destPath, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0755)
	if err != nil {
		d.notify(ProgressInfo{State: StateError, Error: err.Error()})
		return err
	}
	defer out.Close()

	var downloaded int64
	var lastDownloaded int64
	lastTime := time.Now()

	buf := make([]byte, 64*1024) // 64KB buffer
	ticker := time.NewTicker(200 * time.Millisecond)
	defer ticker.Stop()

	doneCh := make(chan struct{})
	go func() {
		for {
			select {
			case <-ticker.C:
				now := time.Now()
				duration := now.Sub(lastTime).Seconds()
				curDownloaded := atomic.LoadInt64(&downloaded)
				delta := curDownloaded - lastDownloaded
				lastDownloaded = curDownloaded
				lastTime = now

				var speedStr string
				if duration > 0 {
					speedBytesPerSec := float64(delta) / duration
					if speedBytesPerSec > 1024*1024 {
						speedStr = fmt.Sprintf("%.2f MB/s", speedBytesPerSec/(1024*1024))
					} else {
						speedStr = fmt.Sprintf("%.1f KB/s", speedBytesPerSec/1024)
					}
				}

				percent := 0.0
				if totalSize > 0 {
					percent = float64(curDownloaded) / float64(totalSize) * 100.0
					if percent > 100.0 {
						percent = 100.0
					}
				}

				d.notify(ProgressInfo{
					State:      StateDownloading,
					Percent:    percent,
					Downloaded: curDownloaded,
					Total:      totalSize,
					Speed:      speedStr,
					TargetFile: destPath,
				})
			case <-doneCh:
				return
			}
		}
	}()

	for {
		nr, rerr := resp.Body.Read(buf)
		if nr > 0 {
			nw, werr := out.Write(buf[0:nr])
			if nw > 0 {
				atomic.AddInt64(&downloaded, int64(nw))
			}
			if werr != nil {
				close(doneCh)
				d.notify(ProgressInfo{State: StateError, Error: werr.Error()})
				return werr
			}
		}
		if rerr != nil {
			close(doneCh)
			if rerr == io.EOF {
				break
			}
			d.notify(ProgressInfo{State: StateError, Error: rerr.Error()})
			return rerr
		}
	}

	d.notify(ProgressInfo{
		State:      StateVerifying,
		Percent:    100,
		Downloaded: downloaded,
		Total:      downloaded,
		Speed:      "0 KB/s",
		TargetFile: destPath,
	})

	return nil
}
