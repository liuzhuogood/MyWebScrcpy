// Package adbshell 提供通过 WebSocket 桥接 adb shell PTY 的 HTTP handler。
//
// 路由：GET /ws/adb-shell?serial=<serial>
//
// 协议：
//   - 客户端 → 服务端：JSON 文本帧
//     - 输入：{"type":"input","data":"<raw bytes base64 or string>"}
//     - resize：{"type":"resize","cols":120,"rows":35}
//   - 服务端 → 客户端：原始 PTY 输出（binary 帧），xterm.js 直接 term.write()。
//
// 安全约束：
//   - serial 在握手时从 URL query 绑定，连接期间不可修改。
//   - 命令固定为 adb -s <serial> shell，不暴露任意命令执行。
//   - 每个 serial 同时只允许一个 shell session；新连接会关闭旧连接。
//   - WebSocket 关闭时 SIGKILL 整个进程组，不产生僵尸进程。
//   - 30 分钟无活动自动关闭（可通过 ADB_SHELL_IDLE_TIMEOUT_MIN 环境变量覆盖）。

//go:build !windows

package adbshell

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/exec"
	"strconv"
	"sync"
	"sync/atomic"
	"syscall"
	"time"

	"github.com/creack/pty"
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
// token 用于 cleanup 时比对"当前 session 是否仍是我"，
// 避免 Go 不允许比较函数值的限制。
type sessionEntry struct {
	token  uint64
	cancel context.CancelFunc
}

var (
	sessionMu  sync.Mutex
	sessions   = make(map[string]sessionEntry)
	sessionSeq uint64 // 通过 atomic 自增生成唯一 token
)

func idleTimeout() time.Duration {
	if v, err := strconv.Atoi(os.Getenv("ADB_SHELL_IDLE_TIMEOUT_MIN")); err == nil && v > 0 {
		return time.Duration(v) * time.Minute
	}
	return 30 * time.Minute
}

// Handler 返回处理 /ws/adb-shell 的 http.HandlerFunc。
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

		// 启动 adb shell 并分配 PTY。
		cmd := exec.CommandContext(ctx, adbPath, "-s", serial, "shell")
		cmd.SysProcAttr = &syscall.SysProcAttr{Setsid: true}
		ptmx, err := pty.Start(cmd)
		if err != nil {
			writeError(conn, fmt.Sprintf("PTY 启动失败: %v", err))
			return
		}
		defer func() {
			ptmx.Close()
			if cmd.Process != nil {
				// SIGKILL 整个进程组，确保不残留子进程。
				syscall.Kill(-cmd.Process.Pid, syscall.SIGKILL)
				cmd.Wait()
			}
		}()

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

		// goroutine A：PTY stdout → WebSocket binary 帧。
		go func() {
			buf := make([]byte, 4096)
			for {
				n, err := ptmx.Read(buf)
				if n > 0 {
					if werr := conn.WriteMessage(websocket.BinaryMessage, buf[:n]); werr != nil {
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

		// 主循环：WebSocket 文本帧 → PTY stdin / resize。
		for {
			select {
			case <-ctx.Done():
				return
			case <-idle.C:
				writeError(conn, "idle timeout")
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
				if _, err := ptmx.Write([]byte(msg.Data)); err != nil {
					return
				}
				resetIdle()
			case "resize":
				cols, rows := msg.Cols, msg.Rows
				if cols == 0 {
					cols = 80
				}
				if rows == 0 {
					rows = 24
				}
				pty.Setsize(ptmx, &pty.Winsize{Cols: cols, Rows: rows})
			}
		}
	}
}

func writeError(conn *websocket.Conn, msg string) {
	conn.WriteMessage(websocket.TextMessage, []byte("\r\n\033[31m[adb-shell] "+msg+"\033[0m\r\n"))
}
