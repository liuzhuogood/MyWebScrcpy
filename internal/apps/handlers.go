package apps

import (
	"encoding/json"
	"net/http"
	"strings"
)

// RegisterRoutes 注册应用安装相关 API 路由。
func RegisterRoutes(mux *http.ServeMux, manager *Manager) {
	h := &handler{manager: manager}
	mux.HandleFunc("POST /api/apps/install", h.install)
}

type handler struct {
	manager *Manager
}

func (h *handler) install(w http.ResponseWriter, r *http.Request) {
	serial := strings.TrimSpace(r.URL.Query().Get("serial"))
	if serial == "" {
		serial = strings.TrimSpace(r.FormValue("serial"))
	}
	if serial == "" {
		writeError(w, http.StatusBadRequest, "missing_serial", "未指定设备序列号")
		return
	}

	// 限制 Multipart 内存占用为 32MB，超出部分写入临时磁盘
	if err := r.ParseMultipartForm(32 << 20); err != nil {
		writeError(w, http.StatusBadRequest, "invalid_multipart", "上传请求无效: "+err.Error())
		return
	}

	file, header, err := r.FormFile("file")
	if err != nil {
		writeError(w, http.StatusBadRequest, "missing_file", "请提供要安装的 APK 文件")
		return
	}
	defer file.Close()

	if !strings.HasSuffix(strings.ToLower(header.Filename), ".apk") {
		writeError(w, http.StatusBadRequest, "invalid_extension", "仅支持安装 .apk 格式文件")
		return
	}

	opts := InstallOptions{
		ReplaceExisting:  r.FormValue("replace") != "false",
		GrantPermissions: r.FormValue("grant") != "false",
		AllowDowngrade:   r.FormValue("downgrade") == "true",
	}

	res, err := h.manager.Install(r.Context(), serial, file, header.Filename, opts)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "install_error", err.Error())
		return
	}

	writeJSON(w, http.StatusOK, res)
}

func writeJSON(w http.ResponseWriter, status int, data any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(data)
}

func writeError(w http.ResponseWriter, status int, code, msg string) {
	writeJSON(w, status, map[string]any{
		"ok":      false,
		"error":   code,
		"message": msg,
	})
}
