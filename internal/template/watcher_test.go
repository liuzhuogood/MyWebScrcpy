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
