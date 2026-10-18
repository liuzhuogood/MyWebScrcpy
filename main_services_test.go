package main

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"mywebscrcpy/internal/ws"
)

func TestConfigureAuxiliaryServicesRegistersMatchingRoutes(t *testing.T) {
	mux := http.NewServeMux()
	hub := ws.NewHub("adb", "")
	dir := t.TempDir()

	aux := configureAuxiliaryServices(dir, nil, hub, mux)
	defer aux.Close()

	rec := httptest.NewRecorder()
	req := httptest.NewRequest("GET", "/api/matching-service/status", nil)
	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d: %s", rec.Code, rec.Body.String())
	}

	var status map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &status); err != nil {
		t.Fatalf("failed to decode status response: %v", err)
	}

	if state, ok := status["state"].(string); !ok || state != "stopped" {
		t.Fatalf("expected state 'stopped', got %v", status["state"])
	}
}
