package taskmanager

import (
	"encoding/json"
	"net/http"

	"mywebscrcpy/internal/db"
)

type Handler struct {
	manager *Manager
}

func NewHandler(manager *Manager) *Handler {
	return &Handler{manager: manager}
}

func (h *Handler) RegisterRoutes(mux *http.ServeMux) {
	h.registerArtifacts(mux)
	mux.HandleFunc("GET /api/tasks/active", h.handleListActive)
	mux.HandleFunc("POST /api/tasks/{id}/kill", h.handleKill)
}

func writeJSON(w http.ResponseWriter, status int, data interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(data)
}

func writeError(w http.ResponseWriter, status int, msg string) {
	writeJSON(w, status, map[string]interface{}{"error": msg, "message": msg})
}

func (h *Handler) handleListActive(w http.ResponseWriter, r *http.Request) {
	tasks, err := h.manager.ListActiveTasks()
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	if tasks == nil {
		tasks = []db.RunningTaskRecord{}
	}
	writeJSON(w, http.StatusOK, map[string]interface{}{"tasks": tasks, "total": len(tasks)})
}

func (h *Handler) handleKill(w http.ResponseWriter, r *http.Request) {
	taskID := r.PathValue("id")
	if taskID == "" {
		writeError(w, http.StatusBadRequest, "缺少任务 ID")
		return
	}

	if err := h.manager.KillTask(taskID); err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]interface{}{"ok": true, "message": "任务进程已终止，设备锁已释放"})
}
