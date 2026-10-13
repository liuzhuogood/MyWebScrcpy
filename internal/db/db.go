package db

import (
	"database/sql"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"time"

	_ "modernc.org/sqlite"
)

// DeviceRecord 对应 SQLite 中的设备持久化记录
type DeviceRecord struct {
	ID              int64  `json:"id"`
	Serial          string `json:"serial"`
	Source          string `json:"source"` // "manual" 或 "auto"
	Alias           string `json:"alias"`
	State           string `json:"state"` // "device", "offline", "connecting", "unauthorized"
	Product         string `json:"product"`
	Model           string `json:"model"`
	DeviceName      string `json:"device_name"`
	TransportID     string `json:"transport_id"`
	AutoReconnect   int    `json:"auto_reconnect"` // 1 启用, 0 禁用
	LastConnectedAt int64  `json:"last_connected_at"`
	CreatedAt       int64  `json:"created_at"`
	UpdatedAt       int64  `json:"updated_at"`
}

// DB 封装 SQLite 数据库操作
type DB struct {
	db   *sql.DB
	path string
	mu   sync.RWMutex
}

// Open 打开或创建 SQLite 数据库，并自动执行建表迁移
func Open(dbPath string) (*DB, error) {
	if err := os.MkdirAll(filepath.Dir(dbPath), 0755); err != nil {
		return nil, fmt.Errorf("创建数据库目录失败: %w", err)
	}

	dsn := fmt.Sprintf("%s?_pragma=busy_timeout(5000)&_pragma=journal_mode(WAL)", dbPath)
	db, err := sql.Open("sqlite", dsn)
	if err != nil {
		return nil, fmt.Errorf("连接 SQLite 失败: %w", err)
	}

	d := &DB{db: db, path: dbPath}
	if err := d.migrate(); err != nil {
		_ = db.Close()
		return nil, fmt.Errorf("执行迁移失败: %w", err)
	}

	return d, nil
}

// Close 关闭数据库
func (d *DB) Close() error {
	return d.db.Close()
}

func (d *DB) migrate() error {
	schema := `
	CREATE TABLE IF NOT EXISTS devices (
		id                  INTEGER PRIMARY KEY AUTOINCREMENT,
		serial              TEXT NOT NULL UNIQUE,
		source              TEXT NOT NULL DEFAULT 'manual',
		alias               TEXT NOT NULL DEFAULT '',
		state               TEXT NOT NULL DEFAULT 'offline',
		product             TEXT NOT NULL DEFAULT '',
		model               TEXT NOT NULL DEFAULT '',
		device_name         TEXT NOT NULL DEFAULT '',
		transport_id        TEXT NOT NULL DEFAULT '',
		auto_reconnect      INTEGER NOT NULL DEFAULT 1,
		last_connected_at   INTEGER NOT NULL DEFAULT 0,
		created_at          INTEGER NOT NULL,
		updated_at          INTEGER NOT NULL
	);
	CREATE INDEX IF NOT EXISTS idx_devices_source ON devices(source);
	CREATE INDEX IF NOT EXISTS idx_devices_state ON devices(state);

	CREATE TABLE IF NOT EXISTS markdown_scripts (
		id                  INTEGER PRIMARY KEY AUTOINCREMENT,
		name                TEXT NOT NULL,
		target_device       TEXT NOT NULL DEFAULT 'auto',
		content             TEXT NOT NULL DEFAULT '',
		extracted_version   TEXT NOT NULL DEFAULT '',
		schedule_cron       TEXT NOT NULL DEFAULT '',
		is_enabled          INTEGER NOT NULL DEFAULT 1,
		timeout_sec         INTEGER NOT NULL DEFAULT 120,
		compiled_version    TEXT NOT NULL DEFAULT '',
		compiled_python     TEXT NOT NULL DEFAULT '',
		compiled_at         INTEGER NOT NULL DEFAULT 0,
		last_run_at         INTEGER NOT NULL DEFAULT 0,
		last_run_status     TEXT NOT NULL DEFAULT '',
		created_at          INTEGER NOT NULL,
		updated_at          INTEGER NOT NULL
	);
	CREATE INDEX IF NOT EXISTS idx_md_scripts_schedule ON markdown_scripts(is_enabled, schedule_cron);

	CREATE TABLE IF NOT EXISTS running_tasks (
		id              TEXT PRIMARY KEY,
		script_id       INTEGER NOT NULL,
		script_name     TEXT NOT NULL,
		device_serial   TEXT NOT NULL,
		pid             INTEGER NOT NULL,
		status          TEXT NOT NULL DEFAULT 'running',
		exit_code       INTEGER DEFAULT 0,
		error_msg       TEXT NOT NULL DEFAULT '',
		started_at      INTEGER NOT NULL,
		ended_at        INTEGER DEFAULT 0
	);
	CREATE INDEX IF NOT EXISTS idx_running_tasks_lock ON running_tasks(device_serial, status);

 CREATE TABLE IF NOT EXISTS script_dispatches (
  id TEXT PRIMARY KEY, script_id INTEGER NOT NULL, started_ns INTEGER NOT NULL
 );
 CREATE INDEX IF NOT EXISTS idx_script_dispatches ON script_dispatches(script_id, started_ns);
 CREATE TABLE IF NOT EXISTS script_dispatch_tasks (
  dispatch_id TEXT NOT NULL, task_id TEXT NOT NULL UNIQUE,
  PRIMARY KEY(dispatch_id, task_id)
 );

	CREATE TABLE IF NOT EXISTS system_configs (
		key         TEXT PRIMARY KEY,
		value       TEXT NOT NULL DEFAULT '',
		updated_at  INTEGER NOT NULL
	);
	`
	if _, err := d.db.Exec(schema); err != nil {
		return err
	}

	// 渐进式列迁移：支持已有旧数据库无缝升级
	_ = d.addColumnIfNotExists("markdown_scripts", "last_run_at", "INTEGER NOT NULL DEFAULT 0")
	_ = d.addColumnIfNotExists("markdown_scripts", "last_run_status", "TEXT NOT NULL DEFAULT ''")
	if err := d.migrateScriptTargets(); err != nil {
		return err
	}
	if err := d.migrateGenerations(); err != nil {
		return err
	}
	return d.migrateArtifacts()
}

func (d *DB) addColumnIfNotExists(table, column, colType string) error {
	query := fmt.Sprintf("ALTER TABLE %s ADD COLUMN %s %s", table, column, colType)
	_, err := d.db.Exec(query)
	return err
}

// ListManualDevices 获取所有手动添加的设备记录
func (d *DB) ListManualDevices() ([]DeviceRecord, error) {
	d.mu.RLock()
	defer d.mu.RUnlock()

	query := `SELECT id, serial, source, alias, state, product, model, device_name, transport_id, auto_reconnect, last_connected_at, created_at, updated_at
	          FROM devices WHERE source = 'manual' ORDER BY id DESC`
	rows, err := d.db.Query(query)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var list []DeviceRecord
	for rows.Next() {
		var item DeviceRecord
		err := rows.Scan(
			&item.ID, &item.Serial, &item.Source, &item.Alias, &item.State,
			&item.Product, &item.Model, &item.DeviceName, &item.TransportID,
			&item.AutoReconnect, &item.LastConnectedAt, &item.CreatedAt, &item.UpdatedAt,
		)
		if err != nil {
			return nil, err
		}
		list = append(list, item)
	}
	return list, rows.Err()
}

// UpsertManualDevice 插入或更新手动设备
func (d *DB) UpsertManualDevice(serial, alias string, autoReconnect int) (*DeviceRecord, error) {
	d.mu.Lock()
	defer d.mu.Unlock()

	now := time.Now().UnixMilli()
	query := `
	INSERT INTO devices (serial, source, alias, state, auto_reconnect, created_at, updated_at)
	VALUES (?, 'manual', ?, 'connecting', ?, ?, ?)
	ON CONFLICT(serial) DO UPDATE SET
		source = 'manual',
		alias = CASE WHEN excluded.alias != '' THEN excluded.alias ELSE devices.alias END,
		auto_reconnect = excluded.auto_reconnect,
		updated_at = excluded.updated_at
	RETURNING id, serial, source, alias, state, product, model, device_name, transport_id, auto_reconnect, last_connected_at, created_at, updated_at;
	`
	var item DeviceRecord
	err := d.db.QueryRow(query, serial, alias, autoReconnect, now, now).Scan(
		&item.ID, &item.Serial, &item.Source, &item.Alias, &item.State,
		&item.Product, &item.Model, &item.DeviceName, &item.TransportID,
		&item.AutoReconnect, &item.LastConnectedAt, &item.CreatedAt, &item.UpdatedAt,
	)
	if err != nil {
		return nil, err
	}
	return &item, nil
}

// UpdateDeviceStatus 当 ADB 状态变化时更新设备元数据与状态
func (d *DB) UpdateDeviceStatus(serial, state, product, model, deviceName, transportID string, isOnline bool) error {
	d.mu.Lock()
	defer d.mu.Unlock()

	now := time.Now().UnixMilli()
	var query string
	if isOnline {
		query = `UPDATE devices SET
			state = ?, product = ?, model = ?, device_name = ?, transport_id = ?,
			last_connected_at = ?, updated_at = ?
			WHERE serial = ?`
		_, err := d.db.Exec(query, state, product, model, deviceName, transportID, now, now, serial)
		return err
	}

	query = `UPDATE devices SET state = ?, updated_at = ? WHERE serial = ?`
	_, err := d.db.Exec(query, state, now, serial)
	return err
}

// UpdateAlias 更新别名
func (d *DB) UpdateAlias(serial, alias string) error {
	d.mu.Lock()
	defer d.mu.Unlock()

	now := time.Now().UnixMilli()
	_, err := d.db.Exec("UPDATE devices SET alias = ?, updated_at = ? WHERE serial = ?", alias, now, serial)
	return err
}

// DeleteManualDevice 删除手动设备记录
func (d *DB) DeleteManualDevice(serial string) error {
	d.mu.Lock()
	defer d.mu.Unlock()

	res, err := d.db.Exec("DELETE FROM devices WHERE serial = ? AND source = 'manual'", serial)
	if err != nil {
		return err
	}
	n, err := res.RowsAffected()
	if err != nil {
		return err
	}
	if n == 0 {
		return fmt.Errorf("设备不存在或非手动添加设备")
	}
	return nil
}
