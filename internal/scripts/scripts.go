package scripts

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"time"
)

// 分类名/脚本名白名单：只允许中文、字母、数字、_、-
var validNameRe = regexp.MustCompile(`^[\p{Han}\w-]+$`)

// ValidName 检查分类名或脚本名是否合法。
func ValidName(name string) bool {
	if name == "" || strings.Contains(name, "..") || strings.Contains(name, "/") || strings.Contains(name, "\\") {
		return false
	}
	return validNameRe.MatchString(name)
}

// ScriptsRoot 返回脚本工作目录，可由 SCRIPTS_DIR 环境变量覆盖。
func ScriptsRoot() string {
	if d := os.Getenv("SCRIPTS_DIR"); d != "" {
		return d
	}
	return "scripts"
}

// EnsureRoot 确保脚本目录存在，首次创建时写入示例分类和示例脚本。
func EnsureRoot() error {
	root := ScriptsRoot()
	if _, err := os.Stat(root); os.IsNotExist(err) {
		if err := os.MkdirAll(root, 0755); err != nil {
			return fmt.Errorf("创建脚本目录失败: %w", err)
		}
		// 写入 README 说明
		readme := `# 脚本目录

本目录由 MyWebScrcpy 脚本管理功能自动管理。
每个子目录为一个「分类」，分类下的 .py 文件为脚本。
脚本元数据通过文件头部 # @key value 注释块承载，请勿手动编辑。

如需操作，请通过 Web 界面进行。
`
		if err := os.WriteFile(filepath.Join(root, "README.md"), []byte(readme), 0644); err != nil {
			return fmt.Errorf("写入 README 失败: %w", err)
		}
		// 创建示例分类
		now := time.Now().Format(time.RFC3339)
		catDir := filepath.Join(root, "示例分类")
		if err := os.MkdirAll(catDir, 0755); err != nil {
			return fmt.Errorf("创建示例分类失败: %w", err)
		}
		// 写入示例脚本
		sample := fmt.Sprintf(`# -*- coding: utf-8 -*-
# @name 示例脚本
# @description 这是一个示例 Python 脚本，可供参考
# @params 
# @created %s
# @updated %s

import subprocess


def main():
    """在此实现你的脚本逻辑"""
    print("Hello from MyWebScrcpy!")
`, now, now)
		if err := os.WriteFile(filepath.Join(catDir, "示例脚本.py"), []byte(sample), 0644); err != nil {
			return fmt.Errorf("写入示例脚本失败: %w", err)
		}
	}
	return nil
}

// safePath 拼接路径并校验是否仍在脚本根目录内，防止目录穿越。
func safePath(parts ...string) (string, error) {
	root := ScriptsRoot()
	absRoot, err := filepath.Abs(root)
	if err != nil {
		return "", fmt.Errorf("获取脚本目录绝对路径失败: %w", err)
	}
	p := filepath.Join(append([]string{absRoot}, parts...)...)
	rel, err := filepath.Rel(absRoot, p)
	if err != nil {
		return "", fmt.Errorf("路径计算失败: %w", err)
	}
	if strings.HasPrefix(rel, "..") || filepath.IsAbs(rel) {
		return "", fmt.Errorf("路径越界: %s", rel)
	}
	return p, nil
}

// ScriptInfo 脚本元数据（也用于创建/更新请求）。
type ScriptInfo struct {
	Name        string `json:"name"`
	Description string `json:"description"`
	Params      string `json:"params"`
	Content     string `json:"content,omitempty"`
	Created     string `json:"created,omitempty"`
	Updated     string `json:"updated,omitempty"`
}

// ScriptDetail 包含元数据和脚本正文。
type ScriptDetail struct {
	Name        string `json:"name"`
	Description string `json:"description"`
	Params      string `json:"params"`
	Content     string `json:"content"`
	Created     string `json:"created"`
	Updated     string `json:"updated"`
}

// --- 分类操作 ---

// ListCategories 列出所有分类（子目录名）。
func ListCategories() ([]string, error) {
	root := ScriptsRoot()
	entries, err := os.ReadDir(root)
	if err != nil {
		return nil, err
	}
	var cats []string
	for _, e := range entries {
		if e.IsDir() {
			cats = append(cats, e.Name())
		}
	}
	return cats, nil
}

// CreateCategory 创建新分类。
func CreateCategory(name string) error {
	if !ValidName(name) {
		return fmt.Errorf("分类名不合法，只允许中文、字母、数字、_、-")
	}
	p, err := safePath(name)
	if err != nil {
		return err
	}
	if _, err := os.Stat(p); err == nil {
		return fmt.Errorf("分类 %s 已存在", name)
	}
	return os.MkdirAll(p, 0755)
}

// RenameCategory 重命名分类。
func RenameCategory(oldName, newName string) error {
	if !ValidName(oldName) || !ValidName(newName) {
		return fmt.Errorf("分类名不合法")
	}
	oldPath, err := safePath(oldName)
	if err != nil {
		return err
	}
	newPath, err := safePath(newName)
	if err != nil {
		return err
	}
	if _, err := os.Stat(oldPath); os.IsNotExist(err) {
		return fmt.Errorf("分类 %s 不存在", oldName)
	}
	if _, err := os.Stat(newPath); err == nil {
		return fmt.Errorf("分类 %s 已存在", newName)
	}
	return os.Rename(oldPath, newPath)
}

// DeleteCategory 删除分类（含其下所有脚本）。
func DeleteCategory(name string) error {
	if !ValidName(name) {
		return fmt.Errorf("分类名不合法")
	}
	p, err := safePath(name)
	if err != nil {
		return err
	}
	if _, err := os.Stat(p); os.IsNotExist(err) {
		return fmt.Errorf("分类 %s 不存在", name)
	}
	return os.RemoveAll(p)
}

// --- 脚本操作 ---

// listScriptFiles 列出分类下的所有 .py 文件（返回不含扩展名的名字）。
func listScriptFiles(category string) ([]string, error) {
	catPath, err := safePath(category)
	if err != nil {
		return nil, err
	}
	if _, err := os.Stat(catPath); os.IsNotExist(err) {
		return nil, fmt.Errorf("分类 %s 不存在", category)
	}
	entries, err := os.ReadDir(catPath)
	if err != nil {
		return nil, err
	}
	var names []string
	for _, e := range entries {
		if !e.IsDir() && strings.HasSuffix(e.Name(), ".py") {
			names = append(names, strings.TrimSuffix(e.Name(), ".py"))
		}
	}
	return names, nil
}

// scriptFilePath 返回脚本文件的完整路径（含 .py 扩展名）。
func scriptFilePath(category, name string) (string, error) {
	if !ValidName(name) {
		return "", fmt.Errorf("脚本名不合法: %s", name)
	}
	return safePath(category, name+".py")
}

// ParseMetadata 解析文件头部的 # @key value 元数据。
func ParseMetadata(content string) map[string]string {
	meta := map[string]string{}
	lines := strings.Split(content, "\n")
	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		if strings.HasPrefix(trimmed, "# @") {
			rest := strings.TrimPrefix(trimmed, "# @")
			if idx := strings.Index(rest, " "); idx > 0 {
				meta[rest[:idx]] = strings.TrimSpace(rest[idx+1:])
			} else {
				meta[rest] = ""
			}
		} else if !strings.HasPrefix(trimmed, "#") && trimmed != "" {
			break
		}
	}
	return meta
}

// StripHeader 去掉文件头部的注释块（# 开头的行），返回脚本正文。
func StripHeader(content string) string {
	lines := strings.Split(content, "\n")
	start := 0
	for i, line := range lines {
		trimmed := strings.TrimSpace(line)
		if trimmed == "" || strings.HasPrefix(trimmed, "#") {
			start = i + 1
		} else {
			break
		}
	}
	if start >= len(lines) {
		return ""
	}
	return strings.Join(lines[start:], "\n")
}

// BuildHeader 根据元数据构建文件头部注释块。
func BuildHeader(info ScriptInfo) string {
	return fmt.Sprintf(`# -*- coding: utf-8 -*-
# @name %s
# @description %s
# @params %s
# @created %s
# @updated %s
`, info.Name, info.Description, info.Params, info.Created, info.Updated)
}

// ListScripts 列出某分类下所有脚本的元数据。
func ListScripts(category string) ([]ScriptInfo, error) {
	names, err := listScriptFiles(category)
	if err != nil {
		return nil, err
	}
	var infos []ScriptInfo
	for _, name := range names {
		fp, err := scriptFilePath(category, name)
		if err != nil {
			continue
		}
		data, err := os.ReadFile(fp)
		if err != nil {
			continue
		}
		meta := ParseMetadata(string(data))
		infos = append(infos, ScriptInfo{
			Name:        name,
			Description: meta["description"],
			Params:      meta["params"],
			Created:     meta["created"],
			Updated:     meta["updated"],
		})
	}
	return infos, nil
}

// GetScript 读取脚本完整内容，返回元数据和正文。
func GetScript(category, name string) (*ScriptDetail, error) {
	fp, err := scriptFilePath(category, name)
	if err != nil {
		return nil, err
	}
	data, err := os.ReadFile(fp)
	if err != nil {
		return nil, fmt.Errorf("读取脚本失败: %w", err)
	}
	content := string(data)
	meta := ParseMetadata(content)
	return &ScriptDetail{
		Name:        name,
		Description: meta["description"],
		Params:      meta["params"],
		Created:     meta["created"],
		Updated:     meta["updated"],
		Content:     StripHeader(content),
	}, nil
}

// CreateScript 创建新脚本。
func CreateScript(category string, info ScriptInfo) (*ScriptDetail, error) {
	if !ValidName(category) || !ValidName(info.Name) {
		return nil, fmt.Errorf("分类名或脚本名不合法")
	}
	if info.Content == "" {
		info.Content = "import subprocess\n\n\ndef main():\n    pass\n"
	}
	now := time.Now().Format(time.RFC3339)
	info.Created = now
	info.Updated = now
	header := BuildHeader(info)
	fullContent := header + "\n" + info.Content

	fp, err := scriptFilePath(category, info.Name)
	if err != nil {
		return nil, err
	}
	if _, err := os.Stat(fp); err == nil {
		return nil, fmt.Errorf("脚本 %s 已存在", info.Name)
	}
	if err := os.WriteFile(fp, []byte(fullContent), 0644); err != nil {
		return nil, fmt.Errorf("写入脚本失败: %w", err)
	}
	return &ScriptDetail{
		Name:        info.Name,
		Description: info.Description,
		Params:      info.Params,
		Content:     info.Content,
		Created:     now,
		Updated:     now,
	}, nil
}

// ErrConflict 表示更新冲突（If-Match 校验失败）。
var ErrConflict = fmt.Errorf("冲突：脚本已被其他操作修改，请刷新后重试")

// UpdateScript 更新脚本内容，支持改名（移动文件）。
// updatedIfMatch 若非空，与当前文件的 @updated 值比对，不一致返回 ErrConflict。
func UpdateScript(category, name string, info ScriptInfo, updatedIfMatch string) (*ScriptDetail, error) {
	fp, err := scriptFilePath(category, name)
	if err != nil {
		return nil, err
	}
	data, err := os.ReadFile(fp)
	if err != nil {
		return nil, fmt.Errorf("读取原脚本失败: %w", err)
	}
	oldMeta := ParseMetadata(string(data))

	// If-Match 轻量并发保护
	if updatedIfMatch != "" && oldMeta["updated"] != "" && oldMeta["updated"] != updatedIfMatch {
		return nil, ErrConflict
	}

	now := time.Now().Format(time.RFC3339)
	created := oldMeta["created"]
	if created == "" {
		created = now
	}

	newName := info.Name
	if newName == "" {
		newName = name
	}

	buildInfo := ScriptInfo{
		Name:        newName,
		Description: info.Description,
		Params:      info.Params,
		Created:     created,
		Updated:     now,
	}
	header := BuildHeader(buildInfo)
	fullContent := header + "\n" + info.Content

	if newName != name {
		newFp, err := scriptFilePath(category, newName)
		if err != nil {
			return nil, err
		}
		if _, err := os.Stat(newFp); err == nil {
			return nil, fmt.Errorf("脚本 %s 已存在", newName)
		}
		if err := os.WriteFile(newFp, []byte(fullContent), 0644); err != nil {
			return nil, fmt.Errorf("写入新脚本失败: %w", err)
		}
		os.Remove(fp)
	} else {
		if err := os.WriteFile(fp, []byte(fullContent), 0644); err != nil {
			return nil, fmt.Errorf("写入脚本失败: %w", err)
		}
	}

	return &ScriptDetail{
		Name:        newName,
		Description: info.Description,
		Params:      info.Params,
		Content:     info.Content,
		Created:     created,
		Updated:     now,
	}, nil
}

// DeleteScript 删除脚本。
func DeleteScript(category, name string) error {
	fp, err := scriptFilePath(category, name)
	if err != nil {
		return err
	}
	if _, err := os.Stat(fp); os.IsNotExist(err) {
		return fmt.Errorf("脚本 %s 不存在", name)
	}
	return os.Remove(fp)
}
