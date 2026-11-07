package scrcpy

import (
	"context"
	"errors"
	"io"
	"net"
	"testing"
	"time"
)

func TestWriteControlContextCancelsBlockedWrite(t *testing.T) {
	client, server := net.Pipe()
	defer client.Close()
	defer server.Close()
	c := &Connection{ctrlConn: client}
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan error, 1)
	go func() { done <- c.WriteControlContext(ctx, []byte{1, 2, 3}) }()
	cancel()
	select {
	case err := <-done:
		if !errors.Is(err, context.Canceled) {
			t.Fatalf("want cancellation, got %v", err)
		}
	case <-time.After(time.Second):
		t.Fatal("blocked control write ignored cancellation")
	}
}

func TestWriteControlContextReleasesDeadlineForNextWrite(t *testing.T) {
	client, server := net.Pipe()
	defer client.Close()
	defer server.Close()
	c := &Connection{ctrlConn: client}
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Millisecond)
	defer cancel()
	if err := c.WriteControlContext(ctx, []byte{1}); !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("want deadline, got %v", err)
	}
	// No bytes were written, so the same test pipe can prove cancellation did not
	// leave an expired socket deadline behind.
	go func() { b := make([]byte, 1); _, _ = io.ReadFull(server, b) }()
	if err := c.WriteControl([]byte{2}); err != nil {
		t.Fatalf("deadline leaked to next write: %v", err)
	}
}

func TestWriteControlContextDeadlineWhileWaitingForWriter(t *testing.T) {
	client, server := net.Pipe()
	defer client.Close()
	defer server.Close()
	c := &Connection{ctrlConn: client}
	firstCtx, cancel := context.WithCancel(context.Background())
	defer cancel()
	first := make(chan error, 1)
	go func() { first <- c.WriteControlContext(firstCtx, []byte{1}) }()
	// The first write remains blocked whichever goroutine acquires the gate first.
	ctx, stop := context.WithTimeout(context.Background(), 20*time.Millisecond)
	defer stop()
	started := time.Now()
	if err := c.WriteControlContext(ctx, []byte{2}); !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("want deadline, got %v", err)
	}
	if time.Since(started) > time.Second {
		t.Fatal("writer lock ignored context")
	}
	cancel()
	<-first
}

type partialControlConnection struct {
	net.Conn
	closed bool
}

func (c *partialControlConnection) Write([]byte) (int, error) { return 1, io.ErrUnexpectedEOF }
func (c *partialControlConnection) Close() error              { c.closed = true; return c.Conn.Close() }
func TestPartialControlWriteClosesSession(t *testing.T) {
	a, b := net.Pipe()
	defer b.Close()
	ctrl := &partialControlConnection{Conn: a}
	v1, v2 := net.Pipe()
	defer v2.Close()
	c := &Connection{ctrlConn: ctrl, videoConn: v1}
	if err := c.WriteControlContext(context.Background(), []byte{1, 2, 3}); err == nil {
		t.Fatal("partial write was treated as success")
	}
	if !ctrl.closed {
		t.Fatal("partial control packet left socket reusable")
	}
	if _, err := v2.Write([]byte{1}); !errors.Is(err, io.ErrClosedPipe) {
		t.Fatalf("partial write did not close video session: %v", err)
	}
}
