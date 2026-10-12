package aiprovider

import (
	"bufio"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"mywebscrcpy/internal/db"
)

func TestPiEventMapping(t *testing.T) {
	state := piStreamState{}
	var got []ChatEvent
	emit := func(e ChatEvent) error { got = append(got, e); return nil }
	records := []string{
		`{"type":"message_update","assistantMessageEvent":{"type":"thinking_delta","delta":"查阅资料"}}`,
		`{"type":"tool_execution_start","toolCallId":"call_1","toolName":"read","args":{"path":"current.md"}}`,
		`{"type":"tool_execution_update","toolCallId":"call_1","toolName":"read","partialResult":{"content":[{"type":"text","text":"第一段"}]}}`,
		`{"type":"tool_execution_update","toolCallId":"call_1","toolName":"read","partialResult":{"content":[{"type":"text","text":"第一段第二段"}]}}`,
		`{"type":"tool_execution_end","toolCallId":"call_1","toolName":"read","result":{"content":[{"type":"text","text":"完整结果"}]},"isError":false}`,
		`{"type":"message_end","message":{"role":"assistant","content":[{"type":"text","text":"最终回复"}],"stopReason":"stop"}}`,
		`{"type":"agent_end","willRetry":false}`,
		`{"type":"agent_settled"}`,
	}
	for i, record := range records {
		var raw piEvent
		if err := json.Unmarshal([]byte(record), &raw); err != nil {
			t.Fatal(err)
		}
		done, err := state.consume(raw, emit)
		if err != nil {
			t.Fatal(err)
		}
		if done != (i == len(records)-1) {
			t.Fatalf("premature completion at %d", i)
		}
	}
	if len(got) != 6 || got[0].Delta != "查阅资料" || got[1].ToolName != "read" || got[3].Output != "第一段第二段" || got[5].Reply != "最终回复" {
		t.Fatalf("unexpected stream: %+v", got)
	}
	var invalid piEvent
	_ = json.Unmarshal([]byte(`{"type":"tool_execution_start","tool":"read"}`), &invalid)
	if _, err := state.consume(invalid, emit); err == nil {
		t.Fatal("guessed an obsolete tool field")
	}
}

func TestPiFailureDoesNotComplete(t *testing.T) {
	state := piStreamState{}
	var raw piEvent
	_ = json.Unmarshal([]byte(`{"type":"message_end","message":{"role":"assistant","stopReason":"error","errorMessage":"model unavailable"}}`), &raw)
	_, _ = state.consume(raw, func(ChatEvent) error { return nil })
	done, err := state.consume(piEvent{Type: "agent_settled"}, func(ChatEvent) error { t.Fatal("unexpected success"); return nil })
	if done || err == nil || err.Error() != "model unavailable" {
		t.Fatalf("done=%v err=%v", done, err)
	}
}

// Uses the embedded standalone Pi with a controlled OpenAI-compatible server.
// This verifies RPC parsing and a real read tool without paid model calls.
func TestRealPiAssistantStream(t *testing.T) {
	paths := bundledPiForTest(t)
	var calls atomic.Int32
	model := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var body struct {
			Stream   bool `json:"stream"`
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
		if !body.Stream {
			t.Error("Pi did not request streaming")
		}
		w.Header().Set("Content-Type", "text/event-stream")
		flusher := w.(http.Flusher)
		send := func(delta any, finish any) {
			chunk := map[string]any{"id": "mock", "object": "chat.completion.chunk", "created": 1, "model": "mock-doc", "choices": []any{map[string]any{"index": 0, "delta": delta, "finish_reason": finish}}}
			data, _ := json.Marshal(chunk)
			fmt.Fprintf(w, "data: %s\n\n", data)
			flusher.Flush()
		}
		call := calls.Add(1)
		if call == 1 {
			send(map[string]any{"role": "assistant", "reasoning_content": "先阅读当前文档。"}, nil)
			time.Sleep(100 * time.Millisecond)
			tools := []any{}
			for index, tool := range []struct{ name, args string }{
				{"read", `{"path":"current.md"}`},
				{"grep", `{"pattern":"等待","path":"current.md"}`},
				{"find", `{"pattern":"*.md","path":"."}`},
			} {
				tools = append(tools, map[string]any{"index": index, "id": "call_" + tool.name, "type": "function", "function": map[string]any{"name": tool.name, "arguments": tool.args}})
			}
			send(map[string]any{"tool_calls": tools}, nil)
			send(map[string]any{}, "tool_calls")
		} else {
			found := false
			for _, message := range body.Messages {
				if message.Role == "tool" && strings.Contains(fmt.Sprint(message.Content), "等待 2 秒") {
					found = true
				}
			}
			if call == 2 && !found {
				t.Error("the real read tool result did not reach the model")
			}
			send(map[string]any{"role": "assistant", "reasoning_content": "已核对文档步骤。", "content": "建议如下：\n"}, nil)
			time.Sleep(100 * time.Millisecond)
			send(map[string]any{"content": "```markdown\n# 步骤\n- 等待 5 秒\n```"}, nil)
			send(map[string]any{}, "stop")
		}
		fmt.Fprint(w, "data: [DONE]\n\n")
	}))
	defer model.Close()
	database, err := db.Open(filepath.Join(t.TempDir(), "test.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer database.Close()
	service := NewService(database, paths)
	if err := service.SaveConfig(ModelConfig{Provider: "custom", BaseURL: model.URL + "/v1", APIKey: "integration-test-key", Model: "mock-doc"}); err != nil {
		t.Fatal(err)
	}
	mux := http.NewServeMux()
	NewHandler(service).RegisterRoutes(mux)
	server := httptest.NewServer(mux)
	defer server.Close()
	request := ChatRequest{Messages: []ChatMessage{{Role: "system", Content: "文档助手"}, {Role: "user", Content: "读取 current.md，然后调整等待时长"}}, Document: "# 步骤\n- 等待 2 秒"}
	data, _ := json.Marshal(request)
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()
	req, _ := http.NewRequestWithContext(ctx, "POST", server.URL+"/api/ai/chat", strings.NewReader(string(data)))
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	if !strings.HasPrefix(resp.Header.Get("Content-Type"), "text/event-stream") {
		t.Fatal("missing SSE response")
	}
	scanner := bufio.NewScanner(resp.Body)
	var events []ChatEvent
	for scanner.Scan() {
		line := scanner.Text()
		if !strings.HasPrefix(line, "data: ") {
			continue
		}
		var event ChatEvent
		if err := json.Unmarshal([]byte(strings.TrimPrefix(line, "data: ")), &event); err != nil {
			t.Fatal(err)
		}
		events = append(events, event)
		if event.Type == "error" {
			t.Fatalf("Pi error: %s", event.Error)
		}
		if event.Type == "thinking_delta" && calls.Load() != 1 {
			t.Fatal("first thinking was buffered until a later model turn")
		}
		// Only assert timing on the first thinking block.
		if event.Type == "thinking_delta" {
			break
		}
	}
	for scanner.Scan() {
		line := scanner.Text()
		if !strings.HasPrefix(line, "data: ") {
			continue
		}
		var event ChatEvent
		if err := json.Unmarshal([]byte(strings.TrimPrefix(line, "data: ")), &event); err != nil {
			t.Fatal(err)
		}
		events = append(events, event)
		if event.Type == "error" {
			t.Fatalf("Pi error: %s", event.Error)
		}
	}
	if err := scanner.Err(); err != nil {
		t.Fatal(err)
	}
	foundTool, foundOutput, foundDone := false, false, false
	toolResults := make(map[string]string)
	for _, event := range events {
		if event.Type == "tool_end" {
			if event.IsError {
				t.Fatalf("bundled tool failed: %+v", event)
			}
			toolResults[event.ToolName] = event.Output
		}
		if event.Type == "tool_start" && event.ToolName == "read" {
			foundTool = true
		}
		if event.Type == "tool_end" && strings.Contains(event.Output, "等待 2 秒") {
			foundOutput = true
		}
		if event.Type == "done" && strings.Contains(event.Reply, "等待 5 秒") {
			foundDone = true
		}
	}
	if !strings.Contains(toolResults["grep"], "等待 2 秒") || !strings.Contains(toolResults["find"], "current.md") {
		t.Fatalf("bundled search tools failed: %+v", toolResults)
	}
	if !foundTool || !foundOutput || !foundDone || calls.Load() != 2 {
		t.Fatalf("tool=%v output=%v done=%v calls=%d events=%+v", foundTool, foundOutput, foundDone, calls.Load(), events)
	}
	// An emitter failure must stop the persistent RPC process promptly.
	ctx2, cancel2 := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel2()
	expected := errors.New("client disconnected")
	start := time.Now()
	err = runPi(ctx2, paths, ModelConfig{Provider: "custom", BaseURL: model.URL + "/v1", APIKey: "integration-test-key", Model: "mock-doc"}, request, func(ChatEvent) error { return expected })
	if !errors.Is(err, expected) || time.Since(start) > 5*time.Second {
		t.Fatalf("cleanup err=%v elapsed=%s", err, time.Since(start))
	}
}
