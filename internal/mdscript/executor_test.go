package mdscript

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"mywebscrcpy/internal/db"
	"mywebscrcpy/internal/taskmanager"
)

func testExecutor(t *testing.T) (*db.DB, *taskmanager.Manager, *Executor) {
	t.Helper()
	d, err := db.Open(filepath.Join(t.TempDir(), "scripts.db"))
	if err != nil {
		t.Fatal(err)
	}
	mgr := taskmanager.NewManager(d)
	t.Cleanup(func() {
		mgr.Close()
		d.Close()
	})
	lookup := func(serial string) (bool, bool) { return serial != "unknown", serial != "offline" }
	check := func(serial, id string) error {
		if id == "common" || id == "private-A" && serial == "device-A" {
			return nil
		}
		return fmt.Errorf("模板 %s 不适用于设备 %s", id, serial)
	}
	return d, mgr, NewExecutor(d, mgr, lookup, check)
}

func waitTask(t *testing.T, d *db.DB, id, status string) {
	t.Helper()
	deadline := time.Now().Add(4 * time.Second)
	for time.Now().Before(deadline) {
		record, err := d.GetTaskByID(id)
		if err != nil {
			t.Fatal(err)
		}
		if record.Status == status {
			return
		}
		time.Sleep(20 * time.Millisecond)
	}
	record, _ := d.GetTaskByID(id)
	t.Fatalf("expected %s, got %+v", status, record)
}

func TestMultiDeviceArgumentsAndIndependentStop(t *testing.T) {
	d, mgr, executor := testExecutor(t)
	content := "version: 1.0.0\ntimeout_sec: 60\n# 流程\n- 等待\n"
	script, err := d.CreateMarkdownScript("多设备", []string{"device-A", "device-B"}, content, "1.0.0", "", 1, 60)
	if err != nil {
		t.Fatal(err)
	}
	outputDir := t.TempDir()
	directory, _ := json.Marshal(outputDir)
	// Stay alive until the test stops each task; slow interpreter startup must
	// not consume the other device's lifetime before independent-stop checks.
	python := fmt.Sprintf("import sys, pathlib, time\np = pathlib.Path(%s) / sys.argv[1]\np.write_text(sys.argv[1], encoding='utf-8')\nwhile True:\n    time.sleep(0.1)\n", directory)
	if err := d.SaveCompiledPython(script.ID, "1.0.0", python); err != nil {
		t.Fatal(err)
	}
	results, err := executor.Execute(script.ID)
	if err != nil || len(results) != 2 || results[0].Status != "started" || results[1].Status != "started" {
		t.Fatalf("%+v %v", results, err)
	}
	deadline := time.Now().Add(15 * time.Second)
	for _, result := range results {
		var data []byte
		for time.Now().Before(deadline) {
			data, err = os.ReadFile(filepath.Join(outputDir, result.DeviceSerial))
			if err == nil && string(data) == result.DeviceSerial {
				break
			}
			record, taskErr := d.GetTaskByID(result.Task.ID)
			if taskErr != nil || record == nil || record.Status != "running" {
				t.Fatalf("Python exited before device readiness: task=%+v error=%v", record, taskErr)
			}
			time.Sleep(20 * time.Millisecond)
		}
		if string(data) != result.DeviceSerial {
			record, taskErr := d.GetTaskByID(result.Task.ID)
			t.Fatalf("Python readiness timed out: argument=%q fileError=%v task=%+v taskError=%v", data, err, record, taskErr)
		}
	}
	// Editing future targets must not change either running instance's device.
	if err := d.UpdateMarkdownScript(script.ID, script.Name, []string{"device-C"}, content, "1.0.0", "", 1, 60); err != nil {
		t.Fatal(err)
	}
	if err := mgr.KillTask(results[0].Task.ID); err != nil {
		t.Fatal(err)
	}
	record, _ := d.GetTaskByID(results[1].Task.ID)
	if record.Status != "running" || record.DeviceSerial != "device-B" {
		t.Fatalf("other instance changed: %+v", record)
	}
	waitTask(t, d, results[0].Task.ID, "killed")
}

func TestPerDevicePreflightAndHistory(t *testing.T) {
	d, mgr, executor := testExecutor(t)
	blocker, err := mgr.StartTask(999, "占用", "busy", 10, "import time\ntime.sleep(10)")
	if err != nil {
		t.Fatal(err)
	}
	defer mgr.KillTask(blocker.ID)
	content := "version: 1.0.0\n- 点击 ![确认](template://common)"
	script, err := d.CreateMarkdownScript("分发", []string{"device-A", "offline", "busy", "unknown"}, content, "1.0.0", "", 1, 10)
	if err != nil {
		t.Fatal(err)
	}
	results, err := executor.Execute(script.ID)
	if err != nil {
		t.Fatal(err)
	}
	expected := []string{"started", "skipped", "skipped", "skipped"}
	for i, result := range results {
		if result.Status != expected[i] || result.Task == nil {
			t.Fatalf("result %d: %+v", i, result)
		}
		if i > 0 && result.Error == "" {
			t.Fatalf("missing rejection reason: %+v", result)
		}
	}
	records, err := d.ListScriptRuns(script.ID)
	if err != nil || len(records) != 4 {
		t.Fatalf("history %+v %v", records, err)
	}
	saved, _ := d.GetMarkdownScript(script.ID)
	if saved.LastRunStatus != "partial" {
		t.Fatalf("summary=%s", saved.LastRunStatus)
	}
	waitTask(t, d, results[0].Task.ID, "completed")
	saved, _ = d.GetMarkdownScript(script.ID)
	if saved.LastRunStatus != "partial" {
		t.Fatalf("completion summary=%s", saved.LastRunStatus)
	}
}

func TestDeviceSpecificTemplateDoesNotRunOnOtherDevices(t *testing.T) {
	d, _, executor := testExecutor(t)
	content := "version: 1.0.0\n- 点击 ![专属](template://private-A)"
	script, _ := d.CreateMarkdownScript("专属", []string{"device-A", "device-B"}, content, "1.0.0", "", 1, 10)
	results, err := executor.Execute(script.ID)
	if err != nil || results[0].Status != "started" || results[1].Status != "failed" || results[1].Task.PID != 0 {
		t.Fatalf("%+v %v", results, err)
	}
	if !strings.Contains(results[1].Error, "不适用") {
		t.Fatal(results[1].Error)
	}
}

func TestScriptHTTPDeviceContractAndCompilationInvalidation(t *testing.T) {
	d, _, executor := testExecutor(t)
	mux := http.NewServeMux()
	NewHandler(d, executor, nil).RegisterRoutes(mux)
	post := func(method, path, body string) *httptest.ResponseRecorder {
		recorder := httptest.NewRecorder()
		mux.ServeHTTP(recorder, httptest.NewRequest(method, path, strings.NewReader(body)))
		return recorder
	}
	for _, body := range []string{
		`{"name":"测试","content":"- 等待"}`,
		`{"name":"测试","target_devices":[],"content":"- 等待"}`,
		`{"name":"测试","target_devices":["auto"],"content":"- 等待"}`,
		`{"name":"测试","target_devices":["device-A","device-A"],"content":"- 等待"}`,
		`{"name":"测试","target_devices":["unknown"],"content":"- 等待"}`,
		`{"name":"测试","target_devices":["device-A"],"content":"target_device: device-A\n- 等待"}`,
		`{"name":"测试","target_devices":["device-A"],"content":"schedule: invalid\n- 等待"}`,
	} {
		if response := post("POST", "/api/md-scripts", body); response.Code != 400 {
			t.Fatalf("accepted invalid configuration %s => %s", body, response.Body.String())
		}
	}
	response := post("POST", "/api/md-scripts", `{"name":"测试","target_devices":["device-A","device-B"],"content":"version: 1.0.0\nenabled: true\n- 等待"}`)
	if response.Code != 200 {
		t.Fatal(response.Body.String())
	}
	var created struct {
		Script db.MarkdownScriptRecord `json:"script"`
	}
	if err := json.Unmarshal(response.Body.Bytes(), &created); err != nil {
		t.Fatal(err)
	}
	if len(created.Script.TargetDevices) != 2 || strings.Contains(response.Body.String(), `"target_device":`) {
		t.Fatal(response.Body.String())
	}
	id := created.Script.ID
	if err := d.SaveCompiledPython(id, "1.0.0", "print('old')"); err != nil {
		t.Fatal(err)
	}
	if err := d.ToggleScriptEnabled(id, 0); err != nil {
		t.Fatal(err)
	}
	response = post("PUT", fmt.Sprintf("/api/md-scripts/%d", id), `{"name":"测试","target_devices":["device-A","device-B"],"content":"version: 1.0.0\nenabled: true\n- 新步骤"}`)
	if response.Code != 200 {
		t.Fatal(response.Body.String())
	}
	saved, _ := d.GetMarkdownScript(id)
	if saved.CompiledPython != "" || saved.IsEnabled != 0 {
		t.Fatalf("stale compilation or re-enabled schedule: %+v", saved)
	}
	if response := post("POST", fmt.Sprintf("/api/md-scripts/%d/run?serial=device-C", id), ""); response.Code != 400 {
		t.Fatal(response.Body.String())
	}
}

func TestGeneratedPythonQuotesAndRequiredDeviceArgument(t *testing.T) {
	// 模拟 Windows 非 UTF-8 标准输出，生成脚本仍应能输出中文。
	t.Setenv("PYTHONIOENCODING", "cp1252")
	content := "version: 1.0.0\n- 输入 \\\"引号\\\" 与 {literal} 和 C:\\test\n"
	meta := ParseMarkdown(content, time.Now())
	source, err := NewCompiler().Compile("任务\n\"\"\"{title}", meta, content)
	if err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(t.TempDir(), "generated.py")
	if err := os.WriteFile(path, []byte(source), 0600); err != nil {
		t.Fatal(err)
	}
	if out, err := exec.Command("python3", "-m", "py_compile", path).CombinedOutput(); err != nil {
		t.Fatalf("Python syntax %s %v", out, err)
	}
	if out, err := exec.Command("python3", path).CombinedOutput(); err == nil || !strings.Contains(string(out), "device_serial") {
		t.Fatalf("missing argument accepted: %s %v", out, err)
	}
	out, err := exec.Command("python3", path, "device-A").CombinedOutput()
	if err != nil || !strings.Contains(string(out), "Device device-A") || !strings.Contains(string(out), "{literal}") {
		t.Fatalf("%s %v", out, err)
	}
}

func TestDispatchSnapshotFastExitAndForeignScript(t *testing.T) {
	d, m, executor := testExecutor(t)
	content := "---\nversion: 1.0.0\nreport:\n  enabled: true\nrecording:\n  enabled: false\n---\n- 等待"
	script, _ := d.CreateMarkdownScript("快照", []string{"device-A", "device-B"}, content, "1.0.0", "", 1, 5)
	d.SaveCompiledPython(script.ID, "1.0.0", "print('快')")
	results, e := executor.ExecuteWithTrigger(script.ID, "schedule")
	if e != nil {
		t.Fatal(e)
	}
	id := results[0].DispatchID
	for _, result := range results {
		if result.DispatchID != id || result.Task == nil {
			t.Fatalf("lost batch %+v", result)
		}
		waitTask(t, d, result.Task.ID, "completed")
	}
	d.UpdateMarkdownScript(script.ID, "已改名", []string{"device-C"}, "version: 2.0", "2.0", "", 1, 5)
	dispatch, e := d.GetDispatch(script.ID, id)
	if e != nil || dispatch.Trigger != "schedule" || dispatch.DispatchState != "finished" {
		t.Fatalf("%+v %v", dispatch, e)
	}
	var snapshot struct {
		ScriptName    string   `json:"script_name"`
		TargetDevices []string `json:"target_devices"`
		CompiledHash  string   `json:"compiled_hash"`
	}
	json.Unmarshal(dispatch.Snapshot, &snapshot)
	if snapshot.ScriptName != "快照" || len(snapshot.TargetDevices) != 2 || snapshot.CompiledHash == "" {
		t.Fatalf("snapshot changed %+v", snapshot)
	}
	foreign, _ := d.CreateMarkdownScript("另一个", []string{"device-A"}, "version: 1", "1", "", 1, 5)
	mux := http.NewServeMux()
	taskmanager.NewHandler(m).RegisterRoutes(mux)
	response := httptest.NewRecorder()
	mux.ServeHTTP(response, httptest.NewRequest("GET", fmt.Sprintf("/api/md-scripts/%d/dispatches/%s", foreign.ID, id), nil))
	if response.Code != 404 {
		t.Fatal("cross-script access", response.Code)
	}
	response = httptest.NewRecorder()
	mux.ServeHTTP(response, httptest.NewRequest("GET", fmt.Sprintf("/api/tasks/%s/logs?after=0&before=2", results[0].Task.ID), nil))
	if response.Code != 400 {
		t.Fatal("ambiguous cursor", response.Code)
	}
}
