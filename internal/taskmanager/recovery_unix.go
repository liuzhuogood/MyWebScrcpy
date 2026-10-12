//go:build !windows

package taskmanager

import (
	"context"
	"os/exec"
	"strconv"
	"strings"
	"time"
)

func storedProcessOwnsPath(pid int, path string) (bool, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	out, e := exec.CommandContext(ctx, "ps", "-p", strconv.Itoa(pid), "-o", "args=").Output()
	if e != nil {
		if exit, ok := e.(*exec.ExitError); ok && exit.ExitCode() == 1 {
			return false, nil
		}
		return false, e
	}
	return strings.Contains(string(out), path), nil
}
