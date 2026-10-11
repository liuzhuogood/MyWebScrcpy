import { onUnmounted, ref } from 'vue'

export interface VisionDetectionObject {
  label?: string
  confidence?: number
  x: number
  y: number
  w: number
  h: number
}

export interface VisionResultMessage {
  type?: string
  device_id?: string
  session_id?: string
  frame_id?: number
  timestamp?: number
  expires_ms?: number
  objects?: VisionDetectionObject[]
}

export function useTemplateMatches(serial: string, getCanvas: () => HTMLCanvasElement | null) {
  const overlayRef = ref<HTMLCanvasElement | null>(null)
  const showBoxes = ref(localStorage.getItem('show-template-boxes') !== 'false')
  const matches = ref<VisionDetectionObject[]>([])

  let ws: WebSocket | null = null
  let disposed = false
  let reconnectTimer: number | undefined
  let clearTimer: number | undefined

  function syncOverlaySize(): { width: number, height: number, ctx: CanvasRenderingContext2D } | null {
    const canvas = getCanvas()
    const overlay = overlayRef.value
    if (!canvas || !overlay || !canvas.width || !canvas.height)
      return null
    if (overlay.width !== canvas.width || overlay.height !== canvas.height) {
      overlay.width = canvas.width
      overlay.height = canvas.height
    }
    const ctx = overlay.getContext('2d')
    if (!ctx)
      return null
    return { width: canvas.width, height: canvas.height, ctx }
  }

  function clear() {
    if (clearTimer !== undefined) {
      window.clearTimeout(clearTimer)
      clearTimer = undefined
    }
    matches.value = []
    const overlay = overlayRef.value
    if (overlay) {
      overlay.getContext('2d')?.clearRect(0, 0, overlay.width, overlay.height)
    }
  }

  function drawDetections(msg: VisionResultMessage) {
    if (!showBoxes.value) {
      clear()
      return
    }

    const state = syncOverlaySize()
    if (!state)
      return
    const { width, height, ctx } = state

    ctx.clearRect(0, 0, width, height)
    if (clearTimer !== undefined) {
      window.clearTimeout(clearTimer)
      clearTimer = undefined
    }

    const objects = msg.objects || []
    matches.value = objects
    if (!objects.length)
      return

    const strokeWidth = Math.max(2, Math.round(width / 400))
    const fontSize = Math.max(12, Math.round(width / 45))
    ctx.lineWidth = strokeWidth
    ctx.font = `600 ${fontSize}px sans-serif`
    ctx.textBaseline = 'top'

    for (const obj of objects) {
      const rx = Math.max(0, Math.min(width, obj.x * width))
      const ry = Math.max(0, Math.min(height, obj.y * height))
      const rw = Math.max(1, Math.min(width - rx, obj.w * width))
      const rh = Math.max(1, Math.min(height - ry, obj.h * height))

      // 亮绿外框
      ctx.strokeStyle = '#00e676'
      ctx.strokeRect(rx, ry, rw, rh)

      // 标签文本与徽标背景
      const pct = Math.round((obj.confidence ?? 0) * 100)
      const text = `${obj.label || '目标'} ${pct}%`
      const textMetrics = ctx.measureText(text)
      const textPadX = 4
      const textPadY = 2
      const textBgW = textMetrics.width + textPadX * 2
      const textBgH = fontSize + textPadY * 2

      let labelX = rx
      let labelY = ry - textBgH
      if (labelY < 0) {
        labelY = ry
      }
      if (labelX + textBgW > width) {
        labelX = Math.max(0, width - textBgW)
      }

      ctx.fillStyle = 'rgba(0, 0, 0, 0.75)'
      ctx.fillRect(labelX, labelY, textBgW, textBgH)

      ctx.fillStyle = '#00e676'
      ctx.fillText(text, labelX + textPadX, labelY + textPadY)
    }

    if (msg.expires_ms && msg.expires_ms > 0) {
      const createdAt = Number(msg.timestamp) || Date.now()
      const remaining = Math.max(100, createdAt + Number(msg.expires_ms) - Date.now())
      clearTimer = window.setTimeout(clear, remaining)
    }
  }

  function connect() {
    if (disposed || !serial)
      return
    try {
      const proto = window.location.protocol === 'https:' ? 'wss:' : 'ws:'
      const url = `${proto}//${window.location.host}/api/vision/results?serial=${encodeURIComponent(serial)}`
      ws = new WebSocket(url)

      ws.onmessage = (event) => {
        try {
          const msg = JSON.parse(event.data) as VisionResultMessage
          if (msg && msg.type === 'detection.result') {
            drawDetections(msg)
          }
        }
        catch {}
      }

      ws.onclose = () => {
        ws = null
        if (!disposed) {
          reconnectTimer = window.setTimeout(connect, 2000)
        }
      }

      ws.onerror = () => {
        // 关闭交由 onclose 兜底重连
      }
    }
    catch {
      if (!disposed) {
        reconnectTimer = window.setTimeout(connect, 3000)
      }
    }
  }

  function setShow(on: boolean) {
    showBoxes.value = on
    localStorage.setItem('show-template-boxes', String(on))
    if (!on) {
      clear()
    }
  }

  function start() {
    disposed = false
    clear()
    connect()
  }

  function stop() {
    disposed = true
    if (reconnectTimer !== undefined) {
      window.clearTimeout(reconnectTimer)
      reconnectTimer = undefined
    }
    if (clearTimer !== undefined) {
      window.clearTimeout(clearTimer)
      clearTimer = undefined
    }
    if (ws) {
      try {
        ws.close()
      }
      catch {}
      ws = null
    }
    clear()
  }

  onUnmounted(() => {
    stop()
  })

  return {
    overlayRef,
    showBoxes,
    matches,
    setShow,
    start,
    stop,
    refresh: () => {},
  }
}
