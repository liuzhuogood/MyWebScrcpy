import { computed, inject, onUnmounted, provide, ref } from 'vue'
import type { InjectionKey, Ref } from 'vue'
import { fetchPageInfo, fetchXml, type PageInfo } from '@/api/ui'
import type { OCRDetectResponse } from '@/api/ocr'
import { useInspectorOcr } from './useInspectorOcr'
import { inspectorOpen, inspectorTab, inspectionMode, xmlHitTest } from './guard'
import { evalXPath, makeXPath, measureBasis, parseXml, toBox, type Box } from './xmlUtils'

export interface InspectorContext {
  serial: string
  getCanvas: () => HTMLCanvasElement | null
  overlayRef: Ref<HTMLCanvasElement | null>
  // 取色
  colorPoint: Ref<{ x: number, y: number } | null>
  colorPinned: Ref<boolean>
  colorText: Ref<{ pos: string, norm: string, color: string }>
  magnifierRef: Ref<HTMLCanvasElement | null>
  hoverColor: (e: MouseEvent) => void
  leaveColor: () => void
  clickCanvas: (e: MouseEvent) => boolean
  closeInspector: () => void
  redraw: () => void
  // XML
  xmlLoading: Ref<boolean>
  xmlError: Ref<string>
  xmlEmpty: Ref<boolean>
  xmlDoc: Ref<Document | null>
  xmlTime: Ref<string>
  xpathText: Ref<string>
  xpathResult: Ref<string>
  xpathError: Ref<boolean>
  showAll: Ref<boolean>
  selectNode: (node: Element | null) => void
  selectedNode: Ref<Element | null>
  selectedTag: Ref<string>
  selectedAttrs: Ref<{ name: string, value: string, checked: boolean, disabled: boolean }[]>
  loadXml: () => Promise<void>
  toggleShowAll: () => void
  clearXpath: () => void
  testXpath: () => void
  toggleAttr: (name: string, on: boolean) => void
  treeRoots: Ref<Element[]>
  // 页面信息
  pageLoading: Ref<boolean>
  pageError: Ref<string>
  pageInfo: Ref<PageInfo | null>
  loadPageInfo: () => Promise<void>
  // OCR
  ocr: ReturnType<typeof useInspectorOcr>
  ocrLoading: Ref<boolean>
  ocrResult: Ref<OCRDetectResponse | null>
  ocrError: Ref<string>
  runOcr: () => Promise<void>
  clearOcr: () => void
}

const key: InjectionKey<InspectorContext> = Symbol('inspector')

export function provideInspector(serial: string, getCanvas: () => HTMLCanvasElement | null, frameVersion?: () => number, isSourceReady?: () => boolean): InspectorContext {
  const overlayRef = ref<HTMLCanvasElement | null>(null)

  // ---------- overlay ----------
  function overlayCtx(): CanvasRenderingContext2D | null {
    const canvas = getCanvas()
    const overlay = overlayRef.value
    if (!canvas || !overlay)
      return null
    if (overlay.width !== canvas.width || overlay.height !== canvas.height) {
      overlay.width = canvas.width
      overlay.height = canvas.height
    }
    return overlay.getContext('2d')
  }

  function clearOverlay() {
    const overlay = overlayRef.value
    overlay?.getContext('2d')?.clearRect(0, 0, overlay.width, overlay.height)
  }

  // ---------- 取色 ----------
  const colorPoint = ref<{ x: number, y: number } | null>(null)
  const colorPinned = ref(false)
  const colorText = ref({ pos: '移动鼠标到画面上', norm: '—', color: '—' })
  const magnifierRef = ref<HTMLCanvasElement | null>(null)
  let raf = 0
  // 放大镜：20×20 源 → 240×240 显示（12 倍）
  const MAG_SRC = 20
  const MAG_SHOW = 240
  const MAG_SCALE = MAG_SHOW / MAG_SRC

  function sourcePoint(e: MouseEvent): { x: number, y: number } | null {
    const canvas = getCanvas()
    if (!canvas?.width || !canvas.height)
      return null
    const rect = canvas.getBoundingClientRect()
    return {
      x: Math.max(0, Math.min(canvas.width - 1, Math.floor(((e.clientX - rect.left) * canvas.width) / rect.width))),
      y: Math.max(0, Math.min(canvas.height - 1, Math.floor(((e.clientY - rect.top) * canvas.height) / rect.height))),
    }
  }

  function drawColor() {
    raf = 0
    const canvas = getCanvas()
    const p = colorPoint.value
    if (!canvas || !p)
      return
    const pixel = canvas.getContext('2d')?.getImageData(p.x, p.y, 1, 1).data
    if (!pixel)
      return
    const hex = `#${[pixel[0], pixel[1], pixel[2]].map(v => v.toString(16).padStart(2, '0')).join('').toUpperCase()}`
    const nx = canvas.width > 1 ? p.x / (canvas.width - 1) : 0
    const ny = canvas.height > 1 ? p.y / (canvas.height - 1) : 0
    colorText.value = {
      pos: `${p.x} × ${p.y}`,
      norm: `${nx.toFixed(4)} × ${ny.toFixed(4)}`,
      color: `${hex}  rgb(${pixel[0]}, ${pixel[1]}, ${pixel[2]})`,
    }
    // 放大镜
    const mg = magnifierRef.value
    if (mg) {
      if (mg.width !== MAG_SHOW || mg.height !== MAG_SHOW) {
        mg.width = MAG_SHOW
        mg.height = MAG_SHOW
      }
      const ctx = mg.getContext('2d')
      if (ctx) {
        ctx.imageSmoothingEnabled = false
        const half = Math.floor(MAG_SRC / 2)
        const sx = Math.max(0, Math.min(canvas.width - MAG_SRC, p.x - half))
        const sy = Math.max(0, Math.min(canvas.height - MAG_SRC, p.y - half))
        ctx.clearRect(0, 0, MAG_SHOW, MAG_SHOW)
        ctx.drawImage(canvas, sx, sy, MAG_SRC, MAG_SRC, 0, 0, MAG_SHOW, MAG_SHOW)
        // 格子线（与区域截图放大镜同风格）
        ctx.strokeStyle = 'rgb(255 255 255 / 28%)'
        ctx.lineWidth = 1
        ctx.beginPath()
        for (let i = 1; i < MAG_SRC; i++) {
          const p = i * MAG_SCALE + 0.5
          ctx.moveTo(p, 0); ctx.lineTo(p, MAG_SHOW)
          ctx.moveTo(0, p); ctx.lineTo(MAG_SHOW, p)
        }
        ctx.stroke()
        ctx.strokeStyle = '#fff'
        ctx.lineWidth = 2
        ctx.strokeRect((p.x - sx) * MAG_SCALE, (p.y - sy) * MAG_SCALE, MAG_SCALE, MAG_SCALE)
      }
    }
    drawBoxes()
  }

  function hoverColor(e: MouseEvent) {
    if (!inspectorOpen.value || !inspectionMode.value || inspectorTab.value !== 'color' || colorPinned.value)
      return
    const p = sourcePoint(e)
    if (!p)
      return
    colorPoint.value = p
    if (!raf)
      raf = requestAnimationFrame(drawColor)
  }

  function leaveColor() {
    if (!inspectorOpen.value || colorPinned.value)
      return
    colorPoint.value = null
    drawBoxes()
  }

  // ---------- XML ----------
  const xmlLoading = ref(false)
  const xmlError = ref('')
  const xmlDoc = ref<Document | null>(null)
  const xmlTime = ref('')
  const xpathText = ref('')
  const xpathResult = ref('尚未测试')
  const xpathError = ref(false)
  const showAll = ref(false)
  const xpathNodes = ref<Element[]>([])
  const selectedNode = ref<Element | null>(null)
  const locatorAttrs = ref<Set<string>>(new Set())
  const basis = ref<{ width: number, height: number } | null>(null)

  const ocr = useInspectorOcr(serial, getCanvas, selectedNode, () => {
    const canvas = getCanvas()
    return selectedNode.value && canvas ? toBox(selectedNode.value, basis.value, canvas.width, canvas.height) : null
  }, drawBoxes, frameVersion, isSourceReady)

  const xmlEmpty = computed(() => !xmlLoading.value && !xmlError.value && !xmlDoc.value)
  const treeRoots = computed<Element[]>(() => {
    const root = xmlDoc.value?.documentElement
    return root ? [root] : []
  })
  const selectedTag = computed(() => selectedNode.value?.tagName ?? '')
  const selectedAttrs = computed(() => {
    const node = selectedNode.value
    if (!node)
      return []
    return [...node.attributes].map(a => ({
      name: a.name,
      value: a.value,
      checked: locatorAttrs.value.has(a.name),
      disabled: !a.value,
    }))
  })

  function boxes(): Box[] {
    const canvas = getCanvas()
    const doc = xmlDoc.value
    if (!doc || !canvas)
      return []
    const nodes = showAll.value
      ? [...doc.querySelectorAll('[bounds]')]
      : xpathNodes.value
    return nodes
      .map(n => toBox(n, basis.value, canvas.width, canvas.height))
      .filter((b): b is Box => !!b)
  }

  function drawBoxes() {
    const ctx = overlayCtx()
    const canvas = getCanvas()
    if (!ctx || !canvas)
      return
    ctx.clearRect(0, 0, canvas.width, canvas.height)
    if (inspectorTab.value === 'ocr') {
      ocr.draw(ctx, canvas.width, canvas.height)
      return
    }
    if (inspectorTab.value === 'color') {
      const p = colorPoint.value
      if (!p)
        return
      // 十字线 + 圆圈
      ctx.save()
      ctx.strokeStyle = '#ffd54a'
      ctx.lineWidth = Math.max(1, canvas.width / 900)
      ctx.setLineDash([8, 6])
      ctx.beginPath()
      ctx.moveTo(p.x, 0); ctx.lineTo(p.x, canvas.height)
      ctx.moveTo(0, p.y); ctx.lineTo(canvas.width, p.y)
      ctx.stroke()
      ctx.setLineDash([])
      ctx.beginPath()
      ctx.arc(p.x, p.y, Math.max(5, canvas.width / 150), 0, Math.PI * 2)
      ctx.stroke()
      ctx.restore()
      return
    }
    ctx.strokeStyle = '#00e676'
    ctx.lineWidth = Math.max(2, canvas.width / 500)
    boxes().forEach(b => ctx.strokeRect(b.x, b.y, b.w, b.h))
    if (selectedNode.value) {
      const box = toBox(selectedNode.value, basis.value, canvas.width, canvas.height)
      if (box) {
        ctx.lineWidth = Math.max(3, canvas.width / 350)
        ctx.strokeRect(box.x, box.y, box.w, box.h)
      }
    }
  }

  async function loadXml() {
    xmlLoading.value = true
    xmlError.value = ''
    xmlHitTest.fn = null
    selectNode(null)
    xpathNodes.value = []
    showAll.value = false
    xpathText.value = ''
    xpathResult.value = '尚未测试'
    try {
      const data = await fetchXml(serial)
      const doc = parseXml(data.xml)
      xmlDoc.value = doc
      basis.value = measureBasis(doc, data.display_size)
      xmlTime.value = data.captured_at ? `快照时间：${data.captured_at}` : ''
      xmlHitTest.fn = (x, y) => hitBox(x, y) !== null
    }
    catch (e) {
      xmlDoc.value = null
      xmlError.value = e instanceof Error ? e.message : '获取失败'
    }
    finally {
      xmlLoading.value = false
      drawBoxes()
    }
  }

  function selectNode(node: Element | null) {
    selectedNode.value = node
    locatorAttrs.value = new Set()
    if (node) {
      // 默认定位属性：非空 resource-id 优先，否则非空 text
      if (node.getAttribute('resource-id'))
        locatorAttrs.value.add('resource-id')
      else if (node.getAttribute('text'))
        locatorAttrs.value.add('text')
      xpathText.value = makeXPath(node, locatorAttrs.value)
    }
    drawBoxes()
  }

  function toggleAttr(name: string, on: boolean) {
    const next = new Set(locatorAttrs.value)
    if (on)
      next.add(name)
    else next.delete(name)
    locatorAttrs.value = next
    if (selectedNode.value)
      xpathText.value = makeXPath(selectedNode.value, next)
    testXpath()
  }

  function testXpath() {
    const doc = xmlDoc.value
    const expr = xpathText.value.trim()
    if (!doc || !expr) {
      xpathNodes.value = []
      showAll.value = false
      xpathResult.value = '尚未测试'
      xpathError.value = false
      drawBoxes()
      return
    }
    try {
      const { total, nodes } = evalXPath(doc, expr)
      xpathNodes.value = nodes
      showAll.value = false
      xpathResult.value = `${total} 个匹配元素`
      xpathError.value = false
    }
    catch {
      xpathNodes.value = []
      xpathResult.value = 'XPath 无效'
      xpathError.value = true
    }
    drawBoxes()
  }

  function toggleShowAll() {
    if (!xmlDoc.value)
      return
    showAll.value = !showAll.value
    const canvas = getCanvas()
    const count = showAll.value && canvas
      ? [...xmlDoc.value.querySelectorAll('[bounds]')]
        .map(n => toBox(n, basis.value, canvas.width, canvas.height))
        .filter(Boolean).length
      : 0
    xpathResult.value = showAll.value ? `已显示 ${count} 个元素` : '尚未测试'
    xpathError.value = false
    drawBoxes()
  }

  function clearXpath() {
    xpathText.value = ''
    xpathNodes.value = []
    showAll.value = false
    xpathResult.value = '尚未测试'
    xpathError.value = false
    drawBoxes()
  }

  function hitBox(clientX: number, clientY: number): Element | null {
    const canvas = getCanvas()
    const doc = xmlDoc.value
    if (!canvas || !doc)
      return null
    const rect = canvas.getBoundingClientRect()
    const px = Math.floor(((clientX - rect.left) * canvas.width) / rect.width)
    const py = Math.floor(((clientY - rect.top) * canvas.height) / rect.height)
    const all = showAll.value
      ? [...doc.querySelectorAll('[bounds]')]
      : [...xpathNodes.value, ...(selectedNode.value ? [selectedNode.value] : [])]
    return all
      .map(n => toBox(n, basis.value, canvas.width, canvas.height))
      .filter((b): b is Box => !!b)
      .filter(b => px >= b.x && px <= b.x + b.w && py >= b.y && py <= b.y + b.h)
      .sort((a, b) => a.w * a.h - b.w * b.h)[0]?.node ?? null
  }

  /** 画布点击：取色 Tab 切换固定；XML Tab 点中绿框选中节点；返回 true 表示已消费 */
  function clickCanvas(e: MouseEvent): boolean {
    if (!inspectorOpen.value || !inspectionMode.value)
      return false
    if (inspectorTab.value === 'color') {
      colorPinned.value = !colorPinned.value
      const p = sourcePoint(e)
      if (p) {
        colorPoint.value = p
        if (!raf)
          raf = requestAnimationFrame(drawColor)
      }
      return true
    }
    if (inspectorTab.value === 'xml' && xmlDoc.value) {
      const node = hitBox(e.clientX, e.clientY)
      if (node) {
        selectNode(node)
        testXpath()
        return true
      }
    }
    return false
  }

  // ---------- 页面信息 ----------
  const pageLoading = ref(false)
  const pageError = ref('')
  const pageInfo = ref<PageInfo | null>(null)
  const pageFetched = ref(false)

  async function loadPageInfo() {
    pageLoading.value = true
    pageError.value = ''
    try {
      pageInfo.value = await fetchPageInfo(serial)
      pageFetched.value = true
    }
    catch (e) {
      pageError.value = e instanceof Error ? e.message : '获取失败'
    }
    finally {
      pageLoading.value = false
    }
  }

  // ---------- 清理 ----------
  function closeInspector() {
    xmlHitTest.fn = null
    colorPinned.value = false
    colorPoint.value = null
    selectNode(null)
    clearXpath()
    clearOverlay()
    ocr.invalidate()
  }

  onUnmounted(() => {
    if (raf)
      cancelAnimationFrame(raf)
    closeInspector()
  })

  const ctx: InspectorContext = {
    serial,
    getCanvas,
    overlayRef,
    colorPoint,
    colorPinned,
    colorText,
    magnifierRef,
    hoverColor,
    leaveColor,
    clickCanvas,
    closeInspector,
    redraw: drawBoxes,
    xmlLoading,
    xmlError,
    xmlEmpty,
    xmlDoc,
    xmlTime,
    xpathText,
    xpathResult,
    xpathError,
    showAll,
    selectedNode,
    selectNode,
    selectedTag,
    selectedAttrs,
    loadXml,
    toggleShowAll,
    clearXpath,
    testXpath,
    toggleAttr,
    treeRoots,
    pageLoading,
    pageError,
    pageInfo,
    loadPageInfo,
    ocr,
    ocrLoading: ocr.loading,
    ocrResult: ocr.result,
    ocrError: ocr.error,
    runOcr: ocr.run,
    clearOcr: ocr.clear,
  }
  provide(key, ctx)
  return ctx
}

export function useInspector(): InspectorContext {
  const ctx = inject(key)
  if (!ctx)
    throw new Error('useInspector 必须在 InspectorPanel 内使用')
  return ctx
}
