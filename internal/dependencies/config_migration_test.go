package dependencies

import (
	"os"
	"path/filepath"
	"testing"
)

func TestOldDependencyConfigResetsWithoutPiMigration(t *testing.T) {
	dir := t.TempDir()
	root := filepath.Join(dir, "dependencies")
	if err := os.Mkdir(root, 0700); err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(root, "config.json")
	if err := os.WriteFile(path, []byte(`{"schema":1,"pi":{"source":"builtin"},"adb":{"source":"system","path":"/old-adb"},"python":{"source":"system","path":"/old-python"}}`), 0600); err != nil {
		t.Fatal(err)
	}
	m, err := NewManager(dir)
	if err != nil {
		t.Fatal(err)
	}
	defer m.Close()
	if m.Settings().Schema != 2 || m.Settings().ADB.Path == "/old-adb" || m.Settings().Python.Path == "/old-python" {
		t.Fatal("old selections were migrated")
	}
	if _, err := os.Stat(filepath.Join(root, "config.v1.backup.json")); !os.IsNotExist(err) {
		t.Fatal("unexpected legacy backup")
	}
	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	var settings Settings
	if err := strictJSON(raw, &settings); err != nil {
		t.Fatalf("reset schema contains removed fields: %v", err)
	}
	next, err := NewManager(dir)
	if err != nil {
		t.Fatal(err)
	}
	defer next.Close()
	if next.Settings() != m.Settings() {
		t.Fatal("new configuration not retained")
	}
}

func TestUnknownDependencySchemaIsRejected(t *testing.T) {
	dir := t.TempDir()
	root := filepath.Join(dir, "dependencies")
	os.Mkdir(root, 0700)
	path := filepath.Join(root, "config.json")
	raw := []byte(`{"schema":999,"pi":{}}`)
	os.WriteFile(path, raw, 0600)
	if m, err := NewManager(dir); err == nil {
		m.Close()
		t.Fatal("unknown schema accepted")
	}
	after, _ := os.ReadFile(path)
	if string(after) != string(raw) {
		t.Fatal("unknown schema overwritten")
	}
}
