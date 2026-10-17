package db

import (
	"database/sql"
	"path/filepath"
	"testing"
	"time"
)

func TestResetLegacyDatabaseOnce(t *testing.T) {
	path := filepath.Join(t.TempDir(), "app.db")
	old, err := sql.Open("sqlite", path)
	if err != nil {
		t.Fatal(err)
	}
	_, err = old.Exec(`CREATE TABLE markdown_scripts(id INTEGER PRIMARY KEY, content TEXT);
 CREATE TABLE system_configs(key TEXT PRIMARY KEY,value TEXT,updated_at INTEGER);
 INSERT INTO system_configs VALUES('ai_model_config','fixture-secret',1);
 CREATE TABLE devices(serial TEXT); INSERT INTO devices VALUES('old-device');`)
	if err != nil {
		t.Fatal(err)
	}
	old.Close()
	current, err := Open(path)
	if err != nil {
		t.Fatal(err)
	}
	var count int
	if err := current.db.QueryRow(`SELECT count(*) FROM sqlite_schema WHERE type='table' AND name IN ('markdown_scripts','running_tasks','script_dispatches','task_reports')`).Scan(&count); err != nil || count != 0 {
		t.Fatalf("legacy tables remain: %d %v", count, err)
	}
	if value, err := current.GetConfig("ai_model_config"); err != nil || value != "" {
		t.Fatalf("legacy model config remains: %v", err)
	}
	if err := current.SetConfig("MACOCR_URL", "http://127.0.0.1:18000/upload"); err != nil {
		t.Fatal(err)
	}
	devices, err := current.ListManualDevices()
	if err != nil || len(devices) != 0 {
		t.Fatalf("old devices remain: %v", err)
	}
	if _, err := current.UpsertManualDevice("new-device", "new", 1); err != nil {
		t.Fatal(err)
	}
	current.Close()
	current, err = Open(path)
	if err != nil {
		t.Fatal(err)
	}
	defer current.Close()
	if value, err := current.GetConfig("MACOCR_URL"); err != nil || value != "http://127.0.0.1:18000/upload" {
		t.Fatalf("restart lost OCR config: %v", err)
	}
	devices, err = current.ListManualDevices()
	if err != nil || len(devices) != 1 {
		t.Fatalf("restart lost devices: %v", err)
	}
	var version int
	if err := current.db.QueryRow("PRAGMA user_version").Scan(&version); err != nil || version != SchemaVersion {
		t.Fatalf("wrong schema version: %d %v", version, err)
	}
}

func TestUpgradeFrom12500DropsTaskTablesAndPreservesOtherData(t *testing.T) {
	path := filepath.Join(t.TempDir(), "upgrade_12500.db")
	raw, err := sql.Open("sqlite", path)
	if err != nil {
		t.Fatal(err)
	}

	// 模拟 12500 版本的数据库：含 devices, operation_logs, system_configs, task_runs, task_run_logs
	const schema12500 = `
	CREATE TABLE devices (
		id INTEGER PRIMARY KEY AUTOINCREMENT, serial TEXT NOT NULL UNIQUE,
		source TEXT NOT NULL DEFAULT 'manual', alias TEXT NOT NULL DEFAULT '',
		state TEXT NOT NULL DEFAULT 'offline', product TEXT NOT NULL DEFAULT '',
		model TEXT NOT NULL DEFAULT '', device_name TEXT NOT NULL DEFAULT '',
		transport_id TEXT NOT NULL DEFAULT '', auto_reconnect INTEGER NOT NULL DEFAULT 1,
		last_connected_at INTEGER NOT NULL DEFAULT 0, created_at INTEGER NOT NULL,
		updated_at INTEGER NOT NULL
	);
	CREATE TABLE operation_logs (
		sequence INTEGER PRIMARY KEY AUTOINCREMENT,
		id TEXT NOT NULL UNIQUE, serial TEXT NOT NULL, type TEXT NOT NULL,
		description TEXT NOT NULL, adb_command TEXT NOT NULL, occurred_at INTEGER NOT NULL
	);
	CREATE TABLE system_configs (
		key TEXT PRIMARY KEY, value TEXT NOT NULL DEFAULT '', updated_at INTEGER NOT NULL
	);
	CREATE TABLE task_runs (
		id TEXT PRIMARY KEY, client_run_id TEXT NOT NULL UNIQUE, name TEXT NOT NULL,
		source TEXT NOT NULL, device_serial TEXT NOT NULL, status TEXT NOT NULL,
		exit_code INTEGER, summary TEXT NOT NULL DEFAULT '', report_markdown TEXT NOT NULL DEFAULT '',
		log_count INTEGER NOT NULL DEFAULT 0, log_truncated INTEGER NOT NULL DEFAULT 0,
		created_at INTEGER NOT NULL, updated_at INTEGER NOT NULL, last_heartbeat_at INTEGER NOT NULL,
		ended_at INTEGER NOT NULL DEFAULT 0
	);
	CREATE TABLE task_run_logs (
		run_id TEXT NOT NULL, seq INTEGER NOT NULL, timestamp INTEGER NOT NULL,
		stream TEXT NOT NULL, message TEXT NOT NULL, PRIMARY KEY(run_id, seq)
	);
	PRAGMA user_version = 12500;
	`
	if _, err := raw.Exec(schema12500); err != nil {
		t.Fatalf("初始化 12500 结构失败: %v", err)
	}

	now := time.Now().UnixMilli()
	// 插入真实业务数据
	if _, err := raw.Exec(`INSERT INTO devices (serial, alias, state, created_at, updated_at) VALUES ('device-12500', '保留设备', 'device', ?, ?)`, now, now); err != nil {
		t.Fatal(err)
	}
	if _, err := raw.Exec(`INSERT INTO system_configs (key, value, updated_at) VALUES ('custom_pref', 'keep_me', ?)`, now); err != nil {
		t.Fatal(err)
	}
	if _, err := raw.Exec(`INSERT INTO operation_logs (id, serial, type, description, adb_command, occurred_at) VALUES ('op-1', 'device-12500', 'action', '点击按钮', 'input tap 100 200', ?)`, now); err != nil {
		t.Fatal(err)
	}
	// 插入旧任务记录与日志
	if _, err := raw.Exec(`INSERT INTO task_runs (id, client_run_id, name, source, device_serial, status, created_at, updated_at, last_heartbeat_at) VALUES ('run-1', 'client-1', '旧任务', 'codex', 'device-12500', 'running', ?, ?, ?)`, now, now, now); err != nil {
		t.Fatal(err)
	}
	if _, err := raw.Exec(`INSERT INTO task_run_logs (run_id, seq, timestamp, stream, message) VALUES ('run-1', 1, ?, 'stdout', '旧日志')`, now); err != nil {
		t.Fatal(err)
	}
	raw.Close()

	// 首次打开：执行升级
	db, err := Open(path)
	if err != nil {
		t.Fatalf("升级打开数据库失败: %v", err)
	}

	// 1. 验证版本已升至 SchemaVersion
	var ver int
	if err := db.db.QueryRow("PRAGMA user_version").Scan(&ver); err != nil || ver != SchemaVersion {
		t.Fatalf("版本未升至 %d: got %d, err %v", SchemaVersion, ver, err)
	}

	// 2. 验证 task_runs 与 task_run_logs 表已被彻底删除
	var taskTableCount int
	if err := db.db.QueryRow(`SELECT count(*) FROM sqlite_schema WHERE type='table' AND name IN ('task_runs', 'task_run_logs')`).Scan(&taskTableCount); err != nil || taskTableCount != 0 {
		t.Fatalf("旧任务表未被删除: count=%d, err=%v", taskTableCount, err)
	}

	// 3. 验证设备表、配置表与操作日志表的数据完整保留
	devices, err := db.ListManualDevices()
	if err != nil || len(devices) != 1 || devices[0].Serial != "device-12500" || devices[0].Alias != "保留设备" {
		t.Fatalf("设备数据未保留: devices=%+v, err=%v", devices, err)
	}
	cfgVal, err := db.GetConfig("custom_pref")
	if err != nil || cfgVal != "keep_me" {
		t.Fatalf("配置数据未保留: val=%s, err=%v", cfgVal, err)
	}
	logs, _, err := db.ListOperationLogs(t.Context(), "device-12500", 0, 10)
	if err != nil || len(logs) != 1 || logs[0].Description != "点击按钮" {
		t.Fatalf("操作日志未保留: logs=%+v, err=%v", logs, err)
	}

	db.Close()

	// 重复打开：幂等且正常
	db2, err := Open(path)
	if err != nil {
		t.Fatalf("重复打开数据库失败: %v", err)
	}
	defer db2.Close()

	var ver2 int
	if err := db2.db.QueryRow("PRAGMA user_version").Scan(&ver2); err != nil || ver2 != SchemaVersion {
		t.Fatalf("重复打开版本异常: %d", ver2)
	}
	devices2, err := db2.ListManualDevices()
	if err != nil || len(devices2) != 1 {
		t.Fatalf("重复打开设备数据异常: %v", err)
	}
}

func TestNewDatabaseHasSchemaVersionAndNoTaskTables(t *testing.T) {
	path := filepath.Join(t.TempDir(), "fresh.db")
	db, err := Open(path)
	if err != nil {
		t.Fatalf("创建新数据库失败: %v", err)
	}
	defer db.Close()

	var ver int
	if err := db.db.QueryRow("PRAGMA user_version").Scan(&ver); err != nil || ver != SchemaVersion {
		t.Fatalf("新数据库版本错误: %d, 期望: %d", ver, SchemaVersion)
	}

	var count int
	if err := db.db.QueryRow(`SELECT count(*) FROM sqlite_schema WHERE type='table' AND name IN ('task_runs', 'task_run_logs')`).Scan(&count); err != nil || count != 0 {
		t.Fatalf("新数据库不应包含旧任务表: count=%d", count)
	}
}

func TestRejectUnknownSchemaWithoutReset(t *testing.T) {
	path := filepath.Join(t.TempDir(), "future.db")
	raw, err := sql.Open("sqlite", path)
	if err != nil {
		t.Fatal(err)
	}
	defer raw.Close()
	if _, err = raw.Exec(`CREATE TABLE valuable(value TEXT); INSERT INTO valuable VALUES('keep'); PRAGMA user_version=99999`); err != nil {
		t.Fatal(err)
	}
	if d, err := Open(path); err == nil {
		d.Close()
		t.Fatal("unknown schema accepted")
	}
	var value string
	if err = raw.QueryRow("SELECT value FROM valuable").Scan(&value); err != nil || value != "keep" {
		t.Fatalf("unknown schema was reset: %v", err)
	}
}
