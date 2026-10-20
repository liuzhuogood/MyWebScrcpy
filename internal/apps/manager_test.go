package apps

import (
	"bytes"
	"context"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestBuildInstallArgs(t *testing.T) {
	opts := InstallOptions{
		ReplaceExisting:  true,
		GrantPermissions: true,
		AllowDowngrade:   true,
	}
	args := BuildInstallArgs("device-123", "/tmp/app.apk", opts)
	joined := strings.Join(args, " ")
	expected := "-s device-123 install -r -t -g -d /tmp/app.apk"
	if joined != expected {
		t.Errorf("expected %q, got %q", expected, joined)
	}

	optsSimple := InstallOptions{
		ReplaceExisting:  false,
		GrantPermissions: false,
		AllowDowngrade:   false,
	}
	argsSimple := BuildInstallArgs("device-123", "/tmp/app.apk", optsSimple)
	joinedSimple := strings.Join(argsSimple, " ")
	expectedSimple := "-s device-123 install -t /tmp/app.apk"
	if joinedSimple != expectedSimple {
		t.Errorf("expected %q, got %q", expectedSimple, joinedSimple)
	}
}

func TestParseInstallOutput(t *testing.T) {
	tests := []struct {
		output   string
		wantOK   bool
		wantCode string
	}{
		{"Performing Streamed Install\nSuccess\n", true, ""},
		{"Failure [INSTALL_FAILED_VERSION_DOWNGRADE]\n", false, "INSTALL_FAILED_VERSION_DOWNGRADE"},
		{"Failure [INSTALL_FAILED_ALREADY_EXISTS: Attempt to re-install ...]\n", false, "INSTALL_FAILED_ALREADY_EXISTS"},
		{"Failure [INSTALL_FAILED_INSUFFICIENT_STORAGE]\n", false, "INSTALL_FAILED_INSUFFICIENT_STORAGE"},
		{"Failure [INSTALL_FAILED_UPDATE_INCOMPATIBLE: Package ...]\n", false, "INSTALL_FAILED_UPDATE_INCOMPATIBLE"},
		{"Failure [INSTALL_FAILED_INVALID_APK: ...]\n", false, "INSTALL_FAILED_INVALID_APK"},
		{"Error: APK corrupted\nINSTALL_PARSE_FAILED_NOT_APK", false, "INSTALL_PARSE_FAILED_NOT_APK"},
	}

	for _, tt := range tests {
		ok, code, msg := ParseInstallOutput(tt.output)
		if ok != tt.wantOK {
			t.Errorf("output %q: got ok=%v, want %v", tt.output, ok, tt.wantOK)
		}
		if code != tt.wantCode {
			t.Errorf("output %q: got code=%q, want %q", tt.output, code, tt.wantCode)
		}
		if msg == "" {
			t.Errorf("output %q: friendlyMsg should not be empty", tt.output)
		}
	}
}

func TestManager_InstallSuccess(t *testing.T) {
	mockRunner := func(ctx context.Context, args ...string) ([]byte, error) {
		return []byte("Performing Streamed Install\nSuccess\n"), nil
	}
	mgr := NewManagerWithRunner(mockRunner)

	content := bytes.NewReader([]byte("fake-apk-binary-content"))
	opts := InstallOptions{ReplaceExisting: true}
	res, err := mgr.Install(context.Background(), "device-123", content, "test.apk", opts)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !res.OK {
		t.Fatalf("expected res.OK to be true, got false")
	}
}

func TestManager_InstallFailure(t *testing.T) {
	mockRunner := func(ctx context.Context, args ...string) ([]byte, error) {
		return []byte("Failure [INSTALL_FAILED_VERSION_DOWNGRADE]\n"), nil
	}
	mgr := NewManagerWithRunner(mockRunner)

	content := bytes.NewReader([]byte("fake-apk-binary-content"))
	opts := InstallOptions{ReplaceExisting: true}
	res, err := mgr.Install(context.Background(), "device-123", content, "test.apk", opts)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if res.OK {
		t.Fatalf("expected res.OK to be false, got true")
	}
	if res.Error != "INSTALL_FAILED_VERSION_DOWNGRADE" {
		t.Errorf("expected error code INSTALL_FAILED_VERSION_DOWNGRADE, got %q", res.Error)
	}
}

func TestHandler_InstallEndpoint(t *testing.T) {
	mockRunner := func(ctx context.Context, args ...string) ([]byte, error) {
		return []byte("Success\n"), nil
	}
	mgr := NewManagerWithRunner(mockRunner)
	mux := http.NewServeMux()
	RegisterRoutes(mux, mgr)

	// 构建 multipart 请求体
	body := &bytes.Buffer{}
	writer := multipart.NewWriter(body)
	part, err := writer.CreateFormFile("file", "app-debug.apk")
	if err != nil {
		t.Fatalf("create form file: %v", err)
	}
	part.Write([]byte("mock-apk-data"))
	_ = writer.WriteField("replace", "true")
	_ = writer.Close()

	req := httptest.NewRequest("POST", "/api/apps/install?serial=test-dev-1", body)
	req.Header.Set("Content-Type", writer.FormDataContentType())
	rec := httptest.NewRecorder()

	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200 OK, got %d: %s", rec.Code, rec.Body.String())
	}
	if !strings.Contains(rec.Body.String(), `"ok":true`) {
		t.Errorf("expected ok:true in body, got %s", rec.Body.String())
	}
}
