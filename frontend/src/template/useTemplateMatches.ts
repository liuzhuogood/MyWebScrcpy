import { onUnmounted, ref } from 'vue'
import { getMatches, type TemplateMatch } from '@/api/templates'

const POLL_MS = 2000
const SHOW_KEY = 'show-green-boxes'

/** 模板命中绿框：轮询 matches 并画到 overlay；开关持久化沿用老 key */
export function useTemplateMatches(serial: string, getCanvas: () => HTMLCanvasElement | null) {
  const overlayRef = ref<HTMLCanvasElement | null>(null)
  const showBoxes = ref(localStorage.getItem(SHOW_KEY) !== 'false')
  const matches = ref<TemplateMatch[]>([])
  let timer: number | undefined
  let stopped = false

  function setShow(on: boolean) {
    showBoxes.value = on
    localStorage.setItem(SHOW_KEY, String(on))
    if (!on)
      clear()
    else void poll()
  }

  function clear() {
    const overlay = overlayRef.value
    overlay?.getContext('2d')?.clearRect(0, 0, overlay.width, overlay.height)
  }

  function draw(list: TemplateMatch[]) {
    const canvas = getCanvas()
    const overlay = overlayRef.value
    if (!canvas || !overlay)
      return
    if (overlay.width !== canvas.width || overlay.height !== canvas.height) {
      overlay.width = canvas.width
      overlay.height = canvas.height
    }
    const ctx = overlay.getContext('2d')
    if (!ctx)
      return
    ctx.clearRect(0, 0, overlay.width, overlay.height)
    if (!showBoxes.value)
      return
    ctx.strokeStyle = '#00e676'
    ctx.lineWidth = Math.max(2, canvas.width / 500)
    for (const m of list) {
      // 后端返回归一化坐标；若已是像素（>1）则直接使用
      const nx = m.x <= 1 && m.w <= 1
      const x = nx ? m.x * canvas.width : m.x
      const y = nx ? m.y * canvas.height : m.y
      const w = nx ? m.w * canvas.width : m.w
      const h = nx ? m.h * canvas.height : m.h
      ctx.strokeRect(x, y, w, h)
      if (m.name || m.confidence !== undefined) {
        ctx.fillStyle = '#00e676'
        ctx.font = `${Math.max(12, canvas.width / 60)}px sans-serif`
        const label = `${m.name || ''} ${m.confidence !== undefined ? Math.round(m.confidence * 100) + '%' : ''}`.trim()
        if (label)
          ctx.fillText(label, x, Math.max(12, y - 4))
      }
    }
  }

  async function poll() {
    if (stopped)
      return
    try {
      const data = await getMatches(serial)
      matches.value = Array.isArray(data.matches) ? data.matches : []
      draw(matches.value)
    }
    catch {
      // 设备离线等情况静默跳过，下轮重试
    }
  }

  function start() {
    stop()
    stopped = false
    void poll()
    timer = window.setInterval(poll, POLL_MS)
  }

  function stop() {
    stopped = true
    if (timer !== undefined) {
      window.clearInterval(timer)
      timer = undefined
    }
    clear()
  }

  onUnmounted(stop)

  return { overlayRef, showBoxes, matches, setShow, start, stop, refresh: poll }
}
