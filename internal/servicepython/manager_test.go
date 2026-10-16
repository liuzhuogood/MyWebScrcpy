package servicepython

import (
	"context"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync/atomic"
	"testing"
	"time"
)

func fixture(t *testing.T) (*Manager, *atomic.Int32, *atomic.Int32) {
	t.Helper()
	m := New(t.TempDir())
	base, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	leases, installs := &atomic.Int32{}, &atomic.Int32{}
	m.acquire = func() (string, func(), error) {
		leases.Add(1)
		return base, func() { leases.Add(-1) }, nil
	}
	ready := map[string]bool{}
	m.run = func(ctx context.Context, python string, args ...string) ([]byte, error) {
		if err := ctx.Err(); err != nil {
			return nil, err
		}
		if args[0] == "-m" && args[1] == "venv" {
			file := pythonPath(args[2])
			if err := os.MkdirAll(filepath.Dir(file), 0700); err != nil {
				return nil, err
			}
			return nil, os.WriteFile(file, []byte("fixture"), 0700)
		}
		if args[0] == "-m" && args[1] == "pip" {
			if !strings.Contains(strings.Join(args, " "), "--upgrade --upgrade-strategy only-if-needed") {
				t.Error("missing controlled upgrade options")
			}
			installs.Add(1)
			ready[python] = true
			return nil, nil
		}
		if ready[python] {
			return []byte(`{"ready":true,"error":""}`), nil
		}
		return []byte(`{"ready":false,"error":"installed version too old"}`), nil
	}
	return m, leases, installs
}

func TestPrepareInstallsOnceAndProtectsRunningEnvironment(t *testing.T) {
	m, leases, installs := fixture(t)
	var stages []string
	python, release, err := m.Prepare(context.Background(), Matching, func(stage string) { stages = append(stages, stage) })
	if err != nil {
		t.Fatal(err)
	}
	if installs.Load() != 1 || leases.Load() != 1 {
		t.Fatal("environment not prepared or leased")
	}
	if _, _, err := m.Prepare(context.Background(), Matching, nil); err == nil {
		t.Fatal("modified a running environment")
	}
	if leases.Load() != 1 {
		t.Fatal("failed preparation leaked base lease")
	}
	release()
	release()
	second, release, err := m.Prepare(context.Background(), Matching, nil)
	if err != nil {
		t.Fatal(err)
	}
	release()
	if second != python || installs.Load() != 1 || leases.Load() != 0 {
		t.Fatal("healthy environment was reinstalled or leaked")
	}
	if !strings.Contains(strings.Join(stages, ","), "installing") || stages[len(stages)-1] != "starting" {
		t.Fatal(stages)
	}
}

func TestServicesHaveSeparateEnvironments(t *testing.T) {
	m, leases, _ := fixture(t)
	matching, releaseMatching, err := m.Prepare(context.Background(), Matching, nil)
	if err != nil {
		t.Fatal(err)
	}
	defer releaseMatching()
	paddle, releasePaddle, err := m.Prepare(context.Background(), PaddleOCR, nil)
	if err != nil {
		t.Fatal(err)
	}
	defer releasePaddle()
	if matching == paddle || leases.Load() != 2 {
		t.Fatal("service environments not isolated")
	}
	script, err := m.PaddleScript(paddle)
	if err != nil {
		t.Fatal(err)
	}
	data, err := os.ReadFile(script)
	if err != nil || !strings.Contains(string(data), "def get_ocr") {
		t.Fatal("packaged OCR script unavailable", err)
	}
}

func TestFailedInstallAndFailedVerificationNeverReturnRuntime(t *testing.T) {
	for _, mode := range []string{"pip-error", "still-old"} {
		t.Run(mode, func(t *testing.T) {
			m, leases, _ := fixture(t)
			run := m.run
			m.run = func(ctx context.Context, python string, args ...string) ([]byte, error) {
				if args[0] == "-m" && args[1] == "pip" && mode == "pip-error" {
					return nil, errors.New("download failed")
				}
				if args[0] == "-B" {
					return []byte(`{"ready":false,"error":"version still wrong"}`), nil
				}
				return run(ctx, python, args...)
			}
			path, release, err := m.Prepare(context.Background(), Matching, nil)
			if err == nil || path != "" || release != nil || leases.Load() != 0 {
				t.Fatal("failed environment accepted or leaked")
			}
		})
	}
}

func TestCancelledInstallReleasesEnvironmentForRetry(t *testing.T) {
	m, leases, _ := fixture(t)
	run := m.run
	entered := make(chan struct{})
	m.run = func(ctx context.Context, python string, args ...string) ([]byte, error) {
		if args[0] == "-m" && args[1] == "pip" {
			close(entered)
			<-ctx.Done()
			return nil, ctx.Err()
		}
		return run(ctx, python, args...)
	}
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan error, 1)
	go func() { _, _, err := m.Prepare(ctx, Matching, nil); done <- err }()
	<-entered
	cancel()
	if err := <-done; !errors.Is(err, context.Canceled) {
		t.Fatal(err)
	}
	if leases.Load() != 0 {
		t.Fatal("cancelled install leaked runtime")
	}
	m.run = run
	_, release, err := m.Prepare(context.Background(), Matching, nil)
	if err != nil {
		t.Fatal(err)
	}
	release()
}

// Opt in to real pip downloads in a disposable environment, never the user's runtime.
func TestRealMatchingEnvironment(t *testing.T) {
	base := os.Getenv("MYWEBSCRCPY_SERVICE_PYTHON_TEST")
	if base == "" {
		t.Skip("set MYWEBSCRCPY_SERVICE_PYTHON_TEST to a Python executable")
	}
	root := t.TempDir()
	source, err := filepath.Abs("../../packages/python-sdk")
	if err != nil {
		t.Fatal(err)
	}
	local := filepath.Join(root, "packages", "python-sdk")
	if err := os.MkdirAll(local, 0700); err != nil {
		t.Fatal(err)
	}
	for _, name := range []string{"README.md", "pyproject.toml"} {
		data, err := os.ReadFile(filepath.Join(source, name))
		if err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(local, name), data, 0600); err != nil {
			t.Fatal(err)
		}
	}
	if err := os.CopyFS(filepath.Join(local, "src"), os.DirFS(filepath.Join(source, "src"))); err != nil {
		t.Fatal(err)
	}
	m := New(root)
	m.acquire = func() (string, func(), error) { return base, func() {}, nil }
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Minute)
	defer cancel()
	python, release, err := m.Prepare(ctx, Matching, func(stage string) { t.Log(stage) })
	if err != nil {
		t.Fatal(err)
	}
	cmd := exec.CommandContext(ctx, python, "-c", "from importlib.metadata import version; from mywebscrcpy import Matcher; print(version('mywebscrcpy-sdk'))")
	output, err := cmd.CombinedOutput()
	if err != nil {
		release()
		t.Fatal(err, string(output))
	}
	t.Log("installed SDK:", string(output))
	release()
	_, release, err = m.Prepare(ctx, Matching, func(stage string) {
		if stage == "installing" {
			t.Error("healthy environment reinstalled")
		}
	})
	if err != nil {
		t.Fatal(err)
	}
	release()
}

func TestIndraEnvironmentAndEmbeddedScript(t *testing.T) {
	m, leases, _ := fixture(t)
	python, release, err := m.Prepare(context.Background(), IndraOCR, nil)
	if err != nil {
		t.Fatal(err)
	}
	defer release()
	script, err := m.IndraScript(python)
	if err != nil {
		t.Fatal(err)
	}
	data, err := os.ReadFile(script)
	if err != nil || !strings.Contains(string(data), "turn=False") || !strings.Contains(python, IndraOCR) || leases.Load() != 1 {
		t.Fatal("Indra runtime is not isolated or script missing", err)
	}
	items, err := requirements(IndraOCR)
	if err != nil || !strings.Contains(strings.Join(items, " "), "indra-ocr==0.3.2") {
		t.Fatal(items, err)
	}
}
