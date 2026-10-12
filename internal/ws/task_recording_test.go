package ws

import (
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestTaskRecordingOwnershipPersistenceAndRange(t *testing.T) {
	dir := t.TempDir()
	h := &Hub{}
	h.recordings = &recordingManager{dir: dir, entries: map[string]*recording{}, bySerial: map[string]string{}}
	rec := &recording{id: "owned", ownerTaskID: "task-a", serial: "phone-a", status: recordingActive, startedAt: time.Now().Add(-time.Minute), path: filepath.Join(dir, "owned.mp4"), stop: make(chan struct{})}
	h.recordings.entries[rec.id] = rec
	if _, e := h.StopTaskRecording(rec.id, rec.serial, "task-b"); e == nil {
		t.Fatal("other task stopped recording")
	}
	if rec.status != recordingActive {
		t.Fatal("ownership check modified recording")
	}
	if _, e := h.StopTaskRecording(rec.id, "phone-b", "task-a"); e == nil {
		t.Fatal("wrong serial accepted")
	}
	if _, e := h.StopTaskRecording(rec.id, rec.serial, "task-a"); e != nil {
		t.Fatal(e)
	}
	rec.status = recordingComplete
	rec.endedAt = time.Now()
	os.WriteFile(rec.path, []byte("0123456789"), 0600)
	h.recordings.persist(rec)
	restored := &recordingManager{dir: dir, entries: map[string]*recording{}, bySerial: map[string]string{}}
	restored.loadPersisted()
	h.recordings = restored
	if _, e := h.TaskRecording("owned", "phone-a", "task-a"); e != nil {
		t.Fatal(e)
	}
	request := httptest.NewRequest("GET", "/media", nil)
	request.Header.Set("Range", "bytes=2-5")
	response := httptest.NewRecorder()
	if e := h.ServeTaskRecording(response, request, "owned", "phone-a", "task-a"); e != nil {
		t.Fatal(e)
	}
	if response.Code != 206 || response.Body.String() != "2345" {
		t.Fatalf("Range failed %d %s", response.Code, response.Body.String())
	}
	restored.entries["owned"].status = recordingExpired
	response = httptest.NewRecorder()
	h.ServeTaskRecording(response, request, "owned", "phone-a", "task-a")
	if response.Code != 410 {
		t.Fatalf("expired status=%d", response.Code)
	}
}
