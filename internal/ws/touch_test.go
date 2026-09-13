package ws

import (
	"encoding/binary"
	"testing"

	"mywebscrcpy/internal/scrcpy"
)

func TestParseRawTouch(t *testing.T) {
	down := scrcpy.TouchEvent(scrcpy.ActionDown, scrcpy.PointerIDMouse, 300, 400, 1080, 1920, 1, 0, 0)
	ev, ok := parseRawTouch(down)
	if !ok || ev.kind != "down" || ev.x != 300.0/1080 || ev.y != 400.0/1920 {
		t.Fatalf("down 解析错误: ok=%v ev=%+v", ok, ev)
	}
	move := scrcpy.TouchEvent(scrcpy.ActionMove, scrcpy.PointerIDMouse, 600, 800, 1080, 1920, 1, 0, 0)
	if ev, ok := parseRawTouch(move); !ok || ev.kind != "move" || ev.x != 600.0/1080 {
		t.Fatalf("move 解析错误: ok=%v ev=%+v", ok, ev)
	}
	up := scrcpy.TouchEvent(scrcpy.ActionUp, scrcpy.PointerIDMouse, 600, 800, 1080, 1920, 0, 0, 0)
	if ev, ok := parseRawTouch(up); !ok || ev.kind != "up" {
		t.Fatalf("up 解析错误: ok=%v ev=%+v", ok, ev)
	}
	// 非触摸消息必须被拒绝
	if _, ok := parseRawTouch(scrcpy.KeyCodeEvent(scrcpy.KeyActionDown, 25, 0, 0)); ok {
		t.Fatal("keycode 消息不应被解析为触摸")
	}
	short := make([]byte, 10)
	short[0] = 2
	if _, ok := parseRawTouch(short); ok {
		t.Fatal("截断消息不应被解析为触摸")
	}
}

func TestPublishTouchSessionFilter(t *testing.T) {
	h := NewHub("", "")
	chA, unsubA := h.subscribeTouch("dev-1", "s1")
	defer unsubA()
	chB, unsubB := h.subscribeTouch("dev-1", "s2")
	defer unsubB()

	h.publishTouch(TouchEventMessage{Type: "touch.event", DeviceID: "dev-1", SessionID: "s1", Kind: "tap", X: 0.5, Y: 0.5}, "s1")
	select {
	case msg := <-chA:
		if msg.Kind != "tap" {
			t.Fatalf("订阅者 A 收到错误事件: %+v", msg)
		}
	default:
		t.Fatal("同会话订阅者 A 应收到事件")
	}
	select {
	case msg := <-chB:
		t.Fatalf("其他会话订阅者 B 不应收到事件: %+v", msg)
	default:
	}
}

func TestPublishTouchDropOldest(t *testing.T) {
	h := NewHub("", "")
	ch, unsub := h.subscribeTouch("dev-1", "")
	defer unsub()
	for i := 0; i < touchEventQueueSize+10; i++ {
		h.publishTouch(TouchEventMessage{Type: "touch.event", DeviceID: "dev-1", Kind: "move", X: float64(i)}, "")
	}
	// 队列满后丢最旧；读空队列，最后一条应为最新事件
	var last TouchEventMessage
	n := 0
	for {
		select {
		case last = <-ch:
			n++
			continue
		default:
		}
		break
	}
	if n == 0 || last.X != float64(touchEventQueueSize+9) {
		t.Fatalf("最新事件应保留: 收到 %d 条, last=%+v", n, last)
	}
}

// 确保 scrcpy.TouchEvent 的字节布局与 parseRawTouch 的假设一致。
func TestTouchEventLayoutStable(t *testing.T) {
	b := scrcpy.TouchEvent(2, 7, 100, 200, 1080, 1920, 0.5, 0, 0)
	if b[0] != 2 || binary.BigEndian.Uint32(b[10:14]) != 100 || binary.BigEndian.Uint32(b[14:18]) != 200 || binary.BigEndian.Uint16(b[18:20]) != 1080 || binary.BigEndian.Uint16(b[20:22]) != 1920 {
		t.Fatalf("TouchEvent 布局变化，parseRawTouch 需同步更新")
	}
}
