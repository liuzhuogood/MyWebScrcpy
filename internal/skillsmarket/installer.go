package skillsmarket

import (
	"context"
	"crypto/sha256"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"os"
	"path/filepath"
	"strings"
	"syscall"
	"time"
)

const (
	maxManifestSize  int64 = 4 << 10 // 4 KiB
	maxSkillFileSize int64 = 1 << 20 // 1 MiB
	maxDirEntries          = 32
	maxDirDepth            = 4
)

// Manager manages checking and installing skills into supported AI assistants.
type Manager struct {
	homeDir string
	sem     chan struct{}
}

// NewManager creates a new Manager for skills installation rooted at the explicit homeDir.
func NewManager(homeDir string) (*Manager, error) {
	if strings.TrimSpace(homeDir) == "" {
		return nil, errors.New("homeDir 不能为空")
	}
	absHome, err := filepath.Abs(homeDir)
	if err != nil {
		return nil, fmt.Errorf("解析 homeDir 失败: %w", err)
	}
	realHome, err := filepath.EvalSymlinks(absHome)
	if err != nil {
		return nil, fmt.Errorf("解析 homeDir 失败: %w", err)
	}
	info, err := os.Stat(realHome)
	if err != nil {
		return nil, fmt.Errorf("homeDir 不可用: %w", err)
	}
	if !info.IsDir() {
		return nil, errors.New("homeDir 必须是目录")
	}
	return &Manager{
		homeDir: realHome,
		sem:     make(chan struct{}, 1),
	}, nil
}

// HomeDir returns the configured base directory.
func (m *Manager) HomeDir() string {
	return m.homeDir
}

func (m *Manager) openHomeRoot() (*os.Root, error) {
	return os.OpenRoot(m.homeDir)
}

// checkNoSymlinkUnderRoot ensures no component of relPath under root is a symlink.
func checkNoSymlinkUnderRoot(root *os.Root, relPath string) error {
	clean := filepath.Clean(filepath.ToSlash(relPath))
	if strings.HasPrefix(clean, "..") || strings.HasPrefix(clean, "/") {
		return fmt.Errorf("路径超出根目录范围: %s", relPath)
	}
	parts := strings.Split(clean, "/")
	curr := ""
	for _, part := range parts {
		if part == "" || part == "." {
			continue
		}
		if curr == "" {
			curr = part
		} else {
			curr = curr + "/" + part
		}
		fi, err := root.Lstat(curr)
		if err != nil {
			if errors.Is(err, os.ErrNotExist) {
				return nil
			}
			return err
		}
		if fi.Mode()&os.ModeSymlink != 0 {
			return fmt.Errorf("路径中包含符号链接: %s", curr)
		}
	}
	return nil
}

// readRegularFileBounded reads a regular file with bounded size and non-blocking open.
func readRegularFileBounded(root *os.Root, relPath string, maxSize int64) ([]byte, error) {
	fi, err := root.Lstat(relPath)
	if err != nil {
		return nil, err
	}
	if !fi.Mode().IsRegular() {
		return nil, fmt.Errorf("非普通文件: %s", relPath)
	}
	if fi.Size() > maxSize {
		return nil, fmt.Errorf("文件大小超出限制 %d: %s (%d bytes)", maxSize, relPath, fi.Size())
	}

	// Open with O_NONBLOCK to prevent indefinite blocking on FIFO/pipes
	f, err := root.OpenFile(relPath, os.O_RDONLY|syscall.O_NONBLOCK, 0)
	if err != nil {
		return nil, err
	}
	defer f.Close()

	// Re-verify on open file descriptor (fstat) to prevent race
	finfo, err := f.Stat()
	if err != nil {
		return nil, err
	}
	if !finfo.Mode().IsRegular() {
		return nil, fmt.Errorf("非普通文件: %s", relPath)
	}
	if !os.SameFile(fi, finfo) {
		return nil, fmt.Errorf("读取时文件已被替换: %s", relPath)
	}
	if finfo.Size() > maxSize {
		return nil, fmt.Errorf("文件大小超出限制: %s", relPath)
	}

	data, err := io.ReadAll(io.LimitReader(f, maxSize+1))
	if err != nil {
		return nil, err
	}
	if int64(len(data)) > maxSize {
		return nil, fmt.Errorf("文件读取超出大小限制: %s", relPath)
	}
	return data, nil
}

// checkDirectoryOwnershipAndIntegrity verifies if existing dir is managed by this app and unmodified.
func checkDirectoryOwnershipAndIntegrity(ctx context.Context, root *os.Root, targetRel string, skillID SkillID) (installed bool, reason string, err error) {
	if err := ctx.Err(); err != nil {
		return false, "操作已取消", err
	}

	// 1. Check parent symlinks along targetRel
	if err := checkNoSymlinkUnderRoot(root, targetRel); err != nil {
		return false, err.Error(), nil
	}

	// 2. Check leaf directory
	fi, err := root.Lstat(targetRel)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return false, "", nil
		}
		return false, "", fmt.Errorf("检查目标目录状态失败: %w", err)
	}
	if fi.Mode()&os.ModeSymlink != 0 {
		return false, "目标目录为符号链接，已拒绝操作", nil
	}
	if !fi.IsDir() {
		return false, "目标路径已存在且不是目录，已保留原文件", nil
	}

	// 3. Open root for the target directory
	targetRoot, err := root.OpenRoot(targetRel)
	if err != nil {
		return false, fmt.Sprintf("无法安全打开目标目录: %v", err), nil
	}
	defer targetRoot.Close()

	// 4. Read .mywebscrcpy-manifest.json safely
	manifestBytes, err := readRegularFileBounded(targetRoot, ".mywebscrcpy-manifest.json", maxManifestSize)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return false, "目标目录已存在非本应用创建的文件，已保留原文件", nil
		}
		return false, fmt.Sprintf("清单文件无效或非普通文件: %v", err), nil
	}

	var manifest Manifest
	if err := json.Unmarshal(manifestBytes, &manifest); err != nil {
		return false, "清单损坏或格式无效，已保留原文件", nil
	}

	// 5. Read expected manifest from package
	_, expectedManifest, err := getSkillFiles(skillID)
	if err != nil {
		return false, "", fmt.Errorf("获取内置技能文件失败: %w", err)
	}

	if manifest.ManagedBy != "mywebscrcpy" || manifest.SkillID != string(skillID) {
		return false, "目标目录所有者或技能标识不匹配，已保留原文件", nil
	}

	// 6. Bounded traversal of target directory using iterative ReadDir
	foundFiles := make(map[string]bool)
	totalEntries := 0
	dirsToVisit := []string{""}

	for len(dirsToVisit) > 0 {
		if err := ctx.Err(); err != nil {
			return false, "操作已取消", err
		}

		currentDirRel := dirsToVisit[0]
		dirsToVisit = dirsToVisit[1:]

		dirName := "."
		if currentDirRel != "" {
			dirName = currentDirRel
		}

		dirFile, err := targetRoot.Open(dirName)
		if err != nil {
			return false, fmt.Sprintf("无法安全读取子目录 %s: %v", currentDirRel, err), nil
		}

		readDirErr := func() error {
			defer dirFile.Close()
			for {
				if err := ctx.Err(); err != nil {
					return err
				}

				entries, err := dirFile.ReadDir(16)
				if err != nil && !errors.Is(err, io.EOF) {
					return fmt.Errorf("读取目录条目失败 %s: %w", currentDirRel, err)
				}

				for _, entry := range entries {
					totalEntries++
					if totalEntries > maxDirEntries {
						return errors.New("目录条目数量超出上限")
					}

					entryName := entry.Name()
					var entryRel string
					if currentDirRel == "" {
						entryRel = entryName
					} else {
						entryRel = currentDirRel + "/" + entryName
					}

					cleanP := filepath.ToSlash(entryRel)
					depth := strings.Count(cleanP, "/") + 1
					if depth > maxDirDepth {
						return fmt.Errorf("目录深度超出限制: %s", cleanP)
					}

					info, err := targetRoot.Lstat(cleanP)
					if err != nil {
						return fmt.Errorf("检查条目状态失败 %s: %w", cleanP, err)
					}
					if info.Mode()&os.ModeSymlink != 0 {
						return fmt.Errorf("发现符号链接: %s", cleanP)
					}

					if entry.IsDir() {
						topDir := strings.Split(cleanP, "/")[0]
						if topDir != "scripts" && topDir != "rules" && topDir != "wiki" && topDir != "templates" {
							return fmt.Errorf("发现非预期子目录: %s", cleanP)
						}
						dirsToVisit = append(dirsToVisit, cleanP)
						continue
					}

					if !info.Mode().IsRegular() {
						return fmt.Errorf("发现非普通文件: %s", cleanP)
					}

					if cleanP == ".mywebscrcpy-manifest.json" {
						foundFiles[cleanP] = true
						continue
					}

					expectedHash, ok := manifest.Files[cleanP]
					if !ok {
						return fmt.Errorf("发现非清单文件 (可能已被用户修改/添加): %s", cleanP)
					}

					actualData, err := readRegularFileBounded(targetRoot, cleanP, maxSkillFileSize)
					if err != nil {
						return fmt.Errorf("读取文件失败 %s: %w", cleanP, err)
					}

					hash := sha256.Sum256(actualData)
					actualHash := fmt.Sprintf("%x", hash[:])
					if actualHash != expectedHash {
						return fmt.Errorf("文件内容已被用户修改: %s", cleanP)
					}

					foundFiles[cleanP] = true
				}

				if errors.Is(err, io.EOF) || len(entries) == 0 {
					break
				}
			}
			return nil
		}()

		if readDirErr != nil {
			if errors.Is(readDirErr, context.Canceled) || errors.Is(readDirErr, context.DeadlineExceeded) {
				return false, "操作已取消", readDirErr
			}
			return false, fmt.Sprintf("目标目录已被修改，已保护用户文件: %v", readDirErr), nil
		}
	}

	// 7. Verify all files declared in manifest are present
	for expectedRel := range manifest.Files {
		if !foundFiles[expectedRel] {
			return false, fmt.Sprintf("目标目录中清单文件缺失，已保护用户文件: %s", expectedRel), nil
		}
	}
	if !foundFiles[".mywebscrcpy-manifest.json"] {
		return false, "缺少清单文件: .mywebscrcpy-manifest.json", nil
	}

	// 8. 确认该安装完全未被用户修改。检查是否与当前最新内置版本完全一致：
	if manifest.Version == expectedManifest.Version && len(manifest.Files) == len(expectedManifest.Files) {
		identical := true
		for k, v := range expectedManifest.Files {
			if manifest.Files[k] != v {
				identical = false
				break
			}
		}
		if identical {
			return true, "", nil // 已安装且是最新版
		}
	}

	// 未被用户修改但版本/内容与最新内置不同的受管技能 -> 可安全升级
	return false, "upgradable", nil
}

// CheckSkillTargetStatus checks the installation status for a single target assistant.
func (m *Manager) CheckSkillTargetStatus(toolID AiToolID, skillID SkillID) TargetStatus {
	return m.CheckSkillTargetStatusContext(context.Background(), toolID, skillID)
}

// CheckSkillTargetStatusContext checks the installation status with context.
func (m *Manager) CheckSkillTargetStatusContext(ctx context.Context, toolID AiToolID, skillID SkillID) TargetStatus {
	targetRelDir, ok := GetTargetSkillsRelPath(toolID)
	if !ok {
		return TargetStatus{
			Installed: false,
			Status:    "error",
			Error:     fmt.Sprintf("未知的目标助手: %s", toolID),
		}
	}

	targetRel := filepath.Join(targetRelDir, string(skillID))
	absTargetDir := filepath.Join(m.homeDir, targetRel)

	root, err := m.openHomeRoot()
	if err != nil {
		return TargetStatus{
			Installed: false,
			Status:    "error",
			Path:      absTargetDir,
			Error:     fmt.Sprintf("打开用户根目录失败: %v", err),
		}
	}
	defer root.Close()

	installed, reason, err := checkDirectoryOwnershipAndIntegrity(ctx, root, targetRel, skillID)
	if err != nil {
		if errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) {
			return TargetStatus{
				Installed: false,
				Status:    "error",
				Path:      absTargetDir,
				Error:     "操作已取消",
			}
		}
		return TargetStatus{
			Installed: false,
			Status:    "error",
			Path:      absTargetDir,
			Error:     err.Error(),
		}
	}

	if !installed {
		if reason == "" {
			return TargetStatus{
				Installed: false,
				Status:    "not_installed",
				Path:      absTargetDir,
			}
		}
		if reason == "upgradable" {
			return TargetStatus{
				Installed: true,
				Status:    "upgradable",
				Path:      absTargetDir,
			}
		}
		return TargetStatus{
			Installed: false,
			Status:    "conflict",
			Path:      absTargetDir,
			Error:     reason,
		}
	}

	return TargetStatus{
		Installed: true,
		Status:    "installed",
		Path:      absTargetDir,
	}
}

// PopulateSkillStatuses fills in InstalledTargets and TargetStatuses for a skill.
func (m *Manager) PopulateSkillStatuses(ctx context.Context, skill *SkillDetail) {
	skill.TargetStatuses = make(map[AiToolID]TargetStatus)
	installed := make([]AiToolID, 0)
	for _, toolID := range SupportedTools {
		status := m.CheckSkillTargetStatusContext(ctx, toolID, skill.ID)
		skill.TargetStatuses[toolID] = status
		if status.Installed {
			installed = append(installed, toolID)
		}
	}
	skill.InstalledTargets = installed
}

// ListSkills returns all skills populated with their live filesystem installation statuses.
func (m *Manager) ListSkills() []SkillSummary {
	return m.ListSkillsContext(context.Background())
}

// ListSkillsContext returns all skills with context.
func (m *Manager) ListSkillsContext(ctx context.Context) []SkillSummary {
	all := AllSkills()
	summaries := make([]SkillSummary, len(all))
	for i := range all {
		m.PopulateSkillStatuses(ctx, &all[i])
		summaries[i] = all[i].SkillSummary
	}
	return summaries
}

// GetSkillDetail returns the full detail for a skill with live filesystem installation status.
func (m *Manager) GetSkillDetail(id string) (*SkillDetail, bool) {
	return m.GetSkillDetailContext(context.Background(), id)
}

// GetSkillDetailContext returns the full detail with context.
func (m *Manager) GetSkillDetailContext(ctx context.Context, id string) (*SkillDetail, bool) {
	skill, ok := GetSkill(id)
	if !ok {
		return nil, false
	}
	m.PopulateSkillStatuses(ctx, skill)
	return skill, true
}

// InstallSkill installs the skill to each specified target assistant atomically.
func (m *Manager) InstallSkill(ctx context.Context, skillID SkillID, targets []AiToolID) ([]SkillInstallTargetResult, error) {
	if !IsKnownSkillID(string(skillID)) {
		return nil, fmt.Errorf("未知的技能: %s", skillID)
	}

	if len(targets) == 0 {
		return nil, errors.New("目标列表不能为空")
	}

	// Validate target IDs and check duplicates
	seen := make(map[AiToolID]bool)
	for _, t := range targets {
		if _, ok := GetTargetSkillsRelPath(t); !ok {
			return nil, fmt.Errorf("未知的目标助手: %s", t)
		}
		if seen[t] {
			return nil, fmt.Errorf("重复的目标助手: %s", t)
		}
		seen[t] = true
	}

	// Load files once
	files, manifest, err := getSkillFiles(skillID)
	if err != nil {
		return nil, fmt.Errorf("获取内置技能文件失败: %w", err)
	}
	manifestData, err := json.MarshalIndent(manifest, "", "  ")
	if err != nil {
		return nil, fmt.Errorf("序列化清单失败: %w", err)
	}

	// Acquire semaphore with context support
	select {
	case <-ctx.Done():
		results := make([]SkillInstallTargetResult, len(targets))
		for i, toolID := range targets {
			targetRelSkillsDir, _ := GetTargetSkillsRelPath(toolID)
			results[i] = SkillInstallTargetResult{
				Target: toolID,
				Status: "failed",
				Error:  "操作已取消",
				Path:   filepath.Join(m.homeDir, targetRelSkillsDir, string(skillID)),
			}
		}
		return results, nil
	case m.sem <- struct{}{}:
		defer func() { <-m.sem }()
	}

	root, err := m.openHomeRoot()
	if err != nil {
		return nil, fmt.Errorf("打开用户根目录失败: %w", err)
	}
	defer root.Close()

	results := make([]SkillInstallTargetResult, len(targets))

	for i, toolID := range targets {
		targetRelSkillsDir, _ := GetTargetSkillsRelPath(toolID)
		targetRel := filepath.Join(targetRelSkillsDir, string(skillID))
		absTargetDir := filepath.Join(m.homeDir, targetRel)

		if err := ctx.Err(); err != nil {
			results[i] = SkillInstallTargetResult{
				Target: toolID,
				Status: "failed",
				Error:  "操作已取消",
				Path:   absTargetDir,
			}
			continue
		}

		// Check current status
		currentStatus := m.CheckSkillTargetStatusContext(ctx, toolID, skillID)
		if currentStatus.Installed && currentStatus.Status != "upgradable" {
			results[i] = SkillInstallTargetResult{
				Target: toolID,
				Status: "already_installed",
				Path:   absTargetDir,
			}
			continue
		}

		if currentStatus.Status == "conflict" || currentStatus.Status == "error" {
			log.Printf("[skillsmarket] 安装检查未通过: %s -> %s (%s)", skillID, absTargetDir, currentStatus.Error)
			results[i] = SkillInstallTargetResult{
				Target: toolID,
				Status: "failed",
				Path:   absTargetDir,
				Error:  currentStatus.Error,
			}
			continue
		}

		// Check parent symlinks before directory creation
		if err := checkNoSymlinkUnderRoot(root, targetRelSkillsDir); err != nil {
			results[i] = SkillInstallTargetResult{
				Target: toolID,
				Status: "failed",
				Path:   absTargetDir,
				Error:  fmt.Sprintf("父级目录安全检查失败: %v", err),
			}
			continue
		}

		if err := root.MkdirAll(targetRelSkillsDir, 0755); err != nil {
			results[i] = SkillInstallTargetResult{
				Target: toolID,
				Status: "failed",
				Path:   absTargetDir,
				Error:  fmt.Sprintf("创建助手技能根目录失败: %v", err),
			}
			continue
		}

		// Create atomic temporary staging directory inside targetRelSkillsDir
		stageName := fmt.Sprintf(".tmp-%s-%d", skillID, time.Now().UnixNano())
		stageRel := filepath.Join(targetRelSkillsDir, stageName)
		if err := root.Mkdir(stageRel, 0700); err != nil {
			results[i] = SkillInstallTargetResult{
				Target: toolID,
				Status: "failed",
				Path:   absTargetDir,
				Error:  fmt.Sprintf("创建临时暂存目录失败: %v", err),
			}
			continue
		}

		installErr := func() error {
			defer func() {
				// Clean up temporary staging directory if it still exists
				_ = root.RemoveAll(stageRel)
			}()

			// Write package files into stageRel
			for relPath, data := range files {
				if err := ctx.Err(); err != nil {
					return err
				}
				destRel := filepath.Join(stageRel, filepath.FromSlash(relPath))
				destDir := filepath.Dir(destRel)
				if err := root.MkdirAll(destDir, 0755); err != nil {
					return fmt.Errorf("创建子目录失败: %w", err)
				}
				mode := os.FileMode(0644)
				if relPath == "scripts/request_client.py" {
					mode = 0755
				}
				if err := root.WriteFile(destRel, data, mode); err != nil {
					return fmt.Errorf("写入文件失败 %s: %w", relPath, err)
				}
			}

			// Write manifest into stageRel
			manifestRel := filepath.Join(stageRel, ".mywebscrcpy-manifest.json")
			if err := root.WriteFile(manifestRel, manifestData, 0644); err != nil {
				return fmt.Errorf("写入清单失败: %w", err)
			}

			// Cancellation check right before committing
			if err := ctx.Err(); err != nil {
				return err
			}

			if currentStatus.Status == "upgradable" {
				backupName := fmt.Sprintf(".tmp-backup-%s-%d", skillID, time.Now().UnixNano())
				backupRel := filepath.Join(targetRelSkillsDir, backupName)
				if err := renameInDirNoReplace(root, targetRelSkillsDir, string(skillID), backupName); err != nil {
					return fmt.Errorf("备份旧版技能目录失败: %w", err)
				}
				defer func() {
					_ = root.RemoveAll(backupRel)
				}()
			} else {
				// Ensure target does not exist before rename (prevent overwrite)
				if fi, err := root.Lstat(targetRel); err == nil || !errors.Is(err, os.ErrNotExist) {
					if fi != nil {
						return fmt.Errorf("目标路径已存在，已保留原文件: %s", targetRel)
					}
					return fmt.Errorf("目标路径状态检查失败: %w", err)
				}
			}

			// Re-verify parent symlinks before rename
			if err := checkNoSymlinkUnderRoot(root, targetRelSkillsDir); err != nil {
				return fmt.Errorf("重命名前父级符号链接校验失败: %w", err)
			}

			// Atomic rename without replacement
			if err := renameInDirNoReplace(root, targetRelSkillsDir, stageName, string(skillID)); err != nil {
				return fmt.Errorf("重命名生效失败: %w", err)
			}
			return nil
		}()

		if installErr != nil {
			log.Printf("[skillsmarket] 安装写入失败: %s -> %s: %v", skillID, absTargetDir, installErr)
			results[i] = SkillInstallTargetResult{
				Target: toolID,
				Status: "failed",
				Path:   absTargetDir,
				Error:  installErr.Error(),
			}
			continue
		}

		// Verify on-disk presence
		verifyStatus := m.CheckSkillTargetStatusContext(ctx, toolID, skillID)
		if !verifyStatus.Installed {
			log.Printf("[skillsmarket] 安装后校验未通过: %s -> %s: %s", skillID, absTargetDir, verifyStatus.Error)
			results[i] = SkillInstallTargetResult{
				Target: toolID,
				Status: "failed",
				Path:   absTargetDir,
				Error:  fmt.Sprintf("安装后校验未通过: %s", verifyStatus.Error),
			}
			continue
		}

		log.Printf("[skillsmarket] 成功安装技能: %s -> %s", skillID, absTargetDir)
		results[i] = SkillInstallTargetResult{
			Target: toolID,
			Status: "installed",
			Path:   absTargetDir,
		}
	}

	return results, nil
}
