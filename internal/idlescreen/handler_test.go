package idlescreen

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestSettingValidationAndPersistenceFailure(t *testing.T) {
	m, _, _, _ := fixture()
	mux := http.NewServeMux()
	m.RegisterRoutes(mux)
	for _, body := range []string{`{}`, `{"enabled":null}`, `{"enabled":"true"}`, `{"enabled":true,"timeout":1}`, `{"enabled":true}{}`} {
		w := httptest.NewRecorder()
		mux.ServeHTTP(w, httptest.NewRequest("PUT", "/api/system/idle-screen", strings.NewReader(body)))
		if w.Code != 400 || m.Config().Enabled {
			t.Fatalf("invalid body accepted: %s code=%d", body, w.Code)
		}
	}
	w := httptest.NewRecorder()
	mux.ServeHTTP(w, httptest.NewRequest("PUT", "/api/system/idle-screen", strings.NewReader(`{"enabled":true}`)))
	if w.Code != 200 || !m.Config().Enabled {
		t.Fatal("valid config did not save")
	}
}
