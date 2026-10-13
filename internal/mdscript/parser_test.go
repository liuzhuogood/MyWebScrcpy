package mdscript

import (
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
