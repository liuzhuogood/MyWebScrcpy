package operationlog

import (
	"bytes"
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"mywebscrcpy/internal/db"
)

func testService(t *testing.T) (*Service, *http.ServeMux, string) {
	t.Helper()
	path := filepath.Join(t.TempDir(), "logs.db")
	database, err := db.Open(path)
	if err != nil {
		t.Fatal(err)
	}
	s := New(database)
	t.Cleanup(func() { s.Close(); database.Close() })
	mux := http.NewServeMux()
	s.RegisterRoutes(mux)
	return s, mux, path
}

func logBody(id string) []byte {
	data, _ := json.Marshal(map[string]any{"items": []db.OperationLog{{ID: id, Serial: "device", Type: "tap", Description: "点击 (1, 2)", ADBCommand: "adb -s device shell input tap 1 2", OccurredAt: time.Now().UnixMilli()}}})
	return data
}

func call(mux *http.ServeMux, method, url string, body []byte) *httptest.ResponseRecorder {
	w := httptest.NewRecorder()
	mux.ServeHTTP(w, httptest.NewRequest(method, url, bytes.NewReader(body)))
	return w
}

func TestOperationLogHTTP(t *testing.T) {
	_, mux, _ := testService(t)
	for _, id := range []string{"one", "one", "two"} {
		if w := call(mux, "POST", "/api/operation-logs", logBody(id)); w.Code != 201 {
			t.Fatalf("append: %d %s", w.Code, w.Body)
		}
	}
	w := call(mux, "GET", "/api/operation-logs?limit=1", nil)
	var page struct {
		Items []db.OperationLog `json:"items"`
		Next  int64             `json:"next_before"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &page); err != nil || w.Code != 200 || len(page.Items) != 1 || page.Items[0].ID != "two" || page.Next == 0 {
		t.Fatalf("page=%s err=%v", w.Body, err)
	}
	for _, url := range []string{"?limit=0", "?limit=501", "?before=-1", "?before=no"} {
		if w := call(mux, "GET", "/api/operation-logs"+url, nil); w.Code != 400 {
			t.Fatalf("invalid query: %s %d", url, w.Code)
		}
	}
	for _, body := range []string{`{"items":[]}`, `{"items":[],"unknown":1}`, string(logBody("three")) + `{}`, strings.Replace(string(logBody("three")), `"type":"tap"`, `"type":"bogus"`, 1)} {
		if w := call(mux, "POST", "/api/operation-logs", []byte(body)); w.Code != 400 {
			t.Fatalf("invalid body: %d %s", w.Code, w.Body)
		}
	}
	if w := call(mux, "DELETE", "/api/operation-logs?serial=other", nil); w.Code != 200 || !strings.Contains(w.Body.String(), `"deleted":0`) {
		t.Fatalf("filter clear: %s", w.Body)
	}
	if w := call(mux, "DELETE", "/api/operation-logs", nil); w.Code != 200 || !strings.Contains(w.Body.String(), `"deleted":2`) {
		t.Fatalf("clear: %s", w.Body)
	}
	if w := call(mux, "POST", "/api/operation-logs", logBody("after")); w.Code != 201 {
		t.Fatal(w.Body)
	}
	w = call(mux, "GET", "/api/operation-logs", nil)
	if err := json.Unmarshal(w.Body.Bytes(), &page); err != nil || len(page.Items) != 1 || page.Items[0].ID != "after" {
		t.Fatalf("clear order: %s", w.Body)
	}
}

func TestBlockedLogWriteDoesNotBlockControl(t *testing.T) {
	s, mux, path := testService(t)
	// Synchronize startup pruning before taking an external SQLite writer lock.
	if _, err := s.submit(context.Background(), job{clear: true}); err != nil {
		t.Fatal(err)
	}
	locker, err := sql.Open("sqlite", path)
	if err != nil {
		t.Fatal(err)
	}
	defer locker.Close()
	conn, err := locker.Conn(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	defer conn.Close()
	if _, err = conn.ExecContext(context.Background(), "BEGIN IMMEDIATE"); err != nil {
		t.Fatal(err)
	}
	defer conn.ExecContext(context.Background(), "ROLLBACK")
	writeDone := make(chan *httptest.ResponseRecorder, 1)
	go func() { writeDone <- call(mux, "POST", "/api/operation-logs", logBody("blocked")) }()
	select {
	case <-writeDone:
		t.Fatal("acknowledged before SQLite commit")
	case <-time.After(100 * time.Millisecond):
	}
	mux.HandleFunc("POST /control", func(w http.ResponseWriter, r *http.Request) { w.WriteHeader(204) })
	controlDone := make(chan int, 1)
	go func() { controlDone <- call(mux, "POST", "/control", nil).Code }()
	select {
	case code := <-controlDone:
		if code != 204 {
			t.Fatal(code)
		}
	case <-time.After(time.Second):
		t.Fatal("control blocked by operation log")
	}
	// With the worker blocked, a full bounded queue rejects immediately.
	for len(s.queue) < cap(s.queue) {
		s.queue <- job{reply: make(chan result, 1)}
	}
	if _, err := s.submit(context.Background(), job{}); !errors.Is(err, ErrUnavailable) {
		t.Fatalf("full queue: %v", err)
	}
	if _, err = conn.ExecContext(context.Background(), "ROLLBACK"); err != nil {
		t.Fatal(err)
	}
	select {
	case w := <-writeDone:
		if w.Code != 201 {
			t.Fatalf("write: %d %s", w.Code, w.Body)
		}
	case <-time.After(3 * time.Second):
		t.Fatal("write did not recover")
	}
}

func TestUnavailableAndClosedService(t *testing.T) {
	s := New(nil)
	mux := http.NewServeMux()
	s.RegisterRoutes(mux)
	for _, method := range []string{"GET", "POST", "DELETE"} {
		if w := call(mux, method, "/api/operation-logs", logBody("one")); w.Code != 503 {
			t.Fatalf("%s: %d", method, w.Code)
		}
	}
	s.Close()
	s.Close()
	s, _, _ = testService(t)
	s.Close()
	if _, err := s.submit(context.Background(), job{}); !errors.Is(err, ErrUnavailable) {
		t.Fatal(err)
	}
}

func TestDatabaseFailureIsNotAcknowledged(t *testing.T) {
	s, mux, _ := testService(t)
	if err := s.database.Close(); err != nil {
		t.Fatal(err)
	}
	for _, method := range []string{"GET", "POST", "DELETE"} {
		if w := call(mux, method, "/api/operation-logs", logBody("failed")); w.Code != 503 {
			t.Fatalf("database failure acknowledged for %s: %d", method, w.Code)
		}
	}
}
