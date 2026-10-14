package operationlog

import (
	"encoding/json"
	"errors"
	"io"
	"log"
	"net/http"
	"strconv"
	"strings"
	"time"

	"mywebscrcpy/internal/db"
)

func (s *Service) RegisterRoutes(mux *http.ServeMux) {
	mux.HandleFunc("POST /api/operation-logs", s.appendLogs)
	mux.HandleFunc("GET /api/operation-logs", s.listLogs)
	mux.HandleFunc("DELETE /api/operation-logs", s.clearLogs)
}

func writeJSON(w http.ResponseWriter, status int, value any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(value)
}

func writeError(w http.ResponseWriter, status int, message string) {
	writeJSON(w, status, map[string]string{"message": message})
}

func (s *Service) appendLogs(w http.ResponseWriter, r *http.Request) {
	var body struct {
		Items []db.OperationLog `json:"items"`
	}
	decoder := json.NewDecoder(http.MaxBytesReader(w, r.Body, 256<<10))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&body); err != nil {
		writeError(w, 400, "无效的日志请求")
		return
	}
	if err := decoder.Decode(new(any)); !errors.Is(err, io.EOF) || len(body.Items) == 0 || len(body.Items) > 50 {
		writeError(w, 400, "每批需要 1–50 条日志，且只能包含一个 JSON 对象")
		return
	}
	now := time.Now()
	for _, item := range body.Items {
		validType := item.Type == "tap" || item.Type == "longpress" || item.Type == "swipe" || item.Type == "key" || item.Type == "text"
		if !validType || strings.TrimSpace(item.ID) == "" || len(item.ID) > 128 || strings.TrimSpace(item.Serial) == "" || len(item.Serial) > 256 ||
			strings.TrimSpace(item.Description) == "" || len(item.Description) > 8192 || strings.TrimSpace(item.ADBCommand) == "" || len(item.ADBCommand) > 16384 ||
			item.OccurredAt <= now.Add(-db.OperationLogRetention).UnixMilli() || item.OccurredAt > now.Add(time.Minute).UnixMilli() {
			writeError(w, 400, "日志字段无效，时间必须在最近 7 天内（允许 1 分钟时钟偏差）")
			return
		}
	}
	if _, err := s.submit(r.Context(), job{items: body.Items}); err != nil {
		writeError(w, 503, "操作日志保存失败，请稍后重试")
		return
	}
	// Only acknowledge a committed transaction, so background retries can be reliable.
	writeJSON(w, http.StatusCreated, map[string]int{"saved": len(body.Items)})
}

func (s *Service) listLogs(w http.ResponseWriter, r *http.Request) {
	if s.database == nil {
		writeError(w, 503, "操作日志服务不可用")
		return
	}
	q := r.URL.Query()
	limit := 100
	if value := q.Get("limit"); value != "" {
		var err error
		limit, err = strconv.Atoi(value)
		if err != nil || limit < 1 || limit > 500 {
			writeError(w, 400, "limit 必须为 1–500")
			return
		}
	}
	var before int64
	if value := q.Get("before"); value != "" {
		var err error
		before, err = strconv.ParseInt(value, 10, 64)
		if err != nil || before < 1 {
			writeError(w, 400, "before 必须为正整数游标")
			return
		}
	}
	items, cursor, err := s.database.ListOperationLogs(r.Context(), q.Get("serial"), before, limit)
	if err != nil {
		log.Printf("[OperationLog] 读取日志失败: %v", err)
		writeError(w, 503, "读取操作日志失败")
		return
	}
	writeJSON(w, 200, map[string]any{"items": items, "next_before": cursor})
}

func (s *Service) clearLogs(w http.ResponseWriter, r *http.Request) {
	deleted, err := s.submit(r.Context(), job{clear: true, serial: r.URL.Query().Get("serial")})
	if err != nil {
		writeError(w, 503, "清理操作日志失败，请稍后重试")
		return
	}
	writeJSON(w, 200, map[string]int64{"deleted": deleted})
}
