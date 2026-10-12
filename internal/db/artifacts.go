package db

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"path/filepath"
	"strconv"
	"time"
)

func (d *DB) ArtifactDir() string { return filepath.Join(d.path+".artifacts", "logs") }
func (d *DB) migrateArtifacts() error {
	_, e := d.db.Exec(`CREATE TABLE IF NOT EXISTS dispatch_details(dispatch_id TEXT PRIMARY KEY,snapshot TEXT NOT NULL,trigger TEXT NOT NULL,state TEXT NOT NULL DEFAULT 'dispatching',ended_at INTEGER NOT NULL DEFAULT 0,error_msg TEXT NOT NULL DEFAULT '');
 CREATE TABLE IF NOT EXISTS task_artifacts(task_id TEXT PRIMARY KEY,state TEXT NOT NULL);
 CREATE TABLE IF NOT EXISTS task_reports(task_id TEXT NOT NULL,revision INTEGER NOT NULL,state TEXT NOT NULL,evidence TEXT NOT NULL,prompt TEXT NOT NULL,output TEXT NOT NULL DEFAULT '',input_hash TEXT NOT NULL,error_msg TEXT NOT NULL DEFAULT '',created_at INTEGER NOT NULL,PRIMARY KEY(task_id,revision));
 CREATE INDEX IF NOT EXISTS idx_task_report_queue ON task_reports(state,created_at);`)
	return e
}
func (d *DB) SaveArtifactState(id string, state any) error {
	b, e := json.Marshal(state)
	if e != nil {
		return e
	}
	d.mu.Lock()
	defer d.mu.Unlock()
	_, e = d.db.Exec(`INSERT INTO task_artifacts(task_id,state) VALUES(?,?) ON CONFLICT(task_id) DO UPDATE SET state=excluded.state`, id, string(b))
	return e
}
func (d *DB) ArtifactState(id string, out any) (bool, error) {
	d.mu.RLock()
	defer d.mu.RUnlock()
	var raw string
	e := d.db.QueryRow(`SELECT state FROM task_artifacts WHERE task_id=?`, id).Scan(&raw)
	if e == sql.ErrNoRows {
		return false, nil
	}
	if e != nil {
		return false, e
	}
	return true, json.Unmarshal([]byte(raw), out)
}
func (d *DB) SaveDispatchSnapshot(id, trigger string, snapshot any) error {
	b, e := json.Marshal(snapshot)
	if e != nil {
		return e
	}
	d.mu.Lock()
	defer d.mu.Unlock()
	_, e = d.db.Exec(`INSERT INTO dispatch_details(dispatch_id,snapshot,trigger) VALUES(?,?,?)`, id, string(b), trigger)
	return e
}
func (d *DB) FinishDispatch(id, message string) error {
	d.mu.Lock()
	defer d.mu.Unlock()
	_, e := d.db.Exec(`UPDATE dispatch_details SET state='finished',ended_at=?,error_msg=? WHERE dispatch_id=?`, time.Now().UnixMilli(), message, id)
	return e
}

type DispatchRecord struct {
	ID            string          `json:"id"`
	Cursor        string          `json:"cursor"`
	ScriptID      int64           `json:"script_id"`
	StartedAt     int64           `json:"started_at"`
	EndedAt       int64           `json:"ended_at"`
	Trigger       string          `json:"trigger"`
	DispatchState string          `json:"dispatch_state"`
	Status        string          `json:"status"`
	ErrorMsg      string          `json:"error_msg"`
	Snapshot      json.RawMessage `json:"snapshot"`
}

func (d *DB) ListDispatches(scriptID int64, cursor int64, limit int) ([]DispatchRecord, error) {
	d.mu.RLock()
	defer d.mu.RUnlock()
	if cursor == 0 {
		cursor = 1<<63 - 1
	}
	rows, e := d.db.Query(`SELECT b.id,b.script_id,b.started_ns,COALESCE(x.ended_at,0),COALESCE(x.trigger,'legacy_unknown'),COALESCE(x.state,'finished'),COALESCE(x.error_msg,''),COALESCE(x.snapshot,'{"availability":"legacy_unavailable","script_name":null,"document":null,"document_hash":null,"compiled_hash":null,"document_version":null,"target_devices":null,"recording":null,"report":null}') FROM script_dispatches b LEFT JOIN dispatch_details x ON x.dispatch_id=b.id WHERE b.script_id=? AND b.started_ns<? ORDER BY b.started_ns DESC LIMIT ?`, scriptID, cursor, limit)
	if e != nil {
		return nil, e
	}
	defer rows.Close()
	items := []DispatchRecord{}
	for rows.Next() {
		var r DispatchRecord
		var raw string
		var ns int64
		if e = rows.Scan(&r.ID, &r.ScriptID, &ns, &r.EndedAt, &r.Trigger, &r.DispatchState, &r.ErrorMsg, &raw); e != nil {
			return nil, e
		}
		r.StartedAt = ns / 1e6
		r.Cursor = strconv.FormatInt(ns, 10)
		r.Snapshot = json.RawMessage(raw)
		items = append(items, r)
	}
	return items, rows.Err()
}
func (d *DB) DispatchTasks(scriptID int64, id string) ([]RunningTaskRecord, error) {
	d.mu.RLock()
	defer d.mu.RUnlock()
	rows, e := d.db.Query(`SELECT t.id,t.script_id,t.script_name,t.device_serial,t.pid,t.status,t.exit_code,t.error_msg,t.started_at,t.ended_at FROM running_tasks t JOIN script_dispatch_tasks m ON m.task_id=t.id JOIN script_dispatches b ON b.id=m.dispatch_id WHERE b.id=? AND b.script_id=? ORDER BY t.started_at,t.id`, id, scriptID)
	if e != nil {
		return nil, e
	}
	defer rows.Close()
	items := []RunningTaskRecord{}
	for rows.Next() {
		var r RunningTaskRecord
		if e = rows.Scan(&r.ID, &r.ScriptID, &r.ScriptName, &r.DeviceSerial, &r.PID, &r.Status, &r.ExitCode, &r.ErrorMsg, &r.StartedAt, &r.EndedAt); e != nil {
			return nil, e
		}
		items = append(items, r)
	}
	return items, rows.Err()
}
func (d *DB) GetDispatch(scriptID int64, id string) (*DispatchRecord, error) {
	d.mu.RLock()
	defer d.mu.RUnlock()
	r := &DispatchRecord{}
	var ns int64
	var raw string
	e := d.db.QueryRow(`SELECT b.id,b.script_id,b.started_ns,COALESCE(x.ended_at,0),COALESCE(x.trigger,'legacy_unknown'),COALESCE(x.state,'finished'),COALESCE(x.error_msg,''),COALESCE(x.snapshot,'{"availability":"legacy_unavailable","script_name":null,"document":null,"document_hash":null,"compiled_hash":null,"document_version":null,"target_devices":null,"recording":null,"report":null}') FROM script_dispatches b LEFT JOIN dispatch_details x ON x.dispatch_id=b.id WHERE b.script_id=? AND b.id=?`, scriptID, id).Scan(&r.ID, &r.ScriptID, &ns, &r.EndedAt, &r.Trigger, &r.DispatchState, &r.ErrorMsg, &raw)
	if e == sql.ErrNoRows {
		return nil, nil
	}
	if e != nil {
		return nil, e
	}
	r.StartedAt = ns / 1e6
	r.Cursor = strconv.FormatInt(ns, 10)
	r.Snapshot = json.RawMessage(raw)
	return r, nil
}
func (d *DB) TaskDispatch(id string) (string, error) {
	d.mu.RLock()
	defer d.mu.RUnlock()
	var value string
	e := d.db.QueryRow(`SELECT dispatch_id FROM script_dispatch_tasks WHERE task_id=?`, id).Scan(&value)
	if e == sql.ErrNoRows {
		return "", nil
	}
	return value, e
}
func (d *DB) DispatchSnapshot(id string, out any) error {
	d.mu.RLock()
	defer d.mu.RUnlock()
	var raw string
	if e := d.db.QueryRow(`SELECT snapshot FROM dispatch_details WHERE dispatch_id=?`, id).Scan(&raw); e != nil {
		return e
	}
	return json.Unmarshal([]byte(raw), out)
}

// ReserveTask atomically claims the serial and associates the batch before any external work.
func (d *DB) ReserveTask(id string, scriptID int64, name, serial, dispatchID string) error {
	d.mu.Lock()
	defer d.mu.Unlock()
	tx, e := d.db.Begin()
	if e != nil {
		return e
	}
	defer tx.Rollback()
	var active int
	if e = tx.QueryRow(`SELECT COUNT(*) FROM running_tasks WHERE device_serial=? AND status IN ('preparing','running','stopping')`, serial).Scan(&active); e != nil {
		return e
	}
	if active > 0 {
		return fmt.Errorf("设备正在运行其他任务")
	}
	if _, e = tx.Exec(`INSERT INTO running_tasks(id,script_id,script_name,device_serial,pid,status,started_at) VALUES(?,?,?,?,0,'preparing',?)`, id, scriptID, name, serial, time.Now().UnixMilli()); e != nil {
		return e
	}
	if dispatchID != "" {
		if _, e = tx.Exec(`INSERT INTO script_dispatch_tasks(dispatch_id,task_id) VALUES(?,?)`, dispatchID, id); e != nil {
			return e
		}
	}
	return tx.Commit()
}
func (d *DB) StartReservedTask(id string, pid int) error {
	d.mu.Lock()
	defer d.mu.Unlock()
	_, e := d.db.Exec(`UPDATE running_tasks SET pid=?,status='running',started_at=? WHERE id=? AND status='preparing'`, pid, time.Now().UnixMilli(), id)
	return e
}
func (d *DB) StopTask(id string) error {
	d.mu.Lock()
	defer d.mu.Unlock()
	_, e := d.db.Exec(`UPDATE running_tasks SET status='stopping' WHERE id=? AND status IN ('preparing','running')`, id)
	return e
}
func (d *DB) RecoverInterruptedTasks() error {
	d.mu.Lock()
	defer d.mu.Unlock()
	_, e := d.db.Exec(`UPDATE running_tasks SET status='failed',error_msg='服务重启，执行已中断',exit_code=-1,ended_at=? WHERE status IN ('preparing','running','stopping')`, time.Now().UnixMilli())
	return e
}

func (d *DB) SetDispatchCompiledHash(id, hash string) error {
	d.mu.Lock()
	defer d.mu.Unlock()
	_, e := d.db.Exec(`UPDATE dispatch_details SET snapshot=json_set(snapshot,'$.compiled_hash',?) WHERE dispatch_id=? AND state='dispatching'`, hash, id)
	return e
}

func (d *DB) RecoverDispatches() error {
	d.mu.Lock()
	_, e := d.db.Exec(`UPDATE dispatch_details SET state='finished',ended_at=?,error_msg='服务重启，批次分发已中断' WHERE state='dispatching'`, time.Now().UnixMilli())
	d.mu.Unlock()
	if e != nil {
		return e
	}
	scripts, e := d.ListMarkdownScripts()
	if e != nil {
		return e
	}
	for _, script := range scripts {
		if e = d.ReconcileScriptRunStatus(script.ID); e != nil {
			return e
		}
	}
	return nil
}
