package dependencies

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"time"

	"github.com/gofrs/flock"
	pythonsdk "mywebscrcpy/packages/python-sdk"
)

const pythonModulesJob = "python-modules"

type PythonModules struct {
	PipAvailable    bool   `json:"pip_available"`
	SDKVersion      string `json:"sdk_version"`
	SDKError        string `json:"sdk_error"`
	RequiredVersion string `json:"required_version"`
	Ready           bool   `json:"ready"`
}

func pythonRelativePath() string {
	if runtime.GOOS == "windows" {
		return "python.exe"
	}
	return "bin/python3.12"
}

func (m *Manager) resolvePython(s Selection) (string, error) {
	if s.Source == "builtin" {
		if _, err := m.installation("python", false); err != nil {
			return "", fmt.Errorf("请安装内置 Python: %w", err)
		}
		return checkExecutable(filepath.Join(m.root, "python", filepath.FromSlash(pythonRelativePath())))
	}
	name := "python3"
	if runtime.GOOS == "windows" {
		name = "python"
	}
	return pathFor(chosenPath("MYWEBSCRCPY_PYTHON", s.Path), name)
}

// The lease protects the selected runtime during generation and task execution.
func AcquirePython() (string, func(), error) {
	if active == nil {
		path, err := PythonPath()
		return path, func() {}, err
	}
	release, err := active.acquire("python")
	if err != nil {
		return "", nil, err
	}
	path, err := active.resolvePython(active.Settings().Python)
	if err != nil {
		release()
		return "", nil, err
	}
	return path, release, nil
}

const pythonProbe = `import importlib.util, json, sys
from importlib.metadata import PackageNotFoundError, version
if sys.version_info < (3, 10):
    raise RuntimeError("需要 Python 3.10 或更高版本")
sdk_version = ""
sdk_error = ""
try:
    sdk_version = version("mywebscrcpy-sdk")
except PackageNotFoundError:
    pass
if sdk_version:
    try:
        from mywebscrcpy import Client, Device, CommandResult, PageInfo
    except Exception as error:
        sdk_error = str(error)
print(json.dumps({"version": sys.version.split()[0], "pip_available": importlib.util.find_spec("pip") is not None, "sdk_version": sdk_version, "sdk_error": sdk_error}))`

func inspectPython(ctx context.Context, tool Tool, path string) Tool {
	ctx, cancel := context.WithTimeout(ctx, 4*time.Second)
	defer cancel()
	cmd := exec.CommandContext(ctx, path, "-B", "-c", pythonProbe)
	cmd.Dir, cmd.Env, cmd.WaitDelay = os.TempDir(), PythonEnvironment(), time.Second
	var output boundedOutput
	cmd.Stdout, cmd.Stderr = &output, &output
	err := cmd.Run()
	if err != nil {
		tool.Error = "Python 不可用或版本低于 3.10"
		return tool
	}
	var info struct {
		Version      string `json:"version"`
		PipAvailable bool   `json:"pip_available"`
		SDKVersion   string `json:"sdk_version"`
		SDKError     string `json:"sdk_error"`
	}
	if err := strictJSON(output.Bytes(), &info); err != nil || info.Version == "" {
		tool.Error = "Python 检测返回无效结果"
		return tool
	}
	tool.Path, tool.Version, tool.Installed = path, "Python "+info.Version, true
	tool.Python = &PythonModules{PipAvailable: info.PipAvailable, SDKVersion: info.SDKVersion, SDKError: info.SDKError, RequiredVersion: pythonsdk.Version, Ready: info.SDKVersion == pythonsdk.Version && info.SDKError == ""}
	return tool
}

// PythonEnvironment matches task execution: installed user packages remain visible, while inherited
// PYTHONPATH cannot shadow the SDK or pip with an unrelated source directory.
func PythonEnvironment() []string {
	env := make([]string, 0, len(os.Environ())+2)
	for _, entry := range os.Environ() {
		name, _, _ := strings.Cut(entry, "=")
		if !strings.EqualFold(name, "PYTHONPATH") && !strings.EqualFold(name, "PYTHONIOENCODING") && !strings.EqualFold(name, "PYTHONDONTWRITEBYTECODE") {
			env = append(env, entry)
		}
	}
	return append(env, "PYTHONIOENCODING=utf-8", "PYTHONDONTWRITEBYTECODE=1")
}

func (m *Manager) InstallPythonModules(s Selection) error {
	guard, err := m.GuardMutation()
	if err != nil {
		return err
	}
	handedOff := false
	defer func() {
		if !handedOff {
			guard()
		}
	}()
	settings := m.Settings()
	settings.Python = s
	if err := validateShape(settings); err != nil {
		return err
	}
	jobLock := flock.New(filepath.Join(m.root, ".install-lock-python"))
	locked, err := jobLock.TryLock()
	if err != nil || !locked {
		return fmt.Errorf("Python 正在安装或清理")
	}
	defer func() {
		if !handedOff {
			_ = jobLock.Unlock()
		}
	}()
	release, err := m.exclusive([]string{"python"}, false)
	if err != nil {
		return err
	}
	defer func() {
		if !handedOff {
			release()
		}
	}()
	tool := m.detect("python", s)
	if !tool.Installed {
		return fmt.Errorf("Python: %s", tool.Error)
	}
	if !tool.Python.PipAvailable {
		return fmt.Errorf("所选 Python 没有 pip，请先在该环境安装 pip")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Minute)
	m.mu.Lock()
	m.cancels[pythonModulesJob] = cancel
	m.tasks[pythonModulesJob] = InstallTask{Stage: "installing"}
	m.jobs.Add(1)
	m.mu.Unlock()
	handedOff = true
	go func() {
		defer m.jobs.Done()
		defer guard()
		defer jobLock.Unlock()
		defer release()
		defer cancel()
		log.Printf("[Dependencies] Python 模块安装开始 source=%s version=%s", s.Source, pythonsdk.Version)
		cmd := exec.CommandContext(ctx, tool.Path, "-m", "pip", "install", "--disable-pip-version-check", "--no-input", "--index-url", "https://pypi.org/simple", "mywebscrcpy-sdk[vision]=="+pythonsdk.Version)
		cmd.Dir = os.TempDir()
		cmd.Env = PythonEnvironment()
		cmd.WaitDelay = 2 * time.Second
		var output boundedOutput
		cmd.Stdout, cmd.Stderr = &output, &output
		err := cmd.Run()
		if err == nil {
			verified := inspectPython(ctx, Tool{}, tool.Path)
			if !verified.Installed || verified.Python == nil || !verified.Python.Ready {
				err = fmt.Errorf("安装后 SDK 版本或导入验证失败")
			}
		}
		task := InstallTask{Stage: "ready"}
		if err != nil {
			task.Stage = "failed"
			task.Error = fmt.Sprintf("依赖安装失败: %v\n%s", err, strings.TrimSpace(output.String()))
			if ctx.Err() != nil {
				task.Stage = "cancelled"
				task.Error = "依赖安装已取消或超时"
			}
		}
		m.mu.Lock()
		delete(m.cancels, pythonModulesJob)
		m.tasks[pythonModulesJob] = task
		m.mu.Unlock()
		log.Printf("[Dependencies] Python 模块安装结束 stage=%s error=%v", task.Stage, err)
	}()
	return nil
}

func registerPythonModules(mux *http.ServeMux, m *Manager, authorize func(http.HandlerFunc) http.HandlerFunc) {
	mux.HandleFunc("POST /api/dependencies/python/modules", authorize(func(w http.ResponseWriter, r *http.Request) {
		var body struct {
			Selection Selection `json:"selection"`
		}
		if err := decode(w, r, &body); err != nil {
			http.Error(w, "Python 依赖安装参数错误", 400)
			return
		}
		if err := m.InstallPythonModules(body.Selection); err != nil {
			http.Error(w, err.Error(), 409)
			return
		}
		w.WriteHeader(202)
	}))
	mux.HandleFunc("DELETE /api/dependencies/python/modules", authorize(func(w http.ResponseWriter, r *http.Request) {
		if err := m.Cancel(pythonModulesJob); err != nil {
			http.Error(w, err.Error(), 409)
			return
		}
		w.WriteHeader(204)
	}))
}
