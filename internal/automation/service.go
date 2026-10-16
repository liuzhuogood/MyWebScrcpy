package automation

import (
	"bytes"
	"context"
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"image"
	"image/draw"
	"image/png"
	"sort"
	"strconv"
	"strings"
	"time"

	"mywebscrcpy/internal/action"
	"mywebscrcpy/internal/dependencies"
	"mywebscrcpy/internal/inputconfig"
	"mywebscrcpy/internal/ocr"
	"mywebscrcpy/internal/template"
	"mywebscrcpy/internal/ws"
)

func newActionID() string {
	var b [12]byte
	if _, err := rand.Read(b[:]); err == nil {
		return "act_" + hex.EncodeToString(b[:])
	}
	return fmt.Sprintf("act_%d", time.Now().UnixNano())
}

// HubSessionProvider 抽象 Hub 的会话操作
type HubSessionProvider interface {
	SubmitAction(ctx context.Context, serial string, req action.Request) (action.Result, error)
	GetDeviceResolution(serial string) (int, int, bool)
}

// Service 自动化与视觉服务实现
type Service struct {
	hub             *ws.Hub
	templateStorage *template.Storage
	ocrService      *ocr.Service
	inputConfig     *inputconfig.Store
	adbPath         string
}

// NewService 创建自动化服务实例
func NewService(
	hub *ws.Hub,
	templateStorage *template.Storage,
	ocrService *ocr.Service,
	inputConfig *inputconfig.Store,
	adbPath string,
) *Service {
	return &Service{
		hub:             hub,
		templateStorage: templateStorage,
		ocrService:      ocrService,
		inputConfig:     inputConfig,
		adbPath:         adbPath,
	}
}

// resolveMode 获取动作最终生效的注入模式
func (s *Service) resolveMode(requested string) string {
	if s.hub != nil {
		return s.hub.ResolveActionMode(requested)
	}
	if requested != "" && action.ValidMode(requested) {
		return requested
	}
	if s.inputConfig != nil {
		if mode := s.inputConfig.Mode(); mode != "" {
			return mode
		}
	}
	return action.ModeSDK
}

// GetDeviceStatus 获取设备状态及当前系统点击模式
func (s *Service) GetDeviceStatus(serial string) (DeviceStatusResponse, error) {
	if serial == "" {
		return DeviceStatusResponse{}, fmt.Errorf("missing serial")
	}

	mode := s.resolveMode("")
	res := DeviceStatusResponse{
		Status:    "ready",
		Serial:    serial,
		TouchMode: mode,
	}

	// 尝试通过 ADB 获取屏幕物理尺寸
	out, err := dependencies.Command(s.adbPath, "-s", serial, "shell", "wm", "size").Output()
	if err == nil {
		lines := strings.Split(string(out), "\n")
		for _, line := range lines {
			line = strings.TrimSpace(line)
			if strings.HasPrefix(line, "Physical size:") || strings.HasPrefix(line, "Override size:") {
				parts := strings.Split(line, ":")
				if len(parts) == 2 {
					wh := strings.Split(strings.TrimSpace(parts[1]), "x")
					if len(wh) == 2 {
						w, _ := strconv.Atoi(wh[0])
						h, _ := strconv.Atoi(wh[1])
						if w > 0 && h > 0 {
							res.Width = w
							res.Height = h
						}
					}
				}
			}
		}
	}

	return res, nil
}

// CaptureFrame 截取当前设备屏幕画面
func (s *Service) CaptureFrame(ctx context.Context, serial string) (image.Image, []byte, error) {
	if serial == "" {
		return nil, nil, fmt.Errorf("missing serial")
	}

	cmd := dependencies.Command(s.adbPath, "-s", serial, "exec-out", "screencap", "-p")
	out, err := cmd.Output()
	if err != nil {
		return nil, nil, fmt.Errorf("adb screencap 失败: %w", err)
	}

	img, err := png.Decode(bytes.NewReader(out))
	if err != nil {
		return nil, nil, fmt.Errorf("解码屏幕截图失败: %w", err)
	}

	return img, out, nil
}

// executeAction 执行动作，优先通过 Hub 投屏会话执行；若投屏未连接则通过 ADB 命令行保底执行
func (s *Service) executeAction(ctx context.Context, serial string, req action.Request) (ActionResult, error) {
	started := time.Now()
	requestID := req.RequestID
	if requestID == "" {
		requestID = newActionID()
	}
	req.RequestID = requestID
	req.DeviceID = serial

	mode := s.resolveMode(req.Mode)
	req.Mode = mode

	if req.ExpiresMS <= 0 {
		req.ExpiresMS = 3000
	}

	// 1. 尝试从 Hub 获取活跃投屏会话执行
	if s.hub != nil {
		actionCtx, cancel := context.WithTimeout(ctx, time.Duration(req.ExpiresMS)*time.Millisecond)
		res, err := s.hub.SubmitAction(actionCtx, serial, req)
		cancel()
		if err == nil {
			elapsed := time.Since(started).Milliseconds()
			return ActionResult{
				RequestID: requestID,
				Status:    map[bool]string{true: "executed", false: "failed"}[res.Executed],
				Executed:  res.Executed,
				Mode:      mode,
				ErrorCode: res.ErrorCode,
				ElapsedMS: elapsed,
			}, nil
		}
	}

	// 2. 没有投屏会话时，通过 ADB 命令行保底执行
	status, err := s.executeActionViaADB(serial, req)
	elapsed := time.Since(started).Milliseconds()
	if err != nil {
		return ActionResult{
			RequestID: requestID,
			Status:    "failed",
			Executed:  false,
			Mode:      mode,
			ErrorCode: err.Error(),
			ElapsedMS: elapsed,
		}, err
	}

	return ActionResult{
		RequestID: requestID,
		Status:    status,
		Executed:  status == "executed",
		Mode:      mode,
		ElapsedMS: elapsed,
	}, nil
}

// executeActionViaADB 使用 adb shell input 执行
func (s *Service) executeActionViaADB(serial string, req action.Request) (string, error) {
	// 获取设备分辨率以便将 0~1 转换为绝对像素
	status, err := s.GetDeviceStatus(serial)
	if err != nil || status.Width <= 0 || status.Height <= 0 {
		return "failed", fmt.Errorf("无法获取设备分辨率以计算坐标")
	}

	switch req.Action {
	case "tap":
		px := int(req.X * float64(status.Width))
		py := int(req.Y * float64(status.Height))
		err := dependencies.Command(s.adbPath, "-s", serial, "shell", "input", "tap", strconv.Itoa(px), strconv.Itoa(py)).Run()
		if err != nil {
			return "failed", err
		}
		return "executed", nil

	case "swipe":
		x1 := int(req.X * float64(status.Width))
		y1 := int(req.Y * float64(status.Height))
		x2 := int(req.X2 * float64(status.Width))
		y2 := int(req.Y2 * float64(status.Height))
		duration := req.DurationMS
		if duration <= 0 {
			duration = 300
		}
		err := dependencies.Command(s.adbPath, "-s", serial, "shell", "input", "swipe",
			strconv.Itoa(x1), strconv.Itoa(y1), strconv.Itoa(x2), strconv.Itoa(y2), strconv.FormatInt(duration, 10)).Run()
		if err != nil {
			return "failed", err
		}
		return "executed", nil

	case "key":
		err := dependencies.Command(s.adbPath, "-s", serial, "shell", "input", "keyevent", strconv.FormatUint(uint64(req.Keycode), 10)).Run()
		if err != nil {
			return "failed", err
		}
		return "executed", nil

	case "text":
		err := dependencies.Command(s.adbPath, "-s", serial, "shell", "input", "text", req.Text).Run()
		if err != nil {
			return "failed", err
		}
		return "executed", nil

	default:
		return "failed", fmt.Errorf("不支持的 ADB 动作类型: %s", req.Action)
	}
}

// Tap 执行点击
func (s *Service) Tap(ctx context.Context, req TapRequest) (ActionResult, error) {
	if req.X < 0 || req.X > 1 || req.Y < 0 || req.Y > 1 {
		return ActionResult{}, fmt.Errorf("坐标无效: (%.4f, %.4f)", req.X, req.Y)
	}
	return s.executeAction(ctx, req.Serial, action.Request{
		Action:    "tap",
		X:         req.X,
		Y:         req.Y,
		ExpiresMS: req.ExpiresMS,
		Mode:      req.Mode,
		Humanize:  req.Humanize,
	})
}

// Swipe 执行滑动
func (s *Service) Swipe(ctx context.Context, req SwipeRequest) (ActionResult, error) {
	x, y, ok := req.GetStartPoint()
	if !ok {
		return ActionResult{}, fmt.Errorf("滑动必须提供起点坐标 (x, y 或 x1, y1)")
	}
	if x < 0 || x > 1 || y < 0 || y > 1 || req.X2 < 0 || req.X2 > 1 || req.Y2 < 0 || req.Y2 > 1 {
		return ActionResult{}, fmt.Errorf("坐标无效: 起点(%.4f, %.4f), 终点(%.4f, %.4f)", x, y, req.X2, req.Y2)
	}

	return s.executeAction(ctx, req.Serial, action.Request{
		Action:     "swipe",
		X:          x,
		Y:          y,
		X2:         req.X2,
		Y2:         req.Y2,
		ExpiresMS:  req.ExpiresMS,
		Mode:       req.Mode,
		Humanize:   req.Humanize,
		DurationMS: req.DurationMS,
	})
}

// Event 执行系统按键事件
func (s *Service) Event(ctx context.Context, req EventRequest) (ActionResult, error) {
	code, ok := NamedEvents[req.Event]
	if !ok {
		return ActionResult{}, fmt.Errorf("不支持的事件名称: %s", req.Event)
	}
	return s.executeAction(ctx, req.Serial, action.Request{
		Action:    "key",
		Keycode:   code,
		ExpiresMS: req.ExpiresMS,
	})
}

// SendKey 发送键盘按键或文本
func (s *Service) SendKey(ctx context.Context, req SendKeyRequest) (ActionResult, error) {
	if req.Text != "" {
		return s.executeAction(ctx, req.Serial, action.Request{
			Action:    "text",
			Text:      req.Text,
			ExpiresMS: req.ExpiresMS,
		})
	}
	if req.Keycode > 0 {
		return s.executeAction(ctx, req.Serial, action.Request{
			Action:    "key",
			Keycode:   req.Keycode,
			ExpiresMS: req.ExpiresMS,
		})
	}
	if len(req.Keycodes) > 0 {
		var lastResult ActionResult
		for _, code := range req.Keycodes {
			res, err := s.executeAction(ctx, req.Serial, action.Request{
				Action:    "key",
				Keycode:   code,
				ExpiresMS: req.ExpiresMS,
			})
			if err != nil || !res.Executed {
				return res, err
			}
			lastResult = res
		}
		return lastResult, nil
	}
	return ActionResult{}, fmt.Errorf("必须提供 text、keycode 或 keycodes")
}

// evaluateCondition 对单张图像执行条件评估
func (s *Service) evaluateCondition(ctx context.Context, img image.Image, rawPNG []byte, cond Condition, kind Kind, serial string) (bool, EvaluationResult, error) {
	switch kind {
	case KindPixel:
		matched, points, err := EvaluatePixelPoints(img, cond.Points, cond.PixelMode)
		if err != nil {
			return false, EvaluationResult{}, err
		}
		return matched, EvaluationResult{Points: points}, nil

	case KindTemplate:
		names := cond.TemplateNames
		if cond.TemplateName != "" {
			names = append([]string{cond.TemplateName}, names...)
		}
		if len(names) == 0 {
			return false, EvaluationResult{}, fmt.Errorf("template_names 不能为空")
		}

		threshold := cond.Threshold
		if threshold <= 0 {
			threshold = 0.88
		}

		var allObjects []MatchedObject
		allMatched := true
		anyMatched := false

		for _, name := range names {
			var tmplImg image.Image
			var err error

			if s.templateStorage != nil {
				asset, findErr := s.templateStorage.FindTemplate(serial, name)
				if findErr == nil && asset != nil {
					tmplImg, err = s.templateStorage.GetTemplateImage(asset.ID)
					if asset.Threshold > 0 {
						threshold = asset.Threshold
					}
				} else {
					err = findErr
				}
			}

			if tmplImg == nil || err != nil {
				return false, EvaluationResult{}, fmt.Errorf("找不到模板: %s", name)
			}

			results, matchErr := MatchTemplatePureGo(img, tmplImg, threshold)
			if matchErr != nil {
				return false, EvaluationResult{}, fmt.Errorf("匹配模板 %s 出错: %w", name, matchErr)
			}

			if len(results) > 0 {
				anyMatched = true
				for _, r := range results {
					allObjects = append(allObjects, MatchedObject{
						Label:      name,
						Confidence: r.Confidence,
						X:          r.X,
						Y:          r.Y,
						W:          r.W,
						H:          r.H,
						Kind:       "template",
					})
				}
			} else {
				allMatched = false
			}
		}

		matched := anyMatched
		if cond.MatchMode == MatchModeAll {
			matched = allMatched
		}
		return matched, EvaluationResult{Objects: allObjects}, nil

	case KindText:
		if len(cond.Keywords) == 0 {
			return false, EvaluationResult{}, fmt.Errorf("keywords 不能为空")
		}
		if s.ocrService == nil {
			return false, EvaluationResult{}, fmt.Errorf("OCR 服务未初始化")
		}

		var imgData = rawPNG
		if len(imgData) == 0 {
			var buf bytes.Buffer
			if err := png.Encode(&buf, img); err != nil {
				return false, EvaluationResult{}, fmt.Errorf("图像编码失败: %w", err)
			}
			imgData = buf.Bytes()
		}

		ocrReq := ocr.DetectRequest{
			Serial:   serial,
			BBox:     cond.BBox,
			Keywords: cond.Keywords,
			Engine:   cond.Engine,
		}

		res, err := s.ocrService.Detect(ctx, imgData, ocrReq)
		if err != nil {
			return false, EvaluationResult{}, fmt.Errorf("OCR 识别失败: %w", err)
		}

		var objects []MatchedObject
		for _, b := range res.Boxes {
			objects = append(objects, MatchedObject{
				Label:      b.Text,
				Confidence: b.Confidence,
				X:          b.X,
				Y:          b.Y,
				W:          b.W,
				H:          b.H,
				Kind:       "text",
			})
		}

		matched := res.Matched
		return matched, EvaluationResult{
			Objects:        objects,
			Text:           res.Text,
			Engine:         res.Engine,
			FallbackReason: res.FallbackReason,
		}, nil

	default:
		return false, EvaluationResult{}, fmt.Errorf("不支持的条件类型: %s", kind)
	}
}

// Check 执行即时单帧检查
func (s *Service) Check(ctx context.Context, req CheckRequest) (CheckResponse, error) {
	started := time.Now()
	img, raw, err := s.CaptureFrame(ctx, req.Serial)
	if err != nil {
		return CheckResponse{}, err
	}

	matched, evalRes, err := s.evaluateCondition(ctx, img, raw, req.Condition, req.Kind, req.Serial)
	if err != nil {
		return CheckResponse{}, err
	}

	status := "not_matched"
	if matched {
		status = "matched"
	}

	return CheckResponse{
		Status:    status,
		Matched:   matched,
		Kind:      req.Kind,
		ElapsedMS: time.Since(started).Milliseconds(),
		Result:    evalRes,
	}, nil
}

// Wait 执行限时持续轮询等待
func (s *Service) Wait(ctx context.Context, req WaitRequest) (WaitResponse, error) {
	started := time.Now()
	timeout := req.TimeoutMS
	if timeout <= 0 {
		timeout = 10000
	}
	pollInterval := req.PollIntervalMS
	if pollInterval <= 0 {
		pollInterval = 100
	}

	deadline := started.Add(time.Duration(timeout) * time.Millisecond)
	var lastEval EvaluationResult

	for time.Now().Before(deadline) {
		img, raw, err := s.CaptureFrame(ctx, req.Serial)
		if err == nil {
			matched, evalRes, evalErr := s.evaluateCondition(ctx, img, raw, req.Condition, req.Kind, req.Serial)
			if evalErr == nil {
				lastEval = evalRes
				if matched {
					return WaitResponse{
						Status:    "matched",
						Matched:   true,
						Kind:      req.Kind,
						ElapsedMS: time.Since(started).Milliseconds(),
						Result:    evalRes,
					}, nil
				}
			}
		}

		remaining := time.Until(deadline)
		if remaining <= 0 {
			break
		}
		sleepDuration := time.Duration(pollInterval) * time.Millisecond
		if sleepDuration > remaining {
			sleepDuration = remaining
		}
		time.Sleep(sleepDuration)
	}

	return WaitResponse{
		Status:    "timeout",
		Matched:   false,
		Kind:      req.Kind,
		ElapsedMS: time.Since(started).Milliseconds(),
		Result:    lastEval,
	}, nil
}

// TapWhen 满足条件后自动点击目标
func (s *Service) TapWhen(ctx context.Context, req TapWhenRequest) (TapWhenResponse, error) {
	waitReq := WaitRequest{
		Serial:         req.Serial,
		Kind:           req.Kind,
		Condition:      req.Condition,
		TimeoutMS:      req.TimeoutMS,
		PollIntervalMS: req.PollIntervalMS,
	}

	waitRes, err := s.Wait(ctx, waitReq)
	if err != nil {
		return TapWhenResponse{}, err
	}
	if !waitRes.Matched {
		return TapWhenResponse{
			Status:    waitRes.Status,
			Matched:   false,
			Kind:      req.Kind,
			ElapsedMS: waitRes.ElapsedMS,
			Result:    waitRes.Result,
		}, nil
	}

	var clickPoint Point
	if req.Kind == KindPixel {
		if req.Tap == nil {
			return TapWhenResponse{}, fmt.Errorf("像素条件点击必须提供显式 tap 坐标")
		}
		clickPoint = *req.Tap
	} else {
		objects := waitRes.Result.Objects
		if len(objects) == 0 {
			return TapWhenResponse{}, fmt.Errorf("没有找到可点击的匹配目标")
		}
		sort.Slice(objects, func(i, j int) bool {
			return objects[i].Confidence > objects[j].Confidence
		})

		idx := req.TargetIndex
		if idx < 0 || idx >= len(objects) {
			idx = 0
		}
		target := objects[idx]
		// 计算包围盒中心点
		clickPoint = Point{
			X: target.X + target.W/2.0,
			Y: target.Y + target.H/2.0,
		}
	}

	actionRes, err := s.Tap(ctx, TapRequest{
		Serial:    req.Serial,
		X:         clickPoint.X,
		Y:         clickPoint.Y,
		ExpiresMS: req.ExpiresMS,
		Mode:      req.Mode,
		Humanize:  req.Humanize,
	})
	if err != nil {
		return TapWhenResponse{}, fmt.Errorf("触发点击失败: %w", err)
	}

	return TapWhenResponse{
		Status:    "matched",
		Matched:   true,
		Kind:      req.Kind,
		ElapsedMS: waitRes.ElapsedMS + actionRes.ElapsedMS,
		TapPoint:  &clickPoint,
		Action:    &actionRes,
		Result:    waitRes.Result,
	}, nil
}

// RegionScreenshot 截取指定区域图片
func (s *Service) RegionScreenshot(ctx context.Context, req RegionScreenshotRequest) ([]byte, error) {
	img, _, err := s.CaptureFrame(ctx, req.Serial)
	if err != nil {
		return nil, err
	}

	bounds := img.Bounds()
	imgW := bounds.Dx()
	imgH := bounds.Dy()

	if req.X < 0 || req.Y < 0 || req.Width <= 0 || req.Height <= 0 ||
		req.X+req.Width > imgW || req.Y+req.Height > imgH {
		return nil, fmt.Errorf("截图区域超出范围: (%d,%d,%d,%d) 在图像尺寸 (%dx%d) 中",
			req.X, req.Y, req.Width, req.Height, imgW, imgH)
	}

	rect := image.Rect(req.X, req.Y, req.X+req.Width, req.Y+req.Height)
	subImg := image.NewRGBA(image.Rect(0, 0, req.Width, req.Height))
	draw.Draw(subImg, subImg.Bounds(), img, rect.Min, draw.Src)

	var buf bytes.Buffer
	if err := png.Encode(&buf, subImg); err != nil {
		return nil, fmt.Errorf("编码裁剪截图失败: %w", err)
	}

	return buf.Bytes(), nil
}
