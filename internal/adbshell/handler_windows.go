//go:build windows

package adbshell

import (
	"net/http"

	"github.com/gorilla/websocket"
)

var upgrader = websocket.Upgrader{
	CheckOrigin: func(r *http.Request) bool { return true },
}

// IsOnline 是注入的设备在线检查函数，避免循环依赖 ws.Hub。
type IsOnline func(serial string) bool

// Handler 返回 Windows 下针对 /ws/adb-shell 的占位 http.HandlerFunc。
func Handler(adbPath string, online IsOnline) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		conn, err := upgrader.Upgrade(w, r, nil)
		if err != nil {
			http.Error(w, "ADB shell interactive PTY is not supported on Windows", http.StatusNotImplemented)
			return
		}
		defer conn.Close()
		_ = conn.WriteMessage(websocket.TextMessage, []byte("\r\n\033[31m[adb-shell] ADB shell interactive PTY is not supported on Windows\033[0m\r\n"))
	}
}
