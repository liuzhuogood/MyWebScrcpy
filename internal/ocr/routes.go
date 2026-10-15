package ocr

import (
	"encoding/json"
	"io"
	"net/http"
	"strconv"
	"strings"
)

func (s *Service) RegisterRoutes(mux *http.ServeMux) {
	mux.HandleFunc("POST /api/vision/ocr", s.handleDetect)
	mux.HandleFunc("GET /api/ocr/status", s.handleStatus)
	mux.HandleFunc("POST /api/ocr/start", s.handleStart)
	mux.HandleFunc("POST /api/ocr/stop", s.handleStop)
	mux.HandleFunc("POST /api/ocr/config", s.handleSaveConfig)
	mux.HandleFunc("POST /api/ocr/test", s.handleTest)
}

func writeJSON(w http.ResponseWriter, status int, data any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(data)
}

func (s *Service) handleStatus(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, s.Status())
}

func (s *Service) handleStart(w http.ResponseWriter, r *http.Request) {
	if err := s.StartPaddle(r.Context()); err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}
	writeJSON(w, http.StatusOK, s.Status())
}

func (s *Service) handleStop(w http.ResponseWriter, r *http.Request) {
	if err := s.StopPaddle(); err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}
	writeJSON(w, http.StatusOK, s.Status())
}

func (s *Service) handleSaveConfig(w http.ResponseWriter, r *http.Request) {
	var body struct {
		MacOCRURL    string `json:"macocr_url"`
		MacOCRAuth   string `json:"macocr_auth"`
		PaddleOCRURL string `json:"paddleocr_url"`
		Engine       string `json:"engine"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "JSON 格式错误"})
		return
	}

	if err := s.SaveConfig(body.MacOCRURL, body.MacOCRAuth, body.PaddleOCRURL, body.Engine); err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}
	writeJSON(w, http.StatusOK, s.Status())
}

func (s *Service) handleTest(w http.ResponseWriter, r *http.Request) {
	var body struct {
		URL  string `json:"url"`
		Auth string `json:"auth"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "JSON 格式错误"})
		return
	}
	if body.URL == "" {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "url 不能为空"})
		return
	}

	elapsed, err := s.TestConnection(r.Context(), body.URL, body.Auth)
	if err != nil {
		writeJSON(w, http.StatusBadGateway, map[string]any{"error": err.Error(), "elapsed_ms": elapsed})
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"status": "ok", "elapsed_ms": elapsed})
}

func (s *Service) handleDetect(w http.ResponseWriter, r *http.Request) {
	contentType := r.Header.Get("Content-Type")

	var imageBytes []byte
	var req DetectRequest

	if strings.HasPrefix(contentType, "multipart/form-data") {
		// 上传图片模式
		if err := r.ParseMultipartForm(32 << 20); err != nil {
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": "解析上传表单失败: " + err.Error()})
			return
		}

		file, _, err := r.FormFile("file")
		if err != nil {
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": "缺少上传图片文件 (file 字段)"})
			return
		}
		defer file.Close()

		data, err := io.ReadAll(file)
		if err != nil {
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": "读取图片内容失败: " + err.Error()})
			return
		}
		imageBytes = data

		req.Serial = r.FormValue("serial")
		req.Mark = r.FormValue("mark") == "true" || r.FormValue("mark") == "1"
		if d, err := strconv.ParseInt(r.FormValue("mark_duration_ms"), 10, 64); err == nil {
			req.MarkDurationMS = d
		}
		req.Engine = r.FormValue("engine")
		if kw := r.FormValue("keywords"); kw != "" {
			req.Keywords = strings.Split(kw, ",")
		}
		if bboxStr := r.FormValue("bbox"); bboxStr != "" {
			var b []float64
			if err := json.Unmarshal([]byte(bboxStr), &b); err == nil && len(b) == 4 {
				req.BBox = b
			}
		}
	} else {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "需要 multipart/form-data 上传待识别图片"})
		return
	}

	if len(imageBytes) == 0 {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "图片数据为空"})
		return
	}

	res, err := s.Detect(r.Context(), imageBytes, req)
	if err != nil {
		writeJSON(w, http.StatusBadGateway, map[string]string{"error": err.Error()})
		return
	}

	writeJSON(w, http.StatusOK, res)
}
