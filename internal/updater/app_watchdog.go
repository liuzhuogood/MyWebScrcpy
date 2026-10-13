package updater

import (
	"encoding/json"
	"fmt"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"syscall"
	"time"
)

var appHealthTimeout = 60 * time.Second
var restartApp = func(path string) error { return exec.Command("/usr/bin/open", "-n", path).Run() }

// AppPlan is written by the native macOS updater, outside the application bundle.
type AppPlan struct {
	App     string `json:"app"`
	Backup  string `json:"backup"`
	Data    string `json:"data"`
	Receipt string `json:"receipt"`
	Version string `json:"version"`
	Pending string `json:"pending"`
}

var runtimeFiles = []string{"data/mywebscrcpy.db", "data/mywebscrcpy.db-wal", "data/mywebscrcpy.db-shm", "dependencies.json", "dependencies/config.json"}

func BackupRuntime(data, destination string) error {
	if err := os.MkdirAll(destination, 0700); err != nil {
		return err
	}
	present := []string{}
	for _, name := range runtimeFiles {
		source := filepath.Join(data, name)
		if fileExists(source) {
			if err := copyFile(source, filepath.Join(destination, filepath.Base(name))); err != nil {
				return err
			}
			present = append(present, name)
		}
	}
	return writeJSON(filepath.Join(destination, "files.json"), present)
}
func RestoreRuntime(data, source string) error {
	bytes, err := os.ReadFile(filepath.Join(source, "files.json"))
	if err != nil {
		return err
	}
	var present []string
	if err = json.Unmarshal(bytes, &present); err != nil {
		return err
	}
	for _, name := range runtimeFiles {
		dest := filepath.Join(data, name)
		if err = os.Remove(dest); err != nil && !os.IsNotExist(err) {
			return err
		}
	}
	for _, name := range present {
		if err = os.MkdirAll(filepath.Dir(filepath.Join(data, name)), 0700); err != nil {
			return err
		}
		if err = copyFile(filepath.Join(source, filepath.Base(name)), filepath.Join(data, name)); err != nil {
			return err
		}
	}
	return nil
}

// WatchApp runs from a copied Go executable, surviving replacement of the bundle.
func WatchApp(path string) error {
	if runtime.GOOS != "darwin" {
		return fmt.Errorf("App watchdog only supports macOS bundles")
	}
	bytes, err := os.ReadFile(path)
	if err != nil {
		return err
	}
	var plan AppPlan
	if err = json.Unmarshal(bytes, &plan); err != nil {
		return err
	}
	if filepath.Ext(plan.App) != ".app" || !fileExists(plan.Backup) {
		return fmt.Errorf("invalid application backup")
	}
	deadline := time.Now().Add(120 * time.Second)
	for !fileExists(filepath.Join(filepath.Dir(path), "installed")) {
		if !fileExists(plan.Pending) {
			return nil
		}
		if time.Now().After(deadline) {
			return fmt.Errorf("安装未确认完成，保留 App 和数据备份")
		}
		time.Sleep(200 * time.Millisecond)
	}
	if err = waitHealthy(updatePlan{Receipt: plan.Receipt, Version: plan.Version}, appHealthTimeout); err == nil {
		err = os.WriteFile(plan.Receipt+".confirmed", []byte("confirmed"), 0600)
		if err == nil {
			_ = os.Remove(plan.Pending)
			log.Printf("[Updater] 完整 App %s 启动验证通过", plan.Version)
			return nil
		}
	}
	// The new host publishes only the PIDs it owns before spawning its backend.
	var pids struct {
		Host    int `json:"host"`
		Backend int `json:"backend"`
	}
	if bytes, readErr := os.ReadFile(plan.Receipt + ".pids"); readErr == nil {
		if json.Unmarshal(bytes, &pids) != nil {
			return fmt.Errorf("invalid startup process receipt")
		}
		for _, pid := range []int{pids.Backend, pids.Host} {
			if pid > 1 {
				if process, findErr := os.FindProcess(pid); findErr == nil {
					_ = process.Kill()
				}
			}
		}
		for _, pid := range []int{pids.Backend, pids.Host} {
			if pid > 1 {
				process, _ := os.FindProcess(pid)
				deadline := time.Now().Add(10 * time.Second)
				for process.Signal(syscall.Signal(0)) == nil {
					if time.Now().After(deadline) {
						return fmt.Errorf("新版进程未退出，保留备份并停止回滚")
					}
					time.Sleep(100 * time.Millisecond)
				}
			}
		}
	} else {
		return fmt.Errorf("新版未启动且无法确认进程状态；保留备份 %s，未自动覆盖应用", plan.Backup)
	}
	log.Printf("[Updater] 新 App 未通过启动验证，恢复完整 App 与数据库")
	// Move the failed bundle aside on the same filesystem; never merge bundle files.
	failed := plan.App + ".failed-update"
	if fileExists(failed) {
		return fmt.Errorf("存在上次失败包 %s，请先处理", failed)
	}
	if err = os.Rename(plan.App, failed); err != nil {
		return err
	}
	if output, copyErr := exec.Command("/usr/bin/ditto", plan.Backup, plan.App).CombinedOutput(); copyErr != nil {
		_ = os.RemoveAll(plan.App)
		_ = os.Rename(failed, plan.App)
		return fmt.Errorf("恢复 App 失败: %v: %s", copyErr, output)
	}
	if err = RestoreRuntime(plan.Data, filepath.Join(filepath.Dir(path), "runtime")); err != nil {
		return err
	}
	_ = os.Remove(plan.Pending)
	if err = restartApp(plan.App); err != nil {
		return err
	}
	return fmt.Errorf("新版启动失败，已恢复旧 App；失败包保留在 %s", failed)
}
