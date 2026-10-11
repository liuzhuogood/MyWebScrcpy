package scheduler

import (
	"fmt"
	"os"
	"path/filepath"
	"testing"
	"time"

	"mywebscrcpy/internal/db"
	"mywebscrcpy/internal/mdscript"
	"mywebscrcpy/internal/taskmanager"
)

func TestScheduledTargetsUseSavedListAndReloadForFutureRuns(t *testing.T) {
	dir := t.TempDir()
	d, err := db.Open(filepath.Join(dir, "schedule.db"))
	if err != nil {
		t.Fatal(err)
	}
	mgr := taskmanager.NewManager(d)
	defer func() {
		tasks, _ := mgr.ListActiveTasks()
		for _, task := range tasks {
			_ = mgr.KillTask(task.ID)
		}
		time.Sleep(30 * time.Millisecond)
		d.Close()
	}()
	executor := mdscript.NewExecutor(d, mgr, func(string) (bool, bool) { return true, true }, func(string, string) error { return nil })
	sched := NewScheduler(d, executor)
	content := "version: 1.0.0\n- 等待"
	script, _ := d.CreateMarkdownScript("多设备定时", []string{"device-A", "device-B"}, content, "1.0.0", "0 9 * * *", 1, 10)
	// 各设备写独立文件，避免 Windows 并发追加共享文件的覆盖与换行差异。
	code := fmt.Sprintf("import sys, pathlib, time\n(pathlib.Path(%q) / sys.argv[1]).write_text(sys.argv[1], encoding='utf-8')\ntime.sleep(10)", dir)
	if err := d.SaveCompiledPython(script.ID, "1.0.0", code); err != nil {
		t.Fatal(err)
	}
	if err := sched.ReloadScript(script.ID); err != nil {
		t.Fatal(err)
	}
	sched.triggerWorker(script.ID)
	assertDeviceArgument := func(serial string) {
		t.Helper()
		deadline := time.Now().Add(3 * time.Second)
		var data []byte
		var err error
		for time.Now().Before(deadline) {
			data, err = os.ReadFile(filepath.Join(dir, serial))
			if err == nil && string(data) == serial {
				return
			}
			time.Sleep(20 * time.Millisecond)
		}
		t.Fatalf("device=%s arguments=%q err=%v", serial, data, err)
	}
	assertDeviceArgument("device-A")
	assertDeviceArgument("device-B")
	runs, _ := d.ListScriptRuns(script.ID)
	if len(runs) != 2 {
		t.Fatalf("not dispatched to both devices: %+v", runs)
	}
	for _, task := range runs {
		if task.Status != "running" {
			t.Fatalf("%+v", task)
		}
	}
	if err := d.UpdateMarkdownScript(script.ID, script.Name, []string{"device-C"}, content, "1.0.0", "0 9 * * *", 1, 10); err != nil {
		t.Fatal(err)
	}
	if err := sched.ReloadScript(script.ID); err != nil {
		t.Fatal(err)
	}
	sched.triggerWorker(script.ID)
	latest, _ := d.ListScriptRuns(script.ID)
	if len(latest) != 3 || latest[0].DeviceSerial != "device-C" {
		t.Fatalf("future target not reloaded: %+v", latest)
	}
	for _, original := range runs {
		record, _ := d.GetTaskByID(original.ID)
		if record.DeviceSerial != original.DeviceSerial || record.Status != "running" {
			t.Fatalf("running instance modified: %+v", record)
		}
	}
	assertDeviceArgument("device-C")
}
