//go:build !windows

package taskmanager

import (
	"os/exec"
	"syscall"
)

func configureTaskProcess(cmd *exec.Cmd) {
	cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}
}

func terminateTaskProcess(pid int) error {
	return syscall.Kill(-pid, syscall.SIGKILL)
}

func terminateStoredTaskProcess(pid int) {
	_ = syscall.Kill(-pid, syscall.SIGKILL)
	_ = syscall.Kill(pid, syscall.SIGKILL)
}
