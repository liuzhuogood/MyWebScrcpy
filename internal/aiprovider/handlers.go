package aiprovider

import (
	"encoding/json"
	"net/http"
)

type Handler struct {
	service *Service
}

func NewHandler(service *Service) *Handler {
	return &Handler{service: service}
}

func (h *Handler) RegisterRoutes(mux *http.ServeMux) {
	mux.HandleFunc("GET /api/ai/config", h.handleGetConfig)
	mux.HandleFunc("POST /api/ai/config", h.handleSaveConfig)
	mux.HandleFunc("POST /api/ai/test", h.handleTest)
	mux.HandleFunc("POST /api/ai/chat", h.handleChat)
}

func writeJSON(w http.ResponseWriter, status int, data interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(data)
}

func writeError(w http.ResponseWriter, status int, msg string) {
	writeJSON(w, status, map[string]interface{}{"error": msg, "message": msg})
}

func (h *Handler) handleGetConfig(w http.ResponseWriter, r *http.Request) {
	cfg, err := h.service.GetConfig()
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	// 安全掩码 API Key 前端展示
	maskedKey := cfg.APIKey
	if len(maskedKey) > 8 {
		maskedKey = maskedKey[:4] + "••••••••" + maskedKey[len(maskedKey)-4:]
	} else if maskedKey != "" {
		maskedKey = "••••••••"
	}
	writeJSON(w, http.StatusOK, map[string]interface{}{
		"provider":   cfg.Provider,
		"base_url":   cfg.BaseURL,
		"model":      cfg.Model,
		"api_key":    cfg.APIKey,
		"masked_key": maskedKey,
	})
}

func (h *Handler) handleSaveConfig(w http.ResponseWriter, r *http.Request) {
	var req ModelConfig
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "无效 JSON")
		return
	}
	if req.BaseURL == "" || req.Model == "" {
		writeError(w, http.StatusBadRequest, "Base URL 和 Model 不能为空")
		return
	}
	if err := h.service.SaveConfig(req); err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]interface{}{"ok": true, "message": "配置已保存"})
}

func (h *Handler) handleTest(w http.ResponseWriter, r *http.Request) {
	var req ModelConfig
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "无效 JSON")
		return
	}
	msg, err := h.service.TestConnection(r.Context(), req)
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]interface{}{"ok": true, "message": msg})
}

func (h *Handler) handleChat(w http.ResponseWriter, r *http.Request) {
	var req ChatRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "无效请求")
		return
	}
	if len(req.Messages) == 0 {
		writeError(w, http.StatusBadRequest, "消息列表不能为空")
		return
	}
	reply, err := h.service.ChatCompletion(r.Context(), req.Messages)
	if err != nil {
		writeError(w, http.StatusBadGateway, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]interface{}{"ok": true, "reply": reply})
}
