package taskrun

import (
	"context"
	"errors"
	"fmt"
	"io"
	"log"
	"math"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"

	"mywebscrcpy/internal/db"
)

var (
	ErrScanInProgress = errors.New("任务目录扫描正在进行中，请勿重复提交")
	ErrInvalidTaskID  = errors.New("任务标识不合法")
	ErrTaskNotFound   = errors.New("指定任务不存在")
	ErrOutputNotFound = errors.New("未声明或未找到指定产出")
	ErrPathTraversal  = errors.New("非法路径访问：禁止越界或符号链接逃逸")
	ErrUnsafeMedia    = errors.New("非法媒体格式：拒绝非安全媒体直出")
)

const (
	ConfigKeyTaskRootDir = "task_root_dir"
	DefaultRelativeDir   = ".mywebscrcppy/tasks"
	StaleThresholdMs     = 24 * 60 * 60 * 1000 // 24小时未更新判定为陈旧
	MaxReportSizeBytes   = 512 * 1024          // 报告预览最大 512KB
	MaxLogTailBytes      = 128 * 1024          // 日志尾部预览最大 128KB
	MaxActionLines       = 100                 // 操作记录最多预览行数
)

type Service struct {
	database      *db.DB
	mu            sync.RWMutex
	scanning      bool
	lastScannedAt int64
	cachedTasks   []TaskSummary
	cachedErrors  []string
}

func NewService(database *db.DB) *Service {
	return &Service{
		database: database,
	}
}

// ResolveDefaultRootDir 解析默认任务根目录 ~/.mywebscrcppy/tasks/
func ResolveDefaultRootDir() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("获取系统用户主目录失败: %w", err)
	}
	return filepath.Clean(filepath.Join(home, DefaultRelativeDir)), nil
}

// ValidateRootDir 仅校验任务根目录路径合法性与写权限，不修改当前配置
func (s *Service) ValidateRootDir(dir string) error {
	resolved, err := s.normalizeDir(dir)
	if err != nil {
		return err
	}
	return s.testWriteDir(resolved)
}

// GetRootDir 获取当前生效的任务根目录绝对路径（只读规范化，不写盘）
func (s *Service) GetRootDir() (string, error) {
	if s.database != nil {
		cfg, err := s.database.GetConfig(ConfigKeyTaskRootDir)
		if err == nil && strings.TrimSpace(cfg) != "" {
			return s.normalizeDir(strings.TrimSpace(cfg))
		}
	}
	def, err := ResolveDefaultRootDir()
	if err != nil {
		return "", err
	}
	return s.normalizeDir(def)
}

// SetRootDir 设置新的根目录并清空旧内存索引（校验写权限）
func (s *Service) SetRootDir(dir string) (string, error) {
	resolved, err := s.normalizeDir(dir)
	if err != nil {
		return "", err
	}
	if err := s.testWriteDir(resolved); err != nil {
		return "", err
	}
	if s.database != nil {
		if err := s.database.SetConfig(ConfigKeyTaskRootDir, resolved); err != nil {
			return "", fmt.Errorf("保存任务根目录配置失败: %w", err)
		}
	}

	s.mu.Lock()
	s.cachedTasks = nil
	s.cachedErrors = nil
	s.lastScannedAt = 0
	s.mu.Unlock()

	log.Printf("[TaskRun] 已更新任务根目录为 %s，旧索引已清空", resolved)
	return resolved, nil
}

func (s *Service) normalizeDir(pathStr string) (string, error) {
	if strings.HasPrefix(pathStr, "~") {
		home, err := os.UserHomeDir()
		if err != nil {
			return "", fmt.Errorf("无法展开 ~ 路径: %w", err)
		}
		pathStr = filepath.Join(home, strings.TrimPrefix(pathStr, "~"))
	}
	abs, err := filepath.Abs(pathStr)
	if err != nil {
		return "", fmt.Errorf("解析目录绝对路径失败: %w", err)
	}
	abs = filepath.Clean(abs)

	// 确保目录存在
	if err := os.MkdirAll(abs, 0755); err != nil {
		return "", fmt.Errorf("任务根目录无法创建或访问 (%s): %w", abs, err)
	}
	return abs, nil
}

func (s *Service) testWriteDir(abs string) error {
	testFile := filepath.Join(abs, fmt.Sprintf(".write_test_%d", time.Now().UnixNano()))
	if err := os.WriteFile(testFile, []byte("ok"), 0644); err != nil {
		return fmt.Errorf("任务根目录不可写 (%s): %w", abs, err)
	}
	_ = os.Remove(testFile)
	return nil
}

// Scan 执行用户主动扫描，重建内存索引
func (s *Service) Scan(ctx context.Context) (*ScanResult, error) {
	s.mu.Lock()
	if s.scanning {
		s.mu.Unlock()
		return nil, ErrScanInProgress
	}
	s.scanning = true
	s.mu.Unlock()

	defer func() {
		s.mu.Lock()
		s.scanning = false
		s.mu.Unlock()
	}()

	rootDir, err := s.GetRootDir()
	if err != nil {
		return nil, err
	}

	evalRootDir, err := filepath.EvalSymlinks(rootDir)
	if err != nil {
		evalRootDir = rootDir
	}

	entries, err := os.ReadDir(rootDir)
	if err != nil {
		return nil, fmt.Errorf("读取任务根目录失败 (%s): %w", rootDir, err)
	}

	var tasks []TaskSummary
	var scanErrors []string
	now := time.Now().UnixMilli()

	for _, entry := range entries {
		if err := ctx.Err(); err != nil {
			return nil, err
		}

		if !entry.IsDir() {
			continue
		}
		taskID := entry.Name()
		if strings.HasPrefix(taskID, ".") {
			continue
		}

		taskDir := filepath.Join(rootDir, taskID)
		evalTaskDir, err := filepath.EvalSymlinks(taskDir)
		if err != nil || !isSubPath(evalRootDir, evalTaskDir) {
			scanErrors = append(scanErrors, fmt.Sprintf("任务 %s 目录逃逸或无法访问: %v", taskID, err))
			continue
		}

		summary, scanErr := s.inspectTaskDir(taskDir, taskID, now)
		if scanErr != nil {
			scanErrors = append(scanErrors, fmt.Sprintf("任务 %s 读取异常: %v", taskID, scanErr))
			summary = TaskSummary{
				ID:           taskID,
				Name:         taskID,
				Status:       "unknown",
				Corrupted:    true,
				ErrorMessage: scanErr.Error(),
				UpdatedAt:    now,
			}
		}
		tasks = append(tasks, summary)
	}

	// 按 UpdatedAt 倒序排列
	sort.Slice(tasks, func(i, j int) bool {
		return tasks[i].UpdatedAt > tasks[j].UpdatedAt
	})

	s.mu.Lock()
	nowActiveRoot, _ := s.GetRootDir()
	if nowActiveRoot != rootDir {
		s.mu.Unlock()
		return nil, errors.New("任务根目录在扫描期间已切换，旧目录扫描结果已丢弃")
	}
	if err := ctx.Err(); err != nil {
		s.mu.Unlock()
		return nil, err
	}
	s.lastScannedAt = now
	s.cachedTasks = tasks
	s.cachedErrors = scanErrors
	s.mu.Unlock()

	log.Printf("[TaskRun] 主动扫描完成：目录 %s，发现 %d 个任务，%d 项异常", rootDir, len(tasks), len(scanErrors))

	return &ScanResult{
		ScannedAt: now,
		Count:     len(tasks),
		RootDir:   rootDir,
		Errors:    scanErrors,
	}, nil
}

// ListTasks 查询最近扫描的内存索引
func (s *Service) ListTasks(deviceSerial, status, source string, page, limit int) ([]TaskSummary, int, int64, string) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	rootDir, _ := s.GetRootDir()
	if s.lastScannedAt == 0 || len(s.cachedTasks) == 0 {
		return []TaskSummary{}, 0, s.lastScannedAt, rootDir
	}

	var filtered []TaskSummary
	for _, t := range s.cachedTasks {
		if deviceSerial != "" && t.DeviceSerial != deviceSerial {
			continue
		}
		if status != "" && t.Status != status {
			continue
		}
		if source != "" && t.Source != source {
			continue
		}
		filtered = append(filtered, t)
	}

	total := len(filtered)
	if limit <= 0 {
		limit = 20
	}
	if limit > 100 {
		limit = 100
	}
	if page <= 0 {
		page = 1
	}

	start := (page - 1) * limit
	if start >= total {
		return []TaskSummary{}, total, s.lastScannedAt, rootDir
	}
	end := start + limit
	if end > total {
		end = total
	}

	return filtered[start:end], total, s.lastScannedAt, rootDir
}

// GetTaskDetail 直接读取指定任务目录，不扫描根目录
func (s *Service) GetTaskDetail(id string) (*TaskDetail, error) {
	if err := validateSafeIdentifier(id); err != nil {
		return nil, ErrInvalidTaskID
	}

	rootDir, err := s.GetRootDir()
	if err != nil {
		return nil, err
	}

	taskDir := filepath.Join(rootDir, id)
	info, err := os.Stat(taskDir)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, ErrTaskNotFound
		}
		return nil, err
	}
	if !info.IsDir() {
		return nil, ErrTaskNotFound
	}

	evalRootDir, err := filepath.EvalSymlinks(rootDir)
	if err != nil {
		evalRootDir = rootDir
	}
	evalTaskDir, err := filepath.EvalSymlinks(taskDir)
	if err != nil {
		return nil, ErrTaskNotFound
	}
	if !isSubPath(evalRootDir, evalTaskDir) {
		return nil, ErrPathTraversal
	}

	now := time.Now().UnixMilli()
	return s.loadTaskDetail(taskDir, id, now)
}

var (
	allowedTaskStatuses = map[string]bool{
		"pending":   true,
		"running":   true,
		"paused":    true,
		"completed": true,
		"failed":    true,
		"cancelled": true,
		"success":   true,
	}

	allowedOutputCategories = map[string]bool{
		"report":     true,
		"log":        true,
		"screenshot": true,
		"video":      true,
		"data":       true,
		"file":       true,
		"directory":  true,
	}
)

func validateTaskDef(def *TaskDef) error {
	if strings.TrimSpace(def.Name) == "" {
		def.Name = "unnamed"
	}
	if strings.TrimSpace(def.DeviceSerial) == "" {
		return errors.New("设备序列号 (device_serial) 不能为空")
	}

	seenIDs := make(map[string]bool)
	for i, out := range def.Outputs {
		outID := strings.TrimSpace(out.ID)
		if outID == "" {
			return fmt.Errorf("第 %d 项产出缺少 id", i+1)
		}
		if seenIDs[outID] {
			return fmt.Errorf("产出标识 %q 重复定义", outID)
		}
		seenIDs[outID] = true

		cat := strings.TrimSpace(out.Category)
		if !allowedOutputCategories[cat] {
			return fmt.Errorf("产出 %s 具有不支持的 category: %q", outID, out.Category)
		}
	}
	return nil
}

func validateTaskState(state *TaskState) error {
	st := strings.TrimSpace(state.Status)
	if !allowedTaskStatuses[st] {
		return fmt.Errorf("未知的任务状态: %q", state.Status)
	}
	if math.IsNaN(state.Progress) || math.IsInf(state.Progress, 0) {
		state.Progress = 0
	} else if state.Progress < 0 {
		state.Progress = 0
	} else if state.Progress > 1.0 && state.Progress <= 100.0 {
		state.Progress = state.Progress / 100.0
	} else if state.Progress > 1.0 {
		state.Progress = 1.0
	}
	return nil
}

func (s *Service) inspectTaskDir(taskDir, taskID string, now int64) (TaskSummary, error) {
	taskDefPath := filepath.Join(taskDir, "task.json")
	statePath := filepath.Join(taskDir, "state.json")

	var taskDef TaskDef
	if err := readJSONFile(taskDefPath, &taskDef); err != nil {
		return TaskSummary{}, fmt.Errorf("读取 task.json 失败: %w", err)
	}
	if strings.TrimSpace(taskDef.Name) == "" {
		taskDef.Name = taskID
	}
	if err := validateTaskDef(&taskDef); err != nil {
		return TaskSummary{
			ID:           taskID,
			Name:         taskDef.Name,
			DeviceSerial: taskDef.DeviceSerial,
			Source:       taskDef.Source,
			Status:       "unknown",
			Goal:         taskDef.Goal,
			Corrupted:    true,
			ErrorMessage: err.Error(),
			UpdatedAt:    now,
		}, nil
	}

	var taskState TaskState
	if err := readJSONFile(statePath, &taskState); err != nil {
		// state.json 缺失或损坏
		return TaskSummary{
			ID:           taskID,
			Name:         taskDef.Name,
			DeviceSerial: taskDef.DeviceSerial,
			Source:       taskDef.Source,
			Status:       "unknown",
			Goal:         taskDef.Goal,
			Corrupted:    true,
			ErrorMessage: fmt.Sprintf("读取 state.json 失败: %v", err),
			UpdatedAt:    now,
		}, nil
	}
	if err := validateTaskState(&taskState); err != nil {
		return TaskSummary{
			ID:           taskID,
			Name:         taskDef.Name,
			DeviceSerial: taskDef.DeviceSerial,
			Source:       taskDef.Source,
			Status:       "unknown",
			Goal:         taskDef.Goal,
			Corrupted:    true,
			ErrorMessage: err.Error(),
			UpdatedAt:    now,
		}, nil
	}

	// 检查必需产出完整性
	complete := true
	for _, out := range taskDef.Outputs {
		if !out.Required {
			continue
		}
		check := s.checkSingleOutput(taskDir, out)
		if !check.Valid {
			complete = false
			break
		}
	}

	isStale := false
	if taskState.UpdatedAt > 0 && (now-taskState.UpdatedAt) > StaleThresholdMs {
		isStale = true
	}

	summary := TaskSummary{
		ID:              taskID,
		Name:            taskDef.Name,
		DeviceSerial:    taskDef.DeviceSerial,
		Source:          taskDef.Source,
		Status:          taskState.Status,
		BusinessResult:  taskState.LastResult,
		OutputsComplete: complete,
		Goal:            taskDef.Goal,
		UpdatedAt:       taskState.UpdatedAt,
		IsStale:         isStale,
	}
	return summary, nil
}

func (s *Service) loadTaskDetail(taskDir, taskID string, now int64) (*TaskDetail, error) {
	taskDefPath := filepath.Join(taskDir, "task.json")
	statePath := filepath.Join(taskDir, "state.json")

	detail := &TaskDetail{ID: taskID, Outputs: []OutputCheckResult{}}

	if err := readJSONFile(taskDefPath, &detail.TaskDef); err != nil {
		detail.Corrupted = true
		detail.ErrorMessage = fmt.Sprintf("读取 task.json 失败: %v", err)
		return detail, nil
	}
	if strings.TrimSpace(detail.TaskDef.Name) == "" {
		detail.TaskDef.Name = taskID
	}
	if err := validateTaskDef(&detail.TaskDef); err != nil {
		detail.Corrupted = true
		detail.ErrorMessage = err.Error()
		return detail, nil
	}

	if err := readJSONFile(statePath, &detail.State); err != nil {
		detail.Corrupted = true
		detail.ErrorMessage = fmt.Sprintf("读取 state.json 失败: %v", err)
	} else if err := validateTaskState(&detail.State); err != nil {
		detail.Corrupted = true
		detail.State.Status = "unknown"
		detail.ErrorMessage = err.Error()
	}

	// 逐项检查声明产出
	allComplete := true
	var outputResults []OutputCheckResult
	for _, out := range detail.TaskDef.Outputs {
		res := s.checkSingleOutput(taskDir, out)
		outputResults = append(outputResults, res)
		if out.Required && !res.Valid {
			allComplete = false
		}
	}
	detail.Outputs = outputResults
	detail.OutputsComplete = allComplete

	// 检查陈旧
	if detail.State.UpdatedAt > 0 && (now-detail.State.UpdatedAt) > StaleThresholdMs {
		detail.IsStale = true
	}

	// 报告预览内容安全读取
	for _, out := range detail.TaskDef.Outputs {
		if out.Category == "report" {
			absPath, err := safeResolvePath(taskDir, out.Path)
			if err == nil {
				if content, err := readTruncatedText(absPath, MaxReportSizeBytes); err == nil {
					detail.ReportContent = content
				}
			}
			break
		}
	}

	// 操作记录 actions.jsonl 安全读取
	actionsPath := filepath.Join(taskDir, "actions.jsonl")
	if actions, err := readRecentActions(actionsPath, MaxActionLines); err == nil {
		detail.RecentActions = actions
	}

	// 日志尾部安全预览
	for _, out := range detail.TaskDef.Outputs {
		if out.Category == "log" {
			absPath, err := safeResolvePath(taskDir, out.Path)
			if err == nil {
				if tail, err := readLogTail(absPath, MaxLogTailBytes); err == nil {
					detail.LogTail = tail
				}
			}
			break
		}
	}

	return detail, nil
}


func (s *Service) checkSingleOutput(taskDir string, out OutputDef) OutputCheckResult {
	res := OutputCheckResult{
		ID:       out.ID,
		Category: out.Category,
		Format:   out.Format,
		Path:     out.Path,
		Required: out.Required,
	}

	absPath, err := safeResolvePath(taskDir, out.Path)
	if err != nil {
		res.Error = err.Error()
		return res
	}

	info, err := os.Stat(absPath)
	if err != nil {
		if os.IsNotExist(err) {
			res.Exists = false
			res.Valid = false
			return res
		}
		res.Error = err.Error()
		return res
	}

	res.Exists = true
	if info.IsDir() {
		// 目录产出：列出非空且有效的文件
		entries, err := os.ReadDir(absPath)
		if err != nil {
			res.Error = err.Error()
			return res
		}
		var files []string
		for _, e := range entries {
			if e.IsDir() {
				continue
			}
			name := e.Name()
			if strings.HasPrefix(name, ".") {
				continue
			}
			subPath := filepath.Join(absPath, name)
			subInfo, err := os.Stat(subPath)
			if err != nil || subInfo.Size() == 0 {
				continue
			}
			if len(out.AllowedTypes) > 0 {
				ext := strings.ToLower(filepath.Ext(name))
				allowed := false
				for _, t := range out.AllowedTypes {
					if strings.ToLower(t) == ext {
						allowed = true
						break
					}
				}
				if !allowed {
					continue
				}
			}
			if out.Category == "screenshot" {
				buf := make([]byte, 16)
				f, err := os.Open(subPath)
				if err != nil {
					continue
				}
				n, _ := io.ReadFull(f, buf)
				_ = f.Close()
				if !isValidImageHeader(buf[:n], out.Format) {
					continue
				}
			}
			files = append(files, name)
		}
		res.Files = files
		res.Valid = len(files) > 0
		return res
	}

	// 单文件产出
	res.SizeBytes = info.Size()
	if out.Category == "log" {
		// 日志允许为空文件，只要文件存在
		res.Valid = true
	} else {
		// 报告、视频、截图等非日志产出不能以空文件满足要求
		if info.Size() == 0 {
			res.Valid = false
			return res
		}
		if out.Category == "screenshot" {
			buf := make([]byte, 16)
			f, err := os.Open(absPath)
			if err != nil {
				res.Valid = false
				res.Error = err.Error()
				return res
			}
			n, _ := io.ReadFull(f, buf)
			_ = f.Close()
			if !isValidImageHeader(buf[:n], out.Format) {
				res.Valid = false
				res.Error = "截图文件格式非法或非安全媒体"
				return res
			}
		}
		res.Valid = true
	}
	return res
}

// GetOutputFile 打开并返回安全的产出文件描述符、MIME 类型及是否为附件
func (s *Service) GetOutputFile(taskID, outputID string, subFiles ...string) (file *os.File, mimeType string, isAttachment bool, err error) {
	if err := validateSafeIdentifier(taskID); err != nil {
		return nil, "", false, ErrInvalidTaskID
	}

	rootDir, err := s.GetRootDir()
	if err != nil {
		return nil, "", false, err
	}
	taskDir := filepath.Join(rootDir, taskID)

	evalRootDir, _ := filepath.EvalSymlinks(rootDir)
	evalTaskDir, err := filepath.EvalSymlinks(taskDir)
	if err != nil || (evalRootDir != "" && !isSubPath(evalRootDir, evalTaskDir)) {
		return nil, "", false, ErrTaskNotFound
	}

	var taskDef TaskDef
	if err := readJSONFile(filepath.Join(taskDir, "task.json"), &taskDef); err != nil {
		return nil, "", false, fmt.Errorf("读取任务定义失败: %w", err)
	}

	var matchedOut *OutputDef
	for i := range taskDef.Outputs {
		if taskDef.Outputs[i].ID == outputID {
			matchedOut = &taskDef.Outputs[i]
			break
		}
	}
	if matchedOut == nil {
		return nil, "", false, ErrOutputNotFound
	}

	absPath, err := safeResolvePath(taskDir, matchedOut.Path)
	if err != nil {
		return nil, "", false, err
	}

	info, err := os.Stat(absPath)
	if err != nil {
		return nil, "", false, ErrOutputNotFound
	}

	subFile := ""
	if len(subFiles) > 0 {
		subFile = strings.TrimSpace(subFiles[0])
	}

	if info.IsDir() {
		if subFile == "" {
			return nil, "", false, errors.New("目标产出为目录，请指定 ?file=文件名 进行预览或下载")
		}
		subClean := filepath.Clean(subFile)
		if strings.HasPrefix(subClean, "..") || strings.HasPrefix(subClean, "/") || strings.HasPrefix(subClean, "\\") {
			return nil, "", false, ErrPathTraversal
		}
		subAbs, err := safeResolvePath(absPath, subClean)
		if err != nil {
			return nil, "", false, err
		}
		subF, err := os.OpenFile(subAbs, os.O_RDONLY, 0)
		if err != nil {
			return nil, "", false, ErrOutputNotFound
		}
		subInfo, err := subF.Stat()
		if err != nil || !subInfo.Mode().IsRegular() {
			_ = subF.Close()
			return nil, "", false, ErrOutputNotFound
		}
		file = subF
		absPath = subAbs
	} else {
		f, err := os.OpenFile(absPath, os.O_RDONLY, 0)
		if err != nil {
			return nil, "", false, ErrOutputNotFound
		}
		fInfo, err := f.Stat()
		if err != nil || !fInfo.Mode().IsRegular() {
			_ = f.Close()
			return nil, "", false, ErrOutputNotFound
		}
		file = f
	}

	// 媒体与文本安全类型检查
	switch matchedOut.Category {
	case "report":
		mimeType = "text/markdown; charset=utf-8"
	case "log":
		mimeType = "text/plain; charset=utf-8"
		isAttachment = true
	case "screenshot":
		ext := strings.ToLower(filepath.Ext(absPath))
		switch ext {
		case ".png":
			mimeType = "image/png"
		case ".jpg", ".jpeg":
			mimeType = "image/jpeg"
		case ".webp":
			mimeType = "image/webp"
		default:
			_ = file.Close()
			return nil, "", false, ErrUnsafeMedia
		}
		// 校验文件头魔数，拒绝假冒图片（如 <html>fake image</html>）
		buf := make([]byte, 16)
		n, _ := io.ReadFull(file, buf)
		if _, err := file.Seek(0, io.SeekStart); err != nil {
			_ = file.Close()
			return nil, "", false, err
		}
		if !isValidImageHeader(buf[:n], ext) {
			_ = file.Close()
			return nil, "", false, ErrUnsafeMedia
		}
	case "video":
		ext := strings.ToLower(filepath.Ext(absPath))
		switch ext {
		case ".mp4":
			mimeType = "video/mp4"
		case ".webm":
			mimeType = "video/webm"
		default:
			_ = file.Close()
			return nil, "", false, ErrUnsafeMedia
		}
	default:
		mimeType = "application/octet-stream"
		isAttachment = true
	}

	return file, mimeType, isAttachment, nil
}

// FindRunningTaskForDevice 供设备状态接口按最近扫描索引关联运行中的任务
func (s *Service) FindRunningTaskForDevice(serial string) (taskID string, hasConflict bool, count int, scannedAt int64) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	scannedAt = s.lastScannedAt
	if scannedAt == 0 {
		return "", false, 0, 0
	}

	var running []string
	for _, t := range s.cachedTasks {
		if t.DeviceSerial == serial && t.Status == "running" {
			running = append(running, t.ID)
		}
	}

	count = len(running)
	if count == 0 {
		return "", false, 0, scannedAt
	}
	if count == 1 {
		return running[0], false, 1, scannedAt
	}
	// count > 1 冲突
	return "", true, count, scannedAt
}
