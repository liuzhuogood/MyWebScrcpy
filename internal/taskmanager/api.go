package taskmanager

import (
	"bufio"
	"context"
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"log"
	"net"
	"net/http"
	"strings"
	"sync"
	"time"
)

// ConfigureAPI serves only the task's device and read-only template assets.
func (m *Manager) ConfigureAPI(handler http.Handler) error {
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		return err
	}
	m.apiURL = "http://" + listener.Addr().String()
	m.apiServer = &http.Server{ReadHeaderTimeout: 5 * time.Second, Handler: http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if !taskAPIPath(r) {
			http.Error(w, "not found", 404)
			return
		}
		token := strings.TrimPrefix(r.Header.Get("Authorization"), "Bearer ")
		var job *managedTask
		m.mu.Lock()
		for _, candidate := range m.procs {
			if token != "" && candidate.apiToken == token && !candidate.killed && candidate.ctx.Err() == nil {
				job = candidate
				break
			}
		}
		m.mu.Unlock()
		if job == nil || r.URL.Query().Get("serial") != job.serial {
			http.Error(w, "unauthorized", 401)
			return
		}
		if strings.HasPrefix(r.URL.Path, "/api/templates/") {
			id := strings.TrimPrefix(r.URL.Path, "/api/templates/")
			id = strings.TrimSuffix(id, "/image")
			if m.TemplateReadable == nil || !m.TemplateReadable(id, job.serial) {
				http.Error(w, "template forbidden", 403)
				return
			}
		}
		ctx, cancel := context.WithCancel(r.Context())
		defer cancel()
		go func() {
			select {
			case <-job.ctx.Done():
				cancel()
			case <-ctx.Done():
			}
		}()
		request := r.WithContext(ctx)
		if r.URL.Path == "/api/vision/stream" {
			handler.ServeHTTP(&taskSocketWriter{ResponseWriter: w, ctx: ctx}, request)
		} else {
			handler.ServeHTTP(w, request)
		}
	})}
	go func() {
		if err := m.apiServer.Serve(listener); err != nil && err != http.ErrServerClosed {
			log.Printf("[TaskRuntime] bridge failed: %v", err)
		}
	}()
	return nil
}

func taskAPIPath(r *http.Request) bool {
	if r.Method == http.MethodPost {
		return r.URL.Path == "/api/adb/commands"
	}
	if r.Method != http.MethodGet {
		return false
	}
	switch r.URL.Path {
	case "/api/ui/page-info", "/api/vision/stream", "/api/templates":
		return true
	}
	if !strings.HasPrefix(r.URL.Path, "/api/templates/") {
		return false
	}
	path := strings.TrimPrefix(r.URL.Path, "/api/templates/")
	parts := strings.Split(path, "/")
	return parts[0] != "" && parts[0] != "." && parts[0] != ".." && parts[0] != "export" && parts[0] != "import" && (len(parts) == 1 || len(parts) == 2 && parts[1] == "image")
}

// HTTP Server.Close does not reclaim hijacked sockets; bind them to the job.
type taskSocketWriter struct {
	http.ResponseWriter
	ctx context.Context
}

func (w *taskSocketWriter) Hijack() (net.Conn, *bufio.ReadWriter, error) {
	hijacker, ok := w.ResponseWriter.(http.Hijacker)
	if !ok {
		return nil, nil, fmt.Errorf("WebSocket transport does not support hijack")
	}
	connection, buffer, err := hijacker.Hijack()
	if err != nil {
		return nil, nil, err
	}
	owned := &taskSocket{Conn: connection, done: make(chan struct{})}
	go func() {
		select {
		case <-w.ctx.Done():
			_ = owned.Close()
		case <-owned.done:
		}
	}()
	return owned, buffer, nil
}

type taskSocket struct {
	net.Conn
	done chan struct{}
	once sync.Once
}

func (c *taskSocket) Close() error {
	var err error
	c.once.Do(func() { close(c.done); err = c.Conn.Close() })
	return err
}

func apiToken() (string, error) {
	value := make([]byte, 32)
	if _, err := rand.Read(value); err != nil {
		return "", fmt.Errorf("创建任务运行令牌失败: %w", err)
	}
	return hex.EncodeToString(value), nil
}
