package dependencies

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"github.com/gofrs/flock"
	"io"
	"log"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"time"
)

type downloader interface {
	Do(*http.Request) (*http.Response, error)
}

func defaultDownloader() downloader {
	return &http.Client{Timeout: 10 * time.Minute, CheckRedirect: func(r *http.Request, via []*http.Request) error {
		if len(via) > 8 || !trustedURL(r.URL) {
			return fmt.Errorf("拒绝不受信任的下载重定向")
		}
		return nil
	}}
}
func trustedURL(u *url.URL) bool {
	if u.Scheme != "https" || u.User != nil || u.Port() != "" && u.Port() != "443" {
		return false
	}
	for _, host := range []string{"github.com", "release-assets.githubusercontent.com", "objects.githubusercontent.com", "dl.google.com"} {
		if u.Hostname() == host {
			return true
		}
	}
	return false
}
func (m *Manager) setTask(id string, t InstallTask) { m.mu.Lock(); m.tasks[id] = t; m.mu.Unlock() }
func (m *Manager) Install(id string, accepted bool) error {
	guard, err := m.GuardMutation()
	if err != nil {
		return err
	}
	guardHandedOff := false
	defer func() {
		if !guardHandedOff {
			guard()
		}
	}()
	if !knownID(id) {
		return fmt.Errorf("未知依赖")
	}
	if !m.supported(id) {
		return fmt.Errorf("此平台不支持内置安装，请配置系统依赖")
	}
	if id == "adb" && !accepted {
		return fmt.Errorf("请先阅读并同意 Android SDK 许可协议")
	}
	if err := safeDirectory(m.root); err != nil {
		return err
	}
	jobLock := flock.New(filepath.Join(m.root, ".install-lock-"+id))
	locked, e := jobLock.TryLock()
	if e != nil || !locked {
		return fmt.Errorf("此依赖正在安装或清理")
	}
	handedOff := false
	defer func() {
		if !handedOff {
			_ = jobLock.Unlock()
		}
	}()
	m.mu.Lock()
	if _, ok := m.cancels[id]; ok || m.tasks[id].Stage == "removing" {
		m.mu.Unlock()
		return fmt.Errorf("依赖安装正在进行")
	}
	ctx, cancel := context.WithCancel(context.Background())
	m.cancels[id] = cancel
	m.tasks[id] = InstallTask{Stage: "downloading"}
	m.jobs.Add(1)
	m.mu.Unlock()
	handedOff = true
	guardHandedOff = true
	go func() {
		defer m.jobs.Done()
		defer guard()
		defer jobLock.Unlock()
		log.Printf("[Dependencies] 安装开始 id=%s", id)
		err := m.install(ctx, id)
		m.mu.Lock()
		delete(m.cancels, id)
		if err != nil {
			stage := "failed"
			if ctx.Err() != nil {
				stage = "cancelled"
			}
			m.tasks[id] = InstallTask{Stage: stage, Error: err.Error()}
		} else {
			m.tasks[id] = InstallTask{Stage: "ready"}
		}
		m.mu.Unlock()
		cancel()
		log.Printf("[Dependencies] 安装结束 id=%s error=%v", id, err)
	}()
	return nil
}
func (m *Manager) Cancel(id string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	cancel, ok := m.cancels[id]
	if !ok {
		return fmt.Errorf("没有正在运行的安装")
	}
	cancel()
	return nil
}
func (m *Manager) install(ctx context.Context, id string) error {
	c, artifacts, err := m.spec(id)
	if err != nil {
		return err
	}
	if err := safeDirectory(m.root); err != nil {
		return err
	}
	temp, err := os.MkdirTemp(m.root, ".install-"+id+"-")
	if err != nil {
		return err
	}
	defer os.RemoveAll(temp)
	payload := filepath.Join(temp, "files")
	if err := os.Mkdir(payload, 0700); err != nil {
		return err
	}
	rec := installation{Schema: 1, ID: id, Version: c.Version, Platform: platform(), Files: map[string]installedFile{}}
	// Stable ordering makes progress and installation logs repeatable.
	names := []string{"pi", "fd", "rg"}
	if id == "adb" {
		names = []string{"adb"}
	}
	for _, name := range names {
		spec, ok := artifacts[name]
		if !ok {
			return fmt.Errorf("安装清单缺少 %s", name)
		}
		archive := filepath.Join(temp, name+".download")
		if err := m.download(ctx, id, spec, archive); err != nil {
			return err
		}
		m.setTask(id, InstallTask{Stage: "verifying"})
		if err := extractArchive(ctx, archive, spec.URL, payload, id, name, rec.Files); err != nil {
			return err
		}
	}
	if err := ctx.Err(); err != nil {
		return err
	}
	if id == "pi-agent" {
		raw, e := catalogFS.ReadFile("catalog/PI-LICENSE")
		if e != nil {
			return e
		}
		name := "third-party/PI-LICENSE"
		p := filepath.Join(payload, filepath.FromSlash(name))
		if e := os.MkdirAll(filepath.Dir(p), 0700); e != nil {
			return e
		}
		if e := os.WriteFile(p, raw, 0644); e != nil {
			return e
		}
		rec.Files[name] = installedFile{SHA256: fmt.Sprintf("%x", sha256.Sum256(raw)), Mode: 0644}
	}
	if err := writeJSON(filepath.Join(payload, "installation.json"), rec); err != nil {
		return err
	}
	if _, err := verifyInstallation(payload, id, true); err != nil {
		return err
	}
	// Probe the staged binaries before publishing; never register a corrupt package.
	for name, args := range map[string][]string{exeName("adb"): {"version"}} {
		if id == "pi-agent" {
			break
		}
		out, e := probe(ctx, filepath.Join(payload, name), args...)
		if e != nil || !strings.HasPrefix(out, "Android Debug Bridge version ") {
			return fmt.Errorf("下载的 ADB 无法运行: %v", e)
		}
	}
	if id == "pi-agent" {
		for _, name := range []string{exeName("pi"), "bin/" + exeName("fd"), "bin/" + exeName("rg")} {
			if _, e := probe(ctx, filepath.Join(payload, name), "--version"); e != nil {
				return e
			}
		}
	}
	m.setTask(id, InstallTask{Stage: "publishing"})
	release, err := m.exclusive([]string{id}, id == "adb" && m.Settings().ADB.Source == "builtin")
	if err != nil {
		return err
	}
	defer release()
	if err := ctx.Err(); err != nil {
		return err
	}
	dir, err := m.ownedDir(id)
	if err != nil {
		return err
	}
	backup := filepath.Join(temp, "old")
	if _, err := os.Lstat(dir); err == nil {
		if err := os.Rename(dir, backup); err != nil {
			return err
		}
	}
	if err := os.Rename(payload, dir); err != nil {
		if _, e := os.Stat(backup); e == nil {
			_ = os.Rename(backup, dir)
		}
		return err
	}
	log.Printf("[Dependencies] 已安装 id=%s version=%s", id, c.Version)
	return nil
}
func (m *Manager) download(ctx context.Context, id string, spec artifact, path string) error {
	u, err := url.Parse(spec.URL)
	if err != nil || !trustedURL(u) {
		return fmt.Errorf("下载来源无效")
	}
	digest, err := hex.DecodeString(spec.SHA256)
	if err != nil || len(digest) != sha256.Size {
		return fmt.Errorf("下载校验信息无效")
	}
	const max = 256 << 20
	for attempt := 0; attempt < 3; attempt++ {
		start := int64(0)
		if info, e := os.Stat(path); e == nil {
			start = info.Size()
		}
		req, e := http.NewRequestWithContext(ctx, "GET", spec.URL, nil)
		if e != nil {
			return e
		}
		req.Header.Set("User-Agent", "MyWebScrcpy-dependencies")
		if start > 0 {
			req.Header.Set("Range", fmt.Sprintf("bytes=%d-", start))
		}
		response, e := m.client.Do(req)
		if e != nil {
			if ctx.Err() != nil {
				return ctx.Err()
			}
			continue
		}
		if response.StatusCode == 200 {
			start = 0
		} else if response.StatusCode != 206 || start == 0 || !strings.HasPrefix(response.Header.Get("Content-Range"), fmt.Sprintf("bytes %d-", start)) {
			response.Body.Close()
			return fmt.Errorf("依赖下载响应错误: HTTP %d", response.StatusCode)
		}
		total := int64(0)
		if response.ContentLength > 0 {
			total = start + response.ContentLength
		}
		if total > max || spec.Size > 0 && total > 0 && total != spec.Size {
			response.Body.Close()
			return fmt.Errorf("下载文件大小不符合清单")
		}
		flags := os.O_CREATE | os.O_WRONLY
		if start > 0 {
			flags |= os.O_APPEND
		} else {
			flags |= os.O_TRUNC
		}
		file, e := os.OpenFile(path, flags, 0600)
		if e != nil {
			response.Body.Close()
			return e
		}
		t := InstallTask{Stage: "downloading", Received: start, Total: total}
		buf := make([]byte, 128*1024)
		last := time.Time{}
		for {
			n, readErr := response.Body.Read(buf)
			if n > 0 {
				t.Received += int64(n)
				if t.Received > max {
					e = fmt.Errorf("下载文件超过限制")
					break
				}
				if _, e = file.Write(buf[:n]); e != nil {
					break
				}
				if time.Since(last) > 150*time.Millisecond {
					m.setTask(id, t)
					last = time.Now()
				}
			}
			if readErr != nil {
				if readErr != io.EOF {
					e = readErr
				}
				break
			}
		}
		closeErr := file.Close()
		response.Body.Close()
		if e == nil {
			e = closeErr
		}
		if ctx.Err() != nil {
			return ctx.Err()
		}
		if e != nil || total > 0 && t.Received != total {
			if attempt < 2 {
				continue
			}
			return fmt.Errorf("依赖下载不完整: %v", e)
		}
		if spec.Size > 0 && t.Received != spec.Size {
			return fmt.Errorf("依赖下载大小校验失败")
		}
		actual, e := fileDigest(path)
		if e != nil {
			return e
		}
		if actual != spec.SHA256 {
			return fmt.Errorf("依赖 SHA-256 校验失败")
		}
		m.setTask(id, t)
		return nil
	}
	return fmt.Errorf("无法下载依赖，请检查网络后重试")
}
func (m *Manager) Uninstall(id string) error {
	guard, err := m.GuardMutation()
	if err != nil {
		return err
	}
	defer guard()
	if !knownID(id) {
		return fmt.Errorf("未知依赖")
	}
	if err := safeDirectory(m.root); err != nil {
		return err
	}
	jobLock := flock.New(filepath.Join(m.root, ".install-lock-"+id))
	locked, e := jobLock.TryLock()
	if e != nil || !locked {
		return fmt.Errorf("此依赖正在安装或清理")
	}
	defer jobLock.Unlock()
	m.mu.Lock()
	if _, ok := m.cancels[id]; ok || m.tasks[id].Stage == "removing" {
		m.mu.Unlock()
		return fmt.Errorf("依赖安装正在进行")
	}
	m.tasks[id] = InstallTask{Stage: "removing"}
	m.mu.Unlock()
	err = m.remove(id)
	if err != nil {
		m.setTask(id, InstallTask{Stage: "failed", Error: err.Error()})
	} else {
		m.setTask(id, InstallTask{Stage: "idle"})
	}
	log.Printf("[Dependencies] 卸载 id=%s error=%v", id, err)
	return err
}
func (m *Manager) remove(id string) error {
	release, err := m.exclusive([]string{id}, id == "adb" && m.Settings().ADB.Source == "builtin")
	if err != nil {
		return err
	}
	defer release()
	dir, err := m.ownedDir(id)
	if err != nil {
		return err
	}
	leftovers, e := filepath.Glob(filepath.Join(m.root, ".install-"+id+"-*"))
	if e != nil {
		return e
	}
	for _, p := range leftovers {
		if e := safeDirectory(p); e != nil {
			return e
		}
		if e := os.RemoveAll(p); e != nil {
			return e
		}
	}
	// Only the fixed app-owned directory is deleted. Configured executable paths
	// and project skills never enter this function. Tombstones remain retryable.
	tomb := filepath.Join(m.root, ".remove-"+id)
	if _, e := os.Lstat(tomb); e == nil {
		if e := safeDirectory(tomb); e != nil {
			return e
		}
		if e := os.RemoveAll(tomb); e != nil {
			return e
		}
	}
	if _, e := os.Lstat(dir); os.IsNotExist(e) {
		return nil
	}
	if err := os.Rename(dir, tomb); err != nil {
		return err
	}
	if err := os.RemoveAll(tomb); err != nil {
		return fmt.Errorf("依赖清理失败，请重试: %w", err)
	}
	return nil
}
