package main

import (
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
)

func TestUpdateGateKeepsStatusAvailableAndBlocksBusiness(t *testing.T) {
	updateGate.Lock()
	defer updateGate.Unlock()
	handler := updateMiddleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { w.WriteHeader(204) }))
	for _, test := range []struct {
		path string
		code int
	}{{"/api/tasks", 503}, {"/ws", 503}, {"/api/version/status", 204}, {"/api/desktop/health", 204}} {
		response := httptest.NewRecorder()
		handler.ServeHTTP(response, httptest.NewRequest("GET", test.path, nil))
		if response.Code != test.code {
			t.Fatalf("%s: %d", test.path, response.Code)
		}
	}
}
func TestProbationRequiresSupervisorCommit(t *testing.T) {
	receipt := filepath.Join(t.TempDir(), "receipt")
	t.Setenv("MYWEBSCRCPY_UPDATE_RECEIPT", receipt)
	handler := updateMiddleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { w.WriteHeader(204) }))
	check := func(expected int) {
		response := httptest.NewRecorder()
		handler.ServeHTTP(response, httptest.NewRequest("GET", "/api/devices", nil))
		if response.Code != expected {
			t.Fatalf("expected %d, got %d", expected, response.Code)
		}
	}
	check(503)
	if err := os.WriteFile(receipt+".confirmed", []byte("ok"), 0600); err != nil {
		t.Fatal(err)
	}
	check(204)
}
