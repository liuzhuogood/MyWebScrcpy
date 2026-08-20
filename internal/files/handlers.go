package files

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"net/http"
	"net/url"
	"os"
	"strconv"
	"strings"
)

// RegisterRoutes 注册文件管理 API。所有路由都经过同一套可选 Bearer 鉴权。
func RegisterRoutes(mux *http.ServeMux, manager *Manager) {
	h := &handler{manager: manager}
	mux.HandleFunc("GET /api/files", h.auth(h.list))
	mux.HandleFunc("GET /api/files/download", h.auth(h.download))
	mux.HandleFunc("POST /api/files/upload", h.auth(h.upload))
	mux.HandleFunc("POST /api/files/folders", h.auth(h.createFolder))
	mux.HandleFunc("POST /api/files/move", h.auth(h.move))
	mux.HandleFunc("POST /api/files/rename", h.auth(h.rename))
	mux.HandleFunc("POST /api/files/delete", h.auth(h.delete))
	mux.HandleFunc("POST /api/files/undo", h.auth(h.undo))
}

type handler struct{ manager *Manager }

func (h *handler) auth(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if !h.manager.Authorized(r) {
			writeError(w, http.StatusUnauthorized, "auth_required", "请输入文件管理访问令牌")
			return
		}
		next(w, r)
	}
}

func writeJSON(w http.ResponseWriter, status int, value interface{}) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(value)
}

func writeError(w http.ResponseWriter, status int, code, message string) {
	writeJSON(w, status, map[string]interface{}{
		"error": map[string]string{"code": code, "message": message},
	})
}

func (h *handler) list(w http.ResponseWriter, r *http.Request) {
	query := r.URL.Query()
	page, pageSize := parsePage(query.Get("page"), query.Get("page_size"))
	result, err := h.manager.List(query.Get("path"), query.Get("q"), query.Get("type"), query.Get("sort"), query.Get("order"), page, pageSize)
	if err != nil {
		writeManagerError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, result)
}

func (h *handler) download(w http.ResponseWriter, r *http.Request) {
	rel := r.URL.Query().Get("path")
	if rel == "" {
		writeError(w, http.StatusBadRequest, "invalid_path", "文件路径无效")
		return
	}
	target, err := h.manager.resolve(rel, false)
	if err != nil {
		writeManagerError(w, err)
		return
	}
	info, err := os.Stat(target)
	if err != nil || info.IsDir() {
		writeError(w, http.StatusNotFound, "not_found", "文件不存在")
		return
	}
	w.Header().Set("Content-Disposition", fmt.Sprintf("attachment; filename*=UTF-8''%s", url.PathEscape(info.Name())))
	http.ServeFile(w, r, target)
}

func (h *handler) upload(w http.ResponseWriter, r *http.Request) {
	if err := r.ParseMultipartForm(32 << 20); err != nil {
		writeError(w, http.StatusBadRequest, "invalid_multipart", "上传请求无效")
		return
	}
	file, header, err := r.FormFile("file")
	if err != nil {
		writeError(w, http.StatusBadRequest, "missing_file", "请选择要上传的文件")
		return
	}
	defer file.Close()
	item, err := h.manager.Upload(r.FormValue("path"), header.Filename, r.FormValue("conflict"), file)
	if err != nil {
		writeManagerError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]interface{}{"ok": true, "item": item})
}

func (h *handler) createFolder(w http.ResponseWriter, r *http.Request) {
	var body struct {
		Path string `json:"path"`
		Name string `json:"name"`
	}
	if !decodeBody(w, r, &body) {
		return
	}
	item, err := h.manager.CreateFolder(body.Path, body.Name)
	if err != nil {
		writeManagerError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]interface{}{"ok": true, "item": item})
}

func (h *handler) move(w http.ResponseWriter, r *http.Request) {
	var body struct {
		Path     string `json:"path"`
		Target   string `json:"target"`
		Conflict string `json:"conflict"`
	}
	if !decodeBody(w, r, &body) {
		return
	}
	item, err := h.manager.Move(body.Path, body.Target, body.Conflict)
	if err != nil {
		writeManagerError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]interface{}{"ok": true, "item": item})
}

func (h *handler) rename(w http.ResponseWriter, r *http.Request) {
	var body struct {
		Path     string `json:"path"`
		Name     string `json:"name"`
		Conflict string `json:"conflict"`
	}
	if !decodeBody(w, r, &body) {
		return
	}
	item, err := h.manager.Rename(body.Path, body.Name, body.Conflict)
	if err != nil {
		writeManagerError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]interface{}{"ok": true, "item": item})
}

func (h *handler) delete(w http.ResponseWriter, r *http.Request) {
	var body struct {
		Paths []string `json:"paths"`
	}
	if !decodeBody(w, r, &body) {
		return
	}
	token, err := h.manager.Delete(body.Paths)
	if err != nil {
		writeManagerError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]interface{}{"ok": true, "undo_token": token})
}

func (h *handler) undo(w http.ResponseWriter, r *http.Request) {
	var body struct {
		Token string `json:"token"`
	}
	if !decodeBody(w, r, &body) {
		return
	}
	if err := h.manager.Undo(body.Token); err != nil {
		writeManagerError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]bool{"ok": true})
}

func decodeBody(w http.ResponseWriter, r *http.Request, target interface{}) bool {
	decoder := json.NewDecoder(io.LimitReader(r.Body, 1<<20))
	if err := decoder.Decode(target); err != nil {
		writeError(w, http.StatusBadRequest, "invalid_json", "请求体无效")
		return false
	}
	return true
}

func parsePage(rawPage, rawSize string) (int, int) {
	page, _ := strconv.Atoi(rawPage)
	pageSize, _ := strconv.Atoi(rawSize)
	if page < 1 {
		page = 1
	}
	if pageSize < 1 {
		pageSize = defaultPageSize
	}
	if pageSize > maxPageSize {
		pageSize = maxPageSize
	}
	return page, pageSize
}

func writeManagerError(w http.ResponseWriter, err error) {
	status, code, message := http.StatusInternalServerError, "internal_error", "文件操作失败，请重试"
	switch {
	case errors.Is(err, ErrInvalid):
		status, code, message = http.StatusBadRequest, "invalid_request", "请求参数无效"
	case errors.Is(err, ErrForbidden):
		status, code, message = http.StatusForbidden, "forbidden", "文件路径不允许访问"
	case errors.Is(err, ErrNotFound):
		status, code, message = http.StatusNotFound, "not_found", "文件或文件夹不存在"
	case errors.Is(err, ErrConflict):
		status, code, message = http.StatusConflict, "conflict", "目标位置已存在同名文件或文件夹"
	case strings.Contains(err.Error(), "超过"):
		status, code, message = http.StatusRequestEntityTooLarge, "file_too_large", err.Error()
	case errors.Is(err, os.ErrNotExist):
		status, code, message = http.StatusNotFound, "not_found", "文件或文件夹不存在"
	default:
		log.Printf("[files] 文件操作失败: %v", err)
	}
	writeError(w, status, code, message)
}
