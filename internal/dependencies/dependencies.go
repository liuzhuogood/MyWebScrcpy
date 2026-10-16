// Package dependencies describes the external tools used by confirmed features.
package dependencies

import (
	"context"
	"fmt"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"
)

type Tool struct {
	ID          string         `json:"id"`
	Name        string         `json:"name"`
	Purpose     string         `json:"purpose"`
	Installed   bool           `json:"installed"`
	Path        string         `json:"path"`
	Version     string         `json:"version"`
	Error       string         `json:"error"`
	DownloadURL string         `json:"download_url"`
	Python      *PythonModules `json:"python,omitempty"`
}

func PythonPath() (string, error) {
	if active != nil {
		return active.resolvePython(active.Settings().Python)
	}
	return configuredPath("MYWEBSCRCPY_PYTHON", "python3")
}
func ADBPath() (string, error) {
	if active != nil {
		return active.resolveADB(active.Settings().ADB)
	}
	if path := chosenPath("MYWEBSCRCPY_ADB", Current().ADB); path != "" {
		return checkExecutable(path)
	}
	// Preserve the project's confirmed ANDROID_HOME / PATH lookup order.
	if home := os.Getenv("ANDROID_HOME"); home != "" {
		if path, err := exec.LookPath(filepath.Join(home, "platform-tools", "adb")); err == nil {
			return path, nil
		}
	}
	return exec.LookPath("adb")
}
func configuredPath(key, command string) (string, error) {
	configured := ""
	if key == "MYWEBSCRCPY_PYTHON" {
		configured = Current().Python
	}
	value := chosenPath(key, configured)
	if value == "" {
		return exec.LookPath(command)
	}
	if !filepath.IsAbs(value) {
		return "", fmt.Errorf("%s 必须是可执行文件的绝对路径", key)
	}
	path, err := checkExecutable(value)
	if err != nil {
		return "", fmt.Errorf("%s 指定的程序不可用: %w", key, err)
	}
	return path, nil
}
func inspect(tool Tool, path string, pathErr error, args ...string) Tool {
	if pathErr != nil {
		tool.Error = pathErr.Error()
		return tool
	}
	tool.Path = path
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	cmd := exec.CommandContext(ctx, path, args...)
	cmd.Dir = os.TempDir()
	var bounded boundedOutput
	cmd.Stdout = &bounded
	cmd.Stderr = &bounded
	err := cmd.Run()
	output := bounded.Bytes()
	if err != nil {
		tool.Error = "程序检查失败: " + err.Error()
		return tool
	}
	tool.Installed = true
	// Version commands have no user inputs or secrets. Bound their displayed output.
	value := strings.SplitN(strings.TrimSpace(string(output)), "\n", 2)[0]
	if len(value) > 512 {
		value = value[:512]
	}
	tool.Version = value
	return tool
}
func List() []Tool {
	if active != nil {
		release, e := active.acquire("adb")
		if e != nil {
			return []Tool{{ID: "adb", Name: "ADB", Error: e.Error()}}
		}
		defer release()
	}
	adb, adbErr := ADBPath()
	python, pythonErr := PythonPath()
	return []Tool{
		inspect(Tool{ID: "adb", Name: "ADB", Purpose: "设备连接与控制", DownloadURL: "https://developer.android.com/tools/releases/platform-tools"}, adb, adbErr, "version"),
		inspect(Tool{ID: "python", Name: "Python", Purpose: "自动化脚本", DownloadURL: "https://www.python.org/downloads/"}, python, pythonErr, "--version"),
		{ID: "scrcpy-server", Name: "scrcpy-server", Purpose: "设备投屏", Installed: true, Version: "与 App 配套", Path: "内置"},
	}
}
func checkExecutable(path string) (string, error) {
	if !filepath.IsAbs(path) {
		return "", fmt.Errorf("依赖必须使用绝对路径")
	}
	executable, err := exec.LookPath(path)
	if err != nil {
		return "", fmt.Errorf("找不到或无法执行程序：%s", path)
	}
	return executable, nil
}
func RegisterRoutes(mux *http.ServeMux, configurable bool) {
	mux.HandleFunc("GET /api/dependencies", func(w http.ResponseWriter, r *http.Request) {
		var managed []ManagedTool
		var settings Settings
		if active != nil {
			managed = active.Tools()
			settings = active.Settings()
		}
		respond(w, struct {
			Tools        []Tool        `json:"tools"`
			Paths        Paths         `json:"paths"`
			Configurable bool          `json:"configurable"`
			Managed      []ManagedTool `json:"managed"`
			Config       Settings      `json:"config"`
		}{List(), Current(), configurable, managed, settings})
	})
}
