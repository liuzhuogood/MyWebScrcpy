package skillsmarket

import (
	"encoding/json"
	"fmt"
	"io"
	"mime"
	"net/http"
	"net/url"
	"strings"
)

// RegisterRoutes registers AI integrations and skills market endpoints on mux.
func RegisterRoutes(mux *http.ServeMux, manager *Manager) {
	mux.HandleFunc("GET /api/ai-integrations", handleAiIntegrations(manager))
	mux.HandleFunc("GET /api/skills", handleListSkills(manager))
	mux.HandleFunc("GET /api/skills/{id}", handleGetSkill(manager))
	mux.HandleFunc("GET /api/skills/{id}/download", handleDownloadSkill(manager))
	mux.HandleFunc("POST /api/skills/{id}/install", handleInstallSkill(manager))
}

func writeJSON(w http.ResponseWriter, status int, data interface{}) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(data)
}

func writeError(w http.ResponseWriter, status int, message string) {
	writeJSON(w, status, map[string]string{"error": message})
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

func handleAiIntegrations(m *Manager) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			w.Header().Set("Allow", "GET")
			writeError(w, http.StatusMethodNotAllowed, "method not allowed")
			return
		}
		tools := AllAiTools(m.HomeDir())
		writeJSON(w, http.StatusOK, AiIntegrationsResponse{Tools: tools})
	}
}

func handleListSkills(m *Manager) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			w.Header().Set("Allow", "GET")
			writeError(w, http.StatusMethodNotAllowed, "method not allowed")
			return
		}
		w.Header().Set("Cache-Control", "no-store")
		skills := m.ListSkillsContext(r.Context())
		writeJSON(w, http.StatusOK, SkillsListResponse{Skills: skills})
	}
}

func handleGetSkill(m *Manager) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			w.Header().Set("Allow", "GET")
			writeError(w, http.StatusMethodNotAllowed, "method not allowed")
			return
		}
		id := r.PathValue("id")
		w.Header().Set("Cache-Control", "no-store")
		detail, ok := m.GetSkillDetailContext(r.Context(), id)
		if !ok {
			writeError(w, http.StatusNotFound, "skill not found")
			return
		}
		writeJSON(w, http.StatusOK, detail)
	}
}

func handleDownloadSkill(m *Manager) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			w.Header().Set("Allow", "GET")
			writeError(w, http.StatusMethodNotAllowed, "method not allowed")
			return
		}
		id := r.PathValue("id")
		if !IsKnownSkillID(id) {
			writeError(w, http.StatusNotFound, "skill not found")
			return
		}

		zipBytes, err := GenerateSkillZip(SkillID(id))
		if err != nil {
			writeError(w, http.StatusInternalServerError, fmt.Sprintf("生成 ZIP 失败: %v", err))
			return
		}

		w.Header().Set("Content-Type", "application/zip")
		w.Header().Set("Content-Disposition", fmt.Sprintf(`attachment; filename="%s.zip"`, id))
		w.Header().Set("Cache-Control", "no-cache")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write(zipBytes)
	}
}

func handleInstallSkill(m *Manager) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			w.Header().Set("Allow", "POST")
			writeError(w, http.StatusMethodNotAllowed, "method not allowed")
			return
		}

		id := r.PathValue("id")
		if !IsKnownSkillID(id) {
			writeError(w, http.StatusNotFound, "skill not found")
			return
		}

		if !verifySameOrigin(r) {
			writeError(w, http.StatusForbidden, "跨域请求已拒绝")
			return
		}

		ct := r.Header.Get("Content-Type")
		mediaType, _, err := mime.ParseMediaType(ct)
		if err != nil || mediaType != "application/json" {
			writeError(w, http.StatusBadRequest, "Content-Type 必须为 application/json")
			return
		}

		r.Body = http.MaxBytesReader(w, r.Body, 1<<20)
		var req SkillInstallRequest
		decoder := json.NewDecoder(r.Body)
		decoder.DisallowUnknownFields()
		if err := decoder.Decode(&req); err != nil {
			writeError(w, http.StatusBadRequest, fmt.Sprintf("请求体格式错误: %v", err))
			return
		}

		// Ensure no trailing JSON or extraneous data
		var extra any
		if err := decoder.Decode(&extra); err != io.EOF {
			writeError(w, http.StatusBadRequest, "请求体包含多余数据")
			return
		}

		if len(req.Targets) == 0 {
			writeError(w, http.StatusBadRequest, "targets 不能为空")
			return
		}

		seen := make(map[AiToolID]bool)
		for _, t := range req.Targets {
			if _, ok := GetTargetSkillsRelPath(t); !ok {
				writeError(w, http.StatusBadRequest, fmt.Sprintf("未知目标助手: %s", t))
				return
			}
			if seen[t] {
				writeError(w, http.StatusBadRequest, fmt.Sprintf("存在重复的目标助手: %s", t))
				return
			}
			seen[t] = true
		}

		results, err := m.InstallSkill(r.Context(), SkillID(id), req.Targets)
		if err != nil {
			writeError(w, http.StatusBadRequest, err.Error())
			return
		}

		w.Header().Set("Cache-Control", "no-store")
		writeJSON(w, http.StatusOK, SkillInstallResponse{
			SkillID: SkillID(id),
			Results: results,
		})
	}
}
