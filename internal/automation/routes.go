package automation

import (
	"encoding/json"
	"net/http"
	"strings"
)

// RegisterRoutes 在 mux 上注册所有自动化与视觉接口
func (s *Service) RegisterRoutes(mux *http.ServeMux) {
	// 动作控制路由
	mux.HandleFunc("POST /api/v1/actions/tap", s.handleTap)
	mux.HandleFunc("POST /api/v1/actions/swipe", s.handleSwipe)
	mux.HandleFunc("POST /api/v1/actions/events", s.handleEvent)
	mux.HandleFunc("POST /api/v1/actions/send-key", s.handleSendKey)
	mux.HandleFunc("GET /api/v1/device-status", s.handleDeviceStatus)

	// 视觉判断与条件点击路由
	mux.HandleFunc("POST /api/v1/vision/check", s.handleCheck)
	mux.HandleFunc("POST /api/v1/vision/wait", s.handleWait)
	mux.HandleFunc("POST /api/v1/vision/actions/tap-when", s.handleTapWhen)
	mux.HandleFunc("POST /api/v1/vision/region-screenshot", s.handleRegionScreenshot)

	// 兼容 yolo_wepork / opencv_match 历史前缀的别名
	mux.HandleFunc("POST /api/v1/vision/actions/tap", s.handleTap)
	mux.HandleFunc("POST /api/v1/vision/actions/swipe", s.handleSwipe)
	mux.HandleFunc("POST /api/v1/vision/actions/events", s.handleEvent)
	mux.HandleFunc("POST /api/v1/vision/actions/send-key", s.handleSendKey)
	mux.HandleFunc("GET /api/v1/vision/device-status", s.handleDeviceStatus)
}

func writeJSON(w http.ResponseWriter, status int, data any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(data)
}

func writeError(w http.ResponseWriter, status int, code, msg string) {
	writeJSON(w, status, map[string]any{
		"error":   msg,
		"code":    code,
		"message": msg,
		"detail": map[string]any{
			"code":    code,
			"message": msg,
		},
	})
}

func (s *Service) handleTap(w http.ResponseWriter, r *http.Request) {
	var req TapRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid_json", "请求体 JSON 格式错误: "+err.Error())
		return
	}
	if req.Serial == "" {
		writeError(w, http.StatusBadRequest, "missing_serial", "缺少 serial 参数")
		return
	}

	res, err := s.Tap(r.Context(), req)
	if err != nil {
		writeError(w, http.StatusBadGateway, "execution_failed", err.Error())
		return
	}
	writeJSON(w, http.StatusOK, res)
}

func (s *Service) handleSwipe(w http.ResponseWriter, r *http.Request) {
	var req SwipeRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid_json", "请求体 JSON 格式错误: "+err.Error())
		return
	}
	if req.Serial == "" {
		writeError(w, http.StatusBadRequest, "missing_serial", "缺少 serial 参数")
		return
	}

	res, err := s.Swipe(r.Context(), req)
	if err != nil {
		writeError(w, http.StatusBadGateway, "execution_failed", err.Error())
		return
	}
	writeJSON(w, http.StatusOK, res)
}

func (s *Service) handleEvent(w http.ResponseWriter, r *http.Request) {
	var req EventRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid_json", "请求体 JSON 格式错误: "+err.Error())
		return
	}
	if req.Serial == "" {
		writeError(w, http.StatusBadRequest, "missing_serial", "缺少 serial 参数")
		return
	}

	res, err := s.Event(r.Context(), req)
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid_event", err.Error())
		return
	}
	writeJSON(w, http.StatusOK, res)
}

func (s *Service) handleSendKey(w http.ResponseWriter, r *http.Request) {
	var req SendKeyRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid_json", "请求体 JSON 格式错误: "+err.Error())
		return
	}
	if req.Serial == "" {
		writeError(w, http.StatusBadRequest, "missing_serial", "缺少 serial 参数")
		return
	}

	res, err := s.SendKey(r.Context(), req)
	if err != nil {
		writeError(w, http.StatusBadGateway, "execution_failed", err.Error())
		return
	}
	writeJSON(w, http.StatusOK, res)
}

func (s *Service) handleDeviceStatus(w http.ResponseWriter, r *http.Request) {
	serial := r.URL.Query().Get("serial")
	if serial == "" {
		writeError(w, http.StatusBadRequest, "missing_serial", "缺少 serial 参数")
		return
	}

	status, err := s.GetDeviceStatus(serial)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "device_error", err.Error())
		return
	}
	writeJSON(w, http.StatusOK, status)
}

func (s *Service) handleCheck(w http.ResponseWriter, r *http.Request) {
	var req CheckRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid_json", "请求体 JSON 格式错误: "+err.Error())
		return
	}
	if req.Serial == "" {
		writeError(w, http.StatusBadRequest, "missing_serial", "缺少 serial 参数")
		return
	}

	res, err := s.Check(r.Context(), req)
	if err != nil {
		writeError(w, http.StatusBadRequest, "check_failed", err.Error())
		return
	}
	writeJSON(w, http.StatusOK, res)
}

func (s *Service) handleWait(w http.ResponseWriter, r *http.Request) {
	var req WaitRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid_json", "请求体 JSON 格式错误: "+err.Error())
		return
	}
	if req.Serial == "" {
		writeError(w, http.StatusBadRequest, "missing_serial", "缺少 serial 参数")
		return
	}

	res, err := s.Wait(r.Context(), req)
	if err != nil {
		writeError(w, http.StatusBadRequest, "wait_failed", err.Error())
		return
	}
	writeJSON(w, http.StatusOK, res)
}

func (s *Service) handleTapWhen(w http.ResponseWriter, r *http.Request) {
	var req TapWhenRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid_json", "请求体 JSON 格式错误: "+err.Error())
		return
	}
	if req.Serial == "" {
		writeError(w, http.StatusBadRequest, "missing_serial", "缺少 serial 参数")
		return
	}

	res, err := s.TapWhen(r.Context(), req)
	if err != nil {
		writeError(w, http.StatusBadRequest, "tap_when_failed", err.Error())
		return
	}
	writeJSON(w, http.StatusOK, res)
}

func (s *Service) handleRegionScreenshot(w http.ResponseWriter, r *http.Request) {
	var req RegionScreenshotRequest
	if strings.Contains(r.Header.Get("Content-Type"), "application/json") {
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			writeError(w, http.StatusBadRequest, "invalid_json", "请求体 JSON 格式错误: "+err.Error())
			return
		}
	}
	if req.Serial == "" {
		req.Serial = r.URL.Query().Get("serial")
	}
	if req.Serial == "" {
		writeError(w, http.StatusBadRequest, "missing_serial", "缺少 serial 参数")
		return
	}

	pngBytes, err := s.RegionScreenshot(r.Context(), req)
	if err != nil {
		writeError(w, http.StatusBadRequest, "screenshot_failed", err.Error())
		return
	}

	w.Header().Set("Content-Type", "image/png")
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write(pngBytes)
}
