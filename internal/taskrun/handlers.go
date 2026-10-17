package taskrun

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"path/filepath"
	"strconv"
	"strings"
)

type Handler struct {
	service *Service
}

func NewHandler(service *Service) *Handler {
	return &Handler{service: service}
}

func (h *Handler) RegisterRoutes(mux *http.ServeMux) {
	// 用户主动扫描接口
	mux.HandleFunc("POST /api/task-runs/scan", h.handleScan)

	// 读接口：列表、单任务详情与声明产出文件
	mux.HandleFunc("GET /api/task-runs", h.handleList)
	mux.HandleFunc("GET /api/task-runs/{id}", h.handleGet)
	mux.HandleFunc("GET /api/task-runs/{id}/outputs/{output_id}", h.handleGetOutput)
}

func writeJSON(w http.ResponseWriter, status int, data any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(data)
}

func writeError(w http.ResponseWriter, status int, msg string) {
	writeJSON(w, status, map[string]string{"error": msg})
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

// handleScan 用户点击扫描刷新时调用
func (h *Handler) handleScan(w http.ResponseWriter, r *http.Request) {
	if !verifySameOrigin(r) {
		writeError(w, http.StatusForbidden, "跨站请求已被拦截 (CSRF 防御)")
		return
	}

	res, err := h.service.Scan(r.Context())
	if err != nil {
		if errors.Is(err, ErrScanInProgress) {
			writeError(w, http.StatusConflict, err.Error())
			return
		}
		writeError(w, http.StatusInternalServerError, fmt.Sprintf("扫描任务目录失败: %v", err))
		return
	}
	writeJSON(w, http.StatusOK, res)
}

// handleList 获取最近一次扫描的内存索引
func (h *Handler) handleList(w http.ResponseWriter, r *http.Request) {
	query := r.URL.Query()
	deviceSerial := query.Get("device_serial")
	status := query.Get("status")
	source := query.Get("source")

	page := 1
	limit := 20
	if p, err := strconv.Atoi(query.Get("page")); err == nil && p > 0 {
		page = p
	}
	if l, err := strconv.Atoi(query.Get("limit")); err == nil && l > 0 {
		limit = l
	}

	items, total, scannedAt, rootDir := h.service.ListTasks(deviceSerial, status, source, page, limit)

	resp := map[string]any{
		"items":      items,
		"total":      total,
		"page":       page,
		"limit":      limit,
		"scanned_at": scannedAt,
		"root_dir":   rootDir,
	}
	writeJSON(w, http.StatusOK, resp)
}

// handleGet 直接读取指定任务目录详情
func (h *Handler) handleGet(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	detail, err := h.service.GetTaskDetail(id)
	if err != nil {
		if errors.Is(err, ErrInvalidTaskID) {
			writeError(w, http.StatusBadRequest, err.Error())
			return
		}
		if errors.Is(err, ErrTaskNotFound) {
			writeError(w, http.StatusNotFound, err.Error())
			return
		}
		writeError(w, http.StatusInternalServerError, fmt.Sprintf("读取任务详情失败: %v", err))
		return
	}
	writeJSON(w, http.StatusOK, detail)
}

// handleGetOutput 安全读取声明产出文件
func (h *Handler) handleGetOutput(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	outputID := r.PathValue("output_id")
	subFile := r.URL.Query().Get("file")

	file, mimeType, isAttachment, err := h.service.GetOutputFile(id, outputID, subFile)
	if err != nil {
		if errors.Is(err, ErrInvalidTaskID) {
			writeError(w, http.StatusBadRequest, err.Error())
			return
		}
		if errors.Is(err, ErrTaskNotFound) || errors.Is(err, ErrOutputNotFound) {
			writeError(w, http.StatusNotFound, err.Error())
			return
		}
		if errors.Is(err, ErrPathTraversal) || errors.Is(err, ErrUnsafeMedia) {
			writeError(w, http.StatusForbidden, err.Error())
			return
		}
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	defer file.Close()

	stat, err := file.Stat()
	if err != nil {
		writeError(w, http.StatusInternalServerError, "获取文件信息失败")
		return
	}

	w.Header().Set("Content-Type", mimeType)
	// 由 http.ServeContent 自动支持完整内容与 206 Partial Content Range

	download := r.URL.Query().Get("download") == "true" || isAttachment
	if download {
		filename := filepath.Base(stat.Name())
		w.Header().Set("Content-Disposition", fmt.Sprintf("attachment; filename=%q", filename))
	} else {
		w.Header().Set("Content-Disposition", "inline")
	}

	http.ServeContent(w, r, stat.Name(), stat.ModTime(), file)
}
