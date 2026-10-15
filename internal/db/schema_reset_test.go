package db

import (
	"database/sql"
	"path/filepath"
	"testing"
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
	run, _, err := current.CreateTaskRun(&TaskRunRecord{ClientRunID: "retained", Name: "new run", Source: "codex", DeviceSerial: "new-device"})
	if err != nil {
		t.Fatal(err)
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
	if _, err = current.GetTaskRunByID(run.ID); err != nil {
		t.Fatalf("restart lost new run: %v", err)
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
