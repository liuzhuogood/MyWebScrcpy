package main

import (
	"encoding/json"
	"testing"
)

func TestTaskArtifactAPIDocumentation(t *testing.T) {
	var spec struct {
		Paths      map[string]json.RawMessage `json:"paths"`
		Components struct {
			Schemas map[string]json.RawMessage `json:"schemas"`
		} `json:"components"`
	}
	if e := json.Unmarshal(openAPISpec, &spec); e != nil {
		t.Fatal(e)
	}
	for _, path := range []string{"/api/md-scripts/{id}/dispatches", "/api/md-scripts/{id}/dispatches/{dispatch_id}", "/api/tasks/{id}", "/api/tasks/{id}/logs", "/api/tasks/{id}/logs/download", "/api/tasks/{id}/recording", "/api/tasks/{id}/recording/media", "/api/tasks/{id}/report", "/api/tasks/{id}/report/retry", "/api/tasks/{id}/report/download"} {
		if _, ok := spec.Paths[path]; !ok {
			t.Error("missing", path)
		}
	}
	for _, name := range []string{"TaskLogPage", "ExecutionSnapshot", "TaskReportResult", "TaskDetail"} {
		if _, ok := spec.Components.Schemas[name]; !ok {
			t.Error("missing schema", name)
		}
	}
}
