package adbcommand

import (
	"context"
	"fmt"
	"os"
	"strings"
	"testing"
	"time"

	"mywebscrcpy/internal/devicegate"
)

func TestHelperADBProcess(t *testing.T) {
	if os.Getenv("GO_WANT_HELPER_ADB") != "1" {
		return
	}
	args := strings.Join(os.Args, " ")
	if strings.Contains(args, " sleep") {
		time.Sleep(80 * time.Millisecond)
	}
	if strings.Contains(args, " fail") {
		fmt.Fprint(os.Stderr, "adb: command failed error")
		os.Exit(1)
	}
	if strings.Contains(args, " output") {
		fmt.Print(strings.Repeat("x", 128))
	} else {
		fmt.Print("ok")
	}
	os.Exit(0)
}

func testManager(t *testing.T) *Manager {
	t.Helper()
	t.Setenv("GO_WANT_HELPER_ADB", "1")
	gate := devicegate.New(2)
	return &Manager{adbPath: os.Args[0], prefix: []string{"-test.run=TestHelperADBProcess", "--"}, maxTimeout: 2 * time.Minute, queueWait: 5 * time.Second, outputMax: 32, gateFor: func(string) *devicegate.Gate { return gate }, jobs: make(map[string]*job)}
}

func TestValidateArgsRejectsTargetOverrides(t *testing.T) {
	for _, args := range [][]string{{}, {"-s", "other"}, {"shell", "-Hhost"}, {"adb", "devices"}, {"shell", "echo", "ok"}} {
		err := validateArgs(args)
		if strings.Join(args, " ") == "shell echo ok" && err != nil {
			t.Fatalf("valid args rejected: %v", err)
		}
		if strings.Join(args, " ") != "shell echo ok" && err == nil {
			t.Fatalf("invalid args accepted: %v", args)
		}
	}
}

func TestAsyncCommandsQueueAndTruncate(t *testing.T) {
	m := testManager(t)
	first, err := m.Submit("phone", Request{Args: []string{"sleep"}, Async: true})
	if err != nil {
		t.Fatal(err)
	}
	second, err := m.Submit("phone", Request{Args: []string{"output"}, Async: true})
	if err != nil {
		t.Fatal(err)
	}
	<-first.done
	<-second.done
	if first.Snapshot().Status != Completed || second.Snapshot().Status != Completed {
		t.Fatalf("unexpected statuses: %+v %+v", first.Snapshot(), second.Snapshot())
	}
	if !second.Snapshot().Truncated {
		t.Fatalf("output was not truncated: %+v", second.Snapshot())
	}
}

func TestCommandTimeout(t *testing.T) {
	m := testManager(t)
	timeout := int64(10)
	job, err := m.Submit("phone", Request{Args: []string{"sleep"}, TimeoutMS: &timeout})
	if err != nil {
		t.Fatal(err)
	}
	if job.Snapshot().Status != TimedOut {
		t.Fatalf("got %+v", job.Snapshot())
	}
}

func TestExecuteDirect(t *testing.T) {
	m := testManager(t)
	ctx := context.Background()

	// Missing serial
	if err := m.ExecuteDirect(ctx, "", "shell", "input"); err == nil || err.Error() != "missing serial" {
		t.Fatalf("expected 'missing serial', got %v", err)
	}

	// Missing args
	if err := m.ExecuteDirect(ctx, "phone"); err == nil || err.Error() != "missing args" {
		t.Fatalf("expected 'missing args', got %v", err)
	}

	// Success execution
	if err := m.ExecuteDirect(ctx, "phone", "shell", "input", "tap", "100", "200"); err != nil {
		t.Fatalf("expected success, got %v", err)
	}

	// Failed execution returns wrapped error with trimmed output
	err := m.ExecuteDirect(ctx, "phone", "fail")
	if err == nil {
		t.Fatal("expected error for fail command, got nil")
	}
	if !strings.Contains(err.Error(), "adb: command failed error") {
		t.Fatalf("expected error to contain output, got: %v", err)
	}

	// Canceled context
	cancelCtx, cancel := context.WithCancel(context.Background())
	cancel()
	if err := m.ExecuteDirect(cancelCtx, "phone", "sleep"); err == nil {
		t.Fatal("expected error for canceled context, got nil")
	}
}
