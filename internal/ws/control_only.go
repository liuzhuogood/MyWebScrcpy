package ws

import (
	"time"

	"github.com/gorilla/websocket"
)

// 群控后台连接只接收真实尺寸与心跳，不订阅视频、音频和剪贴板。
// 每台设备仍使用自身的有界动作队列，慢设备不会阻塞其他设备。
func (h *Hub) pumpSharedControl(c *websocket.Conn, ms *managedSession, done <-chan struct{}) {
	heartbeat := time.NewTicker(2 * time.Second)
	defer heartbeat.Stop()
	for {
		select {
		case <-done:
			return
		case <-heartbeat.C:
			// 旧会话失效后不能继续以旧尺寸保活；客户端需重新获取会话。
			h.sessionMu.Lock()
			active := !h.closed && h.sessions[ms.meta.Serial] == ms
			h.sessionMu.Unlock()
			if !active {
				c.SetWriteDeadline(timeNow())
				_ = c.WriteJSON(map[string]string{"type": "disconnected"})
				_ = c.Close()
				return
			}
			ms.mu.Lock()
			width, height := ms.width, ms.height
			ms.mu.Unlock()
			c.SetWriteDeadline(timeNow())
			if err := c.WriteJSON(map[string]interface{}{"type": "control.meta", "width": width, "height": height}); err != nil {
				_ = c.Close()
				return
			}
		}
	}
}
