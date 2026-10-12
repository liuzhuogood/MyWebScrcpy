package ws

import (
	"context"
	"fmt"
	"mywebscrcpy/internal/taskartifact"
	"net/http"
	"time"
)

func (h *Hub) StartTaskRecording(ctx context.Context, serial, owner string, duration time.Duration, audio bool) (taskartifact.RecordingInfo, error) {
	if owner == "" {
		return taskartifact.RecordingInfo{}, fmt.Errorf("录制缺少任务归属")
	}
	type result struct {
		rec *recording
		err error
	}
	done := make(chan result)
	go func() {
		rec, e := h.recordings.startOwned(serial, duration, audio, owner)
		select {
		case done <- result{rec, e}:
		case <-ctx.Done():
			if rec != nil {
				h.StopTaskRecording(rec.id, serial, owner)
			}
		}
	}()
	select {
	case <-ctx.Done():
		return taskartifact.RecordingInfo{}, fmt.Errorf("录制准备已取消或超时")
	case r := <-done:
		if r.err != nil {
			return taskartifact.RecordingInfo{}, r.err
		}
		if ctx.Err() != nil {
			h.StopTaskRecording(r.rec.id, serial, owner)
			return taskartifact.RecordingInfo{}, ctx.Err()
		}
		return taskRecordingInfo(r.rec), nil
	}
}
func taskRecordingInfo(rec *recording) taskartifact.RecordingInfo {
	rec.mu.Lock()
	defer rec.mu.Unlock()
	info := taskartifact.RecordingInfo{ID: rec.id, State: string(rec.status), StartedAt: rec.startedAt.UnixMilli(), CoverageComplete: rec.taskStopRequested && !rec.videoStartedAt.IsZero(), ErrorMsg: rec.failureReason}
	if !rec.videoStartedAt.IsZero() {
		info.StartedAt = rec.videoStartedAt.UnixMilli()
	}
	if !rec.endedAt.IsZero() {
		info.EndedAt = rec.endedAt.UnixMilli()
	}
	return info
}
func (h *Hub) ownedRecording(id, serial, owner string) (*recording, error) {
	rec, e := h.recordings.get(id, serial)
	if e != nil {
		return nil, e
	}
	rec.mu.Lock()
	valid := owner != "" && owner == rec.ownerTaskID
	rec.mu.Unlock()
	if !valid {
		return nil, fmt.Errorf("录制不属于本次任务")
	}
	return rec, nil
}
func (h *Hub) StopTaskRecording(id, serial, owner string) (taskartifact.RecordingInfo, error) {
	rec, e := h.ownedRecording(id, serial, owner)
	if e != nil {
		return taskartifact.RecordingInfo{}, e
	}
	rec.mu.Lock()
	if rec.status == recordingActive {
		rec.taskStopRequested = true
	}
	rec.mu.Unlock()
	if _, e = h.recordings.stopRecording(id, serial); e != nil {
		return taskartifact.RecordingInfo{}, e
	}
	return taskRecordingInfo(rec), nil
}
func (h *Hub) TaskRecording(id, serial, owner string) (taskartifact.RecordingInfo, error) {
	rec, e := h.ownedRecording(id, serial, owner)
	if e != nil {
		return taskartifact.RecordingInfo{}, e
	}
	return taskRecordingInfo(rec), nil
}
func (h *Hub) ServeTaskRecording(w http.ResponseWriter, r *http.Request, id, serial, owner string) error {
	rec, e := h.ownedRecording(id, serial, owner)
	if e != nil {
		return e
	}
	h.serveRecordingDownload(w, r, rec)
	return nil
}
