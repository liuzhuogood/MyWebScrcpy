import { computed, onUnmounted, ref, watch } from 'vue'
import type { Ref } from 'vue'
import { detectOCR, type OCRBox, type OCRDetectResponse } from '@/api/ocr'
import type { Box } from './xmlUtils'

type Region = { x: number, y: number, w: number, h: number }
type Scope = 'screen' | 'region' | 'element'

function encodeCanvas(canvas: HTMLCanvasElement, signal: AbortSignal): Promise<Blob | null> {
  return new Promise((resolve, reject) => {
    const abort = () => reject(signal.reason)
    if (signal.aborted) { abort(); return }
    signal.addEventListener('abort', abort, { once: true })
    try {
      canvas.toBlob(blob => { signal.removeEventListener('abort', abort); resolve(blob) }, 'image/png')
    }
    catch (error) { signal.removeEventListener('abort', abort); reject(error) }
  })
}

export function useInspectorOcr(
  serial: string,
  getCanvas: () => HTMLCanvasElement | null,
  selectedNode: Ref<Element | null>,
  elementBox: () => Box | null,
  redraw: () => void,
  frameVersion: () => number = () => 0,
  isSourceReady: () => boolean = () => true,
) {
  const loading = ref(false)
  const running = ref(false)
  const continuous = ref(false)
  const engine = ref('auto')
  const scope = ref<Scope>('screen')
  const region = ref<Region | null>(null)
  const selecting = ref(false)
  const result = ref<OCRDetectResponse | null>(null)
  const error = ref('')
  const showBoxes = ref(true)
  const keyword = ref('')
  const selected = ref<number | null>(null)
  const sourceSize = ref({ width: 0, height: 0 })
  const filtered = computed(() => (result.value?.boxes ?? []).map((box, index) => ({ box, index })).filter(({ box }) => box.text.includes(keyword.value.trim())))
  let revision = 0
  let timer: ReturnType<typeof setTimeout> | undefined
  let controller: AbortController | null = null
  let anchor: { x: number, y: number } | null = null
  let capturedAt = 0
  let capturedFrame = 0
  let overlayValid = false
  let selectionSize: { width: number, height: number } | null = null

  function stop() {
    revision++
    running.value = false
    if (timer) clearTimeout(timer)
    timer = undefined
    controller?.abort()
    controller = null
    loading.value = false
  }

  function clear() {
    stop()
    result.value = null
    error.value = ''
    selected.value = null
    overlayValid = false
    redraw()
  }

  function invalidate() {
    selecting.value = false
    region.value = null
    anchor = null
    selectionSize = null
    clear()
  }

  function coordinates(box: OCRBox) {
    const { width, height } = sourceSize.value
    return `x:${Math.round(box.x * width)} y:${Math.round(box.y * height)} w:${Math.round(box.w * width)} h:${Math.round(box.h * height)} px`
  }

  function point(event: PointerEvent) {
    const canvas = getCanvas()
    if (!canvas?.width || !canvas.height) return null
    const rect = canvas.getBoundingClientRect()
    if (!rect.width || !rect.height) return null
    return { x: Math.max(0, Math.min(1, (event.clientX - rect.left) / rect.width)), y: Math.max(0, Math.min(1, (event.clientY - rect.top) / rect.height)) }
  }

  function select(index: number) {
    selected.value = index
    redraw()
  }

  function beginRegionSelection() {
    clear()
    region.value = null
    anchor = null
    selectionSize = null
    scope.value = 'region'
    selecting.value = true
    redraw()
  }

  function pointerDown(event: PointerEvent) {
    if (event.button !== 0) return
    const p = point(event)
    if (!p) return
    const hit = !selecting.value && overlayValid && showBoxes.value && filtered.value.find(({ box }) => p.x >= box.x && p.x <= box.x + box.w && p.y >= box.y && p.y <= box.y + box.h)
    if (hit) { select(hit.index); return }
    if (scope.value === 'region' && selecting.value) {
      event.preventDefault()
      clear()
      region.value = null
      anchor = p
      const canvas = getCanvas()!
      selectionSize = { width: canvas.width, height: canvas.height }
      canvas.setPointerCapture(event.pointerId)
    }
    redraw()
  }

  function pointerMove(event: PointerEvent) {
    if (!anchor) return
    const p = point(event)
    if (!p) return
    region.value = { x: Math.min(p.x, anchor.x), y: Math.min(p.y, anchor.y), w: Math.abs(p.x - anchor.x), h: Math.abs(p.y - anchor.y) }
    redraw()
  }

  function pointerUp(event: PointerEvent) {
    if (!anchor) return
    pointerMove(event)
    anchor = null
    const canvas = getCanvas()
    const area = region.value
    if (canvas && area && (area.w * canvas.width < 2 || area.h * canvas.height < 2)) region.value = null
    if (region.value) selecting.value = false
    redraw()
  }

  function pointerCancel() {
    selecting.value = false
    if (!anchor) return
    anchor = null
    region.value = null
    redraw()
  }

  async function run() {
    if (loading.value) return
    const canvas = getCanvas()
    error.value = ''
    if (!isSourceReady() || !canvas?.width || !canvas.height) { error.value = '投屏画面未就绪'; running.value = false; return }
    const width = canvas.width, height = canvas.height
    let pixels: Region = { x: 0, y: 0, w: width, h: height }
    if (scope.value === 'element') {
      const box = elementBox()
      if (!box || ![box.x, box.y, box.w, box.h].every(Number.isFinite) || box.w <= 0 || box.h <= 0) { error.value = '无法确定选中元素的画面区域'; running.value = false; return }
      pixels = box
    }
    else if (scope.value === 'region') {
      if (!region.value) { error.value = '请先在画面上框选区域'; running.value = false; return }
      const area = region.value
      pixels = { x: area.x * width, y: area.y * height, w: area.w * width, h: area.h * height }
    }
    const left = Math.max(0, Math.min(width, Math.floor(pixels.x)))
    const top = Math.max(0, Math.min(height, Math.floor(pixels.y)))
    const right = Math.max(0, Math.min(width, Math.ceil(pixels.x + pixels.w)))
    const bottom = Math.max(0, Math.min(height, Math.ceil(pixels.y + pixels.h)))
    if (right <= left || bottom <= top) { error.value = '选中元素不在当前画面内'; running.value = false; return }
    const token = ++revision
    const abort = new AbortController()
    controller = abort
    loading.value = true
    running.value = continuous.value
    const started = Date.now(), frame = frameVersion()
    const requestEngine = engine.value
    let stage: 'capture' | 'recognition' = 'capture'
    const timeout = setTimeout(() => abort.abort(new Error('OCR 识别超时')), 30000)
    try {
      let source = canvas
      if (scope.value !== 'screen') {
        source = document.createElement('canvas')
        source.width = right - left
        source.height = bottom - top
        const ctx = source.getContext('2d')
        if (!ctx) throw new Error('截取选中区域失败')
        ctx.drawImage(canvas, left, top, source.width, source.height, 0, 0, source.width, source.height)
      }
      const blob = await encodeCanvas(source, abort.signal)
      if (token !== revision) return
      if (!blob) throw new Error('截取投屏画面失败')
      const form = new FormData()
      form.append('file', blob, scope.value === 'screen' ? 'screen.png' : 'crop.png')
      form.append('serial', serial)
      form.append('engine', requestEngine)
      form.append('bbox', JSON.stringify([left / width, top / height, (right - left) / width, (bottom - top) / height]))
      // Inspector owns this overlay so cancellation, filtering and hiding never leave server marks behind.
      form.append('mark', 'false')
      stage = 'recognition'
      const response = await detectOCR(form, abort.signal)
      if (token !== revision || !isSourceReady() || getCanvas() !== canvas || canvas.width !== width || canvas.height !== height) return
      if (frameVersion() !== frame && Date.now() - started > 600) {
        overlayValid = false
        redraw()
        if (!running.value) error.value = '画面已变化，请重新识别'
        return
      }
      result.value = response
      sourceSize.value = { width, height }
      selected.value = null
      capturedAt = started
      capturedFrame = frame
      overlayValid = true
      redraw()
    }
    catch (e) {
      if (token === revision) {
        running.value = false
        overlayValid = false
        error.value = abort.signal.aborted ? 'OCR 识别超时，请重试' : stage === 'capture' ? '截取投屏画面失败，请重试' : '识别失败，请检查 OCR 服务后重试'
        console.warn('[InspectorOCR] 识别失败', { serial, engine: requestEngine, stage, error: e instanceof Error ? e.message : String(e) })
        redraw()
      }
    }
    finally {
      clearTimeout(timeout)
      if (token === revision) {
        loading.value = false
        controller = null
        if (running.value) timer = setTimeout(() => { timer = undefined; void run() }, Math.max(0, 500 - (Date.now() - started)))
      }
    }
  }

  function frameChanged() {
    const canvas = getCanvas()
    if (selectionSize && canvas && (selectionSize.width !== canvas.width || selectionSize.height !== canvas.height)) { invalidate(); return }
    if (result.value && canvas && (sourceSize.value.width !== canvas.width || sourceSize.value.height !== canvas.height)) { invalidate(); return }
    if (overlayValid && frameVersion() !== capturedFrame && Date.now() - capturedAt >= 600) { overlayValid = false; redraw() }
  }

  function draw(ctx: CanvasRenderingContext2D, width: number, height: number) {
    if (overlayValid && showBoxes.value) {
      for (const { box, index } of filtered.value) {
        ctx.strokeStyle = '#00e676'
        ctx.lineWidth = Math.max(selected.value === index ? 4 : 2, width / 500)
        ctx.strokeRect(box.x * width, box.y * height, box.w * width, box.h * height)
        if (selected.value === index) { ctx.fillStyle = 'rgb(0 230 118 / 16%)'; ctx.fillRect(box.x * width, box.y * height, box.w * width, box.h * height) }
      }
    }
    if (scope.value === 'region' && region.value) {
      const b = region.value
      ctx.save()
      ctx.strokeStyle = '#3b82f6'
      ctx.lineWidth = Math.max(2, width / 500)
      ctx.setLineDash([10, 6])
      ctx.strokeRect(b.x * width, b.y * height, b.w * width, b.h * height)
      ctx.restore()
    }
  }

  watch([engine, scope], () => { pointerCancel(); clear() }, { flush: 'sync' })
  watch(continuous, stop, { flush: 'sync' })
  watch([showBoxes, keyword], redraw, { flush: 'sync' })
  watch(selectedNode, node => { if (node) scope.value = 'element'; else if (scope.value === 'element') scope.value = 'screen' }, { flush: 'sync' })
  onUnmounted(() => { stop(); pointerCancel() })
  return { loading, running, continuous, engine, scope, region, selecting, result, error, showBoxes, keyword, selected, filtered, sourceSize,
    stop, clear, invalidate, run, select, beginRegionSelection, coordinates, draw, frameChanged, pointerDown, pointerMove, pointerUp, pointerCancel }
}
