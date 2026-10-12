package taskmanager

import (
	"context"
	"fmt"
	"mywebscrcpy/internal/taskartifact"
	"net/http"
	"sync"
	"testing"
	"time"
)

type testRecorder struct {
	mu            sync.Mutex
	starts, stops []string
	fail          bool
	gate          chan struct{}
	infos         map[string]taskartifact.RecordingInfo
}

func (r *testRecorder) StartTaskRecording(ctx context.Context, serial, id string, d time.Duration, audio bool) (taskartifact.RecordingInfo, error) {
	r.mu.Lock()
	r.starts = append(r.starts, id)
	r.mu.Unlock()
	if r.gate != nil {
		select {
		case <-ctx.Done():
			return taskartifact.RecordingInfo{}, ctx.Err()
		case <-r.gate:
		}
	}
	if r.fail {
		return taskartifact.RecordingInfo{}, fmt.Errorf("device already recording")
	}
	info := taskartifact.RecordingInfo{ID: "rec_" + id, State: "recording", StartedAt: time.Now().UnixMilli()}
	r.mu.Lock()
	r.infos[info.ID] = info
	r.mu.Unlock()
	return info, nil
}
func (r *testRecorder) StopTaskRecording(recording, serial, id string) (taskartifact.RecordingInfo, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if recording != "rec_"+id {
		return taskartifact.RecordingInfo{}, fmt.Errorf("owner mismatch")
	}
	r.stops = append(r.stops, id)
	info := r.infos[recording]
	info.State = "stopping"
	r.infos[recording] = info
	return info, nil
}
func (r *testRecorder) TaskRecording(recording, serial, id string) (taskartifact.RecordingInfo, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	return r.infos[recording], nil
}
func (r *testRecorder) ServeTaskRecording(http.ResponseWriter, *http.Request, string, string, string) error {
	return nil
}
func TestRecordingFailureContinuesAndSuccessfulStopIsOwned(t *testing.T) {
	_, m := artifactManager(t)
	r := &testRecorder{fail: true, infos: map[string]taskartifact.RecordingInfo{}}
	m.Artifacts.SetRecorder(r)
	options := taskartifact.Options{Recording: taskartifact.RecordingOptions{Enabled: true}}
	task, e := m.StartDispatchedTask(1, "test", "device-a", 3, "print('continues')", "", options)
	if e != nil {
		t.Fatal(e)
	}
	st := artifactDone(t, m, task.ID)
	if st.Artifacts.Recording.State != "failed" {
		t.Fatal(st)
	}
	r.fail = false
	task, e = m.StartDispatchedTask(1, "test", "device-a", 3, "print('recorded')", "", options)
	if e != nil {
		t.Fatal(e)
	}
	artifactDone(t, m, task.ID)
	r.mu.Lock()
	defer r.mu.Unlock()
	if len(r.stops) != 1 || r.stops[0] != task.ID {
		t.Fatalf("stopped another recording %+v", r.stops)
	}
}
func TestKillDuringPreparationDoesNotBlockOtherDevice(t *testing.T) {
	_, m := artifactManager(t)
	r := &testRecorder{gate: make(chan struct{}), infos: map[string]taskartifact.RecordingInfo{}}
	m.Artifacts.SetRecorder(r)
	done := make(chan error, 1)
	go func() {
		_, e := m.StartDispatchedTask(1, "preparing", "device-a", 3, "print('must not run')", "", taskartifact.Options{Recording: taskartifact.RecordingOptions{Enabled: true}})
		done <- e
	}()
	var id string
	end := time.Now().Add(3 * time.Second)
	for time.Now().Before(end) {
		active, _ := m.ListActiveTasks()
		if len(active) > 0 {
			id = active[0].ID
			break
		}
		time.Sleep(time.Millisecond)
	}
	if id == "" {
		t.Fatal("reservation missing")
	}
	other, e := m.StartTask(2, "other", "device-b", 3, "print('independent')")
	if e != nil {
		t.Fatal(e)
	}
	artifactDone(t, m, other.ID)
	if e = m.KillTask(id); e != nil {
		t.Fatal(e)
	}
	if e = <-done; e == nil {
		t.Fatal("cancelled preparation started Python")
	}
	st := artifactDone(t, m, id)
	if st.Artifacts.Recording.State != "failed" {
		t.Fatal(st)
	}
}
func TestStartupRecoveryVerifiesOwnedPythonPath(t *testing.T) {
	d, m := artifactManager(t)
	task, e := m.StartTask(1, "recover", "device-a", 10, "import time\nprint('started')\ntime.sleep(20)")
	if e != nil {
		t.Fatal(e)
	}
	other := NewManager(d)
	defer other.Close()
	if e = other.Recover(); e != nil {
		t.Fatal(e)
	}
	end := time.Now().Add(3 * time.Second)
	for time.Now().Before(end) {
		if exited, _ := processExited(task.PID); exited {
			break
		}
		time.Sleep(time.Millisecond)
	}
	if exited, e := processExited(task.PID); !exited || e != nil {
		t.Fatalf("survivor still running %v", e)
	}
}
