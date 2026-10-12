//go:build windows

package taskmanager

import (
	"context"
	"os/exec"
	"strconv"
	"strings"
	"time"
)

func storedProcessOwnsPath(pid int, path string) (bool, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	command := "(Get-CimInstance Win32_Process -Filter 'ProcessId = " + strconv.Itoa(pid) + "').CommandLine"
	out, e := exec.CommandContext(ctx, "powershell", "-NoProfile", "-NonInteractive", "-Command", command).Output()
	if e != nil {
		return false, e
	}
	return strings.Contains(string(out), path), nil
}
