<script setup lang="ts">
import { computed, ref } from 'vue'
import {
  Check,
  Copy,
  Crop,
  Inspect,
  Loader2,
  Play,
  RefreshCw,
  ScanText,
  Square,
  Wrench,
} from 'lucide-vue-next'
import { Badge } from '@/components/ui/badge'
import { Button } from '@/components/ui/button'
import { Card, CardContent, CardDescription, CardHeader, CardTitle } from '@/components/ui/card'
import { Input } from '@/components/ui/input'
import { ScrollArea } from '@/components/ui/scroll-area'
import { Tabs, TabsContent, TabsList, TabsTrigger } from '@/components/ui/tabs'

// 主视角切换：投屏与检测交互 / OCR 系统服务配置
const activeView = ref<'player' | 'settings'>('player')

// ==================== 视图 1: 投屏与 OCR 交互原型 ====================
const playerMode = ref<'capture' | 'inspector'>('inspector')

// 模拟投屏上的文本与元素数据
interface MockScreenElement {
  id: string
  name: string
  text: string
  bounds: { x: number, y: number, w: number, h: number } // 百分比
  confidence: number
}

const screenElements: MockScreenElement[] = [
  { id: 'el-1', name: 'HeaderTitle', text: '德州竞技大厅', bounds: { x: 30, y: 5, w: 40, h: 5 }, confidence: 0.99 },
  { id: 'el-2', name: 'ChipBalance', text: '金币: 1,280,000', bounds: { x: 8, y: 13, w: 35, h: 4 }, confidence: 0.98 },
  { id: 'el-3', name: 'VipBadge', text: 'VIP 8 尊享', bounds: { x: 68, y: 13, w: 24, h: 4 }, confidence: 0.96 },
  { id: 'el-4', name: 'QuickStartBtn', text: '快速开始', bounds: { x: 18, y: 74, w: 64, h: 8 }, confidence: 0.97 },
  { id: 'el-5', name: 'ActivityCard', text: '每日签到领取奖励', bounds: { x: 12, y: 25, w: 76, h: 18 }, confidence: 0.94 },
  { id: 'el-6', name: 'RoomTab1', text: '新手场 (底注 100)', bounds: { x: 12, y: 48, w: 76, h: 9 }, confidence: 0.95 },
  { id: 'el-7', name: 'RoomTab2', text: '精英场 (底注 1000)', bounds: { x: 12, y: 59, w: 76, h: 9 }, confidence: 0.93 },
  { id: 'el-8', name: 'BottomNav', text: '首页   赛事   俱乐部   我的', bounds: { x: 6, y: 92, w: 88, h: 5 }, confidence: 0.99 },
]

// 检查模式状态
const selectedElementId = ref<string | null>(null)
const isOcrRunning = ref(false)
const ocrResults = ref<MockScreenElement[]>([])
const inspectorTab = ref<'ocr' | 'xml' | 'color'>('ocr')
const highlightedBoxId = ref<string | null>(null)

// 选中的元素
const selectedElement = computed(() => {
  return screenElements.find(e => e.id === selectedElementId.value) || null
})

// 执行检查页 OCR
function runInspectorOcr() {
  isOcrRunning.value = true
  highlightedBoxId.value = null
  setTimeout(() => {
    isOcrRunning.value = false
    if (selectedElement.value) {
      // 仅识别当前选中元素
      ocrResults.value = [selectedElement.value]
    }
    else {
      // 识别全屏所有元素
      ocrResults.value = [...screenElements]
    }
  }, 600)
}

function clearInspectorSelection() {
  selectedElementId.value = null
  ocrResults.value = []
  highlightedBoxId.value = null
}

// 区域截图模式状态
const captureSelection = ref<{ x: number, y: number, w: number, h: number }>({
  x: 10,
  y: 22,
  w: 80,
  h: 24,
})
const isCaptureOcrRunning = ref(false)
const captureOcrText = ref('')
const captureOcrBoxes = ref<MockScreenElement[]>([])
const captureResultVisible = ref(false)
const copied = ref(false)

function runCaptureOcr() {
  isCaptureOcrRunning.value = true
  captureResultVisible.value = true
  setTimeout(() => {
    isCaptureOcrRunning.value = false
    // 找出在选区范围内的文字
    const inside = screenElements.filter(e =>
      e.bounds.y >= captureSelection.value.y - 5
      && (e.bounds.y + e.bounds.h) <= (captureSelection.value.y + captureSelection.value.h + 5),
    )
    captureOcrBoxes.value = inside.length > 0 ? inside : [screenElements[4]]
    captureOcrText.value = captureOcrBoxes.value.map(i => i.text).join('\n')
  }, 500)
}

function copyText(text: string) {
  navigator.clipboard?.writeText(text)
  copied.value = true
  setTimeout(() => { copied.value = false }, 1500)
}

// ==================== 视图 2: 系统设置 > OCR 服务配置 ====================
const paddleInstalled = ref(true)
const paddleRunning = ref(true)
const paddlePort = ref(8031)
const paddlePid = ref(43192)
const paddleBusy = ref(false)

const macOcrUrl = ref('http://10.0.0.11:8000/upload')
const macOcrAuth = ref('admin:wepork_ocr_secret')
const paddleOcrUrl = ref('http://127.0.0.1:8031/upload')
const ocrEngine = ref<'auto' | 'macocr' | 'paddleocr'>('auto')

const testStatus = ref<{ mac?: 'success' | 'failed' | 'testing', paddle?: 'success' | 'failed' | 'testing' }>({})
const testMessage = ref<{ mac?: string, paddle?: string }>({})

function togglePaddleService() {
  paddleBusy.value = true
  setTimeout(() => {
    paddleBusy.value = false
    paddleRunning.value = !paddleRunning.value
    if (paddleRunning.value) {
      paddlePid.value = Math.floor(10000 + Math.random() * 80000)
    }
  }, 700)
}

function testMacOcr() {
  testStatus.value.mac = 'testing'
  setTimeout(() => {
    testStatus.value.mac = 'success'
    testMessage.value.mac = 'macOCR 响应正常 · 耗时 42ms'
  }, 600)
}

function testPaddleOcr() {
  testStatus.value.paddle = 'testing'
  setTimeout(() => {
    if (paddleRunning.value) {
      testStatus.value.paddle = 'success'
      testMessage.value.paddle = 'PaddleOCR 响应正常 · 耗时 98ms'
    }
    else {
      testStatus.value.paddle = 'failed'
      testMessage.value.paddle = '连接失败：服务未启动'
    }
  }, 500)
}
</script>

<template>
  <div class="mx-auto flex h-screen max-w-7xl flex-col bg-background text-foreground">
    <!-- 顶部功能视图切换栏 -->
    <header class="flex shrink-0 items-center justify-between border-b px-5 py-3 bg-card">
      <div class="flex items-center gap-3">
        <ScanText class="size-5 text-primary" />
        <h1 class="text-base font-semibold">MyWebScrcpy · OCR 视觉与服务设计原型</h1>
        <Badge variant="outline" class="text-xs">v1.23.1 规划</Badge>
      </div>
      <div class="flex items-center gap-2">
        <Button
          size="sm"
          :variant="activeView === 'player' ? 'default' : 'outline'"
          @click="activeView = 'player'"
        >
          <Inspect class="mr-1.5 size-4" />投屏与 OCR 交互
        </Button>
        <Button
          size="sm"
          :variant="activeView === 'settings' ? 'default' : 'outline'"
          @click="activeView = 'settings'"
        >
          <Wrench class="mr-1.5 size-4" />系统设置 · OCR 服务
        </Button>
      </div>
    </header>

    <!-- 主展示区 -->
    <div class="flex min-h-0 flex-1 overflow-hidden">
      <!-- ================= 视图 1: 投屏与 OCR 交互 ================= -->
      <div v-if="activeView === 'player'" class="flex h-full w-full overflow-hidden">
        <!-- 左侧模拟设备投屏区域 -->
        <div class="flex flex-1 flex-col items-center justify-center p-4 bg-muted/20">
          <div class="mb-3 flex items-center gap-2">
            <span class="text-xs text-muted-foreground">当前交互模式：</span>
            <Button
              size="sm"
              :variant="playerMode === 'inspector' ? 'default' : 'secondary'"
              class="h-7 text-xs"
              @click="playerMode = 'inspector'"
            >
              <Inspect class="mr-1 size-3.5" />检查面板 (Inspector)
            </Button>
            <Button
              size="sm"
              :variant="playerMode === 'capture' ? 'default' : 'secondary'"
              class="h-7 text-xs"
              @click="playerMode = 'capture'"
            >
              <Crop class="mr-1 size-3.5" />区域截图 (CaptureLayer)
            </Button>
          </div>

          <!-- 模拟手机屏幕 (9:19.5 比例) -->
          <div class="relative aspect-[9/18] h-[580px] rounded-2xl border-4 border-neutral-800 bg-neutral-950 shadow-2xl overflow-hidden select-none">
            <!-- 手机模拟画面 -->
            <div class="absolute inset-0 bg-gradient-to-b from-slate-900 via-indigo-950 to-slate-900 text-white p-3 font-sans">
              <!-- 顶部状态栏 -->
              <div class="flex justify-between text-[10px] text-white/70 mb-4 px-1">
                <span>12:30</span>
                <span>5G · 100%</span>
              </div>

              <!-- 画面内容元素模拟 -->
              <div
                v-for="el in screenElements"
                :key="el.id"
                class="absolute cursor-pointer rounded transition-all duration-150 flex items-center justify-center border text-[11px]"
                :class="[
                  selectedElementId === el.id
                    ? 'border-primary bg-primary/20 text-primary-foreground font-medium ring-2 ring-primary/60'
                    : 'border-white/10 bg-white/5 hover:bg-white/15',
                ]"
                :style="{
                  left: `${el.bounds.x}%`,
                  top: `${el.bounds.y}%`,
                  width: `${el.bounds.w}%`,
                  height: `${el.bounds.h}%`,
                }"
                @click="playerMode === 'inspector' && (selectedElementId = selectedElementId === el.id ? null : el.id)"
              >
                <span class="truncate px-1">{{ el.text }}</span>
              </div>
            </div>

            <!-- ================= 屏幕上的亮绿外框 Overlay (#00e676) ================= -->
            <div class="pointer-events-none absolute inset-0 z-30">
              <!-- 检查模式下的绿框展示 -->
              <template v-if="playerMode === 'inspector' && ocrResults.length > 0">
                <div
                  v-for="box in ocrResults"
                  :key="box.id"
                  class="absolute border-2 transition-all duration-200"
                  :class="[
                    highlightedBoxId === box.id
                      ? 'border-yellow-400 bg-yellow-400/20 ring-2 ring-yellow-300'
                      : 'border-[#00e676] bg-[#00e676]/15',
                  ]"
                  :style="{
                    left: `${box.bounds.x}%`,
                    top: `${box.bounds.y}%`,
                    width: `${box.bounds.w}%`,
                    height: `${box.bounds.h}%`,
                  }"
                >
                  <span class="absolute -top-4 left-0 text-[10px] bg-[#00e676] text-black font-mono font-bold px-1 rounded-sm leading-tight shadow">
                    {{ box.text }} {{ Math.round(box.confidence * 100) }}%
                  </span>
                </div>
              </template>

              <!-- 区域截图模式下的绿框展示 -->
              <template v-if="playerMode === 'capture' && captureOcrBoxes.length > 0">
                <div
                  v-for="box in captureOcrBoxes"
                  :key="box.id"
                  class="absolute border-2 border-[#00e676] bg-[#00e676]/20 transition-all"
                  :style="{
                    left: `${box.bounds.x}%`,
                    top: `${box.bounds.y}%`,
                    width: `${box.bounds.w}%`,
                    height: `${box.bounds.h}%`,
                  }"
                >
                  <span class="absolute -top-4 left-0 text-[10px] bg-[#00e676] text-black font-mono font-bold px-1 rounded-sm leading-tight">
                    OCR {{ Math.round(box.confidence * 100) }}%
                  </span>
                </div>
              </template>
            </div>

            <!-- ================= 模式 2: 区域截图遮罩与操作栏 ================= -->
            <div v-if="playerMode === 'capture'" class="absolute inset-0 z-20 pointer-events-auto">
              <!-- 选区遮罩 -->
              <div
                class="absolute border-2 border-yellow-400 bg-black/10 cursor-move"
                :style="{
                  left: `${captureSelection.x}%`,
                  top: `${captureSelection.y}%`,
                  width: `${captureSelection.w}%`,
                  height: `${captureSelection.h}%`,
                }"
              >
                <!-- 尺寸指示 -->
                <div class="absolute -top-5 left-1 text-[10px] text-yellow-300 font-mono">
                  选区: {{ captureSelection.w }}% × {{ captureSelection.h }}%
                </div>

                <!-- 选区浮动操作栏 (新增 OCR 识别按钮) -->
                <div class="absolute -bottom-10 left-1/2 -translate-x-1/2 flex items-center gap-1 rounded-md bg-neutral-900/90 border border-neutral-700 p-1 shadow-lg text-white">
                  <Button size="sm" variant="ghost" class="h-7 px-2 text-xs hover:bg-neutral-800" title="复制图片">
                    <Copy class="size-3.5 mr-1" />复制
                  </Button>
                  <div class="h-3.5 w-px bg-neutral-700" />
                  <!-- 重点新增的 OCR 识别按钮 -->
                  <Button
                    size="sm"
                    class="h-7 px-2 text-xs bg-emerald-600 hover:bg-emerald-500 text-white font-medium shadow"
                    :disabled="isCaptureOcrRunning"
                    @click="runCaptureOcr"
                  >
                    <Loader2 v-if="isCaptureOcrRunning" class="size-3.5 mr-1 animate-spin" />
                    <ScanText v-else class="size-3.5 mr-1" />
                    OCR 识别
                  </Button>
                  <div class="h-3.5 w-px bg-neutral-700" />
                  <Button size="sm" variant="ghost" class="h-7 px-2 text-xs hover:bg-neutral-800" title="存为模板">
                    存模板
                  </Button>
                </div>
              </div>
            </div>
          </div>
        </div>

        <!-- 右侧边栏交互区 -->
        <aside class="flex w-96 flex-col border-l bg-card">
          <!-- 检查面板模式侧边 -->
          <template v-if="playerMode === 'inspector'">
            <div class="border-b p-3">
              <div class="flex items-center justify-between mb-2">
                <span class="text-sm font-semibold flex items-center gap-1.5">
                  <Inspect class="size-4 text-primary" />
                  检查功能页面
                </span>
                <Badge variant="secondary" class="text-xs">
                  {{ selectedElement ? '选中元素模式' : '全屏模式' }}
                </Badge>
              </div>

              <!-- 状态与执行 OCR 检测主按钮 -->
              <div class="rounded-lg border p-2.5 bg-muted/40 space-y-2">
                <div class="text-xs flex justify-between">
                  <span class="text-muted-foreground">当前检测范围:</span>
                  <span class="font-medium">
                    {{ selectedElement ? `${selectedElement.name} (局部)` : '整个屏幕 (全屏)' }}
                  </span>
                </div>
                <div v-if="selectedElement" class="text-xs text-muted-foreground truncate">
                  坐标: [{{ selectedElement.bounds.x }}, {{ selectedElement.bounds.y }}, {{ selectedElement.bounds.w }}, {{ selectedElement.bounds.h }}]
                </div>

                <div class="flex gap-2 pt-1">
                  <Button
                    size="sm"
                    class="flex-1 bg-emerald-600 hover:bg-emerald-500 text-white font-medium"
                    :disabled="isOcrRunning"
                    @click="runInspectorOcr"
                  >
                    <Loader2 v-if="isOcrRunning" class="size-3.5 mr-1.5 animate-spin" />
                    <ScanText v-else class="size-3.5 mr-1.5" />
                    {{ selectedElement ? '获取选中元素 OCR' : '全屏 OCR 检测' }}
                  </Button>
                  <Button
                    v-if="selectedElement"
                    size="sm"
                    variant="outline"
                    @click="clearInspectorSelection"
                  >
                    取消选择
                  </Button>
                </div>
              </div>
            </div>

            <!-- Tab 栏：取色 / XML / OCR / 页面 -->
            <Tabs v-model="inspectorTab" class="flex min-h-0 flex-1 flex-col">
              <div class="px-3 pt-2">
                <TabsList class="grid w-full grid-cols-3">
                  <TabsTrigger value="ocr">OCR 结果</TabsTrigger>
                  <TabsTrigger value="xml">UI 元素树</TabsTrigger>
                  <TabsTrigger value="color">取色</TabsTrigger>
                </TabsList>
              </div>

              <!-- OCR 识别结果列表 -->
              <TabsContent value="ocr" class="flex-1 min-h-0 overflow-hidden flex flex-col p-3 m-0">
                <div class="flex items-center justify-between mb-2">
                  <span class="text-xs font-medium text-muted-foreground">
                    识别文本项 ({{ ocrResults.length }}) · 屏幕显示绿框
                  </span>
                  <Button
                    v-if="ocrResults.length > 0"
                    size="sm"
                    variant="ghost"
                    class="h-6 text-xs px-1.5"
                    @click="copyText(ocrResults.map(o => o.text).join('\n'))"
                  >
                    <Copy class="size-3 mr-1" />复制全部
                  </Button>
                </div>

                <ScrollArea v-if="ocrResults.length > 0" class="flex-1 rounded-md border">
                  <div class="p-2 space-y-1.5">
                    <div
                      v-for="item in ocrResults"
                      :key="item.id"
                      class="rounded border p-2 text-xs cursor-pointer transition-colors"
                      :class="[
                        highlightedBoxId === item.id
                          ? 'border-yellow-400 bg-yellow-500/10'
                          : 'border-border hover:bg-muted/60',
                      ]"
                      @mouseenter="highlightedBoxId = item.id"
                      @mouseleave="highlightedBoxId = null"
                    >
                      <div class="flex items-center justify-between">
                        <span class="font-medium text-foreground">{{ item.text }}</span>
                        <Badge variant="outline" class="text-[10px] font-mono text-emerald-500 border-emerald-500/30">
                          {{ Math.round(item.confidence * 100) }}%
                        </Badge>
                      </div>
                      <div class="mt-1 flex items-center justify-between text-[11px] text-muted-foreground font-mono">
                        <span>x: {{ item.bounds.x }}% y: {{ item.bounds.y }}%</span>
                        <Button
                          size="sm"
                          variant="ghost"
                          class="h-5 px-1 text-[11px]"
                          @click.stop="copyText(item.text)"
                        >
                          复制
                        </Button>
                      </div>
                    </div>
                  </div>
                </ScrollArea>

                <div v-else class="flex flex-1 flex-col items-center justify-center p-6 text-center border rounded-md border-dashed">
                  <ScanText class="size-8 text-muted-foreground/40 mb-2" />
                  <p class="text-xs text-muted-foreground">点击上方「{{ selectedElement ? '获取选中元素 OCR' : '全屏 OCR 检测' }}」</p>
                  <p class="text-[11px] text-muted-foreground/70 mt-1">识别文本将在屏幕上自动渲染绿色外框</p>
                </div>
              </TabsContent>

              <TabsContent value="xml" class="p-3 m-0 text-xs text-muted-foreground">
                <p>点击左侧画面中的元素可切换当前选中节点。</p>
              </TabsContent>
              <TabsContent value="color" class="p-3 m-0 text-xs text-muted-foreground">
                <p>移动鼠标取色演示。</p>
              </TabsContent>
            </Tabs>
          </template>

          <!-- 区域截图模式侧边（识别结果展示卡片） -->
          <template v-else>
            <div class="p-4 border-b">
              <h2 class="text-sm font-semibold flex items-center gap-1.5">
                <Crop class="size-4 text-primary" />
                区域截图 OCR 识别
              </h2>
              <p class="text-xs text-muted-foreground mt-1">
                在左侧画面拖动选区，点击操作条上的「OCR 识别」。
              </p>
            </div>

            <div class="flex-1 p-4 space-y-4 overflow-y-auto">
              <Card v-if="captureResultVisible">
                <CardHeader class="p-3 pb-2">
                  <div class="flex items-center justify-between">
                    <CardTitle class="text-xs font-semibold">选区识别结果</CardTitle>
                    <Badge variant="outline" class="text-[10px] text-emerald-500 border-emerald-500/30">
                      {{ isCaptureOcrRunning ? '识别中...' : '识别成功' }}
                    </Badge>
                  </div>
                  <CardDescription class="text-[11px]">
                    引擎: PaddleOCR · 屏幕绿框已叠加
                  </CardDescription>
                </CardHeader>
                <CardContent class="p-3 pt-0 space-y-2">
                  <div class="relative rounded border bg-muted/30 p-2 font-mono text-xs">
                    <pre class="whitespace-pre-wrap">{{ captureOcrText || (isCaptureOcrRunning ? '正在处理图像文字...' : '未识别到文字') }}</pre>
                  </div>
                  <div class="flex justify-end gap-2">
                    <Button
                      size="sm"
                      variant="outline"
                      class="h-7 text-xs"
                      :disabled="!captureOcrText"
                      @click="copyText(captureOcrText)"
                    >
                      <Check v-if="copied" class="size-3 mr-1 text-emerald-500" />
                      <Copy v-else class="size-3 mr-1" />
                      {{ copied ? '已复制' : '复制文本' }}
                    </Button>
                  </div>
                </CardContent>
              </Card>

              <div class="rounded-lg border p-3 text-xs space-y-1.5 bg-muted/20">
                <span class="font-medium">使用提示：</span>
                <p class="text-muted-foreground">1. 点击选区工具条上的「OCR 识别」仅对截取的矩形局部画面调用 OCR。</p>
                <p class="text-muted-foreground">2. 识别出的文字区域自动通过 WebSocket 在原屏幕位置叠加亮绿外框展示。</p>
              </div>
            </div>
          </template>
        </aside>
      </div>

      <!-- ================= 视图 2: 系统设置 > OCR 服务配置 ================= -->
      <div v-else class="flex flex-1 flex-col items-center overflow-y-auto p-6 bg-muted/10">
        <div class="w-full max-w-3xl space-y-6">
          <div>
            <h2 class="text-lg font-semibold">OCR 服务管理与配置</h2>
            <p class="text-xs text-muted-foreground mt-0.5">
              配置系统 OCR 服务引擎、本机 paddleocr-server 运行时与远程 macOCR 服务地址。
            </p>
          </div>

          <!-- 卡片 1: packages/paddleocr-server 本地服务管理 -->
          <Card>
            <CardHeader class="pb-3">
              <div class="flex items-center justify-between">
                <div>
                  <CardTitle class="text-sm font-semibold flex items-center gap-2">
                    PaddleOCR 本地服务
                    <Badge :variant="paddleRunning ? 'default' : 'secondary'" class="text-xs">
                      {{ paddleRunning ? `运行中 (PID: ${paddlePid})` : '已停止' }}
                    </Badge>
                  </CardTitle>
                  <CardDescription class="text-xs mt-1">
                    位于项目目录 packages/paddleocr-server，提供本地高速 PP-OCRv5/v6 文字识别。
                  </CardDescription>
                </div>
                <!-- 启动 / 停止控制按钮 -->
                <div class="flex items-center gap-2">
                  <Button
                    size="sm"
                    :variant="paddleRunning ? 'destructive' : 'default'"
                    :disabled="paddleBusy || !paddleInstalled"
                    @click="togglePaddleService"
                  >
                    <Loader2 v-if="paddleBusy" class="size-3.5 mr-1.5 animate-spin" />
                    <Square v-else-if="paddleRunning" class="size-3.5 mr-1.5 fill-current" />
                    <Play v-else class="size-3.5 mr-1.5 fill-current" />
                    {{ paddleRunning ? '停止服务' : '启动服务' }}
                  </Button>
                </div>
              </div>
            </CardHeader>
            <CardContent class="space-y-4 pt-1">
              <!-- 本机 PADDLEOCR_URL 地址显示 -->
              <div class="rounded-lg border bg-muted/40 p-3 flex items-center justify-between">
                <div>
                  <div class="text-xs text-muted-foreground">本机服务地址 (PADDLEOCR_URL)：</div>
                  <div class="font-mono text-sm font-semibold text-foreground mt-0.5">
                    http://127.0.0.1:{{ paddlePort }}/upload
                  </div>
                </div>
                <Button
                  size="sm"
                  variant="outline"
                  class="h-8 text-xs"
                  @click="copyText(`http://127.0.0.1:${paddlePort}/upload`)"
                >
                  <Copy class="size-3.5 mr-1" />复制地址
                </Button>
              </div>

              <!-- 依赖与环境状态 -->
              <div class="grid grid-cols-2 gap-2 text-xs">
                <div class="flex items-center justify-between rounded border p-2">
                  <span class="text-muted-foreground">Python 运行时</span>
                  <span class="font-medium">Python 3.12 (venv)</span>
                </div>
                <div class="flex items-center justify-between rounded border p-2">
                  <span class="text-muted-foreground">paddleocr 模块</span>
                  <Badge variant="outline" class="text-[10px] text-emerald-500 border-emerald-500/30">已安装 (v2.9)</Badge>
                </div>
                <div class="flex items-center justify-between rounded border p-2">
                  <span class="text-muted-foreground">服务端口</span>
                  <span class="font-mono">{{ paddlePort }}</span>
                </div>
                <div class="flex items-center justify-between rounded border p-2">
                  <span class="text-muted-foreground">连通性测试</span>
                  <Button
                    size="sm"
                    variant="link"
                    class="h-auto p-0 text-xs"
                    :disabled="testStatus.paddle === 'testing'"
                    @click="testPaddleOcr"
                  >
                    <RefreshCw v-if="testStatus.paddle === 'testing'" class="size-3 mr-1 animate-spin" />
                    测试连通
                  </Button>
                </div>
              </div>
              <p v-if="testMessage.paddle" class="text-xs" :class="testStatus.paddle === 'success' ? 'text-emerald-500' : 'text-destructive'">
                {{ testMessage.paddle }}
              </p>
            </CardContent>
          </Card>

          <!-- 卡片 2: OCR 全局引擎与 URL 地址配置 -->
          <Card>
            <CardHeader class="pb-3">
              <CardTitle class="text-sm font-semibold">OCR 引擎与远程服务配置</CardTitle>
              <CardDescription class="text-xs">
                可配置全局优先调用的 OCR 引擎类型，及远程 macOCR / 本地 PaddleOCR 地址。
              </CardDescription>
            </CardHeader>
            <CardContent class="space-y-4">
              <!-- 引擎选择 -->
              <div class="space-y-1.5">
                <label class="text-xs font-medium">默认 OCR 引擎模式</label>
                <div class="grid grid-cols-3 gap-2">
                  <Button
                    size="sm"
                    :variant="ocrEngine === 'auto' ? 'default' : 'outline'"
                    class="text-xs"
                    @click="ocrEngine = 'auto'"
                  >
                    自动降级 (auto)
                  </Button>
                  <Button
                    size="sm"
                    :variant="ocrEngine === 'macocr' ? 'default' : 'outline'"
                    class="text-xs"
                    @click="ocrEngine = 'macocr'"
                  >
                    优先 macOCR
                  </Button>
                  <Button
                    size="sm"
                    :variant="ocrEngine === 'paddleocr' ? 'default' : 'outline'"
                    class="text-xs"
                    @click="ocrEngine = 'paddleocr'"
                  >
                    优先 PaddleOCR
                  </Button>
                </div>
                <p class="text-[11px] text-muted-foreground">
                  自动模式：优先请求远程 macOCR，若不可用或报错自动平滑降级至本地 paddleocr-server。
                </p>
              </div>

              <!-- MACOCR_URL 配置 -->
              <div class="space-y-1.5 pt-2 border-t">
                <div class="flex items-center justify-between">
                  <label class="text-xs font-medium">MACOCR_URL</label>
                  <Button
                    size="sm"
                    variant="link"
                    class="h-auto p-0 text-xs"
                    :disabled="testStatus.mac === 'testing'"
                    @click="testMacOcr"
                  >
                    <RefreshCw v-if="testStatus.mac === 'testing'" class="size-3 mr-1 animate-spin" />
                    测试连接
                  </Button>
                </div>
                <Input v-model="macOcrUrl" placeholder="http://10.0.0.11:8000/upload" class="font-mono text-xs" />
                <p v-if="testMessage.mac" class="text-xs text-emerald-500">
                  {{ testMessage.mac }}
                </p>
              </div>

              <!-- MACOCR_AUTH 配置 -->
              <div class="space-y-1.5">
                <label class="text-xs font-medium">MACOCR_AUTH (可选认证凭据)</label>
                <Input v-model="macOcrAuth" type="password" placeholder="用户名:密码 或 Bearer Token" class="font-mono text-xs" />
              </div>

              <!-- PADDLEOCR_URL 覆盖配置 -->
              <div class="space-y-1.5 pt-2 border-t">
                <label class="text-xs font-medium">PADDLEOCR_URL</label>
                <Input v-model="paddleOcrUrl" placeholder="http://127.0.0.1:8031/upload" class="font-mono text-xs" />
                <p class="text-[11px] text-muted-foreground">
                  默认使用本机 paddleocr-server 启动地址；若局域网有独立 GPU 机器亦可填入远程地址。
                </p>
              </div>
            </CardContent>
          </Card>
        </div>
      </div>
    </div>
  </div>
</template>
