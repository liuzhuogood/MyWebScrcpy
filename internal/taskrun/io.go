package taskrun

import (
	"bufio"
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"syscall"
)

func validateSafeIdentifier(id string) error {
	if id == "" || strings.Contains(id, "/") || strings.Contains(id, "\\") || strings.Contains(id, "..") {
		return ErrInvalidTaskID
	}
	return nil
}

func isSubPath(base, target string) bool {
	rel, err := filepath.Rel(base, target)
	if err != nil {
		return false
	}
	return rel == "." || (!strings.HasPrefix(rel, "..") && !filepath.IsAbs(rel))
}

// safeResolvePath 确保相对路径且符号链接没有逃逸出 taskDir
func safeResolvePath(taskDir, relPath string) (string, error) {
	if filepath.IsAbs(relPath) {
		return "", ErrPathTraversal
	}
	clean := filepath.Clean(relPath)
	if strings.HasPrefix(clean, "..") || strings.HasPrefix(clean, "/") || strings.HasPrefix(clean, "\\") {
		return "", ErrPathTraversal
	}

	target := filepath.Join(taskDir, clean)

	evalTaskDir, err := filepath.EvalSymlinks(taskDir)
	if err != nil {
		evalTaskDir = taskDir
	}

	// 符号链接解析检查
	evaled, err := filepath.EvalSymlinks(target)
	if err != nil {
		if os.IsNotExist(err) {
			// 文件尚不存在时，检查父级目录是否在 taskDir 内
			parentEvaled, pErr := filepath.EvalSymlinks(filepath.Dir(target))
			if pErr == nil {
				if !isSubPath(evalTaskDir, parentEvaled) {
					return "", ErrPathTraversal
				}
			}
			return target, nil
		}
		return "", err
	}

	if !isSubPath(evalTaskDir, evaled) {
		return "", ErrPathTraversal
	}
	return evaled, nil
}

func isValidImageHeader(data []byte, format string) bool {
	if len(data) < 3 {
		return false
	}
	switch strings.ToLower(format) {
	case "image/png", ".png", "png":
		return len(data) >= 8 && bytes.HasPrefix(data, []byte("\x89PNG\r\n\x1a\n"))
	case "image/jpeg", "image/jpg", ".jpg", ".jpeg", "jpg", "jpeg":
		return len(data) >= 3 && data[0] == 0xFF && data[1] == 0xD8 && data[2] == 0xFF
	case "image/webp", ".webp", "webp":
		return len(data) >= 12 && string(data[0:4]) == "RIFF" && string(data[8:12]) == "WEBP"
	default:
		// 通用图片格式魔数检测
		if len(data) >= 8 && bytes.HasPrefix(data, []byte("\x89PNG\r\n\x1a\n")) {
			return true
		}
		if len(data) >= 3 && data[0] == 0xFF && data[1] == 0xD8 && data[2] == 0xFF {
			return true
		}
		if len(data) >= 12 && string(data[0:4]) == "RIFF" && string(data[8:12]) == "WEBP" {
			return true
		}
		return false
	}
}

func readJSONFile(path string, target any) error {
	fi, err := os.Lstat(path)
	if err != nil {
		return err
	}
	if !fi.Mode().IsRegular() {
		return fmt.Errorf("非普通文件 (mode=%v)", fi.Mode())
	}
	realFi, err := os.Stat(path)
	if err != nil {
		return err
	}
	if !realFi.Mode().IsRegular() {
		return fmt.Errorf("目标不是普通文件 (mode=%v)", realFi.Mode())
	}

	f, err := os.OpenFile(path, os.O_RDONLY|syscall.O_NONBLOCK, 0)
	if err != nil {
		return err
	}
	defer f.Close()

	limited := io.LimitReader(f, 2*1024*1024)
	data, err := io.ReadAll(limited)
	if err != nil {
		return err
	}
	return json.Unmarshal(data, target)
}

func readTruncatedText(path string, maxBytes int64) (string, error) {
	f, err := os.Open(path)
	if err != nil {
		return "", err
	}
	defer f.Close()

	buf := make([]byte, maxBytes)
	n, err := f.Read(buf)
	if err != nil && err != io.EOF {
		return "", err
	}
	return string(buf[:n]), nil
}

func readLogTail(path string, maxBytes int64) (string, error) {
	info, err := os.Stat(path)
	if err != nil {
		return "", err
	}
	size := info.Size()
	if size == 0 {
		return "", nil
	}

	f, err := os.Open(path)
	if err != nil {
		return "", err
	}
	defer f.Close()

	offset := int64(0)
	if size > maxBytes {
		offset = size - maxBytes
	}
	if _, err := f.Seek(offset, io.SeekStart); err != nil {
		return "", err
	}

	data, err := io.ReadAll(f)
	if err != nil {
		return "", err
	}
	return string(data), nil
}

func readRecentActions(path string, maxLines int) ([]ActionRecord, error) {
	fi, err := os.Stat(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, err
	}
	if !fi.Mode().IsRegular() {
		return nil, errors.New("actions.jsonl 不是普通文件")
	}

	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()

	limited := io.LimitReader(f, 2*1024*1024)
	var lines []string
	scanner := bufio.NewScanner(limited)
	scanner.Buffer(make([]byte, 64*1024), 64*1024)
	for scanner.Scan() {
		text := strings.TrimSpace(scanner.Text())
		if text != "" {
			lines = append(lines, text)
		}
	}
	if err := scanner.Err(); err != nil && err != io.EOF {
		return nil, err
	}

	start := 0
	if len(lines) > maxLines {
		start = len(lines) - maxLines
	}

	var actions []ActionRecord
	for i := len(lines) - 1; i >= start; i-- { // 最近的在前
		var act ActionRecord
		if err := json.Unmarshal([]byte(lines[i]), &act); err == nil {
			actions = append(actions, act)
		}
	}
	return actions, nil
}
