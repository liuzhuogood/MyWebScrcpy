package uipage

import (
	"encoding/json"
	"errors"
	"net/http"
)

// Handler handles HTTP requests for UI page info.
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
	if h.Service == nil {
		writeError(w, http.StatusBadGateway, "service_unavailable", "page service is uninitialized")
		return
	}
	info, err := h.Service.FetchPageInfo(r.Context(), serial)
	if err != nil {
		status, code := http.StatusBadGateway, "page_info_failed"
		switch {
		case errors.Is(err, ErrMissingSerial):
			status, code = http.StatusBadRequest, "missing_serial"
		case errors.Is(err, ErrPageInfoNotFound):
			status, code = http.StatusBadGateway, "page_info_not_found"
		}
		writeError(w, status, code, err.Error())
		return
	}
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(info)
}

func writeError(w http.ResponseWriter, status int, code, message string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(map[string]string{"error": code, "message": message})
}
