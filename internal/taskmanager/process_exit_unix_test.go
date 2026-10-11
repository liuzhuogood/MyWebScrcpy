//go:build !windows

package taskmanager

import "syscall"

func processExited(pid int) (bool, error) {
	err := syscall.Kill(pid, 0)
	if err == syscall.ESRCH {
		return true, nil
	}
	return false, err
}
