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
	"strings"
	"sync"
	"time"

	"mywebscrcpy/internal/dependencies"
	"mywebscrcpy/internal/ocr"
	"mywebscrcpy/internal/taskrun"
	"mywebscrcpy/internal/template"
	"mywebscrcpy/internal/uipage"
	"mywebscrcpy/internal/visionframes"
)

const (
	CollectionBudget = 20 * time.Second
	// frameMaxAge 投屏帧的有效期；前端按 1fps 上传，超期视为投屏已停止。
	frameMaxAge = 5 * time.Second
)

// Service 提供统一设备总状态采集服务
type Service struct {
	adbPath         string
	uiPage          *uipage.Service
	ocrService      *ocr.Service
	templateStorage *template.Storage
	taskRunService  *taskrun.Service
	aiClient        *AIClient
	frames          *visionframes.Store

	deviceLocks sync.Map // map[string]*sync.Mutex，同设备串行采集
}

func NewService(
	adbPath string,
	uiPage *uipage.Service,
	ocrService *ocr.Service,
	templateStorage *template.Storage,
	taskRunService *taskrun.Service,
	aiClient *AIClient,
	frames *visionframes.Store,
) *Service {
	return &Service{
		adbPath:         adbPath,
		uiPage:          uiPage,
		ocrService:      ocrService,
		templateStorage: templateStorage,
		taskRunService:  taskRunService,
		aiClient:        aiClient,
		frames:          frames,
	}
}

func (s *Service) lockChanForDevice(serial string) chan struct{} {
	v, _ := s.deviceLocks.LoadOrStore(serial, make(chan struct{}, 1))
	return v.(chan struct{})
}

// CollectState 执行单轮设备状态采集，预算 20 秒，同设备串行
func (s *Service) CollectState(ctx context.Context, serial, taskID string, templateIDs []string) (*DeviceState, error) {
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

	// 2. 屏幕画面采集：取投屏视频流最新帧（浏览器解码，分辨率与投屏一致）
	var sceneImg image.Image
	var frameBytes []byte
	frame, hasFrame := s.frames.Latest(serial, frameMaxAge)
	if !hasFrame {
		sources["screenshot"] = SourceStatus{
			Status:    "failed",
			Timestamp: time.Now().UnixMilli(),
			Error:     fmt.Sprintf("投屏视频流无最新帧（%v 内未上传），请保持该设备投屏播放", frameMaxAge),
		}
		partialFailures = append(partialFailures, "投屏视频流无最新帧，跳过截图/OCR/模板匹配")
	} else if decoded, dErr := jpeg.Decode(bytes.NewReader(frame.JPEG)); dErr != nil {
		sources["screenshot"] = SourceStatus{
			Status:    "failed",
			Timestamp: time.Now().UnixMilli(),
			Error:     dErr.Error(),
		}
		partialFailures = append(partialFailures, fmt.Sprintf("投屏帧解码失败: %v", dErr))
	} else {
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

	// 4. 关联任务解析
	associatedTask, matchedSkillDir := s.resolveAssociatedTask(serial, taskID)
	state.AssociatedTask = associatedTask
	if s.taskRunService == nil {
		sources["task"] = SourceStatus{
			Status:    "unavailable",
			Timestamp: time.Now().UnixMilli(),
		}
	} else if associatedTask != nil && associatedTask.Conflict {
		sources["task"] = SourceStatus{
			Status:    "failed",
			Timestamp: time.Now().UnixMilli(),
			Error:     associatedTask.ConflictMessage,
		}
	} else if associatedTask == nil || associatedTask.TaskID == "" {
		sources["task"] = SourceStatus{
			Status:    "empty",
			Timestamp: time.Now().UnixMilli(),
		}
	} else {
		sources["task"] = SourceStatus{
			Status:    "success",
			Timestamp: time.Now().UnixMilli(),
		}
	}

	// 5. 三类模板加载与匹配
	if sceneImg != nil {
		allTemplates, tmplErrs := s.collectAllTemplates(serial, matchedSkillDir)
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

func (s *Service) resolveAssociatedTask(serial, explicitTaskID string) (*AssociatedTaskInfo, string) {
	if s.taskRunService == nil {
		return nil, ""
	}

	// 显式指定 task_id 时直接读取该任务目录
	if explicitTaskID != "" {
		detail, err := s.taskRunService.GetTaskDetail(explicitTaskID)
		if err != nil {
			return &AssociatedTaskInfo{
				TaskID:          explicitTaskID,
				Conflict:        true,
				ConflictMessage: fmt.Sprintf("指定任务 %s 读取失败: %v", explicitTaskID, err),
			}, ""
		}
		if detail.TaskDef.DeviceSerial != serial {
			return &AssociatedTaskInfo{
				TaskID:          explicitTaskID,
				Conflict:        true,
				ConflictMessage: fmt.Sprintf("指定任务 %s 所属设备 (%s) 与当前设备 (%s) 不一致", explicitTaskID, detail.TaskDef.DeviceSerial, serial),
			}, ""
		}
		summary := fmt.Sprintf("%d 个声明产出", len(detail.Outputs))
		if detail.OutputsComplete {
			summary += " · 必需产出完整"
		} else {
			summary += " · 必需产出待补齐"
		}
		return &AssociatedTaskInfo{
			TaskID:         explicitTaskID,
			Name:           detail.TaskDef.Name,
			Goal:           detail.TaskDef.Goal,
			Status:         detail.State.Status,
			Progress:       detail.State.Progress,
			Stage:          detail.State.Stage,
			OutputsSummary: summary,
		}, detail.TaskDef.SkillDir
	}

	// 未指定时按最近扫描索引关联
	runningTaskID, hasConflict, count, scannedAt := s.taskRunService.FindRunningTaskForDevice(serial)
	if hasConflict {
		return &AssociatedTaskInfo{
			Conflict:        true,
			ConflictMessage: fmt.Sprintf("发现 %d 个关联该设备的运行任务，请指定 task_id (索引扫描于 %s)", count, formatTime(scannedAt)),
			ScannedAt:       scannedAt,
		}, ""
	}
	if count == 0 || runningTaskID == "" {
		return &AssociatedTaskInfo{
			ScannedAt: scannedAt,
		}, ""
	}

	// 唯一运行中任务
	detail, err := s.taskRunService.GetTaskDetail(runningTaskID)
	if err != nil {
		return &AssociatedTaskInfo{
			TaskID:    runningTaskID,
			ScannedAt: scannedAt,
		}, ""
	}

	summary := fmt.Sprintf("%d 个声明产出", len(detail.Outputs))
	if detail.OutputsComplete {
		summary += " · 必需产出完整"
	} else {
		summary += " · 必需产出待补齐"
	}
	return &AssociatedTaskInfo{
		TaskID:         runningTaskID,
		Name:           detail.TaskDef.Name,
		Goal:           detail.TaskDef.Goal,
		Status:         detail.State.Status,
		Progress:       detail.State.Progress,
		Stage:          detail.State.Stage,
		OutputsSummary: summary,
		ScannedAt:      scannedAt,
	}, detail.TaskDef.SkillDir
}

func (s *Service) collectAllTemplates(serial, skillDir string) ([]LoadedTemplate, []string) {
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
	if skillDir != "" {
		skillTmpls, sErrs := LoadSkillTemplates(skillDir)
		templates = append(templates, skillTmpls...)
		errs = append(errs, sErrs...)
	}

	return templates, errs
}

func randomHex(n int) string {
	b := make([]byte, n)
	_, _ = rand.Read(b)
	return hex.EncodeToString(b)
}

func formatTime(ms int64) string {
	if ms <= 0 {
		return "未扫描"
	}
	return time.UnixMilli(ms).Format("15:04:05")
}
