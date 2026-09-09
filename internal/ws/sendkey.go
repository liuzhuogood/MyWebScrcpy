package ws

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"

	"mywebscrcpy/internal/action"
)

const (
	metaShift uint32 = 0x1
	metaAlt   uint32 = 0x2
	metaCtrl  uint32 = 0x1000
	metaMeta  uint32 = 0x10000
)

func modifiersToMeta(modifiers []string) (uint32, error) {
	var result uint32
	for _, modifier := range modifiers {
		var bit uint32
		switch strings.ToLower(modifier) {
		case "shift":
			bit = metaShift
		case "alt":
			bit = metaAlt
		case "ctrl":
			bit = metaCtrl
		case "meta":
			bit = metaMeta
		default:
			return 0, fmt.Errorf("unsupported modifier %q", modifier)
		}
		if result&bit != 0 {
			return 0, fmt.Errorf("duplicate modifier %q", modifier)
		}
		result |= bit
	}
	return result, nil
}

func (h *Hub) RegisterSendKeyRoute(mux *http.ServeMux) {
	mux.HandleFunc("POST /api/sendkey", h.sendKey)
}
func (h *Hub) sendKey(w http.ResponseWriter, r *http.Request) {
	serial := r.URL.Query().Get("serial")
	if serial == "" {
		writeADBError(w, http.StatusBadRequest, "invalid_request", "missing serial")
		return
	}
	var body struct {
		Keycode   uint32   `json:"keycode"`
		Modifiers []string `json:"modifiers,omitempty"`
	}
	decoder := json.NewDecoder(http.MaxBytesReader(w, r.Body, 4096))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&body); err != nil || body.Keycode == 0 {
		writeADBError(w, http.StatusBadRequest, "invalid_request", "keycode is required")
		return
	}
	meta, err := modifiersToMeta(body.Modifiers)
	if err != nil {
		writeADBError(w, http.StatusBadRequest, "invalid_request", err.Error())
		return
	}
	ms, _, release, err := h.acquireSession(serial)
	if err != nil {
		writeADBError(w, http.StatusServiceUnavailable, "device_unavailable", "device session unavailable")
		return
	}
	defer release()
	result := <-ms.actions.Submit(context.Background(), action.Request{DeviceID: serial, Source: "http", Action: "key", Keycode: body.Keycode, MetaState: meta, ExpiresMS: 3000})
	w.Header().Set("Content-Type", "application/json")
	if !result.Executed {
		w.WriteHeader(http.StatusBadGateway)
	}
	_ = json.NewEncoder(w).Encode(result)
}
