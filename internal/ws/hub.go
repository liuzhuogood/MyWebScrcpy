package ws

import (
	"context"
	"crypto/rand"
	"encoding/binary"
	"encoding/hex"
	"errors"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/exec"
	"strconv"
	"strings"
	"sync"
	"time"

	"mywebscrcpy/internal/action"
	"mywebscrcpy/internal/adbcommand"
	debuglog "mywebscrcpy/internal/debug"
	"mywebscrcpy/internal/devicegate"
	"mywebscrcpy/internal/scrcpy"
	"mywebscrcpy/internal/vision"

	"github.com/gorilla/websocket"
)

var upgrader = websocket.Upgrader{
	ReadBufferSize:  65536,
	WriteBufferSize: 65536,
	CheckOrigin: func(r *http.Request) bool {
		return true // 开发环境允许任意来源
	},
}

const defaultSessionIdleTTL = 60 * time.Second

type deviceEnv struct {
	sdk      int
	jarReady bool
}

// Hub 管理所有设备的 scrcpy server 会话。
type Hub struct {
	adbPath         string
	jarPath         string
	jarSize         int64
	portMu          sync.Mutex
	nextPort        int
	resultMu        sync.Mutex
	results         map[string]map[chan vision.Message]string
	lastResult      map[string]vision.Message
	touchMu         sync.Mutex
	touchSubs       map[string]map[chan TouchEventMessage]string
	events          *debuglog.Ring
	sessionMu       sync.Mutex
	closed          bool
	sessions        map[string]*managedSession
	inflight        map[string]*inflightSession
	envMu           sync.Mutex
	deviceEnvs      map[string]deviceEnv
	idleTTL         time.Duration
	startSessionFn  func(serial string) (*session, *handshakeMeta, error)
	recordings      *recordingManager
	replays         *replayManager
	gates           map[string]*devicegate.Gate
	gateMu          sync.Mutex
	commands        *adbcommand.Manager
	frameConsumerMu sync.RWMutex
	frameConsumer   VideoFrameConsumer
}

// VideoFrame is an encoded frame exactly as it arrived from the active scrcpy
// session. Consumers must return quickly; the hub keeps video fan-out live.
type VideoFrame struct {
	DeviceID   string
	SessionID  string
	FrameID    uint64
	PTS        uint64
	CapturedAt time.Time
	Kind       scrcpy.FrameKind
	Width      uint32
	Height     uint32
	Payload    []byte
}

// VideoFrameConsumer receives live scrcpy frames inside the Go process.
type VideoFrameConsumer interface{ ConsumeVideoFrame(VideoFrame) }

func (h *Hub) SetVideoFrameConsumer(consumer VideoFrameConsumer) {
	h.frameConsumerMu.Lock()
	h.frameConsumer = consumer
	h.frameConsumerMu.Unlock()
}

func (h *Hub) consumeVideoFrame(frame VideoFrame) {
	h.frameConsumerMu.RLock()
	consumer := h.frameConsumer
	h.frameConsumerMu.RUnlock()
	if consumer != nil {
		consumer.ConsumeVideoFrame(frame)
	}
}

func NewHub(adbPath, jarPath string) *Hub {
	var jarSize int64
	if jarPath != "" {
		if info, err := os.Stat(jarPath); err == nil {
			jarSize = info.Size()
		}
	}
	h := &Hub{
		adbPath:    adbPath,
		jarPath:    jarPath,
		jarSize:    jarSize,
		nextPort:   27183,
		results:    make(map[string]map[chan vision.Message]string),
		lastResult: make(map[string]vision.Message),
		sessions:   make(map[string]*managedSession),
		inflight:   make(map[string]*inflightSession),
		deviceEnvs: make(map[string]deviceEnv),
		idleTTL:    defaultSessionIdleTTL,
		gates:      make(map[string]*devicegate.Gate),
		events:     debuglog.New(512),
	}
	h.recordings = newRecordingManager(h)
	h.replays = newReplayManager(h)
	h.commands = adbcommand.New(adbPath, h.gateFor)
	return h
}

func (h *Hub) gateFor(serial string) *devicegate.Gate {
	h.gateMu.Lock()
	defer h.gateMu.Unlock()
	if gate := h.gates[serial]; gate != nil {
		return gate
	}
	gate := devicegate.New(int(envPositiveInt64("ADB_MAX_PARALLEL_PER_SERIAL", 2)))
	h.gates[serial] = gate
	return gate
}

func (h *Hub) recordEvent(e debuglog.Event) {
	if h.events != nil {
		h.events.Append(e)
	}
}

// SubmitAction submits an action to the active device session.
func (h *Hub) SubmitAction(ctx context.Context, serial string, req action.Request) (action.Result, error) {
	if serial == "" {
		return action.Result{}, errors.New("missing serial")
	}
	ms, _, release, err := h.acquireSession(serial)
	if err != nil {
		return action.Result{}, err
	}
	defer release()
	req.DeviceID = serial
	res := <-ms.actions.Submit(ctx, req)
	return res, nil
}

// allocPort 分配一个唯一的本地端口给 forward。
func (h *Hub) allocPort() int {
	h.portMu.Lock()
	defer h.portMu.Unlock()
	p := h.nextPort
	h.nextPort++
	return p
}

type session struct {
	server *scrcpy.Server
	conn   *scrcpy.Connection
}

func (s *session) close() {
	if s == nil {
		return
	}
	if s.conn != nil {
		s.conn.Close()
	}
	if s.server != nil {
		s.server.Stop()
		s.server.RemoveForward()
		s.server.Wait()
	}
}

// ServeWS 处理 WebSocket 连接。query: serial=xxx
func (h *Hub) ServeWS(w http.ResponseWriter, r *http.Request) {
	serial := r.URL.Query().Get("serial")
	if serial == "" {
		http.Error(w, "missing serial", http.StatusBadRequest)
		return
	}

	c, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		log.Printf("[ws] upgrade %s: %v", serial, err)
		return
	}
	defer c.Close()
	c.SetReadLimit(1 << 20)

	log.Printf("[ws] 客户端连接 serial=%s", serial)

	ms, meta, cancel, err := h.acquireSession(serial)
	if err != nil {
		log.Printf("[ws] 启动 server 失败 serial=%s: %v", serial, err)
		writeJSON(c, map[string]interface{}{"type": "error", "message": "启动 scrcpy 失败: " + err.Error()})
		return
	}
	defer cancel()

	log.Printf("[ws] server 就绪 serial=%s codec=%s %dx%d",
		serial, meta.Codec, meta.Width, meta.Height)

	writeJSON(c, meta)

	done := make(chan struct{})
	var wg sync.WaitGroup
	wg.Add(2)

	// video 帧 → WS 写
	go func() {
		defer wg.Done()
		h.pumpSharedVideo(c, ms, done)
	}()

	// WS 控制消息 → control socket
	go func() {
		defer wg.Done()
		defer close(done)
		h.pumpControl(c, ms)
	}()

	wg.Wait()
	log.Printf("[ws] 会话结束 serial=%s", serial)
}

type handshakeMeta struct {
	Type           string `json:"type"`
	Codec          string `json:"codec"`
	Width          uint32 `json:"width"`
	Height         uint32 `json:"height"`
	Serial         string `json:"serial"`
	SessionID      string `json:"session_id"`
	AudioAvailable bool   `json:"audio_available"`
	AudioCodec     string `json:"audio_codec,omitempty"`
	AudioReason    string `json:"audio_reason,omitempty"`
}

func newSessionID() string {
	var b [12]byte
	if _, err := rand.Read(b[:]); err != nil {
		return fmt.Sprintf("session-%d", time.Now().UnixNano())
	}
	return hex.EncodeToString(b[:])
}

func codecName(codecID uint32) string {
	switch codecID {
	case scrcpy.CodecIDH264:
		return "h264"
	case scrcpy.CodecIDH265:
		return "h265"
	case scrcpy.CodecIDAV1:
		return "av1"
	default:
		return "unknown"
	}
}

func writeJSON(c *websocket.Conn, v interface{}) {
	_ = c.WriteJSON(v)
}

// startSession 完成环境探测/缓存 → (按需 push) → forward → start → 快速探测 dial 全流程。
func (h *Hub) startSession(serial string) (*session, *handshakeMeta, error) {
	sess, meta, skippedPush, err := h.startSessionOnce(serial, false)
	if err != nil && skippedPush {
		log.Printf("[scrcpy:%s] 免推启动失败 (%v)，强制重新 push jar 并重试", serial, err)
		h.invalidateJarReady(serial)
		sess, meta, _, err = h.startSessionOnce(serial, true)
	}
	return sess, meta, err
}

func (h *Hub) startSessionOnce(serial string, forcePush bool) (*session, *handshakeMeta, bool, error) {
	t0 := time.Now()
	sdk, needPush := h.resolveDeviceEnv(serial, forcePush)

	cfg := scrcpy.DefaultConfig(serial)
	audioReason := ""
	// Android 11 (API 30) is the first supported release for scrcpy's direct
	// device-output audio capture. Keep older devices on the established
	// video/control path instead of letting an unavailable audio stream prevent
	// screen mirroring altogether.
	if sdk > 0 && sdk < 30 {
		cfg.Audio = false
		audioReason = "audio_unsupported_android"
	}
	server := scrcpy.NewServer(cfg, h.adbPath, h.allocPort())

	skippedPush := !needPush
	if needPush {
		if err := server.Push(h.jarPath); err != nil {
			return nil, nil, false, fmt.Errorf("push jar: %w", err)
		}
	}
	h.markJarReady(serial, sdk)

	if err := server.Forward(); err != nil {
		return nil, nil, skippedPush, fmt.Errorf("forward: %w", err)
	}

	if err := server.Start(); err != nil {
		server.RemoveForward()
		return nil, nil, skippedPush, fmt.Errorf("start server: %w", err)
	}

	// 快速探测 server 就绪：移除原 1500ms 固定死等，初始等待 40ms 后以 50ms~100ms 间隔探测
	time.Sleep(40 * time.Millisecond)

	var conn *scrcpy.Connection
	var dialErr error
	deadline := time.Now().Add(8 * time.Second)
	for attempt := 0; time.Now().Before(deadline); attempt++ {
		conn, dialErr = scrcpy.Dial(server.LocalPort(), cfg.Audio, cfg.Control, 1500*time.Millisecond)
		if dialErr == nil {
			break
		}
		select {
		case <-server.Exited():
			dialErr = fmt.Errorf("server exited early (%v): %w", server.Wait(), dialErr)
			goto dialFinished
		default:
		}
		sleepDur := 50 * time.Millisecond
		if attempt >= 30 {
			sleepDur = 100 * time.Millisecond
		}
		time.Sleep(sleepDur)
	}
dialFinished:
	if conn == nil {
		server.Stop()
		server.RemoveForward()
		return nil, nil, skippedPush, fmt.Errorf("dial scrcpy: %w", dialErr)
	}

	w, hh := conn.Size()
	meta := &handshakeMeta{
		Type:           "meta",
		Codec:          codecName(conn.CodecID()),
		Width:          w,
		Height:         hh,
		Serial:         serial,
		SessionID:      newSessionID(),
		AudioAvailable: cfg.Audio && conn.AudioCodecID() == scrcpy.CodecIDAAC,
		AudioReason:    audioReason,
	}
	if meta.AudioAvailable {
		meta.AudioCodec = "aac"
	}
	log.Printf("[scrcpy:%s] startSession 就绪耗时=%v skippedPush=%v size=%dx%d",
		serial, time.Since(t0).Round(time.Millisecond), skippedPush, w, hh)
	return &session{server: server, conn: conn}, meta, skippedPush, nil
}

func (h *Hub) resolveDeviceEnv(serial string, forcePush bool) (sdk int, needPush bool) {
	h.envMu.Lock()
	env := h.deviceEnvs[serial]
	h.envMu.Unlock()

	if !forcePush && env.sdk > 0 && env.jarReady {
		return env.sdk, false
	}

	sdk = env.sdk
	var remoteJarSize int64 = -1
	if !forcePush {
		if pSDK, pSize, err := probeDeviceEnv(h.adbPath, serial); err == nil {
			if pSDK > 0 {
				sdk = pSDK
			}
			remoteJarSize = pSize
		}
	}
	if sdk <= 0 {
		if s, err := androidSDK(h.adbPath, serial); err == nil && s > 0 {
			sdk = s
		}
	}

	if !forcePush && h.jarSize > 0 && remoteJarSize == h.jarSize {
		log.Printf("[scrcpy:%s] 设备端 jar 已存在且大小匹配 (%d bytes)，跳过 push", serial, remoteJarSize)
		return sdk, false
	}
	return sdk, true
}

func (h *Hub) markJarReady(serial string, sdk int) {
	h.envMu.Lock()
	defer h.envMu.Unlock()
	prev := h.deviceEnvs[serial]
	if sdk <= 0 {
		sdk = prev.sdk
	}
	h.deviceEnvs[serial] = deviceEnv{sdk: sdk, jarReady: true}
}

func (h *Hub) invalidateJarReady(serial string) {
	h.envMu.Lock()
	defer h.envMu.Unlock()
	env := h.deviceEnvs[serial]
	env.jarReady = false
	h.deviceEnvs[serial] = env
}

// probeDeviceEnv 在单次 adb shell 往返中同时获取 Android SDK 版本与设备端 jar 文件大小。
func probeDeviceEnv(adbPath, serial string) (sdk int, jarSize int64, err error) {
	script := "getprop ro.build.version.sdk; stat -c %s " + scrcpy.DeviceServerPath + " 2>/dev/null || true"
	out, err := exec.Command(adbPath, "-s", serial, "shell", script).Output()
	if err != nil {
		return 0, -1, err
	}
	lines := strings.Split(strings.ReplaceAll(string(out), "\r\n", "\n"), "\n")
	var nonEmpty []string
	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		if trimmed != "" {
			nonEmpty = append(nonEmpty, trimmed)
		}
	}
	if len(nonEmpty) == 0 {
		return 0, -1, errors.New("empty probe output")
	}
	sdk, err = strconv.Atoi(nonEmpty[0])
	if err != nil || sdk <= 0 {
		return 0, -1, fmt.Errorf("invalid Android SDK level %q", nonEmpty[0])
	}
	jarSize = -1
	if len(nonEmpty) >= 2 {
		if sz, sErr := strconv.ParseInt(nonEmpty[1], 10, 64); sErr == nil && sz > 0 {
			jarSize = sz
		}
	}
	return sdk, jarSize, nil
}

func androidSDK(adbPath, serial string) (int, error) {
	out, err := exec.Command(adbPath, "-s", serial, "shell", "getprop", "ro.build.version.sdk").Output()
	if err != nil {
		return 0, err
	}
	sdk, err := strconv.Atoi(strings.TrimSpace(string(out)))
	if err != nil || sdk <= 0 {
		return 0, fmt.Errorf("invalid Android SDK level %q", strings.TrimSpace(string(out)))
	}
	return sdk, nil
}

// pumpVideo 读 video 帧并推给浏览器。
// WS 帧格式: [1B kind][8B pts BE][payload...]
func (h *Hub) pumpVideo(c *websocket.Conn, sess *session, done <-chan struct{}) {
	buf := make([]byte, 0, 256*1024)
	for {
		select {
		case <-done:
			return
		default:
		}

		frame, err := sess.conn.ReadFrame()
		if err != nil {
			log.Printf("[ws] video 读取结束: %v", err)
			// 通知浏览器需要重连（旋转卡死、连接断开等）
			writeJSON(c, map[string]interface{}{"type": "disconnected"})
			return
		}

		buf = buf[:0]
		buf = append(buf, byte(frame.Kind))
		var ptsBuf [8]byte
		binary.BigEndian.PutUint64(ptsBuf[:], frame.PTS)
		buf = append(buf, ptsBuf[:]...)

		switch frame.Kind {
		case scrcpy.FrameSession:
			var dim [8]byte
			binary.BigEndian.PutUint32(dim[0:4], frame.Width)
			binary.BigEndian.PutUint32(dim[4:8], frame.Height)
			buf = append(buf, dim[:]...)
		default:
			buf = append(buf, frame.Payload...)
		}

		c.SetWriteDeadline(time.Now().Add(3 * time.Second))
		if err := c.WriteMessage(websocket.BinaryMessage, buf); err != nil {
			log.Printf("[ws] video 写入失败: %v", err)
			return
		}
	}
}

// pumpControl 读浏览器发来的控制消息并写入 control socket。
func (h *Hub) pumpControl(c *websocket.Conn, ms *managedSession) {
	for {
		_, data, err := c.ReadMessage()
		if err != nil {
			if websocket.IsCloseError(err, websocket.CloseNormalClosure, websocket.CloseGoingAway) {
				return
			}
			log.Printf("[ws] control 读取结束: %v", err)
			return
		}
		if len(data) == 0 {
			continue
		}
		result := <-ms.actions.Submit(context.Background(), action.Request{DeviceID: ms.meta.Serial, Source: "browser", Action: "raw", Raw: append([]byte(nil), data...), ExpiresMS: 3000})
		if !result.Executed && result.ErrorCode != "" {
			log.Printf("[ws] 写 control 失败: %s", result.ErrorCode)
			return
		}
	}
}

var _ = errors.New
