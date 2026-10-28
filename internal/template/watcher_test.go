package template

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestDirWatcherNotifiesOnAssetChange(t *testing.T) {
	dir := t.TempDir()
	notified := make(chan struct{}, 1)
	watcher, err := NewDirWatcher(dir, 10*time.Millisecond, func() {
		select {
		case notified <- struct{}{}:
		default:
		}
	})
	if err != nil {
		t.Fatalf("NewDirWatcher: %v", err)
	}
	defer watcher.Close()
	if err := watcher.Start(); err != nil {
		t.Fatalf("Start: %v", err)
	}
	if err := os.WriteFile(filepath.Join(dir, "template.png"), []byte("asset"), 0o644); err != nil {
		t.Fatalf("write asset: %v", err)
	}
	select {
	case <-notified:
	case <-time.After(2 * time.Second):
		t.Fatal("timed out waiting for asset change")
	}
}

// TestDirWatcherCloseWhileEventsPending 覆盖关闭时仍有待处理事件的路径：
// Windows 后端若在 fsnotify 已关闭后仍调用 Add/Remove，会等不到内部读取协程的应答而永久阻塞。
func TestDirWatcherCloseWhileEventsPending(t *testing.T) {
	dir := t.TempDir()
	watcher, err := NewDirWatcher(dir, 5*time.Millisecond, func() {})
	if err != nil {
		t.Fatalf("NewDirWatcher: %v", err)
	}
	if err := watcher.Start(); err != nil {
		t.Fatalf("Start: %v", err)
	}

	sub := filepath.Join(dir, "nested")
	if err := os.Mkdir(sub, 0o755); err != nil {
		t.Fatalf("mkdir nested: %v", err)
	}
	if err := os.WriteFile(filepath.Join(sub, "template.png"), []byte("asset"), 0o644); err != nil {
		t.Fatalf("write asset: %v", err)
	}
	var removeErr error
	for attempt := 0; attempt < 50; attempt++ {
		removeErr = os.RemoveAll(sub)
		if removeErr == nil {
			break
		}
		time.Sleep(20 * time.Millisecond)
	}
	if removeErr != nil {
		t.Fatalf("remove nested: %v", removeErr)
	}

	done := make(chan error, 1)
	go func() { done <- watcher.Close() }()
	select {
	case err := <-done:
		if err != nil {
			t.Fatalf("Close: %v", err)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("Close 未在 5 秒内返回：事件循环与 fsnotify 关闭疑似互锁")
	}
}
