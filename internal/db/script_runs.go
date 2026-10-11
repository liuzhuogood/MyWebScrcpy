package db

import (
	"database/sql"
	"fmt"
	"time"
)

func (d *DB) ListScriptRuns(scriptID int64) ([]RunningTaskRecord, error) {
	d.mu.RLock()
	defer d.mu.RUnlock()
	rows, err := d.db.Query(`SELECT id,script_id,script_name,device_serial,pid,status,exit_code,error_msg,started_at,ended_at FROM running_tasks WHERE script_id=? ORDER BY started_at DESC,id DESC LIMIT 100`, scriptID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	items := []RunningTaskRecord{}
	for rows.Next() {
		var item RunningTaskRecord
		if err := rows.Scan(&item.ID, &item.ScriptID, &item.ScriptName, &item.DeviceSerial, &item.PID, &item.Status, &item.ExitCode, &item.ErrorMsg, &item.StartedAt, &item.EndedAt); err != nil {
			return nil, err
		}
		items = append(items, item)
	}
	return items, rows.Err()
}

func (d *DB) RecordSkippedTask(scriptID int64, name, serial, status, message string) (*RunningTaskRecord, error) {
	d.mu.Lock()
	defer d.mu.Unlock()
	now := time.Now().UnixMilli()
	id := fmt.Sprintf("attempt_%d", time.Now().UnixNano())
	_, err := d.db.Exec(`INSERT INTO running_tasks(id,script_id,script_name,device_serial,pid,status,exit_code,error_msg,started_at,ended_at) VALUES(?,?,?,?,0,?,-1,?,?,?)`, id, scriptID, name, serial, status, message, now, now)
	if err != nil {
		return nil, err
	}
	return &RunningTaskRecord{ID: id, ScriptID: scriptID, ScriptName: name, DeviceSerial: serial, Status: status, ExitCode: -1, ErrorMsg: message, StartedAt: now, EndedAt: now}, nil
}

// ReconcileScriptRunStatus summarizes the latest dispatch, not merely a successful process start.
func (d *DB) ReconcileScriptRunStatus(scriptID int64) error {
	d.mu.Lock()
	defer d.mu.Unlock()
	var dispatchID string
	if err := d.db.QueryRow(`SELECT id FROM script_dispatches WHERE script_id=? ORDER BY started_ns DESC LIMIT 1`, scriptID).Scan(&dispatchID); err != nil {
		if err == sql.ErrNoRows {
			return nil
		}
		return err
	}
	rows, err := d.db.Query(`SELECT t.status FROM script_dispatch_tasks m JOIN running_tasks t ON t.id=m.task_id WHERE m.dispatch_id=?`, dispatchID)
	if err != nil {
		return err
	}
	var statuses []string
	for rows.Next() {
		var status string
		if err := rows.Scan(&status); err != nil {
			rows.Close()
			return err
		}
		statuses = append(statuses, status)
	}
	if err := rows.Err(); err != nil {
		rows.Close()
		return err
	}
	rows.Close()
	running, completed, failed, killed, skipped := 0, 0, 0, 0, 0
	if len(statuses) == 0 {
		return nil
	}
	for _, value := range statuses {
		switch value {
		case "running", "stopping":
			running++
		case "completed":
			completed++
		case "failed":
			failed++
		case "killed":
			killed++
		case "skipped":
			skipped++
		default:
			return fmt.Errorf("未知运行状态: %s", value)
		}
	}
	status := "completed"
	if running > 0 {
		status = "started"
		if failed+killed+skipped > 0 {
			status = "partial"
		}
	} else if completed > 0 && failed+killed+skipped > 0 {
		status = "partial"
	} else if failed > 0 {
		status = "failed"
	} else if killed > 0 {
		status = "killed"
	} else if skipped > 0 {
		status = "skipped"
	}
	_, err = d.db.Exec(`UPDATE markdown_scripts SET last_run_status=? WHERE id=?`, status, scriptID)
	return err
}

// BeginScriptDispatch gives every manual/cron batch an explicit identity, even when timestamps coincide.
func (d *DB) BeginScriptDispatch(scriptID int64) (string, int64, error) {
	d.mu.Lock()
	defer d.mu.Unlock()
	now := time.Now()
	id := fmt.Sprintf("dispatch_%d", now.UnixNano())
	tx, err := d.db.Begin()
	if err != nil {
		return "", 0, err
	}
	defer tx.Rollback()
	if _, err = tx.Exec(`INSERT INTO script_dispatches(id,script_id,started_ns) VALUES(?,?,?)`, id, scriptID, now.UnixNano()); err != nil {
		return "", 0, err
	}
	if _, err = tx.Exec(`UPDATE markdown_scripts SET last_run_at=?,last_run_status='started' WHERE id=?`, now.UnixMilli(), scriptID); err != nil {
		return "", 0, err
	}
	if err := tx.Commit(); err != nil {
		return "", 0, err
	}
	return id, now.UnixMilli(), nil
}

func (d *DB) AttachDispatchTask(dispatchID, taskID string) error {
	d.mu.Lock()
	defer d.mu.Unlock()
	_, err := d.db.Exec(`INSERT INTO script_dispatch_tasks(dispatch_id,task_id) VALUES(?,?)`, dispatchID, taskID)
	return err
}
