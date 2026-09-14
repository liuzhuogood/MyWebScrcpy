package main

import (
	"encoding/json"
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
		// 文件管理
		"/api/files",
		"/api/files/download",
		"/api/files/upload",
		"/api/files/folders",
		"/api/files/move",
		"/api/files/rename",
		"/api/files/delete",
		"/api/files/undo",
		// 脚本管理
		"/api/scripts/categories",
		"/api/scripts/categories/{cat}",
		"/api/scripts",
		"/api/scripts/{cat}",
		"/api/scripts/{cat}/{name}",
		// UI 检查
		"/api/ui/xml",
		"/api/ui/page-info",
		// Vision
		"/api/vision/stream",
		"/api/vision/results",
		"/api/vision/stats",
		"/api/vision/debug",
		// 模板匹配
		"/api/templates",
		"/api/templates/export",
		"/api/templates/import",
		"/api/templates/{id}",
		"/api/templates/{id}/image",
		"/api/templates/status",
		"/api/templates/matches",
		"/api/templates/detect",
		"/api/templates/click",
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
		"ScriptCategoryListResponse",
		"ScriptCategoryCreateRequest",
		"ScriptCategoryRenameRequest",
		"ScriptCategoryActionResponse",
		"ScriptInfo",
		"ScriptDetail",
		"ScriptListResponse",
		"CreateScriptRequest",
		"UpdateScriptRequest",
		"ScriptDetailResponse",
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
		"TemplateStatusResponse",
		"UpdateTemplateStatusRequest",
		"TemplateMatchResult",
		"TemplateMatchesResponse",
		"TemplateDetectRequest",
		"TemplateDetectResponse",
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

func TestAPIDocsUsesCurrentBrowserAddress(t *testing.T) {
	pageBytes, err := webFS.ReadFile("web/api-docs.html")
	if err != nil {
		t.Fatalf("read embedded API docs page: %v", err)
	}
	page := string(pageBytes)
	for _, expected := range []string{
		"location.origin",
		"promptEl.textContent",
		"my-scrcpy-use",
		"if(tag==='脚本')continue",
	} {
		if !strings.Contains(page, expected) {
			t.Errorf("API docs page is missing %q", expected)
		}
	}
}
