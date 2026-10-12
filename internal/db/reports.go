package db

import (
	"database/sql"
	"fmt"
	"time"
)

type ReportRecord struct {
	TaskID    string `json:"task_id"`
	Revision  int    `json:"revision"`
	State     string `json:"state"`
	Evidence  string `json:"-"`
	Prompt    string `json:"-"`
	Output    string `json:"-"`
	InputHash string `json:"input_hash"`
	ErrorMsg  string `json:"error_msg"`
	CreatedAt int64  `json:"created_at"`
}

var ErrReportBusy = fmt.Errorf("报告正在生成")

func (d *DB) EnqueueReport(id, evidence, prompt, hash string) (int, error) {
	d.mu.Lock()
	defer d.mu.Unlock()
	tx, e := d.db.Begin()
	if e != nil {
		return 0, e
	}
	defer tx.Rollback()
	var rev, active int
	if e = tx.QueryRow(`SELECT COALESCE(MAX(revision),0),COALESCE(SUM(state IN ('pending','running')),0) FROM task_reports WHERE task_id=?`, id).Scan(&rev, &active); e != nil {
		return 0, e
	}
	if active > 0 {
		return 0, ErrReportBusy
	}
	rev++
	if _, e = tx.Exec(`INSERT INTO task_reports(task_id,revision,state,evidence,prompt,input_hash,created_at) VALUES(?,?,'pending',?,?,?,?)`, id, rev, evidence, prompt, hash, time.Now().UnixMilli()); e != nil {
		return 0, e
	}
	return rev, tx.Commit()
}
func (d *DB) ClaimReport() (*ReportRecord, error) {
	d.mu.Lock()
	defer d.mu.Unlock()
	r := &ReportRecord{}
	e := d.db.QueryRow(`UPDATE task_reports SET state='running' WHERE (task_id,revision)=(SELECT task_id,revision FROM task_reports WHERE state='pending' ORDER BY created_at LIMIT 1) RETURNING task_id,revision,state,evidence,prompt,output,input_hash,error_msg,created_at`).Scan(&r.TaskID, &r.Revision, &r.State, &r.Evidence, &r.Prompt, &r.Output, &r.InputHash, &r.ErrorMsg, &r.CreatedAt)
	if e == sql.ErrNoRows {
		return nil, nil
	}
	return r, e
}
func (d *DB) FinishReport(id string, revision int, output, message string) error {
	d.mu.Lock()
	defer d.mu.Unlock()
	state := "completed"
	if message != "" {
		state = "failed"
	}
	_, e := d.db.Exec(`UPDATE task_reports SET state=?,output=?,error_msg=? WHERE task_id=? AND revision=? AND state='running'`, state, output, message, id, revision)
	return e
}
func (d *DB) Report(id string, revision int, successful bool) (*ReportRecord, error) {
	d.mu.RLock()
	defer d.mu.RUnlock()
	query := `SELECT task_id,revision,state,evidence,prompt,output,input_hash,error_msg,created_at FROM task_reports WHERE task_id=?`
	args := []any{id}
	if revision > 0 {
		query += ` AND revision=?`
		args = append(args, revision)
	}
	if successful {
		query += ` AND state='completed'`
	}
	query += ` ORDER BY revision DESC LIMIT 1`
	r := &ReportRecord{}
	e := d.db.QueryRow(query, args...).Scan(&r.TaskID, &r.Revision, &r.State, &r.Evidence, &r.Prompt, &r.Output, &r.InputHash, &r.ErrorMsg, &r.CreatedAt)
	if e == sql.ErrNoRows {
		return nil, nil
	}
	return r, e
}
func (d *DB) RecoverReports() error {
	d.mu.Lock()
	defer d.mu.Unlock()
	_, e := d.db.Exec(`UPDATE task_reports SET state='failed',error_msg='服务重启，报告生成已中断' WHERE state='running'`)
	return e
}
func (d *DB) ArtifactTaskIDs() ([]string, error) {
	d.mu.RLock()
	defer d.mu.RUnlock()
	rows, e := d.db.Query(`SELECT task_id FROM task_artifacts`)
	if e != nil {
		return nil, e
	}
	defer rows.Close()
	ids := []string{}
	for rows.Next() {
		var id string
		if e = rows.Scan(&id); e != nil {
			return nil, e
		}
		ids = append(ids, id)
	}
	return ids, rows.Err()
}
