package ws

import (
	"encoding/binary"
	"math"
	"math/rand"
	"time"

	"mywebscrcpy/internal/scrcpy"
)

// hid_touch.go — HID 模式触摸注入。
//
// 通过 scrcpy 4.0 的 UHID 控制消息在设备端创建一个虚拟触摸屏 digitizer，
// 后续 tap / swipe 以 HID input report 形式注入。事件经 Linux input 子系统
// 进入 Android（evdev → InputReader），事件源是 /dev/uhid 虚拟硬件而非
// InputManager.injectInputEvent，get_source() 为 SCREEN/TOUCHPAD 类硬件源，
// 不携带 FLAG_FROM_INJECTION 等 injection 标记，更难被反自动化检测识别。
//
// digitizer report 布局（9 字节，与下方 report descriptor 一致）：
//
//	byte 0   : contact count（按下 1 / 抬起 0）
//	byte 1   : contact identifier（触点 id）
//	byte 2   : tip switch(bit0) + in range(bit1)
//	byte 3..4 : X 绝对坐标（小端 uint16，0..32767）
//	byte 5..6 : Y 绝对坐标（小端 uint16）
//	byte 7..8 : pressure 0..1023（小端）

// hidTouchReportDesc 是触摸屏 digitizer 的 HID report descriptor，
// 按 USB HID 规范编写（写法对照 scrcpy app/src/hid/hid_mouse.c）：
// Usage Page (Digitizers 0x0D) 的 Touch Screen，含 Contact Count、
// Contact Identifier、Tip Switch、In Range，以及 Generic Desktop 的
// X/Y 绝对坐标（0..32767）与 Digitizers Pressure（0..1023）。
// 与 report 布局注释一一对应，字段顺序即字节顺序。
var hidTouchReportDesc = []byte{
	0x05, 0x0D, // Usage Page (Digitizers)
	0x09, 0x04, // Usage (Touch Screen)
	0xA1, 0x01, // Collection (Application)
	0x09, 0x54, //   Usage (Contact Count)
	0x95, 0x01, //   Report Count (1)
	0x75, 0x08, //   Report Size (8)
	0x81, 0x02, //   Input (Data, Variable, Absolute)
	0x09, 0x55, //   Usage (Contact Identifier)
	0x75, 0x08, //   Report Size (8)
	0x95, 0x01, //   Report Count (1)
	0x81, 0x02, //   Input (Data, Variable, Absolute)
	0x09, 0x42, //   Usage (Tip Switch)
	0x95, 0x01, //   Report Count (1)
	0x75, 0x01, //   Report Size (1)
	0x81, 0x02, //   Input (Data, Variable, Absolute)
	0x09, 0x32, //   Usage (In Range)
	0x95, 0x01, //   Report Count (1)
	0x75, 0x01, //   Report Size (1)
	0x81, 0x02, //   Input (Data, Variable, Absolute)
	0x95, 0x06, //   Report Count (6)
	0x75, 0x01, //   Report Size (1)
	0x81, 0x01, //   Input (Constant) padding → flags 字节对齐
	0x05, 0x01, //   Usage Page (Generic Desktop)
	0x09, 0x30, //   Usage (X)
	0x15, 0x00, //   Logical Minimum (0)
	0x26, 0xFF, 0x7F, //   Logical Maximum (32767)
	0x35, 0x00, //   Physical Minimum (0)
	0x46, 0xFF, 0x7F, //   Physical Maximum (32767)
	0x75, 0x10, //   Report Size (16)
	0x95, 0x01, //   Report Count (1)
	0x81, 0x02, //   Input (Data, Variable, Absolute)
	0x09, 0x31, //   Usage (Y)
	0x81, 0x02, //   Input (Data, Variable, Absolute)
	0x05, 0x0D, //   Usage Page (Digitizers)
	0x09, 0x30, //   Usage (Pressure)
	0x15, 0x00, //   Logical Minimum (0)
	0x26, 0xFF, 0x03, //   Logical Maximum (1023)
	0x75, 0x10, //   Report Size (16)
	0x95, 0x01, //   Report Count (1)
	0x81, 0x02, //   Input (Data, Variable, Absolute)
	0xC0, // End Collection
}

// hidTouchReportLen 是一份 digitizer input report 的固定长度。
const hidTouchReportLen = 9

// hidTouchID 当前占用 report 的触点 id，随每次按下递增。
var hidTouchID uint16

// hidTouchReport 组装一份触摸 input report。
// id 为触点 tracking id；count 为本帧触点数（按下/移动 = 1，抬起 = 0）。
// inRange=false 表示抬笔（tip=0）。x/y 为 0..32767 绝对坐标，pressure 0..1023。
func hidTouchReport(id uint8, count int, inRange bool, x, y uint16, pressure uint16) []byte {
	rep := make([]byte, hidTouchReportLen)
	rep[0] = byte(count) // Contact Count：Android MT 解码器按它确定每帧触点记录数
	rep[1] = id          // Contact Identifier：跨报文跟踪同一触点
	flags := uint8(0)    // Tip Switch(bit0) | In Range(bit1)
	if inRange {
		flags |= 0x01 | 0x02
	}
	rep[2] = flags
	binary.LittleEndian.PutUint16(rep[3:5], x)
	binary.LittleEndian.PutUint16(rep[5:7], y)
	binary.LittleEndian.PutUint16(rep[7:9], pressure)
	return rep
}

// hidEnsureDevice 确保 digitizer 已在设备端创建（每会话一次）。
// 调用方已持有 controlMu 锁。
func (e deviceActionExecutor) hidEnsureDevice() error {
	if e.ms.hidTouchReady {
		return nil
	}
	if err := e.ms.sess.conn.WriteControl(scrcpy.UhidCreate(scrcpy.HIDIDTouch, 0x18D1, 0xFFFF, "scrcpy-touch", hidTouchReportDesc)); err != nil {
		return err
	}
	e.ms.hidTouchReady = true
	return nil
}

// hidTap 通过 UHID 注入一次拟人化点击：
// 落点高斯偏移 + 随机压力 + 随机按压时长，与 humanizeTap 同参数节奏。
// 调用方已持有 controlMu 锁。
func (e deviceActionExecutor) hidTap(x, y int32, w, h uint16) error {
	if err := e.hidEnsureDevice(); err != nil {
		return err
	}
	std := humanizeStdTapPx
	if w < 600 {
		std = humanizeStdTapPx / 2
	}
	fx := clampInt32(int32(math.Round(float64(x)+rand.NormFloat64()*std)), 0, int32(w)-1)
	fy := clampInt32(int32(math.Round(float64(y)+rand.NormFloat64()*std)), 0, int32(h)-1)
	pressure := humanizeMinTapPressure + rand.Float64()*(humanizeMaxTapPressure-humanizeMinTapPressure)

	hidTouchID++
	id := uint8(hidTouchID)
	down := hidTouchReport(id, 1, true, hidScaleX(fx, w), hidScaleY(fy, h), hidScalePressure(pressure))
	if err := e.ms.sess.conn.WriteControl(scrcpy.UhidInput(scrcpy.HIDIDTouch, down)); err != nil {
		return err
	}
	hold := time.Duration(humanizeMinTapHoldMS+rand.Intn(humanizeMaxTapHoldMS-humanizeMinTapHoldMS)) * time.Millisecond
	time.Sleep(hold)
	// 抬笔帧：count=0、tip=0，其余字段不再有意义，全零上报。
	up := hidTouchReport(id, 0, false, 0, 0, 0)
	return e.ms.sess.conn.WriteControl(scrcpy.UhidInput(scrcpy.HIDIDTouch, up))
}

// hidSwipe 通过 UHID 注入拟人化拖动：down → 曲线 move 序列 → up，
// 路径节奏与 humanizeSwipe 一致。
// 调用方已持有 controlMu 锁。
func (e deviceActionExecutor) hidSwipe(x1, y1, x2, y2 int32, w, h uint16) error {
	if err := e.hidEnsureDevice(); err != nil {
		return err
	}
	pressure := humanizeMinTapPressure + rand.Float64()*(humanizeMaxTapPressure-humanizeMinTapPressure)
	hidTouchID++
	id := uint8(hidTouchID)

	down := hidTouchReport(id, 1, true, hidScaleX(x1, w), hidScaleY(y1, h), hidScalePressure(pressure))
	if err := e.ms.sess.conn.WriteControl(scrcpy.UhidInput(scrcpy.HIDIDTouch, down)); err != nil {
		return err
	}
	for _, p := range planHumanizedPath(x1, y1, x2, y2, w, h) {
		time.Sleep(p.delay)
		move := hidTouchReport(id, 1, true, hidScaleX(p.x, w), hidScaleY(p.y, h), hidScalePressure(pressure))
		if err := e.ms.sess.conn.WriteControl(scrcpy.UhidInput(scrcpy.HIDIDTouch, move)); err != nil {
			return err
		}
	}
	time.Sleep(time.Duration(humanizeMinMoveDelayMS) * time.Millisecond)
	up := hidTouchReport(id, 0, false, 0, 0, 0)
	return e.ms.sess.conn.WriteControl(scrcpy.UhidInput(scrcpy.HIDIDTouch, up))
}

// hidScaleX 把设备像素 X 坐标缩放到 digitizer 逻辑范围 0..32767。
func hidScaleX(x int32, w uint16) uint16 {
	if w <= 1 {
		return 0
	}
	scaled := int64(x) * 32767 / int64(w-1)
	if scaled < 0 {
		return 0
	}
	if scaled > 32767 {
		return 32767
	}
	return uint16(scaled)
}

// hidScaleY 把设备像素 Y 坐标缩放到 digitizer 逻辑范围 0..32767。
func hidScaleY(y int32, h uint16) uint16 {
	if h <= 1 {
		return 0
	}
	scaled := int64(y) * 32767 / int64(h-1)
	if scaled < 0 {
		return 0
	}
	if scaled > 32767 {
		return 32767
	}
	return uint16(scaled)
}

// hidScalePressure 把 0..1 浮点压力缩放到 digitizer 逻辑范围 0..1023。
func hidScalePressure(p float64) uint16 {
	if p <= 0 {
		return 0
	}
	if p >= 1 {
		return 1023
	}
	return uint16(math.Round(p * 1023))
}
