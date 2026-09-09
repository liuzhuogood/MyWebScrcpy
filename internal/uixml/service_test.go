package uixml

import (
	"context"
	"errors"
	"sync"
	"testing"
	"time"
)

func TestFetchRunsDumpThenRead(t *testing.T) {
	var calls [][]string
	s := NewWithRunner(func(_ context.Context, args ...string) ([]byte, error) {
		calls = append(calls, args)
		if len(calls) == 1 {
			return []byte("UI dump written"), nil
		}
		return []byte(`<hierarchy><node text="ok"/></hierarchy>`), nil
	})
	x, captured, err := s.Fetch(context.Background(), "device-1")
	if err != nil || x == "" || captured.IsZero() {
		t.Fatalf("fetch: %q %v %v", x, captured, err)
	}
	if len(calls) != 2 || calls[0][3] != "uiautomator" || calls[1][2] != "exec-out" {
		t.Fatalf("calls: %#v", calls)
	}
}

func TestFetchRejectsInvalidAndTooLargeXML(t *testing.T) {
	for _, input := range []string{"<a/><b/>", "<a>", "junk<a/>", "<a/>junk"} {
		s := NewWithRunner(func(_ context.Context, args ...string) ([]byte, error) {
			if args[2] == "shell" {
				return nil, nil
			}
			return []byte(input), nil
		})
		if _, _, err := s.Fetch(context.Background(), "d"); !errors.Is(err, ErrInvalidXML) {
			t.Errorf("input %q: got %v", input, err)
		}
	}
	s := NewWithRunner(func(_ context.Context, args ...string) ([]byte, error) {
		if args[2] == "shell" {
			return nil, nil
		}
		return []byte(`<a/>`), nil
	})
	s.max = 2
	if _, _, err := s.Fetch(context.Background(), "d"); !errors.Is(err, ErrTooLarge) {
		t.Fatalf("large: %v", err)
	}
}

func TestFetchSerializesConcurrentDumps(t *testing.T) {
	var mu sync.Mutex
	active, maxActive := 0, 0
	s := NewWithRunner(func(_ context.Context, args ...string) ([]byte, error) {
		mu.Lock()
		active++
		if active > maxActive {
			maxActive = active
		}
		mu.Unlock()
		time.Sleep(5 * time.Millisecond)
		mu.Lock()
		active--
		mu.Unlock()
		if args[2] == "shell" {
			return nil, nil
		}
		return []byte(`<a/>`), nil
	})
	var wg sync.WaitGroup
	for i := 0; i < 5; i++ {
		wg.Add(1)
		go func() { defer wg.Done(); _, _, _ = s.Fetch(context.Background(), "d") }()
	}
	wg.Wait()
	if maxActive != 1 {
		t.Fatalf("max concurrent commands = %d", maxActive)
	}
}
