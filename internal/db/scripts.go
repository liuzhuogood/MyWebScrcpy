package db

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"time"
)

// MarkdownScriptRecord 对应 SQLite 中的 Markdown 自动化脚本
type MarkdownScriptRecord struct {
	ID                       int64    `json:"id"`
	Name                     string   `json:"name"`
	TargetDevices            []string `json:"target_devices"`
	NeedsDeviceConfiguration bool     `json:"needs_device_configuration"`
	Content                  string   `json:"content"`
	ExtractedVersion         string   `json:"extracted_version"`
	ScheduleCron             string   `json:"schedule_cron"`
	IsEnabled                int      `json:"is_enabled"`
	TimeoutSec               int      `json:"timeout_sec"`
	CompiledVersion          string   `json:"compiled_version"`
	CompiledPython           string   `json:"compiled_python"`
	CompiledAt               int64    `json:"compiled_at"`
	LastRunAt                int64    `json:"last_run_at"`
	LastRunStatus            string   `json:"last_run_status"`
	CreatedAt                int64    `json:"created_at"`
	UpdatedAt                int64    `json:"updated_at"`
	NeedsGeneration          bool     `json:"needs_generation"`
}

// RunningTaskRecord 对应 SQLite 中的任务管理器活跃/历史任务
type RunningTaskRecord struct {
	ID           string `json:"id"`
	ScriptID     int64  `json:"script_id"`
	ScriptName   string `json:"script_name"`
	DeviceSerial string `json:"device_serial"`
	PID          int    `json:"pid"`
	Status       string `json:"status"` // "running", "stopping", "completed", "failed", "killed"
	ExitCode     int    `json:"exit_code"`
	ErrorMsg     string `json:"error_msg"`
	StartedAt    int64  `json:"started_at"`
	EndedAt      int64  `json:"ended_at"`
}

// ListMarkdownScripts 获取所有 Markdown 脚本
func (d *DB) ListMarkdownScripts() ([]MarkdownScriptRecord, error) {
	d.mu.RLock()
	defer d.mu.RUnlock()

	query := `SELECT id, name, target_devices, content, extracted_version, schedule_cron,
	                 is_enabled, timeout_sec, compiled_version, compiled_python,
	                 compiled_at, last_run_at, last_run_status, created_at, updated_at
	          FROM markdown_scripts ORDER BY id DESC`
	rows, err := d.db.Query(query)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var list []MarkdownScriptRecord
	for rows.Next() {
		var item MarkdownScriptRecord
		var encodedTargets string
		err := rows.Scan(
			&item.ID, &item.Name, &encodedTargets, &item.Content, &item.ExtractedVersion,
			&item.ScheduleCron, &item.IsEnabled, &item.TimeoutSec, &item.CompiledVersion,
			&item.CompiledPython, &item.CompiledAt, &item.LastRunAt, &item.LastRunStatus,
			&item.CreatedAt, &item.UpdatedAt,
		)
		if err != nil {
			return nil, err
		}
		if err := json.Unmarshal([]byte(encodedTargets), &item.TargetDevices); err != nil {
			return nil, fmt.Errorf("设备配置损坏: %w", err)
		}
		if item.TargetDevices == nil {
			return nil, fmt.Errorf("设备配置损坏: 必须是数组")
		}
		item.NeedsDeviceConfiguration = len(item.TargetDevices) == 0
		list = append(list, item)
	}
	return list, rows.Err()
}

// GetMarkdownScript 根据 ID 获取脚本
func (d *DB) GetMarkdownScript(id int64) (*MarkdownScriptRecord, error) {
	d.mu.RLock()
	defer d.mu.RUnlock()

	query := `SELECT id, name, target_devices, content, extracted_version, schedule_cron,
	                 is_enabled, timeout_sec, compiled_version, compiled_python,
	                 compiled_at, last_run_at, last_run_status, created_at, updated_at
	          FROM markdown_scripts WHERE id = ?`
	var item MarkdownScriptRecord
	var encodedTargets string
	err := d.db.QueryRow(query, id).Scan(
		&item.ID, &item.Name, &encodedTargets, &item.Content, &item.ExtractedVersion,
		&item.ScheduleCron, &item.IsEnabled, &item.TimeoutSec, &item.CompiledVersion,
		&item.CompiledPython, &item.CompiledAt, &item.LastRunAt, &item.LastRunStatus,
		&item.CreatedAt, &item.UpdatedAt,
	)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	if err := json.Unmarshal([]byte(encodedTargets), &item.TargetDevices); err != nil {
		return nil, fmt.Errorf("设备配置损坏: %w", err)
	}
	if item.TargetDevices == nil {
		return nil, fmt.Errorf("设备配置损坏: 必须是数组")
	}
	item.NeedsDeviceConfiguration = len(item.TargetDevices) == 0
	return &item, nil
}

// CreateMarkdownScript 新建脚本
func (d *DB) CreateMarkdownScript(name string, targetDevices []string, content, extractedVer, scheduleCron string, isEnabled, timeoutSec int) (*MarkdownScriptRecord, error) {
	d.mu.Lock()
	defer d.mu.Unlock()

	now := time.Now().UnixMilli()
	query := `INSERT INTO markdown_scripts (
		name, target_devices, content, extracted_version, schedule_cron,
		is_enabled, timeout_sec, compiled_version, compiled_python, compiled_at,
		created_at, updated_at
	) VALUES (?, ?, ?, ?, ?, ?, ?, '', '', 0, ?, ?)`

	if err := ValidateTargetDevices(targetDevices); err != nil {
		return nil, err
	}
	encodedTargets, err := json.Marshal(targetDevices)
	if err != nil {
		return nil, err
	}
	res, err := d.db.Exec(query, name, string(encodedTargets), content, extractedVer, scheduleCron, isEnabled, timeoutSec, now, now)
	if err != nil {
		return nil, fmt.Errorf("插入脚本失败: %w", err)
	}
	id, err := res.LastInsertId()
	if err != nil {
		return nil, err
	}

	return &MarkdownScriptRecord{
		ID:               id,
		Name:             name,
		TargetDevices:    append([]string{}, targetDevices...),
		Content:          content,
		ExtractedVersion: extractedVer,
		ScheduleCron:     scheduleCron,
		IsEnabled:        isEnabled,
		TimeoutSec:       timeoutSec,
		CompiledVersion:  "",
		CompiledPython:   "",
		CompiledAt:       0,
		CreatedAt:        now,
		UpdatedAt:        now,
	}, nil
}

// UpdateMarkdownScript 更新脚本内容及提取元数据
func (d *DB) UpdateMarkdownScript(id int64, name string, targetDevices []string, content, extractedVer, scheduleCron string, isEnabled, timeoutSec int) error {
	d.mu.Lock()
	defer d.mu.Unlock()

	now := time.Now().UnixMilli()
	query := `UPDATE markdown_scripts SET
		name = ?, target_devices = ?, content = ?, extracted_version = ?,
		schedule_cron = ?, is_enabled = ?, timeout_sec = ?, updated_at = MAX(?, updated_at + 1)
		WHERE id = ?`
	if err := ValidateTargetDevices(targetDevices); err != nil {
		return err
	}
	encodedTargets, err := json.Marshal(targetDevices)
	if err != nil {
		return err
	}
	res, err := d.db.Exec(query, name, string(encodedTargets), content, extractedVer, scheduleCron, isEnabled, timeoutSec, now, id)
	if err != nil {
		return err
	}
	n, err := res.RowsAffected()
	if err != nil {
		return err
	}
	if n == 0 {
		return fmt.Errorf("脚本不存在")
	}
	return nil
}

// SaveCompiledPython 保存编译生成的 Python 产物
func (d *DB) SaveCompiledPython(id int64, compiledVer, compiledPython string) error {
	d.mu.Lock()
	defer d.mu.Unlock()

	now := time.Now().UnixMilli()
	query := `UPDATE markdown_scripts SET
		compiled_version = ?, compiled_python = ?, compiled_at = ?
		WHERE id = ?`
	res, err := d.db.Exec(query, compiledVer, compiledPython, now, id)
	if err != nil {
		return err
	}
	n, err := res.RowsAffected()
	if err != nil {
		return err
	}
	if n == 0 {
		return fmt.Errorf("脚本不存在")
	}
	return nil
}

// UpdateScriptRunStatus 更新脚本最近一次执行/跳过状态
func (d *DB) UpdateScriptRunStatus(id int64, status string, timestamp int64) error {
	d.mu.Lock()
	defer d.mu.Unlock()

	query := `UPDATE markdown_scripts SET last_run_at = ?, last_run_status = ? WHERE id = ?`
	_, err := d.db.Exec(query, timestamp, status, id)
	return err
}

// ToggleScriptEnabled 单独快速切换脚本启用/停用状态
func (d *DB) ToggleScriptEnabled(id int64, isEnabled int) error {
	d.mu.Lock()
	defer d.mu.Unlock()

	if isEnabled == 1 {
		var encoded string
		if err := d.db.QueryRow("SELECT target_devices FROM markdown_scripts WHERE id=?", id).Scan(&encoded); err != nil {
			return err
		}
		var targets []string
		if err := json.Unmarshal([]byte(encoded), &targets); err != nil {
			return err
		}
		if err := ValidateTargetDevices(targets); err != nil {
			return fmt.Errorf("请先配置目标设备: %w", err)
		}
	}
	query := `UPDATE markdown_scripts SET is_enabled = ? WHERE id = ?`
	res, err := d.db.Exec(query, isEnabled, id)
	if err != nil {
		return err
	}
	n, err := res.RowsAffected()
	if err != nil {
		return err
	}
	if n == 0 {
		return fmt.Errorf("脚本不存在")
	}
	return nil
}

// DeleteMarkdownScript 删除脚本
func (d *DB) DeleteMarkdownScript(id int64) error {
	d.mu.Lock()
	defer d.mu.Unlock()

	res, err := d.db.Exec("DELETE FROM markdown_scripts WHERE id = ?", id)
	if err != nil {
		return err
	}
	n, err := res.RowsAffected()
	if err != nil {
		return err
	}
	if n == 0 {
		return fmt.Errorf("脚本不存在")
	}
	return nil
}

// ==================== 任务管理器 (running_tasks) ====================

// ListActiveTasks 获取所有处于 running 或 stopping 状态的活跃任务
func (d *DB) ListActiveTasks() ([]RunningTaskRecord, error) {
	d.mu.RLock()
	defer d.mu.RUnlock()

	query := `SELECT id, script_id, script_name, device_serial, pid, status,
	                 exit_code, error_msg, started_at, ended_at
	          FROM running_tasks WHERE status IN ('preparing', 'running', 'stopping')
	          ORDER BY started_at DESC`
	rows, err := d.db.Query(query)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var list []RunningTaskRecord
	for rows.Next() {
		var item RunningTaskRecord
		err := rows.Scan(
			&item.ID, &item.ScriptID, &item.ScriptName, &item.DeviceSerial, &item.PID,
			&item.Status, &item.ExitCode, &item.ErrorMsg, &item.StartedAt, &item.EndedAt,
		)
		if err != nil {
			return nil, err
		}
		list = append(list, item)
	}
	return list, rows.Err()
}

// GetActiveTaskByDevice 查询指定设备上是否有正在运行的任务 (设备互斥锁判断)
func (d *DB) GetActiveTaskByDevice(deviceSerial string) (*RunningTaskRecord, error) {
	d.mu.RLock()
	defer d.mu.RUnlock()

	query := `SELECT id, script_id, script_name, device_serial, pid, status,
	                 exit_code, error_msg, started_at, ended_at
	          FROM running_tasks
	          WHERE device_serial = ? AND status IN ('preparing', 'running', 'stopping')
	          LIMIT 1`
	var item RunningTaskRecord
	err := d.db.QueryRow(query, deviceSerial).Scan(
		&item.ID, &item.ScriptID, &item.ScriptName, &item.DeviceSerial, &item.PID,
		&item.Status, &item.ExitCode, &item.ErrorMsg, &item.StartedAt, &item.EndedAt,
	)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return &item, nil
}

// GetTaskByID 根据 ID 获取任务记录
func (d *DB) GetTaskByID(taskID string) (*RunningTaskRecord, error) {
	d.mu.RLock()
	defer d.mu.RUnlock()

	query := `SELECT id, script_id, script_name, device_serial, pid, status,
	                 exit_code, error_msg, started_at, ended_at
	          FROM running_tasks WHERE id = ?`
	var item RunningTaskRecord
	err := d.db.QueryRow(query, taskID).Scan(
		&item.ID, &item.ScriptID, &item.ScriptName, &item.DeviceSerial, &item.PID,
		&item.Status, &item.ExitCode, &item.ErrorMsg, &item.StartedAt, &item.EndedAt,
	)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return &item, nil
}

// InsertRunningTask 注册新任务
func (d *DB) InsertRunningTask(id string, scriptID int64, scriptName, deviceSerial string, pid int) error {
	d.mu.Lock()
	defer d.mu.Unlock()

	now := time.Now().UnixMilli()
	query := `INSERT INTO running_tasks (
		id, script_id, script_name, device_serial, pid, status,
		exit_code, error_msg, started_at, ended_at
	) VALUES (?, ?, ?, ?, ?, 'running', 0, '', ?, 0)`
	_, err := d.db.Exec(query, id, scriptID, scriptName, deviceSerial, pid, now)
	return err
}

// FinishRunningTask 更新任务完成状态 (完成/失败/强杀)
func (d *DB) FinishRunningTask(id, status string, exitCode int, errorMsg string) error {
	d.mu.Lock()
	defer d.mu.Unlock()

	now := time.Now().UnixMilli()
	query := `UPDATE running_tasks SET
		status = ?, exit_code = ?, error_msg = ?, ended_at = ?
		WHERE id = ?`
	_, err := d.db.Exec(query, status, exitCode, errorMsg, now, id)
	return err
}
