package scheduler

import (
	"encoding/json"
	"net/http"
	"strconv"
)

type Handler struct {
	scheduler *Scheduler
}

func NewHandler(scheduler *Scheduler) *Handler {
	return &Handler{scheduler: scheduler}
}

func (h *Handler) RegisterRoutes(mux *http.ServeMux) {
	mux.HandleFunc("GET /api/scheduler/entries", h.handleGetEntries)
	mux.HandleFunc("POST /api/md-scripts/{id}/toggle-schedule", h.handleToggleSchedule)
}

func writeJSON(w http.ResponseWriter, status int, data interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(data)
}

func writeError(w http.ResponseWriter, status int, msg string) {
	writeJSON(w, status, map[string]interface{}{"error": msg, "message": msg})
}

func (h *Handler) handleGetEntries(w http.ResponseWriter, r *http.Request) {
	entries, err := h.scheduler.GetScheduleEntries()
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	if entries == nil {
		entries = []EntryInfo{}
	}
	writeJSON(w, http.StatusOK, map[string]interface{}{
		"entries": entries,
		"total":   len(entries),
	})
}

func (h *Handler) handleToggleSchedule(w http.ResponseWriter, r *http.Request) {
	idStr := r.PathValue("id")
	id, err := strconv.ParseInt(idStr, 10, 64)
	if err != nil {
		writeError(w, http.StatusBadRequest, "无效脚本 ID")
		return
	}

	var req struct {
		Enabled bool `json:"enabled"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "无效请求 JSON")
		return
	}

	enabledInt := 0
	if req.Enabled {
		enabledInt = 1
	}

	if err := h.scheduler.database.ToggleScriptEnabled(id, enabledInt); err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}

	// 触发热重载
	if err := h.scheduler.ReloadScript(id); err != nil {
		writeError(w, http.StatusInternalServerError, "热重载调度项失败: "+err.Error())
		return
	}

	writeJSON(w, http.StatusOK, map[string]interface{}{
		"ok":         true,
		"script_id":  id,
		"is_enabled": req.Enabled,
	})
}
