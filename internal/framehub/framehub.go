// Package framehub provides a bounded latest-frame fan-out for vision consumers.
package framehub

import (
	"sync"
	"time"
)

type Frame struct {
	DeviceID  string
	SessionID string
	FrameID   uint64
	PTS       uint64
	Timestamp time.Time
	Width     uint32
	Height    uint32
	Key       bool
	Payload   []byte
}

type Hub struct {
	mu       sync.RWMutex
	nextID   uint64
	latest   *Frame
	subs     map[uint64]chan *Frame
	nextSub  uint64
	maxQueue int
}

func New(maxQueue int) *Hub {
	if maxQueue < 1 {
		maxQueue = 1
	}
	return &Hub{subs: make(map[uint64]chan *Frame), maxQueue: maxQueue}
}

func (h *Hub) Publish(f Frame) Frame {
	h.mu.Lock()
	h.nextID++
	if f.FrameID == 0 {
		f.FrameID = h.nextID
	}
	if f.Timestamp.IsZero() {
		f.Timestamp = time.Now()
	}
	f.Payload = append([]byte(nil), f.Payload...)
	h.latest = &f
	for _, ch := range h.subs {
		select {
		case ch <- &f:
		default:
			select {
			case <-ch:
			default:
			}
			select {
			case ch <- &f:
			default:
			}
		}
	}
	h.mu.Unlock()
	return f
}

func (h *Hub) Latest() *Frame {
	h.mu.RLock()
	defer h.mu.RUnlock()
	if h.latest == nil {
		return nil
	}
	f := *h.latest
	f.Payload = append([]byte(nil), f.Payload...)
	return &f
}

func (h *Hub) Subscribe() (uint64, <-chan *Frame, func()) {
	h.mu.Lock()
	defer h.mu.Unlock()
	h.nextSub++
	id := h.nextSub
	ch := make(chan *Frame, h.maxQueue)
	h.subs[id] = ch
	return id, ch, func() {
		h.mu.Lock()
		if c, ok := h.subs[id]; ok {
			delete(h.subs, id)
			close(c)
		}
		h.mu.Unlock()
	}
}
