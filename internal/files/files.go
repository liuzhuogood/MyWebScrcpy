package files

import (
	"errors"
	"fmt"
	"io"
	"mime"
	"net/http"
	"os"
	"path"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"syscall"
	"time"
)

const (
	defaultRoot       = "files"
	defaultPageSize   = 50
	maxPageSize       = 100
	defaultUploadSize = int64(256 * 1024 * 1024)
	trashDirName      = ".trash"
)

var (
	ErrNotFound  = errors.New("文件不存在")
	ErrConflict  = errors.New("目标已存在")
	ErrInvalid   = errors.New("请求参数无效")
	ErrForbidden = errors.New("文件路径不允许访问")
)

// RootPath 返回文件管理根目录，可通过 FILES_DIR 覆盖。
func RootPath() string {
	if root := os.Getenv("FILES_DIR"); root != "" {
		return root
	}
	return defaultRoot
}

// Token 返回文件 API 的可选 Bearer 令牌。未设置时沿用单用户部署模式。
func Token() string { return os.Getenv("FILES_TOKEN") }

// MaxUploadBytes 返回单文件上传上限，可通过 FILES_MAX_UPLOAD_BYTES 覆盖。
func MaxUploadBytes() int64 {
	if raw := os.Getenv("FILES_MAX_UPLOAD_BYTES"); raw != "" {
		var value int64
		if _, err := fmt.Sscanf(raw, "%d", &value); err == nil && value > 0 {
			return value
		}
	}
	return defaultUploadSize
}

// EnsureRoot 确保文件根目录和回收站目录存在。
func EnsureRoot() error {
	root, err := filepath.Abs(RootPath())
	if err != nil {
		return fmt.Errorf("解析文件根目录失败: %w", err)
	}
	if err := os.MkdirAll(filepath.Join(root, trashDirName), 0755); err != nil {
		return fmt.Errorf("创建文件根目录失败: %w", err)
	}
	return nil
}

// Manager 负责单用户文件根目录内的浏览和文件操作。
type Manager struct {
	root           string
	token          string
	maxUploadBytes int64

	trashMu sync.Mutex
	trash   map[string]trashRecord
}

type trashRecord struct {
	items []trashItem
}

type trashItem struct {
	original string
	stored   string
}

// NewManager 创建文件管理器并初始化根目录。
func NewManager(root, token string, maxUploadBytes int64) (*Manager, error) {
	if root == "" {
		root = defaultRoot
	}
	absRoot, err := filepath.Abs(root)
	if err != nil {
		return nil, fmt.Errorf("解析文件根目录失败: %w", err)
	}
	if maxUploadBytes <= 0 {
		maxUploadBytes = defaultUploadSize
	}
	m := &Manager{
		root:           absRoot,
		token:          token,
		maxUploadBytes: maxUploadBytes,
		trash:          make(map[string]trashRecord),
	}
	if err := os.MkdirAll(filepath.Join(absRoot, trashDirName), 0755); err != nil {
		return nil, fmt.Errorf("创建文件根目录失败: %w", err)
	}
	return m, nil
}

// NewManagerFromEnv 按当前进程配置创建文件管理器。
func NewManagerFromEnv() (*Manager, error) {
	return NewManager(RootPath(), Token(), MaxUploadBytes())
}

// Root 返回管理器使用的绝对根目录，供启动日志和测试使用。
func (m *Manager) Root() string { return m.root }

// Authorized 判断请求是否携带正确的文件 API 令牌。
func (m *Manager) Authorized(r *http.Request) bool {
	if m.token == "" {
		return true
	}
	return strings.TrimSpace(r.Header.Get("Authorization")) == "Bearer "+m.token
}

// FileItem 是文件列表和操作响应中的统一元数据。
type FileItem struct {
	Name     string  `json:"name"`
	Path     string  `json:"path"`
	Kind     string  `json:"kind"`
	Mime     string  `json:"mime"`
	Size     int64   `json:"size"`
	Modified string  `json:"modified"`
	Actions  Actions `json:"actions"`
}

type Actions struct {
	Download bool `json:"download"`
	Move     bool `json:"move"`
	Rename   bool `json:"rename"`
	Delete   bool `json:"delete"`
}

type Breadcrumb struct {
	Name string `json:"name"`
	Path string `json:"path"`
}

type Storage struct {
	Used      int64 `json:"used"`
	Total     int64 `json:"total"`
	Available int64 `json:"available"`
}

type ListResult struct {
	Path        string       `json:"path"`
	Breadcrumbs []Breadcrumb `json:"breadcrumbs"`
	Items       []FileItem   `json:"items"`
	Total       int          `json:"total"`
	Page        int          `json:"page"`
	PageSize    int          `json:"page_size"`
	HasMore     bool         `json:"has_more"`
	Storage     Storage      `json:"storage"`
}

// List 返回目录内容；q 非空时在当前目录及子目录中按名称搜索。
func (m *Manager) List(relDir, q, kind, sortBy, order string, page, pageSize int) (ListResult, error) {
	relDir, err := cleanRel(relDir)
	if err != nil {
		return ListResult{}, err
	}
	dir, err := m.resolve(relDir, false)
	if err != nil {
		return ListResult{}, err
	}
	info, err := os.Stat(dir)
	if err != nil {
		return ListResult{}, err
	}
	if !info.IsDir() {
		return ListResult{}, fmt.Errorf("不是文件夹")
	}

	items, err := m.collectItems(relDir, q, kind)
	if err != nil {
		return ListResult{}, err
	}
	sortItems(items, sortBy, order)
	if page < 1 {
		page = 1
	}
	if pageSize < 1 {
		pageSize = defaultPageSize
	}
	if pageSize > maxPageSize {
		pageSize = maxPageSize
	}
	start := (page - 1) * pageSize
	if start > len(items) {
		start = len(items)
	}
	end := start + pageSize
	if end > len(items) {
		end = len(items)
	}
	pageItems := items[start:end]
	if pageItems == nil {
		pageItems = []FileItem{}
	}
	storage := m.storage()
	return ListResult{
		Path:        relDir,
		Breadcrumbs: breadcrumbs(relDir),
		Items:       pageItems,
		Total:       len(items),
		Page:        page,
		PageSize:    pageSize,
		HasMore:     end < len(items),
		Storage:     storage,
	}, nil
}

func (m *Manager) collectItems(relDir, query, kind string) ([]FileItem, error) {
	query = strings.ToLower(strings.TrimSpace(query))
	kind = strings.ToLower(strings.TrimSpace(kind))
	base, err := m.resolve(relDir, false)
	if err != nil {
		return nil, err
	}
	var items []FileItem
	if query == "" {
		entries, err := os.ReadDir(base)
		if err != nil {
			return nil, err
		}
		for _, entry := range entries {
			if entry.Name() == trashDirName || entry.Type()&os.ModeSymlink != 0 {
				continue
			}
			item, err := m.itemFromEntry(relDir, entry)
			if err != nil {
				continue
			}
			if matchesKind(item, kind) {
				items = append(items, item)
			}
		}
		return items, nil
	}

	err = filepath.WalkDir(base, func(current string, entry os.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if current != base && entry.Name() == trashDirName {
			if entry.IsDir() {
				return filepath.SkipDir
			}
			return nil
		}
		if entry.Type()&os.ModeSymlink != 0 {
			if entry.IsDir() {
				return filepath.SkipDir
			}
			return nil
		}
		if current == base {
			return nil
		}
		if !strings.Contains(strings.ToLower(entry.Name()), query) {
			return nil
		}
		rel, err := filepath.Rel(m.root, current)
		if err != nil {
			return err
		}
		info, err := entry.Info()
		if err != nil {
			return nil
		}
		item, err := m.itemFromInfo(filepath.ToSlash(rel), entry.Name(), info)
		if err != nil {
			return nil
		}
		if matchesKind(item, kind) {
			items = append(items, item)
		}
		return nil
	})
	return items, err
}

func (m *Manager) itemFromEntry(parent string, entry os.DirEntry) (FileItem, error) {
	info, err := entry.Info()
	if err != nil {
		return FileItem{}, err
	}
	rel := entry.Name()
	if parent != "" {
		rel = path.Join(parent, rel)
	}
	return m.itemFromInfo(rel, entry.Name(), info)
}

func (m *Manager) itemFromInfo(rel, name string, info os.FileInfo) (FileItem, error) {
	item := FileItem{
		Name:     name,
		Path:     filepath.ToSlash(rel),
		Kind:     kindFor(info, name),
		Mime:     mimeFor(info, name),
		Size:     info.Size(),
		Modified: info.ModTime().Format(time.RFC3339),
		Actions: Actions{
			Download: !info.IsDir(),
			Move:     true,
			Rename:   true,
			Delete:   true,
		},
	}
	if info.IsDir() {
		item.Size = 0
	}
	return item, nil
}

func kindFor(info os.FileInfo, name string) string {
	if info.IsDir() {
		return "folder"
	}
	switch strings.ToLower(filepath.Ext(name)) {
	case ".jpg", ".jpeg", ".png", ".gif", ".webp", ".svg", ".heic", ".bmp":
		return "image"
	case ".mp4", ".mov", ".mkv", ".avi", ".webm":
		return "video"
	case ".mp3", ".wav", ".flac", ".m4a", ".aac", ".ogg":
		return "audio"
	case ".pdf", ".doc", ".docx", ".xls", ".xlsx", ".ppt", ".pptx", ".txt", ".md", ".csv", ".json", ".xml":
		return "document"
	default:
		return "other"
	}
}

func mimeFor(info os.FileInfo, name string) string {
	if info.IsDir() {
		return "inode/directory"
	}
	if value := mime.TypeByExtension(strings.ToLower(filepath.Ext(name))); value != "" {
		return value
	}
	return "application/octet-stream"
}

func matchesKind(item FileItem, kind string) bool {
	return kind == "" || kind == "all" || item.Kind == kind
}

func sortItems(items []FileItem, sortBy, order string) {
	if sortBy == "" {
		sortBy = "name"
	}
	desc := strings.EqualFold(order, "desc")
	sort.SliceStable(items, func(i, j int) bool {
		a, b := items[i], items[j]
		var compare int
		switch sortBy {
		case "modified":
			compare = strings.Compare(a.Modified, b.Modified)
		case "size":
			if a.Size < b.Size {
				compare = -1
			} else if a.Size > b.Size {
				compare = 1
			}
		default:
			if a.Kind != b.Kind {
				if a.Kind == "folder" {
					compare = -1
				} else {
					compare = 1
				}
			} else {
				compare = strings.Compare(strings.ToLower(a.Name), strings.ToLower(b.Name))
			}
		}
		if compare == 0 {
			compare = strings.Compare(strings.ToLower(a.Name), strings.ToLower(b.Name))
		}
		if desc {
			return compare > 0
		}
		return compare < 0
	})
}

func breadcrumbs(rel string) []Breadcrumb {
	result := []Breadcrumb{{Name: "我的文件", Path: ""}}
	if rel == "" {
		return result
	}
	current := ""
	for _, segment := range strings.Split(rel, "/") {
		if segment == "" {
			continue
		}
		current = path.Join(current, segment)
		result = append(result, Breadcrumb{Name: segment, Path: current})
	}
	return result
}

func (m *Manager) storage() Storage {
	var stat syscall.Statfs_t
	if err := syscall.Statfs(m.root, &stat); err != nil {
		return Storage{}
	}
	total := int64(stat.Blocks) * int64(stat.Bsize)
	available := int64(stat.Bavail) * int64(stat.Bsize)
	used := total - available
	if used < 0 {
		used = 0
	}
	return Storage{Used: used, Total: total, Available: available}
}

func (m *Manager) resolve(rel string, allowMissing bool) (string, error) {
	rel, err := cleanRel(rel)
	if err != nil {
		return "", err
	}
	target := m.root
	if rel != "" {
		target = filepath.Join(m.root, filepath.FromSlash(rel))
	}
	if err := ensureWithin(m.root, target); err != nil {
		return "", err
	}
	rootResolved, err := filepath.EvalSymlinks(m.root)
	if err != nil {
		return "", err
	}
	checkPath := target
	if _, statErr := os.Lstat(target); os.IsNotExist(statErr) && allowMissing {
		parent := filepath.Dir(target)
		for parent != m.root {
			if _, err := os.Lstat(parent); err == nil {
				break
			}
			parent = filepath.Dir(parent)
		}
		checkPath = parent
	}
	resolved, err := filepath.EvalSymlinks(checkPath)
	if err != nil {
		if allowMissing && os.IsNotExist(err) {
			return target, nil
		}
		return "", err
	}
	if err := ensureWithin(rootResolved, resolved); err != nil {
		return "", ErrForbidden
	}
	return target, nil
}

func ensureWithin(root, target string) error {
	rel, err := filepath.Rel(root, target)
	if err != nil || filepath.IsAbs(rel) || rel == ".." || strings.HasPrefix(rel, ".."+string(filepath.Separator)) {
		return ErrForbidden
	}
	return nil
}

func cleanRel(raw string) (string, error) {
	if raw == "" {
		return "", nil
	}
	if strings.ContainsRune(raw, '\x00') || strings.Contains(raw, "\\") || filepath.IsAbs(raw) || strings.HasPrefix(raw, "/") {
		return "", ErrForbidden
	}
	if len(raw) >= 2 && raw[1] == ':' {
		return "", ErrForbidden
	}
	parts := strings.Split(raw, "/")
	cleaned := make([]string, 0, len(parts))
	for _, part := range parts {
		switch part {
		case "", ".":
			continue
		case "..":
			return "", ErrForbidden
		case trashDirName:
			return "", ErrForbidden
		default:
			if strings.ContainsRune(part, '\x00') {
				return "", ErrForbidden
			}
			cleaned = append(cleaned, part)
		}
	}
	return path.Join(cleaned...), nil
}

func validateName(name string) error {
	if name == "" || name == "." || name == ".." || name == trashDirName || strings.ContainsAny(name, "/\\") || strings.ContainsRune(name, '\x00') {
		return ErrInvalid
	}
	for _, r := range name {
		if r < 0x20 || r == 0x7f {
			return ErrInvalid
		}
	}
	return nil
}

func conflictTarget(target, conflict string, isDir bool) (string, error) {
	info, err := os.Stat(target)
	if os.IsNotExist(err) {
		return target, nil
	} else if err != nil {
		return "", err
	}
	if info.IsDir() != isDir {
		return "", ErrConflict
	}
	switch strings.ToLower(conflict) {
	case "rename", "keep":
		ext := filepath.Ext(target)
		stem := strings.TrimSuffix(filepath.Base(target), ext)
		for index := 1; index < 10000; index++ {
			candidate := filepath.Join(filepath.Dir(target), fmt.Sprintf("%s (%d)%s", stem, index, ext))
			if _, err := os.Lstat(candidate); os.IsNotExist(err) {
				return candidate, nil
			}
		}
		return "", fmt.Errorf("无法生成不冲突的文件名")
	case "replace":
		return target, nil
	default:
		return "", ErrConflict
	}
}

// CreateFolder 创建文件夹。
func (m *Manager) CreateFolder(parent, name string) (FileItem, error) {
	if err := validateName(name); err != nil {
		return FileItem{}, err
	}
	parentPath, err := m.resolve(parent, false)
	if err != nil {
		return FileItem{}, err
	}
	parentInfo, err := os.Stat(parentPath)
	if err != nil || !parentInfo.IsDir() {
		return FileItem{}, ErrNotFound
	}
	target := filepath.Join(parentPath, name)
	if _, err := m.resolve(path.Join(parent, name), true); err != nil {
		return FileItem{}, err
	}
	if _, err := os.Lstat(target); err == nil {
		return FileItem{}, ErrConflict
	}
	if err := os.Mkdir(target, 0755); err != nil {
		return FileItem{}, err
	}
	return m.itemByRel(path.Join(parent, name))
}

// Upload 保存一个上传文件，并使用临时文件避免留下半成品。
func (m *Manager) Upload(parent, name, conflict string, src io.Reader) (FileItem, error) {
	if err := validateName(name); err != nil {
		return FileItem{}, err
	}
	parentPath, err := m.resolve(parent, false)
	if err != nil {
		return FileItem{}, err
	}
	info, err := os.Stat(parentPath)
	if err != nil || !info.IsDir() {
		return FileItem{}, ErrNotFound
	}
	target := filepath.Join(parentPath, name)
	target, err = conflictTarget(target, conflict, false)
	if err != nil {
		return FileItem{}, err
	}
	temp, err := os.CreateTemp(parentPath, ".upload-*")
	if err != nil {
		return FileItem{}, err
	}
	tempName := temp.Name()
	defer os.Remove(tempName)
	written, err := io.Copy(temp, io.LimitReader(src, m.maxUploadBytes+1))
	if err != nil {
		_ = temp.Close()
		return FileItem{}, err
	}
	if written > m.maxUploadBytes {
		_ = temp.Close()
		return FileItem{}, fmt.Errorf("文件超过 %d 字节上传上限", m.maxUploadBytes)
	}
	if err := temp.Chmod(0644); err != nil {
		_ = temp.Close()
		return FileItem{}, err
	}
	if err := temp.Close(); err != nil {
		return FileItem{}, err
	}
	if _, err := os.Lstat(target); err == nil {
		if strings.EqualFold(conflict, "replace") {
			if err := os.RemoveAll(target); err != nil {
				return FileItem{}, err
			}
		} else {
			return FileItem{}, ErrConflict
		}
	}
	relTarget, err := filepath.Rel(m.root, target)
	if err != nil {
		return FileItem{}, err
	}
	if _, err := m.resolve(filepath.ToSlash(relTarget), true); err != nil {
		return FileItem{}, err
	}
	if err := os.Rename(tempName, target); err != nil {
		return FileItem{}, err
	}
	rel, _ := filepath.Rel(m.root, target)
	return m.itemByRel(filepath.ToSlash(rel))
}

func (m *Manager) itemByRel(rel string) (FileItem, error) {
	target, err := m.resolve(rel, false)
	if err != nil {
		return FileItem{}, err
	}
	info, err := os.Stat(target)
	if err != nil {
		return FileItem{}, ErrNotFound
	}
	return m.itemFromInfo(rel, filepath.Base(target), info)
}

func (m *Manager) moveOrRename(source, destination, conflict string) (FileItem, error) {
	source, err := cleanRel(source)
	if err != nil || source == "" {
		return FileItem{}, ErrInvalid
	}
	destination, err = cleanRel(destination)
	if err != nil || destination == "" {
		return FileItem{}, ErrInvalid
	}
	sourcePath, err := m.resolve(source, false)
	if err != nil {
		return FileItem{}, err
	}
	sourceInfo, err := os.Stat(sourcePath)
	if err != nil {
		return FileItem{}, ErrNotFound
	}
	if sourceInfo.IsDir() && (destination == source || strings.HasPrefix(destination, source+"/")) {
		return FileItem{}, ErrInvalid
	}
	destinationPath, err := m.resolve(destination, true)
	if err != nil {
		return FileItem{}, err
	}
	parentInfo, err := os.Stat(filepath.Dir(destinationPath))
	if err != nil || !parentInfo.IsDir() {
		return FileItem{}, ErrNotFound
	}
	if targetInfo, statErr := os.Stat(destinationPath); statErr == nil && targetInfo.IsDir() && sourceInfo.IsDir() {
		return FileItem{}, ErrConflict
	}
	finalTarget, err := conflictTarget(destinationPath, conflict, sourceInfo.IsDir())
	if err != nil {
		return FileItem{}, err
	}
	if finalTarget == sourcePath {
		return FileItem{}, ErrInvalid
	}
	if _, err := os.Lstat(finalTarget); err == nil && strings.EqualFold(conflict, "replace") {
		if err := os.RemoveAll(finalTarget); err != nil {
			return FileItem{}, err
		}
	}
	if err := os.Rename(sourcePath, finalTarget); err != nil {
		return FileItem{}, err
	}
	rel, _ := filepath.Rel(m.root, finalTarget)
	return m.itemByRel(filepath.ToSlash(rel))
}

// Move 将文件或文件夹移动到目标文件夹。
func (m *Manager) Move(source, targetDir, conflict string) (FileItem, error) {
	source, err := cleanRel(source)
	if err != nil || source == "" {
		return FileItem{}, ErrInvalid
	}
	targetDir, err = cleanRel(targetDir)
	if err != nil {
		return FileItem{}, err
	}
	name := path.Base(source)
	return m.moveOrRename(source, path.Join(targetDir, name), conflict)
}

// Rename 在原目录内重命名文件或文件夹。
func (m *Manager) Rename(source, name, conflict string) (FileItem, error) {
	if err := validateName(name); err != nil {
		return FileItem{}, err
	}
	var err error
	source, err = cleanRel(source)
	if err != nil || source == "" {
		return FileItem{}, ErrInvalid
	}
	return m.moveOrRename(source, path.Join(path.Dir(source), name), conflict)
}

// Delete 将对象移动到回收站，返回可用于撤销的令牌。
func (m *Manager) Delete(paths []string) (string, error) {
	if len(paths) == 0 || len(paths) > 100 {
		return "", ErrInvalid
	}
	token := fmt.Sprintf("%d", time.Now().UnixNano())
	trashRoot := filepath.Join(m.root, trashDirName, token)
	if err := os.MkdirAll(trashRoot, 0700); err != nil {
		return "", err
	}
	record := trashRecord{}
	for index, raw := range paths {
		rel, err := cleanRel(raw)
		if err != nil || rel == "" {
			m.rollbackDelete(record)
			_ = os.RemoveAll(trashRoot)
			return "", ErrInvalid
		}
		source, err := m.resolve(rel, false)
		if err != nil {
			m.rollbackDelete(record)
			_ = os.RemoveAll(trashRoot)
			return "", err
		}
		if _, err := os.Stat(source); err != nil {
			m.rollbackDelete(record)
			_ = os.RemoveAll(trashRoot)
			return "", ErrNotFound
		}
		stored := filepath.Join(trashRoot, fmt.Sprintf("%03d-%s", index, filepath.Base(source)))
		if err := os.Rename(source, stored); err != nil {
			m.rollbackDelete(record)
			_ = os.RemoveAll(trashRoot)
			return "", err
		}
		record.items = append(record.items, trashItem{original: rel, stored: stored})
	}
	m.trashMu.Lock()
	m.trash[token] = record
	m.trashMu.Unlock()
	return token, nil
}

func (m *Manager) rollbackDelete(record trashRecord) {
	for index := len(record.items) - 1; index >= 0; index-- {
		item := record.items[index]
		_ = os.MkdirAll(filepath.Dir(filepath.Join(m.root, filepath.FromSlash(item.original))), 0755)
		_ = os.Rename(item.stored, filepath.Join(m.root, filepath.FromSlash(item.original)))
	}
}

// Undo 恢复一次删除，若原位置已被占用则保持回收站数据不变并返回冲突。
func (m *Manager) Undo(token string) error {
	if token == "" {
		return ErrInvalid
	}
	m.trashMu.Lock()
	record, ok := m.trash[token]
	m.trashMu.Unlock()
	if !ok {
		return ErrNotFound
	}
	for _, item := range record.items {
		target, err := m.resolve(item.original, true)
		if err != nil {
			return err
		}
		if _, err := os.Lstat(target); err == nil {
			return ErrConflict
		}
	}
	restored := make([]trashItem, 0, len(record.items))
	for _, item := range record.items {
		target := filepath.Join(m.root, filepath.FromSlash(item.original))
		if err := os.MkdirAll(filepath.Dir(target), 0755); err != nil {
			m.rollbackUndo(restored)
			return err
		}
		if err := os.Rename(item.stored, target); err != nil {
			m.rollbackUndo(restored)
			return err
		}
		restored = append(restored, item)
	}
	_ = os.RemoveAll(filepath.Join(m.root, trashDirName, token))
	m.trashMu.Lock()
	delete(m.trash, token)
	m.trashMu.Unlock()
	return nil
}

func (m *Manager) rollbackUndo(restored []trashItem) {
	for index := len(restored) - 1; index >= 0; index-- {
		item := restored[index]
		target := filepath.Join(m.root, filepath.FromSlash(item.original))
		_ = os.Rename(target, item.stored)
	}
}
