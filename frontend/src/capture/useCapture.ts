import { computed, ref } from 'vue'
import { captureActive } from './state'

export interface CaptureRect {
  x: number
  y: number
  w: number
  h: number
}

export type CaptureShape = 'square' | 'circle' | 'freehand'
/** inside：截取线条内的图片；line：截图线条区域（空心） */
export type CaptureMode = 'inside' | 'line'

const MIN_SIZE = 8
/** 自由绘制闭合判定：起点与终点距离小于该值（画布像素）即视为闭合 */
const CLOSE_THRESHOLD = 18

/** 选区截图状态机：rect 为显示坐标（相对选区层），bounds 换算为画布像素 */
export function useCapture(getCanvas: () => HTMLCanvasElement | null, getLayer: () => HTMLElement | null) {
  const rect = ref<CaptureRect | null>(null)
  const shape = ref<CaptureShape>('square')
  const mode = ref<CaptureMode>('inside')
  const strokeWidth = ref(2)
  /** 自由绘制路径点（画布像素坐标），闭合后末尾追加首点 */
  const path = ref<Array<{ x: number, y: number }> | null>(null)
  const pathClosed = ref(false)
  /** 自由绘制撤销栈：记录每次落笔结束时的路径快照 */
  const history = ref<Array<{ path: Array<{ x: number, y: number }> | null, closed: boolean }>>([])
  const canUndo = computed(() => history.value.length > 0)

  const active = computed(() => captureActive.value)

  function scale(): { sx: number, sy: number } {
    const canvas = getCanvas()
    if (!canvas?.width || !canvas.height || !canvas.clientWidth || !canvas.clientHeight)
      return { sx: 1, sy: 1 }
    return { sx: canvas.width / canvas.clientWidth, sy: canvas.height / canvas.clientHeight }
  }

  function layerToCanvas(p: { x: number, y: number }): { x: number, y: number } {
    const { sx, sy } = scale()
    return { x: p.x * sx, y: p.y * sy }
  }

  function open() {
    const canvas = getCanvas()
    if (!canvas?.width || !canvas.height || !canvas.clientWidth || !canvas.clientHeight)
      return '画面尚未准备好，暂时无法截图。'
    const w = Math.max(MIN_SIZE, Math.round(canvas.clientWidth * 0.7))
    const h = Math.max(MIN_SIZE, Math.round(canvas.clientHeight * 0.7))
    rect.value = {
      x: Math.max(0, Math.round((canvas.clientWidth - w) / 2)),
      y: Math.max(0, Math.round((canvas.clientHeight - h) / 2)),
      w,
      h,
    }
    shape.value = 'square'
    mode.value = 'inside'
    path.value = null
    pathClosed.value = false
    history.value = []
    captureActive.value = true
    return null
  }

  function close() {
    captureActive.value = false
    rect.value = null
    path.value = null
    pathClosed.value = false
    history.value = []
  }

  function clearFreehand() {
    path.value = null
    pathClosed.value = false
  }

  function setShape(s: CaptureShape) {
    if (s === shape.value)
      return
    history.value = []
    if (s === 'freehand') {
      shape.value = s
      rect.value = null
      clearFreehand()
      return
    }
    shape.value = s
    clearFreehand()
    if (!rect.value) {
      const canvas = getCanvas()
      if (canvas?.clientWidth && canvas.clientHeight) {
        const w = Math.max(MIN_SIZE, Math.round(canvas.clientWidth * 0.5))
        const h = Math.max(MIN_SIZE, Math.round(canvas.clientHeight * 0.5))
        rect.value = {
          x: Math.max(0, Math.round((canvas.clientWidth - w) / 2)),
          y: Math.max(0, Math.round((canvas.clientHeight - h) / 2)),
          w,
          h,
        }
      }
    }
  }

  /** 显示坐标 → 画布像素 bounds */
  function bounds(): { x: number, y: number, width: number, height: number } | null {
    const canvas = getCanvas()
    if (!canvas)
      return null
    if (shape.value === 'freehand') {
      const pts = path.value
      if (!pts || pts.length < 3)
        return null
      const xs = pts.map(p => p.x)
      const ys = pts.map(p => p.y)
      const x = Math.floor(Math.min(...xs))
      const y = Math.floor(Math.min(...ys))
      return {
        x,
        y,
        width: Math.max(1, Math.ceil(Math.max(...xs) - x)),
        height: Math.max(1, Math.ceil(Math.max(...ys) - y)),
      }
    }
    const r = rect.value
    if (!r)
      return null
    const { sx, sy } = scale()
    return {
      x: Math.round(r.x * sx),
      y: Math.round(r.y * sy),
      width: Math.round(r.w * sx),
      height: Math.round(r.h * sy),
    }
  }

  function circleCoords(): { center_x: number, center_y: number, radius: number } | null {
    const b = bounds()
    if (!b)
      return null
    return {
      center_x: Math.round(b.x + b.width / 2),
      center_y: Math.round(b.y + b.height / 2),
      radius: Math.round(Math.min(b.width, b.height) / 2),
    }
  }

  const sizeText = computed(() => {
    if (shape.value === 'circle') {
      const c = circleCoords()
      return c ? `圆心 ${c.center_x}, ${c.center_y} · 半径 ${c.radius}` : ''
    }
    if (shape.value === 'freehand') {
      const b = bounds()
      if (pathClosed.value)
        return b ? `已闭合 · ${b.width} × ${b.height}` : ''
      return path.value && path.value.length ? `绘制中 · ${path.value.length} 点` : '自由绘制，闭合即可'
    }
    const b = bounds()
    return b ? `坐标 ${b.x}, ${b.y} · ${b.width} × ${b.height}` : ''
  })

  function move(dx: number, dy: number) {
    const r = rect.value
    const layer = getLayer()
    if (!r || !layer)
      return
    rect.value = {
      ...r,
      x: Math.max(0, Math.min(layer.clientWidth - r.w, r.x + dx)),
      y: Math.max(0, Math.min(layer.clientHeight - r.h, r.y + dy)),
    }
  }

  function resize(dw: number, dh: number, anchor: 'se' | 'center' = 'se') {
    const r = rect.value
    const layer = getLayer()
    if (!r || !layer)
      return
    if (anchor === 'center' || shape.value === 'circle') {
      // 按绝对值取主导方向的缩放量（含负值），保证方向键可缩小
      const delta = Math.abs(dw) >= Math.abs(dh) ? dw : dh
      const size = Math.max(MIN_SIZE, Math.min(Math.min(layer.clientWidth, layer.clientHeight), r.w + delta))
      const cx = r.x + r.w / 2
      const cy = r.y + r.h / 2
      rect.value = {
        w: size,
        h: size,
        x: Math.max(0, Math.min(layer.clientWidth - size, cx - size / 2)),
        y: Math.max(0, Math.min(layer.clientHeight - size, cy - size / 2)),
      }
      return
    }
    rect.value = {
      ...r,
      w: Math.max(MIN_SIZE, Math.min(layer.clientWidth - r.x, r.w + dw)),
      h: Math.max(MIN_SIZE, Math.min(layer.clientHeight - r.y, r.h + dh)),
    }
  }

  /** 键盘微调：方向键移动，Shift+方向键缩放（步长为 1 画布像素） */
  function nudge(e: KeyboardEvent): boolean {
    if (!['ArrowLeft', 'ArrowRight', 'ArrowUp', 'ArrowDown'].includes(e.key))
      return false
    // 自由绘制无选区可调：吞掉方向键，避免冒泡到投屏变成设备按键
    if (shape.value === 'freehand')
      return true
    const canvas = getCanvas()
    if (!rect.value || !canvas?.clientWidth)
      return false
    const stepX = canvas.width / Math.max(1, canvas.clientWidth)
    const stepY = canvas.height / Math.max(1, canvas.clientHeight)
    if (e.shiftKey) {
      if (e.key === 'ArrowLeft')
        resize(-stepX, 0)
      else if (e.key === 'ArrowRight')
        resize(stepX, 0)
      else if (e.key === 'ArrowUp')
        resize(0, -stepY)
      else resize(0, stepY)
    }
    else {
      if (e.key === 'ArrowLeft')
        move(-stepX, 0)
      else if (e.key === 'ArrowRight')
        move(stepX, 0)
      else if (e.key === 'ArrowUp')
        move(0, -stepY)
      else move(0, stepY)
    }
    return true
  }

  /** 应用 BBOX JSON（矩形 {x,y,width,height} 或圆形 {shape:'circle',center_x,center_y,radius,stroke_width?}） */
  function applyBbox(json: string): string | null {
    const canvas = getCanvas()
    if (!canvas?.width || !canvas.height)
      return '画面尚未准备就绪'
    if (shape.value === 'freehand')
      return '自由绘制选区不支持 BBOX，请用矩形或圆形。'
    let bbox: Record<string, unknown>
    try {
      bbox = JSON.parse(json)
    }
    catch {
      return 'BBOX 格式无效，请输入 JSON。'
    }
    const num = (v: unknown) => typeof v === 'number' && Number.isFinite(v)
    if (bbox.shape === 'circle') {
      if (!num(bbox.center_x) || !num(bbox.center_y) || !num(bbox.radius) || (bbox.radius as number) <= 0)
        return '圆形必须包含有效的 center_x、center_y、radius。'
      shape.value = 'circle'
      if (num(bbox.stroke_width))
        strokeWidth.value = Math.min(10, Math.max(1, Math.round(bbox.stroke_width as number)))
      const diameter = Math.min(canvas.width, canvas.height, Math.max(1, Math.round((bbox.radius as number) * 2)))
      const x = Math.min(canvas.width - diameter, Math.max(0, Math.round((bbox.center_x as number) - (bbox.radius as number))))
      const y = Math.min(canvas.height - diameter, Math.max(0, Math.round((bbox.center_y as number) - (bbox.radius as number))))
      const { sx, sy } = { sx: canvas.clientWidth / canvas.width, sy: canvas.clientHeight / canvas.height }
      rect.value = { x: x * sx, y: y * sy, w: diameter * sx, h: diameter * sy }
      return null
    }
    const vals = ['x', 'y', 'width', 'height'].map(k => bbox[k])
    if (vals.some(v => !num(v)) || (bbox.x as number) < 0 || (bbox.y as number) < 0 || (bbox.width as number) <= 0 || (bbox.height as number) <= 0)
      return 'BBOX 必须包含有效的 x、y、width、height。'
    const x = Math.min(canvas.width - 1, Math.max(0, Math.round(bbox.x as number)))
    const y = Math.min(canvas.height - 1, Math.max(0, Math.round(bbox.y as number)))
    const width = Math.min(canvas.width - x, Math.max(1, Math.round(bbox.width as number)))
    const height = Math.min(canvas.height - y, Math.max(1, Math.round(bbox.height as number)))
    const { sx, sy } = { sx: canvas.clientWidth / canvas.width, sy: canvas.clientHeight / canvas.height }
    rect.value = { x: x * sx, y: y * sy, w: width * sx, h: height * sy }
    return null
  }

  // ---------- 自由绘制 ----------
  // 撤销栈存“改前”快照：落笔/闭合前先 push，undo 恢复栈顶即回到上一步
  function freehandStart(p: { x: number, y: number }) {
    const cp = layerToCanvas(p)
    if (path.value && !pathClosed.value && path.value.length) {
      // 上一段未闭合：继续描线
      path.value = [...path.value, cp]
    }
    else {
      commitHistory()
      path.value = [cp]
      pathClosed.value = false
    }
  }

  function freehandAdd(p: { x: number, y: number }) {
    const pts = path.value
    if (!pts || pathClosed.value)
      return
    const cp = layerToCanvas(p)
    const last = pts[pts.length - 1]
    if (Math.hypot(cp.x - last.x, cp.y - last.y) < 1.5)
      return
    path.value = [...pts, cp]
  }

  function freehandEnd(): 'closed' | 'open' | 'short' {
    const pts = path.value
    if (!pts || pts.length < 3)
      return 'short'
    const first = pts[0]
    const last = pts[pts.length - 1]
    if (Math.hypot(last.x - first.x, last.y - first.y) <= CLOSE_THRESHOLD) {
      commitHistory()
      path.value = [...pts, first]
      pathClosed.value = true
      const b = bounds()
      if (b) {
        const { sx, sy } = scale()
        rect.value = { x: b.x / sx, y: b.y / sy, w: b.width / sx, h: b.height / sy }
      }
    }
    return pathClosed.value ? 'closed' : 'open'
  }

  /** 撤销上一笔自由绘制；无可撤销时返回 false */
  function undo(): boolean {
    const prev = history.value.pop()
    if (!prev)
      return false
    path.value = prev.path ? [...prev.path] : null
    pathClosed.value = prev.closed
    if (!path.value || path.value.length < 3) {
      rect.value = null
    }
    else {
      const b = bounds()
      if (b) {
        const { sx, sy } = scale()
        rect.value = { x: b.x / sx, y: b.y / sy, w: b.width / sx, h: b.height / sy }
      }
    }
    return true
  }

  function commitHistory() {
    history.value.push({ path: path.value ? [...path.value] : null, closed: pathClosed.value })
    if (history.value.length > 100)
      history.value.shift()
  }

  function tracePath(ctx: CanvasRenderingContext2D, ox: number, oy: number) {
    const pts = path.value
    if (!pts || !pts.length)
      return
    ctx.moveTo(pts[0].x + ox, pts[0].y + oy)
    for (let i = 1; i < pts.length; i++)
      ctx.lineTo(pts[i].x + ox, pts[i].y + oy)
  }

  /** 导出选区：inside 保留边界内画面；line 只保留描边那一圈，内部透明（空心） */
  function exportCanvas(): { output: HTMLCanvasElement, sw: number, sh: number } | null {
    const b = bounds()
    const canvas = getCanvas()
    if (!b || !canvas)
      return null
    const sw = strokeWidth.value
    // 空心模式需要给描边外扩半线宽，避免线被裁掉
    const margin = mode.value === 'line' ? Math.ceil(sw / 2) + 1 : 0
    const X = Math.max(0, b.x - margin)
    const Y = Math.max(0, b.y - margin)
    const W = b.width + margin * 2
    const H = b.height + margin * 2
    const output = document.createElement('canvas')
    output.width = W
    output.height = H
    const ctx = output.getContext('2d')
    if (!ctx)
      return null

    if (mode.value === 'inside') {
      ctx.save()
      if (shape.value === 'circle') {
        ctx.beginPath()
        ctx.arc((b.x + b.width / 2) - X, (b.y + b.height / 2) - Y, Math.min(b.width, b.height) / 2, 0, Math.PI * 2)
        ctx.clip()
      }
      else if (shape.value === 'freehand') {
        tracePath(ctx, -X, -Y)
        ctx.closePath()
        ctx.clip()
      }
      ctx.drawImage(canvas, X, Y, W, H, 0, 0, W, H)
      ctx.restore()
    }
    else {
      // line（空心）：先画源图，再用描边掩码保留线那一圈
      ctx.drawImage(canvas, X, Y, W, H, 0, 0, W, H)
      const mask = document.createElement('canvas')
      mask.width = W
      mask.height = H
      const mctx = mask.getContext('2d')
      if (!mctx)
        return null
      mctx.strokeStyle = '#fff'
      mctx.lineWidth = sw
      mctx.lineCap = 'round'
      mctx.lineJoin = 'round'
      mctx.beginPath()
      if (shape.value === 'circle') {
        mctx.arc((b.x + b.width / 2) - X, (b.y + b.height / 2) - Y, Math.min(b.width, b.height) / 2, 0, Math.PI * 2)
      }
      else if (shape.value === 'freehand') {
        tracePath(mctx, -X, -Y)
        mctx.closePath()
      }
      else {
        mctx.rect(margin, margin, b.width, b.height)
      }
      mctx.stroke()
      ctx.globalCompositeOperation = 'destination-in'
      ctx.drawImage(mask, 0, 0)
      ctx.globalCompositeOperation = 'source-over'
    }
    return { output, sw: W, sh: H }
  }

  function toBlob(): Promise<Blob | null> {
    const cap = exportCanvas()
    if (!cap)
      return Promise.resolve(null)
    return new Promise(resolve => cap.output.toBlob(resolve, 'image/png'))
  }

  return {
    rect, shape, mode, strokeWidth, active, sizeText, path, pathClosed, canUndo,
    open, close, move, resize, nudge, applyBbox,
    bounds, circleCoords, exportCanvas, toBlob, scale,
    clearFreehand, setShape, freehandStart, freehandAdd, freehandEnd, undo,
  }
}
