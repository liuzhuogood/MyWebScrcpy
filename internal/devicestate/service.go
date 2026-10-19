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
	"image/png"
	"log"
	"math"
	"strings"
	"sync"
	"time"

	"mywebscrcpy/internal/dependencies"
	"mywebscrcpy/internal/ocr"
	"mywebscrcpy/internal/template"
	"mywebscrcpy/internal/uipage"
	"mywebscrcpy/internal/vision"
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

// VisionResultProvider 提供设备视频流最新视觉检测结果的查询接口
type VisionResultProvider interface {
	LatestVisionResult(serial string) (vision.Message, bool)
}

// CollectOptions 控制状态采集的可选参数
type CollectOptions struct {
	TemplateIDs []string
	IncludeOCR  bool   // 默认 true，若为 false 则跳过 OCR 识别
	OCREngine   string // 可选指定引擎，传空则由 OCR 服务自动读取系统配置 (如 macocr)
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
	visionHub       VisionResultProvider

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

// SetVisionHub 挂载视频流视觉结果提供者（用于读取视频流伴随的模板匹配结果）。
func (s *Service) SetVisionHub(v VisionResultProvider) {
	s.visionHub = v
}

func (s *Service) lockChanForDevice(serial string) chan struct{} {
	v, _ := s.deviceLocks.LoadOrStore(serial, make(chan struct{}, 1))
	return v.(chan struct{})
}

func (s *Service) ensureAwakeAndUnlocked(ctx context.Context, serial string) {
	if s.adbPath == "" || serial == "" {
		return
	}
	// 0. 若为网络 ADB 设备 (ip:port)，确保已建立连接
	if strings.Contains(serial, ":") {
		connCtx, connCancel := context.WithTimeout(ctx, 2*time.Second)
		_ = dependencies.CommandContext(connCtx, s.adbPath, "connect", serial).Run()
		connCancel()
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

// CollectState 执行单轮设备状态采集，默认启用 OCR，预算 20 秒，同设备串行
func (s *Service) CollectState(ctx context.Context, serial string, templateIDs []string) (*DeviceState, error) {
	return s.CollectStateWithOptions(ctx, serial, CollectOptions{
		TemplateIDs: templateIDs,
		IncludeOCR:  true,
	})
}

// CollectStateWithOptions 执行单轮设备状态采集，支持按需控制 OCR 及指定引擎
func (s *Service) CollectStateWithOptions(ctx context.Context, serial string, opts CollectOptions) (*DeviceState, error) {
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

	var mu sync.Mutex
	sources := make(map[string]SourceStatus)
	var partialFailures []string

	addSource := func(name string, st SourceStatus) {
		mu.Lock()
		defer mu.Unlock()
		sources[name] = st
	}
	addFailure := func(msg string) {
		mu.Lock()
		defer mu.Unlock()
		partialFailures = append(partialFailures, msg)
	}

	// 阶段 1：并发获取初始页面上下文 (prePage) 与屏幕最新画面帧 (sceneImg, frameBytes)
	var prePage *uipage.PageInfo
	var frameErr error
	var sceneImg image.Image
	var frameBytes []byte
	var mimeType string

	var stage1Wg sync.WaitGroup
	stage1Wg.Add(2)

	go func() {
		defer stage1Wg.Done()
		if s.uiPage != nil {
			p, err := s.uiPage.FetchPageInfo(collectCtx, serial)
			if err != nil {
				addFailure(fmt.Sprintf("获取初始页面信息失败: %v", err))
			} else {
				prePage = p
			}
		}
	}()

	go func() {
		defer stage1Wg.Done()
		sceneImg, frameBytes, mimeType, frameErr = s.GetLatestFrame(collectCtx, serial)
	}()

	stage1Wg.Wait()

	// 处理截屏画面
	if frameErr == nil && len(frameBytes) > 0 && sceneImg != nil {
		bounds := sceneImg.Bounds()
		imgW := bounds.Dx()
		imgH := bounds.Dy()
		if mimeType == "" {
			mimeType = "image/jpeg"
		}
		state.Screenshot = ScreenshotInfo{
			DataBase64: fmt.Sprintf("data:%s;base64,%s", mimeType, base64.StdEncoding.EncodeToString(frameBytes)),
			Width:      imgW,
			Height:     imgH,
			SizeBytes:  len(frameBytes),
		}
		addSource("screenshot", SourceStatus{
			Status:    "success",
			Timestamp: time.Now().UnixMilli(),
		})
	} else {
		errText := "服务端视频流未生成画面帧"
		if frameErr != nil {
			errText = frameErr.Error()
		}
		addSource("screenshot", SourceStatus{
			Status:    "failed",
			Timestamp: time.Now().UnixMilli(),
			Error:     errText,
		})
		addFailure("获取屏幕画面失败（服务端视频流无可用帧）")
	}

	// 阶段 2 并发执行：OCR 识别（若启用）、页面结束上下文核对 (postPage)、ADB 设备在线状态探测
	var stage2Wg sync.WaitGroup

	// 2.1 OCR 识别
	if opts.IncludeOCR {
		if s.ocrService != nil && len(frameBytes) > 0 {
			stage2Wg.Add(1)
			go func() {
				defer stage2Wg.Done()
				// Engine 传 opts.OCREngine（传空则由 ocrService.effectiveEngine 自动读取系统配置，如 macocr）
				ocrResp, oErr := s.ocrService.Detect(collectCtx, frameBytes, ocr.DetectRequest{
					Serial: serial,
					Engine: opts.OCREngine,
				})
				if oErr != nil {
					mu.Lock()
					state.OCR.Error = oErr.Error()
					mu.Unlock()
					addSource("ocr", SourceStatus{
						Status:    "failed",
						Timestamp: time.Now().UnixMilli(),
						Error:     oErr.Error(),
					})
					addFailure(fmt.Sprintf("OCR 识别请求失败: %v", oErr))
				} else if ocrResp != nil {
					mu.Lock()
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
					isEmpty := len(state.OCR.Boxes) == 0 && strings.TrimSpace(state.OCR.Text) == ""
					mu.Unlock()

					if isEmpty {
						addSource("ocr", SourceStatus{
							Status:    "empty",
							Timestamp: time.Now().UnixMilli(),
						})
					} else {
						addSource("ocr", SourceStatus{
							Status:    "success",
							Timestamp: time.Now().UnixMilli(),
						})
					}
				}
			}()
		} else {
			addSource("ocr", SourceStatus{
				Status:    "unavailable",
				Timestamp: time.Now().UnixMilli(),
			})
		}
	} else {
		// 调用方主动跳过 OCR
		addSource("ocr", SourceStatus{
			Status:    "skipped",
			Timestamp: time.Now().UnixMilli(),
		})
	}

	// 2.2 结束页面上下文核对，验证观测稳定性
	stage2Wg.Add(1)
	go func() {
		defer stage2Wg.Done()
		if sceneImg == nil {
			mu.Lock()
			state.ContextStable = false
			mu.Unlock()
			addSource("page_info", SourceStatus{
				Status:    "failed",
				Timestamp: time.Now().UnixMilli(),
				Error:     "截屏缺失导致上下文无法验证",
			})
			return
		}

		if s.uiPage != nil {
			postPage, pErr := s.uiPage.FetchPageInfo(collectCtx, serial)
			if pErr != nil {
				addFailure(fmt.Sprintf("核对结束页面信息失败: %v", pErr))
				mu.Lock()
				state.PageInfo = prePage
				state.ContextStable = false
				mu.Unlock()
				addSource("page_info", SourceStatus{
					Status:    "failed",
					Timestamp: time.Now().UnixMilli(),
					Error:     pErr.Error(),
				})
			} else {
				mu.Lock()
				state.PageInfo = postPage
				if prePage == nil {
					state.ContextStable = false
				} else if prePage.PackageName != postPage.PackageName || prePage.Activity != postPage.Activity {
					state.ContextStable = false
					partialFailures = append(partialFailures, fmt.Sprintf("观测期间页面上下文发生变化: 从 %s/%s 切换至 %s/%s", prePage.PackageName, prePage.Activity, postPage.PackageName, postPage.Activity))
				} else {
					state.ContextStable = true
				}
				mu.Unlock()
				addSource("page_info", SourceStatus{
					Status:    "success",
					Timestamp: time.Now().UnixMilli(),
				})
			}
		} else {
			mu.Lock()
			state.ContextStable = true
			mu.Unlock()
			addSource("page_info", SourceStatus{
				Status:    "unavailable",
				Timestamp: time.Now().UnixMilli(),
			})
		}
	}()

	// 2.3 ADB 设备在线状态探测
	stage2Wg.Add(1)
	go func() {
		defer stage2Wg.Done()
		devStatus := "offline"
		if s.adbPath != "" {
			stateCmd := dependencies.CommandContext(collectCtx, s.adbPath, "-s", serial, "get-state")
			if stateOut, stateErr := stateCmd.Output(); stateErr == nil {
				devStatus = strings.TrimSpace(string(stateOut))
			} else if sceneImg != nil {
				devStatus = "device"
			}
		} else if sceneImg != nil {
			devStatus = "device"
		}
		mu.Lock()
		state.DeviceInfo["serial"] = serial
		state.DeviceInfo["status"] = devStatus
		mu.Unlock()
	}()

	stage2Wg.Wait()

	// 3. 模板匹配：直接读取视频流最新推理结果缓存（0ms），不再进行耗时数秒的纯 Go 多尺度扫描
	state.TemplateMatches = []TemplateMatchItem{}
	if sceneImg != nil {
		if s.visionHub != nil {
			if msg, ok := s.visionHub.LatestVisionResult(serial); ok && len(msg.Objects) > 0 {
				imgW := state.Screenshot.Width
				imgH := state.Screenshot.Height
				for _, obj := range msg.Objects {
					if len(opts.TemplateIDs) > 0 {
						matched := false
						for _, targetID := range opts.TemplateIDs {
							if targetID == obj.Label {
								matched = true
								break
							}
						}
						if !matched {
							continue
						}
					}
					item := TemplateMatchItem{
						Source:       "stream",
						TemplateID:   obj.Label,
						TemplateName: obj.Label,
						Confidence:   obj.Confidence,
						X:            obj.X,
						Y:            obj.Y,
						W:            obj.W,
						H:            obj.H,
					}
					if imgW > 0 && imgH > 0 {
						item.PixelX = int(math.Round(obj.X * float64(imgW)))
						item.PixelY = int(math.Round(obj.Y * float64(imgH)))
						item.PixelW = int(math.Round(obj.W * float64(imgW)))
						item.PixelH = int(math.Round(obj.H * float64(imgH)))
					}
					if s.templateStorage != nil {
						if tmpl, err := s.templateStorage.GetTemplate(obj.Label); err == nil && tmpl != nil {
							item.TemplateName = tmpl.Name
							if tmpl.Serial == "" || tmpl.Serial == template.GlobalSerial {
								item.Source = "global"
							} else {
								item.Source = "device"
							}
						}
					}
					state.TemplateMatches = append(state.TemplateMatches, item)
				}
			}
		}

		if len(state.TemplateMatches) > 0 {
			addSource("templates", SourceStatus{
				Status:    "success",
				Timestamp: time.Now().UnixMilli(),
			})
		} else {
			addSource("templates", SourceStatus{
				Status:    "empty",
				Timestamp: time.Now().UnixMilli(),
			})
		}
	} else {
		addSource("templates", SourceStatus{
			Status:    "unavailable",
			Timestamp: time.Now().UnixMilli(),
		})
	}

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

// GetLatestFrame 获取指定设备的最新屏幕画面帧。
// 优先从视频流（StreamKeeper 或 visionframes.Store）获取最新帧；若无视频流或超时则通过 adb screencap 保底。
// 返回: 解码图像, 原始字节流, Content-Type ("image/jpeg" 或 "image/png"), 错误
func (s *Service) GetLatestFrame(ctx context.Context, serial string) (image.Image, []byte, string, error) {
	if serial == "" {
		return nil, nil, "", fmt.Errorf("missing device serial")
	}

	var frame visionframes.Frame
	var frameErr error

	if s.streamKeeper != nil {
		frame, frameErr = s.streamKeeper.EnsureFrame(ctx, serial, 3*time.Second)
	} else if s.frames != nil {
		if f, hasFrame := s.frames.Latest(serial, frameMaxAge); hasFrame {
			frame = f
		} else {
			frameErr = fmt.Errorf("没有最近可用的投屏帧")
		}
	} else {
		frameErr = fmt.Errorf("服务端无可用帧源")
	}

	if frameErr == nil && len(frame.JPEG) > 0 {
		if decoded, dErr := jpeg.Decode(bytes.NewReader(frame.JPEG)); dErr == nil {
			return decoded, frame.JPEG, "image/jpeg", nil
		}
	}

	// 视频流未就绪或解析失败时，通过 ADB screencap 保底
	if s.adbPath != "" {
		cmd := dependencies.CommandContext(ctx, s.adbPath, "-s", serial, "exec-out", "screencap", "-p")
		out, err := cmd.Output()
		if err == nil && len(out) > 0 {
			if decoded, pErr := png.Decode(bytes.NewReader(out)); pErr == nil {
				return decoded, out, "image/png", nil
			}
		}
	}

	if frameErr != nil {
		return nil, nil, "", fmt.Errorf("获取最新画面帧失败: %w", frameErr)
	}
	return nil, nil, "", fmt.Errorf("获取最新画面帧失败: 无可用图像数据")
}
