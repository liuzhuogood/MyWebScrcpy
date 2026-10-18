package main

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestOpenAPISpecDocumentsRegisteredAPIs(t *testing.T) {
	var spec struct {
		OpenAPI    string                     `json:"openapi"`
		Paths      map[string]json.RawMessage `json:"paths"`
		Components struct {
			Schemas    map[string]json.RawMessage `json:"schemas"`
			Parameters map[string]json.RawMessage `json:"parameters"`
		} `json:"components"`
	}
	if err := json.Unmarshal(openAPISpec, &spec); err != nil {
		t.Fatalf("parse embedded OpenAPI spec: %v", err)
	}
	if spec.OpenAPI != "3.1.0" {
		t.Fatalf("unexpected OpenAPI version %q", spec.OpenAPI)
	}

	expectedPaths := []string{
		// 设备与控制
		"/api/devices",
		"/api/rotate",
		"/api/screen-state",
		"/api/reboot",
		// 实时投屏
		"/ws",
		// 录像
		"/api/recordings",
		"/api/recordings/{recording_id}",
		"/api/recordings/{recording_id}/stop",
		"/api/recordings/{recording_id}/download",
		"/api/recordings/download",
		// 截图
		"/api/screenshots",
		"/api/screenshots/{screenshot_id}",
		// 回放
		"/api/replay/start",
		"/api/replay/stop",
		"/api/replay/pause",
		"/api/replay/seek",
		"/api/replay/segment",
		"/api/replay",
		// ADB 与按键
		"/api/adb/commands",
		"/api/adb/commands/{command_id}",
		"/api/sendkey",
		"/api/operation-logs",
		// 文件管理
		"/api/files",
		"/api/files/download",
		"/api/files/upload",
		"/api/files/folders",
		"/api/files/move",
		"/api/files/rename",
		"/api/files/delete",
		"/api/files/undo",
		// UI 检查
		"/api/ui/xml",
		"/api/ui/page-info",
		// Vision
		"/api/vision/stream",
		"/api/vision/results",
		"/api/vision/stats",
		"/api/vision/debug",
		// OCR
		"/api/vision/ocr",
		"/api/ocr/status",
		"/api/ocr/start",
		"/api/ocr/stop",
		"/api/ocr/config",
		"/api/ocr/test",
		// 模板管理
		"/api/templates",
		"/api/templates/export",
		"/api/templates/import",
		"/api/templates/{id}",
		"/api/templates/{id}/image",
		// 设备状态与AI决策
		"/api/device-state/{serial}",
		"/api/device-state/{serial}/decision",
		"/api/system/service-config",
		// AI 集成与技能市场
		"/api/ai-integrations",
		"/api/skills",
		"/api/skills/{id}",
		"/api/skills/{id}/download",
		"/api/skills/{id}/install",
		// 系统元接口
		"/api/openapi.json",
	}

	for _, path := range expectedPaths {
		if _, ok := spec.Paths[path]; !ok {
			t.Errorf("registered API %s is missing from the OpenAPI spec", path)
		}
	}

	// 验证核心出入参组件 Schema 完整性
	expectedSchemas := []string{
		"Device",
		"RotateResponse",
		"ScreenStateResponse",
		"SimpleOkResponse",
		"ErrorResponse",
		"StartRecordingRequest",
		"RecordingView",
		"RecordingsListResponse",
		"ScreenshotEntry",
		"ScreenshotsListResponse",
		"StartReplayRequest",
		"PauseReplayRequest",
		"SeekReplayRequest",
		"SegmentReplayRequest",
		"ReplayView",
		"ReplayStatusResponse",
		"ADBCommandRequest",
		"ADBCommandView",
		"SendKeyRequest",
		"FileItem",
		"FileListResponse",
		"FileActionResponse",
		"CreateFolderRequest",
		"MoveFileRequest",
		"RenameFileRequest",
		"DeleteFilesRequest",
		"DeleteFilesResponse",
		"UndoFilesRequest",
		"UIXMLResponse",
		"UIPageInfoResponse",
		"VisionActionRequest",
		"VisionSessionStats",
		"VisionDebugEvent",
		"VisionDebugResponse",
		"VisionObject",
		"VisionMessage",
		"Template",
		"TemplatesListResponse",
		"CreateTemplateRequest",
		"UpdateTemplateRequest",
		"AiTool",
		"AiIntegrationsResponse",
		"SkillSummary",
		"SkillDetail",
		"SkillsListResponse",
		"SkillInstallRequest",
		"SkillInstallTargetResult",
		"SkillInstallResponse",
		"DeviceTotalState",
		"TotalStateAndDecision",
		"ServiceConfig",
	}

	for _, schemaName := range expectedSchemas {
		if _, ok := spec.Components.Schemas[schemaName]; !ok {
			t.Errorf("component schema %s is missing from the OpenAPI spec", schemaName)
		}
	}
}

func TestAPIDocsConsistency(t *testing.T) {
	TestOpenAPISpecDocumentsRegisteredAPIs(t)
}

func TestAPIDocsPageGuardsScriptTagGrouping(t *testing.T) {
	pageBytes, err := webFS.ReadFile("web/api-docs.html")
	if err != nil {
		t.Fatalf("read embedded API docs page: %v", err)
	}
	page := string(pageBytes)
	if !strings.Contains(page, "if(tag==='脚本')continue") {
		t.Errorf("API docs page is missing %q", "if(tag==='脚本')continue")
	}
	if !strings.Contains(page, "api-docs-automation.html") {
		t.Errorf("API docs page is missing link to automation docs")
	}
}

func TestAutomationAPIDocsPage(t *testing.T) {
	pageBytes, err := webFS.ReadFile("web/api-docs-automation.html")
	if err != nil {
		t.Fatalf("read embedded automation API docs page: %v", err)
	}
	page := string(pageBytes)
	if !strings.Contains(page, "if(tag==='脚本')continue") {
		t.Errorf("automation API docs page is missing %q", "if(tag==='脚本')continue")
	}
	if !strings.Contains(page, "/api/openapi-automation.json") {
		t.Errorf("automation API docs page should fetch /api/openapi-automation.json")
	}
	if !strings.Contains(page, "api-docs.html") {
		t.Errorf("automation API docs page is missing link back to full docs")
	}
}

func TestAutomationOpenAPISpecFiltering(t *testing.T) {
	if automationAPIsManager == nil {
		t.Fatalf("automationAPIsManager is not initialized")
	}
	specBytes, err := automationAPIsManager.GetAutomationSpec()
	if err != nil {
		t.Fatalf("GetAutomationSpec failed: %v", err)
	}

	var spec struct {
		OpenAPI string                     `json:"openapi"`
		Info    struct {
			Title string `json:"title"`
		} `json:"info"`
		Paths map[string]json.RawMessage `json:"paths"`
	}
	if err := json.Unmarshal(specBytes, &spec); err != nil {
		t.Fatalf("unmarshal automation spec failed: %v", err)
	}

	if spec.OpenAPI != "3.1.0" {
		t.Fatalf("unexpected OpenAPI version %q", spec.OpenAPI)
	}
	if !strings.Contains(spec.Info.Title, "自动化") {
		t.Errorf("expected title to mention 自动化, got %q", spec.Info.Title)
	}

	// 必须包含核心自动化接口
	mustInclude := []string{
		"/api/devices",
		"/api/screen-state",
		"/api/rotate",
		"/api/sendkey",
		"/api/adb/commands",
		"/api/ui/xml",
		"/api/ui/page-info",
		"/api/v1/actions/tap",
		"/api/vision/ocr",
		"/api/templates",
	}
	for _, p := range mustInclude {
		if _, ok := spec.Paths[p]; !ok {
			t.Errorf("automation spec must include %s", p)
		}
	}

	// 必须排除非自动化运维/元接口
	mustExclude := []string{
		"/api/task-runs",
		"/api/dependencies",
		"/api/version/upgrade",
		"/api/version/status",
		"/api/ai-integrations",
		"/api/skills",
	}
	for _, p := range mustExclude {
		if _, ok := spec.Paths[p]; ok {
			t.Errorf("automation spec must exclude %s", p)
		}
	}
}

func TestAutomationOpenAPISpecHTTPEndpoints(t *testing.T) {
	// 1. 测试 /api/openapi-automation.json
	req1 := httptest.NewRequest(http.MethodGet, "/api/openapi-automation.json", nil)
	w1 := httptest.NewRecorder()
	serveAutomationOpenAPISpec(w1, req1)
	if w1.Code != http.StatusOK {
		t.Fatalf("expected 200 OK from /api/openapi-automation.json, got %d", w1.Code)
	}
	if ct := w1.Header().Get("Content-Type"); !strings.Contains(ct, "application/json") {
		t.Errorf("expected json Content-Type, got %q", ct)
	}

	// 2. 测试 /api/openapi.json?scope=automation
	req2 := httptest.NewRequest(http.MethodGet, "/api/openapi.json?scope=automation", nil)
	w2 := httptest.NewRecorder()
	serveOpenAPISpec(w2, req2)
	if w2.Code != http.StatusOK {
		t.Fatalf("expected 200 OK from /api/openapi.json?scope=automation, got %d", w2.Code)
	}
	var spec2 struct {
		Paths map[string]interface{} `json:"paths"`
	}
	if err := json.Unmarshal(w2.Body.Bytes(), &spec2); err != nil {
		t.Fatalf("parse response failed: %v", err)
	}
	if _, ok := spec2.Paths["/api/dependencies"]; ok {
		t.Errorf("expected dependencies to be excluded from automation spec via ?scope=automation")
	}

	// 3. 测试 /api/openapi.json (全量)
	req3 := httptest.NewRequest(http.MethodGet, "/api/openapi.json", nil)
	w3 := httptest.NewRecorder()
	serveOpenAPISpec(w3, req3)
	if w3.Code != http.StatusOK {
		t.Fatalf("expected 200 OK from /api/openapi.json, got %d", w3.Code)
	}
	var spec3 struct {
		Paths map[string]interface{} `json:"paths"`
	}
	if err := json.Unmarshal(w3.Body.Bytes(), &spec3); err != nil {
		t.Fatalf("parse response failed: %v", err)
	}
	if _, ok := spec3.Paths["/api/dependencies"]; !ok {
		t.Errorf("expected dependencies to be included in full openapi.json")
	}
}
