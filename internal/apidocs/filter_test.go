package apidocs

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

const sampleSpec = `{
  "openapi": "3.1.0",
  "info": {
    "title": "Full API",
    "version": "1.0.0"
  },
  "tags": [
    {"name": "Device", "description": "设备管理"},
    {"name": "System", "description": "系统维护"}
  ],
  "paths": {
    "/api/devices": {
      "get": {
        "tags": ["Device"],
        "summary": "获取设备列表"
      },
      "post": {
        "tags": ["Device"],
        "summary": "连接设备"
      }
    },
    "/api/v1/actions/tap": {
      "post": {
        "tags": ["Actions"],
        "summary": "点击"
      }
    },
    "/api/dependencies/install": {
      "post": {
        "tags": ["System"],
        "summary": "安装依赖"
      }
    }
  }
}`

func TestParseConfig(t *testing.T) {
	jsonConfig := `{
		"title": "Custom Auto API",
		"endpoints": [
			"/api/devices",
			"POST /api/v1/actions/*",
			{ "path": "/api/test", "methods": ["GET", "POST"] }
		]
	}`

	cfg, err := ParseConfig([]byte(jsonConfig))
	if err != nil {
		t.Fatalf("ParseConfig failed: %v", err)
	}

	if cfg.Title != "Custom Auto API" {
		t.Errorf("expected title 'Custom Auto API', got %q", cfg.Title)
	}
	if len(cfg.Endpoints) != 3 {
		t.Fatalf("expected 3 endpoints, got %d", len(cfg.Endpoints))
	}

	if !cfg.Matches("/api/devices", "GET") {
		t.Errorf("expected /api/devices GET to match")
	}
	if !cfg.Matches("/api/devices", "POST") {
		t.Errorf("expected /api/devices POST to match")
	}
	if !cfg.Matches("/api/v1/actions/tap", "POST") {
		t.Errorf("expected /api/v1/actions/tap POST to match")
	}
	if cfg.Matches("/api/v1/actions/tap", "GET") {
		t.Errorf("expected /api/v1/actions/tap GET NOT to match")
	}
	if cfg.Matches("/api/dependencies/install", "POST") {
		t.Errorf("expected /api/dependencies/install NOT to match")
	}
}

func TestFilterOpenAPISpec(t *testing.T) {
	cfg := Config{
		Endpoints: []EndpointRule{
			{Path: "/api/devices", Methods: []string{"GET"}},
			{Path: "/api/v1/actions/", PrefixMatch: true},
		},
	}

	filteredBytes, err := FilterOpenAPISpec([]byte(sampleSpec), cfg)
	if err != nil {
		t.Fatalf("FilterOpenAPISpec failed: %v", err)
	}

	var res struct {
		Info struct {
			Title string `json:"title"`
		} `json:"info"`
		Paths map[string]map[string]interface{} `json:"paths"`
		Tags  []struct {
			Name string `json:"name"`
		} `json:"tags"`
	}

	if err := json.Unmarshal(filteredBytes, &res); err != nil {
		t.Fatalf("unmarshal filtered spec failed: %v", err)
	}

	if len(res.Paths) != 2 {
		t.Fatalf("expected 2 paths, got %d: %v", len(res.Paths), res.Paths)
	}

	if dev, ok := res.Paths["/api/devices"]; !ok {
		t.Errorf("expected /api/devices to be present")
	} else {
		if _, hasGet := dev["get"]; !hasGet {
			t.Errorf("expected GET /api/devices to be kept")
		}
		if _, hasPost := dev["post"]; hasPost {
			t.Errorf("expected POST /api/devices to be removed")
		}
	}

	if _, ok := res.Paths["/api/v1/actions/tap"]; !ok {
		t.Errorf("expected /api/v1/actions/tap to be present")
	}

	if _, ok := res.Paths["/api/dependencies/install"]; ok {
		t.Errorf("expected /api/dependencies/install to be filtered out")
	}

	// Tags check: Device should be kept, System should be omitted
	foundDevice := false
	foundSystem := false
	for _, tag := range res.Tags {
		if tag.Name == "Device" {
			foundDevice = true
		}
		if tag.Name == "System" {
			foundSystem = true
		}
	}
	if !foundDevice {
		t.Errorf("expected Device tag to be kept")
	}
	if foundSystem {
		t.Errorf("expected System tag to be omitted")
	}
}

func TestManagerWithDiskConfig(t *testing.T) {
	tmpDir := t.TempDir()
	cfgPath := filepath.Join(tmpDir, "auto.json")

	initialCfg := `{"endpoints": ["/api/devices"]}`
	if err := os.WriteFile(cfgPath, []byte(initialCfg), 0644); err != nil {
		t.Fatal(err)
	}

	mgr := NewManager(cfgPath, nil, []byte(sampleSpec))
	spec1, err := mgr.GetAutomationSpec()
	if err != nil {
		t.Fatalf("GetAutomationSpec failed: %v", err)
	}

	var parsed1 struct {
		Paths map[string]interface{} `json:"paths"`
	}
	if err := json.Unmarshal(spec1, &parsed1); err != nil {
		t.Fatal(err)
	}
	if len(parsed1.Paths) != 1 || parsed1.Paths["/api/devices"] == nil {
		t.Fatalf("expected only /api/devices, got %v", parsed1.Paths)
	}

	// 更新磁盘配置
	updatedCfg := `{"endpoints": ["/api/v1/actions/*"]}`
	if err := os.WriteFile(cfgPath, []byte(updatedCfg), 0644); err != nil {
		t.Fatal(err)
	}

	spec2, err := mgr.GetAutomationSpec()
	if err != nil {
		t.Fatalf("second GetAutomationSpec failed: %v", err)
	}
	var parsed2 struct {
		Paths map[string]interface{} `json:"paths"`
	}
	if err := json.Unmarshal(spec2, &parsed2); err != nil {
		t.Fatal(err)
	}
	if len(parsed2.Paths) != 1 || parsed2.Paths["/api/v1/actions/tap"] == nil {
		t.Fatalf("expected only /api/v1/actions/tap, got %v", parsed2.Paths)
	}
}
