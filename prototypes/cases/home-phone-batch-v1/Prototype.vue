<script setup lang="ts">
import {
  CheckSquare,
  ChevronLeft,
  ChevronRight,
  EyeOff,
  Grid3X3,
  Home,
  Layers,
  LayoutGrid,
  Loader2,
  PanelRightClose,
  PanelRightOpen,
  Power,
  Search,
  SlidersHorizontal,
  Smartphone,
  Square,
  Usb,
  Wifi,
  X,
} from 'lucide-vue-next'
import { computed, ref, watch } from 'vue'
import { Badge } from '@/components/ui/badge'
import { Button } from '@/components/ui/button'
import { Input } from '@/components/ui/input'
import { initialMockDevices, type MockDevice } from './mockDevices'

type ViewMode = 'deck' | 'compact' | 'phone'
type DashSize = 'small' | 'medium' | 'large' | 'fit'

interface BatchItemResult {
  serial: string
  name: string
  status: 'waiting' | 'running' | 'success' | 'failed' | 'timeout' | 'cancelled'
  error?: string
  elapsedMs?: number
}

// 模拟设备数据
const devices = ref<MockDevice[]>([...initialMockDevices])
const keyword = ref('')
const sourceFilter = ref<'all' | 'auto' | 'manual'>('all')
const showOffline = ref<boolean>(false)
const viewMode = ref<ViewMode>('phone')

// 手机视图设置
const pageSize = ref<number>(9)
const currentPage = ref<number>(1)
const dashSize = ref<DashSize>('small')
const hiddenSerials = ref<string[]>([])

// 批量面板折叠状态（桌面与窄屏通用浮动面板，无模态遮罩）
const isPanelOpen = ref(false)

// 选择状态：三种视图共享同一选择集合
const selectedSerials = ref<string[]>([])

// 批量执行状态
const batchRunning = ref(false)
const currentActionName = ref('')
const batchResults = ref<BatchItemResult[]>([])
let batchAbortController: AbortController | null = null

// 过滤后的设备列表
const filteredDevices = computed(() => {
  return devices.value.filter((d) => {
    // 默认仅在线设备
    if (!showOffline.value && d.state !== 'device')
      return false
    if (sourceFilter.value !== 'all' && d.source !== sourceFilter.value)
      return false
    if (!keyword.value.trim())
      return true
    const kw = keyword.value.trim().toLowerCase()
    return (
      d.serial.toLowerCase().includes(kw)
      || d.model.toLowerCase().includes(kw)
      || (d.marketName && d.marketName.toLowerCase().includes(kw))
    )
  })
})

// 仅在当前过滤结果中过滤掉隐藏设备的设备（用于手机视图计算）
const phoneVisibleDevices = computed(() => {
  return filteredDevices.value.filter(d => !hiddenSerials.value.includes(d.serial))
})

// 手机视图当前页设备
const totalPages = computed(() => {
  const count = phoneVisibleDevices.value.length
  return count > 0 ? Math.ceil(count / pageSize.value) : 1
})

// 页码合法性保护
watch(totalPages, (maxP) => {
  if (currentPage.value > maxP)
    currentPage.value = maxP
  if (currentPage.value < 1)
    currentPage.value = 1
})

// 翻页、筛选或切换每页台数时清空选择，避免操作隐藏设备
watch([currentPage, sourceFilter, showOffline, pageSize, viewMode], () => {
  selectedSerials.value = []
})

const pagedDevices = computed(() => {
  if (viewMode.value !== 'phone')
    return filteredDevices.value
  const start = (currentPage.value - 1) * pageSize.value
  return phoneVisibleDevices.value.slice(start, start + pageSize.value)
})

// 可选设备列表（仅当前页在线正常设备）
const selectableDevices = computed(() => {
  const list = viewMode.value === 'phone' ? pagedDevices.value : filteredDevices.value
  return list.filter(d => d.state === 'device')
})

const isAllSelected = computed(() => {
  if (selectableDevices.value.length === 0)
    return false
  return selectableDevices.value.every(d => selectedSerials.value.includes(d.serial))
})

function toggleSelect(serial: string) {
  if (selectedSerials.value.includes(serial)) {
    selectedSerials.value = selectedSerials.value.filter(s => s !== serial)
  }
  else {
    selectedSerials.value = [...selectedSerials.value, serial]
  }
}

function selectAll() {
  if (isAllSelected.value) {
    selectedSerials.value = []
  }
  else {
    selectedSerials.value = selectableDevices.value.map(d => d.serial)
  }
}

function clearSelection() {
  selectedSerials.value = []
}

// 手机视图显隐控制
function toggleHide(serial: string) {
  if (hiddenSerials.value.includes(serial)) {
    hiddenSerials.value = hiddenSerials.value.filter(s => s !== serial)
  }
  else {
    hiddenSerials.value = [...hiddenSerials.value, serial]
    selectedSerials.value = selectedSerials.value.filter(s => s !== serial)
  }
}

function showAll() {
  hiddenSerials.value = []
}

// 模拟批量派发：有界并发 4，每台独立结果
async function triggerBatchAction(actionName: 'back' | 'home' | 'app_switch') {
  if (selectedSerials.value.length === 0 || batchRunning.value)
    return

  batchRunning.value = true
  const labels: Record<string, string> = {
    back: '批量返回',
    home: '批量回到主页',
    app_switch: '批量最近任务',
  }
  currentActionName.value = labels[actionName] || actionName

  // 冻结目标快照
  const targets = [...selectedSerials.value]
  const initialList: BatchItemResult[] = targets.map((s) => {
    const dev = devices.value.find(d => d.serial === s)
    return {
      serial: s,
      name: dev?.marketName || dev?.model || s,
      status: 'waiting',
    }
  })
  batchResults.value = initialList
  batchAbortController = new AbortController()
  const signal = batchAbortController.signal

  const concurrency = 4
  let currentIndex = 0

  async function worker() {
    while (currentIndex < targets.length) {
      if (signal.aborted)
        break
      const idx = currentIndex++
      const targetSerial = targets[idx]
      const item = batchResults.value.find(r => r.serial === targetSerial)
      if (!item)
        continue

      item.status = 'running'
      const start = Date.now()
      // 模拟 150~600ms 的设备执行耗时，特定设备模拟错误以验证逐台结果展示
      const delay = 180 + Math.random() * 300
      await new Promise(r => setTimeout(r, delay))

      if (signal.aborted) {
        item.status = 'cancelled'
        item.error = '操作已终止'
        break
      }

      item.elapsedMs = Date.now() - start
      // 模拟末尾或者特定设备偶发失败
      if (targetSerial.endsWith('.107:5555')) {
        item.status = 'failed'
        item.error = '目标设备执行超时(15s)'
      }
      else {
        item.status = 'success'
      }
    }
  }

  const workers = Array.from({ length: Math.min(concurrency, targets.length) }, () => worker())
  await Promise.all(workers)

  batchRunning.value = false
  batchAbortController = null
}

function cancelBatch() {
  if (batchAbortController) {
    batchAbortController.abort()
  }
  batchRunning.value = false
}

// 预设每页台数
const presetPageSizes = [4, 9, 16, 25, 36]
function setPageSize(size: number) {
  pageSize.value = Math.max(1, Math.min(64, size))
  currentPage.value = 1
}

const successCount = computed(() => batchResults.value.filter(r => r.status === 'success').length)
const failedCount = computed(() => batchResults.value.filter(r => r.status === 'failed').length)
</script>

<template>
  <div class="min-h-screen bg-slate-950 text-slate-100 flex flex-col font-sans select-none">
    <!-- 顶部模拟导航与视图控制栏 -->
    <header class="border-b border-white/10 bg-slate-900/80 px-4 py-2.5 backdrop-blur-md flex flex-wrap items-center justify-between gap-3 shrink-0">
      <div class="flex items-center gap-3">
        <div class="flex h-9 w-9 items-center justify-center rounded-xl bg-gradient-to-tr from-cyan-500 to-indigo-600 text-white font-bold shadow-md shadow-indigo-500/20">
          <Smartphone class="h-5 w-5" />
        </div>
        <div>
          <div class="flex items-center gap-2">
            <span class="text-sm font-bold text-white">设备列表</span>
            <Badge variant="outline" class="border-emerald-500/40 bg-emerald-500/10 text-[10px] text-emerald-400">
              {{ devices.filter(d => d.state === 'device').length }} 在线
            </Badge>
          </div>
          <p class="text-[10px] text-slate-400">v1.32 原型验证 · 首页手机视图与右侧批量操作面板</p>
        </div>
      </div>

      <!-- 搜索与筛选 -->
      <div class="flex items-center gap-2 flex-1 max-w-md">
        <div class="relative flex-1">
          <Search class="absolute left-2.5 top-1/2 -translate-y-1/2 h-3.5 w-3.5 text-slate-400" />
          <Input
            v-model="keyword"
            placeholder="搜索设备名称、型号或序列号..."
            class="h-8 pl-8 text-xs bg-slate-900/90 border-white/10 text-slate-200 placeholder:text-slate-500 rounded-lg"
          />
        </div>
        <div class="flex items-center rounded-lg border border-white/10 bg-white/5 p-0.5 text-xs">
          <button
            type="button"
            class="px-2.5 py-1 rounded font-medium transition-colors"
            :class="sourceFilter === 'all' ? 'bg-white/15 text-white' : 'text-slate-400 hover:text-white'"
            @click="sourceFilter = 'all'"
          >
            全部
          </button>
          <button
            type="button"
            class="flex items-center gap-1 px-2 py-1 rounded font-medium transition-colors"
            :class="sourceFilter === 'auto' ? 'bg-white/15 text-white' : 'text-slate-400 hover:text-white'"
            @click="sourceFilter = 'auto'"
          >
            <Usb class="h-3 w-3 text-cyan-400" />直连
          </button>
          <button
            type="button"
            class="flex items-center gap-1 px-2 py-1 rounded font-medium transition-colors"
            :class="sourceFilter === 'manual' ? 'bg-white/15 text-white' : 'text-slate-400 hover:text-white'"
            @click="sourceFilter = 'manual'"
          >
            <Wifi class="h-3 w-3 text-indigo-400" />无线
          </button>
        </div>

        <!-- 显示离线设备开关 -->
        <label class="flex items-center gap-1.5 px-2 py-1 text-xs text-slate-400 hover:text-white cursor-pointer select-none">
          <input
            v-model="showOffline"
            type="checkbox"
            class="rounded border-white/20 bg-white/10 text-indigo-600 focus:ring-0 cursor-pointer"
          />
          <span>显示离线</span>
        </label>
      </div>

      <!-- 三种视图切换与面板开关 -->
      <div class="flex items-center gap-2">
        <div class="flex items-center rounded-lg border border-white/10 bg-white/5 p-0.5 text-xs">
          <button
            type="button"
            class="flex items-center gap-1 px-2.5 py-1 rounded transition-colors"
            :class="viewMode === 'deck' ? 'bg-white/20 text-white font-semibold shadow-xs' : 'text-slate-400 hover:text-white'"
            title="3D 全息甲板"
            @click="viewMode = 'deck'"
          >
            <LayoutGrid class="h-3.5 w-3.5" />甲板
          </button>
          <button
            type="button"
            class="flex items-center gap-1 px-2.5 py-1 rounded transition-colors"
            :class="viewMode === 'compact' ? 'bg-white/20 text-white font-semibold shadow-xs' : 'text-slate-400 hover:text-white'"
            title="紧凑阵列"
            @click="viewMode = 'compact'"
          >
            <Grid3X3 class="h-3.5 w-3.5" />紧凑
          </button>
          <button
            type="button"
            class="flex items-center gap-1 px-2.5 py-1 rounded transition-colors"
            :class="viewMode === 'phone' ? 'bg-indigo-600/80 text-white font-semibold shadow-xs' : 'text-slate-400 hover:text-white'"
            title="手机视图（大屏整合）"
            @click="viewMode = 'phone'"
          >
            <Smartphone class="h-3.5 w-3.5" />手机视图
          </button>
        </div>

        <!-- 批量操作面板显隐按钮 -->
        <Button
          variant="outline"
          size="sm"
          class="h-8 border-white/10 bg-white/5 text-xs text-slate-300 hover:bg-white/10 hover:text-white flex items-center gap-1.5"
          :class="{ '!border-indigo-500 !text-indigo-400': isPanelOpen }"
          @click="isPanelOpen = !isPanelOpen"
        >
          <component :is="isPanelOpen ? PanelRightClose : PanelRightOpen" class="h-3.5 w-3.5" />
          批量操作
          <Badge v-if="selectedSerials.length > 0" class="h-4 px-1 text-[10px] bg-indigo-600 text-white leading-none">
            {{ selectedSerials.length }}
          </Badge>
        </Button>
      </div>
    </header>

    <!-- 主工作区：左侧视图展示，右侧批量操作面板 -->
    <div class="flex-1 flex overflow-hidden">
      <!-- 左侧内容区 -->
      <main class="flex-1 flex flex-col min-w-0 overflow-y-auto p-4 space-y-4">
        <!-- 手机视图专属：分页与每页台数控制栏 -->
        <div
          v-if="viewMode === 'phone'"
          class="flex flex-wrap items-center justify-between gap-3 rounded-2xl border border-white/10 bg-white/[0.03] p-3 text-xs"
        >
          <div class="flex items-center gap-2">
            <span class="text-slate-400">每页台数:</span>
            <div class="flex items-center gap-1">
              <button
                v-for="s in presetPageSizes"
                :key="s"
                type="button"
                class="h-6 w-7 rounded border border-white/10 text-[11px] font-medium transition-colors"
                :class="pageSize === s ? 'bg-indigo-600 text-white border-indigo-500' : 'bg-white/5 text-slate-300 hover:bg-white/10'"
                @click="setPageSize(s)"
              >
                {{ s }}
              </button>
            </div>
            <span class="text-slate-500 text-[11px] ml-1">自定义:</span>
            <input
              type="number"
              min="1"
              max="64"
              :value="pageSize"
              class="h-6 w-12 rounded border border-white/10 bg-black/40 text-center font-mono text-[11px] text-white"
              @change="setPageSize(Number(($event.target as HTMLInputElement).value))"
            />
          </div>

          <!-- 分页器 -->
          <div class="flex items-center gap-2">
            <span class="text-slate-400 text-[11px]">
              第 {{ currentPage }} / {{ totalPages }} 页 (共 {{ phoneVisibleDevices.length }} 台可见)
            </span>
            <div class="flex items-center gap-1">
              <Button
                variant="ghost"
                size="icon"
                class="h-6 w-6 rounded border border-white/10 text-slate-300 disabled:opacity-30"
                :disabled="currentPage <= 1"
                @click="currentPage--"
              >
                <ChevronLeft class="h-3 w-3" />
              </Button>
              <Button
                variant="ghost"
                size="icon"
                class="h-6 w-6 rounded border border-white/10 text-slate-300 disabled:opacity-30"
                :disabled="currentPage >= totalPages"
                @click="currentPage++"
              >
                <ChevronRight class="h-3 w-3" />
              </Button>
            </div>
          </div>
        </div>

        <!-- 手机视图 (Phone View) 模拟 -->
        <div
          v-if="viewMode === 'phone'"
          class="flex-1"
        >
          <div
            v-if="pagedDevices.length === 0"
            class="flex flex-col items-center justify-center p-12 text-slate-400 space-y-2 border border-dashed border-white/10 rounded-2xl"
          >
            <Smartphone class="h-10 w-10 text-slate-500" />
            <p class="text-sm">当前无符合条件的设备显示</p>
            <Button v-if="hiddenSerials.length > 0" variant="outline" size="sm" class="text-xs" @click="showAll">
              恢复所有隐藏设备 ({{ hiddenSerials.length }} 台已隐藏)
            </Button>
          </div>

          <div
            v-else
            class="flex flex-wrap items-start gap-4"
          >
            <!-- 手机样式外框卡片 -->
            <div
              v-for="d in pagedDevices"
              :key="d.serial"
              class="group relative flex flex-col rounded-3xl border border-white/15 bg-slate-900/90 shadow-xl transition-all duration-200 overflow-hidden"
              :class="[
                dashSize === 'small' ? 'w-52' : dashSize === 'medium' ? 'w-72' : 'w-96',
                selectedSerials.includes(d.serial) ? 'ring-2 ring-indigo-500 border-indigo-400' : 'hover:border-white/30',
              ]"
            >
              <!-- 手机顶栏：选择勾选框、设备名称、状态指示灯 -->
              <div class="flex items-center justify-between border-b border-white/10 bg-slate-800/80 px-2.5 py-1.5 text-xs shrink-0">
                <div class="flex items-center gap-1.5 min-w-0">
                  <!-- 独立选择框：点击不触发手机触控 -->
                  <button
                    type="button"
                    class="h-4 w-4 rounded flex items-center justify-center text-white shrink-0 transition-colors"
                    :class="selectedSerials.includes(d.serial) ? 'text-indigo-400' : 'text-slate-500 hover:text-slate-300'"
                    :title="selectedSerials.includes(d.serial) ? '取消选中' : '选择设备'"
                    @click.stop="toggleSelect(d.serial)"
                  >
                    <component :is="selectedSerials.includes(d.serial) ? CheckSquare : Square" class="h-3.5 w-3.5" />
                  </button>
                  <span class="truncate font-medium text-slate-200 text-[11px]">
                    {{ d.marketName || d.model }}
                  </span>
                </div>
                <div class="flex items-center gap-1.5 shrink-0">
                  <span
                    class="h-2 w-2 rounded-full"
                    :class="d.state === 'device' ? 'bg-emerald-500' : d.state === 'offline' ? 'bg-slate-500' : 'bg-amber-500'"
                  />
                  <button
                    type="button"
                    class="text-slate-400 hover:text-white p-0.5"
                    title="在手机视图中隐藏"
                    @click.stop="toggleHide(d.serial)"
                  >
                    <EyeOff class="h-3 w-3" />
                  </button>
                </div>
              </div>

              <!-- 手机屏幕区域：真实 9:19.5 宽高比模拟 -->
              <div class="relative bg-black flex items-center justify-center aspect-[9/19.5] w-full overflow-hidden select-none">
                <!-- 模拟画面内容 -->
                <div v-if="d.state === 'device'" class="absolute inset-0 p-3 flex flex-col justify-between text-slate-400 text-[10px]">
                  <div class="flex justify-between items-center text-[9px] text-slate-500">
                    <span>12:45</span>
                    <div class="flex items-center gap-1 font-mono">
                      <span>{{ d.batteryLevel }}%</span>
                      <div class="h-2 w-3 rounded-2xs border border-slate-500" />
                    </div>
                  </div>

                  <!-- 模拟应用界面 -->
                  <div class="my-auto space-y-2 text-center">
                    <div class="h-10 w-10 mx-auto rounded-2xl bg-gradient-to-tr from-indigo-500 to-purple-600 flex items-center justify-center text-white shadow-md">
                      <Smartphone class="h-5 w-5" />
                    </div>
                    <div class="text-[11px] font-semibold text-slate-300">{{ d.model }}</div>
                    <div class="text-[9px] font-mono text-slate-500 truncate px-2">{{ d.serial }}</div>
                  </div>

                  <div class="text-center text-[9px] text-slate-500">
                    单机触控与按键可用
                  </div>
                </div>

                <!-- 离线/异常状态 -->
                <div v-else class="text-center p-4 space-y-1">
                  <span class="text-xs text-amber-400 font-medium">
                    {{ d.state === 'offline' ? '设备已离线' : '未授权 ADB' }}
                  </span>
                  <p class="text-[9px] text-slate-500">仅在线设备订阅实时投屏</p>
                </div>
              </div>

              <!-- 手机底部大屏按键迁移：返回、主页、最近任务、电源 -->
              <div class="flex items-center justify-around border-t border-white/10 bg-slate-900/90 py-1 px-2 text-slate-400">
                <button type="button" class="p-1 hover:text-white rounded" title="返回键">
                  <ChevronLeft class="h-3.5 w-3.5" />
                </button>
                <button type="button" class="p-1 hover:text-white rounded" title="主页键">
                  <Home class="h-3.5 w-3.5" />
                </button>
                <button type="button" class="p-1 hover:text-white rounded" title="任务键">
                  <Layers class="h-3.5 w-3.5" />
                </button>
                <button type="button" class="p-1 hover:text-amber-400 rounded" title="单机电源">
                  <Power class="h-3.5 w-3.5" />
                </button>
              </div>
            </div>
          </div>
        </div>

        <!-- 3D 甲板视图与紧凑视图保留（在此原型中示意共享多选） -->
        <div v-else class="grid grid-cols-1 md:grid-cols-2 xl:grid-cols-3 gap-4">
          <div
            v-for="d in filteredDevices"
            :key="d.serial"
            class="rounded-2xl border border-white/10 bg-slate-900/60 p-4 space-y-3"
            :class="{ 'ring-2 ring-indigo-500': selectedSerials.includes(d.serial) }"
          >
            <div class="flex items-center justify-between">
              <div class="flex items-center gap-2">
                <button
                  type="button"
                  class="h-4 w-4 rounded flex items-center justify-center"
                  :class="selectedSerials.includes(d.serial) ? 'text-indigo-400' : 'text-slate-500'"
                  @click="toggleSelect(d.serial)"
                >
                  <component :is="selectedSerials.includes(d.serial) ? CheckSquare : Square" class="h-4 w-4" />
                </button>
                <span class="font-bold text-sm text-white">{{ d.marketName || d.model }}</span>
              </div>
              <Badge variant="outline" class="text-[10px]">{{ d.state }}</Badge>
            </div>
            <div class="text-xs text-slate-400 font-mono">{{ d.serial }}</div>
            <div class="text-[11px] text-slate-500">（{{ viewMode === 'deck' ? '3D 甲板模式' : '紧凑阵列模式' }}下共享本页选择）</div>
          </div>
        </div>
      </main>

      <!-- 批量操作面板（浮动覆盖层，不占布局宽度，不使设备区域变形，外部完全可交互） -->
      <Transition
        enter-active-class="transition duration-200 ease-out"
        enter-from-class="opacity-0 translate-y-2 sm:translate-y-0 sm:translate-x-4 scale-95"
        enter-to-class="opacity-100 translate-y-0 sm:translate-x-0 scale-100"
        leave-active-class="transition duration-150 ease-in"
        leave-from-class="opacity-100 translate-y-0 sm:translate-x-0 scale-100"
        leave-to-class="opacity-0 translate-y-2 sm:translate-y-0 sm:translate-x-4 scale-95"
      >
        <section
          v-if="isPanelOpen"
          aria-label="原型批量操作面板"
          class="fixed z-50 right-3 sm:right-6 top-16 w-[calc(100vw-1.5rem)] sm:w-84 max-w-sm max-h-[calc(100vh-5rem)] flex flex-col rounded-3xl border border-white/15 bg-slate-900/95 backdrop-blur-3xl shadow-2xl overflow-hidden select-none"
        >
          <!-- 面板头部 -->
          <div class="flex items-center justify-between border-b border-white/10 px-4 py-3 shrink-0">
            <div class="flex items-center gap-2">
              <SlidersHorizontal class="h-4 w-4 text-indigo-400" />
              <span class="font-bold text-sm text-white">批量操作面板</span>
            </div>
            <button
              type="button"
              class="text-slate-400 hover:text-white p-1 rounded-lg cursor-pointer"
              title="收起面板 (Esc)"
              @click="isPanelOpen = false"
            >
              <X class="h-4 w-4" />
            </button>
          </div>

          <!-- 可滚动内容区 -->
          <div class="p-4 space-y-4 overflow-y-auto flex-1 overscroll-contain">
            <!-- 分区一：目标选择 -->
            <div class="space-y-2.5">
              <div class="flex items-center justify-between text-xs">
                <span class="text-slate-400 font-medium">目标设备范围</span>
                <span class="font-semibold text-indigo-400">已选 {{ selectedSerials.length }} 台</span>
              </div>
              <div class="flex items-center gap-2">
                <Button
                  variant="outline"
                  size="sm"
                  class="flex-1 h-7 text-xs border-white/10 bg-white/5 hover:bg-white/10"
                  @click="selectAll"
                >
                  {{ isAllSelected ? '取消全选' : '全选本页' }}
                </Button>
                <Button
                  variant="outline"
                  size="sm"
                  class="flex-1 h-7 text-xs border-white/10 bg-white/5 hover:bg-white/10"
                  :disabled="selectedSerials.length === 0"
                  @click="clearSelection"
                >
                  清空选择
                </Button>
              </div>
              <p class="text-[10px] text-slate-500 leading-tight">
                说明：多选仅作用于当前页在线设备；翻页、更改筛选或切换视图将自动清空选择。
              </p>
            </div>

            <!-- 分区二：离散批量操作 -->
            <div class="space-y-2.5 border-t border-white/10 pt-3">
              <span class="text-xs text-slate-400 font-medium">批量基础按键</span>
              <div class="grid grid-cols-3 gap-2">
                <Button
                  variant="outline"
                  size="sm"
                  class="h-9 flex flex-col items-center justify-center gap-0.5 border-white/10 bg-white/5 hover:bg-white/10 disabled:opacity-40"
                  :disabled="selectedSerials.length === 0 || batchRunning"
                  @click="triggerBatchAction('back')"
                >
                  <ChevronLeft class="h-3.5 w-3.5" />
                  <span class="text-[10px]">返回</span>
                </Button>
                <Button
                  variant="outline"
                  size="sm"
                  class="h-9 flex flex-col items-center justify-center gap-0.5 border-white/10 bg-white/5 hover:bg-white/10 disabled:opacity-40"
                  :disabled="selectedSerials.length === 0 || batchRunning"
                  @click="triggerBatchAction('home')"
                >
                  <Home class="h-3.5 w-3.5" />
                  <span class="text-[10px]">主页</span>
                </Button>
                <Button
                  variant="outline"
                  size="sm"
                  class="h-9 flex flex-col items-center justify-center gap-0.5 border-white/10 bg-white/5 hover:bg-white/10 disabled:opacity-40"
                  :disabled="selectedSerials.length === 0 || batchRunning"
                  @click="triggerBatchAction('app_switch')"
                >
                  <Layers class="h-3.5 w-3.5" />
                  <span class="text-[10px]">最近任务</span>
                </Button>
              </div>

              <!-- 取消操作 -->
              <div v-if="batchRunning" class="flex items-center justify-between pt-1">
                <span class="text-xs text-indigo-400 flex items-center gap-1">
                  <Loader2 class="h-3 w-3 animate-spin" />
                  {{ currentActionName }}执行中...
                </span>
                <Button
                  variant="destructive"
                  size="sm"
                  class="h-6 text-[10px] px-2"
                  @click="cancelBatch"
                >
                  终止派发
                </Button>
              </div>
            </div>

            <!-- 分区三：显示设置分区（仅在手机视图起效） -->
            <div v-if="viewMode === 'phone'" class="space-y-2.5 border-t border-white/10 pt-3">
              <span class="text-xs text-slate-400 font-medium">手机显示分区</span>
              <div class="space-y-2 text-xs">
                <div class="flex items-center justify-between">
                  <span class="text-slate-400">尺寸档位</span>
                  <div class="flex items-center gap-1 rounded bg-white/5 p-0.5">
                    <button
                      v-for="sz in (['small', 'medium', 'large'] as DashSize[])"
                      :key="sz"
                      type="button"
                      class="px-2 py-0.5 rounded text-[10px] transition-colors"
                      :class="dashSize === sz ? 'bg-indigo-600 text-white font-medium' : 'text-slate-400 hover:text-white'"
                      @click="dashSize = sz"
                    >
                      {{ sz === 'small' ? '小' : sz === 'medium' ? '中' : '大' }}
                    </button>
                  </div>
                </div>
                <div class="flex items-center justify-between">
                  <span class="text-slate-400">设备显隐</span>
                  <div class="flex items-center gap-1">
                    <Button
                      variant="ghost"
                      size="sm"
                      class="h-6 px-1.5 text-[10px]"
                      :disabled="hiddenSerials.length === 0"
                      @click="showAll"
                    >
                      全部恢复
                    </Button>
                    <Badge v-if="hiddenSerials.length > 0" variant="secondary" class="h-4 px-1 text-[9px]">
                      {{ hiddenSerials.length }} 台隐藏
                    </Badge>
                  </div>
                </div>
              </div>
            </div>

            <!-- 分区四：当前批次执行结果 -->
            <div v-if="batchResults.length > 0" class="space-y-2 border-t border-white/10 pt-3">
              <div class="flex items-center justify-between text-xs">
                <span class="text-slate-400 font-medium">执行结果反馈</span>
                <div class="flex items-center gap-2 text-[10px]">
                  <span class="text-emerald-400">成功 {{ successCount }}</span>
                  <span v-if="failedCount > 0" class="text-rose-400">失败 {{ failedCount }}</span>
                </div>
              </div>

              <div class="max-h-48 overflow-y-auto space-y-1.5 pr-1 overscroll-contain">
                <div
                  v-for="r in batchResults"
                  :key="r.serial"
                  class="rounded-lg border border-white/5 bg-white/[0.02] p-2 text-xs flex items-center justify-between gap-2"
                >
                  <div class="min-w-0">
                    <div class="truncate text-[11px] font-medium text-slate-200">{{ r.name }}</div>
                    <div class="truncate font-mono text-[9px] text-slate-500">{{ r.serial }}</div>
                    <div v-if="r.error" class="text-[9px] text-rose-400 truncate">{{ r.error }}</div>
                  </div>
                  <div class="shrink-0 text-right">
                    <span
                      v-if="r.status === 'success'"
                      class="inline-flex items-center gap-1 text-[10px] text-emerald-400 font-medium"
                    >
                      成功
                      <span v-if="r.elapsedMs" class="font-mono text-[8px] text-slate-500">{{ r.elapsedMs }}ms</span>
                    </span>
                    <span v-else-if="r.status === 'failed'" class="text-[10px] text-rose-400 font-medium">
                      失败
                    </span>
                    <span v-else-if="r.status === 'running'" class="inline-flex items-center gap-1 text-[10px] text-indigo-400">
                      <Loader2 class="h-3 w-3 animate-spin" />
                      派发中
                    </span>
                    <span v-else-if="r.status === 'cancelled'" class="text-[10px] text-amber-400">
                      已终止
                    </span>
                    <span v-else class="text-[10px] text-slate-500">
                      排队中
                    </span>
                  </div>
                </div>
              </div>
            </div>
          </div>
        </section>
      </Transition>
    </div>
  </div>
</template>
