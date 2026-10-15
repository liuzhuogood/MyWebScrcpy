package db

import (
	"crypto/rand"
	"database/sql"
	"encoding/hex"
	"errors"
	"fmt"
	"strings"
	"time"
)

var (
	ErrNotFound = errors.New("记录不存在")
	ErrConflict = errors.New("数据冲突")
)

// TaskRunRecord 对应独立的外部任务运行记录
type TaskRunRecord struct {
	ID              string `json:"id"`
	ClientRunID     string `json:"client_run_id"`
	Name            string `json:"name"`
	Source          string `json:"source"`
	DeviceSerial    string `json:"device_serial"`
	Status          string `json:"status"` // running, succeeded, failed, cancelled
	ExitCode        *int   `json:"exit_code"`
	Summary         string `json:"summary"`
	ReportMarkdown  string `json:"report_markdown"`
	LogCount        int64  `json:"log_count"`
	LogTruncated    bool   `json:"log_truncated"`
	CreatedAt       int64  `json:"created_at"`
	UpdatedAt       int64  `json:"updated_at"`
	LastHeartbeatAt int64  `json:"last_heartbeat_at"`
	EndedAt         int64  `json:"ended_at"`
}

// TaskRunLogEntry 对应外部任务单条日志记录
type TaskRunLogEntry struct {
	Seq       int64  `json:"seq"`
	Timestamp int64  `json:"timestamp"`
	Stream    string `json:"stream"` // stdout, stderr
	Message   string `json:"message"`
}

// TaskRunFilter 列表查询过滤参数
type TaskRunFilter struct {
	Connectivity    string
	HeartbeatCutoff int64
	Page            int
	Limit           int
	Status          string
	DeviceSerial    string
	Source          string
	Search          string
}

const taskRunsSchema = `
	CREATE TABLE IF NOT EXISTS task_runs (
		id                  TEXT PRIMARY KEY,
		client_run_id       TEXT NOT NULL UNIQUE,
		name                TEXT NOT NULL,
		source              TEXT NOT NULL DEFAULT 'external',
		device_serial       TEXT NOT NULL,
		status              TEXT NOT NULL DEFAULT 'running',
		exit_code           INTEGER DEFAULT NULL,
		summary             TEXT NOT NULL DEFAULT '',
		report_markdown     TEXT NOT NULL DEFAULT '',
		log_count           INTEGER NOT NULL DEFAULT 0,
		log_truncated       INTEGER NOT NULL DEFAULT 0,
		created_at          INTEGER NOT NULL,
		updated_at          INTEGER NOT NULL,
		last_heartbeat_at   INTEGER NOT NULL,
		ended_at            INTEGER NOT NULL DEFAULT 0
	);
	CREATE INDEX IF NOT EXISTS idx_task_runs_device ON task_runs(device_serial, created_at);
	CREATE INDEX IF NOT EXISTS idx_task_runs_status ON task_runs(status, created_at);
	CREATE INDEX IF NOT EXISTS idx_task_runs_created ON task_runs(created_at DESC);

	CREATE TABLE IF NOT EXISTS task_run_logs (
		run_id      TEXT NOT NULL,
		seq         INTEGER NOT NULL,
		timestamp   INTEGER NOT NULL,
		stream      TEXT NOT NULL DEFAULT 'stdout',
		message     TEXT NOT NULL,
		PRIMARY KEY(run_id, seq)
	);
	CREATE INDEX IF NOT EXISTS idx_task_run_logs_seq ON task_run_logs(run_id, seq);
	`

func generateRunID(now int64) string {
	b := make([]byte, 4)
	_, _ = rand.Read(b)
	return fmt.Sprintf("run_%d_%s", now, hex.EncodeToString(b))
}

// CreateTaskRun 创建外部运行记录，基于 client_run_id 进行幂等与冲突检测
func (d *DB) CreateTaskRun(r *TaskRunRecord) (*TaskRunRecord, bool, error) {
	d.mu.Lock()
	defer d.mu.Unlock()

	if r.ClientRunID == "" {
		return nil, false, fmt.Errorf("client_run_id 不能为空")
	}
	if r.Name == "" {
		return nil, false, fmt.Errorf("name 不能为空")
	}
	if r.DeviceSerial == "" {
		return nil, false, fmt.Errorf("device_serial 不能为空")
	}

	tx, err := d.db.Begin()
	if err != nil {
		return nil, false, err
	}
	defer tx.Rollback()

	var existing TaskRunRecord
	var exitCode sql.NullInt64
	var logTruncated int
	query := `SELECT id, client_run_id, name, source, device_serial, status, exit_code, summary, report_markdown, log_count, log_truncated, created_at, updated_at, last_heartbeat_at, ended_at
	          FROM task_runs WHERE client_run_id = ?`
	err = tx.QueryRow(query, r.ClientRunID).Scan(
		&existing.ID, &existing.ClientRunID, &existing.Name, &existing.Source,
		&existing.DeviceSerial, &existing.Status, &exitCode, &existing.Summary,
		&existing.ReportMarkdown, &existing.LogCount, &logTruncated,
		&existing.CreatedAt, &existing.UpdatedAt, &existing.LastHeartbeatAt, &existing.EndedAt,
	)

	if err == nil {
		// 记录已存在，检查关键信息是否一致
		if exitCode.Valid {
			code := int(exitCode.Int64)
			existing.ExitCode = &code
		}
		existing.LogTruncated = (logTruncated != 0)

		reqSource := r.Source
		if reqSource == "" {
			reqSource = "external"
		}
		if existing.Name == r.Name && existing.DeviceSerial == r.DeviceSerial && existing.Source == reqSource {
			return &existing, false, tx.Commit()
		}
		return nil, false, fmt.Errorf("%w: 同一 client_run_id 对应不同内容 (已有: %s/%s/%s, 传入: %s/%s/%s)",
			ErrConflict, existing.Name, existing.DeviceSerial, existing.Source, r.Name, r.DeviceSerial, reqSource)
	} else if err != sql.ErrNoRows {
		return nil, false, err
	}

	now := time.Now().UnixMilli()
	if r.CreatedAt <= 0 {
		r.CreatedAt = now
	}
	r.UpdatedAt = now
	r.LastHeartbeatAt = now
	r.EndedAt = 0
	r.Status = "running"
	r.LogCount = 0
	r.LogTruncated = false
	if r.Source == "" {
		r.Source = "external"
	}
	r.ID = generateRunID(now)

	insertQuery := `INSERT INTO task_runs(
		id, client_run_id, name, source, device_serial, status, summary, report_markdown,
		log_count, log_truncated, created_at, updated_at, last_heartbeat_at, ended_at
	) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`
	_, err = tx.Exec(insertQuery,
		r.ID, r.ClientRunID, r.Name, r.Source, r.DeviceSerial, r.Status, r.Summary, r.ReportMarkdown,
		0, 0, r.CreatedAt, r.UpdatedAt, r.LastHeartbeatAt, 0,
	)
	if err != nil {
		return nil, false, err
	}

	if err := tx.Commit(); err != nil {
		return nil, false, err
	}
	return r, true, nil
}

// HeartbeatTaskRun 保持运行心跳
func (d *DB) HeartbeatTaskRun(id string, now int64) (*TaskRunRecord, error) {
	d.mu.Lock()
	defer d.mu.Unlock()

	rec, err := d.getTaskRunLocked(id)
	if err != nil {
		return nil, err
	}

	if rec.Status != "running" {
		// 终态不可覆盖，直接返回当前终态记录
		return rec, nil
	}

	_, err = d.db.Exec(`UPDATE task_runs SET last_heartbeat_at = ?, updated_at = ? WHERE id = ?`, now, now, id)
	if err != nil {
		return nil, err
	}
	rec.LastHeartbeatAt = now
	rec.UpdatedAt = now
	return rec, nil
}

// AppendTaskRunLogs 批量追加日志，按连续 seq 校验与去重，支持容量截断与终态保护
func (d *DB) AppendTaskRunLogs(id string, entries []TaskRunLogEntry, maxLogs int64) (int64, bool, error) {
	d.mu.Lock()
	defer d.mu.Unlock()

	tx, err := d.db.Begin()
	if err != nil {
		return 0, false, err
	}
	defer tx.Rollback()

	var status string
	var currentCount int64
	var truncatedInt int
	err = tx.QueryRow(`SELECT status, log_count, log_truncated FROM task_runs WHERE id = ?`, id).Scan(&status, &currentCount, &truncatedInt)
	if err == sql.ErrNoRows {
		return 0, false, ErrNotFound
	} else if err != nil {
		return 0, false, err
	}

	// 终态检查：若任务已结束，禁止写入新日志
	if status != "running" {
		return 0, false, fmt.Errorf("%w: 任务已结束，不可追加日志", ErrConflict)
	}

	isTruncated := (truncatedInt != 0)

	// 计算当前数据库内已连续确认的最大 seq（从 1 开始连续）
	rows, err := tx.Query(`SELECT seq FROM task_run_logs WHERE run_id = ? ORDER BY seq ASC`, id)
	if err != nil {
		return 0, false, err
	}
	existingSeqs := make(map[int64]bool)
	var maxContinuousSeq int64 = 0
	for rows.Next() {
		var s int64
		if err := rows.Scan(&s); err == nil {
			existingSeqs[s] = true
			if s == maxContinuousSeq+1 {
				maxContinuousSeq = s
			}
		}
	}
	rows.Close()

	stmtCheck, err := tx.Prepare(`SELECT stream, message FROM task_run_logs WHERE run_id = ? AND seq = ?`)
	if err != nil {
		return 0, false, err
	}
	defer stmtCheck.Close()

	stmtInsert, err := tx.Prepare(`INSERT INTO task_run_logs(run_id, seq, timestamp, stream, message) VALUES(?, ?, ?, ?, ?)`)
	if err != nil {
		return 0, false, err
	}
	defer stmtInsert.Close()

	for _, entry := range entries {
		if entry.Seq <= 0 {
			return 0, false, fmt.Errorf("日志序号 seq 必须大于 0")
		}

		if existingSeqs[entry.Seq] {
			// 序号已存在，检查内容是否一致
			var existingStream, existingMessage string
			err := stmtCheck.QueryRow(id, entry.Seq).Scan(&existingStream, &existingMessage)
			if err == nil {
				if existingStream == entry.Stream && existingMessage == entry.Message {
					continue // 幂等去重
				}
				return 0, false, fmt.Errorf("%w: seq=%d 内容冲突", ErrConflict, entry.Seq)
			} else if err != sql.ErrNoRows {
				return 0, false, err
			}
		}

		// 检查连续性：若尚未确认过且 seq > maxContinuousSeq + 1，拒绝缺口
		if entry.Seq > maxContinuousSeq+1 {
			return maxContinuousSeq, false, fmt.Errorf("日志序号存在缺口: 期望 seq=%d, 实际 seq=%d", maxContinuousSeq+1, entry.Seq)
		}

		// 检查容量上限
		if isTruncated || (maxLogs > 0 && currentCount >= maxLogs) {
			isTruncated = true
			continue // 截断丢弃超出容量的日志
		}

		stream := entry.Stream
		if stream == "" {
			stream = "stdout"
		}
		ts := entry.Timestamp
		if ts <= 0 {
			ts = time.Now().UnixMilli()
		}

		_, err = stmtInsert.Exec(id, entry.Seq, ts, stream, entry.Message)
		if err != nil {
			return 0, false, err
		}
		existingSeqs[entry.Seq] = true
		currentCount++
		if entry.Seq == maxContinuousSeq+1 {
			maxContinuousSeq = entry.Seq
		}
	}

	now := time.Now().UnixMilli()
	truncVal := 0
	if isTruncated {
		truncVal = 1
	}
	_, err = tx.Exec(`UPDATE task_runs SET log_count = ?, log_truncated = ?, updated_at = ? WHERE id = ?`,
		currentCount, truncVal, now, id)
	if err != nil {
		return 0, false, err
	}

	if err := tx.Commit(); err != nil {
		return 0, false, err
	}
	return maxContinuousSeq, isTruncated, nil
}

// FinishTaskRun 提交最终状态和结果
func (d *DB) FinishTaskRun(id string, status string, exitCode *int, summary, reportMarkdown string, now int64) (*TaskRunRecord, error) {
	d.mu.Lock()
	defer d.mu.Unlock()

	if status != "succeeded" && status != "failed" && status != "cancelled" {
		return nil, fmt.Errorf("无效终态: %s", status)
	}

	rec, err := d.getTaskRunLocked(id)
	if err != nil {
		return nil, err
	}

	// 终态检查与冲突判定：终态已定，若再次上报，必须所有结果字段完全一致才为幂等，否则报错 ErrConflict
	if rec.Status != "running" {
		if rec.Status == status {
			sameExit := (rec.ExitCode == nil && exitCode == nil) ||
				(rec.ExitCode != nil && exitCode != nil && *rec.ExitCode == *exitCode)
			if sameExit && rec.Summary == summary && rec.ReportMarkdown == reportMarkdown {
				return rec, nil // 幂等返回
			}
		}
		return nil, fmt.Errorf("%w: 终态结果冲突", ErrConflict)
	}

	var exitVal sql.NullInt64
	if exitCode != nil {
		exitVal = sql.NullInt64{Int64: int64(*exitCode), Valid: true}
	}

	_, err = d.db.Exec(`UPDATE task_runs SET status = ?, exit_code = ?, summary = ?, report_markdown = ?, ended_at = ?, updated_at = ? WHERE id = ?`,
		status, exitVal, summary, reportMarkdown, now, now, id)
	if err != nil {
		return nil, err
	}

	rec.Status = status
	rec.ExitCode = exitCode
	rec.Summary = summary
	rec.ReportMarkdown = reportMarkdown
	rec.EndedAt = now
	rec.UpdatedAt = now
	return rec, nil
}

func (d *DB) getTaskRunLocked(id string) (*TaskRunRecord, error) {
	var rec TaskRunRecord
	var exitCode sql.NullInt64
	var logTruncated int
	query := `SELECT id, client_run_id, name, source, device_serial, status, exit_code, summary, report_markdown, log_count, log_truncated, created_at, updated_at, last_heartbeat_at, ended_at
	          FROM task_runs WHERE id = ?`
	err := d.db.QueryRow(query, id).Scan(
		&rec.ID, &rec.ClientRunID, &rec.Name, &rec.Source,
		&rec.DeviceSerial, &rec.Status, &exitCode, &rec.Summary,
		&rec.ReportMarkdown, &rec.LogCount, &logTruncated,
		&rec.CreatedAt, &rec.UpdatedAt, &rec.LastHeartbeatAt, &rec.EndedAt,
	)
	if err == sql.ErrNoRows {
		return nil, ErrNotFound
	} else if err != nil {
		return nil, err
	}

	if exitCode.Valid {
		code := int(exitCode.Int64)
		rec.ExitCode = &code
	}
	rec.LogTruncated = (logTruncated != 0)
	return &rec, nil
}

// GetTaskRunByID 查询单个运行详情
func (d *DB) GetTaskRunByID(id string) (*TaskRunRecord, error) {
	d.mu.RLock()
	defer d.mu.RUnlock()
	return d.getTaskRunLocked(id)
}

// ListTaskRuns 分页查询外部任务列表
func (d *DB) ListTaskRuns(filter TaskRunFilter) ([]TaskRunRecord, int, error) {
	d.mu.RLock()
	defer d.mu.RUnlock()

	var conditions []string
	var args []any

	if filter.Status != "" {
		conditions = append(conditions, "status = ?")
		args = append(args, filter.Status)
	}
	if filter.Connectivity != "" {
		conditions = append(conditions, "status = 'running'")
		if filter.Connectivity == "disconnected" {
			conditions = append(conditions, "last_heartbeat_at < ?")
		} else {
			conditions = append(conditions, "last_heartbeat_at >= ?")
		}
		args = append(args, filter.HeartbeatCutoff)
	}
	if filter.DeviceSerial != "" {
		conditions = append(conditions, "device_serial = ?")
		args = append(args, filter.DeviceSerial)
	}
	if filter.Source != "" {
		conditions = append(conditions, "source = ?")
		args = append(args, filter.Source)
	}
	if filter.Search != "" {
		conditions = append(conditions, "(name LIKE ? OR summary LIKE ?)")
		pattern := "%" + filter.Search + "%"
		args = append(args, pattern, pattern)
	}

	whereClause := ""
	if len(conditions) > 0 {
		whereClause = " WHERE " + strings.Join(conditions, " AND ")
	}

	var total int
	countQuery := "SELECT COUNT(*) FROM task_runs" + whereClause
	if err := d.db.QueryRow(countQuery, args...).Scan(&total); err != nil {
		return nil, 0, err
	}

	page := filter.Page
	if page < 1 {
		page = 1
	}
	limit := filter.Limit
	if limit < 1 || limit > 100 {
		limit = 20
	}
	offset := (page - 1) * limit

	query := `SELECT id, client_run_id, name, source, device_serial, status, exit_code, summary, report_markdown, log_count, log_truncated, created_at, updated_at, last_heartbeat_at, ended_at
	          FROM task_runs` + whereClause + " ORDER BY created_at DESC LIMIT ? OFFSET ?"
	queryArgs := append(args, limit, offset)

	rows, err := d.db.Query(query, queryArgs...)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()

	var items []TaskRunRecord
	for rows.Next() {
		var rec TaskRunRecord
		var exitCode sql.NullInt64
		var logTruncated int
		err := rows.Scan(
			&rec.ID, &rec.ClientRunID, &rec.Name, &rec.Source,
			&rec.DeviceSerial, &rec.Status, &exitCode, &rec.Summary,
			&rec.ReportMarkdown, &rec.LogCount, &logTruncated,
			&rec.CreatedAt, &rec.UpdatedAt, &rec.LastHeartbeatAt, &rec.EndedAt,
		)
		if err != nil {
			return nil, 0, err
		}
		if exitCode.Valid {
			code := int(exitCode.Int64)
			rec.ExitCode = &code
		}
		rec.LogTruncated = (logTruncated != 0)
		items = append(items, rec)
	}
	if err := rows.Err(); err != nil {
		return nil, 0, err
	}

	if items == nil {
		items = []TaskRunRecord{}
	}
	return items, total, nil
}

// GetTaskRunLogs 增量读取指定运行的日志
func (d *DB) GetTaskRunLogs(id string, after int64, limit int, stream string) ([]TaskRunLogEntry, int64, bool, error) {
	d.mu.RLock()
	defer d.mu.RUnlock()

	var logTruncated int
	err := d.db.QueryRow(`SELECT log_truncated FROM task_runs WHERE id = ?`, id).Scan(&logTruncated)
	if err == sql.ErrNoRows {
		return nil, 0, false, ErrNotFound
	} else if err != nil {
		return nil, 0, false, err
	}

	if limit <= 0 || limit > 1000 {
		limit = 500
	}

	conditions := []string{"run_id = ?", "seq > ?"}
	args := []any{id, after}
	if stream != "" {
		conditions = append(conditions, "stream = ?")
		args = append(args, stream)
	}

	query := `SELECT seq, timestamp, stream, message FROM task_run_logs WHERE ` +
		strings.Join(conditions, " AND ") + ` ORDER BY seq ASC LIMIT ?`
	args = append(args, limit)

	rows, err := d.db.Query(query, args...)
	if err != nil {
		return nil, 0, false, err
	}
	defer rows.Close()

	var entries []TaskRunLogEntry
	var lastSeq int64 = after
	for rows.Next() {
		var entry TaskRunLogEntry
		if err := rows.Scan(&entry.Seq, &entry.Timestamp, &entry.Stream, &entry.Message); err != nil {
			return nil, 0, false, err
		}
		entries = append(entries, entry)
		if entry.Seq > lastSeq {
			lastSeq = entry.Seq
		}
	}
	if err := rows.Err(); err != nil {
		return nil, 0, false, err
	}

	if entries == nil {
		entries = []TaskRunLogEntry{}
	}
	return entries, lastSeq, (logTruncated != 0), nil
}

// GetAllTaskRunLogs 获取全部日志（用于下载）
func (d *DB) GetAllTaskRunLogs(id string) ([]TaskRunLogEntry, error) {
	d.mu.RLock()
	defer d.mu.RUnlock()

	var exists int
	err := d.db.QueryRow(`SELECT 1 FROM task_runs WHERE id = ?`, id).Scan(&exists)
	if err == sql.ErrNoRows {
		return nil, ErrNotFound
	} else if err != nil {
		return nil, err
	}

	rows, err := d.db.Query(`SELECT seq, timestamp, stream, message FROM task_run_logs WHERE run_id = ? ORDER BY seq ASC`, id)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var entries []TaskRunLogEntry
	for rows.Next() {
		var entry TaskRunLogEntry
		if err := rows.Scan(&entry.Seq, &entry.Timestamp, &entry.Stream, &entry.Message); err != nil {
			return nil, err
		}
		entries = append(entries, entry)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return entries, nil
}
