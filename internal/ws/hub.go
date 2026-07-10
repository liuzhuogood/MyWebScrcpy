package ws

import (
	"encoding/binary"
	"errors"
	"fmt"
	"log"
	"net/http"
	"sync"
	"time"

	"mywebscrcpy/internal/scrcpy"

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
	adbPath  string
	jarPath  string
	portMu   sync.Mutex
	nextPort int
}

func NewHub(adbPath, jarPath string) *Hub {
	return &Hub{adbPath: adbPath, jarPath: jarPath, nextPort: 27183}
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

	// 启动 scrcpy server
	sess, meta, err := h.startSession(serial)
	if err != nil {
		log.Printf("[ws] 启动 server 失败 serial=%s: %v", serial, err)
		writeJSON(c, map[string]interface{}{"type": "error", "message": "启动 scrcpy 失败: " + err.Error()})
		return
	}
	defer sess.close()

	log.Printf("[ws] server 就绪 serial=%s codec=%s %dx%d",
		serial, meta.Codec, meta.Width, meta.Height)

	writeJSON(c, meta)

	done := make(chan struct{})
	var wg sync.WaitGroup
	wg.Add(2)

	// video 帧 → WS 写
	go func() {
		defer wg.Done()
		h.pumpVideo(c, sess, done)
	}()

	// WS 控制消息 → control socket
	go func() {
		defer wg.Done()
		defer close(done)
		h.pumpControl(c, sess)
	}()

	wg.Wait()
	log.Printf("[ws] 会话结束 serial=%s", serial)
}

type handshakeMeta struct {
	Type   string `json:"type"`
	Codec  string `json:"codec"`
	Width  uint32 `json:"width"`
	Height uint32 `json:"height"`
	Serial string `json:"serial"`
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
		conn, dialErr = scrcpy.Dial(server.LocalPort(), cfg.Control, 3*time.Second)
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
		Type:   "meta",
		Codec:  codecName(conn.CodecID()),
		Width:  w,
		Height: hh,
		Serial: serial,
	}
	return &session{server: server, conn: conn}, meta, nil
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
func (h *Hub) pumpControl(c *websocket.Conn, sess *session) {
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
		if err := sess.conn.WriteControl(data); err != nil {
			log.Printf("[ws] 写 control 失败: %v", err)
			return
		}
	}
}

var _ = errors.New
