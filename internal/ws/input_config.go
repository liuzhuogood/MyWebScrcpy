package ws

import (
	"encoding/json"
	"net/http"

	"mywebscrcpy/internal/action"
	debuglog "mywebscrcpy/internal/debug"
)

// input_config.go — 系统设置的默认点击/滑动注入模式接口。
//
// GET /api/input-config  读取当前默认注入模式
// PUT /api/input-config  修改默认注入模式（即时生效，影响之后所有
//                        未显式指定 mode 的动作；显式 mode 仍优先）

type inputConfigView struct {
	Mode string `json:"mode"`
}

func (h *Hub) RegisterInputConfigRoutes(mux *http.ServeMux) {
	mux.HandleFunc("GET /api/input-config", h.getInputConfig)
	mux.HandleFunc("PUT /api/input-config", h.putInputConfig)
}

func (h *Hub) getInputConfig(w http.ResponseWriter, _ *http.Request) {
	mode := action.ModeSDK
	if h.inputConfig != nil {
		mode = h.inputConfig.Mode()
	}
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(inputConfigView{Mode: mode})
}

func (h *Hub) putInputConfig(w http.ResponseWriter, r *http.Request) {
	if h.inputConfig == nil {
		writeADBError(w, http.StatusServiceUnavailable, "config_unavailable", "input config store unavailable")
		return
	}
	var body inputConfigView
	decoder := json.NewDecoder(http.MaxBytesReader(w, r.Body, 4096))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&body); err != nil {
		writeADBError(w, http.StatusBadRequest, "invalid_request", "mode is required")
		return
	}
	if !action.ValidMode(body.Mode) || body.Mode == "" {
		writeADBError(w, http.StatusBadRequest, "invalid_request", "mode must be one of sdk, adb, hid")
		return
	}
	if err := h.inputConfig.Set(body.Mode); err != nil {
		writeADBError(w, http.StatusInternalServerError, "storage_failed", err.Error())
		return
	}
	h.recordEvent(debuglog.Event{Type: "input_config.updated", Message: body.Mode})
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(inputConfigView{Mode: h.inputConfig.Mode()})
}
