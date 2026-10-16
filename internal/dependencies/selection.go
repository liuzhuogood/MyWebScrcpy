package dependencies

import (
	"fmt"
	"log"
	"path/filepath"
	"time"

	"github.com/gofrs/flock"
)

// lockSelections freezes only changed dependencies. Python workers are stopped
// before taking their write lease, while new readers are rejected until save ends.
func (m *Manager) lockSelections(previous, next Settings) (func(), error) {
	if err := safeDirectory(m.root); err != nil {
		return nil, err
	}
	cleanup := func() {}
	if next.ADB != previous.ADB {
		release, err := m.exclusive([]string{"adb"}, true)
		if err != nil {
			return nil, err
		}
		cleanup = release
	}
	if next.Python == previous.Python {
		return cleanup, nil
	}
	jobLock := flock.New(filepath.Join(m.root, ".install-lock-python"))
	locked, err := jobLock.TryLock()
	if err != nil || !locked {
		cleanup()
		return nil, fmt.Errorf("Python 正在安装或清理")
	}
	m.mu.Lock()
	m.pythonChanging = true
	m.mu.Unlock()
	releaseGate := func() {
		m.mu.Lock()
		m.pythonChanging = false
		m.mu.Unlock()
		_ = jobLock.Unlock()
		cleanup()
	}
	log.Print("[Dependencies] 切换 Python，停止旧运行时服务")
	if m.BeforePythonChange != nil {
		if err := m.BeforePythonChange(); err != nil {
			releaseGate()
			log.Printf("[Dependencies] 停止旧 Python 服务失败: %v", err)
			return nil, fmt.Errorf("停止旧 Python 服务失败: %w", err)
		}
	}
	// Status probes already holding a read lease have a four-second timeout.
	// Allow them to finish without making users retry a successful worker stop.
	deadline := time.NewTimer(6 * time.Second)
	defer deadline.Stop()
	ticker := time.NewTicker(50 * time.Millisecond)
	defer ticker.Stop()
	for {
		release, err := m.exclusive([]string{"python"}, false)
		if err == nil {
			return func() { release(); releaseGate() }, nil
		}
		select {
		case <-deadline.C:
			releaseGate()
			return nil, fmt.Errorf("等待旧 Python 运行时释放超时: %w", err)
		case <-ticker.C:
		}
	}
}
