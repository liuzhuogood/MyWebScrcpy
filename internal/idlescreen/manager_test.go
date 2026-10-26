package idlescreen

import (
	"context"
	"errors"
	"sync"
	"testing"
	"time"
)

type memoryStore struct {
	raw string
	err error
}

func (s *memoryStore) GetConfig(string) (string, error) { return s.raw, s.err }
func (s *memoryStore) SetConfig(_ string, value string) error {
	if s.err == nil {
		s.raw = value
	}
	return s.err
}

func fixture() (*Manager, *time.Time, *[]string, *[]string) {
	now := time.Unix(1000, 0)
	online := []string{"a", "b"}
	slept := []string{}
	m := &Manager{store: &memoryStore{}, devices: map[string]*deviceState{}, now: func() time.Time { return now }, timeout: IdleTimeout, online: func() []string { return online }, sleep: func(_ context.Context, s string) error { slept = append(slept, s); return nil }, ctx: context.Background(), jobs: make(chan sleepJob, 32)}
	return m, &now, &online, &slept
}
func drain(m *Manager) {
	for {
		select {
		case j := <-m.jobs:
			m.execute(j)
		default:
			return
		}
	}
}

func TestDisabledAndPerDeviceIdle(t *testing.T) {
	m, now, _, slept := fixture()
	m.scan()
	*now = now.Add(2 * IdleTimeout)
	m.scan()
	drain(m)
	if len(*slept) != 0 {
		t.Fatal("disabled setting slept device")
	}
	if err := m.Set(Config{Enabled: true}); err != nil {
		t.Fatal(err)
	}
	release := m.Begin("a")
	*now = now.Add(IdleTimeout - time.Nanosecond)
	m.scan()
	drain(m)
	if len(*slept) != 0 {
		t.Fatal("slept before timeout")
	}
	*now = now.Add(time.Nanosecond)
	m.scan()
	drain(m)
	if len(*slept) != 1 || (*slept)[0] != "b" {
		t.Fatalf("active stream/command must protect only its device: %v", *slept)
	}
	release()
	release()
	*now = now.Add(IdleTimeout - time.Second)
	m.scan()
	drain(m)
	if len(*slept) != 1 {
		t.Fatal("idle countdown must start at release")
	}
	*now = now.Add(time.Second)
	m.scan()
	drain(m)
	if len(*slept) != 2 || (*slept)[1] != "a" {
		t.Fatalf("released device did not sleep: %v", *slept)
	}
	*now = now.Add(IdleTimeout)
	m.scan()
	drain(m)
	if len(*slept) != 2 {
		t.Fatal("repeated sleep without new work")
	}
}

func TestQueuedSleepInvalidated(t *testing.T) {
	for _, change := range []string{"activity", "disabled", "reconnect"} {
		t.Run(change, func(t *testing.T) {
			m, now, online, slept := fixture()
			*online = []string{"a"}
			_ = m.Set(Config{Enabled: true})
			m.scan()
			*now = now.Add(IdleTimeout)
			m.scan()
			switch change {
			case "activity":
				m.Begin("a")()
			case "disabled":
				_ = m.Set(Config{})
			case "reconnect":
				*online = nil
				m.scan()
				*online = []string{"a"}
				m.scan()
			}
			drain(m)
			if len(*slept) != 0 {
				t.Fatalf("stale sleep after %s", change)
			}
		})
	}
}

func TestNewWorkCancelsInFlightSleep(t *testing.T) {
	m, now, online, _ := fixture()
	*online = []string{"a"}
	_ = m.Set(Config{Enabled: true})
	m.scan()
	started := make(chan struct{})
	finished := make(chan struct{})
	m.sleep = func(ctx context.Context, _ string) error { close(started); <-ctx.Done(); return ctx.Err() }
	*now = now.Add(IdleTimeout)
	m.scan()
	job := <-m.jobs
	go func() { m.execute(job); close(finished) }()
	<-started
	release := m.Begin("a")
	defer release()
	select {
	case <-finished:
	case <-time.After(time.Second):
		t.Fatal("new work did not cancel sleep")
	}
	if m.devices["a"].sleeping {
		t.Fatal("stale sleep result overwrote active state")
	}
}

func TestSleepFailureRetriesAndSaveFailure(t *testing.T) {
	m, now, online, _ := fixture()
	*online = []string{"a"}
	_ = m.Set(Config{Enabled: true})
	m.scan()
	calls := 0
	m.sleep = func(context.Context, string) error { calls++; return errors.New("offline") }
	*now = now.Add(IdleTimeout)
	m.scan()
	drain(m)
	m.scan()
	drain(m)
	if calls != 1 {
		t.Fatal("sleep failures must back off")
	}
	*now = now.Add(time.Minute)
	m.scan()
	drain(m)
	if calls != 2 {
		t.Fatal("sleep failure did not retry")
	}
	m.store.(*memoryStore).err = errors.New("disk full")
	if m.Set(Config{}) == nil || !m.Config().Enabled {
		t.Fatal("failed persistence changed active setting")
	}
}

func TestRestartRestoresSettingAndCloseCancels(t *testing.T) {
	store := &memoryStore{raw: `{"enabled":true}`}
	var mu sync.Mutex
	m := New(store, func() []string { mu.Lock(); defer mu.Unlock(); return nil }, func(context.Context, string) error { return nil })
	if !m.Config().Enabled {
		t.Fatal("saved setting was not restored")
	}
	m.Close()
}
