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
	for path := range spec.Paths {
		if strings.HasPrefix(path, "/api/scripts") || strings.HasPrefix(path, "/api/md-scripts") || strings.HasPrefix(path, "/api/tasks/") || strings.HasPrefix(path, "/api/legacy/") {
			t.Errorf("removed API still documented: %s", path)
		}
	}
	for _, path := range []string{"/api/task-runs", "/api/task-runs/{id}", "/api/task-runs/{id}/logs", "/api/task-runs/{id}/heartbeat", "/api/task-runs/{id}/finish"} {
		if _, ok := spec.Paths[path]; !ok {
			t.Error("missing", path)
		}
	}
	var schema struct {
		Properties map[string]json.RawMessage `json:"properties"`
	}
	if err := json.Unmarshal(spec.Components.Schemas["TaskRun"], &schema); err != nil {
		t.Fatal(err)
	}
	for _, field := range []string{"id", "client_run_id", "name", "source", "status", "summary", "report_markdown", "connectivity"} {
		if _, ok := schema.Properties[field]; !ok {
			t.Error("missing actual response field", field)
		}
	}
	for _, field := range []string{"idempotency_key", "task_name", "result_markdown", "is_lost"} {
		if _, ok := schema.Properties[field]; ok {
			t.Error("invented response field", field)
		}
	}
}
