package main

import (
	"bytes"
	"io/fs"
	"log"
	"net/http"
	"strings"
	"time"
)

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
		switch r.URL.Path {
		case "/", "/index.html", "/player", "/dashboard", "/files", "/scripts", "/dependencies":
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
