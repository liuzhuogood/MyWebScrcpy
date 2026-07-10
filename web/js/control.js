// control.js — 浏览器端 scrcpy 控制消息打包
//
// 打包格式与 Go 端 scrcpy/control.go 完全一致（大端序）。
// 打包后的 Uint8Array 直接通过 WebSocket binary 发送，Go 端透传给 control socket。
//
// 控制消息类型:
//   TYPE_INJECT_TOUCH_EVENT = 2  (32 字节)
//   TYPE_INJECT_SCROLL_EVENT = 3 (21 字节)
//   TYPE_INJECT_KEYCODE = 0      (14 字节)
//   TYPE_INJECT_TEXT = 1         (变长)
//   TYPE_BACK_OR_SCREEN_ON = 4   (2 字节)

const TYPE_INJECT_KEYCODE = 0;
const TYPE_INJECT_TEXT = 1;
const TYPE_INJECT_TOUCH_EVENT = 2;
const TYPE_INJECT_SCROLL_EVENT = 3;
const TYPE_BACK_OR_SCREEN_ON = 4;
const TYPE_ROTATE_DEVICE = 11;

const ACTION_DOWN = 0;
const ACTION_UP = 1;
const ACTION_MOVE = 2;
const KEY_DOWN = 0;
const KEY_UP = 1;

// 指针 id（鼠标用 0xFFFFFFFFFFFFFFFF）
const POINTER_MOUSE = 0xFFFFFFFFFFFFFFFFn;
const POINTER_FINGER_BASE = 0n; // 多指时用 0,1,2... 作为 pointerId

// 压力归一化 → uint16
function pressureU16(p) {
  if (p <= 0) return 0;
  if (p >= 1.0) return 0xFFFF;
  return Math.round(p * 0xFFFF);
}

// 滚动量 → int16 定点
function scrollI16(v) {
  let norm = v / 16;
  if (norm > 1) norm = 1;
  if (norm < -1) norm = -1;
  if (norm >= 1.0) return 0x7FFF;
  if (norm <= -1.0) return 0x8000;
  return (norm * 32768) | 0; // 截断为 int16
}

class ControlPacker {
  constructor() {
    this.deviceWidth = 0;
    this.deviceHeight = 0;
  }

  setDeviceSize(w, h) {
    this.deviceWidth = w;
    this.deviceHeight = h;
  }

  // 触摸事件: 32 字节
  // action: ACTION_DOWN/MOVE/UP
  // pointerId: bigint
  // x, y: 设备像素坐标
  // pressure: 0~1
  // actionButton, buttons: 鼠标按钮位掩码
  touch(action, pointerId, x, y, pressure, actionButton = 0, buttons = 0) {
    const buf = new ArrayBuffer(32);
    const view = new DataView(buf);
    view.setUint8(0, TYPE_INJECT_TOUCH_EVENT);
    view.setUint8(1, action);
    view.setBigUint64(2, pointerId, false);       // u64
    view.setInt32(10, x, false);                   // i32
    view.setInt32(14, y, false);                   // i32
    view.setUint16(18, this.deviceWidth, false);   // u16
    view.setUint16(20, this.deviceHeight, false);  // u16
    view.setUint16(22, pressureU16(pressure), false);
    view.setUint32(24, actionButton, false);
    view.setUint32(28, buttons, false);
    return new Uint8Array(buf);
  }

  // 滚动事件: 21 字节
  scroll(x, y, hscroll, vscroll, buttons = 0) {
    const buf = new ArrayBuffer(21);
    const view = new DataView(buf);
    view.setUint8(0, TYPE_INJECT_SCROLL_EVENT);
    view.setInt32(1, x, false);
    view.setInt32(5, y, false);
    view.setUint16(9, this.deviceWidth, false);
    view.setUint16(11, this.deviceHeight, false);
    view.setInt16(13, scrollI16(hscroll), false);
    view.setInt16(15, scrollI16(vscroll), false);
    view.setUint32(17, buttons, false);
    return new Uint8Array(buf);
  }

  // 按键事件: 14 字节
  // action: KEY_DOWN/KEY_UP
  // keycode: Android AKEYCODE_*
  // metastate: 修饰键
  keyCode(action, keycode, metastate = 0, repeat = 0) {
    const buf = new ArrayBuffer(14);
    const view = new DataView(buf);
    view.setUint8(0, TYPE_INJECT_KEYCODE);
    view.setUint8(1, action);
    view.setUint32(2, keycode, false);
    view.setUint32(6, repeat, false);
    view.setUint32(10, metastate, false);
    return new Uint8Array(buf);
  }

  // 返回键 / 点亮屏幕: 2 字节
  back(action = KEY_UP) {
    return new Uint8Array([TYPE_BACK_OR_SCREEN_ON, action]);
  }

  // 旋转设备: 1 字节 (scrcpy 协议会自动切换到下一个方向)
  rotate() {
    return new Uint8Array([TYPE_ROTATE_DEVICE]);
  }

  // 文本注入: 变长
  text(str) {
    const utf8 = new TextEncoder().encode(str);
    const buf = new ArrayBuffer(5 + utf8.length);
    const view = new DataView(buf);
    view.setUint8(0, TYPE_INJECT_TEXT);
    view.setUint32(1, utf8.length, false);
    new Uint8Array(buf).set(utf8, 5);
    return new Uint8Array(buf);
  }
}

// ===== Android keycode 映射 (常用键) =====
// 完整列表见 frameworks/base/core/java/android/view/KeyEvent.java
const AKEYCODE = {
  BACK: 4,
  HOME: 3,
  MENU: 82,
  ENTER: 66,
  DEL: 67,           // 退格
  TAB: 61,
  ESCAPE: 111,
  SPACE: 62,
  DPAD_UP: 19,
  DPAD_DOWN: 20,
  DPAD_LEFT: 21,
  DPAD_RIGHT: 22,
  DPAD_CENTER: 23,
  VOLUME_UP: 24,
  VOLUME_DOWN: 25,
  POWER: 26,
  APP_SWITCH: 187,
};

// 键盘修饰键状态位
const AMETA = {
  SHIFT_ON: 0x01,
  CTRL_ON: 0x1000,
  ALT_ON: 0x02,
  META_ON: 0x10000,
};

// 把 KeyboardEvent 转成 Android keycode + metastate
function keyboardEventToKeyCode(e) {
  const meta = (e.shiftKey ? AMETA.SHIFT_ON : 0) |
               (e.ctrlKey ? AMETA.CTRL_ON : 0) |
               (e.altKey ? AMETA.ALT_ON : 0) |
               (e.metaKey ? AMETA.META_ON : 0);

  const map = {
    'Backspace': AKEYCODE.DEL,
    'Enter': AKEYCODE.ENTER,
    'Tab': AKEYCODE.TAB,
    'Escape': AKEYCODE.ESCAPE,
    ' ': AKEYCODE.SPACE,
    'ArrowUp': AKEYCODE.DPAD_UP,
    'ArrowDown': AKEYCODE.DPAD_DOWN,
    'ArrowLeft': AKEYCODE.DPAD_LEFT,
    'ArrowRight': AKEYCODE.DPAD_RIGHT,
  };
  if (map[e.key] !== undefined) {
    return { keycode: map[e.key], meta };
  }
  return null; // 可打印字符走 text() 注入，不走 keycode
}
