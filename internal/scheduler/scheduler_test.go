package scheduler

import (
	"path/filepath"
	"testing"
	"time"

	"mywebscrcpy/internal/db"
	"mywebscrcpy/internal/mdscript"
	"mywebscrcpy/internal/taskmanager"
)

func TestSchedulerLifecycleAndConflictSkip(t *testing.T) {
	tempDir := t.TempDir()
	dbPath := filepath.Join(tempDir, "scheduler_test.db")

	database, err := db.Open(dbPath)
	if err != nil {
		t.Fatalf("打开数据库失败: %v", err)
	}
	defer database.Close()

	taskMgr := taskmanager.NewManager(database)
	defer taskMgr.Close()
	executor := mdscript.NewExecutor(database, taskMgr, func(string) (bool, bool) { return true, true }, func(string, string) error { return nil })
	executor.ArtifactCheck = func(db.MarkdownScriptRecord) error { return nil }
	sched := NewScheduler(database, executor)

	// 验证真实调度注册；周期留足测试时间，避免后台 Cron 干扰手动触发。
	created, err := database.CreateMarkdownScript(
		"定时测试任务",
		[]string{"10.0.0.30:5555"},
		"---\nversion: 1.0.0\nschedule: \"@every 1h\"\n---\n- 测试步骤",
		"1.0.0",
		"@every 1h",
		1,
		10,
	)
	if err != nil {
		t.Fatalf("创建脚本失败: %v", err)
	}

	database.SaveCompiledPython(created.ID, "1.0.0", "import time\ntime.sleep(1)")
	// 2. 启动调度器并验证注册
	if err := sched.Start(); err != nil {
		t.Fatalf("启动调度器失败: %v", err)
	}
	defer sched.Stop()

	entries, err := sched.GetScheduleEntries()
	if err != nil || len(entries) != 1 {
		t.Fatalf("获取调度项失败: entries=%+v, err=%v", entries, err)
	}
	if entries[0].NextRunAt <= 0 {
		t.Fatalf("NextRunAt 应大于 0: %d", entries[0].NextRunAt)
	}

	// 3. 构造设备忙场景：先手动施加锁
	longTask, err := taskMgr.StartTask(999, "占用任务", "10.0.0.30:5555", 10, "import time\ntime.sleep(5)\n")
	if err != nil {
		t.Fatalf("前置启动占用任务失败: %v", err)
	}
	defer func() {
		_ = taskMgr.KillTask(longTask.ID)
	}()

	// 手动触发 worker 模拟到点触发
	sched.triggerWorker(created.ID)

	// 验证状态是否被标记为 skipped
	reloaded, _ := database.GetMarkdownScript(created.ID)
	if reloaded.LastRunStatus != "skipped" {
		t.Fatalf("设备被占用时状态应为 skipped, got %s", reloaded.LastRunStatus)
	}

	// 4. 释放占用锁后再次触发
	_ = taskMgr.KillTask(longTask.ID)
	time.Sleep(50 * time.Millisecond)

	sched.triggerWorker(created.ID)
	afterFree, _ := database.GetMarkdownScript(created.ID)
	if afterFree.LastRunStatus != "started" {
		t.Fatalf("设备空闲时状态应为 success, got %s", afterFree.LastRunStatus)
	}

	// 5. 测试热重载切换为停用
	_ = database.ToggleScriptEnabled(created.ID, 0)
	_ = sched.ReloadScript(created.ID)

	entriesDisabled, _ := sched.GetScheduleEntries()
	if entriesDisabled[0].IsEnabled {
		t.Fatalf("脚本停用后 IsEnabled 应为 false")
	}
}
