package inputconfig

import (
	"path/filepath"
	"testing"

	"mywebscrcpy/internal/action"
	"mywebscrcpy/internal/db"
)

func openTestDB(t *testing.T) *db.DB {
	t.Helper()
	path := filepath.Join(t.TempDir(), "test.db")
	database, err := db.Open(path)
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	t.Cleanup(func() { database.Close() })
	return database
}

func TestDefaultModeFallback(t *testing.T) {
	s := New(nil)
	if s.Mode() != action.ModeSDK {
		t.Fatalf("default = %s", s.Mode())
	}
}

func TestSetPersistsAndReloads(t *testing.T) {
	database := openTestDB(t)
	s := New(database)
	if err := s.Set(action.ModeHID); err != nil {
		t.Fatalf("set: %v", err)
	}
	if s.Mode() != action.ModeHID {
		t.Fatalf("mode = %s", s.Mode())
	}

	// 模拟重启：新 Store 从数据库加载。
	reloaded := New(database)
	if reloaded.Mode() != action.ModeHID {
		t.Fatalf("reloaded = %s", reloaded.Mode())
	}

	// 改回 sdk 后再读。
	if err := reloaded.Set(action.ModeSDK); err != nil {
		t.Fatalf("set sdk: %v", err)
	}
	if New(database).Mode() != action.ModeSDK {
		t.Fatal("expected sdk after switching back")
	}
}

func TestSetRejectsInvalidMode(t *testing.T) {
	s := New(openTestDB(t))
	for _, mode := range []string{"", "hid ", "mouse"} {
		if err := s.Set(mode); err == nil {
			t.Fatalf("expected error for %q", mode)
		}
	}
	if s.Mode() != action.ModeSDK {
		t.Fatalf("mode changed to %s", s.Mode())
	}
}

func TestMemoryOnlyStore(t *testing.T) {
	s := New(nil)
	if err := s.Set(action.ModeADB); err != nil {
		t.Fatalf("set: %v", err)
	}
	if s.Mode() != action.ModeADB {
		t.Fatalf("mode = %s", s.Mode())
	}
}
