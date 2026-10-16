package dependencies

import (
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
	"time"
)

func pythonSelectionFixture(t *testing.T) Selection {
	t.Helper()
	t.Setenv("MYWEBSCRCPY_TEST_PYTHON", t.TempDir())
	t.Setenv("MYWEBSCRCPY_PYTHON", "")
	exe, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	return Selection{Source: "system", Path: exe}
}

func TestPythonSwitchStopsOldProcessAndPersistsSelection(t *testing.T) {
	m := newTestManager(t)
	next := m.Settings()
	next.Python = pythonSelectionFixture(t)
	if err := os.WriteFile(filepath.Join(os.Getenv("MYWEBSCRCPY_TEST_PYTHON"), "slow"), nil, 0600); err != nil {
		t.Fatal(err)
	}
	release, err := m.acquire("python")
	if err != nil {
		t.Fatal(err)
	}
	cmd := exec.Command(next.Python.Path, "-m", "pip")
	if err := cmd.Start(); err != nil {
		release()
		t.Fatal(err)
	}
	waited := false
	t.Cleanup(func() {
		if !waited {
			_ = cmd.Process.Kill()
			_ = cmd.Wait()
			release()
		}
	})
	// ADB activity must not block an independent Python switch.
	releaseADB, err := m.acquire("adb")
	if err != nil {
		t.Fatal(err)
	}
	defer releaseADB()
	m.BeforePythonChange = func() error {
		if lease, err := m.acquire("python"); err == nil {
			lease()
			t.Error("new Python work entered during switch")
		}
		if err := m.InstallPythonModules(next.Python); err == nil {
			t.Error("pip installation entered during switch")
		}
		if err := cmd.Process.Kill(); err != nil {
			return err
		}
		_ = cmd.Wait()
		waited = true
		release()
		return nil
	}
	if err := m.Apply(next); err != nil {
		t.Fatal(err)
	}
	if !waited || cmd.ProcessState == nil {
		t.Fatal("old process not reaped before saving")
	}
	reloaded, err := NewManager(filepath.Dir(m.root))
	if err != nil || reloaded.Settings() != next {
		t.Fatal("new interpreter not persisted", err)
	}
	lease, err := m.acquire("python")
	if err != nil {
		t.Fatal("Python remained frozen after switch", err)
	}
	lease()
}

func TestPythonSwitchRejectsInvalidTargetBeforeStopping(t *testing.T) {
	m := newTestManager(t)
	previous := m.Settings()
	next := previous
	next.Python = Selection{Source: "system", Path: filepath.Join(t.TempDir(), "missing")}
	m.BeforePythonChange = func() error { t.Error("invalid target stopped services"); return nil }
	if err := m.Apply(next); err == nil || m.Settings() != previous {
		t.Fatal("invalid target changed runtime")
	}
}

func TestPythonSwitchStopFailureKeepsSettingsAndReleasesGate(t *testing.T) {
	m := newTestManager(t)
	previous := m.Settings()
	next := previous
	next.Python = pythonSelectionFixture(t)
	failure := errors.New("worker stop timed out")
	m.BeforePythonChange = func() error { return failure }
	if err := m.Apply(next); !errors.Is(err, failure) || m.Settings() != previous {
		t.Fatal("stop failure was not propagated", err)
	}
	lease, err := m.acquire("python")
	if err != nil {
		t.Fatal("failed switch leaked gate", err)
	}
	lease()
	m.BeforePythonChange = nil
	if err := m.Apply(next); err != nil {
		t.Fatal("failed switch leaked installation lock", err)
	}
}

func TestUnchangedSettingsDoNotLockOrStopPython(t *testing.T) {
	m := newTestManager(t)
	releasePython, err := m.acquire("python")
	if err != nil {
		t.Fatal(err)
	}
	defer releasePython()
	releaseADB, err := m.acquire("adb")
	if err != nil {
		t.Fatal(err)
	}
	defer releaseADB()
	m.BeforePythonChange = func() error { t.Error("unchanged settings stopped Python"); return nil }
	if err := m.Apply(m.Settings()); err != nil {
		t.Fatal("unchanged settings blocked by active dependencies", err)
	}
}

func TestPythonSwitchWaitsForExistingProbe(t *testing.T) {
	m := newTestManager(t)
	next := m.Settings()
	next.Python = pythonSelectionFixture(t)
	release, err := m.acquire("python")
	if err != nil {
		t.Fatal(err)
	}
	probeDone := make(chan struct{})
	m.BeforePythonChange = func() error {
		go func() { time.Sleep(100 * time.Millisecond); release(); close(probeDone) }()
		return nil
	}
	err = m.Apply(next)
	<-probeDone
	if err != nil {
		t.Fatal("existing short probe blocked switch", err)
	}
}
