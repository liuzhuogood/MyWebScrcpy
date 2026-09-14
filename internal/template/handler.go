package template

import (
	"archive/zip"
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"image"
	_ "image/jpeg"
	"image/png"
	"io"
	"net/http"
	"path/filepath"
	"strconv"
	"strings"
	"time"
)

// Handler provides HTTP endpoints for template management and detection.
type Handler struct {
	service *Service
}

// NewHandler creates a new Handler instance.
func NewHandler(s *Service) *Handler {
	return &Handler{service: s}
}

// RegisterRoutes registers all template-related routes on the given HTTP serve mux.
func RegisterRoutes(mux *http.ServeMux, s *Service) *Handler {
	h := NewHandler(s)
	mux.HandleFunc("GET /api/templates", h.handleListTemplates)
	mux.HandleFunc("POST /api/templates", h.handleCreateTemplate)
	mux.HandleFunc("GET /api/templates/export", h.handleExportTemplates)
	mux.HandleFunc("POST /api/templates/import", h.handleImportTemplates)
	mux.HandleFunc("GET /api/templates/status", h.handleGetStatus)
	mux.HandleFunc("PUT /api/templates/status", h.handleSetStatus)
	mux.HandleFunc("GET /api/templates/matches", h.handleGetMatches)
	mux.HandleFunc("POST /api/templates/detect", h.handleDetect)
	mux.HandleFunc("GET /api/templates/{id}", h.handleGetTemplate)
	mux.HandleFunc("PUT /api/templates/{id}", h.handleUpdateTemplate)
	mux.HandleFunc("DELETE /api/templates/{id}", h.handleDeleteTemplate)
	mux.HandleFunc("GET /api/templates/{id}/image", h.handleGetTemplateImage)
	mux.HandleFunc("POST /api/templates/click", h.handleClickTemplate)
	return h
}

// writeJSON marshals value to JSON and writes HTTP response.
func writeJSON(w http.ResponseWriter, status int, v interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}

// writeError writes a standardized JSON error response.
func writeError(w http.ResponseWriter, status int, code, msg string) {
	writeJSON(w, status, map[string]string{
		"error":   code,
		"message": msg,
	})
}

// handleListTemplates GET /api/templates?serial=...
func (h *Handler) handleListTemplates(w http.ResponseWriter, r *http.Request) {
	serial := strings.TrimSpace(r.URL.Query().Get("serial"))
	var list []*Template
	if serial == "" || serial == "all" {
		list = h.service.Storage().ListAll()
	} else {
		list = h.service.Storage().ListTemplates(serial)
	}
	if list == nil {
		list = []*Template{}
	}
	writeJSON(w, http.StatusOK, map[string]interface{}{
		"templates": list,
		"total":     len(list),
	})
}

// handleCreateTemplate POST /api/templates (multipart/form-data or JSON)
func (h *Handler) handleCreateTemplate(w http.ResponseWriter, r *http.Request) {
	contentType := r.Header.Get("Content-Type")
	var req CreateTemplateRequest
	var img image.Image
	var err error

	if strings.HasPrefix(contentType, "multipart/form-data") {
		if err := r.ParseMultipartForm(32 << 20); err != nil {
			writeError(w, http.StatusBadRequest, "invalid_form", err.Error())
			return
		}
		req.Name = r.FormValue("name")
		req.Serial = r.FormValue("serial")
		req.ID = r.FormValue("id")
		req.Method = r.FormValue("method")

		if thStr := r.FormValue("threshold"); thStr != "" {
			if th, parseErr := strconv.ParseFloat(thStr, 64); parseErr == nil {
				req.Threshold = th
			}
		}
		if gsStr := r.FormValue("grayscale"); gsStr != "" {
			gs := (gsStr == "true" || gsStr == "1")
			req.Grayscale = &gs
		}
		if enStr := r.FormValue("enabled"); enStr != "" {
			en := (enStr == "true" || enStr == "1")
			req.Enabled = &en
		}
		if scalesStr := r.FormValue("scales"); scalesStr != "" {
			req.Scales = parseScales(scalesStr)
		}

		file, fileHeader, fileErr := r.FormFile("image")
		if fileErr != nil {
			file, fileHeader, fileErr = r.FormFile("file")
		}
		if fileErr == nil {
			defer file.Close()
			img, _, err = image.Decode(file)
			if err != nil {
				writeError(w, http.StatusBadRequest, "invalid_image", "failed to decode uploaded image")
				return
			}
		} else if b64 := r.FormValue("image_base64"); b64 != "" {
			req.ImageBase64 = b64
		}

		if strings.TrimSpace(req.Name) == "" && fileHeader != nil {
			fn := filepath.Base(fileHeader.Filename)
			ext := filepath.Ext(fn)
			req.Name = strings.TrimSpace(strings.TrimSuffix(fn, ext))
		}
	} else {
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			writeError(w, http.StatusBadRequest, "invalid_json", err.Error())
			return
		}
	}

	tmpl, err := h.service.Storage().CreateTemplate(req, img)
	if err != nil {
		status := http.StatusBadRequest
		if errors.Is(err, ErrTemplateExists) {
			status = http.StatusConflict
		}
		writeError(w, status, "create_template_failed", err.Error())
		return
	}

	writeJSON(w, http.StatusCreated, tmpl)
}

// handleGetTemplate GET /api/templates/{id}
func (h *Handler) handleGetTemplate(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	tmpl, err := h.service.Storage().GetTemplate(id)
	if err != nil {
		if errors.Is(err, ErrTemplateNotFound) {
			writeError(w, http.StatusNotFound, "template_not_found", "template not found")
			return
		}
		writeError(w, http.StatusInternalServerError, "get_template_failed", err.Error())
		return
	}
	writeJSON(w, http.StatusOK, tmpl)
}

// handleUpdateTemplate PUT /api/templates/{id}
func (h *Handler) handleUpdateTemplate(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	contentType := r.Header.Get("Content-Type")
	var req UpdateTemplateRequest
	var img image.Image
	var err error

	if strings.HasPrefix(contentType, "multipart/form-data") {
		if err := r.ParseMultipartForm(32 << 20); err != nil {
			writeError(w, http.StatusBadRequest, "invalid_form", err.Error())
			return
		}
		if name := r.FormValue("name"); name != "" {
			req.Name = &name
		}
		if serial := r.FormValue("serial"); serial != "" {
			req.Serial = &serial
		}
		if method := r.FormValue("method"); method != "" {
			req.Method = &method
		}
		if thStr := r.FormValue("threshold"); thStr != "" {
			if th, parseErr := strconv.ParseFloat(thStr, 64); parseErr == nil {
				req.Threshold = &th
			}
		}
		if gsStr := r.FormValue("grayscale"); gsStr != "" {
			gs := (gsStr == "true" || gsStr == "1")
			req.Grayscale = &gs
		}
		if enStr := r.FormValue("enabled"); enStr != "" {
			en := (enStr == "true" || enStr == "1")
			req.Enabled = &en
		}
		if scalesStr := r.FormValue("scales"); scalesStr != "" {
			sc := parseScales(scalesStr)
			req.Scales = &sc
		}

		file, _, fileErr := r.FormFile("image")
		if fileErr != nil {
			file, _, fileErr = r.FormFile("file")
		}
		if fileErr == nil {
			defer file.Close()
			img, _, err = image.Decode(file)
			if err != nil {
				writeError(w, http.StatusBadRequest, "invalid_image", "failed to decode uploaded image")
				return
			}
		} else if b64 := r.FormValue("image_base64"); b64 != "" {
			req.ImageBase64 = &b64
		}
	} else {
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			writeError(w, http.StatusBadRequest, "invalid_json", err.Error())
			return
		}
	}

	updated, err := h.service.Storage().UpdateTemplate(id, req, img)
	if err != nil {
		status := http.StatusBadRequest
		if errors.Is(err, ErrTemplateNotFound) {
			status = http.StatusNotFound
		}
		writeError(w, status, "update_template_failed", err.Error())
		return
	}

	writeJSON(w, http.StatusOK, updated)
}

// handleDeleteTemplate DELETE /api/templates/{id}
func (h *Handler) handleDeleteTemplate(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	err := h.service.Storage().DeleteTemplate(id)
	if err != nil {
		if errors.Is(err, ErrTemplateNotFound) {
			writeError(w, http.StatusNotFound, "template_not_found", "template not found")
			return
		}
		writeError(w, http.StatusInternalServerError, "delete_template_failed", err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]interface{}{"ok": true, "id": id})
}

// handleGetTemplateImage GET /api/templates/{id}/image
func (h *Handler) handleGetTemplateImage(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	img, err := h.service.Storage().GetTemplateImage(id)
	if err != nil {
		if errors.Is(err, ErrTemplateNotFound) {
			writeError(w, http.StatusNotFound, "template_not_found", "template not found")
			return
		}
		writeError(w, http.StatusInternalServerError, "get_image_failed", err.Error())
		return
	}

	if r.URL.Query().Get("download") == "1" {
		tmpl, err := h.service.Storage().GetTemplate(id)
		name := id
		if err == nil && tmpl != nil && tmpl.Name != "" {
			name = tmpl.Name
		}
		safeName := sanitizeFilename(name)
		if strings.HasSuffix(strings.ToLower(safeName), ".png") {
			safeName = safeName[:len(safeName)-4]
		}
		if safeName == "" {
			safeName = id
		}
		if safeName == "" {
			safeName = "template"
		}
		w.Header().Set("Content-Disposition", fmt.Sprintf(`attachment; filename="%s.png"`, safeName))
	}

	w.Header().Set("Content-Type", "image/png")
	_ = png.Encode(w, img)
}

// handleGetStatus GET /api/templates/status?serial=...
func (h *Handler) handleGetStatus(w http.ResponseWriter, r *http.Request) {
	serial := strings.TrimSpace(r.URL.Query().Get("serial"))
	if serial == "" {
		writeError(w, http.StatusBadRequest, "missing_serial", "serial query parameter is required")
		return
	}
	status := h.service.GetDeviceStatus(serial)
	writeJSON(w, http.StatusOK, status)
}

// handleSetStatus PUT /api/templates/status
func (h *Handler) handleSetStatus(w http.ResponseWriter, r *http.Request) {
	var body struct {
		Serial  string `json:"serial"`
		Enabled bool   `json:"enabled"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		writeError(w, http.StatusBadRequest, "invalid_json", err.Error())
		return
	}
	body.Serial = strings.TrimSpace(body.Serial)
	if body.Serial == "" {
		writeError(w, http.StatusBadRequest, "missing_serial", "serial is required")
		return
	}
	if err := h.service.SetDeviceMatching(body.Serial, body.Enabled); err != nil {
		writeError(w, http.StatusInternalServerError, "set_matching_failed", err.Error())
		return
	}
	status := h.service.GetDeviceStatus(body.Serial)
	writeJSON(w, http.StatusOK, status)
}

// handleGetMatches GET /api/templates/matches?serial=...
func (h *Handler) handleGetMatches(w http.ResponseWriter, r *http.Request) {
	serial := strings.TrimSpace(r.URL.Query().Get("serial"))
	if serial == "" {
		writeError(w, http.StatusBadRequest, "missing_serial", "serial query parameter is required")
		return
	}
	matches := h.service.GetLatestMatches(serial)
	writeJSON(w, http.StatusOK, map[string]interface{}{
		"serial":  serial,
		"matches": matches,
		"count":   len(matches),
	})
}

// handleDetect POST /api/templates/detect
func (h *Handler) handleDetect(w http.ResponseWriter, r *http.Request) {
	contentType := r.Header.Get("Content-Type")
	var req MatchRequest
	var img image.Image
	var err error

	if strings.HasPrefix(contentType, "multipart/form-data") {
		if err := r.ParseMultipartForm(32 << 20); err != nil {
			writeError(w, http.StatusBadRequest, "invalid_form", err.Error())
			return
		}
		req.Serial = r.FormValue("serial")
		if thStr := r.FormValue("min_score"); thStr != "" {
			if th, parseErr := strconv.ParseFloat(thStr, 64); parseErr == nil {
				req.MinScore = th
			}
		}
		if mrStr := r.FormValue("max_results"); mrStr != "" {
			if mr, parseErr := strconv.Atoi(mrStr); parseErr == nil {
				req.MaxResults = mr
			}
		}
		if idsStr := r.FormValue("template_ids"); idsStr != "" {
			if strings.HasPrefix(idsStr, "[") {
				_ = json.Unmarshal([]byte(idsStr), &req.TemplateIDs)
			} else {
				for _, id := range strings.Split(idsStr, ",") {
					if trimmed := strings.TrimSpace(id); trimmed != "" {
						req.TemplateIDs = append(req.TemplateIDs, trimmed)
					}
				}
			}
		}
		file, _, fileErr := r.FormFile("image")
		if fileErr != nil {
			file, _, fileErr = r.FormFile("file")
		}
		if fileErr == nil {
			defer file.Close()
			img, _, err = image.Decode(file)
			if err != nil {
				writeError(w, http.StatusBadRequest, "invalid_image", "failed to decode uploaded image")
				return
			}
		} else if b64 := r.FormValue("image_base64"); b64 != "" {
			req.ImageBase64 = b64
		}
	} else {
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			writeError(w, http.StatusBadRequest, "invalid_json", err.Error())
			return
		}
	}

	if img == nil && req.ImageBase64 != "" {
		img, err = decodeBase64Image(req.ImageBase64)
		if err != nil {
			writeError(w, http.StatusBadRequest, "invalid_image_base64", err.Error())
			return
		}
	}

	if img == nil && req.Serial != "" {
		img, err = h.service.CaptureScreen(r.Context(), req.Serial)
		if err != nil {
			writeError(w, http.StatusBadGateway, "capture_screen_failed", err.Error())
			return
		}
	}

	if img == nil {
		writeError(w, http.StatusBadRequest, "missing_image", "image or valid serial is required")
		return
	}

	opts := MatchOptions{
		MinScore:   req.MinScore,
		MaxResults: req.MaxResults,
	}

	matches, err := h.service.DetectWithOptions(r.Context(), req.Serial, img, req.TemplateIDs, opts)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "detect_failed", err.Error())
		return
	}

	if req.Serial != "" {
		h.service.BroadcastMatches(req.Serial, matches)
	}

	writeJSON(w, http.StatusOK, map[string]interface{}{
		"matches": matches,
		"count":   len(matches),
	})
}

// parseScales parses comma-separated or JSON float numbers.
func parseScales(s string) []float64 {
	s = strings.TrimSpace(s)
	var scales []float64
	if strings.HasPrefix(s, "[") {
		if err := json.Unmarshal([]byte(s), &scales); err == nil {
			return scales
		}
	}
	parts := strings.Split(s, ",")
	for _, p := range parts {
		p = strings.TrimSpace(p)
		if f, err := strconv.ParseFloat(p, 64); err == nil && f > 0 {
			scales = append(scales, f)
		}
	}
	return scales
}

// handleExportTemplates GET /api/templates/export?serial=...&scope=...
func (h *Handler) handleExportTemplates(w http.ResponseWriter, r *http.Request) {
	serial := strings.TrimSpace(r.URL.Query().Get("serial"))
	scope := strings.TrimSpace(r.URL.Query().Get("scope"))

	var list []*Template
	switch strings.ToLower(scope) {
	case "global":
		list = h.service.Storage().ListTemplatesStrict(GlobalSerial)
	case "device":
		if serial != "" && serial != "all" && serial != GlobalSerial {
			list = h.service.Storage().ListTemplatesStrict(serial)
		} else {
			for _, t := range h.service.Storage().ListAll() {
				if t.Serial != GlobalSerial {
					list = append(list, t)
				}
			}
		}
	case "all":
		list = h.service.Storage().ListAll()
	default:
		if serial == "" || serial == "all" {
			list = h.service.Storage().ListAll()
		} else if serial == GlobalSerial {
			list = h.service.Storage().ListTemplatesStrict(GlobalSerial)
		} else {
			list = h.service.Storage().ListTemplates(serial)
		}
	}

	serialTag := serial
	if serialTag == "" {
		if strings.ToLower(scope) == "global" {
			serialTag = "global"
		} else {
			serialTag = "all"
		}
	} else {
		serialTag = sanitizeFilename(serialTag)
	}

	timestamp := time.Now().Format("20060102150405")
	filename := fmt.Sprintf("templates-%s-%s.zip", serialTag, timestamp)

	w.Header().Set("Content-Type", "application/zip")
	w.Header().Set("Content-Disposition", fmt.Sprintf(`attachment; filename="%s"`, filename))

	zipWriter := zip.NewWriter(w)
	defer zipWriter.Close()

	nameCounts := make(map[string]int)
	usedFiles := make(map[string]bool)

	for _, tmpl := range list {
		img, err := h.service.Storage().GetTemplateImage(tmpl.ID)
		if err != nil {
			continue
		}
		entryName := resolveZipEntryName(tmpl.Name, tmpl.ID, nameCounts, usedFiles)
		writer, err := zipWriter.Create(entryName)
		if err != nil {
			continue
		}
		_ = png.Encode(writer, img)
	}
}

// handleImportTemplates POST /api/templates/import
func (h *Handler) handleImportTemplates(w http.ResponseWriter, r *http.Request) {
	if err := r.ParseMultipartForm(64 << 20); err != nil {
		writeError(w, http.StatusBadRequest, "invalid_form", err.Error())
		return
	}

	file, _, err := r.FormFile("file")
	if err != nil {
		file, _, err = r.FormFile("zip")
	}
	if err != nil {
		writeError(w, http.StatusBadRequest, "missing_file", "ZIP file is required in 'file' form field")
		return
	}
	defer file.Close()

	serial := strings.TrimSpace(r.FormValue("serial"))
	serial = NormalizeSerial(serial)

	zipBytes, err := io.ReadAll(file)
	if err != nil {
		writeError(w, http.StatusBadRequest, "read_file_failed", err.Error())
		return
	}

	zipReader, err := zip.NewReader(bytes.NewReader(zipBytes), int64(len(zipBytes)))
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid_zip", "failed to read zip archive: "+err.Error())
		return
	}

	imported := make([]*Template, 0)
	for _, f := range zipReader.File {
		if f.FileInfo().IsDir() {
			continue
		}
		name := f.Name
		if strings.HasPrefix(name, "__MACOSX/") || strings.Contains(name, "/__MACOSX/") {
			continue
		}
		baseName := filepath.Base(name)
		if strings.HasPrefix(baseName, ".") {
			continue
		}
		if !strings.EqualFold(filepath.Ext(baseName), ".png") {
			continue
		}

		tmplName := strings.TrimSuffix(baseName, filepath.Ext(baseName))
		tmplName = strings.TrimSpace(tmplName)
		if tmplName == "" {
			continue
		}

		rc, err := f.Open()
		if err != nil {
			continue
		}
		img, err := png.Decode(rc)
		_ = rc.Close()
		if err != nil {
			continue
		}

		grayscale := true
		enabled := true
		req := CreateTemplateRequest{
			Name:      tmplName,
			Serial:    serial,
			Threshold: DefaultThreshold,
			Grayscale: &grayscale,
			Enabled:   &enabled,
		}

		tmpl, err := h.service.Storage().CreateTemplate(req, img)
		if err != nil {
			continue
		}
		imported = append(imported, tmpl)
	}

	writeJSON(w, http.StatusOK, map[string]interface{}{
		"success":   true,
		"imported":  len(imported),
		"templates": imported,
	})
}

// resolveZipEntryName calculates a unique filename inside the zip for a template.
func resolveZipEntryName(rawName, fallbackID string, nameCounts map[string]int, usedFiles map[string]bool) string {
	baseName := sanitizeFilename(rawName)
	if strings.HasSuffix(strings.ToLower(baseName), ".png") {
		baseName = baseName[:len(baseName)-4]
	}
	if baseName == "" {
		baseName = sanitizeFilename(fallbackID)
	}
	if baseName == "" {
		baseName = "template"
	}

	entryName := baseName + ".png"
	if _, exists := nameCounts[baseName]; exists || usedFiles[entryName] {
		idx := nameCounts[baseName] + 1
		if idx < 2 {
			idx = 2
		}
		for {
			candidate := fmt.Sprintf("%s_%d.png", baseName, idx)
			if !usedFiles[candidate] {
				entryName = candidate
				nameCounts[baseName] = idx
				break
			}
			idx++
		}
	} else {
		nameCounts[baseName] = 1
	}
	usedFiles[entryName] = true
	return entryName
}

// sanitizeFilename strips or replaces characters unsafe for filenames and HTTP headers.
func sanitizeFilename(name string) string {
	var b strings.Builder
	for _, r := range name {
		switch r {
		case '/', '\\', ':', '*', '?', '"', '<', '>', '|', '\r', '\n', '\t', 0:
			b.WriteRune('_')
		default:
			if r < 32 {
				b.WriteRune('_')
			} else {
				b.WriteRune(r)
			}
		}
	}
	return strings.TrimSpace(b.String())
}

// handleClickTemplate POST /api/templates/click
func (h *Handler) handleClickTemplate(w http.ResponseWriter, r *http.Request) {
	var req ClickRequest

	if strings.Contains(r.Header.Get("Content-Type"), "application/json") {
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			writeError(w, http.StatusBadRequest, "invalid_json", err.Error())
			return
		}
	} else {
		_ = r.ParseForm()
		req.Serial = r.FormValue("serial")
		req.Name = r.FormValue("name")
		req.TemplateID = r.FormValue("template_id")
		if r.FormValue("random_offset") == "true" || r.FormValue("random_offset") == "1" {
			req.RandomOffset = true
		}
		req.Mode = r.FormValue("mode")
		if r.FormValue("humanize") == "true" || r.FormValue("humanize") == "1" {
			req.Humanize = true
		}
		if dStr := r.FormValue("duration_ms"); dStr != "" {
			if d, err := strconv.ParseInt(dStr, 10, 64); err == nil {
				req.DurationMS = d
			}
		}
	}

	if req.Serial == "" {
		req.Serial = r.URL.Query().Get("serial")
	}
	if req.Serial == "" {
		writeError(w, http.StatusBadRequest, "missing_serial", "serial parameter is required")
		return
	}

	target := req.Name
	if target == "" {
		target = req.TemplateID
	}
	if target == "" {
		writeError(w, http.StatusBadRequest, "missing_template_name", "name or template_id is required")
		return
	}

	opts := ClickOptions{
		RandomOffset: req.RandomOffset,
		Mode:         req.Mode,
		Humanize:     req.Humanize,
		DurationMS:   req.DurationMS,
	}

	res, err := h.service.ClickTemplate(r.Context(), req.Serial, target, opts)
	if err != nil {
		if errors.Is(err, ErrTemplateNotFound) {
			writeError(w, http.StatusNotFound, "template_not_found", fmt.Sprintf("template %q not found", target))
			return
		}
		if errors.Is(err, ErrTemplateNotMatched) {
			writeError(w, http.StatusUnprocessableEntity, "template_not_matched", fmt.Sprintf("template %q not matched on screen", target))
			return
		}
		writeError(w, http.StatusInternalServerError, "click_failed", err.Error())
		return
	}

	writeJSON(w, http.StatusOK, res)
}

