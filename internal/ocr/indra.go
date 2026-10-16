package ocr

import (
	"context"
	"errors"
	"fmt"
	"math"
	"net/http"
)

func (s *Service) SetIndraControl(control func(bool, func() error) error) { s.indra.control = control }
func (s *Service) SetIndraPort(port int)                                  { s.indra.SetPaddlePort(port) }
func (s *Service) StartIndra(ctx context.Context) error                   { return s.indra.StartPaddle(ctx) }
func (s *Service) StopIndra() error                                       { return s.indra.StopPaddle() }

func (s *Service) handleIndraStart(w http.ResponseWriter, r *http.Request) {
	s.controlIndra(w, true, func() error { return s.StartIndra(r.Context()) })
}
func (s *Service) handleIndraStop(w http.ResponseWriter, r *http.Request) {
	s.controlIndra(w, false, s.StopIndra)
}
func (s *Service) controlIndra(w http.ResponseWriter, enabled bool, action func() error) {
	var err error
	if s.indra.control != nil {
		err = s.indra.control(enabled, action)
	} else {
		err = action()
	}
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}
	writeJSON(w, http.StatusOK, s.Status())
}

// Indra's contract uses pixel coordinates, including boxes smaller than one pixel.
func validateIndraResult(result map[string]any) error {
	if _, ok := result["ocr_result"].(string); !ok {
		return errors.New("Indra 响应缺少 ocr_result")
	}
	boxes, ok := result["ocr_boxes"].([]any)
	if !ok {
		return errors.New("Indra 响应缺少 ocr_boxes")
	}
	for _, raw := range boxes {
		box, ok := raw.(map[string]any)
		if !ok {
			return errors.New("Indra 文字框结构无效")
		}
		if _, ok := box["text"].(string); !ok {
			return errors.New("Indra 文字框缺少 text")
		}
		for _, key := range []string{"x", "y", "w", "h", "confidence"} {
			value, ok := box[key].(float64)
			if !ok || math.IsNaN(value) || math.IsInf(value, 0) || value < 0 {
				return fmt.Errorf("Indra 文字框 %s 无效", key)
			}
			if (key == "w" || key == "h") && value <= 0 {
				return errors.New("Indra 文字框尺寸无效")
			}
			if key == "confidence" && value > 1 {
				return errors.New("Indra 文字框置信度无效")
			}
		}
	}
	return nil
}
