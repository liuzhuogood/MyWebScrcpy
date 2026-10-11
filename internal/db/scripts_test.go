package db

import (
	"path/filepath"
	"testing"
)

func TestMarkdownScriptsAndRunningTasks(t *testing.T) {
	tempDir := t.TempDir()
	dbPath := filepath.Join(tempDir, "test.db")

	d, err := Open(dbPath)
	if err != nil {
		t.Fatalf("打开数据库失败: %v", err)
	}
	defer d.Close()

	// 1. 创建脚本
	created, err := d.CreateMarkdownScript("测试签到", []string{"10.0.0.30:5555"}, "# 步骤", "1.0.0", "*/5 * * * *", 1, 60)
	if err != nil {
		t.Fatalf("创建脚本失败: %v", err)
	}
	if created.ID == 0 || created.Name != "测试签到" {
		t.Fatalf("创建脚本返回值不符: %+v", created)
	}

	// 2. 查询列表与详情
	list, err := d.ListMarkdownScripts()
	if err != nil || len(list) != 1 {
		t.Fatalf("查询列表失败: list=%+v, err=%v", list, err)
	}
	detail, err := d.GetMarkdownScript(created.ID)
	if err != nil || detail == nil || detail.ExtractedVersion != "1.0.0" {
		t.Fatalf("获取详情失败: detail=%+v, err=%v", detail, err)
	}

	// 3. 更新脚本与保存编译代码
	err = d.UpdateMarkdownScript(created.ID, "测试签到v2", []string{"10.0.0.30:5555", "10.0.0.31:5555"}, "# 步骤v2", "1.1.0", "0 * * * *", 1, 90)
	if err != nil {
		t.Fatalf("更新脚本失败: %v", err)
	}
	err = d.SaveCompiledPython(created.ID, "1.1.0", "print('hello')")
	if err != nil {
		t.Fatalf("保存编译代码失败: %v", err)
	}

	afterUpdate, err := d.GetMarkdownScript(created.ID)
	if err != nil || afterUpdate.CompiledPython != "print('hello')" || afterUpdate.CompiledVersion != "1.1.0" {
		t.Fatalf("验证更新后脚本失败: %+v", afterUpdate)
	}

	// 4. 任务管理器与设备锁测试
	taskID := "task_test_01"
	err = d.InsertRunningTask(taskID, created.ID, "测试签到v2", "10.0.0.30:5555", 12345)
	if err != nil {
		t.Fatalf("注册任务失败: %v", err)
	}

	// 查询活跃任务
	activeTasks, err := d.ListActiveTasks()
	if err != nil || len(activeTasks) != 1 || activeTasks[0].PID != 12345 {
		t.Fatalf("查询活跃任务失败: %+v, err=%v", activeTasks, err)
	}

	// 检查设备锁
	lockedTask, err := d.GetActiveTaskByDevice("10.0.0.30:5555")
	if err != nil || lockedTask == nil || lockedTask.PID != 12345 {
		t.Fatalf("检查设备锁应锁定: %+v, err=%v", lockedTask, err)
	}

	// 检查未占用设备
	freeTask, err := d.GetActiveTaskByDevice("10.0.0.99:5555")
	if err != nil || freeTask != nil {
		t.Fatalf("未占用设备应返回 nil: %+v, err=%v", freeTask, err)
	}

	// 结束任务 (释放锁)
	err = d.FinishRunningTask(taskID, "completed", 0, "")
	if err != nil {
		t.Fatalf("结束任务失败: %v", err)
	}
	releasedTask, err := d.GetActiveTaskByDevice("10.0.0.30:5555")
	if err != nil || releasedTask != nil {
		t.Fatalf("任务结束后设备应已释放锁: %+v, err=%v", releasedTask, err)
	}

	// 5. 删除脚本
	err = d.DeleteMarkdownScript(created.ID)
	if err != nil {
		t.Fatalf("删除脚本失败: %v", err)
	}
}
