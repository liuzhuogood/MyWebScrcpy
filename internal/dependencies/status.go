package dependencies

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
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
	var searchErr error
	if id == "adb" {
		path, err = m.resolveADB(s)
	} else if id == "pi-agent" {
		var pErr error
		p, pErr := m.resolvePi(s)
		path = p.Executable
		tool.Path = p.Executable
		tool.SearchPaths = map[string]string{"fd": p.FD, "rg": p.RG}
		if path == "" {
			err = pErr
		} else {
			searchErr = pErr
		}
	} else {
		tool.Error = "未知依赖"
		return tool
	}
	if err != nil {
		tool.Error = err.Error()
		return tool
	}
	tool.Path = path
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
	if id == "pi-agent" {
		help, e := probe(context.Background(), path, "--help")
		if e != nil {
			tool.Error = e.Error()
			return tool
		}
		for _, flag := range []string{"--mode", "rpc", "--offline", "--no-session", "--no-extensions", "--no-skills", "--no-prompt-templates", "--no-themes", "--no-context-files", "--tools", "--skill", "--system-prompt"} {
			if !strings.Contains(help, flag) {
				tool.Error = "Pi 缺少所需能力: " + flag
				return tool
			}
		}
		if err := probeRPC(path); err != nil {
			tool.Error = err.Error()
			return tool
		}
		if searchErr != nil {
			tool.Error = searchErr.Error()
			return tool
		}
		for _, name := range []string{"fd", "rg"} {
			out, e := probe(context.Background(), tool.SearchPaths[name], "--version")
			if e != nil || !strings.HasPrefix(out, map[string]string{"fd": "fd ", "rg": "ripgrep "}[name]) {
				tool.Error = fmt.Sprintf("%s 不可用或不是所需程序", name)
				return tool
			}
		}
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
	}{{"pi-agent", "pi-agent", settings.Pi}, {"adb", "ADB", settings.ADB}} {
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

// get_state checks the actual RPC response without contacting a model provider.
func probeRPC(path string) error {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	temp, err := os.MkdirTemp("", "mywebscrcpy-pi-probe-*")
	if err != nil {
		return err
	}
	defer os.RemoveAll(temp)
	args := []string{"--mode", "rpc", "--no-session", "--offline", "--no-extensions", "--no-skills", "--no-prompt-templates", "--no-themes", "--no-context-files", "--tools", "read,grep,find,ls"}
	cmd := exec.CommandContext(ctx, path, args...)
	cmd.Dir = temp
	cmd.WaitDelay = time.Second
	for _, entry := range os.Environ() {
		name, _, _ := strings.Cut(entry, "=")
		if !strings.EqualFold(name, "PI_CODING_AGENT_DIR") && !strings.EqualFold(name, "PI_PACKAGE_DIR") {
			cmd.Env = append(cmd.Env, entry)
		}
	}
	cmd.Env = append(cmd.Env, "PI_CODING_AGENT_DIR="+temp)
	stdin, err := cmd.StdinPipe()
	if err != nil {
		return err
	}
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return err
	}
	var stderr boundedOutput
	cmd.Stderr = &stderr
	if err := cmd.Start(); err != nil {
		return err
	}
	defer func() { cancel(); stdin.Close(); _ = cmd.Wait() }()
	if _, err := io.WriteString(stdin, "{\"id\":\"dependency-check\",\"type\":\"get_state\"}\n"); err != nil {
		return err
	}
	scanner := bufio.NewScanner(stdout)
	scanner.Buffer(make([]byte, 4096), 64*1024)
	for scanner.Scan() {
		var reply struct {
			ID      string `json:"id"`
			Type    string `json:"type"`
			Command string `json:"command"`
			Success bool   `json:"success"`
		}
		if err := json.Unmarshal(scanner.Bytes(), &reply); err != nil {
			return errors.New("Pi RPC 返回无效 JSON")
		}
		if reply.ID == "dependency-check" {
			if reply.Type == "response" && reply.Command == "get_state" && reply.Success {
				return nil
			}
			return errors.New("Pi RPC 能力检查失败")
		}
	}
	return errors.New("Pi 未返回所需 RPC 响应")
}
