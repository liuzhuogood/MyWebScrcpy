package scrcpy

import (
	"encoding/binary"
	"math"
)

// 压力归一化值 0.0~1.0 转 uint16 定点 (0xFFFF = 1.0)
func pressureU16(p float64) uint16 {
	if p <= 0 {
		return 0
	}
	if p >= 1.0 {
		return 0xFFFF
	}
	return uint16(math.Round(p * 0xFFFF))
}

// 滚动量归一化：clamp(v/16, -1, 1) 再转 int16 定点。
// 设备端会把 wire 上的 int16 / 32768 * 16 还原为实际档数。
func scrollI16(v float64) uint16 {
	norm := v / 16
	if norm > 1 {
		norm = 1
	} else if norm < -1 {
		norm = -1
	}
	if norm >= 1.0 {
		return 0x7FFF
	}
	if norm <= -1.0 {
		return 0x8000
	}
	return uint16(int16(norm * 32768))
}

// TouchEvent 构造一条触摸注入消息 (type=2，共 32 字节)。
//
// action: ActionDown / ActionMove / ActionUp
// pointerID: 触摸点 id；鼠标用 PointerIDMouse，手指用 PointerIDFinger 或自定义递增 id
// x, y: 设备像素绝对坐标
// screenW, screenH: 当前设备分辨率
// pressure: 0.0~1.0
// actionButton / buttons: 鼠标按钮位掩码 (可为 0)
func TouchEvent(action uint8, pointerID uint64, x, y int32,
	screenW, screenH uint16, pressure float64, actionButton, buttons uint32) []byte {
	b := make([]byte, 32)
	b[0] = TypeInjectTouchEvent
	b[1] = action
	binary.BigEndian.PutUint64(b[2:10], pointerID)
	binary.BigEndian.PutUint32(b[10:14], uint32(x))
	binary.BigEndian.PutUint32(b[14:18], uint32(y))
	binary.BigEndian.PutUint16(b[18:20], screenW)
	binary.BigEndian.PutUint16(b[20:22], screenH)
	binary.BigEndian.PutUint16(b[22:24], pressureU16(pressure))
	binary.BigEndian.PutUint32(b[24:28], actionButton)
	binary.BigEndian.PutUint32(b[28:32], buttons)
	return b
}

// ScrollEvent 构造一条滚动注入消息 (type=3，共 21 字节)。
// hscroll / vscroll: 实际滚动档数 (±16 范围)
func ScrollEvent(x, y int32, screenW, screenH uint16, hscroll, vscroll float64, buttons uint32) []byte {
	b := make([]byte, 21)
	b[0] = TypeInjectScrollEvent
	binary.BigEndian.PutUint32(b[1:5], uint32(x))
	binary.BigEndian.PutUint32(b[5:9], uint32(y))
	binary.BigEndian.PutUint16(b[9:11], screenW)
	binary.BigEndian.PutUint16(b[11:13], screenH)
	binary.BigEndian.PutUint16(b[13:15], scrollI16(hscroll))
	binary.BigEndian.PutUint16(b[15:17], scrollI16(vscroll))
	binary.BigEndian.PutUint32(b[17:21], buttons)
	return b
}

// KeyCodeEvent 构造一条按键注入消息 (type=0，共 14 字节)。
// action: KeyActionDown / KeyActionUp
// keycode: Android AKEYCODE_* 值
// metastate: 修饰键状态位掩码
func KeyCodeEvent(action uint8, keycode, repeat, metastate uint32) []byte {
	b := make([]byte, 14)
	b[0] = TypeInjectKeyCode
	b[1] = action
	binary.BigEndian.PutUint32(b[2:6], keycode)
	binary.BigEndian.PutUint32(b[6:10], repeat)
	binary.BigEndian.PutUint32(b[10:14], metastate)
	return b
}

// BackEvent 构造返回/点亮消息 (type=4，2 字节)。
// action: KeyActionDown 点亮屏幕，KeyActionUp 释放。
func BackEvent(action uint8) []byte {
	return []byte{TypeBackOrScreenOn, action}
}

// EventRaw 直接透传已经打包好的控制消息字节。
// 用于浏览器端自行打包、Go 端只做转发的场景。
func EventRaw(b []byte) []byte { return b }
