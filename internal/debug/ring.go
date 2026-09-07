// Package debug stores a bounded, non-sensitive event history for diagnostics.
package debug

import (
	"sync"
	"time"
)

type Event struct {
	At        time.Time              `json:"at"`
	Type      string                 `json:"type"`
	DeviceID  string                 `json:"device_id,omitempty"`
	SessionID string                 `json:"session_id,omitempty"`
	RequestID string                 `json:"request_id,omitempty"`
	Message   string                 `json:"message,omitempty"`
	Fields    map[string]interface{} `json:"fields,omitempty"`
}

type Ring struct {
	mu     sync.RWMutex
	cap    int
	events []Event
}

func New(capacity int) *Ring {
	if capacity < 1 {
		capacity = 1
	}
	return &Ring{cap: capacity}
}

func (r *Ring) Append(e Event) {
	if e.At.IsZero() {
		e.At = time.Now()
	}
	if e.Fields != nil {
		fields := make(map[string]interface{}, len(e.Fields))
		for k, v := range e.Fields {
			fields[k] = v
		}
		e.Fields = fields
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	if len(r.events) == r.cap {
		copy(r.events, r.events[1:])
		r.events[len(r.events)-1] = e
		return
	}
	r.events = append(r.events, e)
}

func (r *Ring) Snapshot(deviceID string, limit int) []Event {
	r.mu.RLock()
	defer r.mu.RUnlock()
	if limit <= 0 || limit > len(r.events) {
		limit = len(r.events)
	}
	out := make([]Event, 0, limit)
	for i := len(r.events) - 1; i >= 0 && len(out) < limit; i-- {
		if deviceID != "" && r.events[i].DeviceID != deviceID {
			continue
		}
		e := r.events[i]
		if e.Fields != nil {
			e.Fields = cloneFields(e.Fields)
		}
		out = append(out, e)
	}
	return out
}

func cloneFields(in map[string]interface{}) map[string]interface{} {
	out := make(map[string]interface{}, len(in))
	for k, v := range in {
		out[k] = v
	}
	return out
}
