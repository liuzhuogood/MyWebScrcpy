package taskmanager

import (
	"context"
	"encoding/json"
	"fmt"
	"mywebscrcpy/internal/db"
	"mywebscrcpy/internal/taskartifact"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync/atomic"
	"testing"
	"time"
)

func artifactManager(t *testing.T) (*db.DB, *Manager) {
	t.Helper()
	d, e := db.Open(filepath.Join(t.TempDir(), "artifacts.db"))
	if e != nil {
		t.Fatal(e)
	}
	m := NewManager(d)
	t.Cleanup(func() { m.Close(); d.Close() })
	return d, m
}
func artifactDone(t *testing.T, m *Manager, id string) taskartifact.State {
	t.Helper()
	end := time.Now().Add(4 * time.Second)
	for time.Now().Before(end) {
		st, e := m.Artifacts.State(id)
		if e == nil && (st.Artifacts.Log.CaptureState == "completed" || st.Artifacts.Log.CaptureState == "degraded") {
			return st
		}
		time.Sleep(10 * time.Millisecond)
	}
	t.Fatal("task did not finalize")
	return taskartifact.State{}
}
func TestArtifactsRealPythonAndReportRetry(t *testing.T) {
	d, m := artifactManager(t)
	var calls atomic.Int32
	m.Artifacts.ConfigureReports(func(ctx context.Context, prompt, evidence string) (taskartifact.ReportOutput, error) {
		if calls.Add(1) > 1 {
			return taskartifact.ReportOutput{}, fmt.Errorf("model offline")
		}
		var ev taskartifact.Evidence
		if e := json.Unmarshal([]byte(evidence), &ev); e != nil {
			return taskartifact.ReportOutput{}, e
		}
		if ev.TaskID == "" || ev.DeviceSerial != "device-a" || ev.Status != "completed" {
			t.Errorf("wrong evidence %s", evidence)
		}
		return taskartifact.ReportOutput{Markdown: "完成", Citations: []taskartifact.Citation{{Seq: ev.Lines[0].Seq}}}, nil
	})
	task, e := m.StartDispatchedTask(1, "日志", "device-a", 5, "import sys\nprint('中文输出')\nprint('警告',file=sys.stderr)\nprint('未换行',end='')", "", taskartifact.Options{Report: taskartifact.ReportOptions{Enabled: true}})
	if e != nil {
		t.Fatal(e)
	}
	st := artifactDone(t, m, task.ID)
	if st.Artifacts.Recording.State != "disabled" {
		t.Fatal(st)
	}
	page, e := m.Artifacts.Logs.Read(task.ID, taskartifact.LogQuery{Limit: 500})
	if e != nil {
		t.Fatal(e)
	}
	var out, errout strings.Builder
	for _, line := range page.Items {
		if line.Stream == "stdout" {
			out.WriteString(line.Message)
		}
		if line.Stream == "stderr" {
			errout.WriteString(line.Message)
		}
	}
	if !strings.Contains(out.String(), "中文输出") || !strings.Contains(out.String(), "未换行") || !strings.Contains(errout.String(), "警告") {
		t.Fatalf("missing output %+v", page.Items)
	}
	deadline := time.Now().Add(3 * time.Second)
	for time.Now().Before(deadline) {
		st, _ = m.Artifacts.State(task.ID)
		if st.Artifacts.Report.State == "completed" {
			break
		}
		time.Sleep(10 * time.Millisecond)
	}
	if st.Artifacts.Report.AvailableRevision != 1 {
		t.Fatalf("%+v", st)
	}
	if e = m.Artifacts.Enqueue(task.ID, true); e != nil {
		t.Fatal(e)
	}
	deadline = time.Now().Add(3 * time.Second)
	for time.Now().Before(deadline) {
		st, _ = m.Artifacts.State(task.ID)
		if st.Artifacts.Report.State == "failed" {
			break
		}
		time.Sleep(10 * time.Millisecond)
	}
	if st.Artifacts.Report.AvailableRevision != 1 || st.Artifacts.Report.GenerationRevision != 2 {
		t.Fatalf("lost success %+v", st)
	}
	mux := http.NewServeMux()
	NewHandler(m).RegisterRoutes(mux)
	w := httptest.NewRecorder()
	mux.ServeHTTP(w, httptest.NewRequest("GET", "/api/tasks/"+task.ID+"/report", nil))
	if w.Code != 200 || !strings.Contains(w.Body.String(), "完成") {
		t.Fatal(w.Body.String())
	}
	other := NewManager(d)
	defer other.Close()
	restored, e := other.Artifacts.State(task.ID)
	if e != nil || restored.Artifacts.Report.AvailableRevision != 1 || restored.Artifacts.Log.LastSeq != st.Artifacts.Log.LastSeq {
		t.Fatalf("restart lost artifacts %+v %v", restored, e)
	}
}
func TestTimeoutAndKillLeaveTerminalLogs(t *testing.T) {
	_, m := artifactManager(t)
	for _, manual := range []bool{false, true} {
		task, e := m.StartTask(1, "timeout", "device-a", 1, "import time\nprint('开始')\ntime.sleep(20)")
		if e != nil {
			t.Fatal(e)
		}
		if manual {
			m.KillTask(task.ID)
		}
		artifactDone(t, m, task.ID)
		page, e := m.Artifacts.Logs.Read(task.ID, taskartifact.LogQuery{Limit: 500})
		if e != nil || !page.Complete || len(page.Items) == 0 {
			t.Fatalf("%+v %v", page, e)
		}
		last := page.Items[len(page.Items)-1]
		if !strings.Contains(last.Message, "任务结束") {
			t.Fatalf("lost final event %+v", last)
		}
	}
}
func TestReportQueueAtomicAndInvalidCitations(t *testing.T) {
	_, m := artifactManager(t)
	entered, release := make(chan struct{}), make(chan struct{})
	m.Artifacts.ConfigureReports(func(context.Context, string, string) (taskartifact.ReportOutput, error) {
		close(entered)
		<-release
		return taskartifact.ReportOutput{Markdown: "伪造", Citations: []taskartifact.Citation{{Seq: 999999}}}, nil
	})
	task, e := m.StartDispatchedTask(1, "test", "device-a", 3, "print('ok')", "", taskartifact.Options{Report: taskartifact.ReportOptions{Enabled: true}})
	if e != nil {
		t.Fatal(e)
	}
	artifactDone(t, m, task.ID)
	<-entered
	for i := 0; i < 10; i++ {
		if e = m.Artifacts.Enqueue(task.ID, true); e == nil {
			t.Fatal("duplicate generation accepted")
		}
	}
	close(release)
	end := time.Now().Add(3 * time.Second)
	for time.Now().Before(end) {
		st, _ := m.Artifacts.State(task.ID)
		if st.Artifacts.Report.State == "failed" {
			if st.Artifacts.Report.AvailableRevision != 0 {
				t.Fatal("invalid citation usable")
			}
			return
		}
		time.Sleep(10 * time.Millisecond)
	}
	t.Fatal("invalid citation not rejected")
}

func TestInitialLogFailurePreventsPythonStart(t *testing.T) {
	d, e := db.Open(filepath.Join(t.TempDir(), "blocked.db"))
	if e != nil {
		t.Fatal(e)
	}
	defer d.Close()
	if e = os.MkdirAll(filepath.Dir(d.ArtifactDir()), 0700); e != nil {
		t.Fatal(e)
	}
	if e = os.WriteFile(d.ArtifactDir(), []byte("blocked"), 0600); e != nil {
		t.Fatal(e)
	}
	m := NewManager(d)
	defer m.Close()
	marker := filepath.Join(t.TempDir(), "python-started")
	task, e := m.StartTask(1, "blocked", "device-a", 3, fmt.Sprintf("open(%q, 'w').write('started')", marker))
	if e == nil || task == nil || task.PID != 0 || task.Status != "failed" {
		t.Fatalf("task=%+v error=%v", task, e)
	}
	if _, e = os.Stat(marker); !os.IsNotExist(e) {
		t.Fatalf("Python ran: %v", e)
	}
	state, e := m.Artifacts.State(task.ID)
	if e != nil || state.Artifacts.Log.Availability != "unavailable" {
		t.Fatalf("%+v %v", state, e)
	}
}
