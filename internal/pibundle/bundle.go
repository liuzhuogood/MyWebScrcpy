// Package pibundle installs the embedded standalone Pi distribution next to
// the application. It never downloads dependencies or searches the host PATH.
package pibundle

import (
	"archive/zip"
	"bytes"
	"crypto/sha256"
	"embed"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"log"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"time"
)

//go:embed payload/*
var payload embed.FS

var ErrUnprepared = errors.New("未内置 Pi 运行环境，请使用 scripts/build.sh 构建")

type Paths struct {
	Executable string
	ToolsDir   string
	SkillsDir  string
}

type fileRecord struct {
	SHA256 string      `json:"sha256"`
	Mode   fs.FileMode `json:"mode"`
}

type manifest struct {
	Version string                `json:"version"`
	GOOS    string                `json:"goos"`
	GOARCH  string                `json:"goarch"`
	SHA256  string                `json:"sha256"`
	Files   map[string]fileRecord `json:"files"`
}

// Prepare uses an absolute application directory, independent of launch cwd.
// Skills are copied only when absent, preserving edits and additional skills.
func Prepare(appDir string, skills fs.FS) (Paths, error) {
	return prepare(appDir, skills, payload)
}

func prepare(appDir string, skills, source fs.FS) (Paths, error) {
	data, err := fs.ReadFile(source, "payload/manifest.json")
	if errors.Is(err, fs.ErrNotExist) {
		return Paths{}, ErrUnprepared
	}
	if err != nil {
		return Paths{}, err
	}
	var config manifest
	if err := json.Unmarshal(data, &config); err != nil {
		return Paths{}, fmt.Errorf("读取内置 Pi 清单失败: %w", err)
	}
	if config.GOOS != runtime.GOOS || config.GOARCH != runtime.GOARCH {
		return Paths{}, fmt.Errorf("内置 Pi 平台 %s/%s 与程序 %s/%s 不匹配", config.GOOS, config.GOARCH, runtime.GOOS, runtime.GOARCH)
	}
	if raw, err := hex.DecodeString(config.SHA256); err != nil || len(raw) != sha256.Size || len(config.Files) == 0 {
		return Paths{}, errors.New("内置 Pi 清单缺少有效校验信息")
	}
	for name, record := range config.Files {
		if !validPath(name) || record.Mode != 0644 && record.Mode != 0755 {
			return Paths{}, fmt.Errorf("无效 Pi 文件记录: %q", name)
		}
	}
	appDir, err = filepath.Abs(appDir)
	if err != nil {
		return Paths{}, err
	}
	root := filepath.Join(appDir, ".pi")
	// Content-addressed runtime directories also separate platform upgrades.
	runtimeDir := filepath.Join(root, "runtime", config.SHA256)
	if err := installRuntime(runtimeDir, config, source); err != nil {
		return Paths{}, fmt.Errorf("释放内置 Pi 失败: %w", err)
	}
	skillsDir := filepath.Join(root, "skills")
	if err := installSkills(skillsDir, skills); err != nil {
		return Paths{}, fmt.Errorf("释放内置技能失败: %w", err)
	}
	name := "pi"
	if runtime.GOOS == "windows" {
		name += ".exe"
	}
	paths := Paths{Executable: filepath.Join(runtimeDir, name), ToolsDir: filepath.Join(runtimeDir, "bin"), SkillsDir: skillsDir}
	for _, name := range []string{name, "bin/fd" + executableSuffix(), "bin/rg" + executableSuffix(), "package.json"} {
		if _, exists := config.Files[name]; !exists {
			return Paths{}, fmt.Errorf("内置 Pi 缺少必需文件: %s", name)
		}
	}
	log.Printf("[PiBundle] Pi=%s platform=%s/%s executable=%q skills_dir=%q", config.Version, config.GOOS, config.GOARCH, paths.Executable, paths.SkillsDir)
	return paths, nil
}

func executableSuffix() string {
	if runtime.GOOS == "windows" {
		return ".exe"
	}
	return ""
}

func validPath(name string) bool {
	return fs.ValidPath(name) && !strings.ContainsAny(name, "\\:") && name != "."
}

func verifyRuntime(dir string, config manifest) error {
	for name, record := range config.Files {
		path := filepath.Join(dir, filepath.FromSlash(name))
		info, err := os.Lstat(path)
		if err != nil {
			return err
		}
		if !info.Mode().IsRegular() {
			return fmt.Errorf("Pi 文件不是普通文件: %s", path)
		}
		if runtime.GOOS != "windows" && info.Mode().Perm() != record.Mode {
			return fmt.Errorf("Pi 文件权限不正确: %s", path)
		}
		file, err := os.Open(path)
		if err != nil {
			return err
		}
		hash := sha256.New()
		_, copyErr := io.Copy(hash, file)
		closeErr := file.Close()
		if copyErr != nil {
			return copyErr
		}
		if closeErr != nil {
			return closeErr
		}
		if hex.EncodeToString(hash.Sum(nil)) != record.SHA256 {
			return fmt.Errorf("Pi 文件校验失败: %s", path)
		}
	}
	return nil
}

func installRuntime(dir string, config manifest, source fs.FS) error {
	if err := verifyRuntime(dir, config); err == nil {
		return nil
	}
	archive, err := fs.ReadFile(source, "payload/runtime.zip")
	if err != nil {
		return err
	}
	if fmt.Sprintf("%x", sha256.Sum256(archive)) != config.SHA256 {
		return errors.New("内置 Pi 压缩包校验失败")
	}
	if err := os.MkdirAll(filepath.Dir(dir), 0755); err != nil {
		return err
	}
	temporary, err := os.MkdirTemp(filepath.Dir(dir), ".extract-*")
	if err != nil {
		return err
	}
	defer os.RemoveAll(temporary)
	reader, err := zip.NewReader(bytes.NewReader(archive), int64(len(archive)))
	if err != nil {
		return err
	}
	seen := make(map[string]bool)
	for _, entry := range reader.File {
		record, exists := config.Files[entry.Name]
		if !exists || !validPath(entry.Name) || seen[entry.Name] || !entry.Mode().IsRegular() {
			return fmt.Errorf("无效 Pi 压缩包文件: %s", entry.Name)
		}
		seen[entry.Name] = true
		path := filepath.Join(temporary, filepath.FromSlash(entry.Name))
		if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
			return err
		}
		if err := extractFile(path, entry, record.Mode); err != nil {
			return err
		}
	}
	if err := verifyRuntime(temporary, config); err != nil {
		return err
	}
	// Another process may have completed installation while we extracted.
	if err := verifyRuntime(dir, config); err == nil {
		return nil
	}
	if _, err := os.Lstat(dir); err == nil {
		backup := dir + ".invalid-" + fmt.Sprint(time.Now().UnixNano())
		if err := os.Rename(dir, backup); err != nil {
			return err
		}
		log.Printf("[PiBundle] 损坏的运行环境已移至 %q", backup)
	}
	if err := os.Rename(temporary, dir); err != nil {
		if checkErr := verifyRuntime(dir, config); checkErr == nil {
			return nil
		}
		return err
	}
	log.Printf("[PiBundle] 内置运行环境已释放 dir=%q", dir)
	return nil
}

func extractFile(path string, entry *zip.File, mode fs.FileMode) error {
	source, err := entry.Open()
	if err != nil {
		return err
	}
	defer source.Close()
	file, err := os.OpenFile(path, os.O_WRONLY|os.O_CREATE|os.O_EXCL, mode)
	if err != nil {
		return err
	}
	// Restore the manifest mode explicitly; a restrictive host umask must not
	// strip execute permissions from the bundled binaries.
	if err := file.Chmod(mode); err != nil {
		_ = file.Close()
		return err
	}
	_, copyErr := io.Copy(file, source)
	closeErr := file.Close()
	if copyErr != nil {
		return copyErr
	}
	return closeErr
}

func installSkills(dir string, skills fs.FS) error {
	if err := os.MkdirAll(dir, 0755); err != nil {
		return err
	}
	return fs.WalkDir(skills, ".", func(name string, entry fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if name == "." {
			return nil
		}
		if !validPath(name) || entry.Type()&fs.ModeSymlink != 0 {
			return fmt.Errorf("无效技能路径: %s", name)
		}
		path := filepath.Join(dir, filepath.FromSlash(name))
		if entry.IsDir() {
			return os.MkdirAll(path, 0755)
		}
		file, err := os.OpenFile(path, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0644)
		if errors.Is(err, os.ErrExist) {
			return nil
		}
		if err != nil {
			return err
		}
		data, readErr := fs.ReadFile(skills, name)
		if readErr == nil {
			_, readErr = file.Write(data)
		}
		closeErr := file.Close()
		if readErr != nil {
			_ = os.Remove(path)
			return readErr
		}
		return closeErr
	})
}
