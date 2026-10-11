package taskmanager

import (
	"path/filepath"
	"testing"
	"time"

	"mywebscrcpy/internal/db"
)

func TestTaskManagerLockAndKill(t *testing.T) {
	tempDir := t.TempDir()
	dbPath := filepath.Join(tempDir, "task_test.db")

	d, err := db.Open(dbPath)
	if err != nil {
		t.Fatalf("打开数据库失败: %v", err)
	}
	defer d.Close()

	mgr := NewManager(d)

	// 长时间运行的 python 脚本
	pythonSleep := "import time\ntime.sleep(10)\n"

	// 1. 启动任务 1
	task1, err := mgr.StartTask(1, "脚本A", "10.0.0.30:5555", 10, pythonSleep)
	if err != nil {
		t.Fatalf("启动任务1失败: %v", err)
	}
	if task1.PID <= 0 {
		t.Fatalf("任务1 PID 非法: %d", task1.PID)
	}

	// 2. 在同一设备上启动任务 2 (应当被排他锁拦截)
	_, err = mgr.StartTask(2, "脚本B", "10.0.0.30:5555", 10, pythonSleep)
	if err == nil {
		t.Fatalf("同一设备启动第二个任务应触发排他锁报错，但成功了")
	}

	// 3. 在不同设备上启动任务 3 (应当允许)
	task3, err := mgr.StartTask(3, "脚本C", "10.0.0.31:5555", 10, pythonSleep)
	if err != nil {
		t.Fatalf("不同设备启动任务应成功: %v", err)
	}
	defer func() {
		_ = mgr.KillTask(task3.ID)
	}()

	// 活跃任务数应为 2
	activeList, err := mgr.ListActiveTasks()
	if err != nil || len(activeList) != 2 {
		t.Fatalf("活跃任务数应为2: got %d, err=%v", len(activeList), err)
	}

	// 4. 强杀任务 1
	err = mgr.KillTask(task1.ID)
	if err != nil {
		t.Fatalf("强杀任务1失败: %v", err)
	}

	time.Sleep(100 * time.Millisecond)

	// 5. 任务1被强杀后，设备锁已释放，应可以重新在该设备启动任务
	task1Again, err := mgr.StartTask(4, "脚本D", "10.0.0.30:5555", 1, "print('done')")
	if err != nil {
		t.Fatalf("锁释放后应能再次在该设备启动任务: %v", err)
	}
	defer func() {
		_ = mgr.KillTask(task1Again.ID)
	}()
}

func TestTaskManagerChineseOutputWithLegacyEncoding(t *testing.T) {
	t.Setenv("PYTHONIOENCODING", "cp1252")
	d, err := db.Open(filepath.Join(t.TempDir(), "encoding.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer d.Close()
	mgr := NewManager(d)
	task, err := mgr.StartTask(1, "中文输出", "device-A", 5, "print('中文任务日志')")
	if err != nil {
		t.Fatal(err)
	}
	defer mgr.KillTask(task.ID)
	deadline := time.Now().Add(4 * time.Second)
	for time.Now().Before(deadline) {
		record, err := d.GetTaskByID(task.ID)
		if err != nil {
			t.Fatal(err)
		}
		if record.Status != "running" {
			if record.Status != "completed" || record.ExitCode != 0 {
				t.Fatalf("中文输出导致任务失败: %+v", record)
			}
			return
		}
		time.Sleep(20 * time.Millisecond)
	}
	t.Fatal("中文输出任务未结束")
}
