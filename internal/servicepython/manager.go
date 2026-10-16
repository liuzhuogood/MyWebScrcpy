// Package servicepython prepares isolated, application-owned service environments.
package servicepython

import (
	"context"
	"crypto/sha256"
	_ "embed"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"time"

	"github.com/gofrs/flock"
	"mywebscrcpy/internal/dependencies"
	indraocrserver "mywebscrcpy/packages/indraocr-server"
	paddleocrserver "mywebscrcpy/packages/paddleocr-server"
	pythonsdk "mywebscrcpy/packages/python-sdk"
)

//go:embed probe.py
var probe string

const Matching = "matching"
const PaddleOCR = "paddleocr"
const IndraOCR = "indraocr"

type Manager struct {
	dataDir string
	acquire func() (string, func(), error)
	run     func(context.Context, string, ...string) ([]byte, error)
}

func New(dataDir string) *Manager {
	return &Manager{dataDir: dataDir, acquire: dependencies.AcquirePython, run: runPython}
}

func requirements(profile string) ([]string, error) {
	items := []string{"packaging>=24"}
	switch profile {
	case Matching:
		return append(items, "mywebscrcpy-sdk[vision]=="+pythonsdk.Version), nil
	case PaddleOCR, IndraOCR:
		content := paddleocrserver.Requirements
		if profile == IndraOCR {
			content = indraocrserver.Requirements
		}
		for _, line := range strings.Split(content, "\n") {
			if line = strings.TrimSpace(line); line != "" && !strings.HasPrefix(line, "#") {
				items = append(items, line)
			}
		}
		return items, nil
	default:
		return nil, errors.New("未知 Python 服务")
	}
}

func (m *Manager) directory(profile, base string) (string, error) {
	if _, err := requirements(profile); err != nil {
		return "", err
	}
	info, err := os.Stat(base)
	if err != nil {
		return "", err
	}
	// Interpreter changes select a fresh environment, without mutating an active one.
	key := sha256.Sum256(fmt.Appendf(nil, "%s:%d:%d", base, info.Size(), info.ModTime().UnixNano()))
	root, err := filepath.Abs(m.dataDir)
	if err != nil {
		return "", err
	}
	root, err = filepath.EvalSymlinks(root)
	if err != nil {
		return "", err
	}
	return filepath.Join(root, "dependencies", "services", profile, fmt.Sprintf("%x", key[:12])), nil
}

func pythonPath(directory string) string {
	if runtime.GOOS == "windows" {
		return filepath.Join(directory, "Scripts", "python.exe")
	}
	return filepath.Join(directory, "bin", "python")
}

// Acquire inspects an existing service environment; it never installs on status reads.
func (m *Manager) Acquire(profile string) (string, func(), error) {
	base, release, err := m.acquire()
	if err != nil {
		return "", nil, err
	}
	directory, err := m.directory(profile, base)
	if err != nil {
		release()
		return "", nil, err
	}
	if _, err := os.Stat(directory); err == nil {
		lock := flock.New(directory + ".lock")
		locked, err := lock.TryRLock()
		if err != nil || !locked {
			release()
			return "", nil, errors.New("服务依赖正在准备中")
		}
		var once sync.Once
		return pythonPath(directory), func() { once.Do(func() { _ = lock.Unlock(); release() }) }, nil
	}
	return pythonPath(directory), release, nil
}

// Prepare returns only after installation, version checks and imports succeed.
// The lease protects the selected interpreter and environment until the service exits.
func (m *Manager) Prepare(ctx context.Context, profile string, stage func(string)) (string, func(), error) {
	ctx, cancel := context.WithTimeout(ctx, 15*time.Minute)
	defer cancel()
	base, releaseBase, err := m.acquire()
	if err != nil {
		return "", nil, err
	}
	handedOff := false
	defer func() {
		if !handedOff {
			releaseBase()
		}
	}()
	directory, err := m.directory(profile, base)
	if err != nil {
		return "", nil, err
	}
	if err := os.MkdirAll(filepath.Dir(directory), 0700); err != nil {
		return "", nil, err
	}
	lock := flock.New(directory + ".lock")
	locked, err := lock.TryLock()
	if err != nil || !locked {
		return "", nil, fmt.Errorf("服务依赖正在使用或准备中，请稍后重试")
	}
	defer func() {
		if !handedOff {
			_ = lock.Unlock()
		}
	}()
	python := pythonPath(directory)
	notify := func(value string) {
		if stage != nil {
			stage(value)
		}
	}
	if _, err := os.Stat(python); os.IsNotExist(err) {
		notify("installing")
		log.Printf("[ServicePython] 创建服务环境 service=%s", profile)
		if _, err := m.run(ctx, base, "-m", "venv", directory); err != nil {
			return "", nil, fmt.Errorf("创建服务 Python 环境失败: %w", err)
		}
	} else if err != nil {
		return "", nil, err
	}
	if err := m.Check(ctx, profile, python); err != nil {
		notify("installing")
		log.Printf("[ServicePython] 安装或升级依赖 service=%s reason=%v", profile, err)
		items, _ := requirements(profile)
		// Source checkouts can install the exact local SDK before its PyPI release.
		if profile == Matching {
			local := filepath.Join(m.dataDir, "packages", "python-sdk")
			if info, err := os.Stat(filepath.Join(local, "pyproject.toml")); err == nil && !info.IsDir() {
				absolute, err := filepath.Abs(local)
				if err != nil {
					return "", nil, err
				}
				items[len(items)-1] = absolute + "[vision]"
			}
		}
		args := append([]string{"-m", "pip", "install", "--disable-pip-version-check", "--no-input", "--upgrade", "--upgrade-strategy", "only-if-needed"}, items...)
		if _, err := m.run(ctx, python, args...); err != nil {
			return "", nil, fmt.Errorf("服务依赖安装失败: %w", err)
		}
		if err := m.Check(ctx, profile, python); err != nil {
			return "", nil, fmt.Errorf("安装后检查失败: %w", err)
		}
	}
	if err := ctx.Err(); err != nil {
		return "", nil, err
	}
	// Downgrade the exclusive preparation lock to a read lease for the worker.
	_ = lock.Unlock()
	locked, err = lock.TryRLock()
	if err != nil || !locked {
		return "", nil, errors.New("服务环境正在修改，请重试")
	}
	var once sync.Once
	release := func() { once.Do(func() { _ = lock.Unlock(); releaseBase() }) }
	handedOff = true
	notify("starting")
	log.Printf("[ServicePython] 服务依赖已就绪 service=%s", profile)
	return python, release, nil
}

func (m *Manager) Check(ctx context.Context, profile, python string) error {
	items, err := requirements(profile)
	if err != nil {
		return err
	}
	encoded, _ := json.Marshal(items)
	ctx, cancel := context.WithTimeout(ctx, 60*time.Second)
	defer cancel()
	output, err := m.run(ctx, python, "-B", "-c", probe, string(encoded), profile)
	if err != nil {
		return err
	}
	var result struct {
		Ready bool   `json:"ready"`
		Error string `json:"error"`
	}
	if err := json.Unmarshal(output, &result); err != nil {
		return fmt.Errorf("服务依赖检查结果无效: %w", err)
	}
	if !result.Ready {
		return fmt.Errorf("依赖未就绪: %s", result.Error)
	}
	return nil
}

func (m *Manager) PaddleScript(python string) (string, error) {
	file := filepath.Join(filepath.Dir(filepath.Dir(python)), "paddleocr-server.py")
	if err := os.WriteFile(file, paddleocrserver.Script, 0600); err != nil {
		return "", err
	}
	return file, nil
}

func (m *Manager) IndraScript(python string) (string, error) {
	file := filepath.Join(filepath.Dir(filepath.Dir(python)), "indraocr-server.py")
	if err := os.WriteFile(file, indraocrserver.Script, 0600); err != nil {
		return "", err
	}
	return file, nil
}

func runPython(ctx context.Context, python string, args ...string) ([]byte, error) {
	cmd := exec.CommandContext(ctx, python, args...)
	cmd.Dir, cmd.WaitDelay = os.TempDir(), 2*time.Second
	for _, entry := range dependencies.PythonEnvironment() {
		key, _, _ := strings.Cut(entry, "=")
		switch strings.ToUpper(key) {
		case "PYTHONHOME", "PYTHONUSERBASE", "PIP_TARGET", "PIP_PREFIX", "PIP_USER", "PIP_CONFIG_FILE", "PIP_PYTHON":
			continue
		}
		cmd.Env = append(cmd.Env, entry)
	}
	cmd.Env = append(cmd.Env, "PIP_CONFIG_FILE="+os.DevNull)
	var stdout, stderr limitedOutput
	cmd.Stdout, cmd.Stderr = &stdout, &stderr
	if err := cmd.Run(); err != nil {
		if ctx.Err() != nil {
			return nil, ctx.Err()
		}
		return nil, fmt.Errorf("%w: %s", err, strings.TrimSpace(stderr.String()))
	}
	return []byte(stdout.String()), nil
}

type limitedOutput struct{ strings.Builder }

func (w *limitedOutput) Write(p []byte) (int, error) {
	n := len(p)
	if remaining := 8192 - w.Len(); remaining > 0 {
		if len(p) > remaining {
			p = p[:remaining]
		}
		_, _ = w.Builder.Write(p)
	}
	return n, nil
}
