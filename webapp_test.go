package main

import (
	"io/fs"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"testing/fstest"
)

func TestAppRoutesAndStaticAssets(t *testing.T) {
	app := fstest.MapFS{
		"index.html":    {Data: []byte(`<div id="app"></div><script src="/assets/app.js"></script>`)},
		"assets/app.js": {Data: []byte("console.log('app')")},
	}
	docs := fstest.MapFS{
		"api-docs.html":            {Data: []byte("API docs")},
		"api-docs-automation.html": {Data: []byte("Automation API docs")},
	}
	mux := http.NewServeMux()
	mux.HandleFunc("/api/devices", func(w http.ResponseWriter, r *http.Request) { w.Write([]byte("[]")) })
	mux.Handle("/api-docs.html", http.FileServer(http.FS(docs)))
	mux.Handle("/api-docs-automation.html", http.FileServer(http.FS(docs)))
	mux.Handle("/", newAppHandler(app))
	tests := []struct {
		method, path string
		status       int
		body         string
	}{
		{"GET", "/", 200, `id="app"`},
		{"GET", "/player?serial=device-1", 200, `id="app"`},
		{"GET", "/dashboard", 200, `id="app"`},
		{"GET", "/scripts", 404, ""},
		{"GET", "/tasks", 404, ""},
		{"GET", "/dependencies", 200, `id="app"`},
		{"GET", "/ai-integration", 200, `id="app"`},
		{"GET", "/ai-integration/skills/mywebscrcpy-device-base", 200, `id="app"`},
		{"GET", "/ai-integration/skills/unknown-skill", 200, `id="app"`},
		{"GET", "/ai-integration/skills/unknown-skill/extra", 404, ""},
		{"GET", "/files?serial=device-1", 200, `id="app"`},
		{"GET", "/assets/app.js", 200, "console.log"},
		{"GET", "/assets/missing.js", 404, ""},
		{"GET", "/api/missing", 404, ""},
		{"GET", "/ws", 404, ""},
		{"GET", "/app/", 404, ""},
		{"GET", "/player.html", 404, ""},
		{"GET", "/dashboard.html", 404, ""},
		{"GET", "/files.html", 404, ""},
		{"GET", "/scripts.html", 404, ""},
		{"GET", "/js/decoder.js", 404, ""},
		{"GET", "/api-docs.html", 200, "API docs"},
		{"GET", "/api-docs-automation.html", 200, "Automation API docs"},
		{"GET", "/api/devices", 200, "[]"},
		{"HEAD", "/player", 200, ""},
		{"POST", "/player", 405, ""},
	}
	for _, tt := range tests {
		t.Run(tt.method+" "+tt.path, func(t *testing.T) {
			w := httptest.NewRecorder()
			mux.ServeHTTP(w, httptest.NewRequest(tt.method, tt.path, nil))
			if w.Code != tt.status || !strings.Contains(w.Body.String(), tt.body) {
				t.Fatalf("status=%d body=%q", w.Code, w.Body.String())
			}
			if tt.method == "HEAD" && w.Body.Len() != 0 {
				t.Fatal("HEAD returned a body")
			}
			if w.Header().Get("Location") != "" {
				t.Fatal("unexpected compatibility redirect")
			}
		})
	}
}

func TestAppWithoutBuild(t *testing.T) {
	w := httptest.NewRecorder()
	newAppHandler(fstest.MapFS{}).ServeHTTP(w, httptest.NewRequest("GET", "/", nil))
	if w.Code != http.StatusServiceUnavailable {
		t.Fatalf("unbuilt app status=%d", w.Code)
	}
}

func TestBuiltAppUsesRootAssetPaths(t *testing.T) {
	page, err := fs.ReadFile(webFS, "web/dist/index.html")
	if err != nil {
		t.Fatalf("read embedded frontend: %v", err)
	}
	if !strings.Contains(string(page), `/assets/`) || !strings.Contains(string(page), `/app-icon.png`) || strings.Contains(string(page), `/app/`) {
		t.Fatal("built frontend must use root asset paths")
	}
	err = fs.WalkDir(webFS, ".", func(path string, entry fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if !entry.IsDir() && path != "web/api-docs.html" && path != "web/api-docs-automation.html" && !strings.HasPrefix(path, "web/dist/") {
			t.Errorf("unexpected embedded legacy/source file: %s", path)
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
}
