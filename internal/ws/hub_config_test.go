package ws

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestServeWSRejectsInvalidQuality(t *testing.T) {
	h := NewHub("adb", "jar")
	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/ws?serial=dev1&quality=49", nil)

	h.ServeWS(w, req)

	if w.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want %d", w.Code, http.StatusBadRequest)
	}
}
