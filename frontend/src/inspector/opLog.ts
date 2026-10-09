import { ref } from 'vue'

// 操作日志：记录投屏操控，可切换“描述 / ADB 命令”视图。

export type OpType = 'tap' | 'longpress' | 'swipe' | 'key' | 'text'

export interface OpEntry {
  time: string
  desc: string
  adb: string
}

export type OpPayload
  = | { x: number, y: number }
    | { x: number, y: number, duration: number }
    | { x1: number, y1: number, x2: number, y2: number, duration: number, path?: { x: number, y: number }[] }
    | { label: string, keycode: string }
    | { text: string }

const MAX_ENTRIES = 500

export const opEntries = ref<OpEntry[]>([])
export const opMode = ref<'desc' | 'adb'>('desc')

function adbPrefix(serial: string): string {
  return serial ? `adb -s '${serial}' shell input` : 'adb shell input'
}

export function recordOp(serial: string, type: OpType, data: OpPayload) {
  const time = new Date().toTimeString().slice(0, 8)
  let desc = ''
  let adb = ''
  const prefix = adbPrefix(serial)
  if (type === 'tap') {
    const d = data as { x: number, y: number }
    desc = `点击 (${d.x}, ${d.y})`
    adb = `${prefix} tap ${d.x} ${d.y}`
  }
  else if (type === 'longpress') {
    const d = data as { x: number, y: number, duration: number }
    desc = `长按 (${d.x}, ${d.y})  ${d.duration}ms`
    adb = `${prefix} swipe ${d.x} ${d.y} ${d.x} ${d.y} ${d.duration}`
  }
  else if (type === 'swipe') {
    const d = data as { x1: number, y1: number, x2: number, y2: number, duration: number, path?: { x: number, y: number }[] }
    // 取样路径：起点 → 中间采样点 → 终点；未取样时退化为起终点直线
    const pts = d.path && d.path.length >= 2 ? d.path : [{ x: d.x1, y: d.y1 }, { x: d.x2, y: d.y2 }]
    const points = pts.length
    desc = `滑动 (${d.x1}, ${d.y1}) → (${d.x2}, ${d.y2}) · ${points} 点 ${d.duration}ms`
    if (points === 2) {
      adb = `${prefix} swipe ${d.x1} ${d.y1} ${d.x2} ${d.y2} ${d.duration}`
    }
    else {
      // 多段 swipe 复现曲线轨迹，把总时长均分到各段
      const per = Math.max(1, Math.round(d.duration / (points - 1)))
      const shellBase = serial ? `adb -s '${serial}' shell` : 'adb shell'
      const segs = pts.slice(0, -1).map((p, i) => `input swipe ${p.x} ${p.y} ${pts[i + 1].x} ${pts[i + 1].y} ${per}`)
      adb = `${shellBase} "${segs.join('; ')}"`
    }
  }
  else if (type === 'key') {
    const d = data as { label: string, keycode: string }
    desc = `按键 ${d.label}`
    adb = `${prefix} keyevent ${d.keycode}`
  }
  else if (type === 'text') {
    const d = data as { text: string }
    desc = `输入 “${d.text}”`
    adb = `${prefix} text "${d.text.replace(/"/g, '\\"')}"`
  }
  else {
    return
  }
  const next = [...opEntries.value, { time, desc, adb }]
  opEntries.value = next.length > MAX_ENTRIES ? next.slice(-MAX_ENTRIES) : next
}

export function clearOps() {
  opEntries.value = []
}
