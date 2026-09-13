package ws

import (
	"encoding/binary"
	"time"
)

// touch.go — 触摸可视化事件广播。
//
// 设备动作执行器（tap/swipe/raw 触摸注入）成功写入控制通道后，把归一化
// 坐标的触摸事件发布给订阅者；浏览器通过 /api/vision/results 复用通道
// 接收，在投屏画面上绘制类似安卓"显示触摸"的波纹与轨迹。

// TouchEventMessage 一条触摸可视化事件。坐标均为 0~1 归一化值。
// Kind: tap（单击）/ swipe（拖动，带终点）/ down / move / up（原始触摸轨迹）。
type TouchEventMessage struct {
	Type      string  `json:"type"` // 恒为 "touch.event"
	DeviceID  string  `json:"device_id"`
	SessionID string  `json:"session_id"`
	Kind      string  `json:"kind"`
	X         float64 `json:"x"`
	Y         float64 `json:"y"`
	X2        float64 `json:"x2,omitempty"`
	Y2        float64 `json:"y2,omitempty"`
	Source    string  `json:"source,omitempty"`
	Timestamp int64   `json:"timestamp"`
}

const touchEventQueueSize = 64

// touchPublisher 绑定设备与会话，由动作执行器调用。
type touchPublisher struct {
	h         *Hub
	serial    string
	sessionID string
}

func (p touchPublisher) emit(kind string, x, y, x2, y2 float64, source string) {
	if p.h == nil {
		return
	}
	p.h.publishTouch(TouchEventMessage{
		Type:      "touch.event",
		DeviceID:  p.serial,
		SessionID: p.sessionID,
		Kind:      kind,
		X:         x,
		Y:         y,
		X2:        x2,
		Y2:        y2,
		Source:    source,
		Timestamp: time.Now().UnixMilli(),
	}, p.sessionID)
}

func (h *Hub) publishTouch(msg TouchEventMessage, sessionID string) {
	h.touchMu.Lock()
	defer h.touchMu.Unlock()
	for ch, sub := range h.touchSubs[msg.DeviceID] {
		if sub != "" && sub != sessionID {
			continue
		}
		select {
		case ch <- msg:
		default: // 订阅者消费不过来时丢弃最旧的一条，保住最新事件
			select {
			case <-ch:
			default:
			}
			select {
			case ch <- msg:
			default:
			}
		}
	}
}

func (h *Hub) subscribeTouch(serial, sessionID string) (chan TouchEventMessage, func()) {
	ch := make(chan TouchEventMessage, touchEventQueueSize)
	h.touchMu.Lock()
	if h.touchSubs == nil {
		h.touchSubs = make(map[string]map[chan TouchEventMessage]string)
	}
	if h.touchSubs[serial] == nil {
		h.touchSubs[serial] = make(map[chan TouchEventMessage]string)
	}
	h.touchSubs[serial][ch] = sessionID
	h.touchMu.Unlock()
	return ch, func() {
		h.touchMu.Lock()
		defer h.touchMu.Unlock()
		if subs := h.touchSubs[serial]; subs != nil {
			delete(subs, ch)
			if len(subs) == 0 {
				delete(h.touchSubs, serial)
			}
		}
	}
}

// rawTouchEvent 从浏览器透传的原始控制字节里解析出的一条触摸事件。
type rawTouchEvent struct {
	kind     string
	x, y     float64 // 归一化坐标
}

// parseRawTouch 识别 scrcpy 触摸注入消息（type=2，32 字节，布局见
// scrcpy.TouchEvent），非触摸消息返回 ok=false。
func parseRawTouch(raw []byte) (rawTouchEvent, bool) {
	if len(raw) < 32 || raw[0] != 2 {
		return rawTouchEvent{}, false
	}
	touchAction := raw[1]
	x := float64(binary.BigEndian.Uint32(raw[10:14]))
	y := float64(binary.BigEndian.Uint32(raw[14:18]))
	w := float64(binary.BigEndian.Uint16(raw[18:20]))
	h := float64(binary.BigEndian.Uint16(raw[20:22]))
	if w < 1 || h < 1 {
		return rawTouchEvent{}, false
	}
	var kind string
	switch touchAction {
	case 0: // ActionDown
		kind = "down"
	case 2: // ActionMove
		kind = "move"
	case 1: // ActionUp
		kind = "up"
	default:
		return rawTouchEvent{}, false
	}
	return rawTouchEvent{kind: kind, x: x / w, y: y / h}, true
}
