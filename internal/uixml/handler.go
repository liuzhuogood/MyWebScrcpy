package uixml

import (
	"encoding/json"
	"errors"
	"net/http"
	"time"
)

type Handler struct {
	Service  *Service
	IsOnline func(string) bool
}

func (h Handler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		w.Header().Set("Allow", http.MethodGet)
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	serial := r.URL.Query().Get("serial")
	if serial == "" {
		writeError(w, http.StatusBadRequest, "missing_serial", ErrMissingSerial.Error())
		return
	}
	if h.IsOnline != nil && !h.IsOnline(serial) {
		writeError(w, http.StatusBadGateway, "device_unavailable", "device is not online")
		return
	}
	snapshot, err := h.Service.FetchSnapshot(r.Context(), serial)
	if err != nil {
		status, code := http.StatusBadGateway, "ui_xml_failed"
		switch {
		case errors.Is(err, ErrMissingSerial):
			status, code = http.StatusBadRequest, "missing_serial"
		case errors.Is(err, ErrTooLarge):
			status, code = http.StatusUnprocessableEntity, "ui_xml_too_large"
		case errors.Is(err, ErrInvalidXML):
			status, code = http.StatusUnprocessableEntity, "invalid_ui_xml"
		}
		writeError(w, status, code, err.Error())
		return
	}
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(struct {
		Serial      string       `json:"serial"`
		CapturedAt  time.Time    `json:"captured_at"`
		XML         string       `json:"xml"`
		DisplaySize *DisplaySize `json:"display_size,omitempty"`
	}{serial, snapshot.CapturedAt, snapshot.XML, snapshot.DisplaySize})
}

func writeError(w http.ResponseWriter, status int, code, message string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(map[string]string{"error": code, "message": message})
}
