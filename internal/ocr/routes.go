package ocr

import (
	"encoding/json"
	"net/http"
	"strings"
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
	input, err := readDetectInput(w, r, false)
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": err.Error()})
		return
	}
	res, err := s.runDetectInput(r, input)
	if err != nil {
		writeDetectError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, res)
}

func (s *Service) handleDetectTextArt(w http.ResponseWriter, r *http.Request) {
	input, err := readDetectInput(w, r, true)
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": err.Error()})
		return
	}
	res, err := s.runDetectInput(r, input)
	if err != nil {
		writeDetectError(w, err)
		return
	}
	textArt := RenderBoxesTextArt(res.Boxes, TextArtOptions{Columns: input.Columns, Rows: input.Rows})
	if strings.Contains(r.Header.Get("Accept"), "text/plain") {
		w.Header().Set("Content-Type", "text/plain; charset=utf-8")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(textArt))
		return
	}
	writeJSON(w, http.StatusOK, TextArtResponse{
		Status: res.Status, Matched: res.Matched, TextArt: textArt, Text: res.Text, Boxes: res.Boxes,
		Engine: res.Engine, ElapsedMS: res.ElapsedMS, FallbackReason: res.FallbackReason,
		Samples: res.Samples, EnginesUsed: res.EnginesUsed,
	})
}
