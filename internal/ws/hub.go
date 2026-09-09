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

// Hub 管理所有设备的 scrcpy server 会话。
type Hub struct {
	adbPath    string
	jarPath    string
	portMu     sync.Mutex
	nextPort   int
	resultMu   sync.Mutex
	results    map[string]map[chan vision.Message]string
	lastResult map[string]vision.Message
	events     *debuglog.Ring
	sessionMu  sync.Mutex
	sessions   map[string]*managedSession
	recordings *recordingManager
	gates      map[string]*devicegate.Gate
	gateMu     sync.Mutex
	commands   *adbcommand.Manager
}

func NewHub(adbPath, jarPath string) *Hub {
	h := &Hub{adbPath: adbPath, jarPath: jarPath, nextPort: 27183, results: make(map[string]map[chan vision.Message]string), lastResult: make(map[string]vision.Message), sessions: make(map[string]*managedSession), gates: make(map[string]*devicegate.Gate), events: debuglog.New(512)}
	h.recordings = newRecordingManager(h)
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

// startSession 完成 push → forward → start → dial 全流程。
func (h *Hub) startSession(serial string) (*session, *handshakeMeta, error) {
	cfg := scrcpy.DefaultConfig(serial)
	audioReason := ""
	// Android 11 (API 30) is the first supported release for scrcpy's direct
	// device-output audio capture. Keep older devices on the established
	// video/control path instead of letting an unavailable audio stream prevent
	// screen mirroring altogether.
	if sdk, err := androidSDK(h.adbPath, serial); err == nil && sdk < 30 {
		cfg.Audio = false
		audioReason = "audio_unsupported_android"
	}
	server := scrcpy.NewServer(cfg, h.adbPath, h.allocPort())

	if err := server.Push(h.jarPath); err != nil {
		return nil, nil, fmt.Errorf("push jar: %w", err)
	}

	if err := server.Forward(); err != nil {
		return nil, nil, fmt.Errorf("forward: %w", err)
	}

	if err := server.Start(); err != nil {
		return nil, nil, fmt.Errorf("start server: %w", err)
	}

	// 等 server 就绪 (JVM 冷启动约 1~2 秒)
	time.Sleep(1500 * time.Millisecond)

	var conn *scrcpy.Connection
	var dialErr error
	for i := 0; i < 15; i++ {
		conn, dialErr = scrcpy.Dial(server.LocalPort(), cfg.Audio, cfg.Control, 3*time.Second)
		if dialErr == nil {
			break
		}
		time.Sleep(500 * time.Millisecond)
	}
	if conn == nil {
		server.Stop()
		server.RemoveForward()
		return nil, nil, fmt.Errorf("dial scrcpy: %w", dialErr)
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
	return &session{server: server, conn: conn}, meta, nil
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
