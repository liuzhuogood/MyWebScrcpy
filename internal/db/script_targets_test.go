package db

import (
	"database/sql"
	"encoding/json"
	"path/filepath"
	"reflect"
	"testing"
)

func TestLegacyScriptTargetsMigration(t *testing.T) {
	path := filepath.Join(t.TempDir(), "legacy.db")
	legacy, err := sql.Open("sqlite", path)
	if err != nil {
		t.Fatal(err)
	}
	_, err = legacy.Exec(`CREATE TABLE markdown_scripts (
id INTEGER PRIMARY KEY, name TEXT NOT NULL, target_device TEXT NOT NULL DEFAULT 'auto',
content TEXT NOT NULL DEFAULT '', extracted_version TEXT NOT NULL DEFAULT '', schedule_cron TEXT NOT NULL DEFAULT '',
is_enabled INTEGER NOT NULL DEFAULT 1, timeout_sec INTEGER NOT NULL DEFAULT 120,
compiled_version TEXT NOT NULL DEFAULT '', compiled_python TEXT NOT NULL DEFAULT '', compiled_at INTEGER NOT NULL DEFAULT 0,
created_at INTEGER NOT NULL DEFAULT 0, updated_at INTEGER NOT NULL DEFAULT 0)`)
	if err != nil {
		t.Fatal(err)
	}
	cases := []struct {
		name, serial, content string
		want                  []string
	}{
		{"explicit", "device-a", "---\ntarget_device: device-a\n---", []string{"device-a"}},
		{"auto", "auto", "---\ntarget_device: auto\n---", []string{}},
		{"empty", "", "# 无设备", []string{}},
		{"conflict", "device-a", "---\ntarget_device: device-b\n---", []string{}},
		{"missing-document-field", "device-a", "# 步骤", []string{"device-a"}},
		{"duplicate-document-field", "device-a", "target_device: device-a\ntarget_device: device-b", []string{}},
	}
	for index, test := range cases {
		_, err = legacy.Exec(`INSERT INTO markdown_scripts(id,name,target_device,content,compiled_python,compiled_version) VALUES(?,?,?,?,?,?)`, index+1, test.name, test.serial, test.content, "old hardcoded python", "1.0.0")
		if err != nil {
			t.Fatal(err)
		}
	}
	legacy.Close()
	database, err := Open(path)
	if err != nil {
		t.Fatal(err)
	}
	check := func() {
		for index, test := range cases {
			item, err := database.GetMarkdownScript(int64(index + 1))
			if err != nil {
				t.Fatal(err)
			}
			encoded, _ := json.Marshal(item)
			var result struct {
				Targets  []string `json:"target_devices"`
				Required bool     `json:"needs_device_configuration"`
			}
			if err := json.Unmarshal(encoded, &result); err != nil {
				t.Fatal(err)
			}
			if !reflect.DeepEqual(result.Targets, test.want) {
				t.Errorf("%s: targets=%v want=%v", test.name, result.Targets, test.want)
			}
			if result.Required != (len(test.want) == 0) {
				t.Errorf("%s: configuration status wrong", test.name)
			}
			if len(test.want) == 0 && item.IsEnabled != 0 {
				t.Errorf("%s: ambiguous task must be paused", test.name)
			}
			if len(test.want) == 0 && database.ToggleScriptEnabled(item.ID, 1) == nil {
				t.Errorf("%s: pending task was enabled", test.name)
			}
			if item.Content != test.content {
				t.Errorf("%s: migration rewrote user document", test.name)
			}
			if item.CompiledPython != "" {
				t.Errorf("%s: legacy device-bound artifact was retained", test.name)
			}
		}
	}
	check()
	database.Close()
	database, err = Open(path)
	if err != nil {
		t.Fatal(err)
	}
	defer database.Close()
	check()
}
