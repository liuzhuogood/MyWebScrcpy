package matchingservice

import (
	"context"
	"encoding/json"
	"errors"
	"log"
	"net"
	"net/http"
	"strings"
	"time"
)

func (m *Manager) RegisterRoutes(mux *http.ServeMux) {
	mux.HandleFunc("GET /api/matching-service/status", func(w http.ResponseWriter, r *http.Request) { writeJSON(w, 200, m.Status()) })
	mux.HandleFunc("POST /api/matching-service/check", func(w http.ResponseWriter, r *http.Request) { _, err := m.Check(r.Context()); m.respond(w, err) })
	mux.HandleFunc("POST /api/matching-service/start", func(w http.ResponseWriter, r *http.Request) {
		action := func() error { return m.Start(r.Context()) }
		var err error
		if m.options.Control != nil {
			err = m.options.Control(true, action)
		} else {
			err = action()
		}
		m.respond(w, err)
	})
	mux.HandleFunc("POST /api/matching-service/stop", func(w http.ResponseWriter, r *http.Request) {
		var err error
		if m.options.Control != nil {
			err = m.options.Control(false, m.Stop)
		} else {
			err = m.Stop()
		}
		m.respond(w, err)
	})
}
func writeJSON(w http.ResponseWriter, status int, value any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(value)
}
func (m *Manager) respond(w http.ResponseWriter, err error) {
	if err != nil {
		writeJSON(w, 409, map[string]any{"error": err.Error(), "status": m.Status()})
		return
	}
	writeJSON(w, 200, m.Status())
}

func (m *Manager) bridge(ctx context.Context, token string) (*http.Server, string, error) {
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		return nil, "", err
	}
	server := &http.Server{ReadHeaderTimeout: 5 * time.Second, BaseContext: func(net.Listener) context.Context { return ctx }}
	server.Handler = http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Authorization") != "Bearer "+token {
			http.Error(w, "unauthorized", 401)
			return
		}
		devices := m.options.Devices()
		serial := r.URL.Query().Get("serial")
		var active bool
		for _, s := range devices {
			if s == serial {
				active = true
				break
			}
		}
		if r.Method == http.MethodPost && r.URL.Path == "/api/vision/frame" {
			if m.options.Frame == nil {
				http.Error(w, "not found", 404)
				return
			}
			if !active {
				http.Error(w, "device forbidden", 403)
				return
			}
			r.Header.Del("Authorization")
			m.options.Frame.ServeHTTP(w, r)
			return
		}
		if r.Method != "GET" {
			http.Error(w, "not found", 404)
			return
		}
		if r.URL.Path == "/api/matching-runtime/devices" {
			if devices == nil {
				devices = []string{}
			}
			writeJSON(w, 200, devices)
			return
		}
		if !active {
			http.Error(w, "device forbidden", 403)
			return
		}
		if r.URL.Path == "/api/vision/stream" {
			m.options.Vision.ServeHTTP(w, r)
			return
		}
		if r.URL.Path == "/api/templates" {
			m.options.API.ServeHTTP(w, r)
			return
		}
		if strings.HasPrefix(r.URL.Path, "/api/templates/") {
			parts := strings.Split(strings.TrimPrefix(r.URL.Path, "/api/templates/"), "/")
			if parts[0] != "" && parts[0] != "export" && parts[0] != "import" && (len(parts) == 1 || len(parts) == 2 && parts[1] == "image") {
				if m.options.TemplateReadable != nil && m.options.TemplateReadable(parts[0], serial) {
					m.options.API.ServeHTTP(w, r)
					return
				}
				http.Error(w, "template forbidden", 403)
				return
			}
		}
		http.Error(w, "not found", 404)
	})
	go func() {
		if err := server.Serve(listener); err != nil && !errors.Is(err, http.ErrServerClosed) {
			log.Printf("[MatchingService] bridge failed: %v", err)
		}
	}()
	return server, "http://" + listener.Addr().String(), nil
}
