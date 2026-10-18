package devicestate

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
)

type Handler struct {
	service  *Service
	aiClient *AIClient
}

func NewHandler(service *Service, aiClient *AIClient) *Handler {
	return &Handler{
		service:  service,
		aiClient: aiClient,
	}
}

func (h *Handler) RegisterRoutes(mux *http.ServeMux) {
	mux.HandleFunc("GET /api/device-state/{serial}", h.handleGetTotalState)
	mux.HandleFunc("POST /api/device-state/{serial}/decision", h.handleGetStateAndDecision)
	mux.HandleFunc("GET /api/system/service-config", h.handleGetServiceConfig)
	mux.HandleFunc("PUT /api/system/service-config", h.handleSaveServiceConfig)
}

func wantsHTML(r *http.Request) bool {
	accept := r.Header.Get("Accept")
	if strings.Contains(accept, "text/html") {
		// 如果显式请求 HTML
		return true
	}
	return false
}

func writeJSON(w http.ResponseWriter, status int, data any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(data)
}

func (h *Handler) handleGetTotalState(w http.ResponseWriter, r *http.Request) {
	serial := r.PathValue("serial")
	if serial == "" {
		http.Error(w, "missing serial", http.StatusBadRequest)
		return
	}

	var templateIDs []string
	if tmplRaw := r.URL.Query().Get("template_ids"); tmplRaw != "" {
		for _, id := range strings.Split(tmplRaw, ",") {
			if strings.TrimSpace(id) != "" {
				templateIDs = append(templateIDs, strings.TrimSpace(id))
			}
		}
	}

	state, err := h.service.CollectState(r.Context(), serial, templateIDs)
	if err != nil {
		http.Error(w, fmt.Sprintf("采集设备总状态失败: %v", err), http.StatusInternalServerError)
		return
	}

	if wantsHTML(r) {
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		if err := RenderHTML(w, *state, nil); err != nil {
			http.Error(w, fmt.Sprintf("渲染 HTML 失败: %v", err), http.StatusInternalServerError)
		}
		return
	}

	writeJSON(w, http.StatusOK, state)
}

func (h *Handler) handleGetStateAndDecision(w http.ResponseWriter, r *http.Request) {
	serial := r.PathValue("serial")
	if serial == "" {
		http.Error(w, "missing serial", http.StatusBadRequest)
		return
	}

	var req struct {
		TemplateIDs string `json:"template_ids"`
		Goal        string `json:"goal"`
	}
	if r.Body != nil {
		r.Body = http.MaxBytesReader(w, r.Body, 1<<20)
		dec := json.NewDecoder(r.Body)
		dec.DisallowUnknownFields()
		if err := dec.Decode(&req); err != nil && err != io.EOF {
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": "请求体 JSON 格式不合法: " + err.Error()})
			return
		}
	}

	if len(req.Goal) > 1000 {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "goal 长度超出限制 (最多 1000 字符)"})
		return
	}

	var templateIDs []string
	if req.TemplateIDs != "" {
		for _, id := range strings.Split(req.TemplateIDs, ",") {
			if strings.TrimSpace(id) != "" {
				templateIDs = append(templateIDs, strings.TrimSpace(id))
			}
		}
	}

	// 1. 单轮采集设备状态
	state, err := h.service.CollectState(r.Context(), serial, templateIDs)
	if err != nil {
		http.Error(w, fmt.Sprintf("采集设备总状态失败: %v", err), http.StatusInternalServerError)
		return
	}

	// 2. 服务端 AI 决策判断（使用本轮快照，不重复采集）
	var decision AIDecisionResult
	if h.aiClient != nil {
		decision = h.aiClient.Decide(r.Context(), state, req.Goal)
	} else {
		decision = AIDecisionResult{
			Available:     false,
			ObservationID: state.ObservationID,
			Error:         "AI 客户端未初始化",
		}
	}

	total := TotalStateAndDecision{
		DeviceState: *state,
		Decision:    decision,
	}

	if wantsHTML(r) {
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		if err := RenderHTML(w, *state, &decision); err != nil {
			http.Error(w, fmt.Sprintf("渲染 HTML 失败: %v", err), http.StatusInternalServerError)
		}
		return
	}

	writeJSON(w, http.StatusOK, total)
}

func (h *Handler) handleGetServiceConfig(w http.ResponseWriter, r *http.Request) {
	cfg, err := h.aiClient.GetConfig()
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}
	writeJSON(w, http.StatusOK, cfg)
}

// verifySameOrigin ensures the request is not a cross-origin / CSRF mutation.
func verifySameOrigin(r *http.Request) bool {
	if r.Header.Get("Sec-Fetch-Site") == "cross-site" {
		return false
	}
	origin := r.Header.Get("Origin")
	if origin == "" {
		return true
	}
	u, err := url.Parse(origin)
	if err != nil || u.Host == "" || u.Scheme == "" {
		return false
	}
	if u.Scheme != "http" && u.Scheme != "https" {
		return false
	}
	return strings.EqualFold(u.Host, r.Host)
}

func (h *Handler) handleSaveServiceConfig(w http.ResponseWriter, r *http.Request) {
	if !verifySameOrigin(r) {
		writeJSON(w, http.StatusForbidden, map[string]string{"error": "跨站请求已被拦截 (CSRF 防御)"})
		return
	}

	r.Body = http.MaxBytesReader(w, r.Body, 1<<20)
	var body ServiceConfig
	dec := json.NewDecoder(r.Body)
	dec.DisallowUnknownFields()
	if err := dec.Decode(&body); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "请求体 JSON 格式不合法: " + err.Error()})
		return
	}

	// 原子前置校验：任何字段不合法均立即拒绝，不产生局部写副作用
	if h.aiClient != nil {
		if err := h.aiClient.ValidateConfig(body); err != nil {
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": err.Error()})
			return
		}
	}

	// 保存 AI 配置
	if h.aiClient != nil {
		if err := h.aiClient.SaveConfig(body); err != nil {
			writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
			return
		}
	}

	// 返回最新脱敏配置
	h.handleGetServiceConfig(w, r)
}
