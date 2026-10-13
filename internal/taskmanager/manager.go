package taskmanager

import (
	"context"
	"errors"
	"fmt"
	"log"
	"mywebscrcpy/internal/db"
	"mywebscrcpy/internal/dependencies"
	"mywebscrcpy/internal/taskartifact"
	pythonsdk "mywebscrcpy/packages/python-sdk"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"time"
)

var ErrDeviceBusy = errors.New("设备正在运行其他任务")

type managedTask struct {
	ctx      context.Context
	cmd      *exec.Cmd
	cancel   context.CancelFunc
	done     chan struct{}
	killed   bool
	apiToken string
	serial   string
}
type Manager struct {
	TemplateReadable func(id, serial string) bool
	database         *db.DB
	mu               sync.Mutex
	procs            map[string]*managedTask
	Artifacts        *taskartifact.Service
	closed           bool
	maintenance      bool
	workers          sync.WaitGroup
	apiURL           string
	apiServer        *http.Server
}

func NewManager(database *db.DB) *Manager {
	return &Manager{database: database, procs: map[string]*managedTask{}, Artifacts: taskartifact.NewService(database)}
}

// StartTask preserves the existing API for direct callers.
func (m *Manager) StartTask(scriptID int64, name, serial string, timeout int, python string) (*db.RunningTaskRecord, error) {
	return m.StartDispatchedTask(scriptID, name, serial, timeout, python, "", taskartifact.Options{})
}
func (m *Manager) StartDispatchedTask(scriptID int64, name, serial string, timeout int, python, dispatch string, options taskartifact.Options) (*db.RunningTaskRecord, error) {
	if e := db.ValidateTargetDevices([]string{serial}); e != nil {
		return nil, e
	}
	if timeout <= 0 {
		timeout = 120
	}
	id := fmt.Sprintf("task_%d", time.Now().UnixNano())
	m.mu.Lock()
	if m.closed || m.maintenance {
		m.mu.Unlock()
		return nil, fmt.Errorf("任务管理器已停止")
	}
	active, e := m.database.GetActiveTaskByDevice(serial)
	if e != nil {
		m.mu.Unlock()
		return nil, e
	}
	if active != nil {
		m.mu.Unlock()
		return nil, fmt.Errorf("%w：设备 [%s] 正被任务「%s」占用", ErrDeviceBusy, serial, active.ScriptName)
	}
	if e = m.database.ReserveTask(id, scriptID, name, serial, dispatch); e != nil {
		m.mu.Unlock()
		return nil, e
	}
	lifecycle, cancel := context.WithCancel(context.Background())
	token, err := apiToken()
	if err != nil {
		m.mu.Unlock()
		cancel()
		m.database.FinishRunningTask(id, "failed", -1, err.Error())
		return nil, err
	}
	job := &managedTask{ctx: lifecycle, cancel: cancel, done: make(chan struct{}), apiToken: token, serial: serial}
	m.procs[id] = job
	m.workers.Add(1)
	m.mu.Unlock()
	w, e := m.Artifacts.Create(id, dispatch, options)
	if e != nil {
		m.database.FinishRunningTask(id, "failed", -1, e.Error())
		m.Artifacts.FailedWithoutLog(id, e)
		m.remove(id, job)
		rec, _ := m.database.GetTaskByID(id)
		return rec, e
	}
	w.Event("任务已登记，准备执行")
	fail := func(err error) (*db.RunningTaskRecord, error) {
		m.finish(id, job, w, -1, err.Error())
		rec, _ := m.database.GetTaskByID(id)
		return rec, err
	}
	m.Artifacts.PrepareRecording(lifecycle, id, serial, timeout, w)
	if lifecycle.Err() != nil {
		return fail(fmt.Errorf("任务已终止"))
	}
	directory, e := os.MkdirTemp("", "mdscript_run_*")
	if e != nil {
		return fail(e)
	}
	if e = pythonsdk.Install(directory); e != nil {
		os.RemoveAll(directory)
		return fail(e)
	}
	tmp, e := os.Create(filepath.Join(directory, "script.py"))
	if e != nil {
		os.RemoveAll(directory)
		return fail(fmt.Errorf("创建临时运行脚本失败: %w", e))
	}
	path := tmp.Name()
	if _, e = tmp.WriteString(python); e != nil {
		tmp.Close()
		os.RemoveAll(directory)
		return fail(e)
	}
	if e = tmp.Close(); e != nil {
		os.RemoveAll(directory)
		return fail(e)
	}
	if e = m.Artifacts.SaveProcessPath(id, path); e != nil {
		os.RemoveAll(directory)
		return fail(e)
	}
	runCtx, runCancel := context.WithTimeout(lifecycle, time.Duration(timeout)*time.Second)
	pythonPath, e := dependencies.PythonPath()
	if e != nil {
		runCancel()
		os.RemoveAll(directory)
		return fail(fmt.Errorf("Python 依赖不可用: %w", e))
	}
	cmd := exec.CommandContext(runCtx, pythonPath, "-u", path, serial)
	cmd.Env = make([]string, 0, len(os.Environ())+5)
	for _, value := range os.Environ() {
		name, _, _ := strings.Cut(value, "=")
		if name == "MYWEBSCRCPY_API_URL" || name == "MYWEBSCRCPY_API_TOKEN" || name == "PYTHONPATH" || name == "PYTHONIOENCODING" || name == "PYTHONUNBUFFERED" {
			continue
		}
		cmd.Env = append(cmd.Env, value)
	}
	cmd.Env = append(cmd.Env, "PYTHONPATH="+directory, "PYTHONIOENCODING=utf-8", "PYTHONUNBUFFERED=1", "MYWEBSCRCPY_API_URL="+m.apiURL, "MYWEBSCRCPY_API_TOKEN="+token)
	cmd.Stdout = w.Writer("stdout")
	cmd.Stderr = w.Writer("stderr")
	configureTaskProcess(cmd)
	cmd.Cancel = func() error { return terminateTaskProcess(cmd.Process.Pid) }
	cmd.WaitDelay = 2 * time.Second
	m.mu.Lock()
	if job.killed {
		m.mu.Unlock()
		runCancel()
		os.RemoveAll(directory)
		return fail(fmt.Errorf("任务已终止"))
	}
	if e = cmd.Start(); e != nil {
		m.mu.Unlock()
		runCancel()
		os.RemoveAll(directory)
		return fail(fmt.Errorf("启动进程失败: %w", e))
	}
	job.cmd = cmd
	if e = m.database.StartReservedTask(id, cmd.Process.Pid); e != nil {
		job.cancel()
		m.mu.Unlock()
		cmd.Wait()
		runCancel()
		os.RemoveAll(directory)
		return fail(e)
	}
	m.mu.Unlock()
	m.Artifacts.Started(id)
	w.Event(fmt.Sprintf("Python 已启动，PID %d", cmd.Process.Pid))
	rec, e := m.database.GetTaskByID(id)
	go func() {
		defer runCancel()
		defer os.RemoveAll(directory)
		waitErr := cmd.Wait()
		code := 0
		msg := ""
		if waitErr != nil {
			code = -1
			msg = waitErr.Error()
			var exit *exec.ExitError
			if errors.As(waitErr, &exit) {
				code = exit.ExitCode()
			}
			if runCtx.Err() == context.DeadlineExceeded {
				msg = fmt.Sprintf("执行超时 (%d 秒)", timeout)
			}
		}
		m.finish(id, job, w, code, msg)
	}()
	return rec, e
}
func (m *Manager) finish(id string, job *managedTask, w *taskartifact.LogCapture, code int, message string) {
	m.mu.Lock()
	status := "completed"
	if code != 0 || message != "" {
		status = "failed"
	}
	if previous, _ := m.database.GetTaskByID(id); previous != nil && previous.Status == "killed" {
		job.killed = true
	}
	if job.killed {
		status = "killed"
		code = -9
		message = "手动终止进程"
	}
	if e := m.database.FinishRunningTask(id, status, code, message); e != nil {
		log.Printf("[TaskManager] task=%s 记录结果失败: %v", id, e)
	}
	m.mu.Unlock()
	rec, e := m.database.GetTaskByID(id)
	if e == nil && rec != nil {
		m.Artifacts.Finish(rec, w)
		if e = m.database.ReconcileScriptRunStatus(rec.ScriptID); e != nil {
			log.Printf("[TaskManager] script=%d 汇总失败: %v", rec.ScriptID, e)
		}
	} else {
		w.Close()
	}
	m.remove(id, job)
	log.Printf("[TaskManager] task=%s status=%s exit=%d", id, status, code)
}
func (m *Manager) remove(id string, job *managedTask) {
	job.cancel()
	m.mu.Lock()
	delete(m.procs, id)
	close(job.done)
	m.mu.Unlock()
	m.workers.Done()
}

// KillTask waits for the actual process exit and artifact finalization before releasing callers.
func (m *Manager) KillTask(id string) error {
	m.mu.Lock()
	task, e := m.database.GetTaskByID(id)
	if e != nil {
		m.mu.Unlock()
		return e
	}
	if task == nil {
		m.mu.Unlock()
		return fmt.Errorf("任务不存在")
	}
	job := m.procs[id]
	if job == nil {
		m.mu.Unlock()
		if task.Status != "running" && task.Status != "stopping" && task.Status != "preparing" {
			return fmt.Errorf("任务非运行状态: %s", task.Status)
		}
		if task.PID > 0 {
			terminateStoredTaskProcess(task.PID)
		}
		if e := m.database.FinishRunningTask(id, "killed", -9, "手动终止进程"); e != nil {
			return e
		}
		return m.database.ReconcileScriptRunStatus(task.ScriptID)
	}
	job.killed = true
	e = m.database.StopTask(id)
	job.cancel()
	m.mu.Unlock()
	if e != nil {
		return e
	}
	<-job.done
	return nil
}
func (m *Manager) GetActiveTaskByDevice(serial string) (*db.RunningTaskRecord, error) {
	return m.database.GetActiveTaskByDevice(serial)
}
func (m *Manager) ListActiveTasks() ([]db.RunningTaskRecord, error) {
	return m.database.ListActiveTasks()
}

// BeginMaintenance atomically rejects new scheduled tasks while requiring idle state.
func (m *Manager) BeginMaintenance() error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if len(m.procs) != 0 {
		return fmt.Errorf("请等待正在运行的脚本任务完成后再更新")
	}
	m.maintenance = true
	return nil
}
func (m *Manager) EndMaintenance() { m.mu.Lock(); m.maintenance = false; m.mu.Unlock() }
