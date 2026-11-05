<script setup lang="ts">
import { computed, nextTick, ref } from 'vue'
import {
  ArrowLeft,
  ChevronDown,
  Home,
  Info,
  Keyboard,
  Layers,
  LayoutGrid,
  MoreHorizontal,
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
import type { MockDevice } from '../mockDevices'

type TabKey = 'actions' | 'view' | 'devices' | 'more'

const props = defineProps<{
  keyword: string
  viewMode: 'deck' | 'phone'
  phoneScale: number
  bitrateMbps: number
  qualityPercent: number
  devices: MockDevice[]
  selectedSerials: string[]
  batchRunning: boolean
  isSyncActive: boolean
}>()

const emit = defineEmits<{
  (e: 'update:keyword', val: string): void
  (e: 'update:viewMode', val: 'deck' | 'phone'): void
  (e: 'update:phoneScale', val: number): void
  (e: 'update:bitrateMbps', val: number): void
  (e: 'update:qualityPercent', val: number): void
  (e: 'update:selectedSerials', val: string[]): void
  (e: 'toggleSelect', serial: string): void
  (e: 'selectAll'): void
  (e: 'clearSelection'): void
  (e: 'triggerBatchAction', action: 'back' | 'home' | 'app_switch'): void
  (e: 'toggleSync'): void
  (e: 'openAdd'): void
  (e: 'openConfig'): void
  (e: 'openAi'): void
  (e: 'openShortcuts'): void
  (e: 'openDocs'): void
}>()

// 当前激活展开的标签页（null 表示收起浮动卡片）
const activeTab = ref<TabKey | null>(null)

// 底部浮动栏自身是否最小化为悬浮气泡
const isDockMinimized = ref(false)

const searchInputRef = ref<HTMLInputElement | null>(null)
const gridContainerRef = ref<HTMLElement | null>(null)

function toggleTab(tab: TabKey) {
  if (activeTab.value === tab) {
    activeTab.value = null
  } else {
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
const onlineCount = computed(() => props.devices.filter(d => d.state === 'device').length)

const cardWidth = computed(() => Math.round(270 * props.phoneScale / 100))

const qualityLabel = computed(() => {
  if (props.qualityPercent <= 60) return '720P 标清'
  if (props.qualityPercent <= 80) return '1080P 高清'
  if (props.qualityPercent < 100) return '2K 超清'
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

  const containerRect = gridContainerRef.value.getBoundingClientRect()
  dragStartX = e.clientX - containerRect.left
  dragStartY = e.clientY - containerRect.top
  isCtrlPressed = e.ctrlKey || e.metaKey
  initialSelectedAtDrag = [...props.selectedSerials]
  hasDraggedDistance = false

  function onMouseMove(moveEvent: MouseEvent) {
    if (!gridContainerRef.value) return
    const container = gridContainerRef.value
    const rect = container.getBoundingClientRect()
    const currentX = moveEvent.clientX - rect.left
    const currentY = moveEvent.clientY - rect.top

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
        const btnLeft = btnRect.left - rect.left
        const btnTop = btnRect.top - rect.top
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
      } else {
        emit('update:selectedSerials', Array.from(selectedInBox))
      }
    }
  }

  function onMouseUp() {
    window.removeEventListener('mousemove', onMouseMove)
    window.removeEventListener('mouseup', onMouseUp)

    if (hasDraggedDistance) {
      setTimeout(() => {
        isDraggingMarquee.value = false
        marqueeBox.value = null
      }, 60)
    } else {
      isDraggingMarquee.value = false
      marqueeBox.value = null
    }
  }

  window.addEventListener('mousemove', onMouseMove)
  window.addEventListener('mouseup', onMouseUp)
}

function handleDeviceClick(serial: string) {
  // 如果刚结束框选拖拽，阻止普通点击冲突
  if (isDraggingMarquee.value || hasDraggedDistance) return
  emit('toggleSelect', serial)
}
</script>

<template>
  <div class="fixed bottom-0 left-0 right-0 pointer-events-none z-50 flex flex-col items-center justify-end pb-5 px-4">
    <!-- 点击遮罩（展开浮动卡片时可点击背景任意处收起） -->
    <div
      v-if="activeTab"
      class="fixed inset-0 bg-black/20 dark:bg-black/40 backdrop-blur-[2px] pointer-events-auto transition-opacity"
      @click="closePanel"
    />

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
        class="pointer-events-auto mb-3 w-full max-w-[460px] rounded-[28px] border border-white/60 bg-white/90 p-5 shadow-2xl backdrop-blur-3xl dark:border-white/15 dark:bg-[#0f1422]/90 text-slate-800 dark:text-slate-100 ring-1 ring-black/5 dark:ring-white/10"
      >
        <!-- iOS 顶部小药丸抓手 (Grabber Handle) -->
        <div
          class="mx-auto mb-3 h-1 w-10 cursor-pointer rounded-full bg-slate-300 transition-colors hover:bg-slate-400 dark:bg-white/20 dark:hover:bg-white/40"
          title="点击收起"
          @click="closePanel"
        />

        <!-- 面板顶栏 -->
        <div class="flex items-center justify-between border-b border-slate-200/70 pb-3 dark:border-white/10">
          <div class="flex items-center gap-2">
            <span
              class="flex h-7 w-7 items-center justify-center rounded-xl bg-blue-600/10 text-blue-600 dark:bg-blue-500/20 dark:text-blue-300"
            >
              <SlidersHorizontal v-if="activeTab === 'actions'" class="h-4 w-4" />
              <LayoutGrid v-else-if="activeTab === 'view'" class="h-4 w-4" />
              <Smartphone v-else-if="activeTab === 'devices'" class="h-4 w-4" />
              <MoreHorizontal v-else-if="activeTab === 'more'" class="h-4 w-4" />
            </span>
            <h3 class="text-sm font-bold tracking-tight">
              {{
                activeTab === 'actions'
                  ? '操作控制'
                  : activeTab === 'view'
                    ? '视图与显示设置'
                    : activeTab === 'devices'
                      ? '设备管理'
                      : '更多功能与服务'
              }}
            </h3>
          </div>
          <button
            type="button"
            class="flex h-7 w-7 items-center justify-center rounded-full bg-slate-100 text-slate-500 transition-colors hover:bg-slate-200 hover:text-slate-900 dark:bg-white/10 dark:text-slate-400 dark:hover:bg-white/20 dark:hover:text-white cursor-pointer"
            title="关闭面板"
            @click="closePanel"
          >
            <X class="h-3.5 w-3.5" />
          </button>
        </div>

        <!-- 1. 操作面板内容（批量控制核心功能） -->
        <div v-if="activeTab === 'actions'" class="space-y-3.5 pt-3.5">
          <div class="flex items-center justify-between text-xs">
            <div class="flex items-center gap-1.5">
              <span class="font-medium text-slate-600 dark:text-slate-400">已选设备：</span>
              <span class="font-bold text-blue-600 dark:text-blue-400">{{ selectedSerials.length }} 台</span>
            </div>
            <div class="flex items-center gap-2">
              <Button
                variant="outline"
                size="sm"
                class="h-7 rounded-xl text-xs cursor-pointer"
                :disabled="batchRunning"
                @click="emit('selectAll')"
              >
                {{ selectedSerials.length === onlineCount && onlineCount > 0 ? '取消全选' : '全选在线' }}
              </Button>
              <Button
                variant="outline"
                size="sm"
                class="h-7 rounded-xl text-xs cursor-pointer"
                :disabled="selectedSerials.length === 0 || batchRunning"
                @click="emit('clearSelection')"
              >
                清空
              </Button>
            </div>
          </div>

          <!-- Android 经典导航手势按键 -->
          <div class="space-y-1.5">
            <span class="text-[11px] font-medium text-slate-500">Android 系统导航快捷键</span>
            <div class="grid grid-cols-3 gap-2">
              <Button
                variant="secondary"
                size="sm"
                class="h-10 rounded-2xl flex items-center justify-center gap-2 border border-slate-200/60 dark:border-white/10 hover:bg-blue-50 dark:hover:bg-blue-950/30 cursor-pointer"
                :disabled="selectedSerials.length === 0 || batchRunning"
                @click="emit('triggerBatchAction', 'back')"
              >
                <ArrowLeft class="h-4 w-4 text-blue-600 dark:text-blue-400" />
                <span class="text-xs font-semibold">返回</span>
              </Button>
              <Button
                variant="secondary"
                size="sm"
                class="h-10 rounded-2xl flex items-center justify-center gap-2 border border-slate-200/60 dark:border-white/10 hover:bg-blue-50 dark:hover:bg-blue-950/30 cursor-pointer"
                :disabled="selectedSerials.length === 0 || batchRunning"
                @click="emit('triggerBatchAction', 'home')"
              >
                <Home class="h-4 w-4 text-blue-600 dark:text-blue-400" />
                <span class="text-xs font-semibold">主页</span>
              </Button>
              <Button
                variant="secondary"
                size="sm"
                class="h-10 rounded-2xl flex items-center justify-center gap-2 border border-slate-200/60 dark:border-white/10 hover:bg-blue-50 dark:hover:bg-blue-950/30 cursor-pointer"
                :disabled="selectedSerials.length === 0 || batchRunning"
                @click="emit('triggerBatchAction', 'app_switch')"
              >
                <Layers class="h-4 w-4 text-blue-600 dark:text-blue-400" />
                <span class="text-xs font-semibold">多任务</span>
              </Button>
            </div>
          </div>

          <!-- 实时同步操作卡片 -->
          <div
            class="flex items-center justify-between rounded-2xl border p-3 transition-colors cursor-pointer"
            :class="isSyncActive
              ? 'border-blue-500/60 bg-blue-500/10 dark:bg-blue-950/30'
              : 'border-slate-200/80 bg-slate-50/70 dark:border-white/10 dark:bg-black/20'"
            @click="emit('toggleSync')"
          >
            <div class="flex items-center gap-3">
              <div
                class="flex h-9 w-9 items-center justify-center rounded-xl transition-all"
                :class="isSyncActive
                  ? 'bg-blue-600 text-white shadow-md shadow-blue-500/30 ring-2 ring-blue-400'
                  : 'bg-slate-200/70 text-slate-500 dark:bg-white/10 dark:text-slate-400'"
              >
                <Radio class="h-4 w-4" :class="{ 'animate-pulse': isSyncActive }" />
              </div>
              <div>
                <div class="text-xs font-bold text-slate-800 dark:text-slate-100 flex items-center gap-1.5">
                  <span>实时同步操作</span>
                  <span
                    v-if="isSyncActive"
                    class="rounded-full bg-blue-600/20 px-2 py-0.5 text-[10px] font-semibold text-blue-600 dark:text-blue-300"
                  >
                    已激活
                  </span>
                </div>
                <div class="text-[10px] text-slate-400">操作主控设备，实时同步派发至已选手机</div>
              </div>
            </div>
            <button
              type="button"
              class="relative h-6 w-11 rounded-full transition-colors focus:outline-hidden cursor-pointer"
              :class="isSyncActive ? 'bg-blue-600' : 'bg-slate-300 dark:bg-slate-700'"
            >
              <span
                class="inline-block h-5 w-5 transform rounded-full bg-white shadow-md transition-transform"
                :class="isSyncActive ? 'translate-x-5.5' : 'translate-x-0.5'"
              />
            </button>
          </div>
        </div>

        <!-- 2. 视图面板内容 -->
        <div v-else-if="activeTab === 'view'" class="space-y-4 pt-3.5">
          <!-- 视图模式切换 -->
          <div class="flex items-center justify-between">
            <span class="text-xs font-medium text-slate-600 dark:text-slate-400">视图模式</span>
            <div class="flex rounded-2xl border border-slate-200/80 bg-slate-100/90 p-1 dark:border-white/10 dark:bg-black/30">
              <button
                type="button"
                class="flex items-center gap-1.5 rounded-xl px-3 py-1.5 text-xs font-medium transition-all cursor-pointer"
                :class="viewMode === 'deck'
                  ? 'bg-white font-semibold text-slate-950 shadow-sm dark:bg-white/20 dark:text-white'
                  : 'text-slate-500 hover:text-slate-900 dark:text-slate-400 dark:hover:text-white'"
                @click="emit('update:viewMode', 'deck')"
              >
                <LayoutGrid class="h-3.5 w-3.5" />
                <span>卡片视图</span>
              </button>
              <button
                type="button"
                class="flex items-center gap-1.5 rounded-xl px-3 py-1.5 text-xs font-medium transition-all cursor-pointer"
                :class="viewMode === 'phone'
                  ? 'bg-white font-semibold text-slate-950 shadow-sm dark:bg-white/20 dark:text-white'
                  : 'text-slate-500 hover:text-slate-900 dark:text-slate-400 dark:hover:text-white'"
                @click="emit('update:viewMode', 'phone')"
              >
                <Smartphone class="h-3.5 w-3.5" />
                <span>手机视图</span>
              </button>
            </div>
          </div>

          <!-- 手机大小滑块 -->
          <div class="space-y-1.5 rounded-2xl border border-slate-200/70 bg-slate-50/60 p-3 dark:border-white/10 dark:bg-black/20">
            <div class="flex items-center justify-between text-xs">
              <span class="font-medium text-slate-700 dark:text-slate-300">卡片尺寸缩放</span>
              <span class="font-mono text-[11px] font-semibold text-blue-600 dark:text-blue-400">
                {{ phoneScale }}% · {{ cardWidth }}px
              </span>
            </div>
            <Slider
              :model-value="[phoneScale]"
              :min="70"
              :max="140"
              :step="5"
              @update:model-value="handleRangeUpdate($event, val => emit('update:phoneScale', val))"
            />
            <div class="flex justify-between text-[10px] text-slate-400 pt-1">
              <button type="button" class="cursor-pointer hover:text-blue-600" @click="emit('update:phoneScale', 75)">紧凑 (75%)</button>
              <button type="button" class="cursor-pointer hover:text-blue-600" @click="emit('update:phoneScale', 100)">标准 (100%)</button>
              <button type="button" class="cursor-pointer hover:text-blue-600" @click="emit('update:phoneScale', 130)">超大 (130%)</button>
            </div>
          </div>

          <!-- 投屏码率滑块 -->
          <div class="space-y-1.5 rounded-2xl border border-slate-200/70 bg-slate-50/60 p-3 dark:border-white/10 dark:bg-black/20">
            <div class="flex items-center justify-between text-xs">
              <span class="font-medium text-slate-700 dark:text-slate-300">投屏码率</span>
              <span class="font-mono text-[11px] font-semibold text-blue-600 dark:text-blue-400">
                {{ bitrateMbps }} Mbps
              </span>
            </div>
            <Slider
              :model-value="[bitrateMbps]"
              :min="2"
              :max="16"
              :step="1"
              @update:model-value="handleRangeUpdate($event, val => emit('update:bitrateMbps', val))"
            />
            <div class="flex justify-between text-[10px] text-slate-400 pt-1">
              <button type="button" class="cursor-pointer hover:text-blue-600" @click="emit('update:bitrateMbps', 2)">流畅 (2M)</button>
              <button type="button" class="cursor-pointer hover:text-blue-600" @click="emit('update:bitrateMbps', 8)">超清 (8M)</button>
              <button type="button" class="cursor-pointer hover:text-blue-600" @click="emit('update:bitrateMbps', 16)">极致 (16M)</button>
            </div>
          </div>

          <!-- 投屏画质滑块 -->
          <div class="space-y-1.5 rounded-2xl border border-slate-200/70 bg-slate-50/60 p-3 dark:border-white/10 dark:bg-black/20">
            <div class="flex items-center justify-between text-xs">
              <span class="font-medium text-slate-700 dark:text-slate-300">投屏质量</span>
              <span class="font-mono text-[11px] font-semibold text-blue-600 dark:text-blue-400">
                {{ qualityPercent }}% · {{ qualityLabel }}
              </span>
            </div>
            <Slider
              :model-value="[qualityPercent]"
              :min="50"
              :max="100"
              :step="5"
              @update:model-value="handleRangeUpdate($event, val => emit('update:qualityPercent', val))"
            />
            <div class="flex justify-between text-[10px] text-slate-400 pt-1">
              <button type="button" class="cursor-pointer hover:text-blue-600" @click="emit('update:qualityPercent', 60)">720P</button>
              <button type="button" class="cursor-pointer hover:text-blue-600" @click="emit('update:qualityPercent', 80)">1080P</button>
              <button type="button" class="cursor-pointer hover:text-blue-600" @click="emit('update:qualityPercent', 100)">100% 原画</button>
            </div>
          </div>
        </div>

        <!-- 3. 设备管理面板（搜索 + 添加设备 + 在线/已选统计 + 01..N 方块支持鼠标拖拽框选） -->
        <div v-else-if="activeTab === 'devices'" class="space-y-3.5 pt-3.5">
          <!-- 搜索与添加设备栏 -->
          <div class="flex items-center gap-2">
            <div class="relative flex-1">
              <Search class="absolute left-3 top-1/2 h-3.5 w-3.5 -translate-y-1/2 text-slate-400" />
              <Input
                ref="searchInputRef"
                :model-value="keyword"
                placeholder="搜索设备名称、型号或序列号..."
                class="h-9 rounded-2xl border-slate-200/80 bg-slate-50/80 pl-8.5 pr-8 text-xs dark:border-white/10 dark:bg-black/20"
                @update:model-value="emit('update:keyword', String($event))"
              />
              <button
                v-if="keyword"
                type="button"
                class="absolute right-2.5 top-1/2 -translate-y-1/2 rounded-full p-0.5 text-slate-400 hover:text-slate-700 dark:hover:text-white cursor-pointer"
                title="清除搜索"
                @click="emit('update:keyword', '')"
              >
                <X class="h-3 w-3" />
              </button>
            </div>
            <Button
              size="sm"
              class="h-9 shrink-0 rounded-2xl bg-blue-600 px-3 text-xs text-white shadow-xs hover:bg-blue-700 cursor-pointer"
              @click="emit('openAdd')"
            >
              <Plus class="mr-1 h-3.5 w-3.5" />
              添加设备
            </Button>
          </div>

          <!-- 在线数量与已选数量指示条 -->
          <div class="flex items-center justify-between text-xs px-1">
            <span class="text-xs font-semibold text-slate-700 dark:text-slate-300">设备编号矩阵</span>
            <div class="flex items-center gap-2">
              <Badge variant="outline" class="h-5 px-2 font-mono text-[10px] text-emerald-600 dark:text-emerald-400 border-emerald-500/30 bg-emerald-500/10">
                {{ onlineCount }} 在线
              </Badge>
              <Badge variant="secondary" class="h-5 px-2 font-mono text-[10px] text-blue-600 dark:text-blue-400 bg-blue-500/10 border border-blue-500/20">
                {{ selectedSerials.length }} 已选
              </Badge>
            </div>
          </div>

          <!-- 设备编号小方格网格 (01..N, 包含离线设备，支持鼠标框选) -->
          <div
            ref="gridContainerRef"
            class="relative grid grid-cols-6 gap-2 rounded-2xl border border-slate-200/80 bg-slate-50/70 p-2.5 dark:border-white/10 dark:bg-black/30 select-none overflow-hidden"
            @mousedown="handleGridMouseDown"
          >
            <!-- 框选半透明蓝色高亮选框 (Marquee Box) -->
            <div
              v-if="marqueeBox"
              class="absolute pointer-events-none rounded-md border border-blue-500 bg-blue-500/20 z-20"
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
              class="relative flex h-10 items-center justify-center rounded-xl border font-mono text-xs font-bold transition-all cursor-pointer select-none"
              :class="selectedSet.has(item.serial)
                ? 'border-2 border-blue-500 bg-blue-600 text-white shadow-sm ring-2 ring-blue-500/30'
                : item.device.state !== 'device'
                  ? 'border-dashed border-slate-300 bg-slate-200/50 text-slate-400 dark:border-white/10 dark:bg-white/5'
                  : 'border-slate-300/80 bg-white text-slate-800 hover:border-blue-400 dark:border-white/20 dark:bg-white/10 dark:text-slate-100'"
              :title="`${item.label} · ${item.device.alias || item.device.serial} (${item.device.state === 'device' ? '在线' : '离线'})`"
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
              <span class="font-mono font-semibold text-slate-800 dark:text-slate-200">MyWebScrcpy v1.31.0</span>
            </div>
            <div class="flex items-center justify-between">
              <span class="text-slate-500 dark:text-slate-400">ADB 服务</span>
              <span class="text-emerald-600 dark:text-emerald-400 font-semibold flex items-center gap-1">
                <span class="h-1.5 w-1.5 rounded-full bg-emerald-500" />
                运行中
              </span>
            </div>
          </div>
        </div>
      </div>
    </Transition>

    <!-- iOS 底部悬浮玻璃操作栏 (Floating Island / Dock Bar) -->
    <div
      v-if="!isDockMinimized"
      class="pointer-events-auto flex items-center gap-1 rounded-full border border-white/60 bg-white/75 p-1.5 shadow-[0_12px_40px_rgba(0,0,0,0.12)] backdrop-blur-2xl backdrop-saturate-150 transition-all dark:border-white/15 dark:bg-slate-900/75 dark:shadow-[0_16px_50px_rgba(0,0,0,0.5)] ring-1 ring-black/5 dark:ring-white/10"
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
      class="pointer-events-auto flex items-center gap-2 rounded-full border border-white/60 bg-white/80 px-4 py-2 shadow-xl backdrop-blur-2xl transition-all hover:scale-105 hover:bg-white dark:border-white/15 dark:bg-slate-900/80 dark:hover:bg-slate-900 cursor-pointer text-slate-800 dark:text-white ring-1 ring-black/5"
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
