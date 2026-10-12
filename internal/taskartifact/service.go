package taskartifact

import (
	"context"
	"fmt"
	"log"
	"mywebscrcpy/internal/db"
	"net/http"
	"sync"
	"time"
)

type Service struct {
	database *db.DB
	Logs     *LogStore
	initErr  error
	mu       sync.Mutex
	recorder Recorder
	model    Summarize
	cancel   context.CancelFunc
	wg       sync.WaitGroup
	wake     chan struct{}
}

func NewService(database *db.DB) *Service {
	s := &Service{database: database, wake: make(chan struct{}, 1)}
	s.Logs, s.initErr = NewLogStore(database.ArtifactDir(), 20<<20, 1<<30)
	if s.Logs != nil {
		s.Logs.Cleanup()
	}
	return s
}
func (s *Service) SetRecorder(r Recorder) { s.recorder = r }
func (s *Service) ConfigureReports(model Summarize) {
	s.model = model
	ctx, cancel := context.WithCancel(context.Background())
	s.cancel = cancel
	for i := 0; i < 2; i++ {
		s.wg.Add(1)
		go s.reportWorker(ctx)
	}
}
func (s *Service) Close() {
	if s.cancel != nil {
		s.cancel()
		s.wg.Wait()
	}
}
func (s *Service) update(id string, fn func(*State)) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	var state State
	found, e := s.database.ArtifactState(id, &state)
	if e != nil {
		return e
	}
	if !found {
		return fmt.Errorf("任务产物不存在")
	}
	fn(&state)
	return s.database.SaveArtifactState(id, state)
}
func (s *Service) Create(id, dispatch string, options Options) (*LogCapture, error) {

	state := State{DispatchID: dispatch, PreparationState: "preparing", Options: options, Artifacts: Artifacts{Log: LogState{Availability: "available", CaptureState: "waiting"}, Recording: RecordingState{State: "disabled"}, Report: ReportState{State: "disabled"}}}
	if options.Recording.Enabled {
		audio := options.Recording.RecordAudio
		state.Artifacts.Recording = RecordingState{State: "not_started", RecordAudio: &audio}
	}
	if options.Report.Enabled {
		state.Artifacts.Report.State = "waiting"
	}
	if e := s.database.SaveArtifactState(id, state); e != nil {
		return nil, e
	}
	if s.initErr != nil {
		return nil, fmt.Errorf("初始化日志存储失败: %w", s.initErr)
	}
	w, e := s.Logs.Create(id, func(v LogState) {
		if e := s.update(id, func(st *State) { st.Artifacts.Log = v }); e != nil {
			log.Printf("[TaskArtifacts] task=%s 日志元数据保存失败: %v", id, e)
		}
	})
	if e != nil {
		return nil, e
	}
	return w, nil
}
func (s *Service) State(id string) (State, error) {
	var state State
	found, e := s.database.ArtifactState(id, &state)
	if e != nil {
		return state, e
	}
	if !found {
		state.PreparationState = "finished"
		state.Artifacts = Artifacts{Log: LogState{Availability: "unavailable", CaptureState: "completed"}, Recording: RecordingState{State: "unavailable"}, Report: ReportState{State: "unavailable"}}
		state.DispatchID, e = s.database.TaskDispatch(id)
		return state, e
	}
	if s.Logs != nil && state.Artifacts.Log.Availability == "available" {
		state.Artifacts.Log, e = s.Logs.Status(id)
		if e != nil {
			return state, e
		}
	}
	if state.Artifacts.Recording.RecordingID != nil && s.recorder != nil {
		task, e := s.database.GetTaskByID(id)
		if e != nil {
			return state, e
		}
		if task != nil {
			info, e := s.recorder.TaskRecording(*state.Artifacts.Recording.RecordingID, task.DeviceSerial, id)
			if e != nil {
				if state.Artifacts.Recording.State != "failed" {
					state.Artifacts.Recording.State = "expired"
				}

				state.Artifacts.Recording.ErrorMsg = "录制已不可用"
			} else {
				state.Artifacts.Recording.State = info.State
				state.Artifacts.Recording.ErrorMsg = info.ErrorMsg
				complete := info.CoverageComplete && info.StartedAt <= task.StartedAt && (info.EndedAt == 0 || info.EndedAt >= task.EndedAt)
				state.Artifacts.Recording.CoverageComplete = &complete
				if state.Artifacts.Recording.State == "completed" || state.Artifacts.Recording.State == "failed" {
					_ = s.update(id, func(v *State) { v.Artifacts.Recording = state.Artifacts.Recording })
				}
			}
		}
	}
	report, e := s.database.Report(id, 0, false)
	if e != nil {
		return state, e
	}
	if report != nil {
		state.Artifacts.Report.State = report.State
		state.Artifacts.Report.GenerationRevision = report.Revision
		state.Artifacts.Report.ErrorMsg = report.ErrorMsg
		success, e := s.database.Report(id, 0, true)
		if e != nil {
			return state, e
		}
		if success != nil {
			state.Artifacts.Report.AvailableRevision = success.Revision
		}
		state.Artifacts.Report.CanRetry = report.State == "completed" || report.State == "failed"
	}
	if state.Options.Report.Enabled && state.Artifacts.Report.State == "failed" {
		task, _ := s.database.GetTaskByID(id)
		state.Artifacts.Report.CanRetry = task != nil && task.EndedAt > 0
	}
	return state, nil
}
func (s *Service) PrepareRecording(parent context.Context, id, serial string, timeoutSec int, w *LogCapture) {
	var state State
	if found, _ := s.database.ArtifactState(id, &state); !found || !state.Options.Recording.Enabled {
		return
	}
	_ = s.update(id, func(st *State) { st.Artifacts.Recording.State = "preparing" })
	if s.recorder == nil {
		s.recordingFailed(id, "录制服务不可用", w)
		return
	}
	duration := time.Duration(timeoutSec)*time.Second + 30*time.Second
	duration = max(time.Minute, min(8*time.Hour, duration))
	ctx, cancel := context.WithTimeout(parent, 5*time.Second)
	defer cancel()
	info, e := s.recorder.StartTaskRecording(ctx, serial, id, duration, state.Options.Recording.RecordAudio)
	if e != nil {
		s.recordingFailed(id, e.Error(), w)
		return
	}
	_ = s.update(id, func(st *State) {
		st.Artifacts.Recording.State = info.State
		st.Artifacts.Recording.RecordingID = &info.ID
	})
	w.Event("任务录制已开始")
}
func (s *Service) recordingFailed(id, msg string, w *LogCapture) {
	_ = s.update(id, func(st *State) { st.Artifacts.Recording.State = "failed"; st.Artifacts.Recording.ErrorMsg = msg })
	w.Event("录制失败: " + msg)
	log.Printf("[TaskArtifacts] task=%s recording failed: %s", id, msg)
}
func (s *Service) Started(id string) {
	_ = s.update(id, func(st *State) { st.PreparationState = "ready" })
}
func (s *Service) Finish(task *db.RunningTaskRecord, w *LogCapture) {
	state, e := s.State(task.ID)
	if e != nil {
		log.Printf("[TaskArtifacts] task=%s 读取收尾状态失败: %v", task.ID, e)
		w.Close()
		return
	}
	if state.Artifacts.Recording.RecordingID != nil && s.recorder != nil {
		info, e := s.recorder.StopTaskRecording(*state.Artifacts.Recording.RecordingID, task.DeviceSerial, task.ID)
		if e != nil {
			s.recordingFailed(task.ID, e.Error(), w)
		} else {
			_ = s.update(task.ID, func(st *State) { st.Artifacts.Recording.State = info.State })
			w.Event("已请求停止任务录制")
		}
	}
	w.Event(fmt.Sprintf("任务结束: %s，退出码 %d；%s", task.Status, task.ExitCode, task.ErrorMsg))
	w.Close()
	_ = s.update(task.ID, func(st *State) {
		st.PreparationState = "finished"
		if task.PID == 0 && task.Status == "failed" {
			st.PreparationState = "failed"
		}
	})
	if state.Options.Report.Enabled {
		if e := s.Enqueue(task.ID, false); e != nil {
			_ = s.update(task.ID, func(st *State) { st.Artifacts.Report.State = "failed"; st.Artifacts.Report.ErrorMsg = e.Error() })
			log.Printf("[TaskArtifacts] task=%s 入队失败: %v", task.ID, e)
		}
	}
}
func (s *Service) Rejected(task *db.RunningTaskRecord, dispatch string, options Options) error {
	w, e := s.Create(task.ID, dispatch, options)
	if e != nil {
		return e
	}
	w.Event("设备执行未启动: " + task.ErrorMsg)
	s.Finish(task, w)
	return nil
}
func (s *Service) ServeRecording(w http.ResponseWriter, r *http.Request, id string) error {
	task, e := s.database.GetTaskByID(id)
	if e != nil {
		return e
	}
	state, e := s.State(id)
	if e != nil {
		return e
	}
	if task == nil || state.Artifacts.Recording.RecordingID == nil || s.recorder == nil {
		return fmt.Errorf("录制不存在")
	}
	return s.recorder.ServeTaskRecording(w, r, *state.Artifacts.Recording.RecordingID, task.DeviceSerial, id)
}

// Recover is only called at application startup, never by individual request handlers.
func (s *Service) Recover() error {
	if e := s.database.RecoverInterruptedTasks(); e != nil {
		return e
	}
	if e := s.database.RecoverDispatches(); e != nil {
		return e
	}
	if e := s.database.RecoverReports(); e != nil {
		return e
	}
	ids, e := s.database.ArtifactTaskIDs()
	if e != nil {
		return e
	}
	for _, id := range ids {
		task, e := s.database.GetTaskByID(id)
		if e != nil {
			return e
		}
		if task == nil {
			continue
		}
		var st State
		s.database.ArtifactState(id, &st)
		if st.PreparationState == "preparing" || st.PreparationState == "ready" {
			_ = s.update(id, func(v *State) {
				v.PreparationState = "finished"
				v.Artifacts.Log.CaptureState = "degraded"
				v.Artifacts.Log.ErrorMsg = "服务重启，日志可能不完整"
				if v.Artifacts.Recording.State == "preparing" || v.Artifacts.Recording.State == "recording" || v.Artifacts.Recording.State == "stopping" {
					v.Artifacts.Recording.State = "failed"
					v.Artifacts.Recording.ErrorMsg = "服务重启，录制已中断"
				}
			})
			s.Logs.Recover(id)
			if st.Options.Report.Enabled {
				s.Enqueue(id, false)
			}
		}
	}
	return nil
}

func (s *Service) SaveProcessPath(id, path string) error {
	return s.update(id, func(st *State) { st.ProcessPath = path })
}
func (s *Service) FailedWithoutLog(id string, cause error) {
	_ = s.update(id, func(st *State) {
		st.PreparationState = "failed"
		st.Artifacts.Log.Availability = "unavailable"
		st.Artifacts.Log.CaptureState = "degraded"
		st.Artifacts.Log.ErrorMsg = cause.Error()
		if st.Options.Recording.Enabled {
			st.Artifacts.Recording.State = "not_started"
		}
	})
	var st State
	s.database.ArtifactState(id, &st)
	if st.Options.Report.Enabled {
		if e := s.Enqueue(id, false); e != nil {
			_ = s.update(id, func(st *State) { st.Artifacts.Report.State = "failed"; st.Artifacts.Report.ErrorMsg = e.Error() })
		}
	}
}
