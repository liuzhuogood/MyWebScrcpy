// Package adbcommand exposes a deliberately host-shell-free ADB command API.
package adbcommand

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"strconv"
	"strings"
	"sync"
	"time"

	"mywebscrcpy/internal/devicegate"
)

const (
	defaultTimeout    = 60 * time.Second
	defaultMaxTimeout = 10 * time.Minute
	defaultQueueWait  = 10 * time.Minute
	defaultOutputMax  = int64(1 << 20)
)

var ErrNotFound = errors.New("command not found")

type Status string

const (
	Queued    Status = "queued"
	Running   Status = "running"
	Completed Status = "completed"
	Failed    Status = "failed"
	TimedOut  Status = "timed_out"
)

type Request struct {
	Args      []string `json:"args"`
	TimeoutMS *int64   `json:"timeout_ms,omitempty"`
	Async     bool     `json:"async,omitempty"`
	Parallel  bool     `json:"parallel,omitempty"`
}
type View struct {
	CommandID string     `json:"command_id"`
	Serial    string     `json:"serial"`
	Status    Status     `json:"status"`
	QueuedAt  time.Time  `json:"queued_at"`
	StartedAt *time.Time `json:"started_at,omitempty"`
	EndedAt   *time.Time `json:"ended_at,omitempty"`
	ExitCode  *int       `json:"exit_code,omitempty"`
	Stdout    string     `json:"stdout,omitempty"`
	Stderr    string     `json:"stderr,omitempty"`
	ElapsedMS int64      `json:"elapsed_ms,omitempty"`
	Truncated bool       `json:"truncated,omitempty"`
	Error     string     `json:"error,omitempty"`
}
type job struct {
	mu sync.Mutex
	View
	args     []string
	timeout  time.Duration
	parallel bool
	done     chan struct{}
}
type Manager struct {
	adbPath               string
	prefix                []string
	maxTimeout, queueWait time.Duration
	outputMax             int64
	gateFor               func(string) *devicegate.Gate
	mu                    sync.Mutex
	jobs                  map[string]*job
	next                  uint64
}

func New(adbPath string, gateFor func(string) *devicegate.Gate) *Manager {
	return &Manager{adbPath: adbPath, maxTimeout: envDuration("ADB_MAX_TIMEOUT_MS", defaultMaxTimeout), queueWait: envDuration("ADB_MAX_QUEUE_WAIT_MS", defaultQueueWait), outputMax: envPositive("ADB_MAX_OUTPUT_BYTES", defaultOutputMax), gateFor: gateFor, jobs: make(map[string]*job)}
}
func envPositive(key string, fallback int64) int64 {
	v, err := strconv.ParseInt(os.Getenv(key), 10, 64)
	if err != nil || v <= 0 {
		return fallback
	}
	return v
}
func envDuration(key string, fallback time.Duration) time.Duration {
	return time.Duration(envPositive(key, fallback.Milliseconds())) * time.Millisecond
}

func (m *Manager) Submit(serial string, req Request) (*job, error) {
	if serial == "" {
		return nil, errors.New("missing serial")
	}
	if err := validateArgs(req.Args); err != nil {
		return nil, err
	}
	timeout := defaultTimeout
	if req.TimeoutMS != nil {
		timeout = time.Duration(*req.TimeoutMS) * time.Millisecond
	}
	if timeout <= 0 || timeout > m.maxTimeout {
		return nil, fmt.Errorf("timeout_ms must be between 1 and %d", m.maxTimeout.Milliseconds())
	}
	m.mu.Lock()
	m.next++
	id := fmt.Sprintf("adb_%d", m.next)
	status := Queued
	if req.Parallel {
		status = Running
	}
	j := &job{View: View{CommandID: id, Serial: serial, Status: status, QueuedAt: time.Now().UTC()}, args: append([]string(nil), req.Args...), timeout: timeout, parallel: req.Parallel, done: make(chan struct{})}
	m.jobs[id] = j
	m.mu.Unlock()
	go m.run(j)
	if req.Async {
		return j, nil
	}
	<-j.done
	return j, nil
}
func (m *Manager) Get(id, serial string) (*job, error) {
	m.mu.Lock()
	j := m.jobs[id]
	m.mu.Unlock()
	if j == nil {
		return nil, ErrNotFound
	}
	if serial == "" || j.Serial != serial {
		return nil, errors.New("serial does not match command")
	}
	return j, nil
}
func (j *job) Snapshot() View { j.mu.Lock(); defer j.mu.Unlock(); v := j.View; return v }

func (m *Manager) run(j *job) {
	defer close(j.done)
	queueCtx, cancelQueue := context.WithTimeout(context.Background(), m.queueWait)
	defer cancelQueue()
	err := m.gateFor(j.Serial).Run(queueCtx, j.parallel, func(_ context.Context) error {
		now := time.Now().UTC()
		j.mu.Lock()
		j.Status = Running
		j.StartedAt = &now
		j.mu.Unlock()
		ctx, cancel := context.WithTimeout(context.Background(), j.timeout)
		defer cancel()
		out := newLimitedOutput(m.outputMax)
		commandArgs := append(append([]string(nil), m.prefix...), "-s", j.Serial)
		commandArgs = append(commandArgs, j.args...)
		cmd := exec.CommandContext(ctx, m.adbPath, commandArgs...)
		cmd.Stdout = out
		cmd.Stderr = out
		err := cmd.Run()
		ended := time.Now().UTC()
		j.mu.Lock()
		j.EndedAt = &ended
		j.ElapsedMS = ended.Sub(now).Milliseconds()
		j.Stdout, j.Stderr = out.String(), ""
		j.Truncated = out.Truncated()
		if errors.Is(ctx.Err(), context.DeadlineExceeded) {
			j.Status = TimedOut
			j.Error = "command timed out"
		} else if err != nil {
			j.Status = Failed
			j.Error = err.Error()
			if exit, ok := err.(*exec.ExitError); ok {
				code := exit.ExitCode()
				j.ExitCode = &code
			}
		} else {
			code := 0
			j.ExitCode = &code
			j.Status = Completed
		}
		j.mu.Unlock()
		return nil
	})
	if err != nil {
		j.mu.Lock()
		if j.Status == Queued || j.Status == Running {
			ended := time.Now().UTC()
			j.EndedAt = &ended
			if errors.Is(err, context.DeadlineExceeded) {
				j.Status = TimedOut
				j.Error = "command queue wait timed out"
			} else {
				j.Status = Failed
				j.Error = err.Error()
			}
		}
		j.mu.Unlock()
	}
}

func validateArgs(args []string) error {
	if len(args) == 0 || len(args) > 128 {
		return errors.New("args must contain 1 to 128 values")
	}
	forbidden := map[string]bool{"adb": true, "-s": true, "--serial": true, "-H": true, "-P": true, "-L": true}
	for _, arg := range args {
		if arg == "" || strings.ContainsRune(arg, '\x00') {
			return errors.New("args contain an invalid value")
		}
		if forbidden[arg] || strings.HasPrefix(arg, "-H") || strings.HasPrefix(arg, "-P") || strings.HasPrefix(arg, "-L") {
			return errors.New("args may not change adb target or host")
		}
	}
	return nil
}

type limitedOutput struct {
	mu        sync.Mutex
	max       int64
	buf       strings.Builder
	n         int64
	truncated bool
}

func newLimitedOutput(max int64) *limitedOutput { return &limitedOutput{max: max} }
func (o *limitedOutput) Write(p []byte) (int, error) {
	o.mu.Lock()
	defer o.mu.Unlock()
	left := o.max - o.n
	if left > 0 {
		if int64(len(p)) > left {
			o.buf.Write(p[:left])
			o.n += left
			o.truncated = true
		} else {
			o.buf.Write(p)
			o.n += int64(len(p))
		}
	} else {
		o.truncated = true
	}
	return len(p), nil
}
func (o *limitedOutput) String() string  { o.mu.Lock(); defer o.mu.Unlock(); return o.buf.String() }
func (o *limitedOutput) Truncated() bool { o.mu.Lock(); defer o.mu.Unlock(); return o.truncated }
