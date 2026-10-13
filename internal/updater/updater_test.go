package updater

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestCompareVersions(t *testing.T) {
	tests := []struct {
		v1       string
		v2       string
		expected int
	}{
		{"v1.12.0", "v1.12.0", 0},
		{"v1.13.0", "v1.12.0", 1},
		{"v1.12.0", "v1.13.0", -1},
		{"v1.12.1", "v1.12.0", 1},
		{"v2.0.0", "v1.99.99", 1},
		{"v1.12.0-rc1", "v1.12.0", 0},
		{"1.13.0", "v1.12.0", 1},
		{"v1.2", "v1.2.0", 0},
	}

	for _, tt := range tests {
		got := CompareVersions(tt.v1, tt.v2)
		if got != tt.expected {
			t.Errorf("CompareVersions(%s, %s) = %d; want %d", tt.v1, tt.v2, got, tt.expected)
		}
	}
}

func TestMatchAsset(t *testing.T) {
	assets := []ReleaseAsset{
		{Name: "mywebscrcpy-linux-amd64", BrowserDownloadURL: "https://example.com/linux-amd64"},
		{Name: "mywebscrcpy-linux-arm64", BrowserDownloadURL: "https://example.com/linux-arm64"},
		{Name: "mywebscrcpy-macos-arm64", BrowserDownloadURL: "https://example.com/macos-arm64"},
		{Name: "mywebscrcpy-windows-amd64.exe", BrowserDownloadURL: "https://example.com/windows-amd64.exe"},
	}

	matched := MatchAsset(assets, "darwin", "arm64")
	if matched == nil || matched.Name != "mywebscrcpy-macos-arm64" {
		t.Fatalf("expected macos-arm64, got %+v", matched)
	}

	matchedWin := MatchAsset(assets, "windows", "amd64")
	if matchedWin == nil || matchedWin.Name != "mywebscrcpy-windows-amd64.exe" {
		t.Fatalf("expected windows-amd64.exe, got %+v", matchedWin)
	}
}

func TestMatchAssetPrefersCLIAndRejectsGUI(t *testing.T) {
	for _, target := range []struct{ os, arch, cli, legacy string }{
		{"linux", "amd64", "linux-amd64-cli", "linux-amd64"},
		{"linux", "arm64", "linux-arm64-cli", "linux-arm64"},
		{"darwin", "amd64", "macos-amd64-cli", "macos-amd64"},
		{"darwin", "arm64", "macos-arm64-cli", "macos-arm64"},
		{"windows", "amd64", "windows-cli-amd64.exe", "windows-amd64.exe"},
	} {
		gui := []ReleaseAsset{{Name: "mywebscrcpy-windows-gui-amd64.exe"}, {Name: "mywebscrcpy-v1.20.1-windows-x86_64-gui-setup.exe"}}
		if MatchAsset(gui, target.os, target.arch) != nil {
			t.Fatal("GUI asset selected for CLI")
		}
		assets := append(gui, ReleaseAsset{Name: "mywebscrcpy-" + target.legacy}, ReleaseAsset{Name: "mywebscrcpy-" + target.cli})
		matched := MatchAsset(assets, target.os, target.arch)
		if matched == nil || matched.Name != "mywebscrcpy-"+target.cli {
			t.Fatalf("%s/%s: expected CLI, got %+v", target.os, target.arch, matched)
		}
	}
}

func TestDownloaderAndApplier(t *testing.T) {
	content := "fake binary content v1.13.0"
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Length", "27")
		_, _ = w.Write([]byte(content))
	}))
	defer server.Close()

	tempDir, err := os.MkdirTemp("", "updater-test")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(tempDir)

	fakeExec := filepath.Join(tempDir, "mywebscrcpy")
	if err := os.WriteFile(fakeExec, []byte("fake binary content v1.12.0"), 0755); err != nil {
		t.Fatal(err)
	}

	applier := &Applier{executablePath: fakeExec}

	downloader := NewDownloader()
	downloader.httpClient = server.Client()

	downloadDest := filepath.Join(tempDir, "downloaded-bin")
	err = downloader.DownloadAsset(server.URL, int64(len(content)), downloadDest)
	if err != nil {
		t.Fatalf("download failed: %v", err)
	}

	progress := downloader.GetProgress()
	if progress.Downloaded != int64(len(content)) {
		t.Fatalf("expected downloaded %d, got %d", len(content), progress.Downloaded)
	}

	// 测试替换
	err = applier.ApplyUpdate(downloadDest)
	if err != nil {
		t.Fatalf("apply update failed: %v", err)
	}

	// 验证 fakeExec 内容已被新内容替换
	newContent, err := os.ReadFile(fakeExec)
	if err != nil {
		t.Fatal(err)
	}
	if string(newContent) != content {
		t.Fatalf("expected %q, got %q", content, string(newContent))
	}
}

func TestUpdaterHandlers(t *testing.T) {
	// 模拟 GitHub API 返回
	mockRelease := ReleaseInfo{
		TagName:     "v1.13.0",
		Name:        "v1.13.0 Release",
		Body:        "## 自动更新特性发布",
		PublishedAt: time.Now(),
		Assets: []ReleaseAsset{
			{Name: "mywebscrcpy-darwin-arm64", Size: 1024, BrowserDownloadURL: "https://example.com/asset"},
			{Name: "mywebscrcpy-linux-amd64", Size: 1024, BrowserDownloadURL: "https://example.com/asset-linux"},
		},
	}

	ghServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewEncoder(w).Encode(mockRelease)
	}))
	defer ghServer.Close()

	tempDir, err := os.MkdirTemp("", "updater-svc-test")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(tempDir)

	fakeExec := filepath.Join(tempDir, "mywebscrcpy")
	_ = os.WriteFile(fakeExec, []byte("bin"), 0755)

	svc := &Service{
		checker: &Checker{
			repo:           "mock/repo",
			currentVersion: "v1.12.0",
			httpClient:     ghServer.Client(),
		},
		downloader: NewDownloader(),
		applier:    &Applier{executablePath: fakeExec},
	}
	// 将 checker 请求重定向到 mock server
	svc.checker.httpClient.Transport = roundTripperFunc(func(r *http.Request) (*http.Response, error) {
		req, _ := http.NewRequest(r.Method, ghServer.URL, r.Body)
		return http.DefaultTransport.RoundTrip(req)
	})

	mux := http.NewServeMux()
	RegisterRoutes(mux, svc)

	// 1. 测试 GET /api/version/check
	req := httptest.NewRequest("GET", "/api/version/check", nil)
	rr := httptest.NewRecorder()
	mux.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("check failed with status %d: %s", rr.Code, rr.Body.String())
	}

	var checkRes CheckResult
	if err := json.Unmarshal(rr.Body.Bytes(), &checkRes); err != nil {
		t.Fatalf("unmarshal check res: %v", err)
	}
	if !checkRes.HasUpdate {
		t.Fatalf("expected has_update=true")
	}
	if checkRes.LatestVersion != "v1.13.0" {
		t.Fatalf("expected v1.13.0, got %s", checkRes.LatestVersion)
	}
}

type roundTripperFunc func(*http.Request) (*http.Response, error)

func (f roundTripperFunc) RoundTrip(r *http.Request) (*http.Response, error) {
	return f(r)
}
