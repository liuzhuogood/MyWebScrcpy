package ocr

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"net/http"
	"strconv"
	"strings"
)

type detectInput struct {
	Request       DetectRequest
	Image         []byte
	Columns, Rows int
}

func readDetectInput(w http.ResponseWriter, r *http.Request, allowQuery bool) (detectInput, error) {
	var input detectInput
	req := &input.Request
	contentType := r.Header.Get("Content-Type")
	r.Body = http.MaxBytesReader(w, r.Body, 32<<20)
	switch {
	case strings.HasPrefix(contentType, "multipart/form-data"):
		if err := r.ParseMultipartForm(32 << 20); err != nil {
			return input, fmt.Errorf("解析上传表单失败: %w", err)
		}
		defer r.MultipartForm.RemoveAll()
		file, _, err := r.FormFile("file")
		if err != nil && !errors.Is(err, http.ErrMissingFile) {
			return input, fmt.Errorf("读取上传图片失败: %w", err)
		}
		if err == nil {
			defer file.Close()
			input.Image, err = io.ReadAll(file)
			if err != nil {
				return input, err
			}
			if len(input.Image) == 0 {
				return input, errors.New("图片数据为空")
			}
		}
		req.Serial = r.FormValue("serial")
		req.Engine = r.FormValue("engine")
		req.Mark = r.FormValue("mark") == "true" || r.FormValue("mark") == "1"
		req.MarkDurationMS, _ = strconv.ParseInt(r.FormValue("mark_duration_ms"), 10, 64)
		if r.Form.Has("stabilization_ms") {
			req.StabilizationMS, err = strconv.ParseInt(r.FormValue("stabilization_ms"), 10, 64)
			if err != nil {
				return input, errors.New("stabilization_ms 必须是整数毫秒")
			}
		}
		if kw := r.FormValue("keywords"); kw != "" {
			req.Keywords = strings.Split(kw, ",")
		}
		if bbox := r.FormValue("bbox"); bbox != "" {
			if err := json.Unmarshal([]byte(bbox), &req.BBox); err != nil {
				return input, errors.New("bbox 必须是归一化坐标数组")
			}
		}
		input.Columns, _ = strconv.Atoi(r.FormValue("columns"))
		input.Rows, _ = strconv.Atoi(r.FormValue("rows"))
	case strings.Contains(contentType, "application/json"):
		var body struct {
			DetectRequest
			Stabilization json.RawMessage `json:"stabilization_ms"`
			Columns       int             `json:"columns"`
			Rows          int             `json:"rows"`
		}
		decoder := json.NewDecoder(r.Body)
		if err := decoder.Decode(&body); err != nil {
			return input, fmt.Errorf("JSON 格式错误: %w", err)
		}
		if err := decoder.Decode(new(any)); err != io.EOF {
			return input, errors.New("JSON 请求只能包含一个对象")
		}
		*req = body.DetectRequest
		input.Columns = body.Columns
		input.Rows = body.Rows
		if len(body.Stabilization) > 0 {
			if bytes.Equal(bytes.TrimSpace(body.Stabilization), []byte("null")) {
				return input, errors.New("stabilization_ms 不能是 null")
			}
			if err := json.Unmarshal(body.Stabilization, &req.StabilizationMS); err != nil {
				return input, errors.New("stabilization_ms 必须是整数毫秒")
			}
		}
	case allowQuery:
		req.Serial = r.URL.Query().Get("serial")
		req.Engine = r.URL.Query().Get("engine")
		input.Columns, _ = strconv.Atoi(r.URL.Query().Get("columns"))
		input.Rows, _ = strconv.Atoi(r.URL.Query().Get("rows"))
		if r.URL.Query().Has("stabilization_ms") {
			var err error
			req.StabilizationMS, err = strconv.ParseInt(r.URL.Query().Get("stabilization_ms"), 10, 64)
			if err != nil {
				return input, errors.New("stabilization_ms 必须是整数毫秒")
			}
		}
	default:
		return input, errors.New("需要 multipart/form-data 或 application/json")
	}
	if err := validateDetectRequest(*req); err != nil {
		return input, err
	}
	if len(input.Image) == 0 && strings.TrimSpace(req.Serial) == "" {
		return input, errors.New("未上传 file 时必须提供 serial")
	}
	if len(input.Image) > 0 && req.StabilizationMS > 0 {
		return input, errors.New("stabilization_ms 大于 0 时不能上传 file，请提供 serial 连续采样")
	}
	return input, nil
}
func (s *Service) runDetectInput(r *http.Request, input detectInput) (*DetectResponse, error) {
	if len(input.Image) > 0 {
		return s.Detect(r.Context(), input.Image, input.Request)
	}
	return s.DetectDevice(r.Context(), input.Request)
}
func writeDetectError(w http.ResponseWriter, err error) {
	code := http.StatusBadGateway
	if errors.Is(err, errOCRBusy) {
		code = http.StatusTooManyRequests
	} else if errors.Is(err, context.DeadlineExceeded) {
		code = http.StatusGatewayTimeout
	} else if errors.Is(err, context.Canceled) {
		code = http.StatusRequestTimeout
	}
	log.Printf("[OCR] 识别失败 status=%d error=%v", code, err)
	writeJSON(w, code, map[string]string{"error": err.Error()})
}
