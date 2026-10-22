<script setup lang="ts">
import { computed, ref } from 'vue'
import {
  AppWindow,
  CheckSquare,
  ChevronLeft,
  EyeOff,
  Home,
  Loader2,
  Maximize,
  Minimize,
  Search,
  SlidersHorizontal,
  Smartphone,
  Square,
  X,
} from 'lucide-vue-next'
import { initialMockDevices, type MockDevice } from './mockDevices'

type DashSize = 'small' | 'medium' | 'large' | 'fit'

// 模拟设备数据源
const devices = ref<MockDevice[]>([...initialMockDevices])
const keyword = ref('')
const sourceFilter = ref<'all' | 'auto' | 'manual'>('all')
const showOffline = ref(false)
const isBatchPanelOpen = ref(true)
const isPageFullscreen = ref(false)
const dashSize = ref<DashSize>('medium')
const hiddenSerials = ref<string[]>([])
const selectedSerials = ref<string[]>([])

// 搜索与离线过滤
const filteredDevices = computed(() => {
  return devices.value.filter((d) => {
    if (!showOffline.value && d.state !== 'device') {
      return false
    }
    if (sourceFilter.value !== 'all' && d.source !== sourceFilter.value) {
      return false
    }
    if (keyword.value.trim()) {
      const q = keyword.value.trim().toLowerCase()
      const matchName = (d.marketName || '').toLowerCase().includes(q)
      const matchModel = (d.model || '').toLowerCase().includes(q)
      const matchSerial = d.serial.toLowerCase().includes(q)
      if (!matchName && !matchModel && !matchSerial) {
        return false
      }
    }
    return true
  })
})

// 稳定按序列号排序生成编号 01..N (不伪造 60 台)
const numberedDevices = computed(() => {
  const sorted = filteredDevices.value.slice().sort((a, b) => a.serial.localeCompare(b.serial))
  return sorted.map((d, idx) => {
    const num = idx + 1
    const label = num < 10 ? `0${num}` : String(num)
    return {
      serial: d.serial,
      number: num,
      label,
      device: d,
    }
  })
})

const serialToNumberMap = computed(() => {
  const map = new Map<string, string>()
  for (const item of numberedDevices.value) {
    map.set(item.serial, item.label)
  }
  return map
})

// 可见设备列表
const visibleDevices = computed(() => {
  return filteredDevices.value.filter(d => !hiddenSerials.value.includes(d.serial))
})

// 在线可选设备
const selectableDevices = computed(() => {
  return visibleDevices.value.filter(d => d.state === 'device')
})

const isAllSelected = computed(() => {
  if (selectableDevices.value.length === 0) return false
  return selectableDevices.value.every(d => selectedSerials.value.includes(d.serial))
})

// 显隐控制方法
function toggleHide(serial: string) {
  if (hiddenSerials.value.includes(serial)) {
    hiddenSerials.value = hiddenSerials.value.filter(s => s !== serial)
  } else {
    hiddenSerials.value = [...hiddenSerials.value, serial]
  }
}

function showAll() {
  hiddenSerials.value = []
}

function showOnlySelected() {
  if (selectedSerials.value.length === 0) return
  const selectedSet = new Set(selectedSerials.value)
  hiddenSerials.value = filteredDevices.value
    .filter(d => !selectedSet.has(d.serial))
    .map(d => d.serial)
}

// 勾选控制
function toggleSelect(serial: string) {
  if (selectedSerials.value.includes(serial)) {
    selectedSerials.value = selectedSerials.value.filter(s => s !== serial)
  } else {
    selectedSerials.value = [...selectedSerials.value, serial]
  }
}

function selectAll() {
  if (isAllSelected.value) {
    selectedSerials.value = []
  } else {
    selectedSerials.value = selectableDevices.value.map(d => d.serial)
  }
}

function clearSelection() {
  selectedSerials.value = []
}

// 模拟批量执行
const batchRunning = ref(false)
const currentAction = ref('')
interface BatchMockResult {
  serial: string
  name: string
  status: 'success' | 'failed' | 'timeout'
  elapsedMs: number
}
const batchResults = ref<BatchMockResult[]>([])

async function triggerAction(action: 'back' | 'home' | 'app_switch') {
  if (selectedSerials.value.length === 0 || batchRunning.value) return
  batchRunning.value = true
  currentAction.value = action === 'back' ? '返回键' : (action === 'home' ? '桌面主页' : '任务切换')
  batchResults.value = []

  const targets = [...selectedSerials.value]
  for (const serial of targets) {
    const dev = devices.value.find(d => d.serial === serial)
    const isError = Math.random() < 0.15
    await new Promise(r => setTimeout(r, 120))
    batchResults.value.push({
      serial,
      name: dev?.marketName || serial,
      status: isError ? 'failed' : 'success',
      elapsedMs: Math.floor(Math.random() * 80 + 40),
    })
  }

  batchRunning.value = false
}

const batchSummary = computed(() => {
  const success = batchResults.value.filter(r => r.status === 'success').length
  const failed = batchResults.value.filter(r => r.status === 'failed').length
  return { success, failed, total: batchResults.value.length }
})

const tileDimensionClass = computed(() => {
  if (dashSize.value === 'small') return 'w-[200px] h-[380px] max-w-full shrink-0'
  if (dashSize.value === 'medium') return 'w-[270px] h-[508px] max-w-full shrink-0'
  if (dashSize.value === 'large') return 'w-[348px] h-[648px] max-w-full shrink-0'
  return 'w-full h-full min-h-0 min-w-0'
})

const gridStyle = computed(() => {
  if (dashSize.value !== 'fit' || visibleDevices.value.length === 0) return {}
  const count = visibleDevices.value.length
  // 模拟铺满行列排布
  const cols = count <= 4 ? count : Math.ceil(Math.sqrt(count))
  const rows = Math.ceil(count / cols)
  return {
    display: 'grid',
    gridTemplateColumns: `repeat(${cols}, minmax(0, 1fr))`,
    gridTemplateRows: `repeat(${rows}, minmax(0, 1fr))`,
    gap: '2px',
    height: '100%',
    width: '100%',
  }
})
</script>

<template>
  <div class="min-h-screen bg-slate-950 text-slate-100 p-4 sm:p-6 space-y-5 select-none font-sans">
    <!-- 原型状态与说明横幅 -->
    <div class="flex items-center justify-between rounded-2xl border border-violet-500/30 bg-violet-950/20 px-4 py-2 text-xs text-violet-300">
      <div class="flex items-center gap-2">
        <span class="inline-block h-2 w-2 rounded-full bg-violet-400 animate-pulse" />
        <span class="font-semibold">v1.32 v2 交互原型：</span>
        <span>右侧预留批量面板 · 编号 01..N 小方块显隐 · 顶部全宽控制中枢不挤压</span>
      </div>
      <span class="font-mono text-[11px] opacity-70">Mock Data · 禁真机网络请求</span>
    </div>

    <!-- 顶部全息控制中枢胶囊：整行铺满，不受右侧面板宽度压缩 -->
    <div class="relative z-20 flex flex-col lg:flex-row items-center justify-between gap-4 rounded-3xl border border-white/10 bg-white/[0.04] p-3.5 shadow-2xl backdrop-blur-2xl lg:px-5">
      <!-- 左侧设备数 -->
      <div class="flex items-center gap-3 shrink-0 min-w-max">
        <div>
          <div class="flex items-center gap-2">
            <span class="text-base font-bold tracking-tight text-white">设备列表</span>
            <span class="rounded-full bg-emerald-500/20 px-2.5 py-0.5 text-[10px] font-semibold text-emerald-300 border border-emerald-500/30">
              {{ devices.filter(d => d.state === 'device').length }} 在线
            </span>
          </div>
          <p class="text-[11px] text-slate-400 mt-0.5">多设备投屏与右侧批量操作中枢</p>
        </div>
      </div>

      <!-- 中间：搜索条与离线开关 -->
      <div class="flex flex-1 flex-wrap items-center justify-center gap-2 max-w-xl w-full">
        <div class="relative flex-1 min-w-[200px]">
          <Search class="absolute left-3 top-1/2 h-4 w-4 -translate-y-1/2 text-slate-500" />
          <input
            v-model="keyword"
            placeholder="搜索设备名称、型号或序列号..."
            class="h-9 w-full rounded-2xl border border-white/10 bg-black/30 pl-9 pr-3 text-xs text-white placeholder:text-slate-500 focus:border-violet-400 focus:outline-none transition-colors"
          />
        </div>

        <!-- 来源分类 -->
        <div class="flex items-center rounded-2xl border border-white/10 bg-black/20 p-1 text-xs">
          <button
            v-for="mode in ['all', 'auto', 'manual'] as const"
            :key="mode"
            type="button"
            class="rounded-xl px-2.5 py-1 font-medium transition-all cursor-pointer"
            :class="sourceFilter === mode ? 'bg-white/15 text-white shadow-xs font-semibold' : 'text-slate-400 hover:text-white'"
            @click="sourceFilter = mode"
          >
            {{ mode === 'all' ? '全部' : (mode === 'auto' ? '直连' : '无线') }}
          </button>
        </div>

        <!-- 显示离线设备开关 -->
        <label class="flex items-center gap-1.5 px-2 py-1 text-xs text-slate-400 cursor-pointer select-none">
          <input
            v-model="showOffline"
            type="checkbox"
            class="h-3.5 w-3.5 rounded border-white/20 bg-black/40 text-violet-600 focus:ring-0 cursor-pointer"
          />
          <span>显示离线</span>
        </label>
      </div>

      <!-- 右侧：批量面板开关与全屏 -->
      <div class="flex items-center gap-2 shrink-0">
        <button
          type="button"
          class="flex items-center gap-1.5 h-9 rounded-2xl border px-3 text-xs font-medium cursor-pointer transition-all"
          :class="isBatchPanelOpen ? 'border-violet-500 bg-violet-600 text-white shadow-md' : 'border-white/10 bg-white/5 text-slate-300 hover:bg-white/10'"
          @click="isBatchPanelOpen = !isBatchPanelOpen"
        >
          <SlidersHorizontal class="h-3.5 w-3.5" />
          <span>批量操作</span>
          <span v-if="selectedSerials.length > 0" class="h-4 px-1 rounded-full bg-white/20 text-[10px] font-mono leading-none">
            {{ selectedSerials.length }}
          </span>
        </button>

        <button
          type="button"
          class="h-9 w-9 flex items-center justify-center rounded-2xl border border-white/10 bg-white/5 text-slate-400 hover:text-white cursor-pointer"
          :title="isPageFullscreen ? '退出全屏' : '全屏'"
          @click="isPageFullscreen = !isPageFullscreen"
        >
          <Minimize v-if="isPageFullscreen" class="h-4 w-4" />
          <Maximize v-else class="h-4 w-4" />
        </button>
      </div>
    </div>

    <!-- 主工作区：左侧设备卡片区域 + 右侧预留批量面板位置 (桌面端并排预留不遮挡卡片；窄屏 order-first 置顶就近操作) -->
    <div class="flex flex-col lg:flex-row items-start gap-5 w-full min-w-0">
      <!-- 右侧：预留批量操作面板 (桌面端 lg:order-last 排在右侧；窄屏 order-first 就近置顶) -->
      <aside
        v-if="isBatchPanelOpen"
        class="w-full lg:w-84 xl:w-96 shrink-0 order-first lg:order-last flex flex-col rounded-3xl border border-white/15 bg-[#0e121d]/95 backdrop-blur-3xl shadow-2xl overflow-hidden select-none"
      >
        <!-- 面板顶栏 -->
        <div class="flex items-center justify-between border-b border-white/10 px-4 py-3 shrink-0">
          <div class="flex items-center gap-2">
            <SlidersHorizontal class="h-4 w-4 text-violet-400" />
            <span class="font-bold text-sm text-white">批量操作面板</span>
          </div>
          <button
            type="button"
            class="p-1 rounded-lg text-slate-400 hover:text-white cursor-pointer transition-colors"
            title="关闭面板"
            @click="isBatchPanelOpen = false"
          >
            <X class="h-4 w-4" />
          </button>
        </div>

        <div class="p-4 space-y-4 overflow-y-auto max-h-[calc(100vh-14rem)] flex-1 overscroll-contain">
          <!-- 分区一：设备显隐 (01..N 小方块) -->
          <div class="space-y-2">
            <div class="flex items-center justify-between gap-1 text-xs">
              <div class="flex items-center gap-1.5">
                <span class="font-medium text-slate-300">设备</span>
                <span class="h-4.5 px-1.5 rounded bg-white/10 text-[10px] font-mono leading-none flex items-center">
                  {{ visibleDevices.length }}/{{ filteredDevices.length }}
                </span>
              </div>
              <div class="flex items-center gap-1">
                <button
                  type="button"
                  class="h-6 px-2 text-[11px] rounded-lg border border-white/10 bg-white/5 hover:bg-white/10 text-slate-200 cursor-pointer disabled:opacity-40"
                  :disabled="visibleDevices.length === filteredDevices.length"
                  @click="showAll"
                >
                  全显
                </button>
                <button
                  type="button"
                  class="h-6 px-2 text-[11px] rounded-lg border border-white/10 bg-white/5 hover:bg-white/10 text-slate-200 cursor-pointer disabled:opacity-40"
                  :disabled="selectedSerials.length === 0"
                  @click="showOnlySelected"
                >
                  只显已选
                </button>
              </div>
            </div>

            <!-- 编号小方块网格 -->
            <div class="grid grid-cols-6 sm:grid-cols-6 md:grid-cols-8 lg:grid-cols-6 xl:grid-cols-8 gap-1.5 max-h-52 overflow-y-auto p-1 rounded-2xl border border-white/10 bg-black/20">
              <button
                v-for="item in numberedDevices"
                :key="item.serial"
                type="button"
                class="relative flex flex-col items-center justify-center h-8 rounded-lg text-xs font-mono font-bold transition-all cursor-pointer"
                :class="[
                  !hiddenSerials.includes(item.serial)
                    ? (selectedSerials.includes(item.serial)
                        ? 'border-2 border-violet-500 bg-violet-600 text-white shadow-xs'
                        : 'border border-white/20 bg-white/10 text-slate-100 hover:border-violet-400')
                    : 'opacity-40 border border-dashed border-white/10 bg-white/5 text-slate-400 line-through',
                ]"
                :title="`[${item.label}] ${item.device.marketName} (${item.serial}) · ${item.device.state === 'device' ? '在线' : '离线'}`"
                @click="toggleHide(item.serial)"
              >
                <span
                  class="absolute top-1 right-1 h-1.5 w-1.5 rounded-full"
                  :class="item.device.state === 'device' ? 'bg-emerald-400' : 'bg-amber-400'"
                />
                <span>{{ item.label }}</span>
              </button>
            </div>
          </div>

          <!-- 分区二：批量选择 -->
          <div class="space-y-2 border-t border-white/10 pt-3">
            <div class="flex items-center justify-between text-xs">
              <span class="text-slate-400 font-medium">批量选择</span>
              <span class="font-semibold text-violet-400">已选 {{ selectedSerials.length }} 台</span>
            </div>
            <div class="flex items-center gap-2">
              <button
                type="button"
                class="flex-1 h-7.5 rounded-xl border border-white/15 bg-white/5 hover:bg-white/10 text-xs font-medium text-slate-200 cursor-pointer disabled:opacity-40"
                :disabled="selectableDevices.length === 0 || batchRunning"
                @click="selectAll"
              >
                {{ isAllSelected ? '取消全选' : '全选' }}
              </button>
              <button
                type="button"
                class="flex-1 h-7.5 rounded-xl border border-white/15 bg-white/5 hover:bg-white/10 text-xs font-medium text-slate-200 cursor-pointer disabled:opacity-40"
                :disabled="selectedSerials.length === 0 || batchRunning"
                @click="clearSelection"
              >
                清空
              </button>
            </div>
          </div>

          <!-- 分区三：动作 -->
          <div class="space-y-2 border-t border-white/10 pt-3">
            <div class="grid grid-cols-3 gap-2">
              <button
                type="button"
                class="h-8 rounded-xl border border-white/15 bg-white/5 hover:bg-white/10 text-xs text-slate-200 flex items-center justify-center gap-1 cursor-pointer disabled:opacity-40"
                :disabled="selectedSerials.length === 0 || batchRunning"
                @click="triggerAction('back')"
              >
                <ChevronLeft class="h-3.5 w-3.5" />
                返回
              </button>
              <button
                type="button"
                class="h-8 rounded-xl border border-white/15 bg-white/5 hover:bg-white/10 text-xs text-slate-200 flex items-center justify-center gap-1 cursor-pointer disabled:opacity-40"
                :disabled="selectedSerials.length === 0 || batchRunning"
                @click="triggerAction('home')"
              >
                <Home class="h-3.5 w-3.5" />
                主页
              </button>
              <button
                type="button"
                class="h-8 rounded-xl border border-white/15 bg-white/5 hover:bg-white/10 text-xs text-slate-200 flex items-center justify-center gap-1 cursor-pointer disabled:opacity-40"
                :disabled="selectedSerials.length === 0 || batchRunning"
                @click="triggerAction('app_switch')"
              >
                <AppWindow class="h-3.5 w-3.5" />
                任务
              </button>
            </div>

            <!-- 运行指示 -->
            <div
              v-if="batchRunning"
              class="flex items-center justify-between rounded-xl bg-violet-950/40 border border-violet-500/30 p-2 text-xs text-violet-300"
            >
              <div class="flex items-center gap-2">
                <Loader2 class="h-3.5 w-3.5 animate-spin text-violet-400" />
                <span>正在执行 {{ currentAction }}...</span>
              </div>
            </div>
          </div>

          <!-- 分区四：显示设置 -->
          <div class="space-y-2 border-t border-white/10 pt-3">
            <div class="flex items-center justify-between text-xs">
              <span class="text-slate-400 font-medium">显示</span>
              <button
                type="button"
                class="flex items-center gap-1 px-1.5 py-0.5 rounded text-[11px] text-slate-400 hover:text-white cursor-pointer"
                @click="isPageFullscreen = !isPageFullscreen"
              >
                <Minimize v-if="isPageFullscreen" class="h-3 w-3" />
                <Maximize v-else class="h-3 w-3" />
                <span>{{ isPageFullscreen ? '退出全屏' : '全屏' }}</span>
              </button>
            </div>
            <div class="flex items-center gap-1 rounded-xl border border-white/10 bg-black/20 p-0.5">
              <button
                v-for="s in (['small', 'medium', 'large', 'fit'] as const)"
                :key="s"
                type="button"
                class="flex-1 h-7 rounded-lg text-xs font-medium transition-colors cursor-pointer"
                :class="dashSize === s ? 'bg-white/15 text-white font-semibold' : 'text-slate-400 hover:text-white'"
                @click="dashSize = s"
              >
                {{ s === 'small' ? '小' : (s === 'medium' ? '中' : (s === 'large' ? '大' : '铺满')) }}
              </button>
            </div>
          </div>

          <!-- 分区五：逐台执行结果 -->
          <div v-if="batchResults.length > 0" class="space-y-2 border-t border-white/10 pt-3">
            <div class="flex items-center justify-between text-xs">
              <span class="text-slate-400 font-medium">执行结果</span>
              <div class="flex items-center gap-2">
                <span class="text-[10px] text-emerald-400">成功 {{ batchSummary.success }}</span>
                <span v-if="batchSummary.failed > 0" class="text-[10px] text-rose-400">失败 {{ batchSummary.failed }}</span>
                <button
                  type="button"
                  class="text-[10px] text-slate-500 hover:text-white cursor-pointer ml-1"
                  @click="batchResults = []"
                >
                  清除
                </button>
              </div>
            </div>

            <div class="max-h-44 overflow-y-auto space-y-1.5 pr-1">
              <div
                v-for="r in batchResults"
                :key="r.serial"
                class="rounded-xl border border-white/5 bg-white/[0.02] p-2 text-xs flex items-center justify-between gap-2"
              >
                <div class="min-w-0">
                  <div class="truncate text-[11px] font-medium text-slate-200">{{ r.name }}</div>
                  <div class="truncate font-mono text-[9px] text-slate-500">{{ r.serial }}</div>
                </div>
                <div class="shrink-0 text-right">
                  <span
                    v-if="r.status === 'success'"
                    class="inline-flex items-center gap-1 text-[10px] text-emerald-400 font-medium"
                  >
                    成功
                    <span class="font-mono text-[8px] text-slate-500">{{ r.elapsedMs }}ms</span>
                  </span>
                  <span v-else class="text-[10px] text-rose-400 font-medium">
                    失败
                  </span>
                </div>
              </div>
            </div>
          </div>
        </div>
      </aside>

      <!-- 左侧：手机视图设备卡片流 (顶端与右侧面板自然对齐) -->
      <div class="flex-1 min-w-0 w-full space-y-4">
        <!-- 空设备提示 (全部被隐藏) -->
        <div
          v-if="visibleDevices.length === 0"
          class="flex flex-col items-center justify-center p-12 text-slate-400 space-y-3 rounded-3xl border border-dashed border-white/10 bg-white/[0.01]"
        >
          <EyeOff class="h-8 w-8 text-slate-500" />
          <h4 class="text-sm font-semibold text-slate-300">所有设备已隐藏</h4>
          <p class="text-xs text-slate-500">可通过右侧编号面板或点击下方按钮重新显示</p>
          <button
            type="button"
            class="px-3 py-1.5 rounded-xl border border-white/15 bg-white/10 text-xs text-white hover:bg-white/20 cursor-pointer mt-2"
            @click="showAll"
          >
            全显
          </button>
        </div>

        <!-- 手机卡片网格：横纵相邻统一 2px 间隙，格子同宽同高 -->
        <div
          v-else
          :class="dashSize === 'fit' ? 'grid gap-[2px] h-[calc(100vh-14rem)] w-full' : 'flex flex-wrap items-start content-start gap-[2px] overflow-y-auto'"
          :style="gridStyle"
        >
          <div
            v-for="d in visibleDevices"
            :key="d.serial"
            class="group relative flex flex-col rounded-[2px] border border-neutral-800 bg-slate-900 overflow-hidden select-none transition-all duration-150"
            :class="[
              tileDimensionClass,
              selectedSerials.includes(d.serial) ? 'ring-1 ring-violet-500 border-violet-500' : 'hover:border-neutral-600',
            ]"
          >
            <!-- 手机卡片顶栏：统一固定高度 28px，编号徽章、多选、名称、状态 -->
            <div class="flex h-7 items-center justify-between border-b border-white/10 bg-slate-950/80 px-2.5 py-1 text-xs text-white shrink-0 select-none">
              <div class="flex min-w-0 items-center gap-1.5">
                <button
                  v-if="d.state === 'device'"
                  type="button"
                  class="flex h-5 w-5 shrink-0 items-center justify-center rounded transition-colors cursor-pointer"
                  :class="selectedSerials.includes(d.serial) ? 'text-violet-400' : 'text-slate-400 hover:text-white'"
                  :title="selectedSerials.includes(d.serial) ? '取消勾选' : '勾选此设备'"
                  @click.stop="toggleSelect(d.serial)"
                >
                  <component :is="selectedSerials.includes(d.serial) ? CheckSquare : Square" class="h-4 w-4" />
                </button>

                <!-- 编号徽章 -->
                <span class="px-1.5 py-0.2 rounded font-mono text-[10px] font-bold bg-violet-500/20 text-violet-300 border border-violet-500/30 shrink-0">
                  {{ serialToNumberMap.get(d.serial) }}
                </span>

                <div class="min-w-0 truncate">
                  <span class="truncate font-semibold text-[11px] text-slate-100">
                    {{ d.marketName }}
                  </span>
                </div>
              </div>

              <div class="flex items-center gap-1.5 shrink-0">
                <span
                  class="h-2 w-2 rounded-full"
                  :class="d.state === 'device' ? 'bg-emerald-400' : 'bg-amber-400'"
                  :title="d.state === 'device' ? '在线' : '离线'"
                />
              </div>
            </div>

            <!-- 模拟手机投屏画面：黑底 contain 居中等比显示，不拉伸不裁剪 -->
            <div class="relative w-full flex-1 min-h-0 bg-black flex flex-col items-center justify-center p-3 text-center overflow-hidden">
              <div class="space-y-1.5">
                <div class="flex items-center justify-center text-slate-600">
                  <Smartphone class="h-9 w-9 opacity-40" />
                </div>
                <div class="text-[11px] text-slate-300 font-medium">{{ d.model }}</div>
                <div class="text-[10px] text-slate-500 font-mono">{{ d.serial }}</div>
                <div class="text-[9px] text-emerald-400/90 font-mono">
                  {{ d.state === 'device' ? (d.resolution ? `${d.resolution} · 60 FPS` : '60 FPS · 实时画面') : d.state }}
                </div>
              </div>
            </div>
          </div>
        </div>
      </div>
    </div>
  </div>
</template>
