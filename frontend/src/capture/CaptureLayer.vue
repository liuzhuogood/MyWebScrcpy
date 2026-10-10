<script setup lang="ts">
import { Copy, Download, ImagePlus, Undo2, X } from 'lucide-vue-next'
import { computed, nextTick, onMounted, onUnmounted, ref } from 'vue'
import { toast } from 'vue-sonner'
import { Button } from '@/components/ui/button'
import {
  Dialog,
  DialogContent,
  DialogDescription,
  DialogFooter,
  DialogHeader,
  DialogTitle,
} from '@/components/ui/dialog'
import { Input } from '@/components/ui/input'
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from '@/components/ui/select'
import { createTemplate } from '@/api/templates'
import { useCapture } from './useCapture'
import type { CaptureMode, CaptureShape } from './useCapture'

const props = defineProps<{
  serial: string
  getCanvas: () => HTMLCanvasElement | null
  actionsTarget: string
}>()

const emit = defineEmits<{ (e: 'close'): void }>()

const layerRef = ref<HTMLElement | null>(null)
const cap = useCapture(props.getCanvas, () => layerRef.value)
const selectionRef = ref<HTMLElement | null>(null)
const magnifierRef = ref<HTMLCanvasElement | null>(null)
const magnifierOn = ref(false)
const magnifierPos = ref({ x: 0, y: 0 })
const bboxText = ref('')
const bboxError = ref('')

// 投屏上按实际线宽预览黄色选区线的 SVG 数据
const vb = computed(() => {
  const c = props.getCanvas()
  return c ? { w: c.width, h: c.height } : { w: 0, h: 0 }
})
const squareSvg = computed(() => (cap.shape.value === 'square' ? cap.bounds() : null))
const circleSvg = computed(() => (cap.shape.value === 'circle' ? cap.circleCoords() : null))
const freePathD = computed(() => {
  const pts = cap.path.value
  if (!pts || pts.length < 2)
    return ''
  let d = `M ${pts[0].x} ${pts[0].y}`
  for (let i = 1; i < pts.length; i++)
    d += ` L ${pts[i].x} ${pts[i].y}`
  return cap.pathClosed.value ? `${d} Z` : d
})

// 存为模板
const saveOpen = ref(false)
const tplName = ref('')
const tplScope = ref<'device' | 'global'>('device')
const tplPreview = ref('')
const tplSize = ref('')
const tplBusy = ref(false)
let tplBlob: Blob | null = null
let tplUrl: string | null = null

// 拖拽状态
let drag: { mode: 'move' | 'resize', dir?: string, startX: number, startY: number, rect: { x: number, y: number, w: number, h: number } } | null = null
// 自由绘制进行中（指针已捕获）
let drawing = false
let layerObserver: ResizeObserver | undefined

function layerPoint(e: PointerEvent | MouseEvent): { x: number, y: number } {
  const rect = layerRef.value?.getBoundingClientRect()
  return { x: e.clientX - (rect?.left ?? 0), y: e.clientY - (rect?.top ?? 0) }
}

function onLayerDown(e: PointerEvent) {
  if (e.button !== undefined && e.button !== 0)
    return
  const target = e.target as HTMLElement
  // 自由绘制：在层背景上描线；操作条等交互控件内不进入绘制
  if (cap.shape.value === 'freehand') {
    if (target.closest?.('.capture-actions'))
      return
    cap.freehandStart(layerPoint(e))
    drawing = true
    ;(e.currentTarget as HTMLElement).setPointerCapture?.(e.pointerId)
    e.preventDefault()
    return
  }
  const r = cap.rect.value
  if (!r)
    return
  // 只在选区内开始拖拽
  if (!target.closest('[data-selection]'))
    return
  const handle = target.closest('[data-handle]') as HTMLElement | null
  const p = layerPoint(e)
  drag = {
    mode: handle ? 'resize' : 'move',
    dir: handle?.dataset.handle,
    startX: p.x,
    startY: p.y,
    rect: { ...r },
  }
  selectionRef.value?.focus({ preventScroll: true })
  ;(e.currentTarget as HTMLElement).setPointerCapture?.(e.pointerId)
  e.preventDefault()
}

function handleStyle(dir: string): Record<string, string> {
  const base: Record<string, string> = {}
  if (dir.includes('n'))
    base.top = '-6px'
  if (dir.includes('s'))
    base.bottom = '-6px'
  if (dir.includes('w'))
    base.left = '-6px'
  if (dir.includes('e'))
    base.right = '-6px'
  if (dir === 'n' || dir === 's') {
    base.left = '50%'
    base.marginLeft = '-6px'
  }
  if (dir === 'e' || dir === 'w') {
    base.top = '50%'
    base.marginTop = '-6px'
  }
  return base
}

function onLayerMove(e: PointerEvent) {
  updateMagnifier(e.clientX, e.clientY)
  if (cap.shape.value === 'freehand') {
    if (drawing) {
      cap.freehandAdd(layerPoint(e))
      e.preventDefault()
    }
    return
  }
  if (!drag)
    return
  const p = layerPoint(e)
  const dx = p.x - drag.startX
  const dy = p.y - drag.startY
  const start = drag.rect
  const layer = layerRef.value
  const maxW = layer?.clientWidth ?? 0
  const maxH = layer?.clientHeight ?? 0
  const r = cap.rect.value
  if (!r)
    return
  if (drag.mode === 'move') {
    cap.rect.value = {
      ...r,
      x: Math.max(0, Math.min(maxW - r.w, start.x + dx)),
      y: Math.max(0, Math.min(maxH - r.h, start.y + dy)),
    }
  }
  else if (cap.shape.value === 'circle') {
    const delta = Math.max(Math.abs(dx), Math.abs(dy)) * (dx + dy >= 0 ? 1 : -1)
    cap.resize(delta, delta, 'center')
  }
  else {
    const dir = drag.dir || 'se'
    const r0 = { ...start }
    let { x, y, w, h } = r0
    if (dir.includes('e'))
      w = start.w + dx
    if (dir.includes('s'))
      h = start.h + dy
    if (dir.includes('w')) {
      x = start.x + dx
      w = start.w - dx
    }
    if (dir.includes('n')) {
      y = start.y + dy
      h = start.h - dy
    }
    // 归一化负尺寸
    if (w < 0) {
      x += w
      w = -w
    }
    if (h < 0) {
      y += h
      h = -h
    }
    const r = cap.rect.value
    if (r)
      cap.rect.value = { x: Math.max(0, x), y: Math.max(0, y), w: Math.max(8, Math.min(maxW - Math.max(0, x), w)), h: Math.max(8, Math.min(maxH - Math.max(0, y), h)) }
  }
  e.preventDefault()
}

function onLayerUp() {
  if (cap.shape.value === 'freehand') {
    if (drawing) {
      drawing = false
      if (cap.freehandEnd() === 'open')
        toast('线条未闭合，继续描到起点附近即闭合')
    }
    layerRef.value?.focus({ preventScroll: true })
    return
  }
  drag = null
  selectionRef.value?.focus({ preventScroll: true })
}

// 层级键盘：自由绘制模式没有选区焦点，方向键在这里吞掉，防止穿透到投屏变成设备按键
function onLayerKey(e: KeyboardEvent) {
  if (cap.shape.value !== 'freehand')
    return
  if (cap.nudge(e)) {
    e.preventDefault()
    e.stopPropagation()
  }
}

function updateMagnifier(clientX: number, clientY: number) {
  const canvas = props.getCanvas()
  const mg = magnifierRef.value
  const layer = layerRef.value
  if (!canvas?.width || !mg || !layer) {
    magnifierOn.value = false
    return
  }
  const canvasRect = canvas.getBoundingClientRect()
  const px = Math.floor(((clientX - canvasRect.left) * canvas.width) / canvasRect.width)
  const py = Math.floor(((clientY - canvasRect.top) * canvas.height) / canvasRect.height)
  if (px < 0 || py < 0 || px >= canvas.width || py >= canvas.height) {
    magnifierOn.value = false
    return
  }
  const layerRect = layer.getBoundingClientRect()
  magnifierPos.value = { x: clientX - layerRect.left + 16, y: clientY - layerRect.top + 16 }
  magnifierOn.value = true
  const ctx = mg.getContext('2d')
  if (!ctx)
    return
  ctx.imageSmoothingEnabled = false
  const size = 9
  const sx = Math.max(0, Math.min(canvas.width - size, px - 4))
  const sy = Math.max(0, Math.min(canvas.height - size, py - 4))
  const viewSize = 240
  const zoom = 8
  const viewPixels = viewSize / zoom
  if (mg.width !== viewSize || mg.height !== viewSize) {
    mg.width = viewSize
    mg.height = viewSize
  }
  ctx.clearRect(0, 0, viewSize, viewSize)
  ctx.drawImage(canvas, sx, sy, size, size, 0, 0, viewSize, viewSize)
  ctx.strokeStyle = 'rgb(255 255 255 / 28%)'
  ctx.lineWidth = 1
  ctx.beginPath()
  for (let i = 1; i < viewPixels; i++) {
    const p = i * zoom + 0.5
    ctx.moveTo(p, 0); ctx.lineTo(p, viewSize)
    ctx.moveTo(0, p); ctx.lineTo(viewSize, p)
  }
  ctx.stroke()
  if (cap.shape.value === 'freehand') {
    const pts = cap.path.value
    if (pts && pts.length) {
      ctx.strokeStyle = '#ffcc00'
      ctx.lineWidth = cap.strokeWidth.value
      ctx.lineCap = 'round'
      ctx.lineJoin = 'round'
      ctx.beginPath()
      ctx.moveTo((pts[0].x - sx) * zoom, (pts[0].y - sy) * zoom)
      for (let i = 1; i < pts.length; i++)
        ctx.lineTo((pts[i].x - sx) * zoom, (pts[i].y - sy) * zoom)
      ctx.stroke()
    }
  }
  else {
    const r = cap.rect.value
    if (r) {
      const selection = cap.bounds()
      if (selection) {
        ctx.strokeStyle = '#ffcc00'
        ctx.lineWidth = cap.strokeWidth.value
        ctx.strokeRect((selection.x - sx) * zoom, (selection.y - sy) * zoom, selection.width * zoom, selection.height * zoom)
      }
    }
  }
}

function onSelectionKey(e: KeyboardEvent) {
  if (e.key === 'Escape') {
    e.stopPropagation()
    close()
    return
  }
  if (cap.nudge(e)) {
    e.preventDefault()
    e.stopPropagation()
  }
}

function applyBbox() {
  bboxError.value = cap.applyBbox(bboxText.value) || ''
  if (!bboxError.value)
    selectionRef.value?.focus({ preventScroll: true })
}

async function downloadBlob(blob: Blob, name: string) {
  const url = URL.createObjectURL(blob)
  const a = document.createElement('a')
  a.href = url
  a.download = name
  document.body.append(a)
  a.click()
  a.remove()
  setTimeout(() => URL.revokeObjectURL(url), 1000)
}

async function saveDownload() {
  const blob = await cap.toBlob()
  if (!blob) {
    toast.error('截图生成失败，请重试。')
    return
  }
  const b = cap.bounds()
  await downloadBlob(blob, `mywebscrcpy-${props.serial || 'screen'}-${b?.width ?? 0}x${b?.height ?? 0}.png`)
}

async function copyImage() {
  try {
    const blob = await cap.toBlob()
    if (!blob) {
      toast.error('截图生成失败，请重试。')
      return
    }
    await navigator.clipboard.write([new ClipboardItem({ 'image/png': blob })])
    toast.success('已复制选区图片')
  }
  catch {
    toast.error('复制失败，当前浏览器不支持图片写入剪贴板')
  }
}

function copyBbox() {
  const b = cap.bounds()
  if (!b)
    return
  const text = cap.shape.value === 'circle'
    ? JSON.stringify({ shape: 'circle', ...cap.circleCoords(), stroke_width: cap.strokeWidth.value })
    : JSON.stringify(b)
  navigator.clipboard.writeText(text).then(
    () => toast.success('BBOX 已复制'),
    () => toast.error('复制失败'),
  )
}

function openSaveTemplate() {
  const b = cap.bounds()
  if (!b)
    return
  cap.toBlob().then((blob) => {
    if (!blob) {
      toast.error('生成选区图像失败')
      return
    }
    tplBlob = blob
    if (tplUrl)
      URL.revokeObjectURL(tplUrl)
    tplUrl = URL.createObjectURL(blob)
    tplPreview.value = tplUrl
    tplSize.value = `${b.width} × ${b.height} px`
    tplName.value = ''
    tplScope.value = 'device'
    saveOpen.value = true
  })
}

async function submitTemplate() {
  const name = tplName.value.trim()
  if (!name) {
    toast.error('请输入模板名称')
    return
  }
  if (!tplBlob) {
    toast.error('选区图片丢失，请重新选择')
    return
  }
  const canvas = props.getCanvas()
  const form = new FormData()
  form.append('image', tplBlob, 'template.png')
  form.append('name', name)
  form.append('serial', tplScope.value === 'global' ? 'global' : props.serial)
  if (canvas?.width && canvas.height) {
    form.append('scene_width', String(canvas.width))
    form.append('scene_height', String(canvas.height))
  }
  tplBusy.value = true
  try {
    await createTemplate(form)
    toast.success(`模板「${name}」已保存`)
    saveOpen.value = false
    close()
  }
  catch (e) {
    toast.error(e instanceof Error ? e.message : '保存模板失败')
  }
  finally {
    tplBusy.value = false
  }
}

function close() {
  if (tplUrl) {
    URL.revokeObjectURL(tplUrl)
    tplUrl = null
  }
  tplBlob = null
  saveOpen.value = false
  cap.close()
  emit('close')
}

defineExpose({ open: cap.open })

onMounted(async () => {
  // 侧栏挂载后先让投屏适应新空间，再按实际显示尺寸初始化选区。
  await nextTick()
  const layer = layerRef.value
  if (!layer)
    return
  window.dispatchEvent(new Event('resize'))
  const err = cap.open()
  if (err) {
    toast.error(err)
    emit('close')
    return
  }
  let width = layer.clientWidth
  let height = layer.clientHeight
  // 面板伸缩或窗口变化时保留原始画布中的选区，避免显示尺寸改变后坐标漂移。
  layerObserver = new ResizeObserver(() => {
    const nextWidth = layer.clientWidth
    const nextHeight = layer.clientHeight
    if (!nextWidth || !nextHeight)
      return
    const r = cap.rect.value
    if (r && width && height) {
      cap.rect.value = {
        x: r.x * nextWidth / width, y: r.y * nextHeight / height,
        w: r.w * nextWidth / width, h: r.h * nextHeight / height,
      }
    }
    width = nextWidth
    height = nextHeight
  })
  layerObserver.observe(layer)
  await nextTick()
  selectionRef.value?.focus({ preventScroll: true })
})
onUnmounted(() => {
  layerObserver?.disconnect()
  cap.close()
})
</script>

<template>
  <div
    ref="layerRef" data-capture-layer
    class="capture-layer pointer-events-auto absolute inset-0 z-20 touch-none overflow-hidden rounded-sm bg-black/40 outline-none"
    tabindex="0"
    @pointerdown="onLayerDown" @pointermove="onLayerMove" @pointerup="onLayerUp"
    @pointerleave="magnifierOn = false"
    @keydown="onLayerKey"
  >
    <!-- 投屏上按实际线宽预览黄色选区线 -->
    <svg
      v-if="vb.w && vb.h"
      class="pointer-events-none absolute inset-0 h-full w-full"
      :viewBox="`0 0 ${vb.w} ${vb.h}`"
      preserveAspectRatio="none"
      aria-hidden="true"
    >
      <rect
        v-if="cap.shape.value === 'square' && squareSvg"
        :x="squareSvg.x" :y="squareSvg.y" :width="squareSvg.width" :height="squareSvg.height"
        fill="none" stroke="#ffcc00" :stroke-width="cap.strokeWidth.value"
      />
      <circle
        v-else-if="cap.shape.value === 'circle' && circleSvg"
        :cx="circleSvg.center_x" :cy="circleSvg.center_y" :r="circleSvg.radius"
        fill="none" stroke="#ffcc00" :stroke-width="cap.strokeWidth.value"
      />
      <path
        v-else-if="cap.shape.value === 'freehand' && freePathD"
        :d="freePathD"
        fill="none" stroke="#ffcc00" :stroke-width="cap.strokeWidth.value"
        stroke-linecap="round" stroke-linejoin="round"
      />
    </svg>

    <!-- 选区 -->
    <div
      v-if="cap.rect.value && cap.shape.value !== 'freehand'" ref="selectionRef" data-selection tabindex="0"
      class="absolute border-2 border-dashed border-white bg-white/5 outline-none"
      :class="{ 'rounded-full': cap.shape.value === 'circle' }"
      :style="{
        left: `${cap.rect.value.x}px`, top: `${cap.rect.value.y}px`,
        width: `${cap.rect.value.w}px`, height: `${cap.rect.value.h}px`,
      }"
      role="region" aria-label="截图选区，方向键微调，Shift加方向键缩放"
      @keydown="onSelectionKey"
    >
      <span
        v-for="dir in (['nw', 'n', 'ne', 'e', 'se', 's', 'sw', 'w'] as const)" :key="dir" data-handle :data-dir="dir"
        class="absolute h-3 w-3 rounded-sm border border-white bg-sky-500"
        :style="handleStyle(dir)"
      />
    </div>

    <!-- 放大镜 -->
    <canvas
      v-show="magnifierOn" ref="magnifierRef" width="240" height="240"
      class="pointer-events-none absolute h-60 w-60 rounded-md border-2 border-white/80 bg-black"
      :style="{ left: `${magnifierPos.x}px`, top: `${magnifierPos.y}px` }"
      aria-hidden="true"
    />

    <Teleport defer :to="actionsTarget">
    <div data-capture-layer class="capture-actions flex w-full flex-col items-stretch gap-1.5 p-3">
      <span class="px-1 font-mono text-[11px] text-muted-foreground">{{ cap.sizeText.value }}</span>
      <div class="flex items-center gap-2">
        <Select :model-value="cap.shape.value" @update:model-value="(v) => cap.setShape(v as CaptureShape)">
          <SelectTrigger class="h-7 w-28 text-xs" aria-label="选区形状"><SelectValue /></SelectTrigger>
          <SelectContent>
            <SelectItem value="square">矩形</SelectItem>
            <SelectItem value="circle">圆形</SelectItem>
            <SelectItem value="freehand">自由绘制</SelectItem>
          </SelectContent>
        </Select>
        <label class="flex shrink-0 items-center gap-1 text-[11px] text-muted-foreground">
          线宽
          <Input
            :model-value="cap.strokeWidth.value" type="number" :min="1" :max="10"
            class="h-7 w-12 px-1 text-xs" aria-label="描边线宽"
            @update:model-value="(v) => { cap.strokeWidth.value = Math.min(10, Math.max(1, Math.round(Number(v) || 1))) }"
          />
        </label>
      </div>
      <Select :model-value="cap.mode.value" @update:model-value="(v) => { cap.mode.value = v as CaptureMode }">
        <SelectTrigger class="h-7 w-full text-xs" aria-label="截图方式"><SelectValue /></SelectTrigger>
        <SelectContent>
          <SelectItem value="inside">截取线条内的图片</SelectItem>
          <SelectItem value="line">截图线条区域（空心）</SelectItem>
        </SelectContent>
      </Select>
      <Input
        v-model="bboxText" class="h-7 w-full font-mono text-[11px]" placeholder='BBOX JSON，如 {"x":0,"y":0,"width":100,"height":100}'
        aria-label="BBOX JSON" @keydown.enter.prevent="applyBbox"
      />
      <p v-if="bboxError" class="text-[11px] text-destructive">{{ bboxError }}</p>
      <Button size="sm" variant="secondary" class="h-7 text-xs" @click="applyBbox()">应用</Button>
      <Button size="sm" variant="secondary" class="h-7 text-xs" @click="saveDownload()"><Download class="h-3.5 w-3.5" />保存</Button>
      <Button size="sm" variant="secondary" class="h-7 text-xs" :disabled="cap.shape.value !== 'freehand' || !cap.canUndo.value" @click="cap.undo()"><Undo2 class="h-3.5 w-3.5" />撤销</Button>
      <Button size="sm" variant="secondary" class="h-7 text-xs" @click="copyImage()"><Copy class="h-3.5 w-3.5" />复制</Button>
      <Button size="sm" variant="secondary" class="h-7 text-xs" @click="copyBbox()">BBOX</Button>
      <Button size="sm" variant="secondary" class="h-7 text-xs" @click="openSaveTemplate()"><ImagePlus class="h-3.5 w-3.5" />存为模板</Button>
      <Button size="sm" variant="ghost" class="h-7 text-xs" @click="close()"><X class="h-3.5 w-3.5" />取消</Button>
    </div>
    </Teleport>

    <!-- 存为模板 -->
    <Dialog v-model:open="saveOpen">
      <DialogContent @pointerdown.stop @pointermove.stop @pointerup.stop>
        <DialogHeader>
          <DialogTitle>存为模板</DialogTitle>
          <DialogDescription>将当前选区存为匹配模板。</DialogDescription>
        </DialogHeader>
        <img v-if="tplPreview" :src="tplPreview" alt="模板预览" class="mx-auto max-h-48 rounded-md border object-contain" />
        <p class="font-mono text-xs text-muted-foreground">{{ tplSize }}</p>
        <div class="space-y-2">
          <label class="text-sm font-medium" for="tpl-name">模板名称</label>
          <Input id="tpl-name" v-model="tplName" autocomplete="off" />
          <label class="text-sm font-medium">作用域</label>
          <Select v-model="tplScope">
            <SelectTrigger aria-label="模板作用域"><SelectValue /></SelectTrigger>
            <SelectContent>
              <SelectItem value="device">当前设备</SelectItem>
              <SelectItem value="global">全局</SelectItem>
            </SelectContent>
          </Select>
        </div>
        <DialogFooter>
          <Button variant="ghost" @click="saveOpen = false">取消</Button>
          <Button :disabled="tplBusy || !tplName.trim()" @click="submitTemplate()">{{ tplBusy ? '保存中…' : '保存模板' }}</Button>
        </DialogFooter>
      </DialogContent>
    </Dialog>
  </div>
</template>

<style scoped>
.capture-magnifier {
  background-image: linear-gradient(rgb(255 255 255 / 18%) 1px, transparent 1px), linear-gradient(90deg, rgb(255 255 255 / 18%) 1px, transparent 1px);
  background-size: 12px 12px;
}
</style>
