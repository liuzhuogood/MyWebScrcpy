package matchingservice

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"sync/atomic"
	"testing"
	"time"
)

func fakePython(t *testing.T, ready bool, block bool) string {
	t.Helper()
	if runtime.GOOS == "windows" {
		t.Skip("shell fixture requires Unix; native Windows verification remains separate")
	}
	entries := []string{}
	for _, name := range []string{"mywebscrcpy-sdk", "httpx", "websocket", "av", "cv2", "numpy"} {
		entries = append(entries, fmt.Sprintf(`{"name":%q,"available":%t,"error":""}`, name, ready))
	}
	probe := fmt.Sprintf(`{"version":"3.14.3","ready":%t,"dependencies":[%s],"error":""}`, ready, strings.Join(entries, ","))
	probeCommand := "printf '%s\\n' '" + probe + "'"
	if block {
		probeCommand = "while :; do :; done"
	}
	script := "#!/bin/sh\nif [ \"$2\" = '-c' ]; then\n" + probeCommand + "\nexit 0\nfi\nprintf '%s\\n' '{\"type\":\"ready\"}'\nwhile read -r line; do :; done\n"
	path := filepath.Join(t.TempDir(), "python")
	if err := os.WriteFile(path, []byte(script), 0700); err != nil {
		t.Fatal(err)
	}
	return path
}
func testManager(path string) *Manager {
	return New(Options{Python: func() (string, func(), error) { return path, func() {}, nil }, Devices: func() []string { return []string{"phone"} }, API: http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { writeJSON(w, 200, map[string]string{"path": r.URL.Path}) }), Vision: http.NotFoundHandler(), TemplateReadable: func(id, serial string) bool { return id == "target" && serial == "phone" }})
}
func TestLifecycleSingleProcessAndRestart(t *testing.T) {
	m := testManager(fakePython(t, true, false))
	defer m.Close()
	if s := m.Status(); s.State != "stopped" || s.PID != 0 {
		t.Fatalf("auto-started: %+v", s)
	}
	if err := m.Start(context.Background()); err != nil {
		t.Fatal(err)
	}
	first := m.Status()
	if first.State != "running" || first.PID == 0 {
		t.Fatalf("not ready: %+v", first)
	}
	if err := m.Start(context.Background()); err == nil {
		t.Fatal("duplicate start accepted")
	}
	if m.Status().PID != first.PID {
		t.Fatal("created second process")
	}
	if err := m.CheckIdle(); err == nil {
		t.Fatal("active service ignored by maintenance")
	}
	if _, err := m.Check(context.Background()); err == nil {
		t.Fatal("running environment recheck accepted")
	}
	if err := m.Stop(); err != nil {
		t.Fatal(err)
	}
	if s := m.Status(); s.PID != 0 || s.State != "stopped" {
		t.Fatalf("did not stop: %+v", s)
	}
	if err := m.Start(context.Background()); err != nil {
		t.Fatal(err)
	}
	m.mu.Lock()
	job := m.process
	m.mu.Unlock()
	if err := job.cmd.Process.Kill(); err != nil {
		t.Fatal(err)
	}
	select {
	case <-job.done:
	case <-time.After(3 * time.Second):
		t.Fatal("crash not reaped")
	}
	if s := m.Status(); s.State != "failed" || s.PID != 0 {
		t.Fatalf("crash incorrectly running: %+v", s)
	}
}
func TestMissingDependenciesBlockStart(t *testing.T) {
	m := testManager(fakePython(t, false, false))
	defer m.Close()
	env, err := m.Check(context.Background())
	if err != nil || env.Ready || len(env.Dependencies) != 6 {
		t.Fatalf("bad detection: %+v %v", env, err)
	}
	if err := m.Start(context.Background()); err == nil {
		t.Fatal("missing dependencies accepted")
	}
	if m.Status().PID != 0 {
		t.Fatal("spawned worker with missing dependencies")
	}
}
func TestCloseCancelsInitialization(t *testing.T) {
	m := testManager(fakePython(t, true, true))
	done := make(chan error, 1)
	go func() { done <- m.Start(context.Background()) }()
	deadline := time.Now().Add(time.Second)
	for m.Status().State != "starting" {
		if time.Now().After(deadline) {
			t.Fatal("did not start checking")
		}
		time.Sleep(time.Millisecond)
	}
	m.Close()
	select {
	case err := <-done:
		if err == nil {
			t.Fatal("closed initialization succeeded")
		}
	case <-time.After(3 * time.Second):
		t.Fatal("initialization leaked")
	}
	if m.Status().PID != 0 {
		t.Fatal("spawned after close")
	}
	if err := m.Start(context.Background()); err == nil {
		t.Fatal("closed manager restarted")
	}
}
func TestReadOnlyBridge(t *testing.T) {
	m := testManager(fakePython(t, true, false))
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	server, url, err := m.bridge(ctx, "token")
	if err != nil {
		t.Fatal(err)
	}
	defer server.Close()
	cases := []struct {
		method, path, token string
		status              int
	}{
		{"GET", "/api/matching-runtime/devices", "", 401},
		{"GET", "/api/matching-runtime/devices", "token", 200},
		{"GET", "/api/templates?serial=phone", "token", 200},
		{"GET", "/api/templates?serial=other", "token", 403},
		{"GET", "/api/templates/target/image?serial=phone", "token", 200},
		{"GET", "/api/templates/private/image?serial=phone", "token", 403},
		{"POST", "/api/templates?serial=phone", "token", 404},
		{"GET", "/api/adb?serial=phone", "token", 404},
		{"GET", "/api/templates/export?serial=phone", "token", 404},
	}
	for _, c := range cases {
		r, _ := http.NewRequest(c.method, url+c.path, nil)
		if c.token != "" {
			r.Header.Set("Authorization", "Bearer "+c.token)
		}
		response, err := http.DefaultClient.Do(r)
		if err != nil {
			t.Fatal(err)
		}
		response.Body.Close()
		if response.StatusCode != c.status {
			t.Errorf("%s %s=%d want %d", c.method, c.path, response.StatusCode, c.status)
		}
	}
}

func TestStopCancelsInitializationAndAllowsRetry(t *testing.T) {
	m := testManager(fakePython(t, true, true))
	defer m.Close()
	done := make(chan error, 1)
	go func() { done <- m.Start(context.Background()) }()
	deadline := time.Now().Add(time.Second)
	for m.Status().State != "starting" {
		if time.Now().After(deadline) {
			t.Fatal("initialization not started")
		}
		time.Sleep(time.Millisecond)
	}
	if err := m.Stop(); err != nil {
		t.Fatal(err)
	}
	if err := <-done; err == nil {
		t.Fatal("cancelled start succeeded")
	}
	if s := m.Status(); s.State != "stopped" || s.PID != 0 {
		t.Fatalf("not stopped: %+v", s)
	}
	m.options.Python = func() (string, func(), error) { return fakePython(t, true, false), func() {}, nil }
	if err := m.Start(context.Background()); err != nil {
		t.Fatal(err)
	}
}

func TestPythonLeaseSpansProcessLifetime(t *testing.T) {
	path := fakePython(t, true, false)
	m := testManager(path)
	defer m.Close()
	var leases atomic.Int32
	m.options.Python = func() (string, func(), error) {
		leases.Add(1)
		return path, func() { leases.Add(-1) }, nil
	}
	if _, err := m.Check(context.Background()); err != nil || leases.Load() != 0 {
		t.Fatalf("check leaked lease: %v %d", err, leases.Load())
	}
	if err := m.Start(context.Background()); err != nil {
		t.Fatal(err)
	}
	if leases.Load() != 1 {
		t.Fatal("process did not protect runtime")
	}
	if err := m.Stop(); err != nil {
		t.Fatal(err)
	}
	if leases.Load() != 0 {
		t.Fatal("stop leaked lease")
	}
	m.options.Python = func() (string, func(), error) { return "", nil, errors.New("runtime busy") }
	env, err := m.Check(context.Background())
	if err != nil || env.Ready || env.Error != "runtime busy" || m.Status().State != "stopped" {
		t.Fatalf("failed acquisition left checking locked: %+v %v", env, err)
	}
	if err := m.Start(context.Background()); err == nil || leases.Load() != 0 {
		t.Fatal("runtime acquisition failure accepted")
	}
}
