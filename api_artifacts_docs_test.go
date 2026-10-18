package main

import (
	"encoding/json"
	"strings"
	"testing"
)

func TestTaskRunDocumentationRemovesLegacyAPIs(t *testing.T) {
	var spec struct {
		Paths      map[string]json.RawMessage `json:"paths"`
	Tags       []struct {
			Name string `json:"name"`
		} `json:"tags"`
		Components struct {
			Schemas map[string]json.RawMessage `json:"schemas"`
		} `json:"components"`
	}
	if err := json.Unmarshal(openAPISpec, &spec); err != nil {
		t.Fatal(err)
	}

	// 1. 验证已废弃清理的旧接口及任务管理接口不再出现在规范中
	for path := range spec.Paths {
		if strings.HasPrefix(path, "/api/scripts") ||
			strings.HasPrefix(path, "/api/md-scripts") ||
			strings.HasPrefix(path, "/api/tasks") ||
			strings.HasPrefix(path, "/api/task-runs") ||
			strings.HasPrefix(path, "/api/legacy/") {
			t.Errorf("removed API still documented: %s", path)
		}
	}

	// 2. 验证保留的核心端点均已注册在规范中
	requiredPaths := []string{
		"/api/device-state/{serial}",
		"/api/device-state/{serial}/decision",
		"/api/system/service-config",
	}
	for _, path := range requiredPaths {
		if _, ok := spec.Paths[path]; !ok {
			t.Errorf("missing registered path %s in OpenAPI spec", path)
		}
	}

	// 3. 验证任务管理相关 Schema 已彻底移除
	removedSchemas := []string{
		"TaskSummary", "TaskDetail", "TaskDef", "TaskState", "TaskListResponse", "ScanResult",
		"TaskRun", "TaskRunLog", "CreateTaskRunRequest", "FinishTaskRunRequest", "TaskRunError",
	}
	for _, schemaName := range removedSchemas {
		if _, ok := spec.Components.Schemas[schemaName]; ok {
			t.Errorf("removed schema %s still documented in OpenAPI spec", schemaName)
		}
	}

	// 4. 验证 Task Runs 标签已移除
	for _, tag := range spec.Tags {
		if strings.EqualFold(tag.Name, "Task Runs") {
			t.Errorf("removed tag %s still present in OpenAPI spec", tag.Name)
		}
	}
}
