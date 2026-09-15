//go:build windows

package adbshell

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/exec"
	"strconv"
	"sync"
	"sync/atomic"
	"time"

	"github.com/gorilla/websocket"
)

var upgrader = websocket.Upgrader{
	ReadBufferSize:  4096,
	WriteBufferSize: 4096,
	CheckOrigin:     func(r *http.Request) bool { return true },
}

// IsOnline 是注入的设备在线检查函数，避免循环依赖 ws.Hub。
type IsOnline func(serial string) bool

type clientMsg struct {
	Type string `json:"type"`
	Data string `json:"data,omitempty"`
	Cols uint16 `json:"cols,omitempty"`
	Rows uint16 `json:"rows,omitempty"`
}

// sessionEntry 记录当前 session 的唯一 token 和 cancel 函数。
type sessionEntry struct {
	token  uint64
	cancel context.CancelFunc
}

var (
	sessionMu  sync.Mutex
	sessions   = make(map[string]sessionEntry)
	sessionSeq uint64
)

func idleTimeout() time.Duration {
	if v, err := strconv.Atoi(os.Getenv("ADB_SHELL_IDLE_TIMEOUT_MIN")); err == nil && v > 0 {
		return time.Duration(v) * time.Minute
	}
	return 30 * time.Minute
}

func decodeInput(data string) []byte {
	if decoded, err := base64.StdEncoding.DecodeString(data); err == nil && len(decoded) > 0 {
		return decoded
	}
	return []byte(data)
}

// Handler 返回 Windows 下针对 /ws/adb-shell 的管道模式 http.HandlerFunc。
func Handler(adbPath string, online IsOnline) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		serial := r.URL.Query().Get("serial")
		if serial == "" {
			http.Error(w, "missing serial", http.StatusBadRequest)
			return
		}
		if !online(serial) {
			http.Error(w, "device not online", http.StatusServiceUnavailable)
			return
		}

		conn, err := upgrader.Upgrade(w, r, nil)
		if err != nil {
			log.Printf("[adbshell] upgrade: %v", err)
			return
		}
		defer conn.Close()

		var writeMu sync.Mutex
		safeWrite := func(messageType int, data []byte) error {
			writeMu.Lock()
			defer writeMu.Unlock()
			return conn.WriteMessage(messageType, data)
		}

		// 关闭同 serial 上的旧 session，为本次 session 分配唯一 token。
		ctx, cancel := context.WithCancel(r.Context())
		defer cancel()
		myToken := atomic.AddUint64(&sessionSeq, 1)
		sessionMu.Lock()
		if old, ok := sessions[serial]; ok {
			old.cancel()
		}
		sessions[serial] = sessionEntry{token: myToken, cancel: cancel}
		sessionMu.Unlock()
		defer func() {
			sessionMu.Lock()
			if cur, ok := sessions[serial]; ok && cur.token == myToken {
				delete(sessions, serial)
			}
			sessionMu.Unlock()
		}()

		// 启动 adb shell 并建立管道
		cmd := exec.CommandContext(ctx, adbPath, "-s", serial, "shell")
		stdinPipe, err := cmd.StdinPipe()
		if err != nil {
			_ = safeWrite(websocket.TextMessage, []byte(fmt.Sprintf("\r\n\033[31m[adb-shell] stdin pipe 失败: %v\033[0m\r\n", err)))
			return
		}
		defer stdinPipe.Close()

		stdoutPipe, err := cmd.StdoutPipe()
		if err != nil {
			_ = safeWrite(websocket.TextMessage, []byte(fmt.Sprintf("\r\n\033[31m[adb-shell] stdout pipe 失败: %v\033[0m\r\n", err)))
			return
		}
		cmd.Stderr = cmd.Stdout

		if err := cmd.Start(); err != nil {
			_ = safeWrite(websocket.TextMessage, []byte(fmt.Sprintf("\r\n\033[31m[adb-shell] 启动命令失败: %v\033[0m\r\n", err)))
			return
		}
		defer func() {
			if cmd.Process != nil {
				_ = cmd.Process.Kill()
				_ = cmd.Wait()
			}
		}()

		// 连接成功提示
		_ = safeWrite(websocket.TextMessage, []byte("\r\n\033[33m[adb-shell] Windows 管道模式已启动（支持标准命令行交互）\033[0m\r\n"))

		idle := time.NewTimer(idleTimeout())
		defer idle.Stop()
		resetIdle := func() {
			if !idle.Stop() {
				select {
				case <-idle.C:
				default:
				}
			}
			idle.Reset(idleTimeout())
		}

		// goroutine A：stdoutPipe → WebSocket binary 帧。
		go func() {
			buf := make([]byte, 4096)
			for {
				n, err := stdoutPipe.Read(buf)
				if n > 0 {
					if werr := safeWrite(websocket.BinaryMessage, buf[:n]); werr != nil {
						cancel()
						return
					}
					resetIdle()
				}
				if err != nil {
					cancel()
					return
				}
			}
		}()

		// 主循环：读取 WebSocket 消息并分发到 stdinPipe
		for {
			select {
			case <-ctx.Done():
				return
			case <-idle.C:
				_ = safeWrite(websocket.TextMessage, []byte("\r\n\033[31m[adb-shell] idle timeout\033[0m\r\n"))
				return
			default:
			}

			conn.SetReadDeadline(time.Now().Add(35 * time.Minute))
			_, raw, err := conn.ReadMessage()
			if err != nil {
				return
			}

			var msg clientMsg
			if err := json.Unmarshal(raw, &msg); err != nil {
				continue
			}

			switch msg.Type {
			case "input":
				payload := decodeInput(msg.Data)
				if _, err := stdinPipe.Write(payload); err != nil {
					return
				}
				resetIdle()
			case "resize":
				// Windows 管道模式下静默忽略或无操作，不报错
			}
		}
	}
}
