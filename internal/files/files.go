package files

import (
	"errors"
	"fmt"
	"io"
	"mime"
	"os"
	"os/exec"
	"path"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"
)

const (
	androidRoot       = "/storage/emulated/0"
	trashDirName      = ".mywebscrcpy-trash"
	defaultPageSize   = 50
	maxPageSize       = 100
	defaultUploadSize = int64(256 * 1024 * 1024)
)

var (
	ErrNotFound          = errors.New("文件不存在")
	ErrConflict          = errors.New("目标已存在")
	ErrInvalid           = errors.New("请求参数无效")
	ErrForbidden         = errors.New("文件路径不允许访问")
	ErrDeviceUnavailable = errors.New("手机不可用")
)

// MaxUploadBytes 返回单文件上传上限。文件本身存储在手机上，不再支持服务端文件根目录配置。
func MaxUploadBytes() int64 {
	if raw := os.Getenv("FILES_MAX_UPLOAD_BYTES"); raw != "" {
		value, err := strconv.ParseInt(raw, 10, 64)
		if err == nil && value > 0 {
			return value
		}
	}
	return defaultUploadSize
}

type commandRunner func(serial string, args ...string) ([]byte, error)
type streamRunner func(serial string, dst io.Writer, args ...string) error

// Manager 通过 ADB 管理指定手机的共享存储，不在服务端保留用户文件。
type Manager struct {
	adbPath        string
	maxUploadBytes int64
	run            commandRunner
	stream         streamRunner

	trashMu sync.Mutex
	trash   map[string]trashRecord
}

type trashRecord struct{ items []trashItem }
type trashItem struct{ original, stored string }

// NewManager 创建手机文件管理器。adbPath 为空时使用 PATH 中的 adb。
func NewManager(adbPath string) *Manager {
	if adbPath == "" {
		adbPath = "adb"
	}
	m := &Manager{adbPath: adbPath, maxUploadBytes: MaxUploadBytes(), trash: make(map[string]trashRecord)}
	m.run = m.exec
	m.stream = m.execStream
	return m
}

func (m *Manager) exec(serial string, args ...string) ([]byte, error) {
	commandArgs := append([]string{"-s", serial}, args...)
	output, err := exec.Command(m.adbPath, commandArgs...).CombinedOutput()
	if err != nil && len(output) > 0 {
		return output, fmt.Errorf("%w: %s", err, strings.TrimSpace(string(output)))
	}
	return output, err
}

func (m *Manager) execStream(serial string, dst io.Writer, args ...string) error {
	commandArgs := append([]string{"-s", serial}, args...)
	cmd := exec.Command(m.adbPath, commandArgs...)
	cmd.Stdout = dst
	stderr, err := cmd.StderrPipe()
	if err != nil {
		return err
	}
	if err := cmd.Start(); err != nil {
		return err
	}
	errOutput, _ := io.ReadAll(stderr)
	if err := cmd.Wait(); err != nil {
		if len(errOutput) > 0 {
			return fmt.Errorf("%w: %s", err, strings.TrimSpace(string(errOutput)))
		}
		return err
	}
	return nil
}

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

type remoteStat struct {
	Kind     string
	Size     int64
	Modified string
	Name     string
}

func (m *Manager) List(serial, relDir, query, kind, sortBy, order string, page, pageSize int) (ListResult, error) {
	relDir, err := cleanRel(relDir)
	if err != nil {
		return ListResult{}, err
	}
	stat, err := m.stat(serial, relDir)
	if err != nil {
		return ListResult{}, err
	}
	if stat.Kind != "folder" {
		return ListResult{}, ErrNotFound
	}
	page, pageSize = normalizePage(page, pageSize)
	base := remotePath(relDir)
	findArgs := "-maxdepth 1 -mindepth 1"
	if strings.TrimSpace(query) != "" {
		findArgs = "-mindepth 1 \\( -type f -o -type d \\)"
	}
	output, err := m.shell(serial, fmt.Sprintf("find %s %s -exec stat -c '%%F\\t%%s\\t%%Y\\t%%n' {} \\;", shellQuote(base), findArgs))
	if err != nil {
		return ListResult{}, mapRemoteError(err)
	}

	items := make([]FileItem, 0)
	query = strings.ToLower(strings.TrimSpace(query))
	for _, line := range strings.Split(string(output), "\n") {
		item, ok := parseStatLine(line)
		if !ok || item.Name == trashDirName || item.Path == trashDirName || strings.HasPrefix(item.Path, trashDirName+"/") {
			continue
		}
		if query != "" && !strings.Contains(strings.ToLower(item.Name), query) {
			continue
		}
		if kind != "" && kind != "all" && item.Kind != kind {
			continue
		}
		items = append(items, item)
	}
	sortItems(items, sortBy, order)
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
	return ListResult{Path: relDir, Breadcrumbs: breadcrumbs(relDir), Items: pageItems, Total: len(items), Page: page, PageSize: pageSize, HasMore: end < len(items), Storage: m.storage(serial)}, nil
}

func normalizePage(page, pageSize int) (int, int) {
	if page < 1 {
		page = 1
	}
	if pageSize < 1 {
		pageSize = defaultPageSize
	}
	if pageSize > maxPageSize {
		pageSize = maxPageSize
	}
	return page, pageSize
}

func parseStatLine(line string) (FileItem, bool) {
	fields := strings.SplitN(strings.TrimSpace(line), "\t", 4)
	if len(fields) != 4 || strings.Contains(strings.ToLower(fields[0]), "symbolic link") {
		return FileItem{}, false
	}
	size, err := strconv.ParseInt(strings.TrimSpace(fields[1]), 10, 64)
	if err != nil {
		return FileItem{}, false
	}
	modifiedUnix, err := strconv.ParseInt(strings.TrimSpace(fields[2]), 10, 64)
	if err != nil {
		modifiedUnix = 0
	}
	remote := strings.TrimSpace(fields[3])
	if remote != androidRoot && !strings.HasPrefix(remote, androidRoot+"/") {
		return FileItem{}, false
	}
	rel := strings.TrimPrefix(remote, androidRoot+"/")
	name := path.Base(rel)
	if rel == "" {
		name = "手机存储"
	}
	isDir := strings.Contains(strings.ToLower(fields[0]), "directory")
	if isDir {
		size = 0
	}
	return FileItem{Name: name, Path: rel, Kind: kindFor(isDir, name), Mime: mimeFor(isDir, name), Size: size, Modified: time.Unix(modifiedUnix, 0).Format(time.RFC3339), Actions: Actions{Download: !isDir, Move: true, Rename: true, Delete: true}}, true
}

func (m *Manager) stat(serial, rel string) (remoteStat, error) {
	rel, err := cleanRel(rel)
	if err != nil {
		return remoteStat{}, err
	}
	output, err := m.shell(serial, fmt.Sprintf("stat -c '%%F\\t%%s\\t%%Y\\t%%n' %s", shellQuote(remotePath(rel))))
	if err != nil {
		return remoteStat{}, mapRemoteError(err)
	}
	item, ok := parseStatLine(strings.TrimSpace(string(output)))
	if !ok {
		return remoteStat{}, ErrNotFound
	}
	return remoteStat{Kind: item.Kind, Size: item.Size, Modified: item.Modified, Name: item.Name}, nil
}

func (m *Manager) item(serial, rel string) (FileItem, error) {
	stat, err := m.stat(serial, rel)
	if err != nil {
		return FileItem{}, err
	}
	isDir := stat.Kind == "folder"
	return FileItem{Name: stat.Name, Path: rel, Kind: stat.Kind, Mime: mimeFor(isDir, stat.Name), Size: stat.Size, Modified: stat.Modified, Actions: Actions{Download: !isDir, Move: true, Rename: true, Delete: true}}, nil
}

func (m *Manager) storage(serial string) Storage {
	output, err := m.shell(serial, "df -k "+shellQuote(androidRoot))
	if err != nil {
		return Storage{}
	}
	var fields []string
	for _, line := range strings.Split(string(output), "\n") {
		parts := strings.Fields(line)
		if len(parts) >= 4 {
			fields = parts
		}
	}
	if len(fields) < 4 {
		return Storage{}
	}
	total, _ := strconv.ParseInt(fields[1], 10, 64)
	used, _ := strconv.ParseInt(fields[2], 10, 64)
	available, _ := strconv.ParseInt(fields[3], 10, 64)
	return Storage{Used: used * 1024, Total: total * 1024, Available: available * 1024}
}

func (m *Manager) CreateFolder(serial, parent, name string) (FileItem, error) {
	if err := validateName(name); err != nil {
		return FileItem{}, err
	}
	parent, err := cleanRel(parent)
	if err != nil {
		return FileItem{}, err
	}
	if stat, err := m.stat(serial, parent); err != nil || stat.Kind != "folder" {
		return FileItem{}, ErrNotFound
	}
	rel := path.Join(parent, name)
	if _, err := m.stat(serial, rel); err == nil {
		return FileItem{}, ErrConflict
	}
	if _, err := m.shell(serial, "mkdir "+shellQuote(remotePath(rel))); err != nil {
		return FileItem{}, mapRemoteError(err)
	}
	return m.item(serial, rel)
}

// Upload 先将请求写入临时文件，再通过 adb push 写入手机，避免服务端留下用户文件。
func (m *Manager) Upload(serial, parent, name, conflict string, src io.Reader) (FileItem, error) {
	if err := validateName(name); err != nil {
		return FileItem{}, err
	}
	parent, err := cleanRel(parent)
	if err != nil {
		return FileItem{}, err
	}
	if stat, err := m.stat(serial, parent); err != nil || stat.Kind != "folder" {
		return FileItem{}, ErrNotFound
	}
	rel, err := m.conflictTarget(serial, path.Join(parent, name), conflict, false)
	if err != nil {
		return FileItem{}, err
	}
	temp, err := os.CreateTemp("", "mywebscrcpy-upload-*")
	if err != nil {
		return FileItem{}, err
	}
	tempName := temp.Name()
	defer os.Remove(tempName)
	written, copyErr := io.Copy(temp, io.LimitReader(src, m.maxUploadBytes+1))
	if copyErr != nil {
		temp.Close()
		return FileItem{}, copyErr
	}
	if err := temp.Close(); err != nil {
		return FileItem{}, err
	}
	if written > m.maxUploadBytes {
		return FileItem{}, fmt.Errorf("文件超过 %d 字节上传上限", m.maxUploadBytes)
	}
	if conflict == "replace" {
		if _, err := m.stat(serial, rel); err == nil {
			if err := m.remove(serial, rel); err != nil {
				return FileItem{}, err
			}
		}
	}
	if _, err := m.run(serial, "push", tempName, remotePath(rel)); err != nil {
		return FileItem{}, mapRemoteError(err)
	}
	return m.item(serial, rel)
}

func (m *Manager) Download(serial, rel string, dst io.Writer) error {
	if stat, err := m.stat(serial, rel); err != nil {
		return err
	} else if stat.Kind == "folder" {
		return ErrInvalid
	}
	if err := m.stream(serial, dst, "exec-out", "cat", remotePath(rel)); err != nil {
		return mapRemoteError(err)
	}
	return nil
}

func (m *Manager) Move(serial, source, targetDir, conflict string) (FileItem, error) {
	source, err := cleanRel(source)
	if err != nil || source == "" {
		return FileItem{}, ErrInvalid
	}
	targetDir, err = cleanRel(targetDir)
	if err != nil {
		return FileItem{}, err
	}
	sourceStat, err := m.stat(serial, source)
	if err != nil {
		return FileItem{}, err
	}
	if targetStat, err := m.stat(serial, targetDir); err != nil || targetStat.Kind != "folder" {
		return FileItem{}, ErrNotFound
	}
	if sourceStat.Kind == "folder" && (targetDir == source || strings.HasPrefix(targetDir, source+"/")) {
		return FileItem{}, ErrInvalid
	}
	return m.moveTo(serial, source, path.Join(targetDir, path.Base(source)), conflict, sourceStat.Kind == "folder")
}

func (m *Manager) Rename(serial, source, name, conflict string) (FileItem, error) {
	if err := validateName(name); err != nil {
		return FileItem{}, err
	}
	source, err := cleanRel(source)
	if err != nil || source == "" {
		return FileItem{}, ErrInvalid
	}
	stat, err := m.stat(serial, source)
	if err != nil {
		return FileItem{}, err
	}
	return m.moveTo(serial, source, path.Join(path.Dir(source), name), conflict, stat.Kind == "folder")
}

func (m *Manager) moveTo(serial, source, destination, conflict string, isDir bool) (FileItem, error) {
	destination, err := cleanRel(destination)
	if err != nil || destination == "" || destination == source {
		return FileItem{}, ErrInvalid
	}
	final, err := m.conflictTarget(serial, destination, conflict, isDir)
	if err != nil {
		return FileItem{}, err
	}
	if conflict == "replace" {
		if _, err := m.stat(serial, final); err == nil {
			if err := m.remove(serial, final); err != nil {
				return FileItem{}, err
			}
		}
	}
	if _, err := m.shell(serial, fmt.Sprintf("mv %s %s", shellQuote(remotePath(source)), shellQuote(remotePath(final)))); err != nil {
		return FileItem{}, mapRemoteError(err)
	}
	return m.item(serial, final)
}

func (m *Manager) conflictTarget(serial, rel, conflict string, isDir bool) (string, error) {
	stat, err := m.stat(serial, rel)
	if err != nil {
		if errors.Is(err, ErrNotFound) {
			return rel, nil
		}
		return "", err
	}
	if (stat.Kind == "folder") != isDir {
		return "", ErrConflict
	}
	switch strings.ToLower(conflict) {
	case "rename", "keep":
		base, ext := path.Base(rel), path.Ext(rel)
		stem := strings.TrimSuffix(base, ext)
		for index := 1; index < 10000; index++ {
			candidate := path.Join(path.Dir(rel), fmt.Sprintf("%s (%d)%s", stem, index, ext))
			if _, err := m.stat(serial, candidate); errors.Is(err, ErrNotFound) {
				return candidate, nil
			}
		}
		return "", errors.New("无法生成不冲突的文件名")
	case "replace":
		return rel, nil
	default:
		return "", ErrConflict
	}
}

func (m *Manager) Delete(serial string, paths []string) (string, error) {
	if len(paths) == 0 || len(paths) > 100 {
		return "", ErrInvalid
	}
	token := fmt.Sprintf("%d", time.Now().UnixNano())
	trashRoot := path.Join(trashDirName, token)
	if _, err := m.shell(serial, "mkdir -p "+shellQuote(remotePath(trashRoot))); err != nil {
		return "", mapRemoteError(err)
	}
	record := trashRecord{}
	rollback := func() {
		for index := len(record.items) - 1; index >= 0; index-- {
			item := record.items[index]
			m.shell(serial, fmt.Sprintf("mv %s %s", shellQuote(remotePath(item.stored)), shellQuote(remotePath(item.original))))
		}
		m.remove(serial, trashRoot)
	}
	for index, raw := range paths {
		rel, err := cleanRel(raw)
		if err != nil || rel == "" {
			rollback()
			return "", ErrInvalid
		}
		if _, err := m.stat(serial, rel); err != nil {
			rollback()
			return "", err
		}
		stored := path.Join(trashRoot, fmt.Sprintf("%03d-%s", index, path.Base(rel)))
		if _, err := m.shell(serial, fmt.Sprintf("mv %s %s", shellQuote(remotePath(rel)), shellQuote(remotePath(stored)))); err != nil {
			rollback()
			return "", mapRemoteError(err)
		}
		record.items = append(record.items, trashItem{original: rel, stored: stored})
	}
	m.trashMu.Lock()
	m.trash[serial+"\x00"+token] = record
	m.trashMu.Unlock()
	return token, nil
}

func (m *Manager) Undo(serial, token string) error {
	if token == "" {
		return ErrInvalid
	}
	key := serial + "\x00" + token
	m.trashMu.Lock()
	record, ok := m.trash[key]
	m.trashMu.Unlock()
	if !ok {
		return ErrNotFound
	}
	for _, item := range record.items {
		if _, err := m.stat(serial, item.original); err == nil {
			return ErrConflict
		}
	}
	for _, item := range record.items {
		if _, err := m.shell(serial, "mkdir -p "+shellQuote(remotePath(path.Dir(item.original)))); err != nil {
			return mapRemoteError(err)
		}
		if _, err := m.shell(serial, fmt.Sprintf("mv %s %s", shellQuote(remotePath(item.stored)), shellQuote(remotePath(item.original)))); err != nil {
			return mapRemoteError(err)
		}
	}
	if err := m.remove(serial, path.Join(trashDirName, token)); err != nil {
		return err
	}
	m.trashMu.Lock()
	delete(m.trash, key)
	m.trashMu.Unlock()
	return nil
}

func (m *Manager) remove(serial, rel string) error {
	if _, err := m.shell(serial, "rm -rf "+shellQuote(remotePath(rel))); err != nil {
		return mapRemoteError(err)
	}
	return nil
}

func (m *Manager) shell(serial, script string) ([]byte, error) { return m.run(serial, "shell", script) }

func mapRemoteError(err error) error {
	if err == nil {
		return nil
	}
	message := strings.ToLower(err.Error())
	if strings.Contains(message, "no devices/emulators found") || strings.Contains(message, "device") && (strings.Contains(message, "not found") || strings.Contains(message, "offline") || strings.Contains(message, "unauthorized")) {
		return ErrDeviceUnavailable
	}
	if strings.Contains(message, "no such file") || strings.Contains(message, "not found") || strings.Contains(message, "does not exist") {
		return ErrNotFound
	}
	if strings.Contains(message, "file exists") || strings.Contains(message, "already exists") {
		return ErrConflict
	}
	return err
}

func remotePath(rel string) string {
	if rel == "" {
		return androidRoot
	}
	return androidRoot + "/" + rel
}

func shellQuote(value string) string { return "'" + strings.ReplaceAll(value, "'", "'\\''") + "'" }

func kindFor(isDir bool, name string) string {
	if isDir {
		return "folder"
	}
	switch strings.ToLower(path.Ext(name)) {
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

func mimeFor(isDir bool, name string) string {
	if isDir {
		return "inode/directory"
	}
	if value := mime.TypeByExtension(strings.ToLower(path.Ext(name))); value != "" {
		return value
	}
	return "application/octet-stream"
}

func sortItems(items []FileItem, sortBy, order string) {
	desc := strings.EqualFold(order, "desc")
	sort.SliceStable(items, func(i, j int) bool {
		a, b := items[i], items[j]
		compare := 0
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
	result := []Breadcrumb{{Name: "手机存储", Path: ""}}
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

func cleanRel(raw string) (string, error) {
	if raw == "" {
		return "", nil
	}
	if strings.ContainsRune(raw, '\x00') || strings.Contains(raw, "\\") || strings.HasPrefix(raw, "/") || strings.HasPrefix(raw, "~") {
		return "", ErrForbidden
	}
	parts := strings.Split(raw, "/")
	cleaned := make([]string, 0, len(parts))
	for _, part := range parts {
		switch part {
		case "", ".":
		case "..", trashDirName:
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
