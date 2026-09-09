package main

import (
	"encoding/json"
	"strings"
	"testing"
)

func TestOpenAPISpecDocumentsRegisteredAPIs(t *testing.T) {
	var spec struct {
		OpenAPI string                     `json:"openapi"`
		Paths   map[string]json.RawMessage `json:"paths"`
	}
	if err := json.Unmarshal(openAPISpec, &spec); err != nil {
		t.Fatalf("parse embedded OpenAPI spec: %v", err)
	}
	if spec.OpenAPI != "3.1.0" {
		t.Fatalf("unexpected OpenAPI version %q", spec.OpenAPI)
	}

	for _, path := range []string{
		"/api/devices", "/api/rotate", "/api/screen-state", "/api/reboot", "/ws",
		"/api/recordings", "/api/recordings/{recording_id}", "/api/recordings/{recording_id}/stop",
		"/api/adb/commands", "/api/sendkey", "/api/files", "/api/files/upload",
		"/api/ui/xml", "/api/vision/stream", "/api/vision/results", "/api/vision/stats", "/api/vision/debug",
	} {
		if _, ok := spec.Paths[path]; !ok {
			t.Errorf("registered API %s is missing from the OpenAPI spec", path)
		}
	}
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
