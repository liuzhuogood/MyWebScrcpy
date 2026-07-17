package scripts

import (
	"encoding/json"
	"log"
	"net/http"
)

// writeJSON 写入 JSON 响应。
func writeJSON(w http.ResponseWriter, status int, v interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(v)
}

// writeError 写入 JSON 错误响应。
func writeError(w http.ResponseWriter, status int, msg string) {
	writeJSON(w, status, map[string]string{"error": msg})
}

// RegisterRoutes 注册脚本管理 API 路由到 mux。
func RegisterRoutes(mux *http.ServeMux) {
	mux.HandleFunc("GET /api/scripts/categories", handleListCategories)
	mux.HandleFunc("POST /api/scripts/categories", handleCreateCategory)
	mux.HandleFunc("PATCH /api/scripts/categories/{cat}", handleRenameCategory)
	mux.HandleFunc("DELETE /api/scripts/categories/{cat}", handleDeleteCategory)
	mux.HandleFunc("GET /api/scripts", handleListScripts)
	mux.HandleFunc("GET /api/scripts/{cat}/{name}", handleGetScript)
	mux.HandleFunc("POST /api/scripts/{cat}", handleCreateScript)
	mux.HandleFunc("PUT /api/scripts/{cat}/{name}", handleUpdateScript)
	mux.HandleFunc("DELETE /api/scripts/{cat}/{name}", handleDeleteScript)
}

// handleListCategories GET /api/scripts/categories
func handleListCategories(w http.ResponseWriter, r *http.Request) {
	cats, err := ListCategories()
	if err != nil {
		log.Printf("[scripts] 列出分类失败: %v", err)
		writeError(w, http.StatusInternalServerError, "列出分类失败")
		return
	}
	if cats == nil {
		cats = []string{}
	}
	writeJSON(w, http.StatusOK, map[string]interface{}{"categories": cats})
}

// handleCreateCategory POST /api/scripts/categories
func handleCreateCategory(w http.ResponseWriter, r *http.Request) {
	var body struct {
		Name string `json:"name"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		writeError(w, http.StatusBadRequest, "请求体无效")
		return
	}
	if !ValidName(body.Name) {
		writeError(w, http.StatusBadRequest, "分类名不合法，只允许中文、字母、数字、_、-")
		return
	}
	if err := CreateCategory(body.Name); err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]interface{}{"ok": true, "name": body.Name})
}

// handleRenameCategory PATCH /api/scripts/categories/{cat}
func handleRenameCategory(w http.ResponseWriter, r *http.Request) {
	cat := r.PathValue("cat")
	var body struct {
		Name string `json:"name"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		writeError(w, http.StatusBadRequest, "请求体无效")
		return
	}
	if !ValidName(cat) || !ValidName(body.Name) {
		writeError(w, http.StatusBadRequest, "分类名不合法")
		return
	}
	if err := RenameCategory(cat, body.Name); err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]interface{}{"ok": true, "name": body.Name})
}

// handleDeleteCategory DELETE /api/scripts/categories/{cat}
func handleDeleteCategory(w http.ResponseWriter, r *http.Request) {
	cat := r.PathValue("cat")
	if !ValidName(cat) {
		writeError(w, http.StatusBadRequest, "分类名不合法")
		return
	}
	if r.URL.Query().Get("confirm") != "true" {
		writeError(w, http.StatusBadRequest, "删除分类需确认：请传 confirm=true 参数")
		return
	}
	if err := DeleteCategory(cat); err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]interface{}{"ok": true})
}

// handleListScripts GET /api/scripts?category=xxx
func handleListScripts(w http.ResponseWriter, r *http.Request) {
	cat := r.URL.Query().Get("category")
	if cat == "" {
		writeError(w, http.StatusBadRequest, "缺少 category 参数")
		return
	}
	if !ValidName(cat) {
		writeError(w, http.StatusBadRequest, "分类名不合法")
		return
	}
	list, err := ListScripts(cat)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	if list == nil {
		list = []ScriptInfo{}
	}
	writeJSON(w, http.StatusOK, map[string]interface{}{"scripts": list})
}

// handleGetScript GET /api/scripts/{cat}/{name}
func handleGetScript(w http.ResponseWriter, r *http.Request) {
	cat := r.PathValue("cat")
	name := r.PathValue("name")
	if !ValidName(cat) || !ValidName(name) {
		writeError(w, http.StatusBadRequest, "参数不合法")
		return
	}
	detail, err := GetScript(cat, name)
	if err != nil {
		writeError(w, http.StatusNotFound, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, detail)
}

// handleCreateScript POST /api/scripts/{cat}
func handleCreateScript(w http.ResponseWriter, r *http.Request) {
	cat := r.PathValue("cat")
	if !ValidName(cat) {
		writeError(w, http.StatusBadRequest, "分类名不合法")
		return
	}
	var body struct {
		Name        string `json:"name"`
		Description string `json:"description"`
		Params      string `json:"params"`
		Content     string `json:"content"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		writeError(w, http.StatusBadRequest, "请求体无效")
		return
	}
	if !ValidName(body.Name) {
		writeError(w, http.StatusBadRequest, "脚本名不合法")
		return
	}
	info := ScriptInfo{
		Name:        body.Name,
		Description: body.Description,
		Params:      body.Params,
		Content:     body.Content,
	}
	detail, err := CreateScript(cat, info)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]interface{}{"ok": true, "script": detail})
}

// handleUpdateScript PUT /api/scripts/{cat}/{name}
func handleUpdateScript(w http.ResponseWriter, r *http.Request) {
	cat := r.PathValue("cat")
	name := r.PathValue("name")
	if !ValidName(cat) || !ValidName(name) {
		writeError(w, http.StatusBadRequest, "参数不合法")
		return
	}
	var body struct {
		Name        string `json:"name"`
		Description string `json:"description"`
		Params      string `json:"params"`
		Content     string `json:"content"`
		Updated     string `json:"updated"` // If-Match 轻量并发保护
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		writeError(w, http.StatusBadRequest, "请求体无效")
		return
	}
	if body.Name != "" && !ValidName(body.Name) {
		writeError(w, http.StatusBadRequest, "新脚本名不合法")
		return
	}
	info := ScriptInfo{
		Name:        body.Name,
		Description: body.Description,
		Params:      body.Params,
		Content:     body.Content,
	}
	detail, err := UpdateScript(cat, name, info, body.Updated)
	if err != nil {
		if err == ErrConflict {
			writeError(w, http.StatusConflict, err.Error())
		} else {
			writeError(w, http.StatusInternalServerError, err.Error())
		}
		return
	}
	writeJSON(w, http.StatusOK, map[string]interface{}{"ok": true, "script": detail})
}

// handleDeleteScript DELETE /api/scripts/{cat}/{name}
func handleDeleteScript(w http.ResponseWriter, r *http.Request) {
	cat := r.PathValue("cat")
	name := r.PathValue("name")
	if !ValidName(cat) || !ValidName(name) {
		writeError(w, http.StatusBadRequest, "参数不合法")
		return
	}
	if err := DeleteScript(cat, name); err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]interface{}{"ok": true})
}
