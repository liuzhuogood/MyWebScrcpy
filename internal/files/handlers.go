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

// RegisterRoutes 注册绑定到当前手机 serial 的文件管理 API。
func RegisterRoutes(mux *http.ServeMux, manager *Manager) {
	h := &handler{manager: manager}
	mux.HandleFunc("GET /api/files", h.list)
	mux.HandleFunc("GET /api/files/download", h.download)
	mux.HandleFunc("POST /api/files/upload", h.upload)
	mux.HandleFunc("POST /api/files/folders", h.createFolder)
	mux.HandleFunc("POST /api/files/move", h.move)
	mux.HandleFunc("POST /api/files/rename", h.rename)
	mux.HandleFunc("POST /api/files/delete", h.delete)
	mux.HandleFunc("POST /api/files/undo", h.undo)
}

type handler struct{ manager *Manager }

func requireSerial(w http.ResponseWriter, r *http.Request) (string, bool) {
	serial := strings.TrimSpace(r.URL.Query().Get("serial"))
	if serial == "" {
		writeError(w, http.StatusBadRequest, "missing_serial", "未指定手机")
		return "", false
	}
	return serial, true
}

func (h *handler) list(w http.ResponseWriter, r *http.Request) {
	serial, ok := requireSerial(w, r)
	if !ok {
		return
	}
	query := r.URL.Query()
	page, pageSize := parsePage(query.Get("page"), query.Get("page_size"))
	result, err := h.manager.List(serial, query.Get("path"), query.Get("q"), query.Get("type"), query.Get("sort"), query.Get("order"), page, pageSize)
	if err != nil {
		writeManagerError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, result)
}

func (h *handler) download(w http.ResponseWriter, r *http.Request) {
	serial, ok := requireSerial(w, r)
	if !ok {
		return
	}
	rel := r.URL.Query().Get("path")
	if rel == "" {
		writeError(w, http.StatusBadRequest, "invalid_path", "文件路径无效")
		return
	}
	item, err := h.manager.item(serial, rel)
	if err != nil {
		writeManagerError(w, err)
		return
	}
	if item.Kind == "folder" {
		writeError(w, http.StatusBadRequest, "invalid_path", "不能下载文件夹")
		return
	}
	w.Header().Set("Content-Disposition", fmt.Sprintf("attachment; filename*=UTF-8''%s", url.PathEscape(item.Name)))
	if err := h.manager.Download(serial, rel, w); err != nil {
		log.Printf("[files] 下载失败: %v", err)
	}
}

func (h *handler) upload(w http.ResponseWriter, r *http.Request) {
	serial, ok := requireSerial(w, r)
	if !ok {
		return
	}
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
	item, err := h.manager.Upload(serial, r.FormValue("path"), header.Filename, r.FormValue("conflict"), file)
	if err != nil {
		writeManagerError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]interface{}{"ok": true, "item": item})
}

func (h *handler) createFolder(w http.ResponseWriter, r *http.Request) {
	serial, ok := requireSerial(w, r)
	if !ok {
		return
	}
	var body struct {
		Path string `json:"path"`
		Name string `json:"name"`
	}
	if !decodeBody(w, r, &body) {
		return
	}
	item, err := h.manager.CreateFolder(serial, body.Path, body.Name)
	if err != nil {
		writeManagerError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]interface{}{"ok": true, "item": item})
}

func (h *handler) move(w http.ResponseWriter, r *http.Request) {
	serial, ok := requireSerial(w, r)
	if !ok {
		return
	}
	var body struct {
		Path     string `json:"path"`
		Target   string `json:"target"`
		Conflict string `json:"conflict"`
	}
	if !decodeBody(w, r, &body) {
		return
	}
	item, err := h.manager.Move(serial, body.Path, body.Target, body.Conflict)
	if err != nil {
		writeManagerError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]interface{}{"ok": true, "item": item})
}

func (h *handler) rename(w http.ResponseWriter, r *http.Request) {
	serial, ok := requireSerial(w, r)
	if !ok {
		return
	}
	var body struct {
		Path     string `json:"path"`
		Name     string `json:"name"`
		Conflict string `json:"conflict"`
	}
	if !decodeBody(w, r, &body) {
		return
	}
	item, err := h.manager.Rename(serial, body.Path, body.Name, body.Conflict)
	if err != nil {
		writeManagerError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]interface{}{"ok": true, "item": item})
}

func (h *handler) delete(w http.ResponseWriter, r *http.Request) {
	serial, ok := requireSerial(w, r)
	if !ok {
		return
	}
	var body struct {
		Paths []string `json:"paths"`
	}
	if !decodeBody(w, r, &body) {
		return
	}
	token, err := h.manager.Delete(serial, body.Paths)
	if err != nil {
		writeManagerError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]interface{}{"ok": true, "undo_token": token})
}

func (h *handler) undo(w http.ResponseWriter, r *http.Request) {
	serial, ok := requireSerial(w, r)
	if !ok {
		return
	}
	var body struct {
		Token string `json:"token"`
	}
	if !decodeBody(w, r, &body) {
		return
	}
	if err := h.manager.Undo(serial, body.Token); err != nil {
		writeManagerError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]bool{"ok": true})
}

func writeJSON(w http.ResponseWriter, status int, value interface{}) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(value)
}

func writeError(w http.ResponseWriter, status int, code, message string) {
	writeJSON(w, status, map[string]interface{}{"error": map[string]string{"code": code, "message": message}})
}

func decodeBody(w http.ResponseWriter, r *http.Request, target interface{}) bool {
	if err := json.NewDecoder(io.LimitReader(r.Body, 1<<20)).Decode(target); err != nil {
		writeError(w, http.StatusBadRequest, "invalid_json", "请求体无效")
		return false
	}
	return true
}

func parsePage(rawPage, rawSize string) (int, int) {
	page, _ := strconv.Atoi(rawPage)
	pageSize, _ := strconv.Atoi(rawSize)
	return normalizePage(page, pageSize)
}

func writeManagerError(w http.ResponseWriter, err error) {
	status, code, message := http.StatusInternalServerError, "internal_error", "文件操作失败，请重试"
	switch {
	case errors.Is(err, ErrInvalid):
		status, code, message = http.StatusBadRequest, "invalid_request", "请求参数无效"
	case errors.Is(err, ErrForbidden):
		status, code, message = http.StatusForbidden, "forbidden", "文件路径不允许访问"
	case errors.Is(err, ErrDeviceUnavailable):
		status, code, message = http.StatusBadGateway, "device_unavailable", "手机未连接或已断开，请返回设备列表重试"
	case errors.Is(err, ErrNotFound), errors.Is(err, os.ErrNotExist):
		status, code, message = http.StatusNotFound, "not_found", "文件或文件夹不存在"
	case errors.Is(err, ErrConflict):
		status, code, message = http.StatusConflict, "conflict", "目标位置已存在同名文件或文件夹"
	case strings.Contains(err.Error(), "超过"):
		status, code, message = http.StatusRequestEntityTooLarge, "file_too_large", err.Error()
	default:
		log.Printf("[files] 文件操作失败: %v", err)
	}
	writeError(w, status, code, message)
}
