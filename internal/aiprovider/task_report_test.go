package aiprovider

import (
	"context"
	"encoding/json"
	"mywebscrcpy/internal/db"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"sync/atomic"
	"testing"
)

func TestTaskSummaryUsesConfiguredHTTPWithoutTools(t *testing.T) {
	d, e := db.Open(filepath.Join(t.TempDir(), "model.db"))
	if e != nil {
		t.Fatal(e)
	}
	defer d.Close()
	svc := &Service{database: d, client: http.DefaultClient}
	if _, e = svc.SummarizeTask(context.Background(), "", "{}"); e == nil {
		t.Fatal("unconfigured model accepted")
	}
	var responseContent atomic.Value
	responseContent.Store(`{"markdown":"完成","citations":[{"seq":1}]}`)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/chat/completions" {
			t.Error(r.URL.Path)
		}
		var body map[string]json.RawMessage
		json.NewDecoder(r.Body).Decode(&body)
		if _, exists := body["tools"]; exists {
			t.Fatal("report received tools")
		}
		if string(body["model"]) != `"test-model"` {
			t.Fatal(string(body["model"]))
		}
		json.NewEncoder(w).Encode(map[string]any{"choices": []any{map[string]any{"message": map[string]string{"role": "assistant", "content": responseContent.Load().(string)}}}})
	}))
	defer server.Close()
	svc.SaveConfig(ModelConfig{Provider: "ollama", BaseURL: server.URL, Model: "test-model"})
	out, e := svc.SummarizeTask(context.Background(), "总结失败原因", `{"task_id":"single"}`)
	if e != nil || out.Markdown != "完成" || len(out.Citations) != 1 {
		t.Fatalf("%+v %v", out, e)
	}
	for _, invalid := range []string{`{"markdown":"完成"}`, `{"markdown":"完成","citations":null}`, `{"markdown":2,"citations":[]}`} {
		responseContent.Store(invalid)
		if _, e = svc.SummarizeTask(context.Background(), "", "{}"); e == nil {
			t.Fatalf("invalid report accepted: %s", invalid)
		}
	}

}
