package action

import (
	"context"
	"encoding/json"
	"math"
	"testing"
	"time"
)

type fakeExecutor struct{}

func (fakeExecutor) Execute(context.Context, Request) error { return nil }

func TestQueueSerializesAcceptedAction(t *testing.T) {
	q := New(fakeExecutor{}, 1)
	r := <-q.Submit(context.Background(), Request{RequestID: "r1", DeviceID: "phone", Action: "tap", X: .5, Y: .5})
	if !r.Accepted || !r.Executed || r.RequestID != "r1" {
		t.Fatalf("unexpected result: %+v", r)
	}
}

func TestQueueRejectsInvalidAction(t *testing.T) {
	q := New(fakeExecutor{}, 1)
	r := <-q.Submit(context.Background(), Request{RequestID: "r2", DeviceID: "phone", Action: "shell"})
	if !r.Accepted || r.ErrorCode != "unsupported_action" {
		t.Fatalf("unexpected result: %+v", r)
	}
}

func TestQueueValidatesStructuredActionFields(t *testing.T) {
	for _, req := range []Request{
		{RequestID: "swipe", DeviceID: "phone", Action: "swipe", X: .1, Y: .1, X2: 1.1, Y2: .2},
		{RequestID: "key", DeviceID: "phone", Action: "key"},
		{RequestID: "text", DeviceID: "phone", Action: "text"},
	} {
		result := <-New(fakeExecutor{}, 1).Submit(context.Background(), req)
		if !result.Accepted || result.Executed || result.ErrorCode == "" {
			t.Fatalf("request %+v should be rejected by validation: %+v", req, result)
		}
	}
}

func TestQueueRejectsNonFiniteCoordinates(t *testing.T) {
	for _, point := range [][2]float64{{math.NaN(), .2}, {math.Inf(1), .2}} {
		result := <-New(fakeExecutor{}, 1).Submit(context.Background(), Request{RequestID: "non-finite", DeviceID: "phone", Action: "tap", X: point[0], Y: point[1]})
		if result.ErrorCode != "invalid_coordinate" {
			t.Fatalf("non-finite coordinate should be rejected: %+v", result)
		}
	}
}

func TestQueueCloseRejectsNewActions(t *testing.T) {
	q := New(fakeExecutor{}, 1)
	q.Close()
	q.Close()
	result := <-q.Submit(context.Background(), Request{RequestID: "closed", DeviceID: "phone", Action: "tap", X: .5, Y: .5})
	if result.Accepted || result.ErrorCode != "queue_closed" {
		t.Fatalf("unexpected closed queue result: %+v", result)
	}
}

func TestQueueCloseCompletesPendingActions(t *testing.T) {
	exec := &gatedExecutor{started: make(chan struct{})}
	q := New(exec, 2)
	first := q.Submit(context.Background(), Request{RequestID: "running", DeviceID: "phone", Action: "tap", X: .5, Y: .5, ExpiresMS: 10})
	<-exec.started
	second := q.Submit(context.Background(), Request{RequestID: "pending", DeviceID: "phone", Action: "tap", X: .5, Y: .5})
	q.Close()
	if result := <-first; result.ErrorCode != "expired" {
		t.Fatalf("unexpected running action result: %+v", result)
	}
	if result := <-second; result.Accepted || result.ErrorCode != "queue_closed" {
		t.Fatalf("unexpected pending action result: %+v", result)
	}
}

type gatedExecutor struct{ started chan struct{} }

func (e *gatedExecutor) Execute(ctx context.Context, _ Request) error {
	select {
	case <-e.started:
	default:
		close(e.started)
	}
	<-ctx.Done()
	return ctx.Err()
}

type blockingExecutor struct{}

func (blockingExecutor) Execute(ctx context.Context, _ Request) error {
	<-ctx.Done()
	return ctx.Err()
}

func TestQueueExpiresAction(t *testing.T) {
	q := New(blockingExecutor{}, 1)
	result := <-q.Submit(context.Background(), Request{RequestID: "expired", DeviceID: "phone", Action: "tap", X: .5, Y: .5, ExpiresMS: 1})
	if !result.Accepted || result.Executed || result.ErrorCode != "expired" {
		t.Fatalf("unexpected expiry result: %+v", result)
	}
}

func TestQueueRejectsWhenFull(t *testing.T) {
	q := New(blockingExecutor{}, 1)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	first := q.Submit(ctx, Request{RequestID: "first", DeviceID: "phone", Action: "tap", X: .5, Y: .5})
	second := <-q.Submit(context.Background(), Request{RequestID: "second", DeviceID: "phone", Action: "tap", X: .5, Y: .5})
	if second.Accepted || second.ErrorCode != "queue_full" {
		t.Fatalf("unexpected queue-full result: %+v", second)
	}
	cancel()
	select {
	case <-first:
	case <-time.After(time.Second):
		t.Fatal("first action did not finish after context cancellation")
	}
}

func TestQueueValidatesMode(t *testing.T) {
	for _, tc := range []struct {
		mode  string
		valid bool
	}{
		{mode: "", valid: true},
		{mode: "sdk", valid: true},
		{mode: "adb", valid: true},
		{mode: "invalid", valid: false},
		{mode: "SHELL", valid: false},
		{mode: "fastboot", valid: false},
	} {
		q := New(fakeExecutor{}, 1)
		req := Request{
			RequestID:  "mode-test",
			DeviceID:   "phone",
			Action:     "tap",
			X:          0.5,
			Y:          0.5,
			Mode:       tc.mode,
			DurationMS: 500,
		}
		res := <-q.Submit(context.Background(), req)
		if tc.valid {
			if !res.Accepted || !res.Executed || res.ErrorCode != "" {
				t.Fatalf("expected mode %q to be accepted, got %+v", tc.mode, res)
			}
		} else {
			if !res.Accepted || res.Executed || res.ErrorCode != "unsupported_mode" {
				t.Fatalf("expected mode %q to fail with unsupported_mode, got %+v", tc.mode, res)
			}
		}
	}
}

func TestRequestJSONSerialization(t *testing.T) {
	req := Request{
		RequestID:  "req-1",
		DeviceID:   "device-1",
		Action:     "swipe",
		X:          0.1,
		Y:          0.2,
		X2:         0.3,
		Y2:         0.4,
		Mode:       "adb",
		DurationMS: 300,
	}
	data, err := json.Marshal(req)
	if err != nil {
		t.Fatalf("marshal failed: %v", err)
	}
	var decoded Request
	if err := json.Unmarshal(data, &decoded); err != nil {
		t.Fatalf("unmarshal failed: %v", err)
	}
	if decoded.Mode != "adb" || decoded.DurationMS != 300 {
		t.Fatalf("unexpected decoded request: %+v", decoded)
	}
}
