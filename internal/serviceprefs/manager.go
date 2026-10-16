package serviceprefs

import (
	"context"
	"fmt"
	"log"
	"strconv"
	"sync"
	"time"
)

type Store interface {
	GetConfig(string) (string, error)
	SetConfig(string, string) error
}

type Manager struct {
	mu        sync.Mutex
	store     Store
	cancels   map[string]context.CancelFunc
	revisions map[string]uint64
	jobs      sync.WaitGroup
}

func New(store Store) *Manager {
	return &Manager{store: store, cancels: map[string]context.CancelFunc{}, revisions: map[string]uint64{}}
}

func (m *Manager) Remember(name string, enabled bool) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.revisions[name]++
	return m.rememberLocked(name, enabled)
}

func (m *Manager) rememberLocked(name string, enabled bool) error {
	if m.store == nil {
		return fmt.Errorf("数据库不可用，无法保存服务启停设置")
	}
	if err := m.store.SetConfig("service_autostart_"+name, strconv.FormatBool(enabled)); err != nil {
		return err
	}
	// An explicit operation supersedes a pending restore, including during install.
	if cancel := m.cancels[name]; cancel != nil {
		cancel()
		delete(m.cancels, name)
	}
	log.Printf("[ServicePreferences] 已保存 service=%s enabled=%t", name, enabled)
	return nil
}

// Control rejects a stale start completion after a newer stop request.
func (m *Manager) Control(name string, enabled bool, action func() error) error {
	m.mu.Lock()
	m.revisions[name]++
	revision := m.revisions[name]
	if cancel := m.cancels[name]; cancel != nil {
		cancel()
		delete(m.cancels, name)
	}
	if !enabled {
		if err := m.rememberLocked(name, false); err != nil {
			m.mu.Unlock()
			return err
		}
	}
	m.mu.Unlock()
	if err := action(); err != nil {
		return err
	}
	if enabled {
		m.mu.Lock()
		defer m.mu.Unlock()
		if m.revisions[name] == revision {
			return m.rememberLocked(name, true)
		}
	}
	return nil
}

// StopForRuntimeChange cancels delayed restoration and stale start completions.
// The next application launch still honors the user's saved startup preference.
func (m *Manager) StopForRuntimeChange(name string, stop func() error) error {
	m.mu.Lock()
	m.revisions[name]++
	if cancel := m.cancels[name]; cancel != nil {
		cancel()
		delete(m.cancels, name)
	}
	m.mu.Unlock()
	return stop()
}

func (m *Manager) RestoreAfter(delay time.Duration, starts map[string]func(context.Context) error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	for name, start := range starts {
		ctx, cancel := context.WithCancel(context.Background())
		m.cancels[name] = cancel
		m.jobs.Add(1)
		go func() {
			defer m.jobs.Done()
			timer := time.NewTimer(delay)
			defer timer.Stop()
			select {
			case <-ctx.Done():
				return
			case <-timer.C:
			}
			m.mu.Lock()
			var value string
			var err error
			if m.store != nil {
				value, err = m.store.GetConfig("service_autostart_" + name)
			}
			m.mu.Unlock()
			if err != nil {
				log.Printf("[ServicePreferences] 读取失败 service=%s error=%v", name, err)
				return
			}
			if value == "" {
				return
			}
			if value != "true" && value != "false" {
				log.Printf("[ServicePreferences] 设置无效 service=%s", name)
				return
			}
			if value != "true" || ctx.Err() != nil {
				return
			}
			log.Printf("[ServicePreferences] 延迟恢复服务 service=%s", name)
			if err := start(ctx); err != nil {
				log.Printf("[ServicePreferences] 自动启动失败 service=%s error=%v", name, err)
			}
		}()
	}
}

func (m *Manager) Close() {
	m.mu.Lock()
	for _, cancel := range m.cancels {
		cancel()
	}
	m.mu.Unlock()
	m.jobs.Wait()
}
