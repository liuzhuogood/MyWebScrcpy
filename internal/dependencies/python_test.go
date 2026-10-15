package dependencies

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
	"time"

	pythonsdk "mywebscrcpy/packages/python-sdk"
)

func TestMain(m *testing.M) {
	if root := os.Getenv("MYWEBSCRCPY_TEST_PYTHON"); root != "" {
		if len(os.Args) > 2 && os.Args[1] == "-m" && os.Args[2] == "pip" {
			if _, err := os.Stat(filepath.Join(root, "slow")); err == nil {
				time.Sleep(10 * time.Second)
			}
			time.Sleep(200 * time.Millisecond)
			if _, err := os.Stat(filepath.Join(root, "fail")); err == nil {
				fmt.Fprintln(os.Stderr, "controlled pip failure")
				os.Exit(1)
			}
			raw, _ := json.Marshal(os.Args[1:])
			_ = os.WriteFile(filepath.Join(root, "installed.json"), raw, 0600)
			os.Exit(0)
		}
		version := ""
		if _, err := os.Stat(filepath.Join(root, "installed.json")); err == nil {
			version = pythonsdk.Version
		}
		fmt.Printf("{\"version\":\"3.12.15\",\"pip_available\":true,\"sdk_version\":%q,\"sdk_error\":\"\"}\n", version)
		os.Exit(0)
	}
	os.Exit(m.Run())
}

func TestPythonSelectionAndModuleInstallation(t *testing.T) {
	m := newTestManager(t)
	root := t.TempDir()
	t.Setenv("MYWEBSCRCPY_TEST_PYTHON", root)
	t.Setenv("MYWEBSCRCPY_PYTHON", "")
	exe, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	s := Selection{Source: "system", Path: exe}
	candidate := m.Settings()
	candidate.Python = s
	if err := m.Apply(candidate); err != nil {
		t.Fatal("missing optional ADB blocked Python configuration", err)
	}
	reloaded, err := NewManager(filepath.Dir(m.root))
	if err != nil || reloaded.Settings().Python != s {
		t.Fatal("Python selection not persisted", err)
	}
	if _, err := m.resolvePython(Selection{Source: "builtin"}); err == nil {
		t.Fatal("builtin silently used system Python")
	}
	if _, err := m.resolvePython(Selection{Source: "system", Path: filepath.Join(root, "missing")}); err == nil {
		t.Fatal("invalid configured Python fell back")
	}
	if err := m.InstallPythonModules(s); err != nil {
		t.Fatal(err)
	}
	if err := m.InstallPythonModules(s); err == nil {
		t.Fatal("duplicate installation accepted")
	}
	if err := m.Uninstall("python"); err == nil {
		t.Fatal("runtime removed while pip was running")
	}
	if _, err := m.acquire("python"); err == nil {
		t.Fatal("runtime lease acquired while pip was changing modules")
	}
	m.jobs.Wait()
	raw, err := os.ReadFile(filepath.Join(root, "installed.json"))
	if err != nil {
		t.Fatal(err)
	}
	var args []string
	if err := json.Unmarshal(raw, &args); err != nil {
		t.Fatal(err)
	}
	expected := []string{"-m", "pip", "install", "--disable-pip-version-check", "--no-input", "--index-url", "https://pypi.org/simple", "mywebscrcpy-sdk[vision]==" + pythonsdk.Version}
	if !reflect.DeepEqual(args, expected) {
		t.Fatalf("unexpected pip command: %v", args)
	}
	tool := m.detect("python", s)
	if !tool.Installed || !tool.Python.Ready || m.tasks[pythonModulesJob].Stage != "ready" {
		t.Fatalf("installed SDK not ready: %+v", tool)
	}
	previous := active
	active = m
	t.Cleanup(func() { active = previous })
	_, release, err := AcquirePython()
	if err != nil {
		t.Fatal(err)
	}
	if err := m.InstallPythonModules(s); err == nil {
		t.Fatal("pip modified a runtime used by a task")
	}
	release()
}

func TestPythonModulesFailureAndCancellationReleaseRuntime(t *testing.T) {
	for _, mode := range []string{"fail", "slow"} {
		t.Run(mode, func(t *testing.T) {
			m := newTestManager(t)
			root := t.TempDir()
			t.Setenv("MYWEBSCRCPY_TEST_PYTHON", root)
			t.Setenv("MYWEBSCRCPY_PYTHON", "")
			if err := os.WriteFile(filepath.Join(root, mode), nil, 0600); err != nil {
				t.Fatal(err)
			}
			exe, err := os.Executable()
			if err != nil {
				t.Fatal(err)
			}
			if err := m.InstallPythonModules(Selection{Source: "system", Path: exe}); err != nil {
				t.Fatal(err)
			}
			if mode == "slow" {
				if err := m.Cancel(pythonModulesJob); err != nil {
					t.Fatal(err)
				}
			}
			m.jobs.Wait()
			task := m.tasks[pythonModulesJob]
			if mode == "slow" && task.Stage != "cancelled" {
				t.Fatalf("cancelled job reported %+v", task)
			}
			if mode == "fail" && (task.Stage != "failed" || !strings.Contains(task.Error, "controlled pip failure")) {
				t.Fatalf("pip error was lost: %+v", task)
			}
			release, err := m.exclusive([]string{"python"}, false)
			if err != nil {
				t.Fatal("runtime remained locked", err)
			}
			release()
		})
	}
}

func TestPythonModulesRouteRejectsUnknownPackageInput(t *testing.T) {
	m := newTestManager(t)
	mux := http.NewServeMux()
	registerPythonModules(mux, m, func(h http.HandlerFunc) http.HandlerFunc { return h })
	r := httptest.NewRequest("POST", "/api/dependencies/python/modules", strings.NewReader(`{"selection":{"source":"system","path":"","fd":"","rg":""},"package":"other-package"}`))
	w := httptest.NewRecorder()
	mux.ServeHTTP(w, r)
	if w.Code != 400 {
		t.Fatal("client could choose arbitrary package", w.Code)
	}
}

func TestExistingConfigKeepsSystemPython(t *testing.T) {
	m := newTestManager(t)
	old := `{"schema":1,"adb":{"source":"system","path":"","fd":"","rg":""},"pi":{"source":"builtin","path":"","fd":"","rg":""}}`
	if err := os.WriteFile(filepath.Join(m.root, "config.json"), []byte(old), 0600); err != nil {
		t.Fatal(err)
	}
	reloaded, err := NewManager(filepath.Dir(m.root))
	if err != nil || reloaded.Settings().Python != (Selection{Source: "system", Path: Current().Python}) {
		t.Fatal("known older configuration changed Python source", err)
	}
}

func TestPythonArchiveMaterializesInternalLinksAndRejectsEscape(t *testing.T) {
	for _, target := range []string{"python3.12", "../../../outside", "/tmp/outside", "missing", "python3"} {
		t.Run(target, func(t *testing.T) {
			var b bytes.Buffer
			gz := gzip.NewWriter(&b)
			tw := tar.NewWriter(gz)
			body := []byte("verified runtime")
			tw.WriteHeader(&tar.Header{Name: "python/bin/python3.12", Mode: 0755, Size: int64(len(body)), Typeflag: tar.TypeReg})
			tw.Write(body)
			tw.WriteHeader(&tar.Header{Name: "python/bin/python3", Linkname: target, Typeflag: tar.TypeSymlink})
			tw.Close()
			gz.Close()
			archive := filepath.Join(t.TempDir(), "python.tar.gz")
			os.WriteFile(archive, b.Bytes(), 0600)
			dest := t.TempDir()
			files := map[string]installedFile{}
			err := extractPythonArchive(context.Background(), archive, dest, files)
			if target != "python3.12" {
				if err == nil {
					t.Fatal("invalid archive link accepted")
				}
				return
			}
			if err != nil {
				t.Fatal(err)
			}
			info, err := os.Lstat(filepath.Join(dest, "bin", "python3"))
			if err != nil || !info.Mode().IsRegular() {
				t.Fatal("link not materialized", err)
			}
			if files["bin/python3"] != files["bin/python3.12"] {
				t.Fatal("materialized content differs")
			}
		})
	}
}

func TestRealManagedPythonFromArchive(t *testing.T) {
	source := os.Getenv("MYWEBSCRCPY_PYTHON_ARCHIVE")
	if source == "" {
		t.Skip("set MYWEBSCRCPY_PYTHON_ARCHIVE to verify standalone runtime and PyPI installation")
	}
	m := newTestManager(t)
	raw, err := os.ReadFile(source)
	if err != nil {
		t.Fatal(err)
	}
	m.client = downloadFunc(func(r *http.Request) (*http.Response, error) {
		return &http.Response{StatusCode: 200, Header: http.Header{}, ContentLength: int64(len(raw)), Body: io.NopCloser(bytes.NewReader(raw))}, nil
	})
	if err := m.install(context.Background(), "python"); err != nil {
		t.Fatal(err)
	}
	s := Selection{Source: "builtin"}
	tool := m.detect("python", s)
	if !tool.Installed || !tool.Python.PipAvailable {
		t.Fatalf("invalid runtime: %+v", tool)
	}
	// A selected system Python may have packages installed with --user. Probe
	// the same site configuration as task execution instead of hiding those packages.
	userBase := t.TempDir()
	t.Setenv("PYTHONUSERBASE", userBase)
	cmd := exec.Command(tool.Path, "-c", "import site; print(site.getusersitepackages())")
	cmd.Env = PythonEnvironment()
	output, err := cmd.Output()
	if err != nil {
		t.Fatal(err)
	}
	userSite := strings.TrimSpace(string(output))
	dist := filepath.Join(userSite, "mywebscrcpy_sdk-0.0.0.dist-info")
	if err := os.MkdirAll(dist, 0700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dist, "METADATA"), []byte("Metadata-Version: 2.1\nName: mywebscrcpy-sdk\nVersion: 0.0.0\n"), 0600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(userSite, "mywebscrcpy.py"), []byte("raise RuntimeError('user SDK probe marker')\n"), 0600); err != nil {
		t.Fatal(err)
	}
	userTool := m.detect("python", s)
	if userTool.Python == nil || userTool.Python.SDKVersion != "0.0.0" || userTool.Python.SDKError != "user SDK probe marker" || userTool.Python.Ready {
		t.Fatalf("user package was hidden: %+v", userTool.Python)
	}
	if err := os.RemoveAll(userBase); err != nil {
		t.Fatal(err)
	}
	if err := m.InstallPythonModules(s); err != nil {
		t.Fatal(err)
	}
	m.jobs.Wait()
	if task := m.tasks[pythonModulesJob]; task.Stage != "ready" {
		t.Fatalf("%+v", task)
	}
	tool = m.detect("python", s)
	if !tool.Python.Ready {
		t.Fatalf("SDK not ready: %+v", tool)
	}
	if _, err := m.installation("python", true); err != nil {
		t.Fatal("pip damaged runtime integrity", err)
	}
	if err := m.Uninstall("python"); err != nil {
		t.Fatal(err)
	}
	if _, err := m.resolvePython(s); err == nil {
		t.Fatal("removed runtime remains usable")
	}
}
