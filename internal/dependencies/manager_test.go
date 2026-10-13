package dependencies

import (
	"archive/zip"
	"bytes"
	"context"
	"crypto/sha256"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

type downloadFunc func(*http.Request) (*http.Response, error)

func (f downloadFunc) Do(r *http.Request) (*http.Response, error) { return f(r) }
func newTestManager(t *testing.T) *Manager {
	t.Helper()
	root := t.TempDir()
	m, e := NewManager(root, filepath.Join(root, ".pi", "skills"))
	if e != nil {
		t.Fatal(e)
	}
	t.Cleanup(m.Close)
	return m
}
func TestManagedSourceNeverFallsBack(t *testing.T) {
	m := newTestManager(t)
	m.settings.ADB = Selection{Source: "builtin"}
	t.Setenv("MYWEBSCRCPY_ADB", os.Args[0])
	if _, e := m.resolveADB(m.settings.ADB); e == nil {
		t.Fatal("missing built-in ADB fell back to system")
	}
	if _, e := m.resolvePi(Selection{Source: "system", Path: filepath.Join(t.TempDir(), "missing-pi")}); e == nil {
		t.Fatal("missing selected Pi accepted")
	}
	original := m.Settings()
	invalid := original
	invalid.ADB.Path = "relative"
	if e := m.Apply(invalid); e == nil {
		t.Fatal("relative path accepted")
	}
	if m.Settings() != original {
		t.Fatal("invalid config changed runtime")
	}
}
func TestUninstallRejectsLinksAndProtectsExternalFiles(t *testing.T) {
	m := newTestManager(t)
	external := t.TempDir()
	sentinel := filepath.Join(external, "adb")
	if e := os.WriteFile(sentinel, []byte("system executable"), 0600); e != nil {
		t.Fatal(e)
	}
	if e := os.Symlink(external, filepath.Join(m.root, "adb")); e != nil {
		t.Skip(e)
	}
	if e := m.Uninstall("adb"); e == nil {
		t.Fatal("symlink uninstall allowed")
	}
	if data, e := os.ReadFile(sentinel); e != nil || string(data) != "system executable" {
		t.Fatal("system file changed")
	}
	if e := m.Uninstall("../outside"); e == nil {
		t.Fatal("unknown ID accepted")
	}
	os.Remove(filepath.Join(m.root, "adb"))
	os.Mkdir(filepath.Join(m.root, "adb"), 0700)
	os.WriteFile(filepath.Join(m.root, "adb", "owned"), []byte("owned"), 0600)
	release, e := m.acquire("adb")
	if e != nil {
		t.Fatal(e)
	}
	if e := m.Uninstall("adb"); e == nil {
		t.Fatal("busy dependency deleted")
	}
	release()
	if e := m.Uninstall("adb"); e != nil {
		t.Fatal(e)
	}
	if _, e := os.Stat(filepath.Join(m.root, "adb")); !os.IsNotExist(e) {
		t.Fatal("owned files remain")
	}
	if _, e := os.Stat(sentinel); e != nil {
		t.Fatal(e)
	}
}
func TestArchiveRejectsEscapeLinksAndDuplicates(t *testing.T) {
	for _, test := range []string{"../escape", "platform-tools/../escape", "platform-tools/link", "duplicate"} {
		t.Run(test, func(t *testing.T) {
			m := newTestManager(t)
			var b bytes.Buffer
			z := zip.NewWriter(&b)
			name := test
			if test == "duplicate" {
				name = "platform-tools/adb"
			}
			h := &zip.FileHeader{Name: name}
			h.SetMode(0644)
			if test == "platform-tools/link" {
				h.SetMode(os.ModeSymlink | 0777)
			}
			f, _ := z.CreateHeader(h)
			f.Write([]byte("bad"))
			if test == "duplicate" {
				duplicate := &zip.FileHeader{Name: name}
				duplicate.SetMode(0644)
				f, _ = z.CreateHeader(duplicate)
				f.Write([]byte("bad"))
			}
			z.Close()
			archive := filepath.Join(t.TempDir(), "test.zip")
			os.WriteFile(archive, b.Bytes(), 0600)
			if e := extractArchive(context.Background(), archive, "https://dl.google.com/test.zip", m.root, "adb", "adb", map[string]installedFile{}); e == nil {
				t.Fatal("unsafe archive accepted")
			}
		})
	}
}
func TestDownloadResumeHashAndSize(t *testing.T) {
	body := []byte(strings.Repeat("verified bytes", 1000))
	sum := fmt.Sprintf("%x", sha256.Sum256(body))
	spec := artifact{URL: "https://dl.google.com/test.zip", SHA256: sum, Size: int64(len(body))}
	m := newTestManager(t)
	target := filepath.Join(t.TempDir(), "archive")
	os.WriteFile(target, body[:13], 0600)
	m.client = downloadFunc(func(r *http.Request) (*http.Response, error) {
		if r.Header.Get("Range") != "bytes=13-" {
			t.Fatalf("missing resume range")
		}
		return &http.Response{StatusCode: 206, Header: http.Header{"Content-Range": {fmt.Sprintf("bytes 13-%d/%d", len(body)-1, len(body))}}, ContentLength: int64(len(body) - 13), Body: io.NopCloser(bytes.NewReader(body[13:]))}, nil
	})
	if e := m.download(context.Background(), "adb", spec, target); e != nil {
		t.Fatal(e)
	}
	spec.SHA256 = strings.Repeat("0", 64)
	os.Remove(target)
	m.client = downloadFunc(func(r *http.Request) (*http.Response, error) {
		return &http.Response{StatusCode: 200, Header: http.Header{}, ContentLength: int64(len(body)), Body: io.NopCloser(bytes.NewReader(body))}, nil
	})
	if e := m.download(context.Background(), "adb", spec, target); e == nil {
		t.Fatal("hash mismatch accepted")
	}
	os.Remove(target)
	m.client = downloadFunc(func(r *http.Request) (*http.Response, error) {
		return &http.Response{StatusCode: 200, Header: http.Header{}, ContentLength: int64(len(body)), Body: io.NopCloser(bytes.NewReader(body[:10]))}, nil
	})
	if e := m.download(context.Background(), "adb", spec, target); e == nil {
		t.Fatal("truncated download accepted")
	}
}
func TestDependencySettingsAllowAllRequestSources(t *testing.T) {
	m := newTestManager(t)
	previous := active
	active = m
	t.Cleanup(func() { active = previous })
	mux := http.NewServeMux()
	RegisterSettings(mux, func(next http.HandlerFunc) http.HandlerFunc { return next })
	for _, test := range []struct{ addr, host, origin string }{
		{"127.0.0.1:1000", "localhost:8080", "http://localhost:8080"},
		{"192.0.2.1:1000", "myhost.example", "https://frontend.example"},
		{"[2001:db8::1]:1000", "proxy.example", "https://frontend.example"},
	} {
		t.Run(test.addr, func(t *testing.T) {
			r := httptest.NewRequest("POST", "http://"+test.host+"/api/dependencies/detect", strings.NewReader(`{"id":"pi-agent","selection":{"source":"builtin","path":"","fd":"","rg":""}}`))
			r.RemoteAddr = test.addr
			r.Header.Set("Origin", test.origin)
			r.Header.Set("Sec-Fetch-Site", "cross-site")
			w := httptest.NewRecorder()
			mux.ServeHTTP(w, r)
			if w.Code != http.StatusOK {
				t.Fatalf("request source was blocked: %d %s", w.Code, w.Body.String())
			}
		})
	}
}
func TestExternalProcessLock(t *testing.T) {
	m := newTestManager(t)
	other, e := NewManager(filepath.Dir(m.root), m.skills)
	if e != nil {
		t.Fatal(e)
	}
	release, e := other.acquire("adb")
	if e != nil {
		t.Fatal(e)
	}
	if _, e := m.exclusive([]string{"adb"}, false); e == nil {
		t.Fatal("another manager lease ignored")
	}
	release()
	release, e = m.exclusive([]string{"adb"}, false)
	if e != nil {
		t.Fatal(e)
	}
	release()
}
func TestRealManagedPiFromLockedArchives(t *testing.T) {
	cache := os.Getenv("MYWEBSCRCPY_PI_ARCHIVE_CACHE")
	if cache == "" {
		t.Skip("set MYWEBSCRCPY_PI_ARCHIVE_CACHE for authentic upstream installation")
	}
	m := newTestManager(t)
	m.client = downloadFunc(func(r *http.Request) (*http.Response, error) {
		raw, e := os.ReadFile(filepath.Join(cache, filepath.Base(r.URL.Path)))
		if e != nil {
			return nil, e
		}
		return &http.Response{StatusCode: 200, Header: http.Header{}, ContentLength: int64(len(raw)), Body: io.NopCloser(bytes.NewReader(raw))}, nil
	})
	if e := m.install(context.Background(), "pi-agent"); e != nil {
		t.Fatal(e)
	}
	if _, e := m.installation("pi-agent", true); e != nil {
		t.Fatal(e)
	}
	detected := m.detect("pi-agent", Selection{Source: "builtin"})
	if !detected.Installed {
		t.Fatal(detected.Error)
	}
	p, release, e := m.AcquirePi()
	if e != nil {
		t.Fatal(e)
	}
	if e := m.Uninstall("pi-agent"); e == nil {
		t.Fatal("active Pi lease ignored")
	}
	release()
	// System tools intentionally live in separate directories, not alongside Pi.
	system := Selection{Source: "system", Path: p.Executable, FD: p.FD, RG: p.RG}
	if runtime.GOOS != "windows" {
		for name, original := range map[string]string{"fd": p.FD, "rg": p.RG} {
			dest := filepath.Join(t.TempDir(), name)
			raw, err := os.ReadFile(original)
			if err != nil {
				t.Fatal(err)
			}
			if err := os.WriteFile(dest, raw, 0755); err != nil {
				t.Fatal(err)
			}
			if name == "fd" {
				system.FD = dest
			} else {
				system.RG = dest
			}
		}
		if d := m.detect("pi-agent", system); !d.Installed {
			t.Fatal(d.Error)
		}
		t.Setenv("PI_CODING_AGENT_DIR", t.TempDir())
		t.Setenv("PATH", strings.Join([]string{filepath.Dir(system.Path), filepath.Dir(system.FD), filepath.Dir(system.RG), os.Getenv("PATH")}, string(os.PathListSeparator)))
		auto := m.detect("pi-agent", Selection{Source: "system"})
		if !auto.Installed || auto.Path != system.Path || auto.SearchPaths["fd"] != system.FD || auto.SearchPaths["rg"] != system.RG {
			t.Fatalf("PATH detection failed: %+v", auto)
		}
		bin := filepath.Join(os.Getenv("PI_CODING_AGENT_DIR"), "bin")
		if err := os.Mkdir(bin, 0700); err != nil {
			t.Fatal(err)
		}
		for name, original := range map[string]string{"fd": system.FD, "rg": system.RG} {
			raw, err := os.ReadFile(original)
			if err != nil {
				t.Fatal(err)
			}
			if err := os.WriteFile(filepath.Join(bin, name), raw, 0755); err != nil {
				t.Fatal(err)
			}
		}
		t.Setenv("PATH", filepath.Dir(system.Path))
		cached := m.detect("pi-agent", Selection{Source: "system"})
		if !cached.Installed || cached.SearchPaths["fd"] != filepath.Join(bin, "fd") || cached.SearchPaths["rg"] != filepath.Join(bin, "rg") {
			t.Fatalf("Pi-managed search tool detection failed: %+v", cached)
		}
	}

	if runtime.GOOS != "windows" {
		adb := filepath.Join(t.TempDir(), "adb")
		if err := os.WriteFile(adb, []byte("#!/bin/sh\nprintf 'Android Debug Bridge version 1.0.41\\nVersion test\\n'\n"), 0755); err != nil {
			t.Fatal(err)
		}
		candidate := m.Settings()
		candidate.ADB = Selection{Source: "system", Path: adb}
		configDir := filepath.Join(m.root, "config.json")
		if err := os.Mkdir(configDir, 0700); err != nil {
			t.Fatal(err)
		}
		original := m.Settings()
		if err := m.Apply(candidate); err == nil {
			t.Fatal("failed atomic rename was reported as success")
		}
		if m.Settings() != original {
			t.Fatal("failed save changed runtime settings")
		}
		os.Remove(configDir)
		if err := m.Apply(candidate); err != nil {
			t.Fatal(err)
		}
		reloaded, err := NewManager(filepath.Dir(m.root), m.skills)
		if err != nil || reloaded.Settings() != candidate {
			t.Fatal("persisted selection did not reload", err)
		}
	}
	if e := m.Uninstall("pi-agent"); e != nil {
		t.Fatal(e)
	}
	if _, e := os.Stat(p.Executable); !os.IsNotExist(e) {
		t.Fatal("Pi was not physically removed")
	}
	if e := m.install(context.Background(), "pi-agent"); e != nil {
		t.Fatal("reinstallation failed", e)
	}
}

func TestRealManagedADBFromArchive(t *testing.T) {
	source := os.Getenv("MYWEBSCRCPY_ADB_ARCHIVE")
	if source == "" {
		t.Skip("set MYWEBSCRCPY_ADB_ARCHIVE to inspect the locked platform archive")
	}
	m := newTestManager(t)
	raw, e := os.ReadFile(source)
	if e != nil {
		t.Fatal(e)
	}
	m.client = downloadFunc(func(r *http.Request) (*http.Response, error) {
		return &http.Response{StatusCode: 200, Header: http.Header{}, ContentLength: int64(len(raw)), Body: io.NopCloser(bytes.NewReader(raw))}, nil
	})
	if e := m.Install("adb", false); e == nil {
		t.Fatal("license confirmation was not required")
	}
	if e := m.install(context.Background(), "adb"); e != nil {
		t.Fatal(e)
	}
	if _, e := m.installation("adb", true); e != nil {
		t.Fatal(e)
	}
	if tool := m.detect("adb", Selection{Source: "builtin"}); !tool.Installed {
		t.Fatal(tool.Error)
	}
	previous := active
	active = m
	t.Cleanup(func() { active = previous })
	m.settings.ADB = Selection{Source: "builtin"}
	out, e := Command(ManagedADB, "version").Output()
	if e != nil || !strings.Contains(string(out), "36.0.2") {
		t.Fatalf("managed ADB command failed: %s %v", out, e)
	}
	m.settings.ADB = Selection{Source: "system", Path: filepath.Join(t.TempDir(), "missing")}
	if _, e := Command(ManagedADB, "version").Output(); e == nil {
		t.Fatal("invalid selected path fell back to installed ADB")
	}
	if e := m.Uninstall("adb"); e != nil {
		t.Fatal(e)
	}
}

func TestMaintenancePreventsConcurrentMutation(t *testing.T) {
	m := newTestManager(t)
	guard, err := m.GuardMutation()
	if err != nil {
		t.Fatal(err)
	}
	if _, err := m.BeginMaintenance(); err == nil {
		t.Fatal("update ignored active dependency mutation")
	}
	guard()
	resume, err := m.BeginMaintenance()
	if err != nil {
		t.Fatal(err)
	}
	if guard, err := m.GuardMutation(); err == nil {
		guard()
		t.Fatal("dependency mutation allowed during update")
	}
	resume()
	guard, err = m.GuardMutation()
	if err != nil {
		t.Fatal(err)
	}
	guard()
}
func TestIndependentDependencyLeases(t *testing.T) {
	m := newTestManager(t)
	release, err := m.acquire("adb")
	if err != nil {
		t.Fatal(err)
	}
	defer release()
	done, err := m.exclusive([]string{"pi-agent"}, false)
	if err != nil {
		t.Fatal("ADB blocked independent Pi publication", err)
	}
	done()
}
