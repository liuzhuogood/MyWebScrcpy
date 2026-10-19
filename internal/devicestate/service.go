package devicestate

import (
	"bytes"
	"context"
	"crypto/rand"
	"encoding/base64"
	"encoding/hex"
	"fmt"
	"image"
	"image/jpeg"
	"log"
	"strings"
	"sync"
	"time"

	"mywebscrcpy/internal/dependencies"
	"mywebscrcpy/internal/ocr"
	"mywebscrcpy/internal/template"
	"mywebscrcpy/internal/uipage"
	"mywebscrcpy/internal/visionframes"
)

const (
	CollectionBudget = 20 * time.Second
	// frameMaxAge 投屏帧的有效期；超期视为投屏已停止或画面不可用。
	frameMaxAge = 5 * time.Second
)

// StreamKeeper 提供设备视频流自主保活与最新帧获取接口
type StreamKeeper interface {
	EnsureFrame(ctx context.Context, serial string, maxWait time.Duration) (visionframes.Frame, error)
	Touch(serial string, ttl time.Duration) error
	IsActive(serial string) bool
}

// Service 提供统一设备总状态采集服务
type Service struct {
	adbPath         string
	uiPage          *uipage.Service
	ocrService      *ocr.Service
	templateStorage *template.Storage
	aiClient        *AIClient
	frames          *visionframes.Store
	streamKeeper    StreamKeeper

	deviceLocks sync.Map // map[string]*sync.Mutex，同设备串行采集
}

func NewService(
	adbPath string,
	uiPage *uipage.Service,
	ocrService *ocr.Service,
	templateStorage *template.Storage,
	aiClient *AIClient,
	frames *visionframes.Store,
) *Service {
	return &Service{
		adbPath:         adbPath,
		uiPage:          uiPage,
		ocrService:      ocrService,
		templateStorage: templateStorage,
		aiClient:        aiClient,
		frames:          frames,
	}
}

// SetStreamKeeper 挂载服务端视频流保活管理器。
func (s *Service) SetStreamKeeper(k StreamKeeper) {
	s.streamKeeper = k
}

func (s *Service) lockChanForDevice(serial string) chan struct{} {
	v, _ := s.deviceLocks.LoadOrStore(serial, make(chan struct{}, 1))
	return v.(chan struct{})
}

func (s *Service) ensureAwakeAndUnlocked(ctx context.Context, serial string) {
	if s.adbPath == "" || serial == "" {
		return
	}
	wakeCtx, wakeCancel := context.WithTimeout(ctx, 2*time.Second)
	defer wakeCancel()

	// 1. 唤醒屏幕 (224 = KEYCODE_WAKEUP)
	wakeCmd := dependencies.CommandContext(wakeCtx, s.adbPath, "-s", serial, "shell", "input", "keyevent", "224")
	if err := wakeCmd.Run(); err != nil && wakeCtx.Err() == nil {
		log.Printf("[DeviceState] ADB 唤醒屏幕重试/忽略 serial=%s: %v", serial, err)
	}

	// 2. 解除锁屏
	dismissCmd := dependencies.CommandContext(wakeCtx, s.adbPath, "-s", serial, "shell", "wm", "dismiss-keyguard")
	if err := dismissCmd.Run(); err != nil && wakeCtx.Err() == nil {
		// 备用解锁 (82 = KEYCODE_MENU)
		menuCmd := dependencies.CommandContext(wakeCtx, s.adbPath, "-s", serial, "shell", "input", "keyevent", "82")
		_ = menuCmd.Run()
	}
}

// CollectState 执行单轮设备状态采集，预算 20 秒，同设备串行
func (s *Service) CollectState(ctx context.Context, serial string, templateIDs []string) (*DeviceState, error) {
	if serial == "" {
		return nil, fmt.Errorf("missing device serial")
	}

	collectCtx, cancel := context.WithTimeout(ctx, CollectionBudget)
	defer cancel()

	sem := s.lockChanForDevice(serial)
	select {
	case sem <- struct{}{}:
		defer func() { <-sem }()
	case <-collectCtx.Done():
		return nil, fmt.Errorf("设备采集等待锁超时或被取消: %w", collectCtx.Err())
	}

	// 0. 采集前主动唤醒并解锁屏幕
	s.ensureAwakeAndUnlocked(collectCtx, serial)

	now := time.Now().UnixMilli()
	obsID := fmt.Sprintf("obs-%d-%s", now, randomHex(4))

	state := &DeviceState{
		ObservationID: obsID,
		Serial:        serial,
		Timestamp:     now,
		DeviceInfo:    make(map[string]any),
	}

	sources := make(map[string]SourceStatus)
	var partialFailures []string

	// 1. 采集前页面上下文
	var prePage *uipage.PageInfo
	if s.uiPage != nil {
		p, err := s.uiPage.FetchPageInfo(collectCtx, serial)
		if err != nil {
			partialFailures = append(partialFailures, fmt.Sprintf("获取初始页面信息失败: %v", err))
		} else {
			prePage = p
		}
	}

	// 2. 屏幕画面采集：由服务端 StreamKeeper 自动维护视频流（15分钟保活与延时刷新），获取最新解码画面帧
	var sceneImg image.Image
	var frameBytes []byte

	if s.streamKeeper != nil {
		frame, err := s.streamKeeper.EnsureFrame(collectCtx, serial, 3*time.Second)
		if err == nil && len(frame.JPEG) > 0 {
			if decoded, dErr := jpeg.Decode(bytes.NewReader(frame.JPEG)); dErr == nil {
				sceneImg = decoded
				frameBytes = frame.JPEG
				state.Screenshot = ScreenshotInfo{
					DataBase64: fmt.Sprintf("data:image/jpeg;base64,%s", base64.StdEncoding.EncodeToString(frame.JPEG)),
					Width:      frame.Width,
					Height:     frame.Height,
					SizeBytes:  len(frame.JPEG),
				}
				sources["screenshot"] = SourceStatus{
					Status:    "success",
					Timestamp: time.Now().UnixMilli(),
				}
			}
		}
	} else if s.frames != nil {
		if frame, hasFrame := s.frames.Latest(serial, frameMaxAge); hasFrame {
			if decoded, dErr := jpeg.Decode(bytes.NewReader(frame.JPEG)); dErr == nil {
				sceneImg = decoded
				frameBytes = frame.JPEG
				state.Screenshot = ScreenshotInfo{
					DataBase64: fmt.Sprintf("data:image/jpeg;base64,%s", base64.StdEncoding.EncodeToString(frame.JPEG)),
					Width:      frame.Width,
					Height:     frame.Height,
					SizeBytes:  len(frame.JPEG),
				}
				sources["screenshot"] = SourceStatus{
					Status:    "success",
					Timestamp: time.Now().UnixMilli(),
				}
			}
		}
	}

	if sceneImg == nil {
		sources["screenshot"] = SourceStatus{
			Status:    "failed",
			Timestamp: time.Now().UnixMilli(),
			Error:     "服务端视频流未生成画面帧",
		}
		partialFailures = append(partialFailures, "获取屏幕画面失败（服务端视频流无可用帧）")
	}

	// 3. OCR 识别
	if s.ocrService != nil && len(frameBytes) > 0 {
		ocrResp, oErr := s.ocrService.Detect(collectCtx, frameBytes, ocr.DetectRequest{Serial: serial, Engine: "auto"})
		if oErr != nil {
			state.OCR.Error = oErr.Error()
			sources["ocr"] = SourceStatus{
				Status:    "failed",
				Timestamp: time.Now().UnixMilli(),
				Error:     oErr.Error(),
			}
			partialFailures = append(partialFailures, fmt.Sprintf("OCR 识别请求失败: %v", oErr))
		} else if ocrResp != nil {
			state.OCR.Engine = ocrResp.Engine
			state.OCR.Text = ocrResp.Text
			for _, b := range ocrResp.Boxes {
				state.OCR.Boxes = append(state.OCR.Boxes, OCRBox{
					Text:       b.Text,
					Confidence: b.Confidence,
					X:          b.X,
					Y:          b.Y,
					W:          b.W,
					H:          b.H,
					PixelX:     b.PixelX,
					PixelY:     b.PixelY,
					PixelW:     b.PixelW,
					PixelH:     b.PixelH,
				})
			}
			if len(state.OCR.Boxes) == 0 && strings.TrimSpace(state.OCR.Text) == "" {
				sources["ocr"] = SourceStatus{
					Status:    "empty",
					Timestamp: time.Now().UnixMilli(),
				}
			} else {
				sources["ocr"] = SourceStatus{
					Status:    "success",
					Timestamp: time.Now().UnixMilli(),
				}
			}
		}
	} else {
		sources["ocr"] = SourceStatus{
			Status:    "unavailable",
			Timestamp: time.Now().UnixMilli(),
		}
	}

	// 4. 模板加载与匹配
	if sceneImg != nil {
		allTemplates, tmplErrs := s.collectAllTemplates(serial)
		if len(tmplErrs) > 0 {
			partialFailures = append(partialFailures, tmplErrs...)
		}
		matches, matchErrs := MatchAllTemplates(sceneImg, allTemplates, templateIDs)
		if len(matchErrs) > 0 {
			partialFailures = append(partialFailures, matchErrs...)
		}
		state.TemplateMatches = matches

		if len(allTemplates) == 0 {
			sources["templates"] = SourceStatus{
				Status:    "empty",
				Timestamp: time.Now().UnixMilli(),
			}
		} else if len(matches) == 0 && len(matchErrs) > 0 {
			sources["templates"] = SourceStatus{
				Status:    "failed",
				Timestamp: time.Now().UnixMilli(),
				Error:     strings.Join(matchErrs, "; "),
			}
		} else if len(matches) == 0 {
			sources["templates"] = SourceStatus{
				Status:    "empty",
				Timestamp: time.Now().UnixMilli(),
			}
		} else {
			sources["templates"] = SourceStatus{
				Status:    "success",
				Timestamp: time.Now().UnixMilli(),
			}
		}
	} else {
		sources["templates"] = SourceStatus{
			Status:    "unavailable",
			Timestamp: time.Now().UnixMilli(),
		}
	}

	// 6. 采集后页面上下文核对，验证观测稳定性
	if sceneImg == nil {
		// 截屏失败，判定为不稳定
		state.ContextStable = false
		sources["page_info"] = SourceStatus{
			Status:    "failed",
			Timestamp: time.Now().UnixMilli(),
			Error:     "截屏缺失导致上下文无法验证",
		}
	} else if s.uiPage != nil {
		postPage, pErr := s.uiPage.FetchPageInfo(collectCtx, serial)
		if pErr != nil {
			partialFailures = append(partialFailures, fmt.Sprintf("核对结束页面信息失败: %v", pErr))
			state.PageInfo = prePage
			state.ContextStable = false
			sources["page_info"] = SourceStatus{
				Status:    "failed",
				Timestamp: time.Now().UnixMilli(),
				Error:     pErr.Error(),
			}
		} else {
			state.PageInfo = postPage
			sources["page_info"] = SourceStatus{
				Status:    "success",
				Timestamp: time.Now().UnixMilli(),
			}
			if prePage == nil {
				state.ContextStable = false
			} else if prePage.PackageName != postPage.PackageName || prePage.Activity != postPage.Activity {
				state.ContextStable = false
				partialFailures = append(partialFailures, fmt.Sprintf("观测期间页面上下文发生变化: 从 %s/%s 切换至 %s/%s", prePage.PackageName, prePage.Activity, postPage.PackageName, postPage.Activity))
			} else {
				state.ContextStable = true
			}
		}
	} else {
		state.ContextStable = true
		sources["page_info"] = SourceStatus{
			Status:    "unavailable",
			Timestamp: time.Now().UnixMilli(),
		}
	}

	// 7. 填充设备指标信息
	deviceInfo := map[string]any{
		"serial": serial,
		"status": "offline",
	}
	if s.adbPath != "" {
		stateCmd := dependencies.CommandContext(collectCtx, s.adbPath, "-s", serial, "get-state")
		if stateOut, stateErr := stateCmd.Output(); stateErr == nil {
			deviceInfo["status"] = strings.TrimSpace(string(stateOut))
		} else if sceneImg != nil {
			deviceInfo["status"] = "device"
		}
	} else if sceneImg != nil {
		deviceInfo["status"] = "device"
	}
	state.DeviceInfo = deviceInfo

	state.Sources = sources
	state.PartialFailures = partialFailures
	return state, nil
}

func (s *Service) collectAllTemplates(serial string, skillDirs ...string) ([]LoadedTemplate, []string) {
	var templates []LoadedTemplate
	var errs []string

	if s.templateStorage != nil {
		// 1. 全局启用模板
		globals := s.templateStorage.ListTemplatesStrict(template.GlobalSerial)
		for _, t := range globals {
			if !t.Enabled {
				continue
			}
			img, iErr := s.templateStorage.GetTemplateImage(t.ID)
			if iErr == nil && img != nil {
				templates = append(templates, LoadedTemplate{
					Source:      "global",
					ID:          t.ID,
					Name:        t.Name,
					Threshold:   t.Threshold,
					Method:      t.Method,
					Grayscale:   t.Grayscale,
					Scales:      t.Scales,
					Enabled:     t.Enabled,
					ContentHash: ComputeImageHash(img),
					Image:       img,
				})
			}
		}

		// 2. 当前设备启用模板
		if serial != "" && serial != template.GlobalSerial {
			devices := s.templateStorage.ListTemplatesStrict(serial)
			for _, t := range devices {
				if !t.Enabled {
					continue
				}
				img, iErr := s.templateStorage.GetTemplateImage(t.ID)
				if iErr == nil && img != nil {
					templates = append(templates, LoadedTemplate{
						Source:      "device",
						ID:          t.ID,
						Name:        t.Name,
						Threshold:   t.Threshold,
						Method:      t.Method,
						Grayscale:   t.Grayscale,
						Scales:      t.Scales,
						Enabled:     t.Enabled,
						ContentHash: ComputeImageHash(img),
						Image:       img,
					})
				}
			}
		}
	}

	// 3. 关联技能模板
	for _, skillDir := range skillDirs {
		if skillDir != "" {
			skillTmpls, sErrs := LoadSkillTemplates(skillDir)
			templates = append(templates, skillTmpls...)
			errs = append(errs, sErrs...)
		}
	}

	return templates, errs
}

func randomHex(n int) string {
	b := make([]byte, n)
	_, _ = rand.Read(b)
	return hex.EncodeToString(b)
}
