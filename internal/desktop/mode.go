// Package desktop keeps launch selection independent of platform GUI libraries.
package desktop

import (
	"fmt"
	"net"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"time"
)

type Mode string

const (
	Service Mode = "service"
	GUI     Mode = "gui"
)

type Options struct {
	GUI, NoGUI       bool
	Shell            string
	DesktopAvailable bool
}

func Select(o Options) (Mode, error) {
	if o.GUI && o.NoGUI {
		return "", fmt.Errorf("--gui 与 --no-gui 不能同时使用")
	}
	if o.NoGUI {
		return Service, nil
	}
	if o.GUI {
		if o.Shell == "" {
			return "", fmt.Errorf("未安装桌面外壳，请使用桌面分发包或 --no-gui")
		}
		if !o.DesktopAvailable {
			return "", fmt.Errorf("当前会话没有可连接的图形环境，请使用 --no-gui")
		}
		return GUI, nil
	}
	if o.Shell != "" && o.DesktopAvailable {
		return GUI, nil
	}
	return Service, nil
}

func ShellPath() string {
	executable, err := os.Executable()
	if err != nil {
		return ""
	}
	name := "mywebscrcpy-gui"
	if runtime.GOOS == "windows" {
		name += ".exe"
	}
	path := filepath.Join(filepath.Dir(executable), name)
	info, err := os.Stat(path)
	if err != nil || info.IsDir() {
		return ""
	}
	return path
}

// Socket connectivity is a preflight. The GUI host still validates its actual
// display connection; automatic launch can fall back only before host readiness.
func DesktopAvailable() bool {
	if runtime.GOOS != "linux" {
		return true
	}
	return LinuxDisplayAvailable(os.Getenv, canConnect)
}

func canConnect(network, address string) bool {
	conn, err := net.DialTimeout(network, address, 300*time.Millisecond)
	if err != nil {
		return false
	}
	conn.Close()
	return true
}

func LinuxDisplayAvailable(getenv func(string) string, connect func(string, string) bool) bool {
	if display := getenv("WAYLAND_DISPLAY"); display != "" {
		path := display
		if !filepath.IsAbs(path) {
			path = filepath.Join(getenv("XDG_RUNTIME_DIR"), display)
		}
		if filepath.IsAbs(path) && connect("unix", path) {
			return true
		}
	}
	display := getenv("DISPLAY")
	if display == "" {
		return false
	}
	colon := strings.LastIndex(display, ":")
	if colon < 0 {
		return false
	}
	host, number := display[:colon], strings.Split(display[colon+1:], ".")[0]
	if number == "" {
		return false
	}
	for _, ch := range number {
		if ch < '0' || ch > '9' {
			return false
		}
	}
	if host == "" || host == "unix" {
		return connect("unix", "/tmp/.X11-unix/X"+number)
	}
	// Remote displays and SSH forwarding use X11 TCP ports 6000 + display number.
	var n int
	if _, err := fmt.Sscanf(number, "%d", &n); err != nil || n > 59535 {
		return false
	}
	return connect("tcp", net.JoinHostPort(host, fmt.Sprint(6000+n)))
}

// Launch returns fallback=true only when automatic GUI initialization failed.
func Launch(shell string, explicit bool) (fallback bool, err error) {
	dir, err := os.MkdirTemp("", "mywebscrcpy-launch-")
	if err != nil {
		return false, err
	}
	defer os.RemoveAll(dir)
	ready := filepath.Join(dir, "ready")
	cmd := exec.Command(shell, "--launcher-ready", ready)
	cmd.Stdout, cmd.Stderr, cmd.Stdin = os.Stdout, os.Stderr, os.Stdin
	err = cmd.Run()
	if err == nil {
		return false, nil
	}
	if _, statErr := os.Stat(ready); os.IsNotExist(statErr) && !explicit {
		return true, err
	}
	return false, fmt.Errorf("启动桌面外壳失败: %w", err)
}
