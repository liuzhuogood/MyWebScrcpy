// Package action serializes device actions and keeps automation behind one gate.
package action

import (
	"context"
	"errors"
	"fmt"
	"math"
	"sync"
	"time"
)

type Request struct {
	RequestID string  `json:"request_id"`
	DeviceID  string  `json:"device_id"`
	Source    string  `json:"source"`
	Action    string  `json:"action"`
	X         float64 `json:"x,omitempty"`
	Y         float64 `json:"y,omitempty"`
	X2        float64 `json:"x2,omitempty"`
	Y2        float64 `json:"y2,omitempty"`
	Text      string  `json:"text,omitempty"`
	Keycode   uint32  `json:"keycode,omitempty"`
	MetaState uint32  `json:"meta_state,omitempty"`
	Raw       []byte  `json:"-"`
	FrameID   uint64  `json:"frame_id,omitempty"`
	ExpiresMS int64   `json:"expires_ms,omitempty"`
}

type Result struct {
	RequestID string `json:"request_id"`
	Accepted  bool   `json:"accepted"`
	Executed  bool   `json:"executed"`
	ErrorCode string `json:"error_code,omitempty"`
}

type Executor interface {
	Execute(context.Context, Request) error
}

type Queue struct {
	exec   Executor
	ch     chan job
	closed chan struct{}
	once   sync.Once
	mu     sync.RWMutex
}

func (q *Queue) Len() int { return len(q.ch) }

type job struct {
	ctx  context.Context
	req  Request
	done chan Result
}

func New(exec Executor, size int) *Queue {
	if size < 1 {
		size = 1
	}
	q := &Queue{exec: exec, ch: make(chan job, size), closed: make(chan struct{})}
	go q.loop()
	return q
}

// Close stops the worker. It is safe to call more than once.
func (q *Queue) Close() {
	q.mu.Lock()
	defer q.mu.Unlock()
	q.once.Do(func() { close(q.closed) })
}

func (q *Queue) Submit(ctx context.Context, req Request) <-chan Result {
	out := make(chan Result, 1)
	if req.RequestID == "" {
		req.RequestID = fmt.Sprintf("action-%d", time.Now().UnixNano())
	}
	q.mu.RLock()
	defer q.mu.RUnlock()
	select {
	case <-q.closed:
		out <- Result{RequestID: req.RequestID, Accepted: false, ErrorCode: "queue_closed"}
		return out
	default:
	}
	select {
	case q.ch <- job{ctx: ctx, req: req, done: out}:
	default:
		out <- Result{RequestID: req.RequestID, Accepted: false, ErrorCode: "queue_full"}
	}
	return out
}

func (q *Queue) loop() {
	for {
		select {
		case <-q.closed:
			q.rejectPending()
			return
		case j := <-q.ch:
			select {
			case <-q.closed:
				j.done <- Result{RequestID: j.req.RequestID, Accepted: false, ErrorCode: "queue_closed"}
				q.rejectPending()
				return
			default:
			}
			if err := validate(j.req); err != nil {
				j.done <- Result{RequestID: j.req.RequestID, Accepted: true, ErrorCode: err.Error()}
				continue
			}
			ctx := j.ctx
			cancel := func() {}
			if j.req.ExpiresMS > 0 {
				ctx, cancel = context.WithTimeout(ctx, time.Duration(j.req.ExpiresMS)*time.Millisecond)
			}
			err := q.exec.Execute(ctx, j.req)
			cancel()
			r := Result{RequestID: j.req.RequestID, Accepted: true, Executed: err == nil}
			if err != nil {
				if errors.Is(err, context.DeadlineExceeded) {
					r.ErrorCode = "expired"
				} else {
					r.ErrorCode = "execution_failed"
				}
			}
			j.done <- r
			select {
			case <-q.closed:
				q.rejectPending()
				return
			default:
			}
		}
	}
}

func (q *Queue) rejectPending() {
	for {
		select {
		case pending := <-q.ch:
			pending.done <- Result{RequestID: pending.req.RequestID, Accepted: false, ErrorCode: "queue_closed"}
		default:
			return
		}
	}
}

func validate(r Request) error {
	if r.DeviceID == "" {
		return errors.New("missing_device_id")
	}
	switch r.Action {
	case "tap", "swipe", "key", "text", "raw":
	default:
		return errors.New("unsupported_action")
	}
	if (r.Action == "tap" || r.Action == "swipe") && !validUnitPoint(r.X, r.Y) {
		return errors.New("invalid_coordinate")
	}
	if r.Action == "swipe" && !validUnitPoint(r.X2, r.Y2) {
		return errors.New("invalid_coordinate")
	}
	if r.Action == "key" && r.Keycode == 0 {
		return errors.New("missing_keycode")
	}
	if r.Action == "text" && r.Text == "" {
		return errors.New("missing_text")
	}
	if r.Action == "raw" && len(r.Raw) == 0 {
		return errors.New("empty_raw_action")
	}
	return nil
}

func validUnitPoint(x, y float64) bool {
	return !math.IsNaN(x) && !math.IsNaN(y) && !math.IsInf(x, 0) && !math.IsInf(y, 0) && x >= 0 && x <= 1 && y >= 0 && y <= 1
}
