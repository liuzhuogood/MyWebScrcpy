package ws

import (
	"bytes"
	"encoding/json"
	"image"
	"image/color"
	"image/png"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func testPNG(t *testing.T, w, h int) []byte {
	t.Helper()
	img := image.NewRGBA(image.Rect(0, 0, w, h))
	img.Set(0, 0, color.RGBA{R: 255, A: 255})
	var buf bytes.Buffer
	if err := png.Encode(&buf, img); err != nil {
		t.Fatal(err)
	}
	return buf.Bytes()
}

func testScreenshotHub(t *testing.T) (*Hub, *screenshotManager, *http.ServeMux) {
	t.Helper()
	dir := t.TempDir()
	h := &Hub{}
	m := newScreenshotManager(dir, defaultScreenshotsQuota)
	screenshotManagers.Store(h, m)
	t.Cleanup(func() { screenshotManagers.Delete(h) })
	mux := http.NewServeMux()
	h.RegisterScreenshotRoutes(mux)
	return h, m, mux
}

func uploadScreenshot(t *testing.T, mux *http.ServeMux, serial string, body []byte) *httptest.ResponseRecorder {
	t.Helper()
	req := httptest.NewRequest(http.MethodPost, "/api/screenshots?serial="+serial, bytes.NewReader(body))
	req.Header.Set("Content-Type", "image/png")
	w := httptest.NewRecorder()
	mux.ServeHTTP(w, req)
	return w
}

func TestScreenshotUploadListGetDelete(t *testing.T) {
	_, _, mux := testScreenshotHub(t)
	pngData := testPNG(t, 3, 2)

	w := uploadScreenshot(t, mux, "phone-a", pngData)
	if w.Code != http.StatusCreated {
		t.Fatalf("upload status=%d body=%s", w.Code, w.Body.String())
	}
	var created screenshotEntry
	if err := json.Unmarshal(w.Body.Bytes(), &created); err != nil {
		t.Fatal(err)
	}
	if !strings.HasPrefix(created.ID, "shot_") || created.Serial != "phone-a" || created.Width != 3 || created.Height != 2 || created.Bytes != int64(len(pngData)) {
		t.Fatalf("unexpected created: %+v", created)
	}
	if created.CreatedAt.IsZero() {
		t.Fatal("created_at missing")
	}

	list := httptest.NewRecorder()
	mux.ServeHTTP(list, httptest.NewRequest(http.MethodGet, "/api/screenshots?serial=phone-a", nil))
	if list.Code != http.StatusOK {
		t.Fatalf("list status=%d", list.Code)
	}
	var listBody struct {
		Screenshots []screenshotEntry `json:"screenshots"`
	}
	if err := json.Unmarshal(list.Body.Bytes(), &listBody); err != nil {
		t.Fatal(err)
	}
	if len(listBody.Screenshots) != 1 || listBody.Screenshots[0].ID != created.ID {
		t.Fatalf("list body=%s", list.Body.String())
	}

	view := httptest.NewRecorder()
	mux.ServeHTTP(view, httptest.NewRequest(http.MethodGet, "/api/screenshots/"+created.ID+"?serial=phone-a", nil))
	if view.Code != http.StatusOK || view.Header().Get("Content-Type") != "image/png" {
		t.Fatalf("view status=%d ct=%q", view.Code, view.Header().Get("Content-Type"))
	}
	if view.Header().Get("Content-Disposition") != "" {
		t.Fatalf("inline view must not set Content-Disposition, got %q", view.Header().Get("Content-Disposition"))
	}
	if !bytes.Equal(view.Body.Bytes(), pngData) {
		t.Fatal("view body mismatch")
	}

	download := httptest.NewRecorder()
	mux.ServeHTTP(download, httptest.NewRequest(http.MethodGet, "/api/screenshots/"+created.ID+"?serial=phone-a&download=1", nil))
	if download.Code != http.StatusOK {
		t.Fatalf("download status=%d", download.Code)
	}
	if !strings.HasPrefix(download.Header().Get("Content-Disposition"), "attachment") {
		t.Fatalf("download disposition=%q", download.Header().Get("Content-Disposition"))
	}

	del := httptest.NewRecorder()
	mux.ServeHTTP(del, httptest.NewRequest(http.MethodDelete, "/api/screenshots/"+created.ID+"?serial=phone-a", nil))
	if del.Code != http.StatusNoContent {
		t.Fatalf("delete status=%d body=%s", del.Code, del.Body.String())
	}
	gone := httptest.NewRecorder()
	mux.ServeHTTP(gone, httptest.NewRequest(http.MethodGet, "/api/screenshots/"+created.ID+"?serial=phone-a", nil))
	if gone.Code != http.StatusNotFound {
		t.Fatalf("after delete status=%d", gone.Code)
	}
}

func TestScreenshotSerialIsolation(t *testing.T) {
	_, _, mux := testScreenshotHub(t)
	pngData := testPNG(t, 1, 1)
	w := uploadScreenshot(t, mux, "phone-a", pngData)
	var created screenshotEntry
	if err := json.Unmarshal(w.Body.Bytes(), &created); err != nil {
		t.Fatal(err)
	}
	for _, tc := range []struct {
		name, method, target string
		want                 int
	}{
		{"cross-serial view", http.MethodGet, "/api/screenshots/" + created.ID + "?serial=phone-b", http.StatusBadRequest},
		{"cross-serial delete", http.MethodDelete, "/api/screenshots/" + created.ID + "?serial=phone-b", http.StatusBadRequest},
		{"missing serial list", http.MethodGet, "/api/screenshots", http.StatusBadRequest},
		{"missing serial view", http.MethodGet, "/api/screenshots/" + created.ID, http.StatusBadRequest},
		{"unknown id", http.MethodGet, "/api/screenshots/shot_nope?serial=phone-a", http.StatusNotFound},
		{"invalid png", http.MethodPost, "/api/screenshots?serial=phone-a", http.StatusBadRequest},
	} {
		t.Run(tc.name, func(t *testing.T) {
			var req *http.Request
			if tc.name == "invalid png" {
				req = httptest.NewRequest(http.MethodPost, tc.target, strings.NewReader("not-a-png"))
			} else {
				req = httptest.NewRequest(tc.method, tc.target, nil)
			}
			rec := httptest.NewRecorder()
			mux.ServeHTTP(rec, req)
			if rec.Code != tc.want {
				t.Fatalf("status=%d want=%d body=%s", rec.Code, tc.want, rec.Body.String())
			}
		})
	}
	other := httptest.NewRecorder()
	mux.ServeHTTP(other, httptest.NewRequest(http.MethodGet, "/api/screenshots?serial=phone-b", nil))
	var body struct {
		Screenshots []screenshotEntry `json:"screenshots"`
	}
	if err := json.Unmarshal(other.Body.Bytes(), &body); err != nil {
		t.Fatal(err)
	}
	if len(body.Screenshots) != 0 {
		t.Fatalf("other serial must be isolated: %+v", body.Screenshots)
	}
}

func TestScreenshotListOrderAndScan(t *testing.T) {
	h := &Hub{}
	dir := t.TempDir()
	m := newScreenshotManager(dir, defaultScreenshotsQuota)
	screenshotManagers.Store(h, m)
	t.Cleanup(func() { screenshotManagers.Delete(h) })
	mux := http.NewServeMux()
	h.RegisterScreenshotRoutes(mux)

	first := testPNG(t, 1, 1)
	w1 := uploadScreenshot(t, mux, "10.0.0.104:5555", first)
	var e1 screenshotEntry
	if err := json.Unmarshal(w1.Body.Bytes(), &e1); err != nil {
		t.Fatal(err)
	}
	time.Sleep(10 * time.Millisecond)
	second := testPNG(t, 2, 2)
	w2 := uploadScreenshot(t, mux, "10.0.0.104:5555", second)
	var e2 screenshotEntry
	if err := json.Unmarshal(w2.Body.Bytes(), &e2); err != nil {
		t.Fatal(err)
	}
	// Filenames must be sanitized (no colon).
	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatal(err)
	}
	for _, e := range entries {
		if strings.Contains(e.Name(), ":") {
			t.Fatalf("filename not sanitized: %q", e.Name())
		}
	}

	list := httptest.NewRecorder()
	mux.ServeHTTP(list, httptest.NewRequest(http.MethodGet, "/api/screenshots?serial=10.0.0.104:5555", nil))
	var body struct {
		Screenshots []screenshotEntry `json:"screenshots"`
	}
	if err := json.Unmarshal(list.Body.Bytes(), &body); err != nil {
		t.Fatal(err)
	}
	if len(body.Screenshots) != 2 || body.Screenshots[0].ID != e2.ID || body.Screenshots[1].ID != e1.ID {
		t.Fatalf("order must be desc: %s", list.Body.String())
	}

	// Restart: rebuild from disk scan.
	h2 := &Hub{}
	m2 := newScreenshotManager(dir, defaultScreenshotsQuota)
	if len(m2.entries) != 2 {
		t.Fatalf("scan rebuilt %d entries, want 2", len(m2.entries))
	}
	screenshotManagers.Store(h2, m2)
	t.Cleanup(func() { screenshotManagers.Delete(h2) })
	mux2 := http.NewServeMux()
	h2.RegisterScreenshotRoutes(mux2)
	list2 := httptest.NewRecorder()
	mux2.ServeHTTP(list2, httptest.NewRequest(http.MethodGet, "/api/screenshots?serial=10.0.0.104:5555", nil))
	var body2 struct {
		Screenshots []screenshotEntry `json:"screenshots"`
	}
	if err := json.Unmarshal(list2.Body.Bytes(), &body2); err != nil {
		t.Fatal(err)
	}
	if len(body2.Screenshots) != 2 {
		t.Fatalf("scan list=%s", list2.Body.String())
	}
	_ = filepath.Join(dir, "keep")
}
