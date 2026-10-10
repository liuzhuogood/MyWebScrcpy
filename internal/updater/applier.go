package updater

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"time"
)

// Applier 负责将下载的新二进制文件安全覆盖当前运行文件并可选择重启服务
type Applier struct {
	executablePath string
}

// NewApplier 创建 Applier 实例，默认定位当前运行中的进程二进制路径
func NewApplier() (*Applier, error) {
	execPath, err := os.Executable()
	if err != nil {
		return nil, fmt.Errorf("failed to get executable path: %w", err)
	}
	// 解析软链接
	realPath, err := filepath.EvalSymlinks(execPath)
	if err == nil {
		execPath = realPath
	}
	return &Applier{
		executablePath: execPath,
	}, nil
}

// GetExecutablePath 返回当前二进制路径
func (a *Applier) GetExecutablePath() string {
	return a.executablePath
}

// ApplyUpdate 将 newBinaryPath 替换到当前运行的 executablePath
func (a *Applier) ApplyUpdate(newBinaryPath string) error {
	if _, err := os.Stat(newBinaryPath); err != nil {
		return fmt.Errorf("new binary does not exist: %w", err)
	}

	targetPath := a.executablePath
	backupPath := targetPath + ".old"

	// 赋予新文件执行权限 (Unix 系统)
	if runtime.GOOS != "windows" {
		if err := os.Chmod(newBinaryPath, 0755); err != nil {
			return fmt.Errorf("failed to chmod new binary: %w", err)
		}
	}

	// 清理之前残留的 backup
	_ = os.Remove(backupPath)

	// 将当前运行的文件重命名为 backup
	if err := os.Rename(targetPath, backupPath); err != nil {
		return fmt.Errorf("failed to backup current binary: %w", err)
	}

	// 将新二进制移动到 targetPath
	if err := os.Rename(newBinaryPath, targetPath); err != nil {
		// 恢复原二进制
		_ = os.Rename(backupPath, targetPath)
		return fmt.Errorf("failed to move new binary into place: %w", err)
	}

	// 更新 VERSION 文件 (若在同级目录存在)
	return nil
}

// RestartProcess 尝试异步重启当前进程
func (a *Applier) RestartProcess() error {
	// 在独立 goroutine 中延迟 1 秒退出或 re-exec，确保 HTTP 响应能够完整发回前端
	go func() {
		time.Sleep(1 * time.Second)

		// 启动新进程
		cmd := exec.Command(a.executablePath, os.Args[1:]...)
		cmd.Stdout = os.Stdout
		cmd.Stderr = os.Stderr
		cmd.Stdin = os.Stdin

		if err := cmd.Start(); err != nil {
			// 如果启动失败，直接退出由守护进程拉起
			os.Exit(0)
		}
		os.Exit(0)
	}()

	return nil
}
