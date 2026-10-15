package dependencies

import (
	"bytes"
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"
)

type boundedOutput struct{ bytes.Buffer }

func (b *boundedOutput) Write(p []byte) (int, error) {
	n := len(p)
	if left := 64*1024 - b.Len(); left > 0 {
		if len(p) > left {
			p = p[:left]
		}
		_, _ = b.Buffer.Write(p)
	}
	return n, nil
}
func probe(ctx context.Context, path string, args ...string) (string, error) {
	ctx, cancel := context.WithTimeout(ctx, 4*time.Second)
	defer cancel()
	cmd := exec.CommandContext(ctx, path, args...)
	cmd.Dir = os.TempDir()
	cmd.WaitDelay = time.Second
	var output boundedOutput
	cmd.Stdout = &output
	cmd.Stderr = &output
	if err := cmd.Run(); err != nil {
		return "", fmt.Errorf("程序检查失败: %w", err)
	}
	return strings.TrimSpace(output.String()), nil
}
func (m *Manager) detect(id string, s Selection) Tool {
	tool := Tool{ID: id, Name: id}
	var path string
	var err error
	if id == "adb" {
		path, err = m.resolveADB(s)
	} else if id == "python" {
		path, err = m.resolvePython(s)
	} else {
		tool.Error = "未知依赖"
		return tool
	}
	if err != nil {
		tool.Error = err.Error()
		return tool
	}
	tool.Path = path
	if id == "python" {
		return inspectPython(context.Background(), tool, path)
	}
	arg := "--version"
	if id == "adb" {
		arg = "version"
	}
	out, err := probe(context.Background(), path, arg)
	if err != nil {
		tool.Error = err.Error()
		return tool
	}
	if id == "adb" && !strings.HasPrefix(out, "Android Debug Bridge version ") {
		tool.Error = "指定程序不是 ADB"
		return tool
	}
	tool.Version = strings.SplitN(out, "\n", 2)[0]
	if len(tool.Version) > 512 {
		tool.Version = tool.Version[:512]
	}
	tool.Installed = true
	return tool
}
func (m *Manager) Tools() []ManagedTool {
	settings := m.Settings()
	tools := []ManagedTool{}
	for _, item := range []struct {
		id, name string
		s        Selection
	}{{"adb", "ADB", settings.ADB}, {"python", "Python", settings.Python}} {
		tool := ManagedTool{ID: item.id, Name: item.name, Selection: item.s, Supported: m.supported(item.id), Effective: Tool{ID: item.id, Name: item.name}}
		release, leaseErr := m.acquire(item.id)
		if leaseErr == nil {
			tool.Effective = m.detect(item.id, item.s)
		} else {
			tool.Effective.Error = leaseErr.Error()
		}
		rec, err := m.installation(item.id, true)
		tool.Installed = err == nil
		if err == nil {
			tool.Version = rec.Version
			if item.id == "python" {
				if p, pErr := m.resolvePython(Selection{Source: "builtin"}); pErr == nil {
					tool.BuiltinPath = p
				}
			} else if item.id == "adb" {
				if p, pErr := m.resolveADB(Selection{Source: "builtin"}); pErr == nil {
					tool.BuiltinPath = p
				}
			}
		}
		if _, e := os.Lstat(filepath.Join(m.root, item.id)); e == nil {
			tool.Residual = !tool.Installed
		}
		if leftovers, _ := filepath.Glob(filepath.Join(m.root, ".install-"+item.id+"-*")); len(leftovers) > 0 {
			tool.Residual = true
		}
		if _, e := os.Lstat(filepath.Join(m.root, ".remove-"+item.id)); e == nil {
			tool.Residual = true
		}
		m.mu.RLock()
		tool.Task = m.tasks[item.id]
		if item.id == "python" {
			task := m.tasks[pythonModulesJob]
			tool.ModulesTask = &task
		}
		m.mu.RUnlock()
		if tool.Task.Stage == "" {
			tool.Task.Stage = "idle"
		}
		if release != nil {
			release()
		}
		tools = append(tools, tool)
	}
	return tools
}
