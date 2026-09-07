package ws

import (
	"encoding/json"
	"fmt"
	"net/http"

	debuglog "mywebscrcpy/internal/debug"
)

// ServeVisionDebug returns a bounded, read-only diagnostic event snapshot.
func (h *Hub) ServeVisionDebug(w http.ResponseWriter, r *http.Request) {
	serial := r.URL.Query().Get("serial")
	if serial == "" {
		http.Error(w, "missing serial", http.StatusBadRequest)
		return
	}
	limit := 100
	if raw := r.URL.Query().Get("limit"); raw != "" {
		if _, err := fmt.Sscanf(raw, "%d", &limit); err != nil {
			http.Error(w, "invalid_limit", http.StatusBadRequest)
			return
		}
	}
	if limit < 1 {
		limit = 1
	}
	if limit > 500 {
		limit = 500
	}
	if h.events == nil {
		h.events = debuglog.New(512)
	}
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]interface{}{
		"device_id": serial,
		"events":    h.events.Snapshot(serial, limit),
	})
}
