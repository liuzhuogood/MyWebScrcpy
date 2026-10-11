// scrcpy 控制消息打包：与后端 internal/scrcpy/control.go 完全一致（大端序）。
// 打包后的 Uint8Array 直接经 WebSocket binary 发送，后端透传给 control socket。
//
//   TYPE_INJECT_KEYCODE      = 0  (14 字节)
//   TYPE_INJECT_TEXT         = 1  (变长)
//   TYPE_INJECT_TOUCH_EVENT  = 2  (32 字节)
//   TYPE_INJECT_SCROLL_EVENT = 3  (21 字节)
//   TYPE_BACK_OR_SCREEN_ON   = 4  (2 字节)
//   TYPE_GET_CLIPBOARD       = 8  (2 字节)
//   TYPE_SET_CLIPBOARD       = 9  (14+N 字节)
//   TYPE_ROTATE_DEVICE       = 11 (1 字节)

export const TYPE_INJECT_KEYCODE = 0
export const TYPE_INJECT_TEXT = 1
export const TYPE_INJECT_TOUCH_EVENT = 2
export const TYPE_INJECT_SCROLL_EVENT = 3
export const TYPE_BACK_OR_SCREEN_ON = 4
export const TYPE_GET_CLIPBOARD = 8
export const TYPE_SET_CLIPBOARD = 9
export const TYPE_ROTATE_DEVICE = 11

export const COPY_KEY_NONE = 0
export const COPY_KEY_COPY = 1
export const COPY_KEY_CUT = 2

export const CLIPBOARD_TEXT_MAX_LENGTH = 262139

export const ACTION_DOWN = 0
export const ACTION_UP = 1
export const ACTION_MOVE = 2
export const KEY_DOWN = 0
export const KEY_UP = 1

/** 鼠标指针 id（多指触摸用 0,1,2…） */
export const POINTER_MOUSE = 0xFFFFFFFFFFFFFFFFn
export const POINTER_FINGER_BASE = 0n

function truncateUtf8(bytes: Uint8Array, maxBytes: number): Uint8Array {
  if (bytes.length <= maxBytes)
    return bytes
  let end = maxBytes
  while (end > 0 && (bytes[end] & 0xC0) === 0x80)
    end--
  return bytes.subarray(0, end)
}

function pressureU16(p: number): number {
  if (p <= 0)
    return 0
  if (p >= 1.0)
    return 0xFFFF
  return Math.round(p * 0xFFFF)
}

function scrollI16(v: number): number {
  let norm = v / 16
  if (norm > 1)
    norm = 1
  if (norm < -1)
    norm = -1
  if (norm >= 1.0)
    return 0x7FFF
  if (norm <= -1.0)
    return 0x8000
  return (norm * 32768) | 0
}

export class ControlPacker {
  deviceWidth = 0
  deviceHeight = 0

  setDeviceSize(w: number, h: number) {
    this.deviceWidth = w
    this.deviceHeight = h
  }

  /** 触摸事件 32 字节；x/y 为设备像素坐标 */
  touch(action: number, pointerId: bigint, x: number, y: number, pressure: number, actionButton = 0, buttons = 0): Uint8Array {
    const buf = new ArrayBuffer(32)
    const view = new DataView(buf)
    view.setUint8(0, TYPE_INJECT_TOUCH_EVENT)
    view.setUint8(1, action)
    view.setBigUint64(2, pointerId, false)
    view.setInt32(10, x, false)
    view.setInt32(14, y, false)
    view.setUint16(18, this.deviceWidth, false)
    view.setUint16(20, this.deviceHeight, false)
    view.setUint16(22, pressureU16(pressure), false)
    view.setUint32(24, actionButton, false)
    view.setUint32(28, buttons, false)
    return new Uint8Array(buf)
  }

  /** 滚动事件 21 字节 */
  scroll(x: number, y: number, hscroll: number, vscroll: number, buttons = 0): Uint8Array {
    const buf = new ArrayBuffer(21)
    const view = new DataView(buf)
    view.setUint8(0, TYPE_INJECT_SCROLL_EVENT)
    view.setInt32(1, x, false)
    view.setInt32(5, y, false)
    view.setUint16(9, this.deviceWidth, false)
    view.setUint16(11, this.deviceHeight, false)
    view.setInt16(13, scrollI16(hscroll), false)
    view.setInt16(15, scrollI16(vscroll), false)
    view.setUint32(17, buttons, false)
    return new Uint8Array(buf)
  }

  /** 按键事件 14 字节 */
  keyCode(action: number, keycode: number, metastate = 0, repeat = 0): Uint8Array {
    const buf = new ArrayBuffer(14)
    const view = new DataView(buf)
    view.setUint8(0, TYPE_INJECT_KEYCODE)
    view.setUint8(1, action)
    view.setUint32(2, keycode, false)
    view.setUint32(6, repeat, false)
    view.setUint32(10, metastate, false)
    return new Uint8Array(buf)
  }

  /** 返回键 / 点亮屏幕 2 字节 */
  back(action = KEY_UP): Uint8Array {
    return new Uint8Array([TYPE_BACK_OR_SCREEN_ON, action])
  }

  /** 旋转设备 1 字节（协议自动切到下一个方向） */
  rotate(): Uint8Array {
    return new Uint8Array([TYPE_ROTATE_DEVICE])
  }

  /** 文本注入变长包 */
  text(str: string): Uint8Array {
    const utf8 = new TextEncoder().encode(str)
    const buf = new ArrayBuffer(5 + utf8.length)
    const view = new DataView(buf)
    view.setUint8(0, TYPE_INJECT_TEXT)
    view.setUint32(1, utf8.length, false)
    new Uint8Array(buf).set(utf8, 5)
    return new Uint8Array(buf)
  }

  /** 读取设备剪贴板 2 字节；copyKey 可选触发复制/剪切按键 */
  getClipboard(copyKey: number = COPY_KEY_NONE): Uint8Array {
    return new Uint8Array([TYPE_GET_CLIPBOARD, copyKey & 0xFF])
  }

  /** 设置设备剪贴板 14+N 字节；paste=true 时设备端同步触发粘贴 */
  setClipboard(str: string, paste: boolean, sequence = 0n): Uint8Array {
    const utf8 = truncateUtf8(new TextEncoder().encode(str), CLIPBOARD_TEXT_MAX_LENGTH)
    const buf = new ArrayBuffer(14 + utf8.length)
    const view = new DataView(buf)
    view.setUint8(0, TYPE_SET_CLIPBOARD)
    view.setBigUint64(1, sequence, false)
    view.setUint8(9, paste ? 1 : 0)
    view.setUint32(10, utf8.length, false)
    new Uint8Array(buf, 14).set(utf8)
    return new Uint8Array(buf)
  }
}

/** 常用 Android keycode（完整列表见 AOSP KeyEvent.java） */
export const AKEYCODE = {
  BACK: 4,
  HOME: 3,
  MENU: 82,
  ENTER: 66,
  DEL: 67,
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
} as const

export const AMETA = {
  SHIFT_ON: 0x01,
  CTRL_ON: 0x1000,
  ALT_ON: 0x02,
  META_ON: 0x10000,
} as const

export interface KeyMapping {
  keycode: number
  meta: number
}

/** KeyboardEvent → Android keycode；可打印字符返回 null，走 text() 注入 */
export function keyboardEventToKeyCode(e: KeyboardEvent): KeyMapping | null {
  const meta
    = (e.shiftKey ? AMETA.SHIFT_ON : 0)
      | (e.ctrlKey ? AMETA.CTRL_ON : 0)
      | (e.altKey ? AMETA.ALT_ON : 0)
      | (e.metaKey ? AMETA.META_ON : 0)

  const map: Record<string, number> = {
    Backspace: AKEYCODE.DEL,
    Enter: AKEYCODE.ENTER,
    Tab: AKEYCODE.TAB,
    Escape: AKEYCODE.ESCAPE,
    ' ': AKEYCODE.SPACE,
    ArrowUp: AKEYCODE.DPAD_UP,
    ArrowDown: AKEYCODE.DPAD_DOWN,
    ArrowLeft: AKEYCODE.DPAD_LEFT,
    ArrowRight: AKEYCODE.DPAD_RIGHT,
  }
  if (map[e.key] !== undefined)
    return { keycode: map[e.key], meta }
  return null
}
