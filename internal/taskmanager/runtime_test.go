package taskmanager

import (
	"encoding/json"
	"io"
	"net/http"
	"os"
	"strings"
	"sync/atomic"
	"testing"
)

func TestStandaloneHTTPTaskBridgeAndFailure(t *testing.T) {
	d, m := artifactManager(t)
	var calls atomic.Int32
	if err := m.ConfigureAPI(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Query().Get("serial") != "device-A" {
			t.Error("wrong target")
		}
		var body struct {
			Args      []string `json:"args"`
			TimeoutMS int64    `json:"timeout_ms"`
		}
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			t.Error(err)
		}
		if len(body.Args) != 3 || body.Args[2] != "ro.product.model" || body.TimeoutMS != 10000 {
			t.Errorf("%+v", body)
		}
		calls.Add(1)
		json.NewEncoder(w).Encode(map[string]any{"status": "failed", "exit_code": 1})
	})); err != nil {
		t.Fatal(err)
	}
	codeBytes, err := os.ReadFile("../../.pi/skills/markdown-to-python/assets/example.py")
	if err != nil {
		t.Fatal(err)
	}
	code := string(codeBytes)
	task, err := m.StartTask(0, "运行库", "device-A", 10, code)
	if err != nil {
		t.Fatal(err)
	}
	state := artifactDone(t, m, task.ID)
	record, _ := d.GetTaskByID(task.ID)
	if calls.Load() != 1 || record.Status != "failed" || record.ExitCode != 1 {
		t.Fatalf("%+v calls=%d state=%+v", record, calls.Load(), state)
	}
	response, err := http.Post(m.apiURL+"/api/adb/commands?serial=device-A", "application/json", strings.NewReader(`{"args":["shell"]}`))
	if err != nil {
		t.Fatal(err)
	}
	io.Copy(io.Discard, response.Body)
	response.Body.Close()
	if response.StatusCode != 401 {
		t.Fatal("bridge accepted unauthenticated caller")
	}

}
