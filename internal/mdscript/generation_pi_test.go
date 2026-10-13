package mdscript

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"sync/atomic"
	"testing"
	"testing/fstest"
	"time"

	"mywebscrcpy/internal/aiprovider"
	"mywebscrcpy/internal/pibundle"
	"mywebscrcpy/internal/taskmanager"
)

func TestRealPiSkillGenerationAndPythonExecution(t *testing.T) {
	payload := os.Getenv("MYWEBSCRCPY_PI_TEST_PAYLOAD")
	if payload == "" {
		t.Skip("设置 MYWEBSCRCPY_PI_TEST_PAYLOAD 验证真实 Pi 生成")
	}
	paths, err := pibundle.PrepareFrom(t.TempDir(), fstest.MapFS{}, os.DirFS(payload))
	if err != nil {
		t.Fatal(err)
	}
	d, g, script, _ := generationFixture(t)
	var turns atomic.Int32
	model := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var body struct {
			Messages []struct {
				Role    string `json:"role"`
				Content any    `json:"content"`
			} `json:"messages"`
		}
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			t.Error(err)
			w.WriteHeader(400)
			return
		}
		w.Header().Set("Content-Type", "text/event-stream")
		send := func(delta any, finish string) {
			var stop any
			if finish != "" {
				stop = finish
			}
			data, _ := json.Marshal(map[string]any{"id": "generation", "object": "chat.completion.chunk", "created": 1, "model": "fixture", "choices": []any{map[string]any{"index": 0, "delta": delta, "finish_reason": stop}}})
			fmt.Fprintf(w, "data: %s\n\n", data)
			w.(http.Flusher).Flush()
		}
		read := func(id, path string) {
			args, _ := json.Marshal(map[string]string{"path": path})
			send(map[string]any{"role": "assistant", "tool_calls": []any{map[string]any{"index": 0, "id": id, "type": "function", "function": map[string]any{"name": "read", "arguments": string(args)}}}}, "")
			send(map[string]any{}, "tool_calls")
		}
		hasResult := func(marker string) bool {
			for _, message := range body.Messages {
				if message.Role == "tool" && strings.Contains(fmt.Sprint(message.Content), marker) {
					return true
				}
			}
			return false
		}
		switch turns.Add(1) {
		case 1:
			read("skill", "skill/SKILL.md")
		case 2:
			if !hasResult("name: markdown-to-python") {
				t.Error("Pi did not read frozen skill")
			}
			read("reference", "skill/references/sdk-api.md")
		case 3:
			if !hasResult("# SDK API sdk-v1") {
				t.Error("Pi did not read frozen SDK contract")
			}
			read("api", "api/openapi.json")
		default:
			if !hasResult("/api/adb/commands") {
				t.Error("Pi did not read actual API document")
			}
			send(map[string]any{"role": "assistant", "content": ready(generatedFixture)}, "")
			send(map[string]any{}, "stop")
		}
		fmt.Fprint(w, "data: [DONE]\n\n")
	}))
	defer model.Close()
	service := aiprovider.NewService(d, paths)
	if err := service.SaveConfig(aiprovider.ModelConfig{Provider: "custom", BaseURL: model.URL + "/v1", Model: "fixture", APIKey: "controlled-test-key"}); err != nil {
		t.Fatal(err)
	}
	g.Provider = service
	reads := 0
	ctx, cancel := context.WithTimeout(context.Background(), 45*time.Second)
	defer cancel()
	if err := g.Generate(ctx, *script, func(event GenerationEvent) error {
		if event.Type == "tool_end" && event.ToolName == "read" && !event.IsError {
			reads++
		}
		return nil
	}); err != nil {
		t.Fatal(err)
	}
	if reads != 3 || turns.Load() != 4 {
		t.Fatalf("reads=%d turns=%d", reads, turns.Load())
	}
	var deviceCalls atomic.Int32
	mgr := taskmanager.NewManager(d)
	defer mgr.Close()
	if err := mgr.ConfigureAPI(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var request struct {
			Args    []string `json:"args"`
			Timeout int64    `json:"timeout_ms"`
		}
		json.NewDecoder(r.Body).Decode(&request)
		if r.URL.Query().Get("serial") != "device-A" || len(request.Args) != 3 || request.Args[2] != "ro.product.model" || request.Timeout != 10000 {
			t.Errorf("unexpected request: %+v", request)
		}
		deviceCalls.Add(1)
		json.NewEncoder(w).Encode(map[string]any{"status": "completed", "exit_code": 0, "stdout": "Controlled Model\n"})
	})); err != nil {
		t.Fatal(err)
	}
	executor := NewExecutor(d, mgr, func(string) (bool, bool) { return true, true }, nil)
	executor.ArtifactCheck = g.Check
	results, err := executor.Execute(script.ID)
	if err != nil {
		t.Fatal(err)
	}
	waitTask(t, d, results[0].Task.ID, "completed")
	if deviceCalls.Load() != 1 {
		t.Fatal("generated Python did not execute device API")
	}
}
