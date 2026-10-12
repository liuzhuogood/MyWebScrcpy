package taskmanager

import (
	"encoding/json"
	"fmt"
	"mywebscrcpy/internal/db"
	"mywebscrcpy/internal/taskartifact"
	"net/http"
	"strconv"
)

type taskView struct {
	db.RunningTaskRecord
	DispatchID       string `json:"dispatch_id"`
	PreparationState string `json:"preparation_state"`
}

func (h *Handler) registerArtifacts(mux *http.ServeMux) {
	mux.HandleFunc("GET /api/tasks/{id}", h.handleDetail)
	mux.HandleFunc("GET /api/tasks/{id}/logs", h.handleLogs)
	mux.HandleFunc("GET /api/tasks/{id}/logs/download", h.handleLogDownload)
	mux.HandleFunc("GET /api/tasks/{id}/recording", h.handleRecording)
	mux.HandleFunc("GET /api/tasks/{id}/recording/media", h.handleMedia)
	mux.HandleFunc("GET /api/tasks/{id}/report", h.handleReport)
	mux.HandleFunc("GET /api/tasks/{id}/report/download", h.handleReportDownload)
	mux.HandleFunc("POST /api/tasks/{id}/report/retry", h.handleRetryReport)
	mux.HandleFunc("GET /api/md-scripts/{script_id}/dispatches", h.handleDispatchList)
	mux.HandleFunc("GET /api/md-scripts/{script_id}/dispatches/{dispatch_id}", h.handleDispatch)
}
func (h *Handler) task(w http.ResponseWriter, r *http.Request) (*db.RunningTaskRecord, taskartifact.State, bool) {
	task, e := h.manager.database.GetTaskByID(r.PathValue("id"))
	if e != nil {
		writeError(w, 500, e.Error())
		return nil, taskartifact.State{}, false
	}
	if task == nil {
		writeError(w, 404, "任务不存在")
		return nil, taskartifact.State{}, false
	}
	state, e := h.manager.Artifacts.State(task.ID)
	if e != nil {
		writeError(w, 500, e.Error())
		return nil, state, false
	}
	return task, state, true
}
func (h *Handler) handleDetail(w http.ResponseWriter, r *http.Request) {
	task, state, ok := h.task(w, r)
	if !ok {
		return
	}
	writeJSON(w, 200, map[string]any{"task": taskView{*task, state.DispatchID, state.PreparationState}, "artifacts": state.Artifacts})
}
func (h *Handler) handleLogs(w http.ResponseWriter, r *http.Request) {
	task, _, ok := h.task(w, r)
	if !ok {
		return
	}
	q := r.URL.Query()
	query := taskartifact.LogQuery{Stream: q.Get("stream"), Q: q.Get("q")}
	var e error
	if q.Has("after") {
		query.HasAfter = true
		query.After, e = strconv.ParseInt(q.Get("after"), 10, 64)
		if e != nil || query.After < 0 {
			writeError(w, 400, "无效 after")
			return
		}
	}
	if q.Has("before") {
		query.HasBefore = true
		query.Before, e = strconv.ParseInt(q.Get("before"), 10, 64)
		if e != nil || query.Before < 1 {
			writeError(w, 400, "无效 before")
			return
		}
	}
	if q.Has("limit") {
		query.Limit, e = strconv.Atoi(q.Get("limit"))
		if e != nil || query.Limit < 1 || query.Limit > 500 {
			writeError(w, 400, "无效 limit")
			return
		}
	}
	if h.manager.Artifacts.Logs == nil {
		writeJSON(w, 200, taskartifact.LogPage{Items: []taskartifact.LogLine{}, Availability: "unavailable", Complete: true})
		return
	}
	page, e := h.manager.Artifacts.Logs.Read(task.ID, query)
	if e != nil {
		writeError(w, 400, e.Error())
		return
	}
	writeJSON(w, 200, page)
}
func (h *Handler) handleLogDownload(w http.ResponseWriter, r *http.Request) {
	task, state, ok := h.task(w, r)
	if !ok {
		return
	}
	if state.Artifacts.Log.Availability != "available" {
		writeError(w, 410, "日志不可用或已过期")
		return
	}
	file, e := h.manager.Artifacts.Logs.Open(task.ID)
	if e != nil {
		writeError(w, 500, e.Error())
		return
	}
	defer file.Close()
	info, e := file.Stat()
	if e != nil {
		writeError(w, 500, e.Error())
		return
	}
	w.Header().Set("Content-Type", "application/x-ndjson; charset=utf-8")
	w.Header().Set("Content-Disposition", fmt.Sprintf("attachment; filename=%q", task.ID+".jsonl"))
	http.ServeContent(w, r, task.ID+".jsonl", info.ModTime(), file)
}
func (h *Handler) handleRecording(w http.ResponseWriter, r *http.Request) {
	_, state, ok := h.task(w, r)
	if !ok {
		return
	}
	writeJSON(w, 200, state.Artifacts.Recording)
}
func (h *Handler) handleMedia(w http.ResponseWriter, r *http.Request) {
	_, _, ok := h.task(w, r)
	if !ok {
		return
	}
	if e := h.manager.Artifacts.ServeRecording(w, r, r.PathValue("id")); e != nil {
		writeError(w, 404, e.Error())
	}
}
func (h *Handler) reportPayload(id string, revision int) (map[string]any, error) {
	state, e := h.manager.Artifacts.State(id)
	if e != nil {
		return nil, e
	}
	report, e := h.manager.database.Report(id, revision, true)
	if e != nil {
		return nil, e
	}
	var out *taskartifact.ReportOutput
	var ev *taskartifact.Evidence
	var created int64
	var hash string
	if report != nil {
		out = &taskartifact.ReportOutput{}
		if e = json.Unmarshal([]byte(report.Output), out); e != nil {
			return nil, e
		}
		ev = &taskartifact.Evidence{}
		if e = json.Unmarshal([]byte(report.Evidence), ev); e != nil {
			return nil, e
		}
		created = report.CreatedAt
		hash = report.InputHash
	}
	return map[string]any{"generation": state.Artifacts.Report, "report": out, "revision": state.Artifacts.Report.AvailableRevision, "evidence": ev, "input_hash": hash, "created_at": created, "log_availability": state.Artifacts.Log.Availability}, nil
}
func (h *Handler) handleReport(w http.ResponseWriter, r *http.Request) {
	task, _, ok := h.task(w, r)
	if !ok {
		return
	}
	revision := 0
	var e error
	if r.URL.Query().Has("revision") {
		revision, e = strconv.Atoi(r.URL.Query().Get("revision"))
		if e != nil || revision < 1 {
			writeError(w, 400, "无效报告修订")
			return
		}
	}
	payload, e := h.reportPayload(task.ID, revision)
	if e != nil {
		writeError(w, 500, e.Error())
		return
	}
	if revision > 0 && payload["report"] == (*taskartifact.ReportOutput)(nil) {
		writeError(w, 404, "报告修订不存在")
		return
	}
	if revision > 0 {
		payload["revision"] = revision
	}
	writeJSON(w, 200, payload)
}
func (h *Handler) handleReportDownload(w http.ResponseWriter, r *http.Request) {
	task, _, ok := h.task(w, r)
	if !ok {
		return
	}
	revision := 0
	var e error
	if r.URL.Query().Has("revision") {
		revision, e = strconv.Atoi(r.URL.Query().Get("revision"))
		if e != nil || revision < 1 {
			writeError(w, 400, "无效报告修订")
			return
		}
	}
	report, e := h.manager.database.Report(task.ID, revision, true)
	if e != nil {
		writeError(w, 500, e.Error())
		return
	}
	if report == nil {
		writeError(w, 404, "报告尚未生成")
		return
	}
	var out taskartifact.ReportOutput
	if e = json.Unmarshal([]byte(report.Output), &out); e != nil {
		writeError(w, 500, "报告读取失败")
		return
	}
	w.Header().Set("Content-Type", "text/markdown; charset=utf-8")
	w.Header().Set("Content-Disposition", fmt.Sprintf("attachment; filename=%q", fmt.Sprintf("%s-report-%d.md", task.ID, report.Revision)))
	w.Write([]byte(out.Markdown))
}
func (h *Handler) handleRetryReport(w http.ResponseWriter, r *http.Request) {
	task, state, ok := h.task(w, r)
	if !ok {
		return
	}
	if !state.Artifacts.Report.CanRetry {
		writeError(w, 409, "本次报告不能重新生成")
		return
	}
	if e := h.manager.Artifacts.Enqueue(task.ID, true); e != nil {
		writeError(w, 409, e.Error())
		return
	}
	writeJSON(w, 202, map[string]any{"ok": true})
}
func (h *Handler) scriptID(w http.ResponseWriter, r *http.Request) (int64, bool) {
	id, e := strconv.ParseInt(r.PathValue("script_id"), 10, 64)
	if e != nil || id < 1 {
		writeError(w, 400, "无效脚本 ID")
		return 0, false
	}
	script, e := h.manager.database.GetMarkdownScript(id)
	if e != nil {
		writeError(w, 500, e.Error())
		return 0, false
	}
	if script == nil {
		writeError(w, 404, "脚本不存在")
		return 0, false
	}
	return id, true
}
func (h *Handler) handleDispatchList(w http.ResponseWriter, r *http.Request) {
	id, ok := h.scriptID(w, r)
	if !ok {
		return
	}
	cursor := int64(0)
	limit := 20
	var e error
	if r.URL.Query().Has("cursor") {
		cursor, e = strconv.ParseInt(r.URL.Query().Get("cursor"), 10, 64)
		if e != nil || cursor < 1 {
			writeError(w, 400, "无效批次游标")
			return
		}
	}
	if r.URL.Query().Has("limit") {
		limit, e = strconv.Atoi(r.URL.Query().Get("limit"))
		if e != nil || limit < 1 || limit > 100 {
			writeError(w, 400, "无效 limit")
			return
		}
	}
	list, e := h.manager.database.ListDispatches(id, cursor, limit+1)
	if e != nil {
		writeError(w, 500, e.Error())
		return
	}
	more := len(list) > limit
	if more {
		list = list[:limit]
	}
	next := ""
	for i := range list {
		tasks, e := h.manager.database.DispatchTasks(id, list[i].ID)
		if e != nil {
			writeError(w, 500, e.Error())
			return
		}
		list[i].Status = dispatchStatus(list[i], tasks)
	}
	if len(list) > 0 {
		next = list[len(list)-1].Cursor
	}
	writeJSON(w, 200, map[string]any{"dispatches": list, "next_cursor": next, "has_more": more})
}
func (h *Handler) handleDispatch(w http.ResponseWriter, r *http.Request) {
	id, ok := h.scriptID(w, r)
	if !ok {
		return
	}
	dispatch, e := h.manager.database.GetDispatch(id, r.PathValue("dispatch_id"))
	if e != nil {
		writeError(w, 500, e.Error())
		return
	}
	if dispatch == nil {
		writeError(w, 404, "执行批次不存在")
		return
	}
	tasks, e := h.manager.database.DispatchTasks(id, dispatch.ID)
	if e != nil {
		writeError(w, 500, e.Error())
		return
	}
	views := []taskView{}
	for _, task := range tasks {
		state, e := h.manager.Artifacts.State(task.ID)
		if e != nil {
			writeError(w, 500, e.Error())
			return
		}
		views = append(views, taskView{task, state.DispatchID, state.PreparationState})
	}
	dispatch.Status = dispatchStatus(*dispatch, tasks)
	writeJSON(w, 200, map[string]any{"dispatch": dispatch, "tasks": views})
}
func dispatchStatus(d db.DispatchRecord, tasks []db.RunningTaskRecord) string {
	if d.ErrorMsg != "" {
		return "failed"
	}
	if d.DispatchState == "dispatching" {
		return "preparing"
	}
	if len(tasks) == 0 {
		return "unavailable"
	}
	counts := map[string]int{}
	for _, t := range tasks {
		counts[t.Status]++
	}
	if counts["running"]+counts["preparing"]+counts["stopping"] > 0 {
		return "running"
	}
	if len(counts) > 1 {
		return "partial"
	}
	return tasks[0].Status
}
