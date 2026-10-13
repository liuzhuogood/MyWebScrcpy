package matchingservice

import (
	"bufio"
	"context"
	"crypto/rand"
	_ "embed"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	pythonsdk "mywebscrcpy/packages/python-sdk"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"sync"
	"time"
)

//go:embed runner.py
var runner []byte

type DeviceStatus struct {
	Serial string `json:"serial"`
	State  string `json:"state"`
	Error  string `json:"error"`
}
type Status struct {
	State       string         `json:"state"`
	PID         int            `json:"pid"`
	Error       string         `json:"error"`
	Environment Environment    `json:"environment"`
	Devices     []DeviceStatus `json:"devices"`
}
type Options struct {
	Python           func() (string, error)
	Devices          func() []string
	TemplateReadable func(string, string) bool
	API              http.Handler
	Vision           http.Handler
}
type run struct {
	cmd       *exec.Cmd
	input     io.WriteCloser
	cancel    context.CancelFunc
	server    *http.Server
	done      chan struct{}
	ready     chan struct{}
	directory string
}
type Manager struct {
	mu          sync.Mutex
	options     Options
	state       string
	environment Environment
	error       string
	devices     map[string]DeviceStatus
	process     *run
	closed      bool
	initCancel  context.CancelFunc
	initDone    chan struct{}
}

func New(options Options) *Manager {
	return &Manager{options: options, state: "stopped", environment: Environment{Dependencies: []Dependency{}}, devices: map[string]DeviceStatus{}}
}
func (m *Manager) Status() Status {
	m.mu.Lock()
	defer m.mu.Unlock()
	result := Status{State: m.state, Error: m.error, Environment: m.environment, Devices: []DeviceStatus{}}
	result.Environment.Dependencies = append([]Dependency{}, m.environment.Dependencies...)
	if m.process != nil && m.process.cmd.Process != nil {
		result.PID = m.process.cmd.Process.Pid
	}
	for _, device := range m.devices {
		result.Devices = append(result.Devices, device)
	}
	sort.Slice(result.Devices, func(i, j int) bool { return result.Devices[i].Serial < result.Devices[j].Serial })
	return result
}
func (m *Manager) Check(ctx context.Context) (Environment, error) {
	m.mu.Lock()
	if m.closed || m.initDone != nil || m.state == "checking" || m.state == "starting" || m.state == "stopping" || m.process != nil {
		m.mu.Unlock()
		return Environment{}, errors.New("服务忙，请停止后检测")
	}
	op, cancel := context.WithCancel(ctx)
	done := make(chan struct{})
	m.initCancel = cancel
	m.initDone = done
	defer m.finishInit(cancel, done)
	m.state = "checking"
	m.mu.Unlock()
	env := inspectPython(op, m.options.Python)
	m.mu.Lock()
	m.environment = env
	if m.state != "stopping" {
		m.state = "stopped"
	}
	m.mu.Unlock()
	return env, nil
}
func (m *Manager) Start(ctx context.Context) error {
	m.mu.Lock()
	if m.closed || m.initDone != nil || m.state == "checking" || m.state == "starting" || m.state == "stopping" || m.process != nil {
		m.mu.Unlock()
		return errors.New("服务忙或已启动")
	}
	op, cancelInit := context.WithCancel(ctx)
	done := make(chan struct{})
	m.initCancel = cancelInit
	m.initDone = done
	defer m.finishInit(cancelInit, done)
	m.state = "starting"
	m.error = ""
	m.mu.Unlock()
	env := inspectPython(op, m.options.Python)
	m.mu.Lock()
	m.environment = env
	m.mu.Unlock()
	if !env.Ready {
		return m.failStart(errors.New("Python 或识图依赖不可用，请先检测环境"))
	}
	directory, err := os.MkdirTemp("", "mywebscrcpy-matching-*")
	if err != nil {
		return m.failStart(err)
	}
	cleanup := func() { os.RemoveAll(directory) }
	if err = pythonsdk.Install(directory); err != nil {
		cleanup()
		return m.failStart(err)
	}
	script := filepath.Join(directory, "runner.py")
	if err = os.WriteFile(script, runner, 0600); err != nil {
		cleanup()
		return m.failStart(err)
	}
	entropy := make([]byte, 32)
	if _, err = rand.Read(entropy); err != nil {
		cleanup()
		return m.failStart(err)
	}
	lifetime, cancel := context.WithCancel(context.Background())
	server, url, err := m.bridge(lifetime, hex.EncodeToString(entropy))
	if err != nil {
		cancel()
		cleanup()
		return m.failStart(err)
	}
	cmd := exec.Command(env.Path, "-u", script)
	cmd.Dir = directory
	cmd.Env = append(os.Environ(), "MYWEBSCRCPY_MATCHING_URL="+url, "MYWEBSCRCPY_MATCHING_TOKEN="+hex.EncodeToString(entropy))
	input, err := cmd.StdinPipe()
	if err != nil {
		cancel()
		server.Close()
		cleanup()
		return m.failStart(err)
	}
	output, err := cmd.StdoutPipe()
	if err != nil {
		input.Close()
		cancel()
		server.Close()
		cleanup()
		return m.failStart(err)
	}
	cmd.Stderr = log.Writer()
	m.mu.Lock()
	if m.closed || op.Err() != nil {
		m.mu.Unlock()
		input.Close()
		cancel()
		server.Close()
		cleanup()
		return m.failStart(errors.New("启动已取消"))
	}
	if err = cmd.Start(); err != nil {
		m.mu.Unlock()
		input.Close()
		cancel()
		server.Close()
		cleanup()
		return m.failStart(err)
	}
	job := &run{cmd: cmd, input: input, cancel: cancel, server: server, done: make(chan struct{}), ready: make(chan struct{}), directory: directory}
	m.process = job
	m.devices = map[string]DeviceStatus{}
	m.mu.Unlock()
	go m.observe(job, output)
	timer := time.NewTimer(15 * time.Second)
	defer timer.Stop()
	select {
	case <-job.ready:
		m.mu.Lock()
		if m.process == job && m.state == "starting" {
			m.state = "running"
		}
		state := m.state
		m.mu.Unlock()
		if state != "running" {
			return errors.New("匹配进程在就绪后退出")
		}
		log.Printf("[MatchingService] started pid=%d", cmd.Process.Pid)
		return nil
	case <-job.done:
		return errors.New("匹配进程未就绪即退出")
	case <-op.Done():
		m.Stop()
		return op.Err()
	case <-timer.C:
		m.Stop()
		return m.failStart(errors.New("匹配进程就绪超时"))
	}
}
func (m *Manager) failStart(err error) error {
	m.mu.Lock()
	if m.state != "stopping" {
		m.state = "failed"
		m.error = err.Error()
	}
	m.mu.Unlock()
	log.Printf("[MatchingService] start failed: %v", err)
	return err
}
func (m *Manager) observe(job *run, output io.Reader) {
	scanner := bufio.NewScanner(output)
	scanner.Buffer(make([]byte, 4096), 65536)
	seenReady := false
	for scanner.Scan() {
		var event struct {
			Type   string `json:"type"`
			Serial string `json:"serial"`
			State  string `json:"state"`
			Error  string `json:"error"`
		}
		if json.Unmarshal(scanner.Bytes(), &event) != nil {
			log.Print("[MatchingService] invalid event")
			continue
		}
		m.mu.Lock()
		if m.process == job {
			if event.Type == "ready" && !seenReady {
				seenReady = true
				close(job.ready)
			}
			if event.Type == "device" && event.Serial != "" {
				if event.State == "removed" {
					delete(m.devices, event.Serial)
				} else if event.State == "waiting" || event.State == "matching" || event.State == "failed" {
					m.devices[event.Serial] = DeviceStatus{event.Serial, event.State, bounded(event.Error, 500)}
				}
			}
		}
		m.mu.Unlock()
	}
	if scanner.Err() != nil {
		job.cmd.Process.Kill()
	}
	err := job.cmd.Wait()
	job.cancel()
	job.server.Close()
	os.RemoveAll(job.directory)
	m.mu.Lock()
	if m.process == job {
		m.process = nil
		m.devices = map[string]DeviceStatus{}
		if m.state == "stopping" {
			m.state = "stopped"
			m.error = ""
		} else {
			m.state = "failed"
			m.error = fmt.Sprintf("匹配进程退出: %v", err)
		}
	}
	m.mu.Unlock()
	close(job.done)
	log.Printf("[MatchingService] exited pid=%d error=%v", job.cmd.Process.Pid, err)
}
func (m *Manager) Stop() error {
	m.mu.Lock()
	job := m.process
	if job == nil {
		if m.initCancel != nil {
			m.state = "stopping"
			cancel, done := m.initCancel, m.initDone
			m.mu.Unlock()
			cancel()
			select {
			case <-done:
			case <-time.After(3 * time.Second):
				return errors.New("环境检测尚未退出")
			}
			m.mu.Lock()
		}
		m.state = "stopped"
		m.error = ""
		m.mu.Unlock()
		return nil
	}
	m.state = "stopping"
	m.mu.Unlock()
	job.input.Close()
	select {
	case <-job.done:
		return nil
	case <-time.After(5 * time.Second):
		job.cancel()
		job.cmd.Process.Kill()
	}
	select {
	case <-job.done:
		return nil
	case <-time.After(3 * time.Second):
		return errors.New("匹配进程尚未退出")
	}
}
func (m *Manager) Close() {
	m.mu.Lock()
	m.closed = true
	m.mu.Unlock()
	if err := m.Stop(); err != nil {
		log.Printf("[MatchingService] close failed: %v", err)
	}
}
func (m *Manager) finishInit(cancel context.CancelFunc, done chan struct{}) {
	cancel()
	m.mu.Lock()
	if m.initDone == done {
		m.initCancel = nil
		m.initDone = nil
	}
	close(done)
	m.mu.Unlock()
}
func (m *Manager) CheckIdle() error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.process != nil || m.initDone != nil || m.state == "starting" || m.state == "checking" || m.state == "stopping" {
		return errors.New("请先停止模板匹配服务")
	}
	return nil
}
