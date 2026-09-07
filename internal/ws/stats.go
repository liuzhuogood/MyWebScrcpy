package ws

import (
	"encoding/json"
	"net/http"
)

func (h *Hub) ServeVisionStats(w http.ResponseWriter, r *http.Request) {
	serial := r.URL.Query().Get("serial")
	if serial == "" {
		http.Error(w, "missing serial", http.StatusBadRequest)
		return
	}
	s, ok := h.stats(serial)
	if !ok {
		http.Error(w, "session_not_found", http.StatusNotFound)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(s)
}
