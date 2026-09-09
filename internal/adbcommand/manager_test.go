package adbcommand

import (
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
	return &Manager{adbPath: os.Args[0], prefix: []string{"-test.run=TestHelperADBProcess", "--"}, maxTimeout: 2 * time.Minute, queueWait: time.Second, outputMax: 32, gateFor: func(string) *devicegate.Gate { return gate }, jobs: make(map[string]*job)}
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
