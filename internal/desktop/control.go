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
	"strconv"
	"strings"
	"sync"
)

type Ready struct {
	URL   string `json:"url"`
	Token string `json:"token"`
	PID   int    `json:"pid"`
}

type Control struct {
	token         string
	stop          chan struct{}
	once          sync.Once
	effectivePort int
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
	mux.HandleFunc("GET /api/desktop/config", c.authorize(func(w http.ResponseWriter, r *http.Request) {
		cfg, _ := LoadConfig()
		envPort, hasEnv := ResolveEnvPort()
		res := ConfigResponse{
			Port:         cfg.Port,
			Effective:    c.effectivePort,
			EnvPort:      envPort,
			IsOverridden: hasEnv,
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(res)
	}))
	mux.HandleFunc("PUT /api/desktop/config", c.authorize(func(w http.ResponseWriter, r *http.Request) {
		var body struct {
			Port int `json:"port"`
		}
		if err := json.NewDecoder(http.MaxBytesReader(w, r.Body, 4096)).Decode(&body); err != nil {
			http.Error(w, "配置格式错误", http.StatusBadRequest)
			return
		}
		if err := ValidatePort(body.Port); err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		if err := SaveConfig(Config{Port: body.Port}); err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		w.WriteHeader(http.StatusNoContent)
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
	host, portStr, err := net.SplitHostPort(listener.Addr().String())
	if err != nil {
		return err
	}
	ip := net.ParseIP(host)
	if ip == nil || !ip.IsLoopback() {
		return fmt.Errorf("桌面服务必须监听回环地址")
	}
	c.effectivePort, _ = strconv.Atoi(portStr)
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
	if err = os.Rename(path+".tmp", path); err != nil {
		return err
	}
	endpoint := struct {
		URL  string `json:"url"`
		Port int    `json:"port"`
		PID  int    `json:"pid"`
	}{
		URL:  "http://" + listener.Addr().String(),
		Port: c.effectivePort,
		PID:  os.Getpid(),
	}
	if endData, err := json.MarshalIndent(endpoint, "", "  "); err == nil {
		_ = os.WriteFile(EndpointFile, endData, 0644)
	}
	return nil
}
