package taskmanager

import (
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"
	"testing"
	"time"

	"mywebscrcpy/internal/db"
)

func TestTimeoutKillsChildProcessAndReleasesOnlyItsDevice(t *testing.T) {
	d, err := db.Open(filepath.Join(t.TempDir(), "timeout.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer d.Close()
	mgr := NewManager(d)
	childPIDPath := filepath.Join(t.TempDir(), "child.pid")
	code := fmt.Sprintf("import subprocess, pathlib, time\np = subprocess.Popen(['python3', '-c', 'import time; time.sleep(30)'])\npathlib.Path(%q).write_text(str(p.pid))\ntime.sleep(30)", childPIDPath)
	task, err := mgr.StartTask(1, "超时", "device-A", 1, code)
	if err != nil {
		t.Fatal(err)
	}
	other, err := mgr.StartTask(2, "继续", "device-B", 10, "import time\ntime.sleep(10)")
	if err != nil {
		t.Fatal(err)
	}
	defer mgr.KillTask(other.ID)
	deadline := time.Now().Add(4 * time.Second)
	for time.Now().Before(deadline) {
		rec, _ := d.GetTaskByID(task.ID)
		if rec.Status == "failed" {
			break
		}
		time.Sleep(20 * time.Millisecond)
	}
	record, _ := d.GetTaskByID(task.ID)
	if record.Status != "failed" || !strings.Contains(record.ErrorMsg, "超时") {
		t.Fatalf("%+v", record)
	}
	raw, err := os.ReadFile(childPIDPath)
	if err != nil {
		t.Fatal(err)
	}
	pid, err := strconv.Atoi(string(raw))
	if err != nil {
		t.Fatal(err)
	}
	deadline = time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		if syscall.Kill(pid, 0) == syscall.ESRCH {
			break
		}
		time.Sleep(20 * time.Millisecond)
	}
	if err := syscall.Kill(pid, 0); err != syscall.ESRCH {
		t.Fatalf("child survived timeout pid=%d err=%v", pid, err)
	}
	active, _ := mgr.GetActiveTaskByDevice("device-A")
	if active != nil {
		t.Fatal("timeout did not release device")
	}
	active, _ = mgr.GetActiveTaskByDevice("device-B")
	if active == nil || active.ID != other.ID {
		t.Fatal("timeout changed another device")
	}
}
