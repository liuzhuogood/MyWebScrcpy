package ws

import (
	"encoding/json"
	"errors"
	"net/http"
	"strings"

	"mywebscrcpy/internal/adbcommand"
)

func (h *Hub) RegisterADBCommandRoutes(mux *http.ServeMux) {
	mux.HandleFunc("POST /api/adb/commands", h.submitADBCommand)
	mux.HandleFunc("GET /api/adb/commands/{command_id}", h.getADBCommand)
}

func (h *Hub) submitADBCommand(w http.ResponseWriter, r *http.Request) {
	var req adbcommand.Request
	decoder := json.NewDecoder(http.MaxBytesReader(w, r.Body, 32<<10))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&req); err != nil {
		writeADBError(w, http.StatusBadRequest, "invalid_request", err.Error())
		return
	}
	job, err := h.commands.Submit(r.URL.Query().Get("serial"), req)
	if err != nil {
		writeADBError(w, http.StatusBadRequest, "invalid_request", err.Error())
		return
	}
	v := job.Snapshot()
	w.Header().Set("Content-Type", "application/json")
	if req.Async {
		w.WriteHeader(http.StatusAccepted)
	} else if v.Status == adbcommand.TimedOut {
		w.WriteHeader(http.StatusGatewayTimeout)
	} else if v.Status == adbcommand.Failed && isADBUnavailable(v.Error) {
		w.WriteHeader(http.StatusServiceUnavailable)
	}
	_ = json.NewEncoder(w).Encode(v)
}
func (h *Hub) getADBCommand(w http.ResponseWriter, r *http.Request) {
	job, err := h.commands.Get(r.PathValue("command_id"), r.URL.Query().Get("serial"))
	if err != nil {
		status := http.StatusBadRequest
		if errors.Is(err, adbcommand.ErrNotFound) {
			status = http.StatusNotFound
		}
		writeADBError(w, status, "command_not_found", err.Error())
		return
	}
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(job.Snapshot())
}

func isADBUnavailable(text string) bool {
	return strings.Contains(text, "device offline") || strings.Contains(text, "device not found") || strings.Contains(text, "no devices")
}
func writeADBError(w http.ResponseWriter, status int, code, message string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(map[string]string{"error": code, "message": message})
}
