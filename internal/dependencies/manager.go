package dependencies

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"io/fs"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"sync"

	"github.com/gofrs/flock"
)

type Selection struct {
	Source string `json:"source"`
	Path   string `json:"path"`
}
type Settings struct {
	Schema int       `json:"schema"`
	ADB    Selection `json:"adb"`
	Python Selection `json:"python"`
}
type installedFile struct {
	SHA256 string      `json:"sha256"`
	Mode   fs.FileMode `json:"mode"`
}
type installation struct {
	Schema   int                      `json:"schema"`
	ID       string                   `json:"id"`
	Platform string                   `json:"platform"`
	Version  string                   `json:"version"`
	Files    map[string]installedFile `json:"files"`
}
type InstallTask struct {
	Stage    string `json:"stage"`
	Received int64  `json:"received"`
	Total    int64  `json:"total"`
	Error    string `json:"error"`
}
type ManagedTool struct {
	ID          string       `json:"id"`
	Name        string       `json:"name"`
	Selection   Selection    `json:"selection"`
	Installed   bool         `json:"installed"`
	Residual    bool         `json:"residual"`
	Version     string       `json:"version"`
	BuiltinPath string       `json:"builtin_path,omitempty"`
	Supported   bool         `json:"supported"`
	Effective   Tool         `json:"effective"`
	Task        InstallTask  `json:"task"`
	ModulesTask *InstallTask `json:"modules_task,omitempty"`
}
type Manager struct {
	operations sync.RWMutex
	uses       map[string]*sync.RWMutex
	jobs       sync.WaitGroup
	root       string
	mu         sync.RWMutex
	settings   Settings
	tasks      map[string]InstallTask
	cancels    map[string]func()
	// The host freezes HTTP/device/task work before changing ADB; cleanup releases it.
	BeforeADBChange func() (func(), error)
	BeforeMutation  func() error
	client          downloader
}

var active *Manager

func Initialize(dataDir string) (*Manager, error) {
	m, err := NewManager(dataDir)
	if err == nil {
		active = m
	}
	return m, err
}
func NewManager(dataDir string) (*Manager, error) {
	base, err := filepath.Abs(dataDir)
	if err != nil {
		return nil, err
	}
	base, err = filepath.EvalSymlinks(base)
	if err != nil {
		return nil, err
	}
	root := filepath.Join(base, "dependencies")
	if err := os.MkdirAll(root, 0700); err != nil {
		return nil, err
	}
	resolved, err := filepath.EvalSymlinks(root)
	if err != nil {
		return nil, err
	}
	if resolved != root {
		return nil, fmt.Errorf("依赖目录不能包含符号链接")
	}
	m := &Manager{root: root, tasks: map[string]InstallTask{}, cancels: map[string]func(){}, uses: map[string]*sync.RWMutex{"adb": {}, "python": {}}, client: defaultDownloader()}
	m.settings = Settings{Schema: 2, ADB: Selection{Source: "system", Path: Current().ADB}, Python: Selection{Source: "system", Path: Current().Python}}
	raw, err := os.ReadFile(filepath.Join(root, "config.json"))
	if err == nil {
		var s Settings
		if err := strictJSON(raw, &s); err == nil && s.Schema == 2 {
			if err := validateShape(s); err != nil {
				return nil, err
			}
			m.settings = s
		} else {
			var header struct {
				Schema int `json:"schema"`
			}
			if err := json.Unmarshal(raw, &header); err != nil {
				return nil, fmt.Errorf("依赖配置解析失败: %w", err)
			}
			if header.Schema != 1 {
				return nil, fmt.Errorf("不支持依赖配置版本 %d", header.Schema)
			}
			// Early-stage upgrade: reset application-owned dependency selections.
			// No Pi configuration or old selection is migrated or archived.
			if err := writeJSON(filepath.Join(root, "config.json"), m.settings); err != nil {
				return nil, err
			}
			log.Print("[Dependencies] 已重置旧版依赖配置，仅保留 ADB 和 Python 设置项")
		}
	} else if !os.IsNotExist(err) {
		return nil, err
	}
	return m, nil
}
func strictJSON(raw []byte, value any) error {
	d := json.NewDecoder(strings.NewReader(string(raw)))
	d.DisallowUnknownFields()
	if err := d.Decode(value); err != nil {
		return err
	}
	if err := d.Decode(new(any)); err != io.EOF {
		return fmt.Errorf("配置只能包含一个 JSON 对象")
	}
	return nil
}
func validateShape(s Settings) error {
	if s.Schema != 2 {
		return fmt.Errorf("不支持依赖配置版本 %d", s.Schema)
	}
	for _, v := range []Selection{s.ADB, s.Python} {
		if v.Source != "builtin" && v.Source != "system" {
			return fmt.Errorf("依赖来源必须为内置或系统")
		}
		if v.Path != "" && !filepath.IsAbs(v.Path) {
			return fmt.Errorf("依赖必须使用绝对路径")
		}
	}
	return nil
}
func (m *Manager) Settings() Settings { m.mu.RLock(); defer m.mu.RUnlock(); return m.settings }
func exeName(name string) string {
	if runtime.GOOS == "windows" {
		return name + ".exe"
	}
	return name
}
func knownID(id string) bool { return id == "adb" || id == "python" }
func (m *Manager) ownedDir(id string) (string, error) {
	if !knownID(id) {
		return "", fmt.Errorf("未知依赖")
	}
	p := filepath.Join(m.root, id)
	if err := safeDirectory(m.root); err != nil {
		return "", err
	}
	if _, err := os.Lstat(p); os.IsNotExist(err) {
		return p, nil
	}
	if err := safeDirectory(p); err != nil {
		return "", err
	}
	return p, nil
}
func safeDirectory(p string) error {
	resolved, err := filepath.EvalSymlinks(p)
	if err != nil {
		return err
	}
	if resolved != p {
		return fmt.Errorf("拒绝访问含符号链接的依赖目录")
	}
	info, err := os.Lstat(p)
	if err != nil {
		return err
	}
	if !info.IsDir() {
		return fmt.Errorf("依赖目录无效")
	}
	return nil
}
func (m *Manager) installation(id string, hashes bool) (installation, error) {
	dir, err := m.ownedDir(id)
	if err != nil {
		return installation{}, err
	}
	return verifyInstallation(dir, id, hashes)
}
func verifyInstallation(dir, id string, hashes bool) (installation, error) {
	info, e := os.Lstat(filepath.Join(dir, "installation.json"))
	if e != nil {
		return installation{}, e
	}
	if !info.Mode().IsRegular() {
		return installation{}, fmt.Errorf("安装清单必须是普通文件")
	}
	raw, err := os.ReadFile(filepath.Join(dir, "installation.json"))
	if err != nil {
		return installation{}, err
	}
	var rec installation
	if err := strictJSON(raw, &rec); err != nil {
		return rec, err
	}
	if rec.Schema != 1 || rec.ID != id || rec.Platform != runtime.GOOS+"/"+runtime.GOARCH || len(rec.Files) == 0 {
		return rec, fmt.Errorf("依赖安装清单无效")
	}
	required := []string{exeName("adb")}

	if id == "python" {
		required = []string{pythonRelativePath()}
	}
	for _, name := range required {
		if _, ok := rec.Files[name]; !ok {
			return rec, fmt.Errorf("依赖缺少 %s", name)
		}
	}
	for name, f := range rec.Files {
		h, e := hex.DecodeString(f.SHA256)
		if !fs.ValidPath(name) || strings.ContainsAny(name, "\\:") || name == "." || e != nil || len(h) != sha256.Size || f.Mode != 0644 && f.Mode != 0755 {
			return rec, fmt.Errorf("依赖文件记录无效")
		}
		p := filepath.Join(dir, filepath.FromSlash(name))
		real, e := filepath.EvalSymlinks(p)
		if e != nil {
			return rec, e
		}
		if real != p {
			return rec, fmt.Errorf("依赖文件不能包含符号链接")
		}
		info, e := os.Lstat(p)
		if e != nil {
			return rec, e
		}
		if !info.Mode().IsRegular() {
			return rec, fmt.Errorf("依赖文件不是普通文件")
		}
		if runtime.GOOS != "windows" && info.Mode().Perm() != f.Mode {
			return rec, fmt.Errorf("依赖文件权限错误")
		}
		if hashes {
			actual, e := fileDigest(p)
			if e != nil {
				return rec, e
			}
			if actual != f.SHA256 {
				return rec, fmt.Errorf("依赖文件校验失败: %s", name)
			}
		}
	}
	return rec, nil
}
func fileDigest(p string) (string, error) {
	f, e := os.Open(p)
	if e != nil {
		return "", e
	}
	defer f.Close()
	h := sha256.New()
	_, e = io.Copy(h, f)
	return hex.EncodeToString(h.Sum(nil)), e
}
func pathFor(configured, name string) (string, error) {
	if configured != "" {
		return checkExecutable(configured)
	}
	p, err := exec.LookPath(name)
	if err != nil {
		return "", fmt.Errorf("未在 PATH 中找到 %s，请配置执行路径", name)
	}
	return p, nil
}
func (m *Manager) resolveADB(s Selection) (string, error) {
	if s.Source == "builtin" {
		if _, e := m.installation("adb", false); e != nil {
			return "", fmt.Errorf("请安装内置 ADB: %w", e)
		}
		return checkExecutable(filepath.Join(m.root, "adb", exeName("adb")))
	}
	// Preserve explicit environment and ANDROID_HOME behavior from the previous release.
	if p := chosenPath("MYWEBSCRCPY_ADB", s.Path); p != "" {
		return checkExecutable(p)
	}
	if home := os.Getenv("ANDROID_HOME"); home != "" {
		if p, e := exec.LookPath(filepath.Join(home, "platform-tools", exeName("adb"))); e == nil {
			return p, nil
		}
	}
	p, err := exec.LookPath("adb")
	if err != nil {
		return "", fmt.Errorf("未在 PATH 中找到 adb，请配置执行路径")
	}
	return p, nil
}
func (m *Manager) acquire(id string) (func(), error) {
	if !knownID(id) {
		return nil, fmt.Errorf("未知依赖")
	}
	if err := safeDirectory(m.root); err != nil {
		return nil, err
	}
	if !m.uses[id].TryRLock() {
		return nil, fmt.Errorf("依赖正在修改，请稍后重试")
	}
	lock := flock.New(filepath.Join(m.root, ".lock-"+id))
	ok, e := lock.TryRLock()
	if e != nil || !ok {
		m.uses[id].RUnlock()
		return nil, fmt.Errorf("依赖正在修改，请稍后重试")
	}
	return func() { _ = lock.Unlock(); m.uses[id].RUnlock() }, nil
}
func (m *Manager) exclusive(ids []string, adb bool) (func(), error) {
	if err := safeDirectory(m.root); err != nil {
		return nil, err
	}
	cleanup := func() {}
	if adb && m.BeforeADBChange != nil {
		var err error
		cleanup, err = m.BeforeADBChange()
		if err != nil {
			return nil, err
		}
	}
	locks := []*flock.Flock{}
	held := []string{}
	release := func() {
		for i := len(locks) - 1; i >= 0; i-- {
			_ = locks[i].Unlock()
		}
		for i := len(held) - 1; i >= 0; i-- {
			m.uses[held[i]].Unlock()
		}
		cleanup()
	}
	for _, id := range ids {
		if !knownID(id) || !m.uses[id].TryLock() {
			release()
			return nil, fmt.Errorf("依赖正在使用，请稍后重试")
		}
		held = append(held, id)
		lock := flock.New(filepath.Join(m.root, ".lock-"+id))
		ok, err := lock.TryLock()
		if err != nil || !ok {
			release()
			return nil, fmt.Errorf("依赖正在使用，请稍后重试")
		}
		locks = append(locks, lock)
	}
	return release, nil
}
func (m *Manager) Apply(s Settings) error {
	guard, err := m.GuardMutation()
	if err != nil {
		return err
	}
	defer guard()
	if err := validateShape(s); err != nil {
		return err
	}
	m.mu.RLock()
	busy := len(m.cancels) > 0
	m.mu.RUnlock()
	if busy {
		return fmt.Errorf("请等待依赖安装完成")
	}
	release, err := m.exclusive([]string{"adb", "python"}, s.ADB != m.Settings().ADB)
	if err != nil {
		return err
	}
	defer release()
	previous := m.Settings()
	// Validate changed selections. Unavailable optional tools must not prevent
	// saving a usable Python environment or another independent dependency.
	for _, item := range []struct {
		id        string
		next, old Selection
	}{{"adb", s.ADB, previous.ADB}, {"python", s.Python, previous.Python}} {
		if item.next != item.old {
			if tool := m.detect(item.id, item.next); !tool.Installed {
				return fmt.Errorf("%s: %s", item.id, tool.Error)
			}
		}
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	if err := writeJSON(filepath.Join(m.root, "config.json"), s); err != nil {
		return err
	}
	m.settings = s
	log.Print("[Dependencies] 已保存依赖来源，新操作使用新配置")
	return nil
}
func writeJSON(p string, value any) error {
	raw, e := json.MarshalIndent(value, "", "  ")
	if e != nil {
		return e
	}
	f, e := os.CreateTemp(filepath.Dir(p), ".config-*")
	if e != nil {
		return e
	}
	defer os.Remove(f.Name())
	if _, e = f.Write(raw); e != nil {
		f.Close()
		return e
	}
	if e = f.Sync(); e != nil {
		f.Close()
		return e
	}
	if e = f.Close(); e != nil {
		return e
	}
	return os.Rename(f.Name(), p)
}
func (m *Manager) Close() {
	m.mu.Lock()
	for _, cancel := range m.cancels {
		cancel()
	}
	m.mu.Unlock()
	m.jobs.Wait()
}

func (m *Manager) CheckIdle() error {
	m.mu.RLock()
	defer m.mu.RUnlock()
	if len(m.cancels) > 0 {
		return fmt.Errorf("请等待依赖安装或取消完成")
	}
	for _, task := range m.tasks {
		if task.Stage == "removing" {
			return fmt.Errorf("请等待依赖卸载完成")
		}
	}
	return nil
}

// BeginMaintenance freezes all dependency mutations for application updates.
func (m *Manager) BeginMaintenance() (func(), error) {
	if !m.operations.TryLock() {
		return nil, fmt.Errorf("请等待依赖安装、配置或卸载完成")
	}
	return m.operations.Unlock, nil
}
func (m *Manager) GuardMutation() (func(), error) {
	if !m.operations.TryRLock() {
		return nil, fmt.Errorf("更新正在进行，请稍后修改依赖")
	}
	if m.BeforeMutation != nil {
		if err := m.BeforeMutation(); err != nil {
			m.operations.RUnlock()
			return nil, err
		}
	}
	return m.operations.RUnlock, nil
}
