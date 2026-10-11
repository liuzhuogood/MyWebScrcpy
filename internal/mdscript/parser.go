package mdscript

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"regexp"
	"strconv"
	"strings"
	"time"
)

var (
	reVersion  = regexp.MustCompile(`(?mi)^version\s*:\s*["']?([a-zA-Z0-9._-]+)["']?`)
	reSchedule = regexp.MustCompile(`(?mi)^schedule\s*:\s*["']?([^"'\r\n]+)["']?`)
	reTimeout  = regexp.MustCompile(`(?mi)^timeout_sec\s*:\s*(\d+)`)
	reEnabled  = regexp.MustCompile(`(?mi)^enabled\s*:\s*(true|false)`)
	reTemplate = regexp.MustCompile(`!\[(.*?)\]\((template:\/\/([^?\s)]+)(?:\?threshold=([0-9.]+))?)\)`)
)

// TemplateRef 脚本中引用的模板信息
type TemplateRef struct {
	Alt       string  `json:"alt"`
	ID        string  `json:"id"`
	Threshold float64 `json:"threshold"`
}

// ParsedMeta 正则提取得到的元数据
type ParsedMeta struct {
	Name             string        `json:"name"`
	ExtractedVersion string        `json:"extracted_version"`
	IsFallbackVer    bool          `json:"is_fallback_ver"`
	ScheduleCron     string        `json:"schedule_cron"`
	TimeoutSec       int           `json:"timeout_sec"`
	IsEnabled        bool          `json:"is_enabled"`
	Templates        []TemplateRef `json:"templates"`
	ContentHash      string        `json:"content_hash"`
}

// ParseMarkdown 提取 Markdown 文本中的元数据
func ParseMarkdown(content string, fallbackTime time.Time) ParsedMeta {
	meta := ParsedMeta{
		TimeoutSec: 120,
		IsEnabled:  true,
	}

	// 1. 版本号提取 (无则按修改时间戳兜底)
	if m := reVersion.FindStringSubmatch(content); len(m) > 1 && strings.TrimSpace(m[1]) != "" {
		meta.ExtractedVersion = strings.TrimSpace(m[1])
		meta.IsFallbackVer = false
	} else {
		if fallbackTime.IsZero() {
			fallbackTime = time.Now()
		}
		meta.ExtractedVersion = fmt.Sprintf("ts_%s", fallbackTime.Format("20060102150405"))
		meta.IsFallbackVer = true
	}

	// 2. Cron 定时周期提取
	if m := reSchedule.FindStringSubmatch(content); len(m) > 1 {
		meta.ScheduleCron = strings.TrimSpace(m[1])
	}

	// 4. 超时时间
	if m := reTimeout.FindStringSubmatch(content); len(m) > 1 {
		if sec, err := strconv.Atoi(m[1]); err == nil && sec > 0 {
			meta.TimeoutSec = sec
		}
	}

	// 5. 是否启用
	if m := reEnabled.FindStringSubmatch(content); len(m) > 1 {
		meta.IsEnabled = strings.ToLower(m[1]) == "true"
	}

	// 6. 模板图片引用提取
	matches := reTemplate.FindAllStringSubmatch(content, -1)
	for _, m := range matches {
		tmpl := TemplateRef{
			Alt:       m[1],
			ID:        m[3],
			Threshold: 0.85,
		}
		if len(m) > 4 && m[4] != "" {
			if th, err := strconv.ParseFloat(m[4], 64); err == nil && th > 0 {
				tmpl.Threshold = th
			}
		}
		meta.Templates = append(meta.Templates, tmpl)
	}

	// 计算哈希
	h := sha256.Sum256([]byte(content))
	meta.ContentHash = hex.EncodeToString(h[:])

	return meta
}
