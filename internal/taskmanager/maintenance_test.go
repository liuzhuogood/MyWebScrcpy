package taskmanager

import (
	"mywebscrcpy/internal/taskartifact"
	"testing"
	"time"
)

func TestMaintenanceAndCloseRejectDispatchedTasks(t *testing.T) {
	_, m := artifactManager(t)
	if err := m.BeginMaintenance(); err != nil {
		t.Fatal(err)
	}
	if _, err := m.StartDispatchedTask(1, "test", "device-a", 3, "print('blocked')", "dispatch", taskartifact.Options{}); err == nil {
		t.Fatal("maintenance allowed a dispatched task")
	}
	m.EndMaintenance()
	task, err := m.StartTask(1, "test", "device-a", 3, "print('resumed')")
	if err != nil {
		t.Fatal(err)
	}
	artifactDone(t, m, task.ID)
	m.Close()
	if _, err := m.StartTask(1, "test", "device-a", 3, "print('blocked')"); err == nil {
		t.Fatal("closed manager allowed a task")
	}
}

func TestCloseCancelsPreparingTaskAndFinalizesArtifacts(t *testing.T) {
	_, m := artifactManager(t)
	r := &testRecorder{gate: make(chan struct{}), infos: map[string]taskartifact.RecordingInfo{}}
	m.Artifacts.SetRecorder(r)
	started := make(chan error, 1)
	go func() {
		_, err := m.StartDispatchedTask(1, "preparing", "device-a", 3, "print('must not run')", "dispatch", taskartifact.Options{Recording: taskartifact.RecordingOptions{Enabled: true}})
		started <- err
	}()
	var id string
	deadline := time.Now().Add(3 * time.Second)
	for time.Now().Before(deadline) {
		r.mu.Lock()
		if len(r.starts) > 0 {
			id = r.starts[0]
		}
		r.mu.Unlock()
		if id != "" {
			break
		}
		time.Sleep(time.Millisecond)
	}
	if id == "" {
		t.Fatal("task did not reach recording preparation")
	}
	if err := m.BeginMaintenance(); err == nil {
		t.Fatal("maintenance ignored a preparing task")
	}
	closed := make(chan struct{})
	go func() { m.Close(); close(closed) }()
	select {
	case <-closed:
	case <-time.After(3 * time.Second):
		t.Fatal("close did not finish")
	}
	if err := <-started; err == nil {
		t.Fatal("cancelled preparation started Python")
	}
	if state := artifactDone(t, m, id); state.Artifacts.Recording.State != "failed" {
		t.Fatalf("recording was not finalized: %+v", state.Artifacts.Recording)
	}
}
