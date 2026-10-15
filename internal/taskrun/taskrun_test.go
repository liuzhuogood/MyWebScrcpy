package taskrun

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"mywebscrcpy/internal/db"
)

func newTestService(t *testing.T) (*Service, *http.ServeMux) {
	t.Helper()
	dbPath := filepath.Join(t.TempDir(), "test.db")
	database, err := db.Open(dbPath)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { database.Close() })

	svc := NewService(database)

	mux := http.NewServeMux()
	handler := NewHandler(svc)
	handler.RegisterRoutes(mux)
	return svc, mux
}

func TestTaskRunCreateWithoutToken(t *testing.T) {
	_, mux := newTestService(t)
	req := httptest.NewRequest("POST", "/api/task-runs", strings.NewReader(`{"client_run_id":"no-token","name":"direct","device_serial":"device"}`))
	w := httptest.NewRecorder()
	mux.ServeHTTP(w, req)
	if w.Code != http.StatusCreated {
		t.Fatalf("direct create: %d %s", w.Code, w.Body.String())
	}
}

func TestTaskRunLifecycleAndHeartbeat(t *testing.T) {
	svc, mux := newTestService(t)

	// 1. 创建任务
	body := `{"client_run_id":"client-123","name":"自动化测试","device_serial":"dev-phone","source":"python-sdk"}`
	req := httptest.NewRequest("POST", "/api/task-runs", strings.NewReader(body))
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)
	if rec.Code != http.StatusCreated {
		t.Fatalf("create failed: %d body=%s", rec.Code, rec.Body.String())
	}

	var created TaskRunView
	if err := json.Unmarshal(rec.Body.Bytes(), &created); err != nil {
		t.Fatal(err)
	}
	if created.ID == "" || created.Status != "running" || created.Connectivity != "connected" {
		t.Fatalf("unexpected created view: %+v", created)
	}

	runID := created.ID

	// 2. 心跳保活
	req = httptest.NewRequest("POST", fmt.Sprintf("/api/task-runs/%s/heartbeat", runID), nil)
	rec = httptest.NewRecorder()
	mux.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("heartbeat failed: %d", rec.Code)
	}

	// 3. 验证失联派生状态 (mock 超过 60s 心跳超时)
	now := time.Now().UnixMilli()
	oldTime := now - 65000
	_, err := svc.database.HeartbeatTaskRun(runID, oldTime)
	if err != nil {
		t.Fatal(err)
	}
	dbRec, err := svc.database.GetTaskRunByID(runID)
	if err != nil {
		t.Fatal(err)
	}
	view := svc.DeriveView(dbRec, now)
	if !view.Disconnected || view.Connectivity != "disconnected" || view.ConnectivityMessage != "失联，结果未知" {
		t.Fatalf("expected disconnected view, got: %+v", view)
	}

	// 4. 恢复心跳后变回 connected
	req = httptest.NewRequest("POST", fmt.Sprintf("/api/task-runs/%s/heartbeat", runID), nil)
	rec = httptest.NewRecorder()
	mux.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("heartbeat restore failed: %d", rec.Code)
	}
	var restored TaskRunView
	json.Unmarshal(rec.Body.Bytes(), &restored)
	if restored.Disconnected || restored.Connectivity != "connected" {
		t.Fatalf("expected reconnected view, got: %+v", restored)
	}

	// 5. 追加批量日志
	logsBody := `{"entries":[{"seq":1,"timestamp":1000,"stream":"stdout","message":"hello"},{"seq":2,"timestamp":1001,"stream":"stdout","message":"world"}]}`
	req = httptest.NewRequest("POST", fmt.Sprintf("/api/task-runs/%s/logs", runID), strings.NewReader(logsBody))
	rec = httptest.NewRecorder()
	mux.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("append logs failed: %d body=%s", rec.Code, rec.Body.String())
	}

	// 6. 结束任务
	finishBody := `{"status":"succeeded","exit_code":0,"summary":"任务完成","report_markdown":"# 成功"}`
	req = httptest.NewRequest("POST", fmt.Sprintf("/api/task-runs/%s/finish", runID), strings.NewReader(finishBody))
	rec = httptest.NewRecorder()
	mux.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("finish failed: %d body=%s", rec.Code, rec.Body.String())
	}
	var finished TaskRunView
	json.Unmarshal(rec.Body.Bytes(), &finished)
	if finished.Status != "succeeded" || finished.Connectivity != "closed" || finished.Summary != "任务完成" {
		t.Fatalf("unexpected finished view: %+v", finished)
	}

	// 7. 查询详情与日志下载
	req = httptest.NewRequest("GET", fmt.Sprintf("/api/task-runs/%s", runID), nil)
	rec = httptest.NewRecorder()
	mux.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("get detail failed: %d", rec.Code)
	}

	req = httptest.NewRequest("GET", fmt.Sprintf("/api/task-runs/%s/logs?download=true", runID), nil)
	rec = httptest.NewRecorder()
	mux.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("download logs failed: %d", rec.Code)
	}
	if !strings.Contains(rec.Body.String(), "hello") || !strings.Contains(rec.Body.String(), "world") {
		t.Fatalf("missing downloaded logs: %s", rec.Body.String())
	}
}

func TestTaskRunListFilter(t *testing.T) {
	_, mux := newTestService(t)

	for i := 1; i <= 3; i++ {
		b := fmt.Sprintf(`{"client_run_id":"c-%d","name":"任务-%d","device_serial":"phone-%d","source":"test"}`, i, i, i)
		req := httptest.NewRequest("POST", "/api/task-runs", strings.NewReader(b))
		rec := httptest.NewRecorder()
		mux.ServeHTTP(rec, req)
		if rec.Code != http.StatusCreated {
			t.Fatalf("create run %d failed: %d", i, rec.Code)
		}
	}

	// 分页与设备筛选查询
	req := httptest.NewRequest("GET", "/api/task-runs?device_serial=phone-2", nil)
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("list failed: %d", rec.Code)
	}

	var res struct {
		Items []TaskRunView `json:"items"`
		Total int           `json:"total"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &res); err != nil {
		t.Fatal(err)
	}
	if res.Total != 1 || len(res.Items) != 1 || res.Items[0].DeviceSerial != "phone-2" {
		t.Fatalf("unexpected filter result: %+v", res)
	}
}

func TestReviewRejectTrailingJSON(t *testing.T) {
	_, mux := newTestService(t)
	req := httptest.NewRequest("POST", "/api/task-runs", strings.NewReader(`{"client_run_id":"trailing","name":"test","source":"codex","device_serial":"test"}{"ignored":true}`))
	w := httptest.NewRecorder()
	mux.ServeHTTP(w, req)
	if w.Code != http.StatusBadRequest {
		t.Fatalf("trailing JSON accepted: status=%d body=%s", w.Code, w.Body.String())
	}
}

func TestReviewRejectUnknownLogStream(t *testing.T) {
	d, mux := newTestService(t)
	r, _, err := d.database.CreateTaskRun(&db.TaskRunRecord{ClientRunID: "rev-stream", Name: "review", DeviceSerial: "test-serial", Source: "codex"})
	if err != nil {
		t.Fatal(err)
	}
	req := httptest.NewRequest("POST", "/api/task-runs/"+r.ID+"/logs", strings.NewReader(`{"entries":[{"seq":1,"timestamp":1,"stream":"unexpected","message":"test"}]}`))
	w := httptest.NewRecorder()
	mux.ServeHTTP(w, req)
	if w.Code != http.StatusBadRequest {
		t.Fatalf("unknown log stream accepted: status=%d", w.Code)
	}
}

func TestLogRequestRejectsUndocumentedOrMissingFields(t *testing.T) {
	svc, mux := newTestService(t)
	run, _, err := svc.database.CreateTaskRun(&db.TaskRunRecord{ClientRunID: "strict-logs", Name: "strict", DeviceSerial: "fixture", Source: "codex"})
	if err != nil {
		t.Fatal(err)
	}
	for _, body := range []string{
		`{"entries":[{"seq":1,"timestamp":1,"stream":"stdout","message":"test","run_id":"other"}]}`,
		`{"entries":[{"seq":1,"stream":"stdout","message":"test"}]}`,
		`{"entries":[]}`,
	} {
		w := httptest.NewRecorder()
		mux.ServeHTTP(w, httptest.NewRequest("POST", "/api/task-runs/"+run.ID+"/logs", strings.NewReader(body)))
		if w.Code != 400 {
			t.Fatalf("invalid body accepted: %d %s", w.Code, body)
		}
	}
}
