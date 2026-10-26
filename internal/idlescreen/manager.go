// Package idlescreen sleeps devices after ten minutes without application work.
package idlescreen

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"sync"
	"time"
)

const IdleTimeout = 10 * time.Minute
const configKey = "idle_screen"

type Store interface {
	GetConfig(string) (string, error)
	SetConfig(string, string) error
}

type Config struct {
	Enabled bool `json:"enabled"`
}

type deviceState struct {
	refs       int
	last       time.Time
	online     bool
	sleeping   bool
	queued     bool
	generation uint64
	cancel     context.CancelFunc
	retryAt    time.Time
}

type sleepJob struct {
	serial     string
	generation uint64
	device     *deviceState
}

type Manager struct {
	mu      sync.Mutex
	store   Store
	config  Config
	devices map[string]*deviceState
	now     func() time.Time
	timeout time.Duration
	sleep   func(context.Context, string) error
	online  func() []string
	ctx     context.Context
	cancel  context.CancelFunc
	jobs    chan sleepJob
	wg      sync.WaitGroup
}

// New starts one scheduler and one bounded worker; sleep must honor cancellation.
func New(store Store, online func() []string, sleep func(context.Context, string) error) *Manager {
	ctx, cancel := context.WithCancel(context.Background())
	m := &Manager{store: store, devices: map[string]*deviceState{}, now: time.Now, timeout: IdleTimeout, sleep: sleep, online: online, ctx: ctx, cancel: cancel, jobs: make(chan sleepJob, 32)}
	if store != nil {
		raw, err := store.GetConfig(configKey)
		if err != nil {
			log.Printf("[IdleScreen] 读取配置失败: %v", err)
		} else if raw != "" {
			if err := json.Unmarshal([]byte(raw), &m.config); err != nil {
				log.Printf("[IdleScreen] 配置无效: %v", err)
			}
		}
	}
	m.wg.Add(2)
	go m.run()
	go m.worker()
	return m
}

func (m *Manager) Config() Config { m.mu.Lock(); defer m.mu.Unlock(); return m.config }

func (m *Manager) Set(c Config) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.store == nil {
		return fmt.Errorf("数据库不可用，无法保存熄屏设置")
	}
	raw, _ := json.Marshal(c)
	if err := m.store.SetConfig(configKey, string(raw)); err != nil {
		return err
	}
	if m.config == c {
		return nil
	}
	m.config = c
	for _, d := range m.devices {
		m.invalidate(d)
		d.last = m.now()
		d.sleeping = false
	}
	log.Printf("[IdleScreen] 设置已保存 enabled=%t timeout=%s", c.Enabled, m.timeout)
	return nil
}

func (m *Manager) invalidate(d *deviceState) {
	d.generation++
	d.queued = false
	if d.cancel != nil {
		d.cancel()
		d.cancel = nil
	}
}

// Begin counts both queued and running work, including persistent stream leases.
func (m *Manager) Begin(serial string) func() {
	m.mu.Lock()
	d := m.devices[serial]
	if d == nil {
		d = &deviceState{last: m.now()}
		m.devices[serial] = d
	}
	m.invalidate(d)
	d.refs++
	d.sleeping = false
	d.last = m.now()
	m.mu.Unlock()
	var once sync.Once
	return func() {
		once.Do(func() {
			m.mu.Lock()
			defer m.mu.Unlock()
			d.refs--
			d.last = m.now()
		})
	}
}

func (m *Manager) scan() {
	online := m.online()
	m.mu.Lock()
	defer m.mu.Unlock()
	now := m.now()
	seen := make(map[string]bool, len(online))
	for _, serial := range online {
		seen[serial] = true
		d := m.devices[serial]
		if d == nil {
			d = &deviceState{last: now}
			m.devices[serial] = d
		}
		if !d.online {
			d.online = true
			d.last = now
			d.sleeping = false
			m.invalidate(d)
		}
		if !m.config.Enabled || d.refs > 0 || d.sleeping || d.queued || now.Sub(d.last) < m.timeout || now.Before(d.retryAt) {
			continue
		}
		job := sleepJob{serial: serial, generation: d.generation, device: d}
		select {
		case m.jobs <- job:
			d.queued = true
		default: /* bounded; retry next scan */
		}
	}
	for serial, d := range m.devices {
		if !seen[serial] && d.online {
			d.online = false
			m.invalidate(d)
		}
		if !d.online && d.refs == 0 {
			delete(m.devices, serial)
		}
	}
}

func (m *Manager) run() {
	defer m.wg.Done()
	ticker := time.NewTicker(5 * time.Second)
	defer ticker.Stop()
	m.scan()
	for {
		select {
		case <-m.ctx.Done():
			return
		case <-ticker.C:
			m.scan()
		}
	}
}

func (m *Manager) worker() {
	defer m.wg.Done()
	for {
		select {
		case <-m.ctx.Done():
			return
		case job := <-m.jobs:
			m.execute(job)
		}
	}
}

func (m *Manager) execute(job sleepJob) {
	m.mu.Lock()
	d := m.devices[job.serial]
	if m.ctx.Err() != nil || !m.config.Enabled || d == nil || d != job.device || !d.online || d.refs > 0 || d.sleeping || d.generation != job.generation || m.now().Sub(d.last) < m.timeout {
		m.mu.Unlock()
		return
	}
	ctx, cancel := context.WithTimeout(m.ctx, 5*time.Second)
	d.cancel = cancel
	m.mu.Unlock()
	err := m.sleep(ctx, job.serial)
	cancel()
	m.mu.Lock()
	defer m.mu.Unlock()
	if d.generation != job.generation {
		return
	}
	d.cancel = nil
	d.queued = false
	if err != nil {
		d.retryAt = m.now().Add(time.Minute)
		log.Printf("[IdleScreen] 熄屏失败 serial=%s: %v", job.serial, err)
		return
	}
	d.sleeping = true
	log.Printf("[IdleScreen] 空闲超时已熄屏 serial=%s idle=%s", job.serial, m.timeout)
}

func (m *Manager) Close() {
	m.cancel()
	m.mu.Lock()
	for _, d := range m.devices {
		m.invalidate(d)
	}
	m.mu.Unlock()
	m.wg.Wait()
}
