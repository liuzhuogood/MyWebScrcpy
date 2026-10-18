package ocr

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"image"
	_ "image/jpeg"
	"image/png"
	"io"
	"log"
	"mime/multipart"
	"net/http"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"mywebscrcpy/internal/db"
	"mywebscrcpy/internal/dependencies"
	"mywebscrcpy/internal/servicepython"
	"mywebscrcpy/internal/vision"
	"mywebscrcpy/internal/ws"
)

const (
	DefaultMacOCRURL    = "http://10.0.0.11:8000/upload"
	DefaultPaddleOCRURL = "http://127.0.0.1:8031/upload"
	DefaultPaddlePort   = 8031
	DefaultIndraPort    = 8032
	DefaultIndraOCRURL  = "http://127.0.0.1:8032/upload"
)

type Box struct {
	Text       string  `json:"text"`
	Confidence float64 `json:"confidence"`
	X          float64 `json:"x"`
	Y          float64 `json:"y"`
	W          float64 `json:"w"`
	H          float64 `json:"h"`
	PixelX     int     `json:"pixel_x"`
	PixelY     int     `json:"pixel_y"`
	PixelW     int     `json:"pixel_w"`
	PixelH     int     `json:"pixel_h"`
}

type DetectRequest struct {
	Serial          string    `json:"serial"`
	BBox            []float64 `json:"bbox"` // [x, y, w, h] 归一化，默认为全屏 [0, 0, 1, 1]
	Mark            bool      `json:"mark"`
	MarkDurationMS  int64     `json:"mark_duration_ms"`
	Keywords        []string  `json:"keywords"`
	Engine          string    `json:"engine"` // auto, parallel, indraocr, macocr, paddleocr
	StabilizationMS int64     `json:"stabilization_ms"`
}

type DetectResponse struct {
	Samples        int      `json:"samples"`
	EnginesUsed    []string `json:"engines_used"`
	Status         string   `json:"status"` // matched | not_found
	Matched        bool     `json:"matched"`
	Text           string   `json:"text"`
	Boxes          []Box    `json:"boxes"`
	Engine         string   `json:"engine"`
	ElapsedMS      int64    `json:"elapsed_ms"`
	FallbackReason []string `json:"fallback_reason,omitempty"`
}

type TextArtResponse struct {
	Samples        int      `json:"samples"`
	EnginesUsed    []string `json:"engines_used"`
	Status         string   `json:"status"` // matched | not_found
	Matched        bool     `json:"matched"`
	TextArt        string   `json:"text_art"`
	Text           string   `json:"text"`
	Boxes          []Box    `json:"boxes"`
	Engine         string   `json:"engine"`
	ElapsedMS      int64    `json:"elapsed_ms"`
	FallbackReason []string `json:"fallback_reason,omitempty"`
}

type StatusResponse struct {
	IndraInstalled  bool   `json:"indra_installed"`
	IndraRunning    bool   `json:"indra_running"`
	IndraReady      bool   `json:"indra_ready"`
	IndraPID        int    `json:"indra_pid"`
	IndraPort       int    `json:"indra_port"`
	IndraOCRURL     string `json:"indraocr_url"`
	IndraStatus     string `json:"indra_status,omitempty"`
	IndraError      string `json:"indra_error,omitempty"`
	PaddleInstalled bool   `json:"paddle_installed"`
	PaddleRunning   bool   `json:"paddle_running"`
	PaddleReady     bool   `json:"paddle_ready"`
	PaddlePID       int    `json:"paddle_pid"`
	PaddlePort      int    `json:"paddle_port"`
	PaddleOCRURL    string `json:"paddleocr_url"`
	MacOCRURL       string `json:"macocr_url"`
	MacOCRAuthSet   bool   `json:"macocr_auth_set"`
	Engine          string `json:"engine"`
	PaddleStatus    string `json:"paddle_status,omitempty"`
	PaddleError     string `json:"paddle_error,omitempty"`
}

type Service struct {
	// Each local engine owns its process, preparation state and runtime lease.
	captureScreen         func(context.Context, string) ([]byte, error)
	detectSlots           chan struct{}
	parallelEngineTimeout time.Duration
	markMu                sync.Mutex
	markRevision          uint64
	markRequests          map[string]uint64
	profile               string
	indra                 *Service
	mu                    sync.RWMutex
	db                    *db.DB
	hub                   *ws.Hub
	paddleCmd             *exec.Cmd
	paddleRunning         bool
	paddleDone            chan struct{}
	paddleReadyCh         chan struct{}
	paddleReady           bool
	paddlePort            int
	httpClient            *http.Client
	installed             bool
	installedMissing      string
	installedPy           string
	installedAt           time.Time
	lastStartErr          string
	runtime               pythonRuntime
	prepareCancel         context.CancelFunc
	prepareDone           chan struct{}
	preparePhase          string
	control               func(bool, func() error) error
}

func NewService(database *db.DB, hub *ws.Hub) *Service {
	s := newLocalService(database, hub, servicepython.PaddleOCR, DefaultPaddlePort)
	s.indra = newLocalService(database, hub, servicepython.IndraOCR, DefaultIndraPort)
	return s
}

func newLocalService(database *db.DB, hub *ws.Hub, profile string, port int) *Service {
	return &Service{
		profile:               profile,
		captureScreen:         captureDeviceScreen,
		detectSlots:           make(chan struct{}, 2),
		parallelEngineTimeout: 5 * time.Second,
		db:                    database,
		hub:                   hub,
		paddlePort:            port,
		httpClient:            &http.Client{Timeout: 30 * time.Second},
	}
}

func (s *Service) SetPaddlePort(port int) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.paddlePort = port
}

func (s *Service) getConfig(key, fallback string) string {
	if s.db != nil {
		val, err := s.db.GetConfig(key)
		if err == nil && strings.TrimSpace(val) != "" {
			return strings.TrimSpace(val)
		}
	}
	if env := os.Getenv(key); strings.TrimSpace(env) != "" {
		return strings.TrimSpace(env)
	}
	return fallback
}

func (s *Service) setConfig(key, value string) error {
	if s.db == nil {
		return errors.New("数据库未就绪，无法持久化配置")
	}
	return s.db.SetConfig(key, value)
}

func (s *Service) checkInstalled(running bool) (bool, string) {
	if running {
		return true, ""
	}
	s.mu.RLock()
	preparing := s.preparePhase != ""
	s.mu.RUnlock()
	if preparing {
		return false, ""
	}

	pyPath, err := dependencies.PythonPath()
	if s.runtime != nil {
		var release func()
		pyPath, release, err = s.runtime.Acquire(s.profile)
		if err == nil {
			defer release()
		}
	}
	if err != nil || pyPath == "" {
		return false, "未找到 Python 运行时"
	}

	s.mu.RLock()
	if s.installedPy == pyPath && time.Since(s.installedAt) < 5*time.Second {
		cached := s.installed
		missing := s.installedMissing
		s.mu.RUnlock()
		return cached, missing
	}
	s.mu.RUnlock()

	var installed bool
	var missing string
	if s.runtime != nil {
		err := s.runtime.Check(context.Background(), s.profile, pyPath)
		installed = err == nil
		missing = ""
		if err != nil {
			missing = err.Error()
		}
	} else {
		if s.profile == servicepython.IndraOCR {
			installed, missing = false, "请通过应用管理的 Python 环境启动 Indra OCR"
		} else {
			installed, missing = probePaddleDependencies(context.Background(), pyPath)
		}
	}

	s.mu.Lock()
	s.installed = installed
	s.installedMissing = missing
	s.installedPy = pyPath
	s.installedAt = time.Now()
	s.mu.Unlock()

	return installed, missing
}

func probePaddleDependencies(ctx context.Context, pyPath string) (bool, string) {
	ctx, cancel := context.WithTimeout(ctx, 3*time.Second)
	defer cancel()

	probeScript := "import importlib.util, sys\n" +
		"mods = ['paddle', 'paddleocr', 'fastapi', 'uvicorn', 'multipart']\n" +
		"missing = [m for m in mods if importlib.util.find_spec(m) is None]\n" +
		"if missing:\n" +
		"    sys.stderr.write(','.join(missing))\n" +
		"    sys.exit(1)\n" +
		"sys.exit(0)\n"

	cmd := exec.CommandContext(ctx, pyPath, "-B", "-c", probeScript)
	cmd.Dir = os.TempDir()
	cmd.Env = dependencies.PythonEnvironment()
	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	err := cmd.Run()
	if err != nil {
		missing := strings.TrimSpace(stderr.String())
		if missing == "" {
			if ctx.Err() != nil {
				missing = "探测超时"
			} else {
				missing = err.Error()
			}
		}
		return false, missing
	}
	return true, ""
}

func (s *Service) Status() StatusResponse {
	s.mu.RLock()
	running := s.paddleCmd != nil && s.paddleRunning
	ready := running && s.paddleReady
	pid := 0
	if running {
		pid = s.paddleCmd.Process.Pid
	}
	port := s.paddlePort
	lastErr := s.lastStartErr
	phase := s.preparePhase
	s.mu.RUnlock()

	macURL := s.getConfig("MACOCR_URL", DefaultMacOCRURL)
	macAuth := s.getConfig("MACOCR_AUTH", "")
	paddleURL := s.getConfig("PADDLEOCR_URL", fmt.Sprintf("http://127.0.0.1:%d/upload", port))
	engine := s.getConfig("OCR_ENGINE", "auto")

	installed, missing := s.checkInstalled(running)

	paddleStatus := "installed"
	if !installed {
		paddleStatus = "uninstalled"
	}
	if running && !ready {
		paddleStatus = "starting"
	} else if ready {
		paddleStatus = "running"
	} else if lastErr != "" {
		paddleStatus = "failed"
	}
	if phase != "" {
		paddleStatus = phase
	}

	displayErr := lastErr
	if !running && !installed && displayErr == "" && phase == "" {
		displayErr = fmt.Sprintf("缺少依赖: %s", missing)
	}

	status := StatusResponse{
		PaddleInstalled: installed,
		PaddleRunning:   running,
		PaddleReady:     ready,
		PaddlePID:       pid,
		PaddlePort:      port,
		PaddleOCRURL:    paddleURL,
		MacOCRURL:       macURL,
		MacOCRAuthSet:   macAuth != "",
		Engine:          engine,
		PaddleStatus:    paddleStatus,
		PaddleError:     displayErr,
	}
	if s.indra != nil {
		i := s.indra.Status()
		status.IndraInstalled, status.IndraRunning, status.IndraReady = i.PaddleInstalled, i.PaddleRunning, i.PaddleReady
		status.IndraPID, status.IndraPort = i.PaddlePID, i.PaddlePort
		status.IndraStatus, status.IndraError = i.PaddleStatus, i.PaddleError
		status.IndraOCRURL = s.getConfig("INDRAOCR_URL", fmt.Sprintf("http://127.0.0.1:%d/upload", i.PaddlePort))
	}
	return status
}

func (s *Service) StartPaddle(ctx context.Context) error {
	s.mu.RLock()
	if s.paddleCmd != nil && s.paddleRunning {
		if s.paddleReady {
			s.mu.RUnlock()
			return nil
		}
		cmd, done, ready, port := s.paddleCmd, s.paddleDone, s.paddleReadyCh, s.paddlePort
		s.mu.RUnlock()
		return s.waitForPaddleReady(ctx, cmd, done, ready, port)
	}
	s.mu.RUnlock()

	if s.runtime != nil {
		return s.startManagedPaddle(ctx)
	}

	if s.profile == servicepython.IndraOCR {
		return errors.New("Indra OCR 需要应用管理的 Python 环境")
	}
	pyPath, err := dependencies.PythonPath()
	if err != nil {
		s.mu.Lock()
		s.lastStartErr = err.Error()
		s.mu.Unlock()
		return fmt.Errorf("找不到可用的 Python 运行时: %w", err)
	}

	serverScript := filepath.Join("packages", "paddleocr-server", "server.py")
	if _, err := os.Stat(serverScript); err != nil {
		s.mu.Lock()
		s.lastStartErr = err.Error()
		s.mu.Unlock()
		return fmt.Errorf("找不到 paddleocr-server 脚本 (%s): %w", serverScript, err)
	}

	// 依赖探测会自行获取服务锁，因此必须在 StartPaddle 的状态锁之外执行。
	installed, missing := s.checkInstalled(false)
	if !installed {
		err := fmt.Errorf("PaddleOCR 依赖未就绪，缺少必要模块: %s", missing)
		s.mu.Lock()
		s.lastStartErr = err.Error()
		s.mu.Unlock()
		return err
	}

	return s.startPaddleProcess(ctx, pyPath, serverScript, func() {})
}

func (s *Service) startPaddleProcess(ctx context.Context, pyPath, serverScript string, release func()) error {
	handedOff := false
	defer func() {
		if !handedOff {
			release()
		}
	}()
	s.mu.Lock()
	if s.paddleCmd != nil && s.paddleRunning {
		if s.paddleReady {
			s.mu.Unlock()
			return nil
		}
		cmd, done, ready, port := s.paddleCmd, s.paddleDone, s.paddleReadyCh, s.paddlePort
		s.mu.Unlock()
		return s.waitForPaddleReady(ctx, cmd, done, ready, port)
	}

	port := s.paddlePort
	// 独立于 HTTP 请求 Context 创建进程，进程生命周期由 Service 管理
	cmd := exec.Command(pyPath, serverScript, "--host", "127.0.0.1", "--port", fmt.Sprintf("%d", port))
	cmd.Env = dependencies.PythonEnvironment()
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	done := make(chan struct{})
	ready := make(chan struct{})
	s.paddleDone = done
	s.paddleReadyCh = ready
	s.paddleReady = false
	s.paddleCmd = cmd
	s.paddleRunning = false
	s.lastStartErr = ""

	if err := cmd.Start(); err != nil {
		s.paddleCmd = nil
		s.paddleRunning = false
		s.paddleDone = nil
		s.lastStartErr = err.Error()
		s.mu.Unlock()
		log.Printf("[OCR] 启动 paddleocr-server 失败: %v", err)
		return fmt.Errorf("启动 paddleocr-server 失败: %w", err)
	}
	s.paddleRunning = true
	handedOff = true
	s.mu.Unlock()

	log.Printf("[OCR] 已启动 service=%s PID=%d port=%d", s.profile, cmd.Process.Pid, port)

	go func() {
		_ = cmd.Wait()
		release()
		s.mu.Lock()
		if s.paddleCmd == cmd {
			s.paddleCmd = nil
			s.paddleRunning = false
			s.paddleDone = nil
			s.paddleReadyCh = nil
			s.paddleReady = false
		}
		s.mu.Unlock()
		close(done)
		log.Printf("[OCR] 服务进程已退出 service=%s", s.profile)
	}()

	return s.waitForPaddleReady(ctx, cmd, done, ready, port)
}

func (s *Service) waitForPaddleReady(ctx context.Context, cmd *exec.Cmd, done <-chan struct{}, ready <-chan struct{}, port int) error {
	startupTimeout := 120 * time.Second
	if s.profile == servicepython.IndraOCR {
		startupTimeout = 300 * time.Second
	}
	readyCtx, cancel := context.WithTimeout(ctx, startupTimeout)
	defer cancel()
	url := fmt.Sprintf("http://127.0.0.1:%d/health", port)
	ticker := time.NewTicker(100 * time.Millisecond)
	defer ticker.Stop()
	client := &http.Client{Timeout: time.Second}

	fail := func(err error) error {
		_ = stopProcess(cmd, done)
		s.mu.Lock()
		if s.paddleCmd == cmd || s.paddleCmd == nil {
			s.lastStartErr = err.Error()
		}
		s.mu.Unlock()
		return err
	}

	for {
		select {
		case <-ready:
			return nil
		case <-done:
			err := errors.New("OCR 服务进程在模型就绪前退出，请检查服务日志")
			s.mu.Lock()
			if s.paddleCmd == nil {
				s.lastStartErr = err.Error()
			}
			s.mu.Unlock()
			return err
		case <-readyCtx.Done():
			return fail(fmt.Errorf("等待 OCR 服务模型就绪失败: %w", readyCtx.Err()))
		case <-ticker.C:
			req, err := http.NewRequestWithContext(readyCtx, http.MethodGet, url, nil)
			if err != nil {
				continue
			}
			resp, err := client.Do(req)
			if err != nil {
				continue
			}
			var body struct {
				Ready  bool   `json:"ready"`
				Detail string `json:"detail"`
				Engine string `json:"engine"`
				PID    int    `json:"pid"`
			}
			_ = json.NewDecoder(resp.Body).Decode(&body)
			resp.Body.Close()
			if strings.HasPrefix(body.Detail, "Model initialization failed:") {
				return fail(errors.New(body.Detail))
			}
			if resp.StatusCode != http.StatusOK || !body.Ready {
				continue
			}
			if s.profile == servicepython.IndraOCR && (body.Engine != servicepython.IndraOCR || body.PID != cmd.Process.Pid) {
				continue
			}

			s.mu.Lock()
			if s.paddleCmd == cmd && s.paddleRunning {
				s.paddleReady = true
				if s.paddleReadyCh != nil {
					close(s.paddleReadyCh)
					s.paddleReadyCh = nil
				}
				s.mu.Unlock()
				return nil
			}
			s.mu.Unlock()
			return errors.New("OCR 服务在就绪确认时已退出")
		}
	}
}

func (s *Service) StopPaddle() error {
	s.mu.Lock()
	cancel, preparing := s.prepareCancel, s.prepareDone
	cmd := s.paddleCmd
	done := s.paddleDone
	s.mu.Unlock()
	if cancel != nil {
		cancel()
		<-preparing
		s.mu.RLock()
		cmd, done = s.paddleCmd, s.paddleDone
		s.mu.RUnlock()
		s.mu.Lock()
		s.lastStartErr = ""
		s.mu.Unlock()
	}

	if cmd == nil || cmd.Process == nil {
		return nil
	}

	return stopProcess(cmd, done)
}

func stopProcess(cmd *exec.Cmd, done <-chan struct{}) error {
	if cmd == nil || cmd.Process == nil || done == nil {
		return nil
	}
	if err := cmd.Process.Signal(os.Interrupt); err != nil {
		_ = cmd.Process.Kill()
	}
	select {
	case <-done:
		return nil
	case <-time.After(2 * time.Second):
		_ = cmd.Process.Kill()
	}
	select {
	case <-done:
		return nil
	case <-time.After(2 * time.Second):
		return errors.New("等待 OCR 服务退出超时")
	}
}

func (s *Service) SaveConfig(macURL, macAuth, paddleURL, indraURL, engine string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if macURL != "" {
		if err := s.setConfig("MACOCR_URL", macURL); err != nil {
			return err
		}
	}
	if macAuth != "" {
		if err := s.setConfig("MACOCR_AUTH", macAuth); err != nil {
			return err
		}
	}
	if paddleURL != "" {
		if err := s.setConfig("PADDLEOCR_URL", paddleURL); err != nil {
			return err
		}
	}
	if indraURL != "" {
		if err := s.setConfig("INDRAOCR_URL", indraURL); err != nil {
			return err
		}
	}
	if engine != "" {
		if err := s.setConfig("OCR_ENGINE", engine); err != nil {
			return err
		}
	}
	return nil
}

func (s *Service) TestConnection(ctx context.Context, targetURL, auth string) (int64, error) {
	started := time.Now()
	testImg := image.NewRGBA(image.Rect(0, 0, 10, 10))
	var buf bytes.Buffer
	if err := png.Encode(&buf, testImg); err != nil {
		return 0, err
	}

	_, err := s.uploadToOCR(ctx, buf.Bytes(), targetURL, auth)
	elapsed := time.Since(started).Milliseconds()
	if err != nil {
		return elapsed, err
	}
	return elapsed, nil
}

func (s *Service) uploadToOCR(ctx context.Context, pngBytes []byte, ocrURL, auth string) (map[string]any, error) {
	var body bytes.Buffer
	writer := multipart.NewWriter(&body)
	part, err := writer.CreateFormFile("file", "image.png")
	if err != nil {
		return nil, fmt.Errorf("创建 multipart 失败: %w", err)
	}
	if _, err := part.Write(pngBytes); err != nil {
		return nil, fmt.Errorf("写入图片内容失败: %w", err)
	}
	if err := writer.Close(); err != nil {
		return nil, fmt.Errorf("关闭 multipart 写入器失败: %w", err)
	}

	parsed, err := url.Parse(ocrURL)
	if err != nil {
		return nil, fmt.Errorf("非法 OCR URL: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, ocrURL, &body)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", writer.FormDataContentType())
	req.Header.Set("Accept", "application/json")

	if auth != "" {
		encoded := base64.StdEncoding.EncodeToString([]byte(auth))
		req.Header.Set("Authorization", "Basic "+encoded)
	} else if parsed.User != nil {
		pass, _ := parsed.User.Password()
		token := base64.StdEncoding.EncodeToString([]byte(parsed.User.Username() + ":" + pass))
		req.Header.Set("Authorization", "Basic "+token)
	}

	resp, err := s.httpClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		respBody, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("OCR 接口返回 HTTP %d: %s", resp.StatusCode, string(respBody))
	}

	var result map[string]any
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("解析 OCR 响应 JSON 失败: %w", err)
	}
	return result, nil
}

func (s *Service) detectSingle(ctx context.Context, imgBytes []byte, req DetectRequest, strict bool) (*DetectResponse, error) {
	started := time.Now()
	s.mu.RLock()
	macURL := s.getConfig("MACOCR_URL", DefaultMacOCRURL)
	macAuth := s.getConfig("MACOCR_AUTH", "")
	paddleURL := s.getConfig("PADDLEOCR_URL", fmt.Sprintf("http://127.0.0.1:%d/upload", s.paddlePort))
	s.indra.mu.RLock()
	indraPort := s.indra.paddlePort
	s.indra.mu.RUnlock()
	indraURL := s.getConfig("INDRAOCR_URL", fmt.Sprintf("http://127.0.0.1:%d/upload", indraPort))
	defaultEngine := s.getConfig("OCR_ENGINE", "auto")
	s.mu.RUnlock()

	engine := req.Engine
	if engine == "" {
		engine = defaultEngine
	}
	if engine != "auto" && engine != "indraocr" && engine != "macocr" && engine != "paddleocr" {
		engine = "auto"
	}

	var result map[string]any
	var engineUsed string
	var errorsList []string

	if engine == "auto" || engine == "indraocr" {
		res, err := s.uploadToOCR(ctx, imgBytes, indraURL, "")
		if err == nil {
			err = validateIndraResult(res)
		}
		if err == nil {
			result, engineUsed = res, "indraocr"
		} else {
			errorsList = append(errorsList, fmt.Sprintf("Indra OCR: %v", err))
			if engine == "indraocr" {
				return nil, fmt.Errorf("Indra OCR 识别失败: %w", err)
			}
			if ctx.Err() != nil {
				return nil, ctx.Err()
			}
			log.Printf("[OCR] Indra OCR 不可用，继续尝试下一引擎: %v", err)
		}
	}
	if result == nil && (engine == "auto" || engine == "macocr") {
		res, err := s.uploadToOCR(ctx, imgBytes, macURL, macAuth)
		if err == nil {
			result = res
			engineUsed = "macocr"
		} else {
			errorsList = append(errorsList, fmt.Sprintf("macOCR: %v", err))
			if engine == "macocr" {
				return nil, fmt.Errorf("macOCR 识别失败: %w", err)
			}
		}
	}

	if result == nil {
		res, err := s.uploadToOCR(ctx, imgBytes, paddleURL, "")
		if err == nil {
			result = res
			engineUsed = "paddleocr"
		} else {
			errorsList = append(errorsList, fmt.Sprintf("PaddleOCR: %v", err))
			return nil, fmt.Errorf("所有 OCR 服务均不可用 (%s)", strings.Join(errorsList, "; "))
		}
	}

	if strict {
		if err := validateIndraResult(result); err != nil {
			return nil, fmt.Errorf("%s OCR 响应无效: %w", engineUsed, err)
		}
	}
	imgConfig, _, err := image.DecodeConfig(bytes.NewReader(imgBytes))
	imgW, imgH := imgConfig.Width, imgConfig.Height
	if err != nil || imgW <= 0 || imgH <= 0 {
		imgW, imgH = 1, 1
	}

	normBBox := req.BBox
	if len(normBBox) != 4 {
		normBBox = []float64{0, 0, 1, 1}
	}
	ox, oy, ow, oh := normBBox[0], normBBox[1], normBBox[2], normBBox[3]

	var boxes []Box
	var visionObjects []vision.Object

	rawBoxes, _ := result["ocr_boxes"].([]any)
	for _, rb := range rawBoxes {
		item, ok := rb.(map[string]any)
		if !ok {
			continue
		}
		text, _ := item["text"].(string)
		text = strings.TrimSpace(text)
		if text == "" {
			continue
		}

		if len(req.Keywords) > 0 {
			matchedKeyword := false
			lowerText := strings.ToLower(text)
			for _, kw := range req.Keywords {
				if strings.Contains(lowerText, strings.ToLower(kw)) {
					matchedKeyword = true
					break
				}
			}
			if !matchedKeyword {
				continue
			}
		}

		conf, _ := item["confidence"].(float64)
		rx, _ := item["x"].(float64)
		ry, _ := item["y"].(float64)
		rw, _ := item["w"].(float64)
		rh, _ := item["h"].(float64)

		var nx, ny, nw, nh float64
		var px, py, pw, ph int
		if engineUsed == "indraocr" || rx > 1.0 || ry > 1.0 || rw > 1.0 || rh > 1.0 {
			nx = rx / float64(imgW)
			ny = ry / float64(imgH)
			nw = rw / float64(imgW)
			nh = rh / float64(imgH)
			px, py, pw, ph = int(rx), int(ry), int(rw), int(rh)
		} else {
			nx, ny, nw, nh = rx, ry, rw, rh
			px = int(rx * float64(imgW))
			py = int(ry * float64(imgH))
			pw = int(rw * float64(imgW))
			ph = int(rh * float64(imgH))
		}

		gx := ox + nx*ow
		gy := oy + ny*oh
		gw := nw * ow
		gh := nh * oh

		boxes = append(boxes, Box{
			Text:       text,
			Confidence: conf,
			X:          gx,
			Y:          gy,
			W:          gw,
			H:          gh,
			PixelX:     px,
			PixelY:     py,
			PixelW:     pw,
			PixelH:     ph,
		})

		visionObjects = append(visionObjects, vision.Object{
			Label:      text,
			Confidence: conf,
			X:          gx,
			Y:          gy,
			W:          gw,
			H:          gh,
		})
	}

	fullText, _ := result["ocr_result"].(string)
	duration := req.MarkDurationMS
	if duration <= 0 {
		duration = 3000
	}

	if req.Mark && req.Serial != "" && len(visionObjects) > 0 && s.hub != nil {
		s.hub.PublishVisionResult(req.Serial, vision.Message{
			Type:      "detection.result",
			DeviceID:  req.Serial,
			Timestamp: time.Now().UnixMilli(),
			ExpiresMS: duration,
			Objects:   visionObjects,
		})
	}

	status := "not_found"
	matched := false
	if len(boxes) > 0 {
		status = "matched"
		matched = true
	}
	if boxes == nil {
		boxes = []Box{}
	}

	return &DetectResponse{
		Status:         status,
		Matched:        matched,
		Text:           strings.TrimSpace(fullText),
		Boxes:          boxes,
		Engine:         engineUsed,
		ElapsedMS:      time.Since(started).Milliseconds(),
		FallbackReason: errorsList,
	}, nil
}
