package mdscript

import (
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"log"
	"mywebscrcpy/internal/taskartifact"
	"sync"
	"time"

	"mywebscrcpy/internal/db"
	"mywebscrcpy/internal/taskmanager"
)

type DeviceRunResult struct {
	DispatchID   string                `json:"dispatch_id"`
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
	ArtifactCheck func(db.MarkdownScriptRecord) error
	deviceLookup  func(string) (known bool, online bool)
	templateCheck func(serial, id string) error
}

func NewExecutor(database *db.DB, manager *taskmanager.Manager, deviceLookup func(string) (bool, bool), templateCheck func(string, string) error) *Executor {
	return &Executor{database: database, manager: manager, deviceLookup: deviceLookup, templateCheck: templateCheck}
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
	return e.ExecuteWithTrigger(scriptID, "manual")
}

func (e *Executor) ExecuteWithTrigger(scriptID int64, trigger string) (results []DeviceRunResult, runErr error) {
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
	options, err := ParseOptions(script.Content)
	if err != nil {
		return nil, err
	}
	if e.ArtifactCheck == nil {
		return nil, fmt.Errorf("Python 产物校验服务不可用")
	}
	if err := e.ArtifactCheck(*script); err != nil {
		return nil, err
	}
	dispatchID, runAt, err := e.database.BeginScriptDispatch(script.ID)
	if err != nil {
		return nil, err
	}
	defer func() {
		message := ""
		if runErr != nil {
			message = runErr.Error()
		}
		if err := e.database.FinishDispatch(dispatchID, message); err != nil {
			log.Printf("[MdScript] dispatch=%s 收尾失败: %v", dispatchID, err)
		}
		if runErr != nil {
			if err := e.database.UpdateScriptRunStatus(script.ID, "failed", runAt); err != nil {
				log.Printf("[MdScript] script=%d 记录批次失败状态失败: %v", script.ID, err)
			}
			log.Printf("[MdScript] script=%d dispatch=%s failed: %v", script.ID, dispatchID, runErr)
		}
	}()
	meta := ParseMarkdown(script.Content, time.UnixMilli(script.UpdatedAt))
	snapshot := taskartifact.Snapshot{Availability: "available", ScriptName: script.Name, Document: script.Content, DocumentHash: meta.ContentHash, DocumentVersion: meta.ExtractedVersion, TargetDevices: script.TargetDevices, Options: options}
	if err := e.database.SaveDispatchSnapshot(dispatchID, trigger, snapshot); err != nil {
		return nil, err
	}
	python := script.CompiledPython
	compiledHash := sha256.Sum256([]byte(python))
	if err := e.database.SetDispatchCompiledHash(dispatchID, hex.EncodeToString(compiledHash[:])); err != nil {
		return nil, err
	}
	results = make([]DeviceRunResult, 0, len(script.TargetDevices))
	for _, serial := range script.TargetDevices {
		result := DeviceRunResult{DispatchID: dispatchID, DeviceSerial: serial, Status: "skipped"}
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
			task, err := e.manager.StartDispatchedTask(script.ID, script.Name, serial, meta.TimeoutSec, python, dispatchID, options)
			if err != nil {
				result.Task = task
				result.Error = err.Error()
				if !errors.Is(err, taskmanager.ErrDeviceBusy) {
					result.Status = "failed"
				}
			} else {
				result.Task = task
				result.Status = "started"
			}
		}
		if result.Error != "" && result.Task == nil {
			task, err := e.database.RecordSkippedTask(script.ID, script.Name, serial, result.Status, result.Error)
			if err != nil {
				return results, fmt.Errorf("记录设备执行结果失败: %w", err)
			}
			result.Task = task
			if err := e.database.AttachDispatchTask(dispatchID, task.ID); err != nil {
				return results, err
			}
			if err := e.manager.Artifacts.Rejected(task, dispatchID, options); err != nil {
				return results, err
			}
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
	if err := e.database.FinishDispatch(dispatchID, ""); err != nil {
		return results, err
	}
	if err := e.database.ReconcileScriptRunStatus(script.ID); err != nil {
		return results, err
	}
	return results, nil
}
