package taskmanager

import (
	"context"
	"fmt"
	"log"
	"os/exec"
	"strconv"
	"strings"
	"syscall"
	"time"
)

func configureTaskProcess(cmd *exec.Cmd) {
	cmd.SysProcAttr = &syscall.SysProcAttr{CreationFlags: syscall.CREATE_NEW_PROCESS_GROUP}
}

func terminateTaskProcess(pid int) error {
	// /T 同时结束子进程，避免仅停止 Python 主进程后遗留后台任务。
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	output, err := exec.CommandContext(ctx, "taskkill", "/PID", strconv.Itoa(pid), "/T", "/F").CombinedOutput()
	if err != nil {
		return fmt.Errorf("终止任务进程树 PID=%d: %w (%s)", pid, err, strings.TrimSpace(string(output)))
	}
	return nil
}

func terminateStoredTaskProcess(pid int) {
	if err := terminateTaskProcess(pid); err != nil {
		log.Printf("[TaskManager] PID=%d 终止已登记进程失败: %v", pid, err)
	}
}
