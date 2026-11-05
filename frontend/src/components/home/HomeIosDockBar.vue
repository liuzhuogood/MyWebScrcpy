<script setup lang="ts">
import { computed, nextTick, onUnmounted, ref } from 'vue'
import { onClickOutside, onKeyStroke } from '@vueuse/core'
import {
  ArrowLeft,
  ChevronDown,
  Home,
  Info,
  Keyboard,
  Layers,
  LayoutGrid,
  MoreHorizontal,
  Maximize2,
  Minimize2,
  Plus,
  Radio,
  Search,
  Settings,
  SlidersHorizontal,
  Smartphone,
  Sparkles,
  X,
} from 'lucide-vue-next'
import { Badge } from '@/components/ui/badge'
import { Button } from '@/components/ui/button'
import { Input } from '@/components/ui/input'
import { Slider } from '@/components/ui/slider'
import type { Device } from '@/api/devices'
import type { BatchItemResult, BatchSummary } from '@/composables/useDeviceBatch'
import type { HomeViewMode } from '@/composables/usePlayerSettings'
import {
  HOME_PHONE_SCALE_MAX,
  HOME_PHONE_SCALE_MIN,
  HOME_QUALITY_MAX,
  HOME_QUALITY_MIN,
  usePlayerSettings,
} from '@/composables/usePlayerSettings'

type TabKey = 'actions' | 'view' | 'devices' | 'more'

const props = withDefaults(
  defineProps<{
    keyword: string
    viewMode: HomeViewMode
    devices: Device[]
    selectedSerials: string[]
    onlineCount: number
    isAllSelected?: boolean
    batchRunning?: boolean
    currentAction?: string
    batchResults?: BatchItemResult[]
    batchSummary?: BatchSummary
    isSyncActive?: boolean
    displayName?: (device: Device) => string
  }>(),
  {
    isAllSelected: false,
    batchRunning: false,
    currentAction: '',
    batchResults: () => [],
    batchSummary: () => ({
      batchId: '',
      action: '',
      total: 0,
      success: 0,
      failed: 0,
      cancelled: 0,
      unknown: 0,
      inProgress: false,
    }),
    isSyncActive: false,
    displayName: (d: Device) => d.alias || d.model || d.serial,
  },
)

const emit = defineEmits<{
  (e: 'update:keyword', val: string): void
  (e: 'update:viewMode', val: HomeViewMode): void
  (e: 'update:selectedSerials', val: string[]): void
  (e: 'toggleSelect', serial: string): void
  (e: 'selectAll'): void
  (e: 'clearSelection'): void
  (e: 'triggerAction', action: 'back' | 'home' | 'app_switch'): void
  (e: 'toggleSync'): void
  (e: 'cancelBatch'): void
  (e: 'clearResults'): void
  (e: 'openAdd'): void
  (e: 'openConfig'): void
  (e: 'openAi'): void
  (e: 'openShortcuts'): void
  (e: 'openDocs'): void
}>()

const settings = usePlayerSettings()
const { homePhoneScale, homeQuality, homeBitrate: bitrateValue } = settings

// 当前激活展开的标签页（null 表示收起浮动卡片）
const activeTab = ref<TabKey | null>(null)
const dockRootRef = ref<HTMLElement | null>(null)
onClickOutside(dockRootRef, closePanel)
onKeyStroke('Escape', event => {
  // Let input methods and nested dialogs consume Escape first.
  queueMicrotask(() => {
    if (event.defaultPrevented || event.isComposing || document.querySelector('[role="dialog"][data-state="open"]')) return
    closePanel()
  })
})

// 底部浮动栏自身是否最小化为悬浮气泡
const isDockMinimized = ref(false)

const searchInputRef = ref<HTMLInputElement | null>(null)
const gridContainerRef = ref<HTMLElement | null>(null)
const devicesExpanded = ref(false)
let cleanupMarquee: (() => void) | undefined
let marqueeResetTimer: ReturnType<typeof setTimeout> | undefined
onUnmounted(() => { cleanupMarquee?.(); clearTimeout(marqueeResetTimer) })

function toggleTab(tab: TabKey) {
  if (activeTab.value === tab) {
    activeTab.value = null
  }
  else {
    activeTab.value = tab
    if (tab === 'devices') {
      nextTick(() => {
        searchInputRef.value?.focus()
      })
    }
  }
}

function closePanel() {
  activeTab.value = null
}

// 编号设备生成 (稳定按 serial 排序，包含离线设备)
const numberedDevices = computed(() => {
  return [...props.devices]
    .sort((a, b) => a.serial.localeCompare(b.serial))
    .map((device, index) => {
      const label = index + 1 < 10 ? `0${index + 1}` : String(index + 1)
      return { device, label, serial: device.serial }
    })
})

const selectedSet = computed(() => new Set(props.selectedSerials))

const cardWidth = computed(() => Math.round(270 * homePhoneScale.value / 100))

const qualityLabel = computed(() => {
  if (homeQuality.value <= 60) return '720P 标清'
  if (homeQuality.value <= 80) return '1080P 高清'
  if (homeQuality.value < 100) return '2K 超清'
  return '100% 原画'
})

function handleRangeUpdate(value: number | Array<number> | undefined, apply: (value: number) => void) {
  const next = Array.isArray(value) ? value[0] : value
  if (typeof next === 'number') {
    apply(next)
  }
}

// 鼠标拖拽框选逻辑 (Marquee / Box Selection)
const isDraggingMarquee = ref(false)
const marqueeBox = ref<{ left: number; top: number; width: number; height: number } | null>(null)
let dragStartX = 0
let dragStartY = 0
let initialSelectedAtDrag: string[] = []
let isCtrlPressed = false
let hasDraggedDistance = false

function handleGridMouseDown(e: MouseEvent) {
  // 仅响应鼠标左键，且容器存在
  if (e.button !== 0 || !gridContainerRef.value) return

  cleanupMarquee?.()
  clearTimeout(marqueeResetTimer)
  const container = gridContainerRef.value
  const containerRect = container.getBoundingClientRect()
  if (e.clientX >= containerRect.left + container.clientWidth || e.clientY >= containerRect.top + container.clientHeight) return
  e.preventDefault()
  dragStartX = e.clientX - containerRect.left + container.scrollLeft
  dragStartY = e.clientY - containerRect.top + container.scrollTop
  isCtrlPressed = e.ctrlKey || e.metaKey
  initialSelectedAtDrag = [...props.selectedSerials]
  hasDraggedDistance = false

  function onMouseMove(moveEvent: MouseEvent) {
    if (!gridContainerRef.value) return
    const container = gridContainerRef.value
    const rect = container.getBoundingClientRect()
    const currentX = moveEvent.clientX - rect.left + container.scrollLeft
    const currentY = moveEvent.clientY - rect.top + container.scrollTop

    const diffX = Math.abs(currentX - dragStartX)
    const diffY = Math.abs(currentY - dragStartY)

    if (!hasDraggedDistance && (diffX > 4 || diffY > 4)) {
      hasDraggedDistance = true
      isDraggingMarquee.value = true
    }

    if (hasDraggedDistance) {
      const left = Math.min(currentX, dragStartX)
      const top = Math.min(currentY, dragStartY)
      const width = Math.abs(currentX - dragStartX)
      const height = Math.abs(currentY - dragStartY)

      marqueeBox.value = { left, top, width, height }

      // 实时计算与数字按钮的重叠相交 (AABB 碰撞检测)
      const buttons = container.querySelectorAll<HTMLElement>('[data-device-serial]')
      const selectedInBox = new Set<string>()

      buttons.forEach((btn) => {
        const btnRect = btn.getBoundingClientRect()
        const btnLeft = btnRect.left - rect.left + container.scrollLeft
        const btnTop = btnRect.top - rect.top + container.scrollTop
        const btnRight = btnLeft + btnRect.width
        const btnBottom = btnTop + btnRect.height

        const boxRight = left + width
        const boxBottom = top + height

        const intersects = !(left > btnRight || boxRight < btnLeft || top > btnBottom || boxBottom < btnTop)
        if (intersects) {
          const serial = btn.dataset.deviceSerial
          if (serial) selectedInBox.add(serial)
        }
      })

      if (isCtrlPressed) {
        const combined = new Set(initialSelectedAtDrag)
        selectedInBox.forEach(s => combined.add(s))
        emit('update:selectedSerials', Array.from(combined))
      }
      else {
        emit('update:selectedSerials', Array.from(selectedInBox))
      }
    }
  }

  function onMouseUp() {
    cleanupMarquee?.()
    cleanupMarquee = undefined

    if (hasDraggedDistance) {
      marqueeResetTimer = setTimeout(() => {
        isDraggingMarquee.value = false
        marqueeBox.value = null
      }, 60)
    }
    else {
      isDraggingMarquee.value = false
      marqueeBox.value = null
    }
  }

  window.addEventListener('mousemove', onMouseMove)
  window.addEventListener('mouseup', onMouseUp)
  cleanupMarquee = () => {
    window.removeEventListener('mousemove', onMouseMove)
    window.removeEventListener('mouseup', onMouseUp)
  }
}

function handleDeviceClick(serial: string) {
  // 如果刚结束框选拖拽，阻止普通点击冲突
  if (isDraggingMarquee.value || hasDraggedDistance) return
  emit('toggleSelect', serial)
}
</script>

<template>
  <div ref="dockRootRef" class="fixed bottom-0 left-0 right-0 pointer-events-none z-50 flex flex-col items-center justify-end pb-5 px-4">
    <!-- iOS 风格浮动弹层卡片 (Floating Sheet Card) -->
    <Transition
      enter-active-class="transition duration-250 ease-out"
      enter-from-class="opacity-0 translate-y-6 scale-95"
      enter-to-class="opacity-100 translate-y-0 scale-100"
      leave-active-class="transition duration-180 ease-in"
      leave-from-class="opacity-100 translate-y-0 scale-100"
      leave-to-class="opacity-0 translate-y-6 scale-95"
    >
      <div
        v-if="activeTab"
        :class="activeTab === 'devices' && devicesExpanded ? 'max-w-7xl p-3' : 'max-w-lg p-4'"
        class="pointer-events-auto relative mb-2 w-full overflow-hidden rounded-3xl border border-white/80 bg-white/85 shadow-[0_20px_50px_rgba(0,0,0,0.15)] backdrop-blur-2xl transition-all dark:border-white/15 dark:bg-[#0e121d]/90 dark:shadow-[0_20px_50px_rgba(0,0,0,0.6)]"
      >
        <!-- 卡片顶部抓手横条 & 标题 -->
        <div class="flex items-center justify-between pb-3 border-b border-slate-200/60 dark:border-white/10">
          <div class="flex items-center gap-2">
            <span class="h-2 w-2 rounded-full bg-blue-600 dark:bg-blue-400" />
            <h3 class="text-sm font-bold text-slate-800 dark:text-slate-100">
              {{
                activeTab === 'actions' ? '批量操作控制' :
                activeTab === 'view' ? '显示与视图设置' :
                activeTab === 'devices' ? '设备筛选与管理' : '系统扩展与更多'
              }}
            </h3>
          </div>
          <button
            type="button"
            class="flex h-6 w-6 items-center justify-center rounded-full text-slate-400 transition-colors hover:bg-slate-200/50 hover:text-slate-700 dark:hover:bg-white/10 dark:hover:text-white cursor-pointer"
            aria-label="关闭"
            @click="closePanel"
          >
            <X class="h-3.5 w-3.5" />
          </button>
        </div>

        <!-- 1. 操作面板内容 -->
        <div v-if="activeTab === 'actions'" class="space-y-4 pt-3.5">
          <div class="flex items-center justify-between">
            <div class="flex items-center gap-2">
              <span class="text-xs text-slate-500 dark:text-slate-400">已选中设备</span>
              <Badge variant="secondary" class="font-mono text-xs font-semibold px-2">
                {{ selectedSerials.length }} 台
              </Badge>
            </div>
            <div class="flex items-center gap-2">
              <Button
                variant="outline"
                size="sm"
                class="h-7 rounded-xl text-xs font-medium cursor-pointer"
                :disabled="batchRunning"
                @click="emit('selectAll')"
              >
                {{ isAllSelected ? '取消全选' : '全选在线' }}
              </Button>
              <Button
                variant="ghost"
                size="sm"
                class="h-7 rounded-xl text-xs font-medium cursor-pointer text-slate-500 hover:text-slate-800 dark:text-slate-400 dark:hover:text-white"
                :disabled="selectedSerials.length === 0 || batchRunning"
                @click="emit('clearSelection')"
              >
                清空
              </Button>
            </div>
          </div>

          <!-- 导航三键与同屏同步操作 -->
          <div class="space-y-2">
            <span class="text-[11px] font-semibold tracking-wider text-slate-400 uppercase">系统导航指令</span>
            <div class="grid grid-cols-4 gap-2">
              <Button
                variant="secondary"
                size="sm"
                class="h-9 rounded-xl flex items-center justify-center gap-1.5 cursor-pointer border border-slate-200/60 bg-white/70 shadow-xs hover:border-blue-400 hover:bg-blue-50/50 dark:border-white/10 dark:bg-white/5 dark:hover:border-blue-500/30 dark:hover:bg-blue-950/20"
                :disabled="selectedSerials.length === 0 || batchRunning"
                title="返回"
                @click="emit('triggerAction', 'back')"
              >
                <ArrowLeft class="h-4 w-4 text-blue-600 dark:text-blue-400" />
                <span class="text-xs">返回</span>
              </Button>

              <Button
                variant="secondary"
                size="sm"
                class="h-9 rounded-xl flex items-center justify-center gap-1.5 cursor-pointer border border-slate-200/60 bg-white/70 shadow-xs hover:border-blue-400 hover:bg-blue-50/50 dark:border-white/10 dark:bg-white/5 dark:hover:border-blue-500/30 dark:hover:bg-blue-950/20"
                :disabled="selectedSerials.length === 0 || batchRunning"
                title="主页"
                @click="emit('triggerAction', 'home')"
              >
                <Home class="h-4 w-4 text-blue-600 dark:text-blue-400" />
                <span class="text-xs">主页</span>
              </Button>

              <Button
                variant="secondary"
                size="sm"
                class="h-9 rounded-xl flex items-center justify-center gap-1.5 cursor-pointer border border-slate-200/60 bg-white/70 shadow-xs hover:border-blue-400 hover:bg-blue-50/50 dark:border-white/10 dark:bg-white/5 dark:hover:border-blue-500/30 dark:hover:bg-blue-950/20"
                :disabled="selectedSerials.length === 0 || batchRunning"
                title="多任务"
                @click="emit('triggerAction', 'app_switch')"
              >
                <Layers class="h-4 w-4 text-blue-600 dark:text-blue-400" />
                <span class="text-xs">多任务</span>
              </Button>

              <Button
                size="sm"
                :variant="isSyncActive ? 'default' : 'secondary'"
                class="h-9 rounded-xl flex items-center justify-center gap-1.5 cursor-pointer transition-all border border-slate-200/60 shadow-xs"
                :class="isSyncActive
                  ? 'bg-purple-600 text-white hover:bg-purple-700 shadow-md ring-2 ring-purple-500/30 border-transparent'
                  : 'bg-white/70 text-slate-700 hover:border-purple-400 hover:bg-purple-50/50 dark:border-white/10 dark:bg-white/5 dark:text-slate-300 dark:hover:border-purple-500/30 dark:hover:bg-purple-950/20'"
                :disabled="selectedSerials.length === 0 || batchRunning"
                title="群控：操作一台手机时，把操作实时同步到其他已选手机"
                @click="emit('toggleSync')"
              >
                <Radio class="h-4 w-4" :class="{ 'animate-pulse text-white': isSyncActive, 'text-purple-600 dark:text-purple-400': !isSyncActive }" />
                <span class="text-xs font-medium">{{ isSyncActive ? '群控中' : '群控' }}</span>
              </Button>
            </div>
          </div>

          <!-- 群控状态提示条 -->
          <div
            v-if="isSyncActive"
            class="flex items-center justify-between rounded-xl bg-purple-50/80 p-2.5 text-xs text-purple-700 dark:bg-purple-950/40 dark:text-purple-300 border border-purple-200/60 dark:border-purple-800/40"
          >
            <div class="flex items-center gap-2">
              <span class="h-2 w-2 rounded-full bg-purple-500 animate-pulse" />
              <span>已开启群控：触控操作任意已选设备将实时同步至其余 {{ Math.max(0, selectedSerials.length - 1) }} 台设备</span>
            </div>
          </div>

          <!-- 批量执行状态反馈 -->
          <div
            v-if="batchRunning"
            class="flex items-center justify-between rounded-xl bg-blue-50/80 p-2.5 text-xs text-blue-700 dark:bg-blue-950/40 dark:text-blue-300 border border-blue-200/60 dark:border-blue-800/40"
          >
            <div class="flex items-center gap-2">
              <span class="h-2 w-2 rounded-full bg-blue-500 animate-ping" />
              <span>正在向 {{ selectedSerials.length }} 台设备派发【{{ currentAction }}】指令...</span>
            </div>
            <Button variant="ghost" size="sm" class="h-6 rounded-lg px-2 text-xs text-red-600 hover:text-red-700" @click="emit('cancelBatch')">
              取消
            </Button>
          </div>
          <div
            v-else-if="batchResults.length"
            class="flex items-center justify-between text-[11px] text-slate-500 dark:text-slate-400 bg-slate-50/60 dark:bg-white/5 p-2 rounded-xl"
          >
            <span>执行完成：成功 {{ batchSummary.success }} / 失败 {{ batchSummary.failed }} / 待确认 {{ batchSummary.unknown }}</span>
            <button type="button" class="cursor-pointer hover:text-slate-900 dark:hover:text-white" @click="emit('clearResults')">清除记录</button>
          </div>
        </div>

        <!-- 2. 视图面板内容 -->
        <div v-if="activeTab === 'view'" class="space-y-4 pt-3.5">
          <!-- 模式切换：卡片 vs 手机 -->
          <div class="flex items-center justify-between">
            <span class="text-xs font-medium text-slate-600 dark:text-slate-300">首页展现模式</span>
            <div class="flex rounded-xl border border-slate-200/80 bg-slate-100/80 p-0.5 dark:border-white/10 dark:bg-black/20">
              <button
                type="button"
                class="flex h-7.5 items-center gap-1.5 rounded-lg px-3 text-xs font-medium transition-all cursor-pointer"
                :class="viewMode === 'deck'
                  ? 'bg-white font-semibold text-slate-950 shadow-xs dark:bg-white/15 dark:text-white'
                  : 'text-slate-500 hover:text-slate-950 dark:text-slate-400 dark:hover:text-white'"
                @click="emit('update:viewMode', 'deck')"
              >
                <LayoutGrid class="h-3.5 w-3.5 text-blue-600 dark:text-blue-400" />
                <span>卡片视图</span>
              </button>
              <button
                type="button"
                class="flex h-7.5 items-center gap-1.5 rounded-lg px-3 text-xs font-medium transition-all cursor-pointer"
                :class="viewMode === 'phone'
                  ? 'bg-white font-semibold text-slate-950 shadow-xs dark:bg-white/15 dark:text-white'
                  : 'text-slate-500 hover:text-slate-950 dark:text-slate-400 dark:hover:text-white'"
                @click="emit('update:viewMode', 'phone')"
              >
                <Smartphone class="h-3.5 w-3.5 text-blue-600 dark:text-blue-400" />
                <span>手机视图</span>
              </button>
            </div>
          </div>

          <!-- 手机大小滑块 -->
          <div class="space-y-1.5 rounded-2xl border border-slate-200/60 bg-slate-50/50 p-3 dark:border-white/5 dark:bg-black/20">
            <div class="flex items-center justify-between text-xs">
              <span class="font-medium text-slate-700 dark:text-slate-300">手机卡片尺寸</span>
              <span class="font-mono font-semibold text-blue-600 dark:text-blue-400">{{ homePhoneScale }}% · {{ cardWidth }}px</span>
            </div>
            <Slider
              :model-value="[homePhoneScale]"
              :min="HOME_PHONE_SCALE_MIN"
              :max="HOME_PHONE_SCALE_MAX"
              :step="5"
              aria-label="手机大小"
              @update:model-value="handleRangeUpdate($event, value => homePhoneScale = value)"
            />
            <div class="flex justify-between text-[10px] text-slate-400">
              <button type="button" class="cursor-pointer hover:text-blue-600" @click="homePhoneScale = 75">紧凑 (75%)</button>
              <button type="button" class="cursor-pointer hover:text-blue-600" @click="homePhoneScale = 100">标准 (100%)</button>
              <button type="button" class="cursor-pointer hover:text-blue-600" @click="homePhoneScale = 130">超大 (130%)</button>
            </div>
          </div>

          <!-- 投屏码率滑块 -->
          <div class="space-y-1.5 rounded-2xl border border-slate-200/60 bg-slate-50/50 p-3 dark:border-white/5 dark:bg-black/20">
            <div class="flex items-center justify-between text-xs">
              <span class="font-medium text-slate-700 dark:text-slate-300">实时投屏码率</span>
              <span class="font-mono font-semibold text-blue-600 dark:text-blue-400">{{ bitrateValue }} Mbps</span>
            </div>
            <Slider
              :model-value="[bitrateValue]"
              :min="2"
              :max="16"
              :step="1"
              aria-label="投屏码率"
              @update:model-value="handleRangeUpdate($event, value => bitrateValue = value)"
            />
            <div class="flex justify-between text-[10px] text-slate-400">
              <button type="button" class="cursor-pointer hover:text-blue-600" @click="bitrateValue = 2">流畅 (2M)</button>
              <button type="button" class="cursor-pointer hover:text-blue-600" @click="bitrateValue = 8">超清 (8M)</button>
              <button type="button" class="cursor-pointer hover:text-blue-600" @click="bitrateValue = 16">极致 (16M)</button>
            </div>
          </div>

          <!-- 投屏质量滑块 -->
          <div class="space-y-1.5 rounded-2xl border border-slate-200/60 bg-slate-50/50 p-3 dark:border-white/5 dark:bg-black/20">
            <div class="flex items-center justify-between text-xs">
              <span class="font-medium text-slate-700 dark:text-slate-300">投屏画质清晰度</span>
              <span class="font-mono font-semibold text-blue-600 dark:text-blue-400">{{ qualityLabel }} ({{ homeQuality }}%)</span>
            </div>
            <Slider
              :model-value="[homeQuality]"
              :min="HOME_QUALITY_MIN"
              :max="HOME_QUALITY_MAX"
              :step="5"
              aria-label="投屏画质"
              @update:model-value="handleRangeUpdate($event, value => homeQuality = value)"
            />
            <div class="flex justify-between text-[10px] text-slate-400">
              <button type="button" class="cursor-pointer hover:text-blue-600" @click="homeQuality = 60">720P</button>
              <button type="button" class="cursor-pointer hover:text-blue-600" @click="homeQuality = 80">1080P</button>
              <button type="button" class="cursor-pointer hover:text-blue-600" @click="homeQuality = 90">2K</button>
              <button type="button" class="cursor-pointer hover:text-blue-600" @click="homeQuality = 100">原画</button>
            </div>
          </div>
        </div>

        <!-- 3. 设备管理面板内容 (集成搜索 + 连接 + 编号方块 + 原生拉框框选) -->
        <div v-if="activeTab === 'devices'" class="space-y-2 pt-2">
          <!-- 搜索框与添加按钮 -->
          <div class="flex items-center gap-2">
            <div class="relative flex-1">
              <Search class="absolute left-3 top-1/2 h-3.5 w-3.5 -translate-y-1/2 text-slate-400" />
              <Input
                ref="searchInputRef"
                :model-value="keyword"
                placeholder="搜索设备名称、型号或序列号"
                class="h-9 rounded-xl pl-9 pr-8 text-xs border-slate-200/80 bg-white/70 dark:border-white/10 dark:bg-black/20"
                @update:model-value="emit('update:keyword', String($event))"
              />
              <button
                v-if="keyword"
                type="button"
                class="absolute right-2.5 top-1/2 -translate-y-1/2 text-slate-400 hover:text-slate-600 dark:hover:text-white cursor-pointer"
                aria-label="清除搜索"
                @click="emit('update:keyword', '')"
              >
                <X class="h-3.5 w-3.5" />
              </button>
            </div>
            <Button
              size="sm"
              class="h-9 rounded-xl bg-blue-600 text-white hover:bg-blue-700 px-3 cursor-pointer shadow-sm shrink-0"
              @click="emit('openAdd')"
            >
              <Plus class="h-4 w-4 mr-1" />
              <span class="text-xs">添加</span>
            </Button>
          </div>

          <!-- 在线与已选统计 -->
          <div class="flex items-center justify-between text-xs text-slate-500">
            <div class="flex items-center gap-2">
              <span class="flex items-center gap-1 font-medium">
                <span class="h-2 w-2 rounded-full bg-emerald-500" />
                {{ onlineCount }} 台在线
              </span>
              <span class="text-slate-300 dark:text-white/20">|</span>
              <span class="flex items-center gap-1 font-medium text-blue-600 dark:text-blue-400">
                {{ selectedSerials.length }} 台已选
              </span>
            </div>
            <Button variant="ghost" size="icon" class="h-6 w-6" :aria-label="devicesExpanded ? '收起设备区域' : '展开设备区域'" :title="devicesExpanded ? '收起设备区域' : '展开设备区域'" @click="devicesExpanded = !devicesExpanded">
              <Minimize2 v-if="devicesExpanded" class="h-3.5 w-3.5" /><Maximize2 v-else class="h-3.5 w-3.5" />
            </Button>
          </div>

          <!-- 01..N 设备编号方块矩阵（支持原生拖拽拉框多选） -->
          <div
            ref="gridContainerRef"
            data-device-grid
            :class="devicesExpanded ? 'max-h-[calc(100dvh-15rem)] grid-cols-[repeat(auto-fill,minmax(2.25rem,1fr))] gap-0.5 p-1' : 'max-h-52 grid-cols-8 gap-1 p-1.5'"
            class="relative grid overflow-y-auto rounded-xl border border-slate-200/80 bg-slate-50/60 dark:border-white/10 dark:bg-black/30 select-none"
            @mousedown="handleGridMouseDown"
          >
            <!-- 鼠标拉框选区指示盒 -->
            <div
              v-if="isDraggingMarquee && marqueeBox"
              class="pointer-events-none absolute z-20 rounded-lg border border-blue-500 bg-blue-500/20 backdrop-blur-[1px] transition-all duration-75"
              :style="{
                left: `${marqueeBox.left}px`,
                top: `${marqueeBox.top}px`,
                width: `${marqueeBox.width}px`,
                height: `${marqueeBox.height}px`,
              }"
            />

            <button
              v-for="item in numberedDevices"
              :key="item.serial"
              type="button"
              :data-device-serial="item.serial"
              class="relative flex items-center justify-center border font-mono font-bold transition-all cursor-pointer select-none"
              :class="[devicesExpanded ? 'h-6 rounded-md text-[10px]' : 'h-7 rounded-lg text-[11px]', selectedSet.has(item.serial)
                ? 'border-2 border-blue-500 bg-blue-600 text-white shadow-sm ring-2 ring-blue-500/30'
                : item.device.state !== 'device'
                  ? 'border-dashed border-slate-300 bg-slate-200/50 text-slate-400 dark:border-white/10 dark:bg-white/5'
                  : 'border-slate-300/80 bg-white text-slate-800 hover:border-blue-400 dark:border-white/20 dark:bg-white/10 dark:text-slate-100']"
              :title="`${item.label} · ${displayName(item.device)} (${item.device.state === 'device' ? '在线' : item.device.state})`"
              @click="handleDeviceClick(item.serial)"
            >
              {{ item.label }}
              <span
                class="absolute right-1 top-1 h-1.5 w-1.5 rounded-full"
                :class="item.device.state === 'device' ? 'bg-emerald-400' : 'bg-amber-400'"
              />
            </button>
          </div>
        </div>

        <!-- 4. 更多面板内容 -->
        <div v-else-if="activeTab === 'more'" class="space-y-3.5 pt-3.5">
          <div class="grid grid-cols-2 gap-2.5">
            <button
              type="button"
              class="group flex items-center gap-3 rounded-2xl border border-slate-200/70 bg-white/70 p-3 text-left transition-all hover:border-blue-500/40 hover:bg-blue-50/50 hover:shadow-sm dark:border-white/10 dark:bg-white/5 dark:hover:border-blue-400/30 dark:hover:bg-blue-950/20 cursor-pointer"
              @click="emit('openConfig'); closePanel()"
            >
              <div class="flex h-9 w-9 shrink-0 items-center justify-center rounded-xl bg-blue-100 text-blue-600 transition-transform group-hover:scale-105 dark:bg-blue-500/20 dark:text-blue-300">
                <Settings class="h-4 w-4" />
              </div>
              <div class="min-w-0">
                <div class="text-xs font-bold text-slate-800 dark:text-slate-100">系统配置</div>
                <div class="truncate text-[10px] text-slate-400">ADB / Python / OCR</div>
              </div>
            </button>

            <button
              type="button"
              class="group flex items-center gap-3 rounded-2xl border border-slate-200/70 bg-white/70 p-3 text-left transition-all hover:border-indigo-500/40 hover:bg-indigo-50/50 hover:shadow-sm dark:border-white/10 dark:bg-white/5 dark:hover:border-indigo-400/30 dark:hover:bg-indigo-950/20 cursor-pointer"
              @click="emit('openAi'); closePanel()"
            >
              <div class="flex h-9 w-9 shrink-0 items-center justify-center rounded-xl bg-indigo-100 text-indigo-600 transition-transform group-hover:scale-105 dark:bg-indigo-500/20 dark:text-indigo-300">
                <Sparkles class="h-4 w-4" />
              </div>
              <div class="min-w-0">
                <div class="text-xs font-bold text-slate-800 dark:text-slate-100">AI 技能市场</div>
                <div class="truncate text-[10px] text-slate-400">自动化与决策助手</div>
              </div>
            </button>

            <button
              type="button"
              class="group flex items-center gap-3 rounded-2xl border border-slate-200/70 bg-white/70 p-3 text-left transition-all hover:border-emerald-500/40 hover:bg-emerald-50/50 hover:shadow-sm dark:border-white/10 dark:bg-white/5 dark:hover:border-emerald-400/30 dark:hover:bg-emerald-950/20 cursor-pointer"
              @click="emit('openShortcuts'); closePanel()"
            >
              <div class="flex h-9 w-9 shrink-0 items-center justify-center rounded-xl bg-emerald-100 text-emerald-600 transition-transform group-hover:scale-105 dark:bg-emerald-500/20 dark:text-emerald-300">
                <Keyboard class="h-4 w-4" />
              </div>
              <div class="min-w-0">
                <div class="text-xs font-bold text-slate-800 dark:text-slate-100">快捷键指南</div>
                <div class="truncate text-[10px] text-slate-400">高效键盘操控手册</div>
              </div>
            </button>

            <button
              type="button"
              class="group flex items-center gap-3 rounded-2xl border border-slate-200/70 bg-white/70 p-3 text-left transition-all hover:border-cyan-500/40 hover:bg-cyan-50/50 hover:shadow-sm dark:border-white/10 dark:bg-white/5 dark:hover:border-cyan-400/30 dark:hover:bg-cyan-950/20 cursor-pointer"
              @click="emit('openDocs'); closePanel()"
            >
              <div class="flex h-9 w-9 shrink-0 items-center justify-center rounded-xl bg-cyan-100 text-cyan-600 transition-transform group-hover:scale-105 dark:bg-cyan-500/20 dark:text-cyan-300">
                <Info class="h-4 w-4" />
              </div>
              <div class="min-w-0">
                <div class="text-xs font-bold text-slate-800 dark:text-slate-100">关于与文档</div>
                <div class="truncate text-[10px] text-slate-400">OpenAPI 规范与版本</div>
              </div>
            </button>
          </div>

          <!-- 系统状态概要卡片 -->
          <div class="rounded-2xl border border-slate-200/70 bg-slate-50/70 p-3 dark:border-white/10 dark:bg-black/20 text-xs space-y-1.5">
            <div class="flex items-center justify-between">
              <span class="text-slate-500 dark:text-slate-400">版本</span>
              <span class="font-mono font-semibold text-slate-800 dark:text-slate-200">MyWebScrcpy v1.34.0</span>
            </div>
            <div class="flex items-center justify-between">
              <span class="text-slate-500 dark:text-slate-400">设备状态</span>
              <span class="text-emerald-600 dark:text-emerald-400 font-semibold flex items-center gap-1">
                <span class="h-1.5 w-1.5 rounded-full bg-emerald-500" />
                {{ onlineCount }} 在线
              </span>
            </div>
          </div>
        </div>
      </div>
    </Transition>

    <!-- iOS 底部悬浮玻璃操作栏 (Floating Island / Dock Bar) -->
    <div
      v-if="!isDockMinimized"
      class="liquid-glass pointer-events-auto flex items-center gap-1 rounded-full border p-1.5 transition-all"
    >
      <!-- 按钮 1：操作（收纳多机批量导航与同步） -->
      <button
        type="button"
        data-tab="actions"
        class="group relative flex items-center gap-2 rounded-full px-4 py-2 text-xs font-medium transition-all cursor-pointer"
        :class="activeTab === 'actions'
          ? 'bg-slate-900 text-white shadow-md dark:bg-white dark:text-slate-900'
          : 'text-slate-700 hover:bg-black/5 hover:text-slate-950 dark:text-slate-200 dark:hover:bg-white/10 dark:hover:text-white'"
        @click="toggleTab('actions')"
      >
        <SlidersHorizontal
          class="h-4 w-4 transition-transform group-hover:scale-110"
          :class="activeTab === 'actions' ? 'text-blue-300 dark:text-blue-600' : 'text-blue-600 dark:text-blue-400'"
        />
        <span class="tracking-wide">操作</span>
        <span
          v-if="selectedSerials.length"
          class="flex h-4.5 min-w-[18px] items-center justify-center rounded-full bg-blue-600 px-1.5 text-[10px] font-mono font-bold text-white shadow-xs"
        >
          {{ selectedSerials.length }}
        </span>
      </button>

      <!-- 分隔线 -->
      <div class="h-4 w-px bg-slate-300/60 dark:bg-white/15" />

      <!-- 按钮 2：视图 -->
      <button
        type="button"
        data-tab="view"
        class="group relative flex items-center gap-2 rounded-full px-4 py-2 text-xs font-medium transition-all cursor-pointer"
        :class="activeTab === 'view'
          ? 'bg-slate-900 text-white shadow-md dark:bg-white dark:text-slate-900'
          : 'text-slate-700 hover:bg-black/5 hover:text-slate-950 dark:text-slate-200 dark:hover:bg-white/10 dark:hover:text-white'"
        @click="toggleTab('view')"
      >
        <component
          :is="viewMode === 'deck' ? LayoutGrid : Smartphone"
          class="h-4 w-4 transition-transform group-hover:scale-110"
          :class="activeTab === 'view' ? 'text-cyan-300 dark:text-cyan-600' : 'text-cyan-600 dark:text-cyan-400'"
        />
        <span class="tracking-wide">视图</span>
      </button>

      <!-- 分隔线 -->
      <div class="h-4 w-px bg-slate-300/60 dark:bg-white/15" />

      <!-- 按钮 3：设备 -->
      <button
        type="button"
        data-tab="devices"
        class="group relative flex items-center gap-2 rounded-full px-4 py-2 text-xs font-medium transition-all cursor-pointer"
        :class="activeTab === 'devices'
          ? 'bg-slate-900 text-white shadow-md dark:bg-white dark:text-slate-900'
          : 'text-slate-700 hover:bg-black/5 hover:text-slate-950 dark:text-slate-200 dark:hover:bg-white/10 dark:hover:text-white'"
        @click="toggleTab('devices')"
      >
        <Smartphone
          class="h-4 w-4 transition-transform group-hover:scale-110"
          :class="activeTab === 'devices' ? 'text-emerald-300 dark:text-emerald-600' : 'text-emerald-600 dark:text-emerald-400'"
        />
        <span class="tracking-wide">设备</span>
      </button>

      <!-- 分隔线 -->
      <div class="h-4 w-px bg-slate-300/60 dark:bg-white/15" />

      <!-- 按钮 4：更多 -->
      <button
        type="button"
        data-tab="more"
        class="group relative flex items-center gap-2 rounded-full px-4 py-2 text-xs font-medium transition-all cursor-pointer"
        :class="activeTab === 'more'
          ? 'bg-slate-900 text-white shadow-md dark:bg-white dark:text-slate-900'
          : 'text-slate-700 hover:bg-black/5 hover:text-slate-950 dark:text-slate-200 dark:hover:bg-white/10 dark:hover:text-white'"
        @click="toggleTab('more')"
      >
        <MoreHorizontal
          class="h-4 w-4 transition-transform group-hover:scale-110"
          :class="activeTab === 'more' ? 'text-blue-300 dark:text-blue-600' : 'text-slate-500 dark:text-slate-400'"
        />
        <span class="tracking-wide">更多</span>
      </button>

      <!-- 最小化折叠开关 -->
      <button
        type="button"
        class="ml-0.5 flex h-7 w-7 items-center justify-center rounded-full text-slate-400 transition-colors hover:bg-black/5 hover:text-slate-700 dark:hover:bg-white/10 dark:hover:text-white cursor-pointer"
        title="隐藏到底部小气泡"
        @click="isDockMinimized = true; activeTab = null"
      >
        <ChevronDown class="h-3.5 w-3.5" />
      </button>
    </div>

    <!-- 最小化气泡小胶囊 (点击展开底栏) -->
    <button
      v-else
      type="button"
      class="liquid-glass pointer-events-auto flex cursor-pointer items-center gap-2 rounded-full border px-4 py-2 text-slate-800 transition-all hover:scale-105 dark:text-white"
      title="展开操作栏"
      @click="isDockMinimized = false"
    >
      <Sparkles class="h-4 w-4 text-blue-500 animate-pulse" />
      <span class="text-xs font-semibold">控制中心</span>
      <span
        v-if="selectedSerials.length"
        class="flex h-4 min-w-[16px] items-center justify-center rounded-full bg-blue-600 px-1 text-[9px] font-bold text-white"
      >
        {{ selectedSerials.length }}
      </span>
    </button>
  </div>
</template>
