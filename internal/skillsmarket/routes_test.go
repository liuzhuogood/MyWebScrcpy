package skillsmarket

import (
	"archive/zip"
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func setupTestServer(t *testing.T) (http.Handler, *Manager) {
	tempHome := t.TempDir()
	mgr, err := NewManager(tempHome)
	if err != nil {
		t.Fatalf("NewManager error: %v", err)
	}
	mux := http.NewServeMux()
	RegisterRoutes(mux, mgr)
	return mux, mgr
}

func TestRoutesAiIntegrations(t *testing.T) {
	handler, mgr := setupTestServer(t)

	req := httptest.NewRequest("GET", "/api/ai-integrations", nil)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rec.Code)
	}

	var resp AiIntegrationsResponse
	if err := json.NewDecoder(rec.Body).Decode(&resp); err != nil {
		t.Fatal(err)
	}
	if len(resp.Tools) != 3 {
		t.Fatalf("expected 3 tools, got %d", len(resp.Tools))
	}
	// Verify host path is populated
	for _, tool := range resp.Tools {
		if !strings.HasPrefix(tool.TargetPath, mgr.HomeDir()) {
			t.Fatalf("tool %s targetPath not rooted at homeDir: %s", tool.ID, tool.TargetPath)
		}
	}
}

func TestRoutesListAndGetSkills(t *testing.T) {
	handler, _ := setupTestServer(t)

	// List skills
	req := httptest.NewRequest("GET", "/api/skills", nil)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rec.Code)
	}

	var listResp SkillsListResponse
	if err := json.NewDecoder(rec.Body).Decode(&listResp); err != nil {
		t.Fatal(err)
	}
	if len(listResp.Skills) != 3 {
		t.Fatalf("expected 3 skills, got %d", len(listResp.Skills))
	}

	// Get existing skill
	req = httptest.NewRequest("GET", "/api/skills/mywebscrcpy-device-base", nil)
	rec = httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rec.Code)
	}

	var detail SkillDetail
	if err := json.NewDecoder(rec.Body).Decode(&detail); err != nil {
		t.Fatal(err)
	}
	if detail.ID != SkillDeviceBase || len(detail.Details.Capabilities) == 0 {
		t.Fatalf("unexpected detail: %+v", detail)
	}

	// Get unknown skill
	req = httptest.NewRequest("GET", "/api/skills/unknown-skill", nil)
	rec = httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusNotFound {
		t.Fatalf("expected 404 for unknown skill, got %d", rec.Code)
	}
}

func TestRoutesDownloadSkill(t *testing.T) {
	handler, _ := setupTestServer(t)

	// Valid skill download
	req := httptest.NewRequest("GET", "/api/skills/mywebscrcpy-kuaishou-daily/download", nil)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rec.Code)
	}

	ct := rec.Header().Get("Content-Type")
	if !strings.Contains(ct, "application/zip") {
		t.Fatalf("expected application/zip, got %s", ct)
	}

	cd := rec.Header().Get("Content-Disposition")
	if !strings.Contains(cd, "mywebscrcpy-kuaishou-daily.zip") {
		t.Fatalf("unexpected Content-Disposition: %s", cd)
	}

	// Verify valid zip
	body := rec.Body.Bytes()
	r, err := zip.NewReader(bytes.NewReader(body), int64(len(body)))
	if err != nil {
		t.Fatalf("invalid zip bytes: %v", err)
	}
	if len(r.File) == 0 {
		t.Fatal("empty zip archive")
	}

	// Unknown skill download
	req = httptest.NewRequest("GET", "/api/skills/unknown-skill/download", nil)
	rec = httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusNotFound {
		t.Fatalf("expected 404, got %d", rec.Code)
	}
}

func TestRoutesInstallSkillSecurityAndValidation(t *testing.T) {
	handler, mgr := setupTestServer(t)

	// 1. Success install
	reqBody := `{"targets":["codex","claude-code"]}`
	req := httptest.NewRequest("POST", "/api/skills/mywebscrcpy-device-base/install", strings.NewReader(reqBody))
	req.Host = "localhost:8080"
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Origin", "http://localhost:8080")
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
	}

	var installResp SkillInstallResponse
	if err := json.NewDecoder(rec.Body).Decode(&installResp); err != nil {
		t.Fatal(err)
	}
	if len(installResp.Results) != 2 {
		t.Fatalf("expected 2 results, got %d", len(installResp.Results))
	}
	for _, res := range installResp.Results {
		if res.Status != "installed" {
			t.Fatalf("target %s failed: %+v", res.Target, res)
		}
	}

	// 2. Cross-origin rejection
	req = httptest.NewRequest("POST", "/api/skills/mywebscrcpy-device-base/install", strings.NewReader(reqBody))
	req.Host = "localhost:8080"
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Origin", "http://evil.com")
	rec = httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusForbidden {
		t.Fatalf("expected 403 on cross-origin, got %d", rec.Code)
	}

	// 3. Sec-Fetch-Site cross-site rejection
	req = httptest.NewRequest("POST", "/api/skills/mywebscrcpy-device-base/install", strings.NewReader(reqBody))
	req.Host = "localhost:8080"
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Sec-Fetch-Site", "cross-site")
	rec = httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusForbidden {
		t.Fatalf("expected 403 on cross-site, got %d", rec.Code)
	}

	// 4. Content-Type rejection
	req = httptest.NewRequest("POST", "/api/skills/mywebscrcpy-device-base/install", strings.NewReader(reqBody))
	req.Host = "localhost:8080"
	req.Header.Set("Content-Type", "text/plain")
	rec = httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected 400 on non-json content-type, got %d", rec.Code)
	}

	// 5. Unknown fields rejection
	req = httptest.NewRequest("POST", "/api/skills/mywebscrcpy-device-base/install", strings.NewReader(`{"targets":["codex"],"unknown_field":123}`))
	req.Host = "localhost:8080"
	req.Header.Set("Content-Type", "application/json")
	rec = httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected 400 on unknown field, got %d", rec.Code)
	}

	// 6. Duplicate targets rejection
	req = httptest.NewRequest("POST", "/api/skills/mywebscrcpy-device-base/install", strings.NewReader(`{"targets":["codex","codex"]}`))
	req.Host = "localhost:8080"
	req.Header.Set("Content-Type", "application/json")
	rec = httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected 400 on duplicate targets, got %d", rec.Code)
	}

	// 7. Empty targets rejection
	req = httptest.NewRequest("POST", "/api/skills/mywebscrcpy-device-base/install", strings.NewReader(`{"targets":[]}`))
	req.Host = "localhost:8080"
	req.Header.Set("Content-Type", "application/json")
	rec = httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected 400 on empty targets, got %d", rec.Code)
	}

	// 8. Unknown skill 404
	req = httptest.NewRequest("POST", "/api/skills/unknown-skill/install", strings.NewReader(`{"targets":["codex"]}`))
	req.Host = "localhost:8080"
	req.Header.Set("Content-Type", "application/json")
	rec = httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusNotFound {
		t.Fatalf("expected 404 on unknown skill, got %d", rec.Code)
	}

	// 9. Status of skill reflects installation
	detail, ok := mgr.GetSkillDetail("mywebscrcpy-device-base")
	if !ok {
		t.Fatal("skill not found")
	}
	if !detail.TargetStatuses[ToolCodex].Installed || !detail.TargetStatuses[ToolClaudeCode].Installed {
		t.Fatalf("target statuses should reflect installed targets: %+v", detail.TargetStatuses)
	}
}

func TestRoutesMethodNotAllowed(t *testing.T) {
	handler, _ := setupTestServer(t)

	req := httptest.NewRequest("DELETE", "/api/ai-integrations", nil)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusMethodNotAllowed {
		t.Fatalf("expected 405, got %d", rec.Code)
	}
}

func TestNilTargetsEmptyArrayContract(t *testing.T) {
	handler, _ := setupTestServer(t)

	// Check /api/skills
	req := httptest.NewRequest("GET", "/api/skills", nil)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rec.Code)
	}

	var rawList map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &rawList); err != nil {
		t.Fatal(err)
	}
	skillsList, ok := rawList["skills"].([]any)
	if !ok || len(skillsList) == 0 {
		t.Fatal("expected non-empty skills array")
	}
	firstSkill := skillsList[0].(map[string]any)
	val, exists := firstSkill["installedTargets"]
	if !exists {
		t.Fatal("installedTargets key missing")
	}
	arr, isArr := val.([]any)
	if !isArr || len(arr) != 0 {
		t.Fatalf("expected empty array [], got: %v (type %T)", val, val)
	}

	// Check /api/skills/:id
	req = httptest.NewRequest("GET", "/api/skills/mywebscrcpy-device-base", nil)
	rec = httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	var rawDetail map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &rawDetail); err != nil {
		t.Fatal(err)
	}
	val, exists = rawDetail["installedTargets"]
	if !exists {
		t.Fatal("installedTargets key missing from detail")
	}
	arr, isArr = val.([]any)
	if !isArr || len(arr) != 0 {
		t.Fatalf("expected empty array [] in detail, got: %v (type %T)", val, val)
	}
}

func TestRoutesTrailingJSONRejected(t *testing.T) {
	handler, _ := setupTestServer(t)

	// Body contains valid JSON followed by another JSON object
	body := strings.NewReader(`{"targets":["codex"]}{"targets":["claude-code"]}`)
	req := httptest.NewRequest("POST", "/api/skills/mywebscrcpy-device-base/install", body)
	req.Host = "localhost:8080"
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected 400 on trailing JSON payload, got %d", rec.Code)
	}
}

func TestRoutesBadContentTypeRejected(t *testing.T) {
	handler, _ := setupTestServer(t)

	for _, badType := range []string{
		"application/jsonBAD",
		"application/json; bad-syntax",
		"text/plain",
		"multipart/form-data",
	} {
		body := strings.NewReader(`{"targets":["codex"]}`)
		req := httptest.NewRequest("POST", "/api/skills/mywebscrcpy-device-base/install", body)
		req.Host = "localhost:8080"
		req.Header.Set("Content-Type", badType)
		rec := httptest.NewRecorder()
		handler.ServeHTTP(rec, req)

		if rec.Code != http.StatusBadRequest {
			t.Fatalf("expected 400 on bad Content-Type %q, got %d", badType, rec.Code)
		}
	}
}

func TestRoutesCrossOriginRejected(t *testing.T) {
	handler, _ := setupTestServer(t)

	// Cross-site via Sec-Fetch-Site
	body := strings.NewReader(`{"targets":["codex"]}`)
	req := httptest.NewRequest("POST", "/api/skills/mywebscrcpy-device-base/install", body)
	req.Host = "localhost:8080"
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Sec-Fetch-Site", "cross-site")
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusForbidden {
		t.Fatalf("expected 403 on Sec-Fetch-Site: cross-site, got %d", rec.Code)
	}

	// Malformed Origin
	body = strings.NewReader(`{"targets":["codex"]}`)
	req = httptest.NewRequest("POST", "/api/skills/mywebscrcpy-device-base/install", body)
	req.Host = "localhost:8080"
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Origin", "://bad-url")
	rec = httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusForbidden {
		t.Fatalf("expected 403 on malformed Origin, got %d", rec.Code)
	}

	// Cross-origin Origin header
	body = strings.NewReader(`{"targets":["codex"]}`)
	req = httptest.NewRequest("POST", "/api/skills/mywebscrcpy-device-base/install", body)
	req.Host = "localhost:8080"
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Origin", "http://attacker.example.com")
	rec = httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusForbidden {
		t.Fatalf("expected 403 on attacker Origin, got %d", rec.Code)
	}
}

func TestRoutesNoStoreHeader(t *testing.T) {
	handler, _ := setupTestServer(t)

	for _, path := range []string{
		"/api/skills",
		"/api/skills/mywebscrcpy-device-base",
	} {
		req := httptest.NewRequest("GET", path, nil)
		rec := httptest.NewRecorder()
		handler.ServeHTTP(rec, req)

		if rec.Code != http.StatusOK {
			t.Fatalf("expected 200 for %s, got %d", path, rec.Code)
		}
		cc := rec.Header().Get("Cache-Control")
		if cc != "no-store" {
			t.Fatalf("expected Cache-Control: no-store on %s, got %q", path, cc)
		}
	}
}
