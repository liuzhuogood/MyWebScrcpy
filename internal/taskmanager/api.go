package taskmanager

import (
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"log"
	"net"
	"net/http"
	"strings"
	"time"
)

// ConfigureAPI creates a task-only bridge, independent of GUI ports and TLS.
func (m *Manager) ConfigureAPI(handler http.Handler) error {
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		return err
	}
	m.apiURL = "http://" + listener.Addr().String()
	m.apiServer = &http.Server{ReadHeaderTimeout: 5 * time.Second, Handler: http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if !(r.URL.Path == "/api/adb/commands" && r.Method == http.MethodPost || r.URL.Path == "/api/ui/page-info" && r.Method == http.MethodGet) {
			http.Error(w, "not found", 404)
			return
		}
		token := strings.TrimPrefix(r.Header.Get("Authorization"), "Bearer ")
		serial := ""
		m.mu.Lock()
		for _, job := range m.procs {
			if job.apiToken == token && token != "" && !job.killed {
				serial = job.serial
				break
			}
		}
		m.mu.Unlock()
		if serial == "" || r.URL.Query().Get("serial") != serial {
			http.Error(w, "unauthorized", 401)
			return
		}
		handler.ServeHTTP(w, r)
	})}
	go func() {
		if err := m.apiServer.Serve(listener); err != nil && err != http.ErrServerClosed {
			log.Printf("[TaskRuntime] bridge failed: %v", err)
		}
	}()
	return nil
}

func apiToken() (string, error) {
	value := make([]byte, 32)
	if _, err := rand.Read(value); err != nil {
		return "", fmt.Errorf("创建任务运行令牌失败: %w", err)
	}
	return hex.EncodeToString(value), nil
}
