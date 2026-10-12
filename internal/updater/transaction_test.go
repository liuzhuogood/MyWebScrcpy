package updater

import (
	"encoding/json"
	"fmt"
	"net"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"testing"
	"time"
)

func TestPolicyBlocksNonBinaryUpgrade(t *testing.T) {
	for _, policy := range []Policy{{"docker", "image", "镜像升级"}, {"desktop", "app", "完整应用升级"}, {"systemd", "managed", "服务部署升级"}} {
		t.Run(policy.Mode, func(t *testing.T) {
			svc := &Service{policy: policy}
			response := httptest.NewRecorder()
			svc.handleUpgrade(response, httptest.NewRequest("POST", "/api/version/upgrade", strings.NewReader(`{"download_url":"https://untrusted.example/file"}`)))
			if response.Code != http.StatusConflict {
				t.Fatalf("unexpected status %d", response.Code)
			}
		})
	}
}
func TestExplicitDockerPolicy(t *testing.T) {
	t.Setenv("MYWEBSCRCPY_RUNTIME", "docker")
	if policy := DetectPolicy(true); policy.Method != "image" {
		t.Fatalf("Docker must take precedence: %+v", policy)
	}
}
func TestChecksumAndExactAsset(t *testing.T) {
	file := filepath.Join(t.TempDir(), "binary")
	if err := os.WriteFile(file, []byte("bad"), 0600); err != nil {
		t.Fatal(err)
	}
	if verifyFile(file, "sha256:"+strings.Repeat("0", 64)) == nil {
		t.Fatal("corrupt update accepted")
	}
	if MatchAsset([]ReleaseAsset{{Name: "untrusted-macos-arm64"}}, "darwin", "arm64") != nil {
		t.Fatal("ambiguous asset accepted")
	}
}
func TestBackupRestoresDatabaseAndRemovesNewWAL(t *testing.T) {
	root := t.TempDir()
	backup := t.TempDir()
	os.MkdirAll(filepath.Join(root, "data"), 0700)
	database := filepath.Join(root, "data", "mywebscrcpy.db")
	os.WriteFile(database, []byte("old"), 0600)
	if err := BackupRuntime(root, backup); err != nil {
		t.Fatal(err)
	}
	os.WriteFile(database, []byte("migrated"), 0600)
	os.WriteFile(database+"-wal", []byte("new wal"), 0600)
	if err := RestoreRuntime(root, backup); err != nil {
		t.Fatal(err)
	}
	bytes, _ := os.ReadFile(database)
	if string(bytes) != "old" {
		t.Fatal("database was not restored")
	}
	if fileExists(database + "-wal") {
		t.Fatal("new WAL survived rollback")
	}
}

// This subprocess emulates an actual candidate: bind a fresh port, mutate a DB,
// publish its readiness, then serve health until the parent test cleans it up.
func TestUpdateChild(t *testing.T) {
	if os.Getenv("MYWEBSCRCPY_TEST_UPDATE_CHILD") != "1" {
		return
	}
	receipt := os.Getenv("MYWEBSCRCPY_UPDATE_RECEIPT")
	marker := os.Getenv("MYWEBSCRCPY_TEST_UPDATE_MARKER")
	_ = os.WriteFile(marker, []byte(strconv.Itoa(os.Getpid())), 0600)
	if receipt == "" {
		select {}
	}
	_ = os.WriteFile(filepath.Join("data", "mywebscrcpy.db"), []byte("migrated"), 0600)
	_ = os.WriteFile(filepath.Join("data", "mywebscrcpy.db-wal"), []byte("new wal"), 0600)
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		os.Exit(2)
	}
	version := "v1.13.0"
	if os.Getenv("MYWEBSCRCPY_TEST_BAD_HEALTH") == "1" {
		version = "v1.12.0"
	}
	if PublishHealth(listener, false, version) != nil {
		os.Exit(2)
	}
	_ = http.Serve(listener, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewEncoder(w).Encode(map[string]string{"version": version})
	}))
	os.Exit(0)
}
func workerFixture(t *testing.T) (updatePlan, string) {
	t.Helper()
	if runtime.GOOS == "windows" {
		t.Skip("test executable replacement is verified separately on Windows")
	}
	root := t.TempDir()
	stage := t.TempDir()
	exe, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	target := filepath.Join(root, "app")
	candidate := filepath.Join(stage, "candidate")
	for _, dest := range []string{target, candidate} {
		if err = copyFile(exe, dest); err != nil {
			t.Fatal(err)
		}
	}
	os.MkdirAll(filepath.Join(root, "data"), 0700)
	os.WriteFile(filepath.Join(root, "data", "mywebscrcpy.db"), []byte("old"), 0600)
	marker := filepath.Join(stage, "pid")
	t.Setenv("MYWEBSCRCPY_TEST_UPDATE_CHILD", "1")
	t.Setenv("MYWEBSCRCPY_TEST_UPDATE_MARKER", marker)
	t.Cleanup(func() {
		if bytes, err := os.ReadFile(marker); err == nil {
			pid, _ := strconv.Atoi(string(bytes))
			if pid > 1 {
				process, _ := os.FindProcess(pid)
				_ = process.Kill()
			}
		}
	})
	plan := updatePlan{Target: target, Candidate: candidate, Version: "v1.13.0", Directory: root, Args: []string{"-test.run=^TestUpdateChild$"}, Stopped: filepath.Join(stage, "stopped"), Receipt: filepath.Join(stage, "ready"), Status: filepath.Join(root, "data", "status.json")}
	os.WriteFile(plan.Stopped, []byte("stopped"), 0600)
	file := filepath.Join(stage, "plan.json")
	if err = writeJSON(file, plan); err != nil {
		t.Fatal(err)
	}
	return plan, file
}
func TestWorkerCommitsOnlyAfterHealthyStartup(t *testing.T) {
	plan, file := workerFixture(t)
	if err := RunWorker(file); err != nil {
		t.Fatal(err)
	}
	if !fileExists(plan.Receipt + ".confirmed") {
		t.Fatal("new service not committed")
	}
	bytes, _ := os.ReadFile(plan.Status)
	var status ProgressInfo
	json.Unmarshal(bytes, &status)
	if status.State != StateSuccess {
		t.Fatalf("unexpected progress %s", bytes)
	}
}
func TestWorkerRollsBackFailedStartupAndDatabase(t *testing.T) {
	plan, file := workerFixture(t)
	t.Setenv("MYWEBSCRCPY_TEST_BAD_HEALTH", "1")
	previous := workerHealthTimeout
	workerHealthTimeout = 200 * time.Millisecond
	defer func() { workerHealthTimeout = previous }()
	if err := RunWorker(file); err == nil {
		t.Fatal("bad startup accepted")
	}
	bytes, _ := os.ReadFile(filepath.Join(plan.Directory, "data", "mywebscrcpy.db"))
	if string(bytes) != "old" {
		t.Fatalf("DB rollback failed: %q", bytes)
	}
	if fileExists(filepath.Join(plan.Directory, "data", "mywebscrcpy.db-wal")) {
		t.Fatal("candidate WAL survived rollback")
	}
	bytes, _ = os.ReadFile(plan.Status)
	var status ProgressInfo
	json.Unmarshal(bytes, &status)
	if status.State != StateError {
		t.Fatal(fmt.Sprintf("unexpected status %s", bytes))
	}
}

func TestMacAppRollbackRestoresWholeBundleAndDatabase(t *testing.T) {
	if runtime.GOOS != "darwin" {
		t.Skip("macOS bundle rollback uses ditto")
	}
	root := t.TempDir()
	stage := filepath.Join(root, "backup")
	data := filepath.Join(root, "runtime")
	app := filepath.Join(root, "Test.app")
	backup := filepath.Join(stage, "previous.app")
	for _, dir := range []string{app, backup, filepath.Join(data, "data")} {
		if err := os.MkdirAll(dir, 0700); err != nil {
			t.Fatal(err)
		}
	}
	os.WriteFile(filepath.Join(backup, "old-resource"), []byte("old"), 0600)
	os.WriteFile(filepath.Join(app, "new-resource"), []byte("new"), 0600)
	database := filepath.Join(data, "data", "mywebscrcpy.db")
	os.WriteFile(database, []byte("old database"), 0600)
	if err := BackupRuntime(data, filepath.Join(stage, "runtime")); err != nil {
		t.Fatal(err)
	}
	os.WriteFile(database, []byte("new migration"), 0600)
	plan := AppPlan{App: app, Backup: backup, Data: data, Receipt: filepath.Join(stage, "ready"), Version: "v1.13.0", Pending: filepath.Join(data, "pending")}
	planFile := filepath.Join(stage, "plan.json")
	writeJSON(planFile, plan)
	os.WriteFile(filepath.Join(stage, "installed"), []byte("installed"), 0600)
	os.WriteFile(plan.Pending, []byte("pending"), 0600)
	writeJSON(plan.Receipt+".pids", map[string]int{"host": 0, "backend": 0})
	previousTimeout, previousRestart := appHealthTimeout, restartApp
	appHealthTimeout = 100 * time.Millisecond
	restarted := false
	restartApp = func(path string) error { restarted = path == app; return nil }
	defer func() { appHealthTimeout = previousTimeout; restartApp = previousRestart }()
	if err := WatchApp(planFile); err == nil {
		t.Fatal("invalid startup should report rollback")
	}
	if !restarted || !fileExists(filepath.Join(app, "old-resource")) || fileExists(filepath.Join(app, "new-resource")) {
		t.Fatal("whole App was not restored")
	}
	if !fileExists(filepath.Join(app+".failed-update", "new-resource")) {
		t.Fatal("failed App was not retained")
	}
	bytes, _ := os.ReadFile(database)
	if string(bytes) != "old database" {
		t.Fatal("App data was not restored")
	}
	if fileExists(plan.Pending) {
		t.Fatal("stale pending state remained after rollback")
	}
}

func TestRestartKeepsRelativeDataDirectory(t *testing.T) {
	args := []string{"--no-gui", "--data-dir", "runtime", "--listen", "127.0.0.1:8080"}
	normalized := restartArgs(args, "/absolute/runtime")
	if normalized[2] != "/absolute/runtime" || args[2] != "runtime" {
		t.Fatal("data directory changed or original args mutated")
	}
	if got := restartArgs([]string{"--data-dir=runtime"}, "/absolute/runtime")[0]; got != "--data-dir=/absolute/runtime" {
		t.Fatal(got)
	}
}
