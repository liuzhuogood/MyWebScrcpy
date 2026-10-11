package mdscript

import (
	"errors"
	"fmt"
	"log"
	"sync"
	"time"

	"mywebscrcpy/internal/db"
	"mywebscrcpy/internal/taskmanager"
)

type DeviceRunResult struct {
	DeviceSerial string                `json:"device_serial"`
	Status       string                `json:"status"`
	Task         *db.RunningTaskRecord `json:"task,omitempty"`
	Error        string                `json:"error"`
}

// Executor shares the saved device snapshot and preflight checks across manual and scheduled runs.
type Executor struct {
	mu            sync.Mutex
	database      *db.DB
	manager       *taskmanager.Manager
	compiler      *Compiler
	deviceLookup  func(string) (known bool, online bool)
	templateCheck func(serial, id string) error
}

func NewExecutor(database *db.DB, manager *taskmanager.Manager, deviceLookup func(string) (bool, bool), templateCheck func(string, string) error) *Executor {
	return &Executor{database: database, manager: manager, compiler: NewCompiler(), deviceLookup: deviceLookup, templateCheck: templateCheck}
}

func (e *Executor) ValidateTargets(targets []string) error {
	if err := db.ValidateTargetDevices(targets); err != nil {
		return err
	}
	for _, serial := range targets {
		known, _ := e.deviceLookup(serial)
		if !known {
			return fmt.Errorf("目标设备不存在: %s", serial)
		}
	}
	return nil
}

func (e *Executor) Execute(scriptID int64) (results []DeviceRunResult, runErr error) {
	e.mu.Lock()
	defer e.mu.Unlock()
	script, err := e.database.GetMarkdownScript(scriptID)
	if err != nil {
		return nil, err
	}
	if script == nil {
		return nil, fmt.Errorf("脚本不存在")
	}
	if err := db.ValidateTargetDevices(script.TargetDevices); err != nil {
		return nil, fmt.Errorf("请先配置目标设备: %w", err)
	}
	dispatchID, runAt, err := e.database.BeginScriptDispatch(script.ID)
	if err != nil {
		return nil, err
	}
	defer func() {
		if runErr != nil {
			if err := e.database.UpdateScriptRunStatus(script.ID, "failed", runAt); err != nil {
				log.Printf("[MdScript] script=%d 记录批次失败状态失败: %v", script.ID, err)
			}
			log.Printf("[MdScript] script=%d dispatch=%s failed: %v", script.ID, dispatchID, runErr)
		}
	}()
	meta := ParseMarkdown(script.Content, time.UnixMilli(script.UpdatedAt))
	python := script.CompiledPython
	if e.compiler.ShouldRecompile(meta.ExtractedVersion, script.CompiledVersion, python) {
		python, err = e.compiler.Compile(script.Name, meta, script.Content)
		if err != nil {
			return nil, fmt.Errorf("执行前编译失败: %w", err)
		}
		if err := e.database.SaveCompiledPython(script.ID, meta.ExtractedVersion, python); err != nil {
			return nil, err
		}
	}
	results = make([]DeviceRunResult, 0, len(script.TargetDevices))
	for _, serial := range script.TargetDevices {
		result := DeviceRunResult{DeviceSerial: serial, Status: "skipped"}
		known, online := e.deviceLookup(serial)
		if !known {
			result.Error = "目标设备不存在"
		} else if !online {
			result.Error = "目标设备离线"
		}
		if result.Error == "" {
			for _, ref := range meta.Templates {
				if err := e.templateCheck(serial, ref.ID); err != nil {
					result.Status = "failed"
					result.Error = err.Error()
					break
				}
			}
		}
		if result.Error == "" {
			task, err := e.manager.StartTask(script.ID, script.Name, serial, meta.TimeoutSec, python)
			if err != nil {
				result.Error = err.Error()
				if !errors.Is(err, taskmanager.ErrDeviceBusy) {
					result.Status = "failed"
				}
			} else {
				result.Task = task
				result.Status = "started"
			}
		}
		if result.Error != "" {
			task, err := e.database.RecordSkippedTask(script.ID, script.Name, serial, result.Status, result.Error)
			if err != nil {
				return results, fmt.Errorf("记录设备执行结果失败: %w", err)
			}
			result.Task = task
		}
		if err := e.database.AttachDispatchTask(dispatchID, result.Task.ID); err != nil {
			return results, fmt.Errorf("记录执行批次失败: %w", err)
		}
		log.Printf("[MdScript] script=%d device=%s status=%s error=%s", script.ID, serial, result.Status, result.Error)
		results = append(results, result)
	}
	status := "started"
	started, failed := 0, 0
	for _, result := range results {
		if result.Status == "started" {
			started++
		}
		if result.Status == "failed" {
			failed++
		}
	}
	if started == 0 {
		status = "skipped"
		if failed > 0 {
			status = "failed"
		}
	} else if started < len(results) {
		status = "partial"
	}
	if err := e.database.UpdateScriptRunStatus(script.ID, status, runAt); err != nil {
		return results, err
	}
	if err := e.database.ReconcileScriptRunStatus(script.ID); err != nil {
		return results, err
	}
	return results, nil
}
