package ocr

import (
	"context"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
	"time"
)

type preparingRuntime struct {
	entered chan struct{}
	failed  bool
}

func (r *preparingRuntime) Acquire(string) (string, func(), error) { return "unused", func() {}, nil }
func (r *preparingRuntime) Check(context.Context, string, string) error {
	return errors.New("not installed")
}
func (r *preparingRuntime) PaddleScript(string) (string, error) {
	return "", errors.New("must not launch after failed preparation")
}
func (r *preparingRuntime) Prepare(ctx context.Context, _ string, stage func(string)) (string, func(), error) {
	stage("installing")
	close(r.entered)
	if !r.failed {
		<-ctx.Done()
		return "", nil, ctx.Err()
	}
	return "", nil, errors.New("controlled pip failure")
}

func TestManagedPreparationReportsInstallingAndCanBeStopped(t *testing.T) {
	runtime := &preparingRuntime{entered: make(chan struct{})}
	s := NewManagedService(nil, nil, runtime, nil)
	done := make(chan error, 1)
	go func() { done <- s.StartPaddle(context.Background()) }()
	<-runtime.entered
	status := s.Status()
	if status.PaddleStatus != "installing" || status.PaddleRunning || status.PaddleError != "" {
		t.Fatalf("unexpected preparing status: %+v", status)
	}
	if err := s.StopPaddle(); err != nil {
		t.Fatal(err)
	}
	select {
	case err := <-done:
		if !errors.Is(err, context.Canceled) {
			t.Fatal(err)
		}
	case <-time.After(time.Second):
		t.Fatal("preparation not cancelled")
	}
	status = s.Status()
	if status.PaddleRunning || status.PaddleStatus == "installing" || status.PaddleStatus == "failed" {
		t.Fatalf("stop did not settle: %+v", status)
	}
}

func TestManagedInstallFailureNeverStartsPaddle(t *testing.T) {
	s := NewManagedService(nil, nil, &preparingRuntime{entered: make(chan struct{}), failed: true}, nil)
	if err := s.StartPaddle(context.Background()); err == nil {
		t.Fatal("failed installation accepted")
	}
	status := s.Status()
	if status.PaddleStatus != "failed" || status.PaddleRunning || status.PaddlePID != 0 {
		t.Fatalf("bad failure state: %+v", status)
	}
}

func TestStopWaitsForRuntimeLeaseRelease(t *testing.T) {
	python, err := exec.LookPath("python3")
	if err != nil {
		t.Skip("Python fixture requires python3")
	}
	script := filepath.Join(t.TempDir(), "server.py")
	code := `import http.server, sys
port = int(sys.argv[sys.argv.index('--port')+1])
class Handler(http.server.BaseHTTPRequestHandler):
    def do_GET(self):
        self.send_response(200)
        self.end_headers()
        self.wfile.write(b'{"ready":true}')
    def log_message(self, *args): pass
http.server.HTTPServer(('127.0.0.1',port), Handler).serve_forever()
`
	if err := os.WriteFile(script, []byte(code), 0600); err != nil {
		t.Fatal(err)
	}
	s := NewService(nil, nil)
	s.SetPaddlePort(getFreePort(t))
	releasing, allowRelease := make(chan struct{}), make(chan struct{})
	if err := s.startPaddleProcess(context.Background(), python, script, func() { close(releasing); <-allowRelease }); err != nil {
		t.Fatal(err)
	}
	stopped := make(chan error, 1)
	go func() { stopped <- s.StopPaddle() }()
	select {
	case <-releasing:
	case <-time.After(time.Second):
		t.Fatal("runtime lease not released")
	}
	select {
	case err := <-stopped:
		t.Fatalf("Stop returned before lease release: %v", err)
	default:
	}
	close(allowRelease)
	select {
	case err := <-stopped:
		if err != nil {
			t.Fatal(err)
		}
	case <-time.After(time.Second):
		t.Fatal("stop did not complete")
	}
}

func (p *preparingRuntime) IndraScript(python string) (string, error) { return p.PaddleScript(python) }
