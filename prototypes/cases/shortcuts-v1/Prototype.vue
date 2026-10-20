<script setup lang="ts">
import { onMounted, onUnmounted, ref } from 'vue'
import {
  Eye,
  EyeOff,
  GripVertical,
  Home,
  Keyboard,
  Maximize2,
  Minimize2,
  RotateCcw,
  Settings2,
  Shield,
  Smartphone,
  Sliders,
  Undo2,
  Usb,
  VolumeX,
  Wifi,
  X,
  FileSpreadsheet,
} from 'lucide-vue-next'
import { Badge } from '@/components/ui/badge'
import { Button } from '@/components/ui/button'
import { Card, CardContent } from '@/components/ui/card'
import { Dialog, DialogContent, DialogHeader, DialogTitle } from '@/components/ui/dialog'
import { Tabs, TabsContent, TabsList, TabsTrigger } from '@/components/ui/tabs'
import ShortcutSettingsPanel from './ShortcutSettingsPanel.vue'
import { useShortcuts } from './useShortcuts'

interface SimulatedDevice {
  id: string
  name: string
  serial: string
  ip: boolean
  battery: number
  status: '投屏中' | '在线' | '离线'
}

const shortcutState = useShortcuts()
const configOpen = ref(window.location.search.includes('dialog=1') || window.location.hash.includes('dialog'))
const activeTab = ref('shortcuts')

// 演练区交互状态
const bossKeyActive = ref(false)
const floatingWindowsVisible = ref(true)
const toolbarVisible = ref(true)
const inspectorVisible = ref(false)
const deviceVolumeMuted = ref(false)
const lastActionLog = ref<string>('系统快捷键服务已就绪，支持拖动排序与 Option+1~4 快速投屏')

// 模拟小窗位置与参数
const floatingWindowOpacity = ref(100)
const simulatedDevices = ref<SimulatedDevice[]>([
  { id: 'dev-1', name: 'Pixel 9 Pro', serial: '192.168.1.101:5555', ip: true, battery: 88, status: '投屏中' },
  { id: 'dev-2', name: 'Galaxy S24 Ultra', serial: 'R5CW123XYZ', ip: false, battery: 64, status: '在线' },
  { id: 'dev-3', name: 'Xiaomi 14 Pro', serial: '192.168.1.108:5555', ip: true, battery: 92, status: '在线' },
  { id: 'dev-4', name: 'OnePlus 12', serial: 'OP12984ABC', ip: false, battery: 45, status: '在线' },
])

const activeDevice = ref<SimulatedDevice>(simulatedDevices.value[0])

// 拖拽排序逻辑
const draggedIndex = ref<number | null>(null)
const dragOverIndex = ref<number | null>(null)

function onDragStart(e: DragEvent, index: number) {
  draggedIndex.value = index
  if (e.dataTransfer) {
    e.dataTransfer.effectAllowed = 'move'
    e.dataTransfer.setData('text/plain', String(index))
  }
}

function onDragOver(e: DragEvent, index: number) {
  e.preventDefault()
  if (draggedIndex.value === null || draggedIndex.value === index) return
  dragOverIndex.value = index
}

function onDrop(e: DragEvent, targetIndex: number) {
  e.preventDefault()
  if (draggedIndex.value === null || draggedIndex.value === targetIndex) {
    draggedIndex.value = null
    dragOverIndex.value = null
    return
  }

  const fromIdx = draggedIndex.value
  const [moved] = simulatedDevices.value.splice(fromIdx, 1)
  simulatedDevices.value.splice(targetIndex, 0, moved)
  draggedIndex.value = null
  dragOverIndex.value = null

  const prefix = shortcutState.currentOs.value === 'mac' ? 'Option+' : 'Alt+'
  lastActionLog.value = `[拖动排序] 已将「${moved.name}」移至第 ${targetIndex + 1} 位，快捷键自动绑定为 ${prefix}${targetIndex + 1}`
}

function onDragEnd() {
  draggedIndex.value = null
  dragOverIndex.value = null
}

function openDeviceByIndex(index: number) {
  if (index < 0 || index >= simulatedDevices.value.length) return
  const target = simulatedDevices.value[index]
  activeDevice.value = target
  floatingWindowsVisible.value = true
  bossKeyActive.value = false
  simulatedDevices.value.forEach((d, i) => {
    d.status = (i === index) ? '投屏中' : '在线'
  })
  const prefix = shortcutState.currentOs.value === 'mac' ? '⌥' : 'Alt+'
  lastActionLog.value = `[快速投屏 ${prefix} ${index + 1}] 已唤出第 ${index + 1} 台设备「${target.name}」投屏小窗`
}

function triggerBossKey() {
  bossKeyActive.value = !bossKeyActive.value
  if (bossKeyActive.value) {
    if (shortcutState.bossBehavior.value.muteAudio) {
      deviceVolumeMuted.value = true
    }
    lastActionLog.value = `[老板键] 隐匿模式已激活：所有投屏画面已安全隔离，音频已静音`
  } else {
    deviceVolumeMuted.value = false
    lastActionLog.value = `[老板键] 隐匿模式已退出：恢复正常工作工作区与投屏小窗`
  }
}

function toggleFloatingWindows() {
  floatingWindowsVisible.value = !floatingWindowsVisible.value
  lastActionLog.value = floatingWindowsVisible.value
    ? '[小窗总开关] 已展开恢复所有设备画中画小窗'
    : '[小窗总开关] 已隐藏所有画中画小窗'
}

function toggleToolbar() {
  toolbarVisible.value = !toolbarVisible.value
  lastActionLog.value = toolbarVisible.value
    ? '[小窗顶栏] 控制条已展开显示'
    : '[小窗顶栏] 控制条已收起为极简小横条'
}

function toggleInspector() {
  inspectorVisible.value = !inspectorVisible.value
  lastActionLog.value = inspectorVisible.value
    ? '[检查器] 已呼出取色坐标与辅助参考线侧栏'
    : '[检查器] 已收起辅助检查抽屉'
}

function restoreAll() {
  floatingWindowsVisible.value = true
  toolbarVisible.value = true
  floatingWindowOpacity.value = 100
  bossKeyActive.value = false
  deviceVolumeMuted.value = false
  lastActionLog.value = '[恢复小窗] 所有投屏小窗与控制状态已重置为正常状态'
}

// 监听键盘按键，进行真实的页面内快捷键拦截响应
function handleKeyDown(e: KeyboardEvent) {
  // 如果正在按键录制，不触发动作
  if (shortcutState.recordingId.value) return

  // 如果在输入框中，忽略
  const tag = (e.target as HTMLElement)?.tagName?.toLowerCase()
  if (tag === 'input' || tag === 'textarea') return

  // 1. Option/Alt + 1~9: 快速打开第 N 台设备
  // 使用 e.code 避免 macOS 下 Option + 数字产生特殊字符 (¡, ™, £...)
  const digitMatch = e.code.match(/^Digit([1-9])$/)
  if (e.altKey && digitMatch) {
    e.preventDefault()
    const num = parseInt(digitMatch[1], 10)
    openDeviceByIndex(num - 1)
    return
  }

  // 2. Alt/Option + H -> 老板键
  if (e.altKey && (e.key === 'h' || e.key === 'H' || e.code === 'KeyH')) {
    e.preventDefault()
    triggerBossKey()
  }
  // 3. Alt/Option + W -> 投屏小窗总开关
  else if (e.altKey && (e.key === 'w' || e.key === 'W' || e.code === 'KeyW')) {
    e.preventDefault()
    toggleFloatingWindows()
  }
  // 4. Alt/Option + T -> 小窗控制条
  else if (e.altKey && (e.key === 't' || e.key === 'T' || e.code === 'KeyT')) {
    e.preventDefault()
    toggleToolbar()
  }
  // 5. Alt/Option + I -> 辅助检查器
  else if (e.altKey && (e.key === 'i' || e.key === 'I' || e.code === 'KeyI')) {
    e.preventDefault()
    toggleInspector()
  }
  // 6. Alt/Option + P -> 恢复小窗 (仅桌面模式)
  else if (e.altKey && (e.key === 'p' || e.key === 'P' || e.code === 'KeyP')) {
    e.preventDefault()
    restoreAll()
  }
}

onMounted(() => {
  window.addEventListener('keydown', handleKeyDown)
})

onUnmounted(() => {
  window.removeEventListener('keydown', handleKeyDown)
})
</script>

<template>
  <div class="relative min-h-[85vh] w-full overflow-hidden rounded-xl border bg-background text-foreground shadow-sm">
    <!-- 顶部导航栏 -->
    <header class="flex h-14 items-center justify-between border-b px-4 bg-card/60 backdrop-blur-sm">
      <div class="flex items-center gap-3">
        <Smartphone class="size-6 text-primary" />
        <div class="flex items-center gap-2">
          <span class="font-semibold tracking-tight">MyWebScrcpy</span>
          <Badge variant="outline" class="font-mono text-[10px]">快捷键与设备排序 v1.1</Badge>
        </div>
      </div>

      <div class="flex items-center gap-2">
        <Button
          variant="default"
          size="sm"
          class="gap-1.5 shadow-xs"
          @click="configOpen = true; activeTab = 'shortcuts'"
        >
          <Settings2 class="size-4" />
          <span>系统设置 · 快捷键</span>
          <Badge variant="secondary" class="ml-1 h-4.5 px-1.5 text-[10px]">独立配置块</Badge>
        </Button>
      </div>
    </header>

    <!-- 快捷键演练与交互体验横幅 -->
    <div class="border-b bg-muted/20 px-4 py-2.5">
      <div class="flex flex-wrap items-center justify-between gap-3">
        <div class="flex flex-wrap items-center gap-2">
          <Badge variant="secondary" class="gap-1 px-2 py-0.5 text-xs font-normal">
            <Keyboard class="size-3.5 text-primary" />
            <span>演练区快捷键：</span>
          </Badge>

          <!-- 序号打开设备按键组 -->
          <div class="flex items-center gap-1 rounded-md border bg-background/80 px-1.5 py-0.5">
            <span class="text-[10px] text-muted-foreground mr-1">按序号打开:</span>
            <Button
              v-for="(dev, idx) in simulatedDevices"
              :key="dev.id"
              size="xs"
              :variant="activeDevice.id === dev.id ? 'default' : 'ghost'"
              class="h-6 px-1.5 text-xs gap-1 font-mono"
              @click="openDeviceByIndex(idx)"
            >
              <span>{{ shortcutState.currentOs.value === 'mac' ? '⌥' : 'Alt+' }}{{ idx + 1 }}</span>
              <span class="text-[10px] opacity-75 truncate max-w-[60px]">{{ dev.name.split(' ')[0] }}</span>
            </Button>
          </div>

          <div class="flex flex-wrap items-center gap-1.5">
            <Button
              size="xs"
              :variant="bossKeyActive ? 'destructive' : 'outline'"
              class="h-7 text-xs gap-1 font-mono"
              @click="triggerBossKey"
            >
              <Shield class="size-3" />
              <span>老板键 {{ shortcutState.currentOs.value === 'mac' ? '⌥ H' : 'Alt+H' }}</span>
              <span class="text-[10px] opacity-70">({{ bossKeyActive ? '恢复' : '隐匿' }})</span>
            </Button>

            <Button
              size="xs"
              :variant="!floatingWindowsVisible ? 'secondary' : 'outline'"
              class="h-7 text-xs gap-1 font-mono"
              @click="toggleFloatingWindows"
            >
              <EyeOff v-if="floatingWindowsVisible" class="size-3" />
              <Eye v-else class="size-3" />
              <span>小窗总开关 {{ shortcutState.currentOs.value === 'mac' ? '⌥ W' : 'Alt+W' }}</span>
            </Button>

            <Button
              size="xs"
              variant="outline"
              class="h-7 text-xs gap-1 font-mono"
              @click="toggleToolbar"
            >
              <Sliders class="size-3" />
              <span>小窗控制栏 {{ shortcutState.currentOs.value === 'mac' ? '⌥ T' : 'Alt+T' }}</span>
            </Button>

            <Button
              size="xs"
              variant="outline"
              class="h-7 text-xs gap-1 font-mono"
              @click="toggleInspector"
            >
              <span>检查器 {{ shortcutState.currentOs.value === 'mac' ? '⌥ I' : 'Alt+I' }}</span>
            </Button>

            <Button
              size="xs"
              variant="ghost"
              class="h-7 text-xs text-muted-foreground gap-1"
              @click="restoreAll"
            >
              <RotateCcw class="size-3" />
              <span>恢复所有</span>
            </Button>
          </div>
        </div>

        <!-- 触发动作反馈条 -->
        <div class="flex items-center gap-2 text-xs text-muted-foreground">
          <span class="inline-block size-2 rounded-full bg-emerald-500 animate-pulse" />
          <span class="max-w-[340px] truncate">{{ lastActionLog }}</span>
        </div>
      </div>
    </div>

    <!-- 主展示工作区 -->
    <main class="relative p-6 space-y-6">
      <!-- 老板键激活时的“安全伪装”展示覆盖层 -->
      <transition
        enter-active-class="transition-opacity duration-200 ease-out"
        enter-from-class="opacity-0"
        enter-to-class="opacity-100"
        leave-active-class="transition-opacity duration-150 ease-in"
        leave-from-class="opacity-100"
        leave-to-class="opacity-0"
      >
        <div
          v-if="bossKeyActive"
          class="absolute inset-0 z-40 flex flex-col items-center justify-center bg-background/95 backdrop-blur-md p-8"
        >
          <div class="w-full max-w-2xl space-y-4 rounded-xl border bg-card p-6 shadow-xl text-left">
            <div class="flex items-center justify-between border-b pb-4">
              <div class="flex items-center gap-3">
                <div class="flex size-10 items-center justify-center rounded-lg bg-emerald-500/10 text-emerald-600">
                  <FileSpreadsheet class="size-6" />
                </div>
                <div>
                  <h4 class="text-base font-semibold">Q4 运营数据统计与复盘报表.xlsx</h4>
                  <p class="text-xs text-muted-foreground">最后自动保存于 19:42:05 · 云端协同已同步</p>
                </div>
              </div>
              <Badge variant="outline" class="gap-1 text-emerald-600 border-emerald-500/30">
                <Shield class="size-3" />
                老板键隐匿模式中
              </Badge>
            </div>

            <div class="space-y-2 py-2">
              <div class="grid grid-cols-4 gap-2 text-xs text-muted-foreground border-b pb-2 font-medium">
                <span>指标维度</span>
                <span>目标值</span>
                <span>实际达成</span>
                <span>环比增长</span>
              </div>
              <div class="grid grid-cols-4 gap-2 text-xs py-1">
                <span>每日活跃并发</span>
                <span>120,000</span>
                <span>135,420</span>
                <span class="text-emerald-600">+12.8%</span>
              </div>
              <div class="grid grid-cols-4 gap-2 text-xs py-1">
                <span>视觉匹配吞吐</span>
                <span>850 ops/s</span>
                <span>1,024 ops/s</span>
                <span class="text-emerald-600">+20.4%</span>
              </div>
              <div class="grid grid-cols-4 gap-2 text-xs py-1">
                <span>端到端投屏延迟</span>
                <span>&lt; 35ms</span>
                <span>28.4ms</span>
                <span class="text-emerald-600">-18.8%</span>
              </div>
            </div>

            <div class="flex items-center justify-between pt-3 border-t text-xs text-muted-foreground">
              <span>投屏与设备音频已完全隔离静音</span>
              <Button size="sm" variant="default" @click="triggerBossKey">
                退出隐匿状态 (Alt+H)
              </Button>
            </div>
          </div>
        </div>
      </transition>

      <!-- 演练与展示工作区：左右双栏结构 -->
      <div class="grid gap-6 lg:grid-cols-3">
        <!-- 左侧 2 列：在线设备列表与拖动排序 -->
        <div class="lg:col-span-2 space-y-4">
          <div class="flex items-center justify-between">
            <div class="flex items-center gap-2">
              <h2 class="text-base font-semibold">在线设备</h2>
              <Badge variant="secondary">{{ simulatedDevices.length }} 台可用</Badge>
              <span class="text-xs text-muted-foreground ml-2 hidden sm:inline">
                💡 按住把手拖动排序，卡片序号与 Option+1~{{ simulatedDevices.length }} 实时联动
              </span>
            </div>
            <div class="flex items-center gap-2">
              <Button
                variant="outline"
                size="sm"
                class="gap-1.5"
                @click="toggleFloatingWindows"
              >
                <EyeOff v-if="floatingWindowsVisible" class="size-3.5" />
                <Eye v-else class="size-3.5" />
                <span>{{ floatingWindowsVisible ? '隐藏投屏小窗 (Alt+W)' : '唤出投屏小窗 (Alt+W)' }}</span>
              </Button>
            </div>
          </div>

          <!-- 设备卡片网格（支持拖拽排序） -->
          <div class="grid gap-3 sm:grid-cols-2">
            <Card
              v-for="(d, index) in simulatedDevices"
              :key="d.id"
              draggable="true"
              class="relative overflow-hidden transition-all duration-150 cursor-pointer select-none"
              :class="[
                d.id === activeDevice.id ? 'border-primary ring-1 ring-primary/40 bg-primary/[0.02]' : 'hover:border-primary/50',
                draggedIndex === index ? 'opacity-40 scale-[0.98]' : '',
                dragOverIndex === index ? 'border-primary ring-2 ring-primary/40' : '',
              ]"
              @dragstart="onDragStart($event, index)"
              @dragover="onDragOver($event, index)"
              @drop="onDrop($event, index)"
              @dragend="onDragEnd"
              @click="openDeviceByIndex(index)"
            >
              <CardContent class="p-4 space-y-3">
                <div class="flex items-start justify-between gap-2">
                  <div class="flex items-center gap-3">
                    <!-- 拖动手柄 -->
                    <div
                      class="flex size-7 items-center justify-center rounded text-muted-foreground/50 hover:bg-muted hover:text-foreground cursor-grab active:cursor-grabbing"
                      title="按住拖动以调整设备次序"
                    >
                      <GripVertical class="size-4" />
                    </div>
                    <div class="flex size-9 items-center justify-center rounded-lg bg-primary/10 text-primary">
                      <Smartphone class="size-5" />
                    </div>
                    <div>
                      <h3 class="font-medium text-sm flex items-center gap-2">
                        <span>{{ d.name }}</span>
                        <Badge v-if="d.id === activeDevice.id" variant="default" class="h-4.5 px-1.5 text-[10px]">投屏中</Badge>
                      </h3>
                      <p class="font-mono text-xs text-muted-foreground flex items-center gap-1 mt-0.5">
                        <Wifi v-if="d.ip" class="size-3" />
                        <Usb v-else class="size-3" />
                        {{ d.serial }}
                      </p>
                    </div>
                  </div>

                  <!-- 快捷键徽标：Option + 1, Option + 2... -->
                  <Badge
                    variant="outline"
                    class="font-mono text-xs font-semibold px-2 py-0.5 gap-1 border-primary/40 bg-primary/5 text-primary shrink-0"
                    title="按下此快捷键可直接呼出投屏"
                  >
                    <span class="text-[10px] text-muted-foreground">快捷键</span>
                    <span>{{ shortcutState.currentOs.value === 'mac' ? '⌥' : 'Alt+' }}{{ index + 1 }}</span>
                  </Badge>
                </div>

                <div class="flex items-center justify-between pt-2 border-t text-xs text-muted-foreground">
                  <span>电量: {{ d.battery }}%</span>
                  <span class="text-primary font-medium hover:underline">
                    {{ d.id === activeDevice.id ? '正在右侧投屏' : '点击切换投屏' }}
                  </span>
                </div>
              </CardContent>
            </Card>
          </div>
        </div>

        <!-- 右侧 1 列：画中画投屏小窗交互仿真 -->
        <div class="lg:col-span-1">
          <div class="flex items-center justify-between pb-2">
            <span class="text-xs font-medium text-muted-foreground">画中画实时仿真小窗：</span>
            <Badge v-if="!floatingWindowsVisible" variant="outline" class="text-[10px] text-destructive">
              已隐藏 (按 Alt+W 唤出)
            </Badge>
            <Badge v-else variant="outline" class="text-[10px] text-emerald-600">
              投屏活跃 · {{ activeDevice.name }}
            </Badge>
          </div>

          <transition
            enter-active-class="transition duration-200 ease-out transform"
            enter-from-class="scale-95 opacity-0 translate-y-2"
            enter-to-class="scale-100 opacity-100 translate-y-0"
            leave-active-class="transition duration-150 ease-in transform"
            leave-from-class="scale-100 opacity-100 translate-y-0"
            leave-to-class="scale-95 opacity-0 translate-y-2"
          >
            <div
              v-if="floatingWindowsVisible"
              class="flex flex-col w-full max-w-[300px] mx-auto rounded-xl border-2 border-border/80 bg-background shadow-xl overflow-hidden transition-all duration-300"
              :style="{ opacity: floatingWindowOpacity / 100 }"
            >
              <!-- 小窗顶部控制条 (支持 Alt+T 显示与隐藏) -->
              <div
                v-if="toolbarVisible"
                class="flex h-10 items-center justify-between border-b bg-muted/60 px-2 text-xs transition-all"
              >
                <div class="flex items-center gap-1.5 truncate">
                  <span class="size-2 rounded-full bg-emerald-500 inline-block" />
                  <span class="font-medium truncate">{{ activeDevice.name }}</span>
                  <Badge variant="outline" class="font-mono text-[9px] px-1 py-0 ml-0.5 border-primary/30 text-primary">
                    {{ shortcutState.currentOs.value === 'mac' ? '⌥' : 'Alt+' }}{{ simulatedDevices.findIndex(d => d.id === activeDevice.id) + 1 }}
                  </Badge>
                  <VolumeX v-if="deviceVolumeMuted" class="size-3 text-destructive" />
                </div>

                <div class="flex items-center gap-1">
                  <button
                    type="button"
                    title="辅助检查器 (Alt+I)"
                    class="rounded p-1 hover:bg-muted text-muted-foreground hover:text-foreground"
                    @click="toggleInspector"
                  >
                    <Sliders class="size-3.5" />
                  </button>
                  <button
                    type="button"
                    title="折叠为小横条 (Alt+T)"
                    class="rounded p-1 hover:bg-muted text-muted-foreground hover:text-foreground"
                    @click="toolbarVisible = false"
                  >
                    <Minimize2 class="size-3.5" />
                  </button>
                  <button
                    type="button"
                    title="隐藏小窗 (Alt+W)"
                    class="rounded p-1 hover:bg-muted text-muted-foreground hover:text-foreground"
                    @click="floatingWindowsVisible = false"
                  >
                    <X class="size-3.5" />
                  </button>
                </div>
              </div>

              <!-- 当控制条折叠时显示的小把手横条 -->
              <div
                v-else
                class="group flex h-3 w-full cursor-pointer items-center justify-center bg-muted/90 hover:bg-primary/20 transition-colors"
                title="点击展开控制栏 (快捷键 Alt+T)"
                @click="toolbarVisible = true"
              >
                <div class="h-1 w-12 rounded-full bg-muted-foreground/40 group-hover:bg-primary" />
              </div>

              <!-- 手机投屏画面仿真 -->
              <div class="relative aspect-[9/18] w-full bg-gradient-to-br from-slate-900 via-indigo-950 to-neutral-900 text-white flex flex-col justify-between p-4 overflow-hidden select-none">
                <!-- 手机状态栏 -->
                <div class="flex items-center justify-between text-[11px] text-white/80">
                  <span class="font-mono">19:43</span>
                  <div class="flex items-center gap-1.5 font-mono text-[10px]">
                    <span>5G</span>
                    <span>{{ activeDevice.battery }}%</span>
                  </div>
                </div>

                <!-- 手机锁屏/主屏模拟 -->
                <div class="flex flex-col items-center gap-2 my-auto">
                  <span class="text-3xl font-extralight tracking-tight">19:43</span>
                  <span class="text-xs text-white/70">{{ activeDevice.name }}</span>
                  <div class="mt-4 grid grid-cols-3 gap-3">
                    <div v-for="i in 6" :key="i" class="size-9 rounded-xl bg-white/10 backdrop-blur-xs flex items-center justify-center">
                      <div class="size-4 rounded-md bg-white/20" />
                    </div>
                  </div>
                </div>

                <!-- 辅助检查器/参考线浮层 (支持 Alt+I 切换) -->
                <div
                  v-if="inspectorVisible"
                  class="absolute inset-0 bg-blue-900/60 backdrop-blur-xs p-3 text-[11px] space-y-2 border-2 border-dashed border-blue-400"
                >
                  <div class="flex items-center justify-between border-b border-white/20 pb-1">
                    <span class="font-semibold text-blue-200">辅助检查参考抽屉 (Alt+I)</span>
                    <X class="size-3.5 cursor-pointer text-white/80" @click="inspectorVisible = false" />
                  </div>
                  <div class="space-y-1 font-mono text-[10px] text-white/90">
                    <p>设备: {{ activeDevice.serial }}</p>
                    <p>坐标: X: 480, Y: 1024</p>
                    <p>取色: #2D3748 (RGBA: 45,55,72,1)</p>
                  </div>
                </div>

                <!-- 底部虚拟导航栏 -->
                <div class="flex items-center justify-around border-t border-white/10 pt-2 text-white/70">
                  <button type="button" class="p-1 hover:text-white" title="返回 (Alt+Backspace)">
                    <Undo2 class="size-4" />
                  </button>
                  <button type="button" class="p-1 hover:text-white" title="主屏 (Alt+Home)">
                    <Home class="size-4" />
                  </button>
                  <button type="button" class="p-1 hover:text-white" title="任务 (Alt+M)">
                    <div class="size-3.5 rounded-sm border-2 border-current" />
                  </button>
                </div>
              </div>
            </div>

            <!-- 小窗隐藏状态下的占位插画 -->
            <div
              v-else
              class="flex flex-col items-center justify-center h-[360px] rounded-xl border border-dashed border-muted p-6 text-center space-y-2 text-muted-foreground"
            >
              <EyeOff class="size-8 text-muted-foreground/60" />
              <p class="text-xs font-medium">投屏小窗当前已隐藏</p>
              <p class="text-[11px]">按 Alt + W 快捷键或点击上方按钮唤出</p>
              <Button size="xs" variant="outline" @click="floatingWindowsVisible = true">
                恢复显示小窗
              </Button>
            </div>
          </transition>
        </div>
      </div>

      <!-- 系统设置 · 快捷键独立配置面板（平铺展示，便于直接评审） -->
      <div class="mt-8 rounded-xl border bg-card p-6 shadow-xs space-y-4">
        <div class="flex flex-wrap items-center justify-between gap-3 border-b pb-4">
          <div class="flex items-center gap-2.5">
            <div class="flex size-9 items-center justify-center rounded-lg bg-primary/10 text-primary">
              <Settings2 class="size-5" />
            </div>
            <div>
              <div class="flex items-center gap-2">
                <h3 class="text-base font-semibold">系统设置 · 快捷键独立设置板块</h3>
                <Badge variant="secondary" class="text-xs">系统设置 Tab 预览</Badge>
              </div>
              <p class="text-xs text-muted-foreground">在系统设置弹窗中单独成块，支持按键自定义录制、跨环境（桌面 GUI / Web 网页）自适应与智能隐藏</p>
            </div>
          </div>
          <div class="flex items-center gap-2">
            <Button size="sm" variant="outline" class="gap-1.5" @click="configOpen = true; activeTab = 'shortcuts'">
              <Maximize2 class="size-3.5" />
              <span>弹窗模式查看</span>
            </Button>
          </div>
        </div>

        <ShortcutSettingsPanel :shortcut-state="shortcutState" />
      </div>
    </main>

    <!-- 系统配置弹窗 (重点呈现快捷键独立板块) -->
    <Dialog v-model:open="configOpen">
      <DialogContent class="flex h-[min(720px,90dvh)] flex-col gap-0 overflow-hidden p-0 sm:max-w-2xl" :aria-describedby="undefined">
        <DialogHeader class="shrink-0 border-b px-5 py-4">
          <div class="flex items-center justify-between pr-4">
            <DialogTitle>系统设置</DialogTitle>
            <Badge variant="outline" class="text-xs">原型配置方案</Badge>
          </div>
        </DialogHeader>

        <Tabs v-model="activeTab" class="flex min-h-0 flex-1 flex-col gap-0">
          <div class="shrink-0 px-5 pt-4">
            <TabsList class="grid h-auto w-full grid-cols-5">
              <TabsTrigger value="general">通用</TabsTrigger>
              <TabsTrigger value="player">投屏</TabsTrigger>
              <TabsTrigger value="shortcuts" class="font-medium text-primary data-[state=active]:bg-primary/10">
                快捷键
              </TabsTrigger>
              <TabsTrigger value="service">服务</TabsTrigger>
              <TabsTrigger value="dependencies">依赖</TabsTrigger>
            </TabsList>
          </div>

          <div class="min-h-0 flex-1 overflow-y-auto px-5 py-5 [scrollbar-gutter:stable]">
            <TabsContent value="general" class="m-0 space-y-4">
              <p class="text-sm font-medium">外观与语言</p>
              <div class="rounded-lg border p-4 text-xs text-muted-foreground">
                通用选项包含主题明暗切换、语言偏好设置等。
              </div>
            </TabsContent>

            <TabsContent value="player" class="m-0 space-y-4">
              <p class="text-sm font-medium">默认投屏偏好</p>
              <div class="rounded-lg border p-4 text-xs text-muted-foreground">
                投屏比特率、分辨率、无边框模式及浮动小窗默认尺寸设置。
              </div>
            </TabsContent>

            <!-- 核心：快捷键独立设置板块 -->
            <TabsContent value="shortcuts" class="m-0">
              <ShortcutSettingsPanel :shortcut-state="shortcutState" />
            </TabsContent>

            <TabsContent value="service" class="m-0 space-y-4">
              <p class="text-sm font-medium">服务配置</p>
              <div class="rounded-lg border p-4 text-xs text-muted-foreground">
                本地回环端口（1989）及 AI 视觉决策服务等配置。
              </div>
            </TabsContent>

            <TabsContent value="dependencies" class="m-0 space-y-4">
              <p class="text-sm font-medium">运行依赖</p>
              <div class="rounded-lg border p-4 text-xs text-muted-foreground">
                ADB 工具包、Python 环境及 OCR 引擎状态检测。
              </div>
            </TabsContent>
          </div>
        </Tabs>

        <footer class="flex shrink-0 items-center justify-between border-t px-5 py-3 bg-muted/20">
          <div class="flex items-center gap-2 text-xs text-muted-foreground">
            <Keyboard class="size-3.5" />
            <span>按键设置即时生效，已保存到本地配置</span>
          </div>
          <div class="flex items-center gap-2">
            <Button variant="outline" size="sm" @click="configOpen = false">
              完成
            </Button>
          </div>
        </footer>
      </DialogContent>
    </Dialog>
  </div>
</template>
