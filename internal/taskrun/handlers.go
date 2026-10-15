package taskrun

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"time"

	"mywebscrcpy/internal/db"
)

type Handler struct {
	service *Service
}

func NewHandler(service *Service) *Handler {
	return &Handler{service: service}
}

func (h *Handler) RegisterRoutes(mux *http.ServeMux) {
	// 外部执行端直接上报运行、心跳、日志和结果
	mux.HandleFunc("POST /api/task-runs", h.handleCreate)
	mux.HandleFunc("POST /api/task-runs/{id}/heartbeat", h.handleHeartbeat)
	mux.HandleFunc("POST /api/task-runs/{id}/logs", h.handleAppendLogs)
	mux.HandleFunc("POST /api/task-runs/{id}/finish", h.handleFinish)

	// 读接口：常规查询与详情
	mux.HandleFunc("GET /api/task-runs", h.handleList)
	mux.HandleFunc("GET /api/task-runs/{id}", h.handleGet)
	mux.HandleFunc("GET /api/task-runs/{id}/logs", h.handleGetLogs)

}

func decodeStrictJSON(w http.ResponseWriter, r *http.Request, maxBytes int64, v any) error {
	dec := json.NewDecoder(http.MaxBytesReader(w, r.Body, maxBytes))
	dec.DisallowUnknownFields()
	if err := dec.Decode(v); err != nil {
		return err
	}
	var trailing json.RawMessage
	if err := dec.Decode(&trailing); err != io.EOF {
		return errors.New("请求包含尾随的多余 JSON 数据")
	}
	return nil
}

func writeJSON(w http.ResponseWriter, status int, data any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(data)
}

func writeError(w http.ResponseWriter, status int, msg string) {
	writeJSON(w, status, map[string]string{"error": msg})
}

func (h *Handler) handleCreate(w http.ResponseWriter, r *http.Request) {

	var req struct {
		ClientRunID    string `json:"client_run_id"`
		Name           string `json:"name"`
		Source         string `json:"source"`
		DeviceSerial   string `json:"device_serial"`
		Summary        string `json:"summary"`
		ReportMarkdown string `json:"report_markdown"`
	}

	if err := decodeStrictJSON(w, r, 1<<20, &req); err != nil {
		writeError(w, http.StatusBadRequest, fmt.Sprintf("请求格式不合法: %v", err))
		return
	}

	if req.ClientRunID == "" || req.Name == "" || req.DeviceSerial == "" {
		writeError(w, http.StatusBadRequest, "client_run_id, name, device_serial 均为必填字段")
		return
	}

	rec := &db.TaskRunRecord{
		ClientRunID:    req.ClientRunID,
		Name:           req.Name,
		Source:         req.Source,
		DeviceSerial:   req.DeviceSerial,
		Summary:        req.Summary,
		ReportMarkdown: req.ReportMarkdown,
	}

	createdRec, isNew, err := h.service.database.CreateTaskRun(rec)
	if err != nil {
		if errors.Is(err, db.ErrConflict) {
			writeError(w, http.StatusConflict, err.Error())
			return
		}
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}

	view := h.service.DeriveView(createdRec, time.Now().UnixMilli())
	status := http.StatusCreated
	if !isNew {
		status = http.StatusOK
	}
	writeJSON(w, status, view)
}

func (h *Handler) handleHeartbeat(w http.ResponseWriter, r *http.Request) {

	id := r.PathValue("id")
	if id == "" {
		writeError(w, http.StatusBadRequest, "缺少运行 ID")
		return
	}

	rec, err := h.service.database.HeartbeatTaskRun(id, time.Now().UnixMilli())
	if err != nil {
		if errors.Is(err, db.ErrNotFound) {
			writeError(w, http.StatusNotFound, "运行不存在")
			return
		}
		if errors.Is(err, db.ErrConflict) {
			writeError(w, http.StatusConflict, err.Error())
			return
		}
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}

	view := h.service.DeriveView(rec, time.Now().UnixMilli())
	writeJSON(w, http.StatusOK, view)
}

func (h *Handler) handleAppendLogs(w http.ResponseWriter, r *http.Request) {

	id := r.PathValue("id")
	if id == "" {
		writeError(w, http.StatusBadRequest, "缺少运行 ID")
		return
	}

	var req struct {
		Entries []db.TaskRunLogEntry `json:"entries"`
	}

	if err := decodeStrictJSON(w, r, 4<<20, &req); err != nil {
		writeError(w, http.StatusBadRequest, fmt.Sprintf("请求格式不合法: %v", err))
		return
	}

	if len(req.Entries) == 0 {
		writeError(w, 400, "entries 不能为空")
		return
	}
	for _, entry := range req.Entries {
		if entry.Stream != "stdout" && entry.Stream != "stderr" {
			writeError(w, http.StatusBadRequest, fmt.Sprintf("未知的日志流类型: %s (仅支持 stdout 或 stderr)", entry.Stream))
			return
		}
		if entry.Timestamp <= 0 {
			writeError(w, 400, "timestamp 必须为正数毫秒时间戳")
			return
		}
		if entry.Seq <= 0 {
			writeError(w, http.StatusBadRequest, "seq 必须大于 0")
			return
		}
	}

	ackSeq, truncated, err := h.service.database.AppendTaskRunLogs(id, req.Entries, MaxLogsPerRun)
	if err != nil {
		if errors.Is(err, db.ErrNotFound) {
			writeError(w, http.StatusNotFound, "运行不存在")
			return
		}
		if errors.Is(err, db.ErrConflict) {
			writeError(w, http.StatusConflict, err.Error())
			return
		}
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"id":               id,
		"acknowledged_seq": ackSeq,
		"truncated":        truncated,
	})
}

func (h *Handler) handleFinish(w http.ResponseWriter, r *http.Request) {

	id := r.PathValue("id")
	if id == "" {
		writeError(w, http.StatusBadRequest, "缺少运行 ID")
		return
	}

	var req struct {
		Status         string `json:"status"`
		ExitCode       *int   `json:"exit_code"`
		Summary        string `json:"summary"`
		ReportMarkdown string `json:"report_markdown"`
	}

	if err := decodeStrictJSON(w, r, 1<<20, &req); err != nil {
		writeError(w, http.StatusBadRequest, fmt.Sprintf("请求格式不合法: %v", err))
		return
	}

	if req.Status != "succeeded" && req.Status != "failed" && req.Status != "cancelled" {
		writeError(w, http.StatusBadRequest, "终态 status 只能为 succeeded, failed 或 cancelled")
		return
	}

	rec, err := h.service.database.FinishTaskRun(id, req.Status, req.ExitCode, req.Summary, req.ReportMarkdown, time.Now().UnixMilli())
	if err != nil {
		if errors.Is(err, db.ErrNotFound) {
			writeError(w, http.StatusNotFound, "运行不存在")
			return
		}
		if errors.Is(err, db.ErrConflict) {
			writeError(w, http.StatusConflict, err.Error())
			return
		}
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}

	view := h.service.DeriveView(rec, time.Now().UnixMilli())
	writeJSON(w, http.StatusOK, view)
}

func (h *Handler) handleList(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query()
	page, _ := strconv.Atoi(q.Get("page"))
	if page < 1 {
		page = 1
	}
	limit, _ := strconv.Atoi(q.Get("limit"))
	if limit < 1 || limit > 100 {
		limit = 20
	}

	filter := db.TaskRunFilter{
		Page:            page,
		Limit:           limit,
		Status:          q.Get("status"),
		Connectivity:    q.Get("connectivity"),
		HeartbeatCutoff: time.Now().UnixMilli() - HeartbeatTimeoutMs,
		DeviceSerial:    q.Get("device_serial"),
		Source:          q.Get("source"),
		Search:          q.Get("q"),
	}

	if filter.Connectivity != "" && filter.Connectivity != "connected" && filter.Connectivity != "disconnected" {
		writeError(w, 400, "无效连接状态")
		return
	}
	if filter.Status != "" && filter.Status != "running" && filter.Status != "succeeded" && filter.Status != "failed" && filter.Status != "cancelled" {
		writeError(w, 400, "无效任务状态")
		return
	}
	items, total, err := h.service.database.ListTaskRuns(filter)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}

	now := time.Now().UnixMilli()
	views := make([]TaskRunView, len(items))
	for i := range items {
		views[i] = h.service.DeriveView(&items[i], now)
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"items": views,
		"total": total,
		"page":  page,
		"limit": limit,
	})
}

func (h *Handler) handleGet(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	if id == "" {
		writeError(w, http.StatusBadRequest, "缺少运行 ID")
		return
	}

	rec, err := h.service.database.GetTaskRunByID(id)
	if err != nil {
		if errors.Is(err, db.ErrNotFound) {
			writeError(w, http.StatusNotFound, "运行不存在")
			return
		}
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}

	view := h.service.DeriveView(rec, time.Now().UnixMilli())
	writeJSON(w, http.StatusOK, view)
}

func (h *Handler) handleGetLogs(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	if id == "" {
		writeError(w, http.StatusBadRequest, "缺少运行 ID")
		return
	}

	q := r.URL.Query()
	if q.Get("download") == "true" {
		entries, err := h.service.database.GetAllTaskRunLogs(id)
		if err != nil {
			if errors.Is(err, db.ErrNotFound) {
				writeError(w, http.StatusNotFound, "运行不存在")
				return
			}
			writeError(w, http.StatusInternalServerError, err.Error())
			return
		}

		w.Header().Set("Content-Type", "text/plain; charset=utf-8")
		w.Header().Set("Content-Disposition", fmt.Sprintf(`attachment; filename="task-run-%s.log"`, id))
		w.WriteHeader(http.StatusOK)
		for _, e := range entries {
			tStr := time.UnixMilli(e.Timestamp).Format("2006-01-02 15:04:05.000")
			fmt.Fprintf(w, "[%s] [%s] %s\n", tStr, e.Stream, e.Message)
		}
		return
	}

	after, _ := strconv.ParseInt(q.Get("after"), 10, 64)
	limit, _ := strconv.Atoi(q.Get("limit"))
	stream := q.Get("stream")

	entries, lastSeq, truncated, err := h.service.database.GetTaskRunLogs(id, after, limit, stream)
	if err != nil {
		if errors.Is(err, db.ErrNotFound) {
			writeError(w, http.StatusNotFound, "运行不存在")
			return
		}
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"items":            entries,
		"acknowledged_seq": lastSeq,
		"truncated":        truncated,
	})
}
