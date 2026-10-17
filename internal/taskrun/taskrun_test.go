package taskrun

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
	"time"

	"mywebscrcpy/internal/db"
)

func setupTestTaskRunService(t *testing.T) (*Service, string) {
	t.Helper()
	tempDir := t.TempDir()
	dbPath := filepath.Join(tempDir, "test.db")
	database, err := db.Open(dbPath)
	if err != nil {
		t.Fatalf("创建临时数据库失败: %v", err)
	}
	t.Cleanup(func() { _ = database.Close() })

	tasksRoot := filepath.Join(tempDir, "tasks")
	if err := os.MkdirAll(tasksRoot, 0755); err != nil {
		t.Fatal(err)
	}

	svc := NewService(database)
	if _, err := svc.SetRootDir(tasksRoot); err != nil {
		t.Fatalf("设置任务根目录失败: %v", err)
	}

	return svc, tasksRoot
}

func createMockTaskDir(t *testing.T, root, taskID string, taskDef TaskDef, taskState TaskState, files map[string]string) string {
	t.Helper()
	dir := filepath.Join(root, taskID)
	if err := os.MkdirAll(dir, 0755); err != nil {
		t.Fatal(err)
	}

	taskBytes, _ := json.Marshal(taskDef)
	if err := os.WriteFile(filepath.Join(dir, "task.json"), taskBytes, 0644); err != nil {
		t.Fatal(err)
	}

	stateBytes, _ := json.Marshal(taskState)
	if err := os.WriteFile(filepath.Join(dir, "state.json"), stateBytes, 0644); err != nil {
		t.Fatal(err)
	}

	for relPath, content := range files {
		fullPath := filepath.Join(dir, relPath)
		if err := os.MkdirAll(filepath.Dir(fullPath), 0755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(fullPath, []byte(content), 0644); err != nil {
			t.Fatal(err)
		}
	}

	return dir
}

func TestManualScanAndMemoryIndex(t *testing.T) {
	svc, root := setupTestTaskRunService(t)
	handler := NewHandler(svc)
	mux := http.NewServeMux()
	handler.RegisterRoutes(mux)

	// 1. 未扫描前：查询列表返回为空，scanned_at 为 0
	req := httptest.NewRequest("GET", "/api/task-runs", nil)
	w := httptest.NewRecorder()
	mux.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("期望状态 200，实际 %d", w.Code)
	}
	var initResp struct {
		Items     []TaskSummary `json:"items"`
		Total     int           `json:"total"`
		ScannedAt int64         `json:"scanned_at"`
	}
	_ = json.Unmarshal(w.Body.Bytes(), &initResp)
	if initResp.ScannedAt != 0 || len(initResp.Items) != 0 {
		t.Fatalf("未扫描前应为空: %+v", initResp)
	}

	// 2. 创建 2 个任务：一个完整成功，一个业务成功但缺少必需产出
	now := time.Now().UnixMilli()
	createMockTaskDir(t, root, "task-complete", TaskDef{
		Name:         "完整任务",
		DeviceSerial: "phone-1",
		Source:       "codex",
		Goal:         "打卡并保留报告",
		Outputs: []OutputDef{
			{ID: "rep", Category: "report", Format: "markdown", Path: "report.md", Required: true},
			{ID: "log", Category: "log", Format: "text/utf-8", Path: "logs/task.log", Required: true},
		},
	}, TaskState{
		Status:     "success",
		LastResult: "已完成",
		UpdatedAt:  now,
	}, map[string]string{
		"report.md":      "# 成功报告",
		"logs/task.log": "执行日志",
	})

	createMockTaskDir(t, root, "task-missing-output", TaskDef{
		Name:         "缺少产出任务",
		DeviceSerial: "phone-1",
		Source:       "codex",
		Goal:         "录屏任务",
		Outputs: []OutputDef{
			{ID: "rep", Category: "report", Format: "markdown", Path: "report.md", Required: true},
			{ID: "vid", Category: "video", Format: "video/mp4", Path: "videos/rec.mp4", Required: true},
		},
	}, TaskState{
		Status:     "success",
		LastResult: "业务执行完成",
		UpdatedAt:  now - 1000,
	}, map[string]string{
		"report.md": "# 报告完成，但录屏丢失",
		// 故意不创建 videos/rec.mp4
	})

	// 3. 用户主动触发扫描
	scanReq := httptest.NewRequest("POST", "/api/task-runs/scan", nil)
	scanW := httptest.NewRecorder()
	mux.ServeHTTP(scanW, scanReq)
	if scanW.Code != http.StatusOK {
		t.Fatalf("扫描失败: code=%d body=%s", scanW.Code, scanW.Body.String())
	}
	var scanRes ScanResult
	_ = json.Unmarshal(scanW.Body.Bytes(), &scanRes)
	if scanRes.Count != 2 || scanRes.ScannedAt == 0 {
		t.Fatalf("扫描结果不符合预期: %+v", scanRes)
	}

	// 4. 查询列表与独立产出完整性验证
	listReq := httptest.NewRequest("GET", "/api/task-runs", nil)
	listW := httptest.NewRecorder()
	mux.ServeHTTP(listW, listReq)
	var listResp struct {
		Items     []TaskSummary `json:"items"`
		Total     int           `json:"total"`
		ScannedAt int64         `json:"scanned_at"`
	}
	_ = json.Unmarshal(listW.Body.Bytes(), &listResp)
	if listResp.Total != 2 {
		t.Fatalf("期望 2 个任务，实际 %d", listResp.Total)
	}

	for _, item := range listResp.Items {
		if item.ID == "task-complete" {
			if !item.OutputsComplete || item.Status != "success" {
				t.Fatalf("完整任务产出完整性判断错误: %+v", item)
			}
		} else if item.ID == "task-missing-output" {
			if item.OutputsComplete || item.Status != "success" {
				t.Fatalf("缺少产出任务应标明产出不完整: %+v", item)
			}
		}
	}
}

func TestCorruptedTaskAndRootDirectorySwitch(t *testing.T) {
	svc, root := setupTestTaskRunService(t)
	handler := NewHandler(svc)
	mux := http.NewServeMux()
	handler.RegisterRoutes(mux)

	// 创建一个正常任务和一个半写/损坏的无效任务
	now := time.Now().UnixMilli()
	createMockTaskDir(t, root, "valid-task", TaskDef{
		Name:         "正常任务",
		DeviceSerial: "dev-1",
		Source:       "manual",
	}, TaskState{
		Status:    "running",
		UpdatedAt: now,
	}, nil)

	corruptedDir := filepath.Join(root, "broken-task")
	_ = os.MkdirAll(corruptedDir, 0755)
	_ = os.WriteFile(filepath.Join(corruptedDir, "task.json"), []byte("{broken json..."), 0644)

	// 扫描不应崩溃，正常任务与损坏任务同时返回
	scanReq := httptest.NewRequest("POST", "/api/task-runs/scan", nil)
	scanW := httptest.NewRecorder()
	mux.ServeHTTP(scanW, scanReq)
	if scanW.Code != http.StatusOK {
		t.Fatalf("扫描失败: %d", scanW.Code)
	}

	listReq := httptest.NewRequest("GET", "/api/task-runs", nil)
	listW := httptest.NewRecorder()
	mux.ServeHTTP(listW, listReq)
	var listResp struct {
		Items []TaskSummary `json:"items"`
		Total int           `json:"total"`
	}
	_ = json.Unmarshal(listW.Body.Bytes(), &listResp)
	if listResp.Total != 2 {
		t.Fatalf("期望 2 个任务包含损坏项，实际 %d", listResp.Total)
	}

	// 切换根目录：内存索引应被清空，等待下次扫描
	newRoot := filepath.Join(t.TempDir(), "new_tasks")
	_ = os.MkdirAll(newRoot, 0755)
	if _, err := svc.SetRootDir(newRoot); err != nil {
		t.Fatal(err)
	}

	afterSwitchReq := httptest.NewRequest("GET", "/api/task-runs", nil)
	afterSwitchW := httptest.NewRecorder()
	mux.ServeHTTP(afterSwitchW, afterSwitchReq)
	var clearedResp struct {
		Items []TaskSummary `json:"items"`
		Total int           `json:"total"`
	}
	_ = json.Unmarshal(afterSwitchW.Body.Bytes(), &clearedResp)
	if clearedResp.Total != 0 {
		t.Fatalf("切换根目录后旧索引未清空: %d", clearedResp.Total)
	}
}

func TestDirectDetailFetchAndSecurityChecks(t *testing.T) {
	svc, root := setupTestTaskRunService(t)
	handler := NewHandler(svc)
	mux := http.NewServeMux()
	handler.RegisterRoutes(mux)

	now := time.Now().UnixMilli()
	taskDir := createMockTaskDir(t, root, "secure-task", TaskDef{
		Name:         "安全验证任务",
		DeviceSerial: "dev-secure",
		Source:       "test",
		Outputs: []OutputDef{
			{ID: "rep", Category: "report", Format: "markdown", Path: "report.md", Required: true},
			{ID: "shot", Category: "screenshot", Format: "image/png", Path: "screenshots/shot.png", Required: false},
			{ID: "fake-svg", Category: "screenshot", Format: "image/svg+xml", Path: "screenshots/malicious.svg", Required: false},
			{ID: "escape", Category: "report", Format: "markdown", Path: "../outside.md", Required: false},
		},
	}, TaskState{
		Status:    "success",
		UpdatedAt: now,
	}, map[string]string{
		"report.md":                  "# 安全报告",
		"screenshots/shot.png":       "\x89PNG\r\n\x1a\n\x00\x00\x00\rIHDR\x00\x00\x00\x01\x00\x00\x00\x01\x08\x06\x00\x00\x00\x1f\x15c4\x00\x00\x00\nIDATx\x9cc\x00\x01\x00\x00\x05\x00\x01\r\n-\xb4\x00\x00\x00\x00IEND\xaeB`\x82",
		"screenshots/malicious.svg":  "<svg onload=alert(1)>",
		"actions.jsonl":              "{\"timestamp\":1000,\"action\":\"step1\",\"result\":\"ok\"}\n{\"timestamp\":2000,\"action\":\"step2\",\"result\":\"done\"}",
	})

	// 1. 直接获取任务详情（无需先扫描根目录）
	getReq := httptest.NewRequest("GET", "/api/task-runs/secure-task", nil)
	getW := httptest.NewRecorder()
	mux.ServeHTTP(getW, getReq)
	if getW.Code != http.StatusOK {
		t.Fatalf("获取详情失败: %d", getW.Code)
	}
	var detail TaskDetail
	_ = json.Unmarshal(getW.Body.Bytes(), &detail)
	if detail.TaskDef.Name != "安全验证任务" || len(detail.RecentActions) != 2 {
		t.Fatalf("详情读取异常: %+v", detail)
	}

	// 2. 读取合法产出文件
	outReq := httptest.NewRequest("GET", "/api/task-runs/secure-task/outputs/shot", nil)
	outW := httptest.NewRecorder()
	mux.ServeHTTP(outW, outReq)
	if outW.Code != http.StatusOK || outW.Header().Get("Content-Type") != "image/png" {
		t.Fatalf("合法截图读取异常: code=%d type=%s", outW.Code, outW.Header().Get("Content-Type"))
	}

	// 3. 非法媒体直出（SVG）应被拒绝
	svgReq := httptest.NewRequest("GET", "/api/task-runs/secure-task/outputs/fake-svg", nil)
	svgW := httptest.NewRecorder()
	mux.ServeHTTP(svgW, svgReq)
	if svgW.Code != http.StatusForbidden {
		t.Fatalf("SVG直出未被拒绝: code=%d", svgW.Code)
	}

	// 4. 路径穿越产出拒绝
	escReq := httptest.NewRequest("GET", "/api/task-runs/secure-task/outputs/escape", nil)
	escW := httptest.NewRecorder()
	mux.ServeHTTP(escW, escReq)
	if escW.Code != http.StatusForbidden {
		t.Fatalf("路径穿越未被拦截: code=%d", escW.Code)
	}

	// 5. 符号链接逃逸检查
	outsideFile := filepath.Join(t.TempDir(), "secret.txt")
	_ = os.WriteFile(outsideFile, []byte("secret"), 0644)
	symlinkPath := filepath.Join(taskDir, "symlink-outside.txt")
	_ = os.Symlink(outsideFile, symlinkPath)

	_, err := safeResolvePath(taskDir, "symlink-outside.txt")
	if err == nil {
		t.Fatal("符号链接越界未被检测出")
	}
}

func TestDeviceRunningTaskAssociation(t *testing.T) {
	svc, root := setupTestTaskRunService(t)

	now := time.Now().UnixMilli()
	// 未扫描前：无关联
	taskID, hasConflict, count, _ := svc.FindRunningTaskForDevice("phone-assoc")
	if taskID != "" || hasConflict || count != 0 {
		t.Fatalf("未扫描前不应关联")
	}

	// 创建 1 个运行中任务
	createMockTaskDir(t, root, "run-single", TaskDef{
		DeviceSerial: "phone-assoc",
	}, TaskState{
		Status:    "running",
		UpdatedAt: now,
	}, nil)

	_, _ = svc.Scan(t.Context())

	taskID, hasConflict, count, _ = svc.FindRunningTaskForDevice("phone-assoc")
	if taskID != "run-single" || hasConflict || count != 1 {
		t.Fatalf("唯一运行中任务关联失败: task=%s, conflict=%v, count=%d", taskID, hasConflict, count)
	}

	// 增加第 2 个运行中任务 -> 产生冲突
	createMockTaskDir(t, root, "run-second", TaskDef{
		DeviceSerial: "phone-assoc",
	}, TaskState{
		Status:    "running",
		UpdatedAt: now,
	}, nil)

	_, _ = svc.Scan(t.Context())

	taskID, hasConflict, count, _ = svc.FindRunningTaskForDevice("phone-assoc")
	if taskID != "" || !hasConflict || count != 2 {
		t.Fatalf("多运行任务应判定为冲突: task=%s, conflict=%v, count=%d", taskID, hasConflict, count)
	}
}

func TestTaskSecurityAndIntegrityChecks(t *testing.T) {
	svc, root := setupTestTaskRunService(t)
	handler := NewHandler(svc)
	mux := http.NewServeMux()
	handler.RegisterRoutes(mux)

	// 1. 测试同前缀逃逸符号链接
	probeDir := filepath.Join(root, "probe")
	probeOutside := filepath.Join(root, "probe-outside")
	_ = os.MkdirAll(probeOutside, 0755)
	_ = os.WriteFile(filepath.Join(probeOutside, "secret.md"), []byte("SECRET"), 0644)
	createMockTaskDir(t, root, "probe", TaskDef{
		Outputs: []OutputDef{{ID: "rep", Category: "report", Format: "markdown", Path: "report.md", Required: true}},
	}, TaskState{Status: "success"}, nil)
	_ = os.Symlink(filepath.Join(probeOutside, "secret.md"), filepath.Join(probeDir, "report.md"))

	w := httptest.NewRecorder()
	mux.ServeHTTP(w, httptest.NewRequest("GET", "/api/task-runs/probe/outputs/rep", nil))
	if w.Code != http.StatusForbidden {
		t.Fatalf("前缀逃逸符号链接未被阻止，HTTP状态码=%d", w.Code)
	}

	// 2. 外部任务目录符号链接逃逸
	temp := t.TempDir()
	external := filepath.Join(temp, "external-task")
	createMockTaskDir(t, temp, "external-task", TaskDef{Name: "ext"}, TaskState{Status: "success"}, nil)
	_ = os.Symlink(external, filepath.Join(root, "linked"))
	detail, err := svc.GetTaskDetail("linked")
	if err == nil && detail != nil && !detail.Corrupted {
		t.Fatal("任务目录符号链接逃逸未被阻止")
	}

	// 3. 空截图与伪图片检测
	shotsDir := filepath.Join(root, "empty-shots")
	createMockTaskDir(t, root, "empty-shots", TaskDef{
		Outputs: []OutputDef{{ID: "shots", Category: "screenshot", Format: "image/png", Path: "screenshots", Required: true, AllowedTypes: []string{".png"}}},
	}, TaskState{Status: "success"}, nil)
	_ = os.MkdirAll(filepath.Join(shotsDir, "screenshots"), 0755)
	_ = os.WriteFile(filepath.Join(shotsDir, "screenshots", "empty.png"), nil, 0644)

	d, err := svc.GetTaskDetail("empty-shots")
	if err != nil || d.OutputsComplete {
		t.Fatalf("空图片目录不应判定产出完整: err=%v complete=%v", err, d.OutputsComplete)
	}

	// 4. 伪造 PNG (HTML 假冒图片)
	createMockTaskDir(t, root, "fake-png", TaskDef{
		Outputs: []OutputDef{{ID: "shot", Category: "screenshot", Format: "image/png", Path: "shot.png", Required: true}},
	}, TaskState{Status: "success"}, map[string]string{
		"shot.png": "<html>fake image</html>",
	})
	dFake, _ := svc.GetTaskDetail("fake-png")
	if dFake.OutputsComplete {
		t.Fatal("假冒图片不应判定产出完整")
	}
	wFake := httptest.NewRecorder()
	mux.ServeHTTP(wFake, httptest.NewRequest("GET", "/api/task-runs/fake-png/outputs/shot", nil))
	if wFake.Code != http.StatusForbidden {
		t.Fatalf("伪造图片输出应返回 403 Forbidden，实际=%d", wFake.Code)
	}

	// 5. 目录产出子文件安全访问
	realPNG := []byte("\x89PNG\r\n\x1a\n\x00\x00\x00\rIHDR\x00\x00\x00\x01\x00\x00\x00\x01\x08\x06\x00\x00\x00\x1f\x15c4\x00\x00\x00\nIDATx\x9cc\x00\x01\x00\x00\x05\x00\x01\r\n-\xb4\x00\x00\x00\x00IEND\xaeB`\x82")
	_ = os.WriteFile(filepath.Join(shotsDir, "screenshots", "valid.png"), realPNG, 0644)
	wSub := httptest.NewRecorder()
	mux.ServeHTTP(wSub, httptest.NewRequest("GET", "/api/task-runs/empty-shots/outputs/shots?file=valid.png", nil))
	if wSub.Code != http.StatusOK {
		t.Fatalf("子文件读取失败: code=%d", wSub.Code)
	}
}
