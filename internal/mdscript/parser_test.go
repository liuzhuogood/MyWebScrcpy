package mdscript

import (
	"strings"
	"testing"
	"time"
)

func TestParseMarkdown(t *testing.T) {
	mdContent := `---
name: 自动打卡
version: 1.2.0
schedule: "*/15 * * * *"
target_device: 10.0.0.30:5555
timeout_sec: 180
enabled: true
---

# 步骤
- 唤醒手机
- 点击主图标：
  ![首页图标](template://tmpl_home_icon?threshold=0.92)
- 滑动屏幕
`
	meta := ParseMarkdown(mdContent, time.Time{})
	if meta.ExtractedVersion != "1.2.0" {
		t.Fatalf("提取版本错误: got %s, want 1.2.0", meta.ExtractedVersion)
	}
	if meta.IsFallbackVer {
		t.Fatalf("有版本号不应标记为 Fallback")
	}
	if meta.ScheduleCron != "*/15 * * * *" {
		t.Fatalf("提取 Schedule 错误: %s", meta.ScheduleCron)
	}
	if meta.TimeoutSec != 180 {
		t.Fatalf("提取 Timeout 错误: %d", meta.TimeoutSec)
	}
	if !meta.IsEnabled {
		t.Fatalf("提取 Enabled 错误: %v", meta.IsEnabled)
	}
	if len(meta.Templates) != 1 {
		t.Fatalf("提取模板数量错误: %d", len(meta.Templates))
	}
	if meta.Templates[0].ID != "tmpl_home_icon" || meta.Templates[0].Threshold != 0.92 {
		t.Fatalf("提取模板详情错误: %+v", meta.Templates[0])
	}
}

func TestParseMarkdownFallbackTimestamp(t *testing.T) {
	mdWithoutVersion := `---
name: 无版本任务
schedule: "0 8 * * *"
---
- 仅普通步骤
`
	fixedTime := time.Date(2026, 10, 11, 15, 30, 0, 0, time.UTC)
	meta := ParseMarkdown(mdWithoutVersion, fixedTime)
	if meta.ExtractedVersion != "ts_20261011153000" {
		t.Fatalf("时间戳兜底版本错误: %s", meta.ExtractedVersion)
	}
	if !meta.IsFallbackVer {
		t.Fatalf("未写版本应标记为 Fallback")
	}
}

func TestCompilerCacheAndOutput(t *testing.T) {
	compiler := NewCompiler()

	// 1. 缓存复用测试
	if compiler.ShouldRecompile("1.0.0", "1.0.0", "print('code')") {
		t.Fatalf("版本一致且代码非空不应重新生成")
	}
	if !compiler.ShouldRecompile("1.1.0", "1.0.0", "print('code')") {
		t.Fatalf("版本不一致应重新生成")
	}
	if !compiler.ShouldRecompile("1.0.0", "1.0.0", "") {
		t.Fatalf("编译代码为空应重新生成")
	}

	// 2. 编译产物测试
	meta := ParsedMeta{
		ExtractedVersion: "1.2.0",
		ScheduleCron:     "*/5 * * * *",
		TimeoutSec:       60,
	}
	content := `- 第一步操作
- 点击模板：
  ![确认按钮](template://tmpl_ok?threshold=0.88)`

	pyCode, err := compiler.Compile("测试脚本", meta, content)
	if err != nil {
		t.Fatalf("编译失败: %v", err)
	}
	if !strings.Contains(pyCode, "tmpl_ok") || !strings.Contains(pyCode, "0.88") {
		t.Fatalf("生成的 Python 未包含模板匹配: %s", pyCode)
	}
	if !strings.Contains(pyCode, "第一步操作") {
		t.Fatalf("生成的 Python 未包含普通步骤: %s", pyCode)
	}
}
