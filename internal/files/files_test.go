package files

import (
	"bytes"
	"encoding/json"
	"io"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func newTestManager(t *testing.T, token string) *Manager {
	t.Helper()
	m, err := NewManager(t.TempDir(), token, 8)
	if err != nil {
		t.Fatal(err)
	}
	return m
}

func TestManagerListSearchAndSort(t *testing.T) {
	m := newTestManager(t, "")
	if err := os.WriteFile(filepath.Join(m.Root(), "z.txt"), []byte("z"), 0644); err != nil {
		t.Fatal(err)
	}
	if _, err := m.CreateFolder("", "图片"); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(m.Root(), "图片", "cat.jpg"), []byte("image"), 0644); err != nil {
		t.Fatal(err)
	}
	result, err := m.List("", "", "all", "name", "asc", 1, 100)
	if err != nil {
		t.Fatal(err)
	}
	if len(result.Items) != 2 || result.Items[0].Kind != "folder" || result.Items[0].Name != "图片" {
		t.Fatalf("unexpected list: %#v", result.Items)
	}
	page, err := m.List("", "", "all", "name", "asc", 1, 1)
	if err != nil || len(page.Items) != 1 || !page.HasMore {
		t.Fatalf("expected paginated list, result=%#v err=%v", page, err)
	}
	result, err = m.List("", "cat", "image", "name", "asc", 1, 100)
	if err != nil {
		t.Fatal(err)
	}
	if len(result.Items) != 1 || result.Items[0].Path != "图片/cat.jpg" {
		t.Fatalf("unexpected search result: %#v", result.Items)
	}
}

func TestManagerUploadConflictAndLimit(t *testing.T) {
	m := newTestManager(t, "")
	if _, err := m.Upload("", "a.txt", "fail", strings.NewReader("1234")); err != nil {
		t.Fatal(err)
	}
	if _, err := m.Upload("", "a.txt", "fail", strings.NewReader("again")); err != ErrConflict {
		t.Fatalf("expected conflict, got %v", err)
	}
	item, err := m.Upload("", "a.txt", "rename", strings.NewReader("copy"))
	if err != nil || item.Path != "a (1).txt" {
		t.Fatalf("expected renamed copy, item=%#v err=%v", item, err)
	}
	if _, err := m.Upload("", "large.txt", "fail", strings.NewReader("123456789")); err == nil {
		t.Fatal("expected upload size error")
	}
}

func TestManagerMoveDeleteUndoAndTraversal(t *testing.T) {
	m := newTestManager(t, "")
	if _, err := m.CreateFolder("", "目标"); err != nil {
		t.Fatal(err)
	}
	if _, err := m.Upload("", "a.txt", "fail", strings.NewReader("data")); err != nil {
		t.Fatal(err)
	}
	item, err := m.Move("a.txt", "目标", "fail")
	if err != nil || item.Path != "目标/a.txt" {
		t.Fatalf("move failed: %#v %v", item, err)
	}
	item, err = m.Rename(item.Path, "renamed.txt", "fail")
	if err != nil || item.Path != "目标/renamed.txt" {
		t.Fatalf("rename failed: %#v %v", item, err)
	}
	token, err := m.Delete([]string{item.Path})
	if err != nil || token == "" {
		t.Fatalf("delete failed: %q %v", token, err)
	}
	if _, err := os.Stat(filepath.Join(m.Root(), "目标", "renamed.txt")); !os.IsNotExist(err) {
		t.Fatalf("deleted file still exists: %v", err)
	}
	if err := m.Undo(token); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(m.Root(), "目标", "renamed.txt")); err != nil {
		t.Fatalf("undo did not restore file: %v", err)
	}
	if _, err := m.List("../", "", "all", "name", "asc", 1, 10); err != ErrForbidden {
		t.Fatalf("expected traversal rejection, got %v", err)
	}
	if _, err := m.List(".trash", "", "all", "name", "asc", 1, 10); err != ErrForbidden {
		t.Fatalf("expected trash rejection, got %v", err)
	}
}

func TestHandlerAuthTraversalAndDownload(t *testing.T) {
	m := newTestManager(t, "secret")
	if err := os.WriteFile(filepath.Join(m.Root(), "hello.txt"), []byte("hello"), 0644); err != nil {
		t.Fatal(err)
	}
	mux := http.NewServeMux()
	RegisterRoutes(mux, m)

	unauthorized := httptest.NewRecorder()
	mux.ServeHTTP(unauthorized, httptest.NewRequest(http.MethodGet, "/api/files", nil))
	if unauthorized.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401, got %d", unauthorized.Code)
	}

	request := httptest.NewRequest(http.MethodGet, "/api/files?path=../", nil)
	request.Header.Set("Authorization", "Bearer secret")
	traversal := httptest.NewRecorder()
	mux.ServeHTTP(traversal, request)
	if traversal.Code != http.StatusForbidden {
		t.Fatalf("expected 403, got %d", traversal.Code)
	}

	request = httptest.NewRequest(http.MethodGet, "/api/files/download?path=hello.txt", nil)
	request.Header.Set("Authorization", "Bearer secret")
	download := httptest.NewRecorder()
	mux.ServeHTTP(download, request)
	if download.Code != http.StatusOK || download.Body.String() != "hello" {
		t.Fatalf("unexpected download: %d %q", download.Code, download.Body.String())
	}

	request = httptest.NewRequest(http.MethodGet, "/api/files/download?path=../outside.txt", nil)
	request.Header.Set("Authorization", "Bearer secret")
	blocked := httptest.NewRecorder()
	mux.ServeHTTP(blocked, request)
	if blocked.Code != http.StatusForbidden {
		t.Fatalf("expected forbidden download, got %d", blocked.Code)
	}
}

func TestHandlerRejectsSymlinkOutsideRoot(t *testing.T) {
	m := newTestManager(t, "")
	outside := filepath.Join(t.TempDir(), "private.txt")
	if err := os.WriteFile(outside, []byte("private"), 0600); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(outside, filepath.Join(m.Root(), "link.txt")); err != nil {
		t.Skipf("symlink unavailable: %v", err)
	}
	mux := http.NewServeMux()
	RegisterRoutes(mux, m)
	request := httptest.NewRequest(http.MethodGet, "/api/files/download?path=link.txt", nil)
	response := httptest.NewRecorder()
	mux.ServeHTTP(response, request)
	if response.Code != http.StatusForbidden {
		t.Fatalf("expected symlink escape rejection, got %d %s", response.Code, response.Body.String())
	}
}

func TestHandlerUploadMultipart(t *testing.T) {
	m := newTestManager(t, "")
	mux := http.NewServeMux()
	RegisterRoutes(mux, m)
	var body bytes.Buffer
	writer := multipart.NewWriter(&body)
	if err := writer.WriteField("path", ""); err != nil {
		t.Fatal(err)
	}
	part, err := writer.CreateFormFile("file", "上传.txt")
	if err != nil {
		t.Fatal(err)
	}
	_, _ = io.WriteString(part, "content")
	if err := writer.Close(); err != nil {
		t.Fatal(err)
	}
	request := httptest.NewRequest(http.MethodPost, "/api/files/upload", &body)
	request.Header.Set("Content-Type", writer.FormDataContentType())
	response := httptest.NewRecorder()
	mux.ServeHTTP(response, request)
	if response.Code != http.StatusOK {
		t.Fatalf("upload failed: %d %s", response.Code, response.Body.String())
	}
	var payload struct {
		Item FileItem `json:"item"`
	}
	if err := json.Unmarshal(response.Body.Bytes(), &payload); err != nil {
		t.Fatal(err)
	}
	if payload.Item.Path != "上传.txt" {
		t.Fatalf("unexpected uploaded item: %#v", payload.Item)
	}
}
