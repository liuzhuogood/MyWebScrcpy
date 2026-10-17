package main

import (
	"bytes"
	"io/fs"
	"log"
	"net/http"
	"strings"
	"time"

	"mywebscrcpy/internal/skillsmarket"
)

func serveOpenAPISpec(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		w.Header().Set("Allow", http.MethodGet)
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	_, _ = w.Write(openAPISpec)
}

// Only Vue routes receive the shell. Missing assets and API paths must stay 404.
func newAppHandler(appRoot fs.FS) http.Handler {
	index, indexErr := fs.ReadFile(appRoot, "index.html")
	if indexErr != nil {
		log.Printf("新前端资源未构建: %v", indexErr)
	}
	files := http.FileServer(http.FS(appRoot))
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet && r.Method != http.MethodHead {
			w.Header().Set("Allow", "GET, HEAD")
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}
		switch {
		case r.URL.Path == "/" || r.URL.Path == "/index.html" || r.URL.Path == "/player" ||
			r.URL.Path == "/dashboard" || r.URL.Path == "/files" || r.URL.Path == "/tasks" ||
			r.URL.Path == "/dependencies" || r.URL.Path == "/ai-integration" ||
			skillsmarket.IsSkillDetailRoutePath(r.URL.Path):
			if indexErr != nil {
				http.Error(w, "web app unavailable", http.StatusServiceUnavailable)
				return
			}
			w.Header().Set("Content-Type", "text/html; charset=utf-8")
			w.Header().Set("Cache-Control", "no-cache")
			http.ServeContent(w, r, "index.html", time.Time{}, bytes.NewReader(index))
		default:
			if strings.HasPrefix(r.URL.Path, "/api/") || strings.HasPrefix(r.URL.Path, "/ws") {
				http.NotFound(w, r)
				return
			}
			files.ServeHTTP(w, r)
		}
	})
}
