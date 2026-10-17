package ocr

import (
	"encoding/json"
	"io"
	"net/http"
	"strconv"
	"strings"

	"mywebscrcpy/internal/dependencies"
)

func (s *Service) RegisterRoutes(mux *http.ServeMux) {
	mux.HandleFunc("POST /api/vision/ocr", s.handleDetect)
	mux.HandleFunc("POST /api/vision/ocr-text-art", s.handleDetectTextArt)
	mux.HandleFunc("GET /api/ocr/status", s.handleStatus)
	mux.HandleFunc("POST /api/ocr/start", s.handleStart)
	mux.HandleFunc("POST /api/ocr/stop", s.handleStop)
	mux.HandleFunc("POST /api/ocr/indra/start", s.handleIndraStart)
	mux.HandleFunc("POST /api/ocr/indra/stop", s.handleIndraStop)
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
	action := func() error { return s.StartPaddle(r.Context()) }
	var err error
	if s.control != nil {
		err = s.control(true, action)
	} else {
		err = action()
	}
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}
	writeJSON(w, http.StatusOK, s.Status())
}

func (s *Service) handleStop(w http.ResponseWriter, r *http.Request) {
	var err error
	if s.control != nil {
		err = s.control(false, s.StopPaddle)
	} else {
		err = s.StopPaddle()
	}
	if err != nil {
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
		IndraOCRURL  string `json:"indraocr_url"`
		Engine       string `json:"engine"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "JSON 格式错误"})
		return
	}

	if err := s.SaveConfig(body.MacOCRURL, body.MacOCRAuth, body.PaddleOCRURL, body.IndraOCRURL, body.Engine); err != nil {
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

func (s *Service) handleDetectTextArt(w http.ResponseWriter, r *http.Request) {
	contentType := r.Header.Get("Content-Type")

	var imageBytes []byte
	var req DetectRequest
	var cols int
	var rows int

	if strings.HasPrefix(contentType, "multipart/form-data") {
		if err := r.ParseMultipartForm(32 << 20); err != nil {
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": "解析上传表单失败: " + err.Error()})
			return
		}

		file, _, err := r.FormFile("file")
		if err == nil && file != nil {
			defer file.Close()
			data, err := io.ReadAll(file)
			if err != nil {
				writeJSON(w, http.StatusBadRequest, map[string]string{"error": "读取图片内容失败: " + err.Error()})
				return
			}
			imageBytes = data
		}

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
		if c, err := strconv.Atoi(r.FormValue("columns")); err == nil {
			cols = c
		}
		if ro, err := strconv.Atoi(r.FormValue("rows")); err == nil {
			rows = ro
		}
	} else if strings.Contains(contentType, "application/json") {
		var body struct {
			Serial         string    `json:"serial"`
			BBox           []float64 `json:"bbox"`
			Mark           bool      `json:"mark"`
			MarkDurationMS int64     `json:"mark_duration_ms"`
			Keywords       []string  `json:"keywords"`
			Engine         string    `json:"engine"`
			Columns        int       `json:"columns"`
			Rows           int       `json:"rows"`
		}
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": "JSON 格式错误: " + err.Error()})
			return
		}
		req.Serial = body.Serial
		req.BBox = body.BBox
		req.Mark = body.Mark
		req.MarkDurationMS = body.MarkDurationMS
		req.Keywords = body.Keywords
		req.Engine = body.Engine
		cols = body.Columns
		rows = body.Rows
	} else {
		// 支持通过 URL Query 参数指定 serial 等
		req.Serial = r.URL.Query().Get("serial")
		req.Engine = r.URL.Query().Get("engine")
		if c, err := strconv.Atoi(r.URL.Query().Get("columns")); err == nil {
			cols = c
		}
		if ro, err := strconv.Atoi(r.URL.Query().Get("rows")); err == nil {
			rows = ro
		}
	}

	// 若未上传图片但指定了 serial，则调用 ADB 自动截图
	if len(imageBytes) == 0 && req.Serial != "" {
		cmd := dependencies.Command(dependencies.ManagedADB, "-s", req.Serial, "exec-out", "screencap", "-p")
		out, err := cmd.Output()
		if err != nil {
			writeJSON(w, http.StatusBadGateway, map[string]string{"error": "获取设备截图失败: " + err.Error()})
			return
		}
		imageBytes = out
	}

	if len(imageBytes) == 0 {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "缺少图片输入: 请上传 file 或提供 serial 参数"})
		return
	}

	res, err := s.Detect(r.Context(), imageBytes, req)
	if err != nil {
		writeJSON(w, http.StatusBadGateway, map[string]string{"error": err.Error()})
		return
	}

	textArt := RenderBoxesTextArt(res.Boxes, TextArtOptions{
		Columns: cols,
		Rows:    rows,
	})

	// 若客户端 Accept 明确指定为 text/plain，则直接输出纯文本
	if strings.Contains(r.Header.Get("Accept"), "text/plain") {
		w.Header().Set("Content-Type", "text/plain; charset=utf-8")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(textArt))
		return
	}

	writeJSON(w, http.StatusOK, TextArtResponse{
		Status:         res.Status,
		Matched:        res.Matched,
		TextArt:        textArt,
		Text:           res.Text,
		Boxes:          res.Boxes,
		Engine:         res.Engine,
		ElapsedMS:      res.ElapsedMS,
		FallbackReason: res.FallbackReason,
	})
}
