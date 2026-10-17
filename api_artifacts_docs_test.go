package main

import (
	"encoding/json"
	"strings"
	"testing"
)

func TestTaskRunDocumentationRemovesLegacyAPIs(t *testing.T) {
	var spec struct {
		Paths      map[string]json.RawMessage `json:"paths"`
		Components struct {
			Schemas map[string]json.RawMessage `json:"schemas"`
		} `json:"components"`
	}
	if err := json.Unmarshal(openAPISpec, &spec); err != nil {
		t.Fatal(err)
	}

	// 1. 验证已废弃清理的旧接口不再出现在规范中
	for path := range spec.Paths {
		if strings.HasPrefix(path, "/api/scripts") ||
			strings.HasPrefix(path, "/api/md-scripts") ||
			strings.HasPrefix(path, "/api/tasks/") ||
			strings.HasPrefix(path, "/api/legacy/") ||
			path == "/api/task-runs/{id}/heartbeat" ||
			path == "/api/task-runs/{id}/logs" ||
			path == "/api/task-runs/{id}/finish" {
			t.Errorf("removed API still documented: %s", path)
		}
	}

	// 2. 验证 v1.27.1 批准的新端点均已注册在规范中
	requiredPaths := []string{
		"/api/task-runs/scan",
		"/api/task-runs",
		"/api/task-runs/{id}",
		"/api/task-runs/{id}/outputs/{output_id}",
		"/api/device-state/{serial}",
		"/api/device-state/{serial}/decision",
		"/api/system/service-config",
	}
	for _, path := range requiredPaths {
		if _, ok := spec.Paths[path]; !ok {
			t.Errorf("missing registered path %s in OpenAPI spec", path)
		}
	}

	// 3. 验证 TaskSummary 结构属性
	var summarySchema struct {
		Properties map[string]json.RawMessage `json:"properties"`
	}
	if err := json.Unmarshal(spec.Components.Schemas["TaskSummary"], &summarySchema); err != nil {
		t.Fatalf("unmarshal TaskSummary schema: %v", err)
	}
	for _, field := range []string{"id", "status", "business_result", "outputs_complete", "goal", "updated_at", "is_stale"} {
		if _, ok := summarySchema.Properties[field]; !ok {
			t.Errorf("TaskSummary missing field: %s", field)
		}
	}

	// 4. 验证 TaskDetail 结构属性
	var detailSchema struct {
		Properties map[string]json.RawMessage `json:"properties"`
	}
	if err := json.Unmarshal(spec.Components.Schemas["TaskDetail"], &detailSchema); err != nil {
		t.Fatalf("unmarshal TaskDetail schema: %v", err)
	}
	for _, field := range []string{"id", "task_def", "state", "outputs", "outputs_complete", "is_stale"} {
		if _, ok := detailSchema.Properties[field]; !ok {
			t.Errorf("TaskDetail missing field: %s", field)
		}
	}
}
