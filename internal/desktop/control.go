package desktop

import (
	"crypto/rand"
	"crypto/subtle"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"mywebscrcpy/internal/dependencies"
	"net"
	"net/http"
	"os"
	"strings"
	"sync"
)

type Ready struct {
	URL   string `json:"url"`
	Token string `json:"token"`
	PID   int    `json:"pid"`
}

type Control struct {
	token string
	stop  chan struct{}
	once  sync.Once
}

func NewControl() (*Control, error) {
	bytes := make([]byte, 32)
	if _, err := rand.Read(bytes); err != nil {
		return nil, err
	}
	return &Control{token: hex.EncodeToString(bytes), stop: make(chan struct{})}, nil
}
func (c *Control) Done() <-chan struct{} { return c.stop }
func (c *Control) Register(mux *http.ServeMux) {
	dependencies.RegisterSettings(mux, c.authorize)
	registerVerification(mux, c.authorize)
	mux.HandleFunc("GET /api/desktop/health", c.authorize(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]string{"status": "ready"})
	}))
	mux.HandleFunc("POST /api/desktop/shutdown", c.authorize(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusAccepted)
		c.once.Do(func() { close(c.stop) })
	}))
}
func (c *Control) authorize(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		supplied := strings.TrimPrefix(r.Header.Get("Authorization"), "Bearer ")
		if subtle.ConstantTimeCompare([]byte(supplied), []byte(c.token)) != 1 {
			http.Error(w, "unauthorized", http.StatusUnauthorized)
			return
		}
		next(w, r)
	}
}
func (c *Control) Publish(path string, listener net.Listener) error {
	host, _, err := net.SplitHostPort(listener.Addr().String())
	if err != nil {
		return err
	}
	ip := net.ParseIP(host)
	if ip == nil || !ip.IsLoopback() {
		return fmt.Errorf("桌面服务必须监听回环地址")
	}
	value := Ready{URL: "http://" + listener.Addr().String(), Token: c.token, PID: os.Getpid()}
	contents, err := json.Marshal(value)
	if err != nil {
		return err
	}
	file, err := os.OpenFile(path+".tmp", os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0600)
	if err != nil {
		return err
	}
	if _, err = file.Write(contents); err != nil {
		file.Close()
		os.Remove(file.Name())
		return err
	}
	if err = file.Close(); err != nil {
		os.Remove(file.Name())
		return err
	}
	return os.Rename(path+".tmp", path)
}
