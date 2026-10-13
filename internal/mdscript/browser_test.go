package mdscript

import (
	"context"
	"encoding/json"
	"fmt"
	"mywebscrcpy/internal/aiprovider"
	"mywebscrcpy/internal/taskmanager"
	"net/http"
	"net/http/httptest"
	"os"
	"os/signal"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// Opt-in fixture serves the production frontend and handlers with controlled model/device data.
func TestBrowserGenerationFixture(t *testing.T) {
	if os.Getenv("MYWEBSCRCPY_GENERATION_BROWSER_PROOF") != "1" {
		t.Skip("浏览器验收专用服务，默认不启动")
	}
	d, g, _, model := generationFixture(t)
	model.respond = func(ctx context.Context, input aiprovider.GenerationInput, attempt int) (string, error) {
		select {
		case <-ctx.Done():
			return "", ctx.Err()
		case <-time.After(2 * time.Second):
		}
		if strings.Contains(input.Document, "请求失败") {
			return "", fmt.Errorf("受控模型连接失败")
		}
		if strings.Contains(input.Document, "坐标不详") {
			return `{"status":"needs_input","python":"","issues":[{"step":"步骤 2","message":"请提供点击坐标"}]}`, nil
		}
		return ready(generatedFixture), nil
	}
	mgr := taskmanager.NewManager(d)
	defer mgr.Close()
	if err := mgr.ConfigureAPI(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		json.NewEncoder(w).Encode(map[string]any{"status": "completed", "exit_code": 0, "stdout": "Browser Phone\n"})
	})); err != nil {
		t.Fatal(err)
	}
	executor := NewExecutor(d, mgr, func(string) (bool, bool) { return true, true }, nil)
	executor.ArtifactCheck = g.Check
	mux := http.NewServeMux()
	handler := NewHandler(d, executor, nil)
	handler.Generator = g
	handler.RegisterRoutes(mux)
	taskmanager.NewHandler(mgr).RegisterRoutes(mux)
	respond := func(path string, value any) {
		mux.HandleFunc(path, func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Content-Type", "application/json")
			json.NewEncoder(w).Encode(value)
		})
	}
	respond("GET /api/devices", []any{map[string]any{"serial": "device-A", "alias": "验收设备", "state": "device"}})
	respond("GET /api/scheduler/entries", map[string]any{"entries": []any{}})
	respond("GET /api/templates", map[string]any{"templates": []any{}})
	respond("GET /api/ai/config", map[string]any{"provider": "custom", "base_url": "http://controlled-model/v1", "model": "fixture", "api_key": "", "has_key": false})
	frontend, err := filepath.Abs("../../web/dist")
	if err != nil {
		t.Fatal(err)
	}
	mux.HandleFunc("GET /scripts", func(w http.ResponseWriter, r *http.Request) {
		http.ServeFile(w, r, filepath.Join(frontend, "index.html"))
	})
	mux.Handle("GET /assets/", http.FileServer(http.Dir(frontend)))
	server := httptest.NewServer(mux)
	defer server.Close()
	fmt.Println("GENERATION_BROWSER_URL=" + server.URL + "/scripts")
	signalChannel := make(chan os.Signal, 1)
	signal.Notify(signalChannel, os.Interrupt)
	defer signal.Stop(signalChannel)
	<-signalChannel
}
