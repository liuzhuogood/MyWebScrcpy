package action

import (
	"context"
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
