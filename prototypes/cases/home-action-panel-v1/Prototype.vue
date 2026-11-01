<script setup lang="ts">
import { computed, nextTick, ref } from 'vue'
import {
  ArrowLeft,
  Battery,
  BatteryCharging,
  CheckSquare,
  ChevronRight,
  Home,
  Layers,
  LayoutGrid,
  Maximize,
  PanelRightClose,
  PanelRightOpen,
  Plus,
  RefreshCw,
  Search,
  SlidersHorizontal,
  Smartphone,
  Square,
  Thermometer,
  Usb,
  Wifi,
  X,
} from 'lucide-vue-next'
import { toast } from 'vue-sonner'
import { Badge } from '@/components/ui/badge'
import { Button } from '@/components/ui/button'
import { Input } from '@/components/ui/input'
import { MOCK_DEVICES, type MockDevice } from './mockDevices'

// 设备数据源
const devices = ref<MockDevice[]>(JSON.parse(JSON.stringify(MOCK_DEVICES)))

// 面板展开/收起状态（默认展开）
const isPanelOpen = ref(true)

// 原设备列表 Bar 迁入的状态与全新滑动调节
const keyword = ref('')
// 视图模式：去掉 compact，仅保留 'deck'（卡片）与 'phone'（手机视图）
const viewMode = ref<'deck' | 'phone'>('deck')

// 1. 手机大小滑动调节（70% ~ 140%，默认 100% 对齐 270px 拟物手机标准宽）
const phoneScale = ref(100)
const cardWidth = computed(() => Math.round(270 * (phoneScale.value / 100)))

// 2. 投屏码率滑动调节（2 Mbps ~ 16 Mbps，默认 8 Mbps）
const bitrateMbps = ref(8)

// 3. 投屏质量滑动调节（50% ~ 100%，默认 90%）
const qualityPercent = ref(90)
const qualityLabel = computed(() => {
  if (qualityPercent.value <= 60) return '720P 标清'
  if (qualityPercent.value <= 80) return '1080P 高清'
  if (qualityPercent.value < 100) return '2K 超清'
  return '100% 原画'
})

// 手机视图显示尺寸
const dashSize = ref<'small' | 'medium' | 'large' | 'fill'>('medium')
const isPageFullscreen = ref(false)

// 显隐过滤状态（存储隐藏的 serial）
const hiddenSerials = ref<string[]>([])

// 批量勾选设备（存储选中的 serial）
const selectedSerials = ref<string[]>([devices.value[0].serial, devices.value[1].serial])

// 批量操作运行状态
const batchRunning = ref(false)
const currentAction = ref('')

// 操作反馈历史记录
interface ActionResult {
  id: string
  time: string
  action: string
  serial: string
  name: string
  status: 'success' | 'failed'
  message: string
}
const actionHistory = ref<ActionResult[]>([
  {
    id: '1',
    time: '23:58:12',
    action: '返回 (Back)',
    serial: devices.value[0].serial,
    name: devices.value[0].alias || devices.value[0].serial,
    status: 'success',
    message: '注入按键返回成功',
  },
  {
    id: '2',
    time: '23:58:12',
    action: '返回 (Back)',
    serial: devices.value[1].serial,
    name: devices.value[1].alias || devices.value[1].serial,
    status: 'success',
    message: '注入按键返回成功',
  },
])



// 刷新状态
const isRefreshing = ref(false)
function triggerRefresh() {
  isRefreshing.value = true
  setTimeout(() => {
    isRefreshing.value = false
    toast.success('设备列表已刷新')
  }, 450)
}

// 展开面板并聚焦搜索框
function expandAndFocusSearch() {
  isPanelOpen.value = true
  nextTick(() => {
    const el = document.querySelector<HTMLInputElement>('#panel-search-input')
    el?.focus()
  })
}

// 过滤后的设备列表
const filteredDevices = computed(() => {
  return devices.value.filter((d) => {
    // 关键字搜索
    if (keyword.value.trim()) {
      const q = keyword.value.trim().toLowerCase()
      const matchName = (d.alias || '').toLowerCase().includes(q)
      const matchModel = d.telemetry.model.toLowerCase().includes(q)
      const matchSerial = d.serial.toLowerCase().includes(q)
      const matchApp = d.telemetry.currentApp.toLowerCase().includes(q)
      if (!matchName && !matchModel && !matchSerial && !matchApp)
        return false
    }

    return true
  })
})

// 最终在界面上可见的设备列表（剔除 hiddenSerials）
const visibleDevices = computed(() => {
  return filteredDevices.value.filter(d => !hiddenSerials.value.includes(d.serial))
})

// 在线数量与总数统计
const onlineCount = computed(() => devices.value.filter(d => d.state === 'device').length)
const totalCount = computed(() => devices.value.length)

// 显隐切换
function toggleHide(serial: string) {
  if (hiddenSerials.value.includes(serial))
    hiddenSerials.value = hiddenSerials.value.filter(s => s !== serial)
  else
    hiddenSerials.value.push(serial)
}

function showAllDevices() {
  hiddenSerials.value = []
  toast.success('已全部显示')
}

function showOnlySelected() {
  if (selectedSerials.value.length === 0) {
    toast.error('未勾选任何设备')
    return
  }
  const toHide: string[] = []
  devices.value.forEach((d) => {
    if (!selectedSerials.value.includes(d.serial))
      toHide.push(d.serial)
  })
  hiddenSerials.value = toHide
  toast.success(`仅显示已选 ${selectedSerials.value.length} 台设备`)
}

// 选择与勾选控制
const selectableDevices = computed(() => filteredDevices.value.filter(d => d.state === 'device'))
const isAllSelected = computed(() => {
  if (selectableDevices.value.length === 0)
    return false
  return selectableDevices.value.every(d => selectedSerials.value.includes(d.serial))
})

function toggleSelectDevice(serial: string) {
  if (selectedSerials.value.includes(serial))
    selectedSerials.value = selectedSerials.value.filter(s => s !== serial)
  else
    selectedSerials.value.push(serial)
}

function selectAll() {
  selectedSerials.value = selectableDevices.value.map(d => d.serial)
  toast.success(`已全选 ${selectedSerials.value.length} 台在线设备`)
}

function clearSelection() {
  selectedSerials.value = []
}

// 模拟触发批量操作
function triggerBatchAction(actionName: 'back' | 'home' | 'app_switch') {
  if (selectedSerials.value.length === 0) {
    toast.error('请先勾选需要操作的设备')
    return
  }

  const labelMap = {
    back: '返回 (Back)',
    home: '主页 (Home)',
    app_switch: '多任务 (AppSwitch)',
  }
  const label = labelMap[actionName]
  batchRunning.value = true
  currentAction.value = label

  setTimeout(() => {
    batchRunning.value = false
    currentAction.value = ''

    const timeStr = new Date().toTimeString().split(' ')[0]
    selectedSerials.value.forEach((s) => {
      const dev = devices.value.find(d => d.serial === s)
      actionHistory.value.unshift({
        id: Math.random().toString(36).slice(2, 9),
        time: timeStr,
        action: label,
        serial: s,
        name: dev?.alias || s,
        status: 'success',
        message: '指令已下发并执行',
      })
    })
    toast.success(`批量执行「${label}」完成（共 ${selectedSerials.value.length} 台）`)
  }, 500)
}

// 模拟添加设备
function addMockDevice() {
  const index = devices.value.length + 1
  const serial = `MOCK_DEV_${index}`
  devices.value.push({
    serial,
    state: 'device',
    source: index % 2 === 0 ? 'manual' : 'auto',
    alias: `新增测试机 0${index}`,
    telemetry: {
      brand: 'TestPhone',
      manufacturer: 'Android',
      model: `TP-0${index}`,
      androidRelease: '14',
      batteryLevel: Math.floor(Math.random() * 60) + 40,
      batteryCharging: false,
      batteryTempCelsius: 32.5,
      cpuTempCelsius: 36.8,
      cpuTempSupported: true,
      cpuUsagePercent: 14,
      currentApp: '测试应用',
      currentPackage: 'com.example.test',
      iconType: 'default',
      resolution: '1080 × 2400',
      screenOn: true,
      screenLocked: false,
    },
  })
  toast.success(`已添加模拟设备 ${serial}`)
}

// 获取应用徽标背景颜色
function getAppBadgeBg(iconType: string) {
  switch (iconType) {
    case 'wechat': return 'bg-emerald-500'
    case 'kuaishou': return 'bg-amber-500'
    case 'douyin': return 'bg-black'
    case 'xiaohongshu': return 'bg-red-500'
    default: return 'bg-violet-600'
  }
}
</script>

<template>
  <div class="space-y-6">
    <!-- 原型验证与场景说明条 -->
    <div class="rounded-2xl border border-violet-500/20 bg-violet-500/5 p-4 text-xs dark:bg-violet-500/10 dark:border-violet-500/30">
      <div class="flex flex-wrap items-center justify-between gap-3">
        <div class="space-y-1">
          <div class="flex items-center gap-2">
            <span class="font-bold text-sm text-violet-700 dark:text-violet-300">首页右侧“操作”面板重构演示</span>
            <Badge variant="outline" class="border-violet-500/40 text-violet-700 dark:text-violet-300">
              去顶部Bar · 右侧集成 · 剔除紧凑阵列
            </Badge>
          </div>
          <p class="text-slate-600 dark:text-slate-400">
            原顶部横条的搜索、过滤、视图切换、刷新与连接等全部迁入右侧面板；面板由原“批量操作”更名为“操作”，支持平滑折叠收起与展开，收起后设备视口自适应铺满。
          </p>
        </div>
        <div class="flex items-center gap-2">
          <Button
            size="sm"
            variant="outline"
            class="h-8 text-xs cursor-pointer"
            @click="isPanelOpen = !isPanelOpen"
          >
            <component :is="isPanelOpen ? PanelRightClose : PanelRightOpen" class="h-3.5 w-3.5 mr-1" />
            {{ isPanelOpen ? '收起面板' : '展开面板' }}
          </Button>
          <Button
            size="sm"
            variant="secondary"
            class="h-8 text-xs cursor-pointer"
            @click="addMockDevice"
          >
            <Plus class="h-3.5 w-3.5 mr-1" />
            加一台设备
          </Button>
        </div>
      </div>
    </div>

    <!-- 真实首页模拟主容器（左右分栏结构：完全移除顶部Bar，视野极度纯净） -->
    <div class="relative min-h-[750px] rounded-3xl border border-slate-200/80 bg-slate-100/60 p-4 dark:border-white/10 dark:bg-black/40 overflow-hidden flex flex-col">
      <!-- 核心工作区：直接顶格展示，左侧设备区 + 右侧可收起“操作”面板 -->
      <div class="flex-1 flex items-start gap-4 w-full min-w-0">
        
        <!-- 左侧：设备主展示区（完全无横条/提示条干扰，随右侧面板收起自动扩展宽度） -->
        <main class="flex-1 min-w-0 transition-all duration-300">
          <!-- 空结果提示 -->
          <div v-if="visibleDevices.length === 0" class="py-20 text-center text-slate-400 dark:text-slate-500">
            <Smartphone class="mx-auto h-12 w-12 stroke-[1.2] mb-3 opacity-60" />
            <p class="text-sm font-medium">未找到符合条件的设备</p>
            <p class="text-xs mt-1">请尝试在右侧操作面板中调整搜索词、来源或取消隐藏</p>
          </div>

          <!-- 模式一：手机拟物卡片视图 (deck，支持滑块无级调节尺寸) -->
          <div
            v-else-if="viewMode === 'deck'"
            class="grid gap-4 transition-all duration-200"
            :style="{ gridTemplateColumns: `repeat(auto-fill, minmax(${cardWidth}px, 1fr))` }"
          >
            <div
              v-for="(dev, idx) in visibleDevices"
              :key="dev.serial"
              class="group relative flex flex-col rounded-[28px] border border-slate-200/90 bg-white/90 p-4 shadow-sm hover:shadow-md transition-all dark:border-white/10 dark:bg-slate-900/90 select-none"
              :class="{
                'ring-2 ring-violet-500/80 border-violet-500/80': selectedSerials.includes(dev.serial),
                'opacity-65': dev.state === 'offline',
              }"
            >
              <!-- 手机顶部状态微缩栏 -->
              <div class="flex items-center justify-between text-[11px] text-slate-500 dark:text-slate-400 pb-2 border-b border-slate-100 dark:border-white/5">
                <div class="flex items-center gap-1.5">
                  <span class="font-mono font-bold text-slate-700 dark:text-slate-200">
                    {{ String(idx + 1).padStart(2, '0') }}
                  </span>
                  <component :is="dev.source === 'auto' ? Usb : Wifi" class="h-3 w-3" />
                  <span class="truncate max-w-[90px]">{{ dev.alias || dev.serial }}</span>
                </div>
                <div class="flex items-center gap-2">
                  <span v-if="dev.state === 'device'" class="flex items-center gap-0.5 text-amber-600 dark:text-amber-400">
                    <Thermometer class="h-3 w-3" />
                    {{ dev.telemetry.batteryTempCelsius }}°C
                  </span>
                  <span v-if="dev.state === 'device'" class="flex items-center gap-0.5 text-emerald-600 dark:text-emerald-400">
                    <component :is="dev.telemetry.batteryCharging ? BatteryCharging : Battery" class="h-3 w-3" />
                    {{ dev.telemetry.batteryLevel }}%
                  </span>
                  <span v-else class="text-rose-500 font-medium">离线</span>
                </div>
              </div>

              <!-- 手机视口与前台应用展示区 -->
              <div class="my-3 flex flex-col items-center justify-center rounded-2xl bg-slate-50 dark:bg-black/30 p-4 border border-slate-100 dark:border-white/5 min-h-[140px]">
                <div
                  v-if="dev.state === 'device' && dev.telemetry.screenOn"
                  class="flex flex-col items-center text-center space-y-2"
                >
                  <div
                    class="h-12 w-12 rounded-2xl flex items-center justify-center text-white shadow-md font-bold text-sm"
                    :class="getAppBadgeBg(dev.telemetry.iconType)"
                  >
                    {{ dev.telemetry.currentApp.slice(0, 2) }}
                  </div>
                  <div>
                    <p class="font-bold text-xs text-slate-900 dark:text-white">{{ dev.telemetry.currentApp }}</p>
                    <p class="text-[10px] text-slate-400 font-mono">{{ dev.telemetry.currentPackage }}</p>
                  </div>
                </div>
                <div v-else-if="dev.state === 'device' && !dev.telemetry.screenOn" class="text-center space-y-1">
                  <div class="h-10 w-10 mx-auto rounded-full bg-slate-200 dark:bg-white/10 flex items-center justify-center text-slate-500">
                    <Smartphone class="h-5 w-5" />
                  </div>
                  <p class="text-xs font-medium text-slate-500 dark:text-slate-400">已休眠锁屏</p>
                </div>
                <div v-else class="text-center text-slate-400">
                  <p class="text-xs">设备未连接</p>
                </div>
              </div>

              <!-- 卡片底部操作栏 -->
              <div class="flex items-center justify-between pt-1">
                <button
                  type="button"
                  class="flex items-center gap-1.5 text-xs text-slate-600 hover:text-violet-600 dark:text-slate-400 dark:hover:text-violet-300 cursor-pointer"
                  @click="toggleSelectDevice(dev.serial)"
                >
                  <component
                    :is="selectedSerials.includes(dev.serial) ? CheckSquare : Square"
                    class="h-4 w-4"
                    :class="{ 'text-violet-600 dark:text-violet-400': selectedSerials.includes(dev.serial) }"
                  />
                  <span>{{ selectedSerials.includes(dev.serial) ? '已选' : '勾选' }}</span>
                </button>

                <Button size="sm" variant="outline" class="h-7 text-xs rounded-xl px-2.5 cursor-pointer">
                  投屏操控
                  <ChevronRight class="h-3 w-3 ml-0.5" />
                </Button>
              </div>
            </div>
          </div>

          <!-- 模式二：首页手机视图 / 紧密网格投屏 (phone，支持滑块无级调节尺寸) -->
          <div
            v-else-if="viewMode === 'phone'"
            class="grid gap-2 transition-all duration-200"
            :style="{ gridTemplateColumns: `repeat(auto-fill, minmax(${Math.round(cardWidth * 0.75)}px, 1fr))` }"
          >
            <div
              v-for="(dev, idx) in visibleDevices"
              :key="dev.serial"
              class="relative rounded-2xl bg-black border border-white/15 overflow-hidden aspect-[9/16] flex flex-col justify-between p-2 shadow-sm"
              :class="{ 'ring-2 ring-violet-500': selectedSerials.includes(dev.serial) }"
            >
              <!-- 顶部编号与状态 -->
              <div class="flex items-center justify-between text-[10px] text-white/90 z-10">
                <span class="rounded bg-black/60 px-1 py-0.5 font-mono font-bold">
                  {{ String(idx + 1).padStart(2, '0') }}
                </span>
                <span class="rounded bg-black/60 px-1 py-0.5">
                  {{ dev.telemetry.batteryLevel }}%
                </span>
              </div>

              <!-- 模拟视频画面帧 -->
              <div class="absolute inset-0 flex flex-col items-center justify-center text-center p-3">
                <div class="w-10 h-10 rounded-xl mb-1 flex items-center justify-center text-white text-xs font-bold" :class="getAppBadgeBg(dev.telemetry.iconType)">
                  {{ dev.telemetry.currentApp.slice(0, 2) }}
                </div>
                <span class="text-xs text-white/80 font-medium truncate max-w-[90%]">{{ dev.telemetry.currentApp }}</span>
                <span class="text-[9px] text-white/40 font-mono mt-0.5">{{ dev.telemetry.resolution }}</span>
              </div>

              <!-- 底部别名与勾选 -->
              <div class="flex items-center justify-between text-[10px] text-white/90 z-10 bg-black/50 px-1.5 py-1 rounded-lg backdrop-blur-xs">
                <span class="truncate max-w-[80px]">{{ dev.alias || dev.serial }}</span>
                <button type="button" class="cursor-pointer" @click="toggleSelectDevice(dev.serial)">
                  <component
                    :is="selectedSerials.includes(dev.serial) ? CheckSquare : Square"
                    class="h-3.5 w-3.5 text-violet-400"
                  />
                </button>
              </div>
            </div>
          </div>
        </main>

        <!-- ================= 右侧：可收起展开的“操作”面板 ================= -->
        
        <!-- 1. 展开状态 (Expanded) -->
        <aside
          v-if="isPanelOpen"
          aria-label="操作面板"
          class="w-80 lg:w-84 shrink-0 flex flex-col rounded-3xl border border-slate-200/90 bg-white/95 dark:border-white/15 dark:bg-[#0e121d]/95 backdrop-blur-3xl shadow-xl overflow-hidden select-none transition-all duration-300"
        >
          <!-- 面板顶栏 Header -->
          <div class="flex items-center justify-between border-b border-slate-200/80 px-4 py-3.5 dark:border-white/10 shrink-0">
            <div class="flex items-center gap-2">
              <SlidersHorizontal class="h-4 w-4 text-violet-600 dark:text-violet-400" />
              <span class="font-bold text-sm text-slate-900 dark:text-white">操作</span>
            </div>
            
            <div class="flex items-center gap-1">
              <!-- 刷新设备 -->
              <button
                type="button"
                class="p-1.5 rounded-xl text-slate-400 hover:text-slate-900 hover:bg-slate-100 dark:hover:text-white dark:hover:bg-white/10 cursor-pointer transition-colors"
                title="刷新设备列表"
                @click="triggerRefresh"
              >
                <RefreshCw class="h-3.5 w-3.5" :class="{ 'animate-spin': isRefreshing }" />
              </button>

              <!-- 添加设备 -->
              <button
                type="button"
                class="p-1.5 rounded-xl text-slate-400 hover:text-slate-900 hover:bg-slate-100 dark:hover:text-white dark:hover:bg-white/10 cursor-pointer transition-colors"
                title="连接设备"
                @click="addMockDevice"
              >
                <Plus class="h-3.5 w-3.5" />
              </button>

              <!-- 收起按钮 -->
              <button
                type="button"
                class="p-1.5 rounded-xl text-slate-400 hover:text-slate-900 hover:bg-slate-100 dark:hover:text-white dark:hover:bg-white/10 cursor-pointer transition-colors ml-1"
                title="收起操作面板"
                aria-label="收起操作面板"
                @click="isPanelOpen = false"
              >
                <PanelRightClose class="h-4 w-4" />
              </button>
            </div>
          </div>

          <!-- 面板内容滚动区 -->
          <div class="p-4 space-y-4 overflow-y-auto max-h-[calc(100vh-16rem)] flex-1 overscroll-contain text-xs">
            
            <!-- ====== 模块 1：设备查找、过滤与视图模式（原设备列表 Bar 核心功能） ====== -->
            <div class="space-y-2.5 pb-3 border-b border-slate-200/80 dark:border-white/10">
              <span class="text-[11px] font-bold uppercase tracking-wider text-slate-400 dark:text-slate-500">设备控制</span>
              
              <!-- 搜索框 -->
              <div class="relative w-full">
                <Search class="absolute left-2.5 top-1/2 h-3.5 w-3.5 -translate-y-1/2 text-slate-400 dark:text-slate-500" />
                <Input
                  id="panel-search-input"
                  v-model="keyword"
                  placeholder="搜索名称、型号、包名..."
                  class="h-8 rounded-xl border-slate-200/90 bg-slate-100/80 pl-8 pr-7 text-xs text-slate-900 placeholder:text-slate-400 focus:border-violet-500 dark:border-white/10 dark:bg-black/30 dark:text-white dark:placeholder:text-slate-500 transition-colors"
                />
                <button
                  v-if="keyword"
                  type="button"
                  class="absolute right-2 top-1/2 -translate-y-1/2 text-slate-400 hover:text-slate-600 dark:hover:text-white cursor-pointer"
                  @click="keyword = ''"
                >
                  <X class="h-3 w-3" />
                </button>
              </div>

              <!-- 视图模式切换（剔除紧凑阵列，仅保留手机卡片与手机视图） -->
              <div class="flex items-center justify-between pt-0.5">
                <span class="text-slate-600 dark:text-slate-400">视图模式</span>
                <div class="flex items-center rounded-xl border border-slate-200/80 bg-slate-100/80 dark:border-white/10 dark:bg-black/20 p-0.5 text-xs">
                  <button
                    type="button"
                    class="flex items-center gap-1 rounded-lg px-2.5 py-1 font-medium transition-all cursor-pointer"
                    :class="viewMode === 'deck'
                      ? 'bg-white text-slate-950 shadow-xs font-semibold dark:bg-white/15 dark:text-white'
                      : 'text-slate-500 hover:text-slate-950 dark:text-slate-400 dark:hover:text-white'"
                    title="手机卡片视图"
                    @click="viewMode = 'deck'"
                  >
                    <LayoutGrid class="h-3 w-3" />
                    <span>卡片</span>
                  </button>
                  <button
                    type="button"
                    class="flex items-center gap-1 rounded-lg px-2.5 py-1 font-medium transition-all cursor-pointer"
                    :class="viewMode === 'phone'
                      ? 'bg-white text-slate-950 shadow-xs font-semibold dark:bg-white/15 dark:text-white'
                      : 'text-slate-500 hover:text-slate-950 dark:text-slate-400 dark:hover:text-white'"
                    title="手机视图"
                    @click="viewMode = 'phone'"
                  >
                    <Smartphone class="h-3 w-3" />
                    <span>手机</span>
                  </button>
                </div>
              </div>

              <!-- ====== 滑动调节组：手机大小、投屏码率、画质 ====== -->
              <div class="space-y-3 pt-2">
                <!-- 1. 手机大小滑动条 -->
                <div class="space-y-1.5">
                  <div class="flex items-center justify-between text-xs">
                    <span class="font-medium text-slate-700 dark:text-slate-300">手机大小</span>
                    <span class="font-mono text-[11px] font-semibold text-violet-600 dark:text-violet-400">
                      {{ phoneScale }}% ({{ cardWidth }}px)
                    </span>
                  </div>
                  <input
                    type="range"
                    min="70"
                    max="140"
                    step="5"
                    v-model.number="phoneScale"
                    class="w-full h-1.5 rounded-lg appearance-none cursor-pointer bg-slate-200 dark:bg-white/15 accent-violet-600"
                  />
                  <div class="flex justify-between text-[10px] text-slate-400">
                    <span class="cursor-pointer hover:text-violet-600" @click="phoneScale = 75">紧凑 75%</span>
                    <span class="cursor-pointer hover:text-violet-600" @click="phoneScale = 100">标准 100%</span>
                    <span class="cursor-pointer hover:text-violet-600" @click="phoneScale = 130">超大 130%</span>
                  </div>
                </div>

                <!-- 2. 投屏码率滑动条 -->
                <div class="space-y-1.5">
                  <div class="flex items-center justify-between text-xs">
                    <span class="font-medium text-slate-700 dark:text-slate-300">投屏码率</span>
                    <span class="font-mono text-[11px] font-semibold text-violet-600 dark:text-violet-400">
                      {{ bitrateMbps }} Mbps
                    </span>
                  </div>
                  <input
                    type="range"
                    min="2"
                    max="16"
                    step="1"
                    v-model.number="bitrateMbps"
                    class="w-full h-1.5 rounded-lg appearance-none cursor-pointer bg-slate-200 dark:bg-white/15 accent-violet-600"
                  />
                  <div class="flex justify-between text-[10px] text-slate-400">
                    <span class="cursor-pointer hover:text-violet-600" @click="bitrateMbps = 2">2M 流畅</span>
                    <span class="cursor-pointer hover:text-violet-600" @click="bitrateMbps = 8">8M 超清</span>
                    <span class="cursor-pointer hover:text-violet-600" @click="bitrateMbps = 16">16M 极致</span>
                  </div>
                </div>

                <!-- 3. 投屏画质滑动条 -->
                <div class="space-y-1.5">
                  <div class="flex items-center justify-between text-xs">
                    <span class="font-medium text-slate-700 dark:text-slate-300">投屏质量</span>
                    <span class="font-mono text-[11px] font-semibold text-violet-600 dark:text-violet-400">
                      {{ qualityPercent }}% · {{ qualityLabel }}
                    </span>
                  </div>
                  <input
                    type="range"
                    min="50"
                    max="100"
                    step="5"
                    v-model.number="qualityPercent"
                    class="w-full h-1.5 rounded-lg appearance-none cursor-pointer bg-slate-200 dark:bg-white/15 accent-violet-600"
                  />
                  <div class="flex justify-between text-[10px] text-slate-400">
                    <span class="cursor-pointer hover:text-violet-600" @click="qualityPercent = 60">720P</span>
                    <span class="cursor-pointer hover:text-violet-600" @click="qualityPercent = 80">1080P</span>
                    <span class="cursor-pointer hover:text-violet-600" @click="qualityPercent = 100">100% 原画</span>
                  </div>
                </div>
              </div>
            </div>

            <!-- ====== 模块 2：编号设备显隐小方块选择区 ====== -->
            <div class="space-y-2 pb-3 border-b border-slate-200/80 dark:border-white/10">
              <div class="flex items-center justify-between">
                <div class="flex items-center gap-2">
                  <span class="text-[11px] font-bold uppercase tracking-wider text-slate-400 dark:text-slate-500">设备显隐</span>
                  <span class="rounded-full bg-emerald-500/15 px-2 py-0.5 text-[10px] font-semibold text-emerald-700 dark:bg-emerald-500/20 dark:text-emerald-300 border border-emerald-500/30">
                    {{ onlineCount }}/{{ totalCount }}
                  </span>
                </div>
                <div class="flex items-center gap-1.5 text-[11px]">
                  <button
                    type="button"
                    class="text-violet-600 dark:text-violet-400 hover:underline cursor-pointer"
                    @click="showAllDevices"
                  >
                    全显
                  </button>
                  <span class="text-slate-300 dark:text-white/20">|</span>
                  <button
                    type="button"
                    class="text-violet-600 dark:text-violet-400 hover:underline cursor-pointer"
                    @click="showOnlySelected"
                  >
                    只显已选
                  </button>
                </div>
              </div>

              <!-- 编号小方块网格 -->
              <div class="grid grid-cols-6 gap-1.5">
                <button
                  v-for="(dev, idx) in devices"
                  :key="dev.serial"
                  type="button"
                  class="relative flex flex-col items-center justify-center h-9 rounded-xl border text-[11px] font-mono font-bold transition-all cursor-pointer"
                  :class="[
                    hiddenSerials.includes(dev.serial)
                      ? 'border-dashed border-slate-300 bg-slate-100/50 text-slate-400 dark:border-white/10 dark:bg-white/5 dark:text-slate-600 line-through'
                      : dev.state === 'device'
                        ? 'border-emerald-500/40 bg-emerald-500/10 text-emerald-800 dark:text-emerald-300 dark:bg-emerald-500/15'
                        : 'border-slate-300 bg-slate-100 text-slate-500 dark:border-white/10 dark:bg-white/5 dark:text-slate-400',
                    selectedSerials.includes(dev.serial) ? 'ring-1.5 ring-violet-500' : '',
                  ]"
                  :title="`${String(idx + 1).padStart(2, '0')}: ${dev.alias || dev.serial} (${hiddenSerials.includes(dev.serial) ? '已隐藏，点击显示' : '显示中，点击隐藏'})`"
                  @click="toggleHide(dev.serial)"
                >
                  <span>{{ String(idx + 1).padStart(2, '0') }}</span>
                  <div
                    v-if="selectedSerials.includes(dev.serial)"
                    class="absolute -top-1 -right-1 h-2.5 w-2.5 rounded-full bg-violet-600 border border-white dark:border-black"
                  />
                </button>
              </div>
            </div>

            <!-- ====== 模块 3：批量目标与动作 ====== -->
            <div class="space-y-2.5 pb-3 border-b border-slate-200/80 dark:border-white/10">
              <div class="flex items-center justify-between">
                <span class="text-[11px] font-bold uppercase tracking-wider text-slate-400 dark:text-slate-500">批量控制</span>
                <span class="text-[11px] text-slate-500 dark:text-slate-400">已选 <b class="text-violet-600 dark:text-violet-400">{{ selectedSerials.length }}</b> 台</span>
              </div>

              <!-- 全选与清空 -->
              <div class="flex items-center gap-2">
                <Button
                  size="sm"
                  variant="outline"
                  class="flex-1 h-7 text-xs rounded-xl cursor-pointer"
                  @click="isAllSelected ? clearSelection() : selectAll()"
                >
                  <component :is="isAllSelected ? CheckSquare : Square" class="h-3.5 w-3.5 mr-1" />
                  {{ isAllSelected ? '取消全选' : '全选在线' }}
                </Button>
                <Button
                  size="sm"
                  variant="ghost"
                  class="h-7 text-xs rounded-xl px-2 text-slate-500 hover:text-slate-900 dark:hover:text-white cursor-pointer"
                  :disabled="selectedSerials.length === 0"
                  @click="clearSelection"
                >
                  清空
                </Button>
              </div>

              <!-- 批量动作按钮行 -->
              <div class="grid grid-cols-3 gap-2 pt-1">
                <Button
                  size="sm"
                  variant="secondary"
                  class="h-8 rounded-xl text-xs gap-1 cursor-pointer"
                  :disabled="batchRunning || selectedSerials.length === 0"
                  @click="triggerBatchAction('back')"
                >
                  <ArrowLeft class="h-3 w-3" />
                  <span>返回</span>
                </Button>
                <Button
                  size="sm"
                  variant="secondary"
                  class="h-8 rounded-xl text-xs gap-1 cursor-pointer"
                  :disabled="batchRunning || selectedSerials.length === 0"
                  @click="triggerBatchAction('home')"
                >
                  <Home class="h-3 w-3" />
                  <span>主页</span>
                </Button>
                <Button
                  size="sm"
                  variant="secondary"
                  class="h-8 rounded-xl text-xs gap-1 cursor-pointer"
                  :disabled="batchRunning || selectedSerials.length === 0"
                  @click="triggerBatchAction('app_switch')"
                >
                  <Layers class="h-3 w-3" />
                  <span>多任务</span>
                </Button>
              </div>
            </div>

            <!-- ====== 模块 4：显示设置（仅在手机视图生效） ====== -->
            <div v-if="viewMode === 'phone'" class="space-y-2 pb-3 border-b border-slate-200/80 dark:border-white/10">
              <span class="text-[11px] font-bold uppercase tracking-wider text-slate-400 dark:text-slate-500">网格显示规格</span>
              <div class="flex items-center justify-between gap-1">
                <div class="flex items-center rounded-xl border border-slate-200/80 bg-slate-100/80 dark:border-white/10 dark:bg-black/20 p-0.5 text-[11px]">
                  <button
                    v-for="size in (['small', 'medium', 'large', 'fill'] as const)"
                    :key="size"
                    type="button"
                    class="rounded-lg px-2 py-0.5 font-medium transition-all cursor-pointer capitalize"
                    :class="dashSize === size
                      ? 'bg-white text-slate-950 shadow-xs font-semibold dark:bg-white/15 dark:text-white'
                      : 'text-slate-500 hover:text-slate-950 dark:text-slate-400 dark:hover:text-white'"
                    @click="dashSize = size"
                  >
                    {{ size === 'fill' ? '铺满' : size === 'small' ? '小' : size === 'medium' ? '中' : '大' }}
                  </button>
                </div>

                <Button
                  size="sm"
                  variant="ghost"
                  class="h-7 px-2 text-xs rounded-xl cursor-pointer"
                  @click="isPageFullscreen = !isPageFullscreen; toast.info(isPageFullscreen ? '进入全屏' : '退出全屏')"
                >
                  <Maximize class="h-3 w-3 mr-1" />
                  <span>{{ isPageFullscreen ? '还原' : '全屏' }}</span>
                </Button>
              </div>
            </div>

            <!-- ====== 模块 5：操作执行历史 ====== -->
            <div class="space-y-2">
              <div class="flex items-center justify-between">
                <span class="text-[11px] font-bold uppercase tracking-wider text-slate-400 dark:text-slate-500">执行反馈</span>
                <button
                  v-if="actionHistory.length > 0"
                  type="button"
                  class="text-[10px] text-slate-400 hover:text-slate-600 dark:hover:text-white cursor-pointer"
                  @click="actionHistory = []"
                >
                  清空
                </button>
              </div>

              <div v-if="actionHistory.length === 0" class="text-center py-4 text-slate-400">
                暂无操作记录
              </div>
              <div v-else class="space-y-1.5 max-h-36 overflow-y-auto">
                <div
                  v-for="item in actionHistory"
                  :key="item.id"
                  class="flex items-start justify-between p-2 rounded-xl bg-slate-50 dark:bg-white/5 border border-slate-100 dark:border-white/5 text-[11px]"
                >
                  <div class="space-y-0.5 truncate">
                    <div class="flex items-center gap-1.5 font-medium text-slate-800 dark:text-slate-200">
                      <span class="text-emerald-500">●</span>
                      <span>{{ item.action }}</span>
                      <span class="text-slate-400">·</span>
                      <span class="truncate max-w-[100px] text-slate-500">{{ item.name }}</span>
                    </div>
                    <p class="text-[10px] text-slate-400">{{ item.message }}</p>
                  </div>
                  <span class="font-mono text-[10px] text-slate-400 shrink-0">{{ item.time }}</span>
                </div>
              </div>
            </div>

          </div>
        </aside>

        <!-- 2. 收起状态 (Collapsed 窄边条) -->
        <aside
          v-else
          aria-label="操作面板（已收起）"
          class="w-13 shrink-0 flex flex-col items-center py-3.5 rounded-3xl border border-slate-200/90 bg-white/95 dark:border-white/15 dark:bg-[#0e121d]/95 backdrop-blur-3xl shadow-lg select-none transition-all duration-300 gap-4"
        >
          <!-- 展开按钮 -->
          <button
            type="button"
            class="p-2 rounded-2xl bg-violet-50 text-violet-600 hover:bg-violet-100 dark:bg-violet-500/10 dark:text-violet-400 dark:hover:bg-violet-500/20 cursor-pointer transition-colors shadow-2xs"
            title="展开操作面板"
            aria-label="展开操作面板"
            @click="isPanelOpen = true"
          >
            <PanelRightOpen class="h-5 w-5" />
          </button>

          <!-- 快速搜索触发展开 -->
          <button
            type="button"
            class="p-2 rounded-xl text-slate-400 hover:text-slate-900 hover:bg-slate-100 dark:hover:text-white dark:hover:bg-white/10 cursor-pointer transition-colors"
            title="搜索设备"
            @click="expandAndFocusSearch"
          >
            <Search class="h-4 w-4" />
          </button>

          <!-- 快速刷新 -->
          <button
            type="button"
            class="p-2 rounded-xl text-slate-400 hover:text-slate-900 hover:bg-slate-100 dark:hover:text-white dark:hover:bg-white/10 cursor-pointer transition-colors"
            title="刷新设备列表"
            @click="triggerRefresh"
          >
            <RefreshCw class="h-4 w-4" :class="{ 'animate-spin': isRefreshing }" />
          </button>

          <!-- 快速切换视图 -->
          <button
            type="button"
            class="p-2 rounded-xl text-slate-400 hover:text-slate-900 hover:bg-slate-100 dark:hover:text-white dark:hover:bg-white/10 cursor-pointer transition-colors"
            :title="`切换视图 (当前: ${viewMode === 'deck' ? '卡片' : '手机'})`"
            @click="viewMode = viewMode === 'deck' ? 'phone' : 'deck'"
          >
            <component :is="viewMode === 'deck' ? LayoutGrid : Smartphone" class="h-4 w-4 text-violet-600 dark:text-violet-400" />
          </button>

          <!-- 已选设备微缩徽章 -->
          <div
            v-if="selectedSerials.length > 0"
            class="flex flex-col items-center gap-1 cursor-pointer"
            title="点击展开已选操作"
            @click="isPanelOpen = true"
          >
            <div class="h-6 w-6 rounded-full bg-violet-600 text-white font-bold text-[10px] flex items-center justify-center shadow-xs">
              {{ selectedSerials.length }}
            </div>
            <span class="text-[9px] text-slate-400">已选</span>
          </div>

          <div class="h-px w-6 bg-slate-200 dark:bg-white/10 my-1" />

          <!-- 竖排文字“操 作” -->
          <div
            class="flex flex-col items-center text-xs font-bold text-slate-400 hover:text-violet-600 dark:hover:text-violet-400 tracking-widest cursor-pointer py-1 select-none"
            title="点击展开操作面板"
            @click="isPanelOpen = true"
          >
            <span>操</span>
            <span class="mt-1">作</span>
          </div>
        </aside>

      </div>
    </div>
  </div>
</template>
