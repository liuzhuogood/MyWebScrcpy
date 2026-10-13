package mdscript

import (
	"encoding/json"
	"fmt"
	"github.com/robfig/cron/v3"
	"log"
	"net/http"
	"regexp"
	"strconv"
	"strings"
	"time"

	"mywebscrcpy/internal/aiprovider"
	"mywebscrcpy/internal/db"
)

type ScriptChangeNotifier interface {
	ReloadScript(scriptID int64) error
	RemoveScript(scriptID int64)
}

type Handler struct {
	database  *db.DB
	Generator *Generator
	executor  *Executor
	notifier  ScriptChangeNotifier
}

func NewHandler(database *db.DB, executor *Executor, notifier ScriptChangeNotifier) *Handler {
	return &Handler{
		database: database,
		executor: executor,
		notifier: notifier,
	}
}

func (h *Handler) RegisterRoutes(mux *http.ServeMux) {
	mux.HandleFunc("GET /api/md-scripts", h.handleList)
	mux.HandleFunc("GET /api/md-scripts/{id}", h.handleGet)
	mux.HandleFunc("GET /api/md-scripts/{id}/runs", h.handleRuns)
	mux.HandleFunc("POST /api/md-scripts", h.handleCreate)
	mux.HandleFunc("PUT /api/md-scripts/{id}", h.handleUpdate)
	mux.HandleFunc("DELETE /api/md-scripts/{id}", h.handleDelete)
	mux.HandleFunc("POST /api/md-scripts/{id}/compile", h.handleCompile)
	mux.HandleFunc("POST /api/md-scripts/{id}/run", h.handleRun)
}

func writeJSON(w http.ResponseWriter, status int, data interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(data)
}

func writeError(w http.ResponseWriter, status int, msg string) {
	writeJSON(w, status, map[string]interface{}{"error": msg, "message": msg})
}

func (h *Handler) handleList(w http.ResponseWriter, r *http.Request) {
	list, err := h.database.ListMarkdownScripts()
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	if list == nil {
		list = []db.MarkdownScriptRecord{}
	}
	for i := range list {
		h.annotate(&list[i])
	}
	writeJSON(w, http.StatusOK, map[string]interface{}{"scripts": list, "total": len(list)})
}

func (h *Handler) handleGet(w http.ResponseWriter, r *http.Request) {
	idStr := r.PathValue("id")
	id, err := strconv.ParseInt(idStr, 10, 64)
	if err != nil {
		writeError(w, http.StatusBadRequest, "无效脚本 ID")
		return
	}
	item, err := h.database.GetMarkdownScript(id)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	if item == nil {
		writeError(w, http.StatusNotFound, "脚本不存在")
		return
	}
	meta := ParseMarkdown(item.Content, time.UnixMilli(item.UpdatedAt))
	h.annotate(item)
	recompile := item.NeedsGeneration
	writeJSON(w, http.StatusOK, map[string]interface{}{
		"script":           item,
		"meta":             meta,
		"should_recompile": recompile,
	})
}

var regexpLegacyTarget = regexp.MustCompile(`(?mi)^target_devices?\s*:`)

type scriptReq struct {
	Name          string   `json:"name"`
	Content       string   `json:"content"`
	TargetDevices []string `json:"target_devices"`
}

func (h *Handler) handleCreate(w http.ResponseWriter, r *http.Request) {
	var req scriptReq
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "无效 JSON 请求")
		return
	}
	if strings.TrimSpace(req.Name) == "" {
		writeError(w, http.StatusBadRequest, "脚本名称不能为空")
		return
	}

	if err := h.validateRequest(req); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	meta := ParseMarkdown(req.Content, time.Now())
	isEnabled := 1
	if !meta.IsEnabled {
		isEnabled = 0
	}

	created, err := h.database.CreateMarkdownScript(
		req.Name,
		req.TargetDevices,
		req.Content,
		meta.ExtractedVersion,
		meta.ScheduleCron,
		isEnabled,
		meta.TimeoutSec,
	)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	if h.notifier != nil {
		if err := h.notifier.ReloadScript(created.ID); err != nil {
			log.Printf("[MdScript] script=%d 调度重载失败: %v", created.ID, err)
		}
	}
	h.annotate(created)
	writeJSON(w, http.StatusOK, map[string]interface{}{"ok": true, "script": created, "meta": meta})
}

func (h *Handler) handleUpdate(w http.ResponseWriter, r *http.Request) {
	idStr := r.PathValue("id")
	id, err := strconv.ParseInt(idStr, 10, 64)
	if err != nil {
		writeError(w, http.StatusBadRequest, "无效脚本 ID")
		return
	}

	var req scriptReq
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "无效 JSON 请求")
		return
	}

	existing, err := h.database.GetMarkdownScript(id)
	if err != nil || existing == nil {
		writeError(w, http.StatusNotFound, "脚本不存在")
		return
	}

	name := req.Name
	if name == "" {
		name = existing.Name
	}

	if err := h.validateRequest(req); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	meta := ParseMarkdown(req.Content, time.Now())
	isEnabled := existing.IsEnabled
	previousMeta := ParseMarkdown(existing.Content, time.UnixMilli(existing.UpdatedAt))
	if meta.IsEnabled != previousMeta.IsEnabled {
		if meta.IsEnabled {
			isEnabled = 1
		} else {
			isEnabled = 0
		}
	}

	err = h.database.UpdateMarkdownScript(
		id,
		name,
		req.TargetDevices,
		req.Content,
		meta.ExtractedVersion,
		meta.ScheduleCron,
		isEnabled,
		meta.TimeoutSec,
	)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}

	updated, err := h.database.GetMarkdownScript(id)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	if h.notifier != nil {
		if err := h.notifier.ReloadScript(id); err != nil {
			log.Printf("[MdScript] script=%d 调度重载失败: %v", id, err)
		}
	}
	h.annotate(updated)
	writeJSON(w, http.StatusOK, map[string]interface{}{"ok": true, "script": updated, "meta": meta})
}

func (h *Handler) handleDelete(w http.ResponseWriter, r *http.Request) {
	idStr := r.PathValue("id")
	id, err := strconv.ParseInt(idStr, 10, 64)
	if err != nil {
		writeError(w, http.StatusBadRequest, "无效脚本 ID")
		return
	}
	if err := h.database.DeleteMarkdownScript(id); err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	if h.notifier != nil {
		h.notifier.RemoveScript(id)
	}
	writeJSON(w, http.StatusOK, map[string]interface{}{"ok": true})
}

func (h *Handler) annotate(script *db.MarkdownScriptRecord) {
	script.NeedsGeneration = h.Generator == nil || h.Generator.Check(*script) != nil
}

func (h *Handler) handleCompile(w http.ResponseWriter, r *http.Request) {
	id, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
	if err != nil {
		writeError(w, 400, "无效脚本 ID")
		return
	}
	if h.Generator == nil {
		writeError(w, 503, "Python 生成服务不可用")
		return
	}
	existing, err := h.database.GetMarkdownScript(id)
	if err != nil {
		writeError(w, 500, err.Error())
		return
	}
	if existing == nil {
		writeError(w, 404, "脚本不存在")
		return
	}
	if _, err := ParseOptions(existing.Content); err != nil {
		writeError(w, 400, err.Error())
		return
	}
	if err := h.executor.ValidateTargets(existing.TargetDevices); err != nil {
		writeError(w, 400, err.Error())
		return
	}
	flusher, ok := w.(http.Flusher)
	if !ok {
		writeError(w, 500, "当前连接不支持实时输出")
		return
	}
	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("X-Accel-Buffering", "no")
	emit := func(event GenerationEvent) error {
		data, err := json.Marshal(event)
		if err != nil {
			return err
		}
		if _, err := fmt.Fprintf(w, "data: %s\n\n", data); err != nil {
			return err
		}
		flusher.Flush()
		return nil
	}
	if err := h.Generator.Generate(r.Context(), *existing, emit); err != nil {
		log.Printf("[PythonGeneration] script=%d failed: %v", id, err)
		_ = emit(GenerationEvent{ChatEvent: aiprovider.ChatEvent{Type: "error", Error: err.Error()}})
	}
}

func (h *Handler) handleRun(w http.ResponseWriter, r *http.Request) {
	idStr := r.PathValue("id")
	id, err := strconv.ParseInt(idStr, 10, 64)
	if err != nil {
		writeError(w, http.StatusBadRequest, "无效脚本 ID")
		return
	}

	existing, err := h.database.GetMarkdownScript(id)
	if err != nil || existing == nil {
		writeError(w, http.StatusNotFound, "脚本不存在")
		return
	}

	if r.URL.Query().Has("serial") {
		writeError(w, http.StatusBadRequest, "目标设备由任务配置指定")
		return
	}
	results, err := h.executor.Execute(id)
	if err != nil {
		writeError(w, http.StatusConflict, err.Error())
		return
	}
	dispatchID := ""
	if len(results) > 0 {
		dispatchID = results[0].DispatchID
	}
	writeJSON(w, http.StatusOK, map[string]interface{}{"ok": true, "dispatch_id": dispatchID, "results": results, "total": len(results)})
}

func (h *Handler) validateRequest(req scriptReq) error {
	if _, err := ParseOptions(req.Content); err != nil {
		return err
	}
	if err := h.executor.ValidateTargets(req.TargetDevices); err != nil {
		return err
	}
	meta := ParseMarkdown(req.Content, time.Now())
	if meta.ScheduleCron != "" {
		parser := cron.NewParser(cron.SecondOptional | cron.Minute | cron.Hour | cron.Dom | cron.Month | cron.Dow | cron.Descriptor)
		if _, err := parser.Parse(meta.ScheduleCron); err != nil {
			return fmt.Errorf("定时表达式无效: %w", err)
		}
	}
	// The archived legacy document is not rewritten on migration, but cannot be saved as a new targeting source.
	if regexpLegacyTarget.MatchString(req.Content) {
		return fmt.Errorf("请从 Markdown 删除目标设备字段，并在任务配置中选择设备")
	}
	return nil
}

func (h *Handler) handleRuns(w http.ResponseWriter, r *http.Request) {
	id, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
	if err != nil {
		writeError(w, http.StatusBadRequest, "无效脚本 ID")
		return
	}
	items, err := h.database.ListScriptRuns(id)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]interface{}{"tasks": items, "total": len(items)})
}
