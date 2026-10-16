package serviceprefs

import (
	"context"
	"path/filepath"
	"testing"
	"time"

	"mywebscrcpy/internal/db"
)

func TestRememberSurvivesRestartAndRestoresOnlyEnabledService(t *testing.T) {
	path := filepath.Join(t.TempDir(), "app.db")
	database, err := db.Open(path)
	if err != nil {
		t.Fatal(err)
	}
	first := New(database)
	if err := first.Remember("matching", true); err != nil {
		t.Fatal(err)
	}
	if err := first.Remember("paddleocr", false); err != nil {
		t.Fatal(err)
	}
	first.Close()
	database.Close()
	database, err = db.Open(path)
	if err != nil {
		t.Fatal(err)
	}
	defer database.Close()
	second := New(database)
	defer second.Close()
	started := make(chan string, 2)
	second.RestoreAfter(40*time.Millisecond, map[string]func(context.Context) error{
		"matching":  func(context.Context) error { started <- "matching"; return nil },
		"paddleocr": func(context.Context) error { started <- "paddleocr"; return nil },
	})
	select {
	case <-started:
		t.Fatal("startup was not delayed")
	case <-time.After(10 * time.Millisecond):
	}
	select {
	case name := <-started:
		if name != "matching" {
			t.Fatal(name)
		}
	case <-time.After(time.Second):
		t.Fatal("enabled service not restored")
	}
	second.Close()
	select {
	case name := <-started:
		t.Fatal("disabled service restored", name)
	default:
	}
}

func TestManualStopCancelsPendingAndInFlightRestore(t *testing.T) {
	for _, delay := range []time.Duration{time.Hour, 0} {
		t.Run(delay.String(), func(t *testing.T) {
			database, err := db.Open(filepath.Join(t.TempDir(), "app.db"))
			if err != nil {
				t.Fatal(err)
			}
			defer database.Close()
			m := New(database)
			defer m.Close()
			if err := m.Remember("matching", true); err != nil {
				t.Fatal(err)
			}
			entered := make(chan struct{})
			m.RestoreAfter(delay, map[string]func(context.Context) error{
				"matching": func(ctx context.Context) error { close(entered); <-ctx.Done(); return ctx.Err() },
			})
			if delay == 0 {
				select {
				case <-entered:
				case <-time.After(time.Second):
					t.Fatal("restore did not enter")
				}
			}
			if err := m.Remember("matching", false); err != nil {
				t.Fatal(err)
			}
			m.Close()
			value, err := database.GetConfig("service_autostart_matching")
			if err != nil || value != "false" {
				t.Fatal("stop preference was not saved", value, err)
			}
		})
	}
}

func TestRestoreServicesRunIndependentlyAndShutdownCancelsBoth(t *testing.T) {
	database, err := db.Open(filepath.Join(t.TempDir(), "app.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer database.Close()
	m := New(database)
	defer m.Close()
	entered := make(chan string, 2)
	starts := map[string]func(context.Context) error{}
	for _, name := range []string{"matching", "paddleocr"} {
		if err := m.Remember(name, true); err != nil {
			t.Fatal(err)
		}
		starts[name] = func(ctx context.Context) error { entered <- name; <-ctx.Done(); return ctx.Err() }
	}
	m.RestoreAfter(0, starts)
	for range 2 {
		select {
		case <-entered:
		case <-time.After(time.Second):
			t.Fatal("one service blocked the other")
		}
	}
	m.Close()
}

func TestSlowStartCannotOverwriteNewerStopPreference(t *testing.T) {
	database, err := db.Open(filepath.Join(t.TempDir(), "app.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer database.Close()
	m := New(database)
	defer m.Close()
	entered, complete := make(chan struct{}), make(chan struct{})
	done := make(chan error, 1)
	go func() { done <- m.Control("matching", true, func() error { close(entered); <-complete; return nil }) }()
	<-entered
	if err := m.Control("matching", false, func() error { return nil }); err != nil {
		t.Fatal(err)
	}
	close(complete)
	if err := <-done; err != nil {
		t.Fatal(err)
	}
	value, err := database.GetConfig("service_autostart_matching")
	if err != nil || value != "false" {
		t.Fatal("stale completion restored enabled state", value, err)
	}
}
