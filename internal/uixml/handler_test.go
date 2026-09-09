package uixml

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestHandlerRequiresOnlineSerialAndReturnsSnapshot(t *testing.T) {
	s := NewWithRunner(func(_ context.Context, args ...string) ([]byte, error) {
		if args[2] == "shell" {
			return nil, nil
		}
		return []byte(`<hierarchy/>`), nil
	})
	h := Handler{Service: s, IsOnline: func(serial string) bool { return serial == "ok" }}
	for _, tc := range []struct {
		name, target string
		status       int
	}{
		{"missing", "/api/ui/xml", http.StatusBadRequest},
		{"offline", "/api/ui/xml?serial=no", http.StatusBadGateway},
		{"ok", "/api/ui/xml?serial=ok", http.StatusOK},
	} {
		t.Run(tc.name, func(t *testing.T) {
			r := httptest.NewRequest(http.MethodGet, tc.target, nil)
			w := httptest.NewRecorder()
			h.ServeHTTP(w, r)
			if w.Code != tc.status {
				t.Fatalf("status = %d, want %d: %s", w.Code, tc.status, w.Body.String())
			}
			if tc.name == "ok" && !strings.Contains(w.Body.String(), "hierarchy") {
				t.Fatalf("body = %s", w.Body.String())
			}
		})
	}
}
