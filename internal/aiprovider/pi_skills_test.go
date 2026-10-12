package aiprovider

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync/atomic"
	"testing"
	"time"
)

func TestResolvePiSkillsDir(t *testing.T) {
	root := t.TempDir()
	expected := filepath.Join(root, "skills")
	if err := os.Mkdir(expected, 0700); err != nil {
		t.Fatal(err)
	}
	cwd, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	relative, err := filepath.Rel(cwd, expected)
	if err != nil {
		t.Fatal(err)
	}
	actual, err := resolvePiSkillsDir(relative)
	if err != nil || actual != expected {
		t.Fatalf("dir=%q err=%v", actual, err)
	}
	actual, err = resolvePiSkillsDir(filepath.Join(root, "absent"))
	if err != nil || actual != "" {
		t.Fatalf("missing directory guessed another source: %q %v", actual, err)
	}
	notDir := filepath.Join(root, "file")
	if err := os.WriteFile(notDir, []byte("not a directory"), 0600); err != nil {
		t.Fatal(err)
	}
	if _, err := resolvePiSkillsDir(notDir); err == nil {
		t.Fatal("accepted file as skills directory")
	}
}

func TestRealPiLoadsProjectSkills(t *testing.T) {
	paths := bundledPiForTest(t)
	root := t.TempDir()
	skillsDir := filepath.Join(root, ".pi", "skills")
	skillDir := filepath.Join(skillsDir, "project-review")
	referenceDir := filepath.Join(skillDir, "references")
	if err := os.MkdirAll(referenceDir, 0700); err != nil {
		t.Fatal(err)
	}
	skillPath := filepath.Join(skillDir, "SKILL.md")
	referencePath := filepath.Join(referenceDir, "rules.md")
	if err := os.WriteFile(referencePath, []byte("PROJECT_REFERENCE_MARKER"), 0600); err != nil {
		t.Fatal(err)
	}
	var calls atomic.Int32
	var description atomic.Value
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
		registered := false
		for _, message := range body.Messages {
			if message.Role == "system" && strings.Contains(fmt.Sprint(message.Content), description.Load().(string)) && strings.Contains(fmt.Sprint(message.Content), skillPath) {
				registered = true
			}
		}
		if !registered {
			t.Error("project skill metadata was not added to the Pi system prompt")
		}
		w.Header().Set("Content-Type", "text/event-stream")
		send := func(delta any, finish string) {
			var stop any
			if finish != "" {
				stop = finish
			}
			data, _ := json.Marshal(map[string]any{"id": "skill-check", "object": "chat.completion.chunk", "created": 1, "model": "mock-doc", "choices": []any{map[string]any{"index": 0, "delta": delta, "finish_reason": stop}}})
			fmt.Fprintf(w, "data: %s\n\n", data)
			w.(http.Flusher).Flush()
		}
		turn := (calls.Add(1) - 1) % 3
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
		switch turn {
		case 0:
			read("call_skill", skillPath)
		case 1:
			if !hasResult("PROJECT_SKILL_BODY_MARKER") {
				t.Error("Pi did not actually read the skill instructions")
			}
			read("call_reference", referencePath)
		case 2:
			if !hasResult("PROJECT_REFERENCE_MARKER") {
				t.Error("Pi did not actually read the skill reference")
			}
			send(map[string]any{"role": "assistant", "content": "项目技能及参考资料已读取。"}, "")
			send(map[string]any{}, "stop")
		}
		fmt.Fprint(w, "data: [DONE]\n\n")
	}))
	defer model.Close()
	cfg := ModelConfig{Provider: "custom", BaseURL: model.URL + "/v1", APIKey: "skill-test-key", Model: "mock-doc"}
	req := ChatRequest{Messages: []ChatMessage{{Role: "system", Content: "文档助手"}, {Role: "user", Content: "使用项目技能检查文档"}}, Document: "# 文档"}
	for _, version := range []string{"第一版项目审核技能", "第二版项目审核技能"} {
		description.Store(version)
		text := "---\nname: project-review\ndescription: " + version + "\n---\n\nPROJECT_SKILL_BODY_MARKER\n查阅 references/rules.md 并给出建议。"
		if err := os.WriteFile(skillPath, []byte(text), 0600); err != nil {
			t.Fatal(err)
		}
		ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
		var events []ChatEvent
		paths.SkillsDir = skillsDir
		err := runPi(ctx, paths, cfg, req, func(event ChatEvent) error { events = append(events, event); return nil })
		cancel()
		if err != nil {
			t.Fatal(err)
		}
		reads := 0
		for _, event := range events {
			if event.Type == "tool_start" && event.ToolName == "read" {
				reads++
			}
		}
		if reads != 2 || events[len(events)-1].Type != "done" {
			t.Fatalf("unexpected events: %+v", events)
		}
	}
	if calls.Load() != 6 {
		t.Fatalf("model calls=%d", calls.Load())
	}
	if _, err := os.Stat(skillPath); err != nil {
		t.Fatal("persistent project skill was removed after a request")
	}
}
