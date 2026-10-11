package taskmanager

import (
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"testing"
	"time"

	"mywebscrcpy/internal/db"
)

func TestManualKillStopsChildProcessAndPreservesOtherDevice(t *testing.T) {
	for _, source := range []string{"进程表", "已登记PID"} {
		t.Run(source, func(t *testing.T) {
			testManualKill(t, source == "已登记PID")
		})
	}
}

func testManualKill(t *testing.T, storedPID bool) {
	t.Helper()
	d, err := db.Open(filepath.Join(t.TempDir(), "kill.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer d.Close()
	mgr := NewManager(d)
	childPIDPath := filepath.Join(t.TempDir(), "child.pid")
	code := fmt.Sprintf("import subprocess, pathlib, time, sys\np = subprocess.Popen([sys.executable, '-c', 'import time; time.sleep(30)'])\npathlib.Path(%q).write_text(str(p.pid))\ntime.sleep(30)", childPIDPath)
	task, err := mgr.StartTask(1, "手动停止", "device-A", 15, code)
	if err != nil {
		t.Fatal(err)
	}
	defer mgr.KillTask(task.ID)
	other, err := mgr.StartTask(2, "继续", "device-B", 15, "import time\ntime.sleep(30)")
	if err != nil {
		t.Fatal(err)
	}
	defer mgr.KillTask(other.ID)
	var childPID int
	deadline := time.Now().Add(5 * time.Second)
	for time.Now().Before(deadline) {
		raw, err := os.ReadFile(childPIDPath)
		if err == nil {
			childPID, err = strconv.Atoi(string(raw))
			if err == nil && childPID > 0 {
				break
			}
		}
		time.Sleep(20 * time.Millisecond)
	}
	if childPID <= 0 {
		t.Fatal("子进程未就绪")
	}
	killer := mgr
	if storedPID {
		// 模拟服务重新创建管理器后，仅能从数据库读取 PID 的清理路径。
		killer = NewManager(d)
	}
	if err := killer.KillTask(task.ID); err != nil {
		t.Fatal(err)
	}
	for _, pid := range []int{task.PID, childPID} {
		deadline = time.Now().Add(3 * time.Second)
		for time.Now().Before(deadline) {
			if exited, err := processExited(pid); exited && err == nil {
				break
			}
			time.Sleep(20 * time.Millisecond)
		}
		if exited, err := processExited(pid); !exited || err != nil {
			t.Fatalf("停止任务后进程仍存活 pid=%d err=%v", pid, err)
		}
	}
	record, err := d.GetTaskByID(task.ID)
	if err != nil || record == nil || record.Status != "killed" {
		t.Fatalf("任务未记录为已终止: %+v err=%v", record, err)
	}
	active, err := mgr.GetActiveTaskByDevice("device-A")
	if err != nil || active != nil {
		t.Fatalf("设备锁未释放: %+v err=%v", active, err)
	}
	active, err = mgr.GetActiveTaskByDevice("device-B")
	if err != nil || active == nil || active.ID != other.ID {
		t.Fatalf("另一设备任务受影响: %+v err=%v", active, err)
	}
	if exited, err := processExited(other.PID); exited || err != nil {
		t.Fatalf("另一设备进程受影响: exited=%v err=%v", exited, err)
	}
}
