package taskmanager

import (
	"context"
	"errors"
	"fmt"
	"log"
	"os"
	"os/exec"
	"sync"
	"syscall"
	"time"

	"mywebscrcpy/internal/db"
)

var ErrDeviceBusy = errors.New("设备正在运行其他任务")

// Manager 任务管理器，统一管理 Python 进程生命周期与设备排他锁
type Manager struct {
	database *db.DB
	mu       sync.Mutex
	procs    map[string]*exec.Cmd
}

func NewManager(database *db.DB) *Manager {
	return &Manager{
		database: database,
		procs:    make(map[string]*exec.Cmd),
	}
}

// StartTask 启动托管任务 (核心逻辑：检查设备互斥锁 -> 注册 PID -> 启动进程 -> 监听退出)
func (m *Manager) StartTask(scriptID int64, scriptName, deviceSerial string, timeoutSec int, pythonCode string) (*db.RunningTaskRecord, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	if err := db.ValidateTargetDevices([]string{deviceSerial}); err != nil {
		return nil, err
	}
	// 1. 检查设备排他锁
	active, err := m.database.GetActiveTaskByDevice(deviceSerial)
	if err != nil {
		return nil, fmt.Errorf("检查设备排他锁失败: %w", err)
	}
	if active != nil {
		return nil, fmt.Errorf("%w：设备 [%s] 正被任务「%s」占用", ErrDeviceBusy, deviceSerial, active.ScriptName)
	}

	taskID := fmt.Sprintf("task_%d", time.Now().UnixNano())

	// 2. 准备执行隔离脚本
	tmpFile, err := os.CreateTemp("", "mdscript_run_*.py")
	if err != nil {
		return nil, fmt.Errorf("创建临时运行脚本失败: %w", err)
	}
	defer tmpFile.Close()

	if _, err := tmpFile.WriteString(pythonCode); err != nil {
		_ = os.Remove(tmpFile.Name())
		return nil, fmt.Errorf("写入运行脚本失败: %w", err)
	}
	scriptPath := tmpFile.Name()

	// 设置超时 context
	if timeoutSec <= 0 {
		timeoutSec = 120
	}
	ctx, cancel := context.WithTimeout(context.Background(), time.Duration(timeoutSec)*time.Second)

	cmd := exec.CommandContext(ctx, "python3", scriptPath, deviceSerial)
	cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}
	cmd.Cancel = func() error { return syscall.Kill(-cmd.Process.Pid, syscall.SIGKILL) }
	cmd.WaitDelay = 2 * time.Second

	if err := cmd.Start(); err != nil {
		cancel()
		_ = os.Remove(scriptPath)
		return nil, fmt.Errorf("启动进程失败: %w", err)
	}

	pid := cmd.Process.Pid
	m.procs[taskID] = cmd

	// 3. 在数据库登记任务，立即对目标设备施加排他锁
	if err := m.database.InsertRunningTask(taskID, scriptID, scriptName, deviceSerial, pid); err != nil {
		_ = syscall.Kill(-cmd.Process.Pid, syscall.SIGKILL)
		cancel()
		_ = os.Remove(scriptPath)
		delete(m.procs, taskID)
		_ = cmd.Wait()
		return nil, fmt.Errorf("注册任务到数据库失败: %w", err)
	}

	// 4. 异步监听进程生命周期并在结束后释放锁
	go func() {
		defer cancel()
		defer func() {
			_ = os.Remove(scriptPath)
			m.mu.Lock()
			delete(m.procs, taskID)
			m.mu.Unlock()
		}()

		waitErr := cmd.Wait()
		exitCode := 0
		status := "completed"
		errMsg := ""

		if waitErr != nil {
			if ctx.Err() == context.DeadlineExceeded {
				status = "failed"
				errMsg = fmt.Sprintf("执行超时 (%d 秒)", timeoutSec)
				exitCode = -1
			} else if exitErr, ok := waitErr.(*exec.ExitError); ok {
				status = "failed"
				exitCode = exitErr.ExitCode()
				errMsg = exitErr.Error()
			} else {
				status = "failed"
				errMsg = waitErr.Error()
			}
		}

		m.mu.Lock()
		// 检查是否已被手动强杀 (如果是 killed 则保留 killed 状态)
		rec, readErr := m.database.GetTaskByID(taskID)
		if readErr != nil {
			log.Printf("[TaskManager] task=%s 读取结束状态失败: %v", taskID, readErr)
		}
		if rec != nil && rec.Status == "killed" {
			m.mu.Unlock()
			return
		}
		if err := m.database.FinishRunningTask(taskID, status, exitCode, errMsg); err != nil {
			log.Printf("[TaskManager] task=%s 记录结束状态失败: %v", taskID, err)
		}
		m.mu.Unlock()
		if err := m.database.ReconcileScriptRunStatus(scriptID); err != nil {
			log.Printf("[TaskManager] script=%d 汇总结果失败: %v", scriptID, err)
		}
		log.Printf("[TaskManager] 任务 %s (PID: %d) 结束，状态: %s，设备 %s 已释放锁", taskID, pid, status, deviceSerial)
	}()

	return &db.RunningTaskRecord{
		ID:           taskID,
		ScriptID:     scriptID,
		ScriptName:   scriptName,
		DeviceSerial: deviceSerial,
		PID:          pid,
		Status:       "running",
		StartedAt:    time.Now().UnixMilli(),
	}, nil
}

// KillTask 强制结束任务进程并立即释放设备锁
func (m *Manager) KillTask(taskID string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	task, err := m.database.GetTaskByID(taskID)
	if err != nil {
		return fmt.Errorf("查询任务失败: %w", err)
	}
	if task == nil {
		return fmt.Errorf("任务不存在")
	}

	if task.Status != "running" && task.Status != "stopping" {
		return fmt.Errorf("任务非运行状态: %s", task.Status)
	}

	// 杀掉系统进程
	if cmd, ok := m.procs[taskID]; ok && cmd.Process != nil {
		_ = syscall.Kill(-cmd.Process.Pid, syscall.SIGKILL)
	} else if task.PID > 0 {
		_ = syscall.Kill(-task.PID, syscall.SIGKILL)
		_ = syscall.Kill(task.PID, syscall.SIGKILL)
	}

	// 立即将状态置为 killed，设备锁即刻释放
	if err := m.database.FinishRunningTask(taskID, "killed", -9, "手动终止进程"); err != nil {
		return fmt.Errorf("更新任务状态失败: %w", err)
	}

	delete(m.procs, taskID)
	if err := m.database.ReconcileScriptRunStatus(task.ScriptID); err != nil {
		log.Printf("[TaskManager] script=%d 汇总终止结果失败: %v", task.ScriptID, err)
	}
	log.Printf("[TaskManager] 任务 %s (PID: %d) 已被强杀，设备 %s 锁已释放", taskID, task.PID, task.DeviceSerial)
	return nil
}

// GetActiveTaskByDevice 查询指定设备是否有活跃任务 (设备互斥排他查询)
func (m *Manager) GetActiveTaskByDevice(deviceSerial string) (*db.RunningTaskRecord, error) {
	return m.database.GetActiveTaskByDevice(deviceSerial)
}

// ListActiveTasks 获取当前活跃任务
func (m *Manager) ListActiveTasks() ([]db.RunningTaskRecord, error) {
	return m.database.ListActiveTasks()
}
