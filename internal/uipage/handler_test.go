package uipage

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestHandlerServeHTTP(t *testing.T) {
	windowDump := `
  mCurrentFocus=Window{a833e82 u0 com.uutalk.im/com.jiangxia.im.MainActivity}
`
	wmSize := `Physical size: 1080x2400`
	wmDensity := `Physical density: 420`
	inputDump := `SurfaceOrientation: 0`

	s := NewWithRunner(func(_ context.Context, args ...string) ([]byte, error) {
		cmd := strings.Join(args, " ")
		switch {
		case strings.Contains(cmd, "window"):
			return []byte(windowDump), nil
		case strings.Contains(cmd, "size"):
			return []byte(wmSize), nil
		case strings.Contains(cmd, "density"):
			return []byte(wmDensity), nil
		case strings.Contains(cmd, "input"):
			return []byte(inputDump), nil
		default:
			return nil, nil
		}
	})

	h := Handler{
		Service: s,
		IsOnline: func(serial string) bool {
			return serial == "online-device"
		},
	}

	tests := []struct {
		name       string
		method     string
		target     string
		wantStatus int
		checkBody  func(t *testing.T, body string)
	}{
		{
			name:       "method not allowed",
			method:     http.MethodPost,
			target:     "/api/ui/page-info?serial=online-device",
			wantStatus: http.StatusMethodNotAllowed,
		},
		{
			name:       "missing serial",
			method:     http.MethodGet,
			target:     "/api/ui/page-info",
			wantStatus: http.StatusBadRequest,
			checkBody: func(t *testing.T, body string) {
				if !strings.Contains(body, "missing_serial") {
					t.Errorf("expected missing_serial, got %s", body)
				}
			},
		},
		{
			name:       "device offline",
			method:     http.MethodGet,
			target:     "/api/ui/page-info?serial=offline-device",
			wantStatus: http.StatusBadGateway,
			checkBody: func(t *testing.T, body string) {
				if !strings.Contains(body, "device_unavailable") {
					t.Errorf("expected device_unavailable, got %s", body)
				}
			},
		},
		{
			name:       "success",
			method:     http.MethodGet,
			target:     "/api/ui/page-info?serial=online-device",
			wantStatus: http.StatusOK,
			checkBody: func(t *testing.T, body string) {
				var info PageInfo
				if err := json.Unmarshal([]byte(body), &info); err != nil {
					t.Fatalf("unmarshal error: %v, body: %s", err, body)
				}
				if info.PackageName != "com.uutalk.im" {
					t.Errorf("PackageName = %s, want com.uutalk.im", info.PackageName)
				}
				if info.Activity != "com.jiangxia.im.MainActivity" {
					t.Errorf("Activity = %s, want com.jiangxia.im.MainActivity", info.Activity)
				}
				if info.Display == nil || info.Display.Width != 1080 {
					t.Errorf("Display = %+v", info.Display)
				}
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			r := httptest.NewRequest(tt.method, tt.target, nil)
			w := httptest.NewRecorder()
			h.ServeHTTP(w, r)

			if w.Code != tt.wantStatus {
				t.Fatalf("status = %d, want %d: %s", w.Code, tt.wantStatus, w.Body.String())
			}
			if tt.checkBody != nil {
				tt.checkBody(t, w.Body.String())
			}
		})
	}
}
