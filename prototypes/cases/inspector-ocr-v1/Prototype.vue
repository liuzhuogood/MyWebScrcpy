<script setup lang="ts">
import { computed, onUnmounted, ref } from 'vue'
import { ArrowLeft, Check, Copy, Crop, Loader2, Maximize, ScanSearch, ScanText, Square, X } from 'lucide-vue-next'
import { Badge } from '@/components/ui/badge'
import { Button } from '@/components/ui/button'
import { Input } from '@/components/ui/input'
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from '@/components/ui/select'
import { Switch } from '@/components/ui/switch'
import { Tabs, TabsContent, TabsList, TabsTrigger } from '@/components/ui/tabs'
import type { OCRBox } from '@/api/ocr'

const open = ref(true)
const tab = ref('ocr')
const scope = ref('screen')
const engine = ref('auto')
const continuous = ref(false)
const boxesVisible = ref(true)
const busy = ref(false)
const running = ref(false)
const completed = ref(false)
const selected = ref<number | null>(null)
const keyword = ref('')
const copied = ref('')
const error = ref('')
const area = ref<{ x: number, y: number, w: number, h: number } | null>(null)
const anchor = ref<{ x: number, y: number } | null>(null)
let timer: ReturnType<typeof setTimeout> | undefined
let copyTimer: ReturnType<typeof setTimeout> | undefined
let revision = 0

// 原型内的画面与文字框共用坐标，不接设备或真实服务。
const screenSize = { width: 1080, height: 2400 }
const source: OCRBox[] = [
  { text: '设置', confidence: .998, x: .10, y: .135, w: .20, h: .045 },
  { text: '账号与安全', confidence: .989, x: .13, y: .30, w: .44, h: .035 },
  { text: '消息通知', confidence: .996, x: .13, y: .39, w: .35, h: .035 },
  { text: '隐私设置', confidence: .983, x: .13, y: .48, w: .35, h: .035 },
  { text: '通用', confidence: .997, x: .13, y: .61, w: .18, h: .035 },
  { text: '关于我们', confidence: .992, x: .13, y: .70, w: .35, h: .035 },
]
const results = ref<OCRBox[]>([])
const filtered = computed(() => results.value.map((box, index) => ({ box, index })).filter(({ box }) => box.text.includes(keyword.value.trim())))
const activeEngine = computed(() => ({ auto: 'Indra OCR', indraocr: 'Indra OCR', macocr: 'macOCR', paddleocr: 'PaddleOCR' })[engine.value])
const boxStyle = (box: { x: number, y: number, w: number, h: number }) => ({ left: `${box.x * 100}%`, top: `${box.y * 100}%`, width: `${box.w * 100}%`, height: `${box.h * 100}%` })
const pixelCoordinates = (box: OCRBox) => `x:${Math.round(box.x * screenSize.width)} y:${Math.round(box.y * screenSize.height)} w:${Math.round(box.w * screenSize.width)} h:${Math.round(box.h * screenSize.height)} px`

function stop() {
  revision++
  if (timer) clearTimeout(timer)
  busy.value = running.value = false
}
function reset() {
  stop()
  results.value = []
  selected.value = null
  completed.value = false
  error.value = ''
}
function changeScope(value: string) {
  reset()
  scope.value = value
  area.value = null
}
function recognize() {
  if (scope.value === 'region' && !area.value) { error.value = '请先在画面上框选区域'; return }
  if (timer) clearTimeout(timer)
  busy.value = true
  error.value = ''
  running.value = continuous.value
  const token = ++revision
  timer = setTimeout(() => {
    if (token !== revision) return
    results.value = source.filter(box => {
      if (scope.value === 'screen') return true
      const a = area.value!
      return box.x + box.w / 2 >= a.x && box.x + box.w / 2 <= a.x + a.w && box.y + box.h / 2 >= a.y && box.y + box.h / 2 <= a.y + a.h
    })
    selected.value = null
    completed.value = true
    busy.value = false
    if (running.value) timer = setTimeout(recognize, 1000)
  }, 420)
}
function drag(event: PointerEvent, start = false) {
  if (scope.value !== 'region' || !open.value || tab.value !== 'ocr' || running.value || busy.value) return
  const target = event.currentTarget as HTMLElement
  const rect = target.getBoundingClientRect()
  const point = { x: Math.max(0, Math.min(1, (event.clientX - rect.left) / rect.width)), y: Math.max(0, Math.min(1, (event.clientY - rect.top) / rect.height)) }
  if (start) {
    reset()
    target.setPointerCapture(event.pointerId)
    anchor.value = point
    area.value = null
  }
  else if (anchor.value) {
    area.value = { x: Math.min(point.x, anchor.value.x), y: Math.min(point.y, anchor.value.y), w: Math.abs(point.x - anchor.value.x), h: Math.abs(point.y - anchor.value.y) }
  }
}
function endDrag() {
  if (area.value && (area.value.w < .02 || area.value.h < .02)) area.value = null
  anchor.value = null
}
async function copy(text: string, key: string) {
  try {
    await navigator.clipboard.writeText(text)
    copied.value = key
    if (copyTimer) clearTimeout(copyTimer)
    copyTimer = setTimeout(() => { copied.value = '' }, 1400)
  }
  catch { error.value = '复制失败，请重试' }
}
onUnmounted(() => { stop(); if (copyTimer) clearTimeout(copyTimer) })
</script>

<template>
  <div class="overflow-hidden rounded-xl border bg-background shadow-sm">
    <header class="flex flex-wrap items-center gap-2 border-b px-3 py-2.5">
      <Button variant="ghost" size="icon" aria-label="返回设备" @click="open = false; stop()"><ArrowLeft class="size-4" /></Button>
      <div class="mr-auto"><h1 class="text-sm font-semibold">Pixel 8</h1><p class="text-[11px] text-muted-foreground">{{ screenSize.width }} × {{ screenSize.height }}</p></div>
      <Badge variant="outline" class="mr-2 text-xs">已连接</Badge>
      <Button :variant="open ? 'secondary' : 'ghost'" size="sm" @click="open = !open; stop()"><ScanSearch class="mr-1.5 size-4" />检查</Button>
      <Button variant="ghost" size="icon" aria-label="放大画面" @click="open = !open; stop()"><Maximize class="size-4" /></Button>
    </header>

    <div class="grid min-h-[650px]" :class="open ? 'md:grid-cols-[minmax(0,1fr)_350px]' : ''">
      <main class="relative flex min-w-0 items-center justify-center bg-muted/35 px-5 py-7">
        <div class="relative aspect-[9/19] w-[250px] max-w-full select-none overflow-hidden rounded-[27px] border-[6px] border-foreground/85 bg-background shadow-xl"
          :class="scope === 'region' && open && tab === 'ocr' ? 'cursor-crosshair touch-none' : ''"
          @pointerdown="drag($event, true)" @pointermove="drag($event)" @pointerup="endDrag" @pointercancel="endDrag">
          <div class="absolute left-0 right-0 top-[3%] flex justify-between px-5 text-[10px] font-semibold"><span>9:41</span><span>••• ▰</span></div>
          <div class="absolute left-[42%] top-[2%] h-2.5 w-[16%] rounded-full bg-foreground/80" />
          <div class="absolute left-[9%] top-[22%] h-[33%] w-[82%] rounded-xl bg-muted/70" />
          <div class="absolute left-[9%] top-[58%] h-[20%] w-[82%] rounded-xl bg-muted/70" />
          <div v-for="(box, index) in source" :key="box.text" class="absolute flex items-center whitespace-nowrap text-[13px]" :class="index === 0 ? 'text-[21px] font-bold' : ''" :style="boxStyle(box)">{{ box.text }}</div>
          <span v-for="y in [30, 39, 48, 61, 70]" :key="y" class="absolute right-[14%] text-xs text-muted-foreground" :style="{ top: `${y}%` }">›</span>
          <div class="absolute bottom-[2%] left-[34%] h-1 w-[32%] rounded-full bg-foreground/70" />
          <template v-if="open && tab === 'ocr' && boxesVisible && completed">
            <button v-for="({ box, index }) in filtered" :key="index" type="button" :aria-label="`定位 ${box.text}`" :style="boxStyle(box)" class="absolute rounded-[2px] border border-emerald-500 bg-emerald-500/5 transition-colors focus-visible:outline-2 focus-visible:outline-primary" :class="selected === index ? 'border-2 bg-emerald-500/20 ring-2 ring-emerald-500/20' : ''" @click.stop="selected = index" />
          </template>
          <div v-if="area && scope === 'region' && open && tab === 'ocr'" class="pointer-events-none absolute border-2 border-dashed border-primary bg-primary/10" :style="boxStyle(area)" />
        </div>
        <span v-if="scope === 'region' && open && tab === 'ocr' && !area" class="absolute bottom-3 text-xs text-muted-foreground">在画面上拖动框选识别区域</span>
      </main>

      <aside v-if="open" class="flex min-w-0 flex-col border-t md:border-l md:border-t-0">
        <div class="flex items-center justify-between px-4 py-3"><h2 class="text-sm font-semibold">检查</h2><Button variant="ghost" size="icon" class="size-7" aria-label="关闭检查" @click="open = false; stop()"><X class="size-4" /></Button></div>
        <Tabs v-model="tab" class="flex min-h-0 flex-1 flex-col gap-0" @update:model-value="stop()">
          <TabsList class="mx-3 grid grid-cols-5"><TabsTrigger value="page">页面</TabsTrigger><TabsTrigger value="xml">XML</TabsTrigger><TabsTrigger value="color">取色</TabsTrigger><TabsTrigger value="ocr">OCR</TabsTrigger><TabsTrigger value="log">记录</TabsTrigger></TabsList>
          <TabsContent value="ocr" class="m-0 flex flex-1 flex-col gap-4 p-4">
            <div class="space-y-3">
              <Select v-model="engine" @update:model-value="reset()"><SelectTrigger aria-label="识别引擎" class="w-full text-xs"><SelectValue /></SelectTrigger><SelectContent><SelectItem value="auto">自动 · 优先 Indra OCR</SelectItem><SelectItem value="indraocr">Indra OCR</SelectItem><SelectItem value="macocr">macOCR</SelectItem><SelectItem value="paddleocr">PaddleOCR</SelectItem></SelectContent></Select>
              <div class="grid grid-cols-2 gap-1 rounded-lg bg-muted p-1"><Button size="sm" :variant="scope === 'screen' ? 'default' : 'ghost'" :aria-pressed="scope === 'screen'" class="h-7 text-xs" @click="changeScope('screen')"><ScanText class="mr-1.5 size-3.5" />全屏</Button><Button size="sm" :variant="scope === 'region' ? 'default' : 'ghost'" :aria-pressed="scope === 'region'" class="h-7 text-xs" @click="changeScope('region')"><Crop class="mr-1.5 size-3.5" />框选区域</Button></div>
              <div class="flex items-center justify-between text-xs"><label for="continuous-ocr">连续识别</label><Switch id="continuous-ocr" v-model="continuous" @update:model-value="stop()" /></div>
              <Button v-if="running" variant="destructive" size="sm" class="w-full" @click="stop"><Square class="mr-1.5 size-3.5" />停止识别</Button>
              <Button v-else size="sm" class="w-full" :disabled="busy" @click="recognize"><Loader2 v-if="busy" class="mr-1.5 size-4 animate-spin" /><ScanText v-else class="mr-1.5 size-4" />{{ busy ? '识别中…' : completed ? '重新识别' : '开始识别' }}</Button>
            </div>
            <p v-if="error" role="alert" class="text-xs text-destructive">{{ error }}</p>
            <template v-if="completed">
              <div class="space-y-3 border-t pt-3">
                <div class="flex items-center justify-between text-xs"><span class="text-muted-foreground">{{ activeEngine }} · 48 ms · {{ results.length }} 处</span><Button size="sm" variant="ghost" class="h-6 px-1.5 text-xs" :disabled="!results.length" @click="copy(results.map(b => b.text).join('\n'), 'all')"><Check v-if="copied === 'all'" class="mr-1 size-3" /><Copy v-else class="mr-1 size-3" />{{ copied === 'all' ? '已复制' : '复制全部' }}</Button></div>
                <div class="flex items-center justify-between text-xs"><label for="ocr-boxes">显示文字框</label><Switch id="ocr-boxes" v-model="boxesVisible" /></div>
                <Input v-if="results.length" v-model="keyword" aria-label="筛选识别文字" placeholder="筛选文字" class="h-8 text-xs" />
              </div>
              <div v-if="filtered.length" class="max-h-[310px] space-y-1 overflow-auto">
                <div v-for="({ box, index }) in filtered" :key="index" class="flex items-center gap-2 rounded-lg border p-2.5" :class="selected === index ? 'border-primary bg-primary/5' : 'border-transparent hover:bg-muted/60'">
                  <button type="button" class="min-w-0 flex-1 text-left text-sm focus-visible:outline-2 focus-visible:outline-primary" :aria-label="`选中 ${box.text}`" @click="selected = index"><span class="block truncate">{{ box.text }}</span><span class="block text-[10px] text-muted-foreground">{{ (box.confidence * 100).toFixed(1) }}%</span><span class="block break-words font-mono text-[10px] leading-4 text-muted-foreground">{{ pixelCoordinates(box) }}</span></button>
                  <Button variant="ghost" size="icon" class="size-7" :aria-label="`复制 ${box.text}`" @click="copy(box.text, String(index))"><Check v-if="copied === String(index)" class="size-3.5" /><Copy v-else class="size-3.5" /></Button>
                </div>
              </div>
              <div v-else class="py-12 text-center text-xs text-muted-foreground">{{ results.length ? '没有匹配的文字' : '未识别到文字' }}</div>
              <div class="mt-auto flex justify-end"><Button size="sm" variant="ghost" class="text-xs text-muted-foreground" @click="reset">清空结果</Button></div>
            </template>
            <div v-else class="flex min-h-40 flex-1 flex-col items-center justify-center gap-2 text-muted-foreground"><ScanText class="size-8 opacity-40" /><p class="text-xs">{{ busy ? '正在识别画面文字' : '识别结果将显示在这里' }}</p></div>
          </TabsContent>
          <TabsContent v-for="item in [{ id: 'page', text: '页面信息' }, { id: 'xml', text: '界面节点' }, { id: 'color', text: '画面取色' }, { id: 'log', text: '操作记录' }]" :key="item.id" :value="item.id" class="m-0 p-4 text-xs text-muted-foreground">{{ item.text }}</TabsContent>
        </Tabs>
      </aside>
    </div>
  </div>
</template>
