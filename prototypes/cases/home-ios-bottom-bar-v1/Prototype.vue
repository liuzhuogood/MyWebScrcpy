<script setup lang="ts">
import { computed, ref } from 'vue'
import {
  Battery,
  BatteryCharging,
  CheckCircle2,
  Globe,
  Moon,
  Plus,
  Search,
  Smartphone,
  Square,
  Sun,
  Thermometer,
} from 'lucide-vue-next'
import { toast } from 'vue-sonner'
import { Badge } from '@/components/ui/badge'
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
import { Toaster } from '@/components/ui/sonner'
import HomeAuroraBackground from '@/components/home/HomeAuroraBackground.vue'
import IosBottomBar from './components/IosBottomBar.vue'
import { MOCK_DEVICES, type MockDevice } from './mockDevices'

// 模拟设备数据源
const devices = ref<MockDevice[]>(JSON.parse(JSON.stringify(MOCK_DEVICES)))

// 搜索关键词
const keyword = ref('')

// 视图模式：'deck'（卡片视图）与 'phone'（紧密手机视图）
const viewMode = ref<'deck' | 'phone'>('deck')

// 手机卡片缩放比例 (70% ~ 140%，默认 100%)
const phoneScale = ref(100)
const cardWidth = computed(() => Math.round(270 * (phoneScale.value / 100)))

// 投屏码率 (2 ~ 16 Mbps)
const bitrateMbps = ref(8)

// 投屏质量 (50% ~ 100%)
const qualityPercent = ref(90)

// 批量勾选设备（存储选中的 serial）
const selectedSerials = ref<string[]>([devices.value[0].serial, devices.value[1].serial])

// 批量操作运行状态
const batchRunning = ref(false)

// 实时同步操作状态
const isSyncActive = ref(false)
function toggleSync() {
  if (selectedSerials.value.length === 0) {
    toast.error('请先勾选需要参与同步操作的设备')
    return
  }
  isSyncActive.value = !isSyncActive.value
  if (isSyncActive.value) {
    toast.success(`已开启同步操作：对主控设备的操作将实时同步至已选的 ${selectedSerials.value.length} 台设备`)
  } else {
    toast.info('已关闭同步操作')
  }
}

// 刷新设备列表模拟
const isRefreshing = ref(false)
function triggerRefresh() {
  isRefreshing.value = true
  setTimeout(() => {
    isRefreshing.value = false
    toast.success('设备列表已刷新，发现 5 台在线设备')
  }, 500)
}

// 连接网络设备对话框
const isAddDialogOpen = ref(false)
const newDeviceIp = ref('')
const newDevicePort = ref('5555')

function handleAddDevice() {
  if (!newDeviceIp.value.trim()) {
    toast.error('请输入设备 IP 地址')
    return
  }
  const newSerial = `${newDeviceIp.value.trim()}:${newDevicePort.value.trim() || '5555'}`
  const existing = devices.value.find(d => d.serial === newSerial)
  if (existing) {
    toast.info('该设备已在列表中')
    isAddDialogOpen.value = false
    return
  }
  devices.value.push({
    serial: newSerial,
    state: 'device',
    source: 'manual',
    alias: `新连接设备 (${newDeviceIp.value.trim()})`,
    telemetry: {
      brand: 'Android',
      manufacturer: 'Generic',
      model: 'Android Device',
      androidRelease: '14',
      batteryLevel: 95,
      batteryCharging: true,
      batteryTempCelsius: 32.0,
      cpuTempSupported: false,
      cpuUsagePercent: 15,
      currentApp: '桌面主屏幕',
      currentPackage: 'com.android.launcher',
      iconType: 'system',
      resolution: '1080 × 2400',
      transportSpeed: 'Wi-Fi 6',
      screenOn: true,
      screenLocked: false,
    },
  })
  toast.success(`成功连接网络设备: ${newSerial}`)
  isAddDialogOpen.value = false
  newDeviceIp.value = ''
}

// 过滤后的设备列表（包含在线与离线设备，受搜索关键字筛选）
const filteredDevices = computed(() => {
  return devices.value.filter((d) => {
    if (keyword.value.trim()) {
      const q = keyword.value.trim().toLowerCase()
      const matchAlias = (d.alias || '').toLowerCase().includes(q)
      const matchModel = (d.telemetry.model || '').toLowerCase().includes(q)
      const matchSerial = d.serial.toLowerCase().includes(q)
      const matchApp = (d.telemetry.currentApp || '').toLowerCase().includes(q)
      if (!matchAlias && !matchModel && !matchSerial && !matchApp) {
        return false
      }
    }
    return true
  })
})

// 编号设备稳定映射 (01..N)
const numberedDevices = computed(() => {
  const sorted = [...filteredDevices.value].sort((a, b) => a.serial.localeCompare(b.serial))
  return sorted.map((d, index) => {
    const num = index + 1
    const label = num < 10 ? `0${num}` : String(num)
    return {
      device: d,
      serial: d.serial,
      label,
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

// 可见设备列表（按搜索过滤）
const visibleDevices = computed(() => {
  return filteredDevices.value
})

// 在线可选设备
const selectableDevices = computed(() => {
  return visibleDevices.value.filter(d => d.state === 'device')
})

// 批量勾选切换
function toggleSelect(serial: string) {
  if (selectedSerials.value.includes(serial)) {
    selectedSerials.value = selectedSerials.value.filter(s => s !== serial)
  } else {
    selectedSerials.value = [...selectedSerials.value, serial]
  }
}

function selectAll() {
  const onlineSerials = selectableDevices.value.map(d => d.serial)
  const isAll = onlineSerials.length > 0 && onlineSerials.every(s => selectedSerials.value.includes(s))
  if (isAll) {
    selectedSerials.value = []
    toast.info('已取消全选')
  } else {
    selectedSerials.value = onlineSerials
    toast.success(`已全选 ${onlineSerials.length} 台在线设备`)
  }
}

function clearSelection() {
  selectedSerials.value = []
}

// 批量手势触发
function handleBatchAction(action: 'back' | 'home' | 'app_switch') {
  if (selectedSerials.value.length === 0) {
    toast.error('请先选择目标设备')
    return
  }
  const actionName = action === 'back' ? '返回' : action === 'home' ? '主页' : '多任务'
  batchRunning.value = true
  toast.promise(
    new Promise((resolve) => {
      setTimeout(() => {
        batchRunning.value = false
        resolve(true)
      }, 600)
    }),
    {
      loading: `正在向 ${selectedSerials.value.length} 台设备派发【${actionName}】指令...`,
      success: `【${actionName}】指令已成功派发至 ${selectedSerials.value.length} 台设备`,
      error: '派发失败',
    },
  )
}

// 明暗模式切换（供原型内部调试）
const isDarkMode = ref(false)
function toggleDarkMode() {
  isDarkMode.value = !isDarkMode.value
  if (isDarkMode.value) {
    document.documentElement.classList.add('dark')
  } else {
    document.documentElement.classList.remove('dark')
  }
}
</script>

<template>
  <div class="relative min-h-screen bg-slate-50 text-slate-900 transition-colors dark:bg-[#060810] dark:text-slate-100 flex flex-col">
    <Toaster position="top-center" rich-colors />
    <HomeAuroraBackground />

    <!-- 顶部极简导航栏 -->
    <header class="relative z-20 flex shrink-0 items-center justify-between border-b border-slate-200/80 bg-white/70 px-6 py-3.5 backdrop-blur-2xl dark:border-white/10 dark:bg-black/30">
      <div class="flex items-center gap-3">
        <div class="flex h-9 w-9 items-center justify-center rounded-2xl bg-gradient-to-tr from-blue-600 to-indigo-600 text-white shadow-md shadow-blue-500/25">
          <Smartphone class="h-5 w-5" />
        </div>
        <div>
          <div class="flex items-center gap-2">
            <h1 class="text-sm font-bold tracking-tight text-slate-900 dark:text-white">MyWebScrcpy</h1>
            <span class="rounded-full bg-blue-500/10 px-2 py-0.5 text-[10px] font-semibold text-blue-600 dark:bg-blue-500/20 dark:text-blue-300">
              iOS 悬浮底栏原型
            </span>
          </div>
          <p class="text-[11px] text-slate-400">底部居中悬浮 · 零空间侵占 · 沉浸式操控</p>
        </div>
      </div>

      <!-- 右侧辅助工具 -->
      <div class="flex items-center gap-2">
        <button
          type="button"
          class="flex h-8 items-center gap-1.5 rounded-full border border-slate-200/80 bg-white/80 px-3 text-xs font-medium text-slate-700 shadow-xs hover:bg-slate-100 dark:border-white/10 dark:bg-white/5 dark:text-slate-300 dark:hover:bg-white/10 cursor-pointer"
          @click="toggleDarkMode"
        >
          <component :is="isDarkMode ? Sun : Moon" class="h-3.5 w-3.5 text-amber-500" />
          <span>{{ isDarkMode ? '亮色' : '暗色' }}</span>
        </button>

        <Button
          size="sm"
          class="h-8 rounded-full bg-blue-600 px-3.5 text-xs text-white shadow-sm hover:bg-blue-700"
          @click="isAddDialogOpen = true"
        >
          <Plus class="mr-1 h-3.5 w-3.5" />
          连接设备
        </Button>
      </div>
    </header>

    <!-- 主展示区：无右侧固定侧栏，100% 宽度呈现设备内容，底部留足空间给悬浮底栏 -->
    <main class="relative z-10 flex-1 px-4 sm:px-6 lg:px-8 py-6 pb-32">
      <!-- 搜索状态提示胶囊（如果有过滤） -->
      <div v-if="keyword" class="mb-4 flex items-center justify-between rounded-2xl border border-blue-500/20 bg-blue-50/60 px-4 py-2 text-xs text-blue-700 dark:bg-blue-950/20 dark:text-blue-300">
        <div class="flex items-center gap-2">
          <Search class="h-3.5 w-3.5" />
          <span>正在筛选关键字: <strong>"{{ keyword }}"</strong></span>
        </div>
        <button type="button" class="cursor-pointer font-medium hover:underline" @click="keyword = ''">清除筛选</button>
      </div>

      <!-- 设备为空状态 -->
      <div
        v-if="visibleDevices.length === 0"
        class="flex flex-col items-center justify-center rounded-3xl border border-slate-200/80 bg-white/70 p-16 text-center backdrop-blur-2xl shadow-sm dark:border-white/10 dark:bg-white/[0.03] mt-8"
      >
        <div class="flex h-16 w-16 items-center justify-center rounded-2xl bg-gradient-to-tr from-blue-600 to-cyan-500 text-white border border-white/20 mb-4 shadow-lg shadow-blue-500/20">
          <Smartphone class="h-8 w-8 text-white" />
        </div>
        <h3 class="text-lg font-bold text-slate-900 dark:text-white">
          未检测到匹配设备
        </h3>
        <p class="mt-1.5 text-xs text-slate-500 dark:text-slate-400 max-w-md">
          请检查设备搜索关键字，或点击下方按钮重新扫描连接。
        </p>
        <div class="mt-6 flex items-center gap-3">
          <Button
            size="sm"
            class="rounded-2xl bg-blue-600 text-white px-4 text-xs shadow-md"
            @click="isAddDialogOpen = true"
          >
            连接设备
          </Button>
          <Button
            size="sm"
            variant="outline"
            class="rounded-2xl border-slate-300 bg-white dark:border-white/10 dark:bg-white/10"
            @click="triggerRefresh"
          >
            重新扫描
          </Button>
        </div>
      </div>

      <!-- 视图 1：3D拟物手机卡片视图 (Deck View) -->
      <div
        v-else-if="viewMode === 'deck'"
        class="grid grid-cols-1 gap-6 sm:grid-cols-2 md:grid-cols-3 xl:grid-cols-4 2xl:grid-cols-5 transition-all"
      >
        <div
          v-for="device in visibleDevices"
          :key="device.serial"
          class="group relative flex flex-col rounded-3xl border border-slate-200/90 bg-white/80 p-5 shadow-sm backdrop-blur-xl transition-all duration-300 hover:-translate-y-1 hover:border-blue-400 hover:shadow-xl dark:border-white/10 dark:bg-[#0f1422]/70 dark:hover:border-blue-500/50"
          :class="{
            'ring-2 ring-blue-600 shadow-blue-500/10': selectedSerials.includes(device.serial),
            'opacity-60': device.state !== 'device',
          }"
          :style="{ width: `${cardWidth}px`, maxWidth: '100%', margin: '0 auto' }"
        >
          <!-- 卡片顶栏：序列号编号 + 选中框 + 状态点 -->
          <div class="flex items-center justify-between pb-3 border-b border-slate-100 dark:border-white/5">
            <div class="flex items-center gap-2">
              <span class="flex h-5 w-7 items-center justify-center rounded-md bg-slate-100 font-mono text-[11px] font-bold text-slate-700 dark:bg-white/10 dark:text-slate-200">
                {{ serialToNumberMap.get(device.serial) || '00' }}
              </span>
              <span
                class="flex items-center gap-1 text-[11px] font-medium"
                :class="device.state === 'device' ? 'text-emerald-600 dark:text-emerald-400' : 'text-amber-500'"
              >
                <span class="h-1.5 w-1.5 rounded-full" :class="device.state === 'device' ? 'bg-emerald-500 animate-pulse' : 'bg-amber-500'" />
                {{ device.state === 'device' ? '在线' : '离线' }}
              </span>
            </div>

            <!-- 选择复选框 -->
            <button
              type="button"
              class="flex h-6 w-6 items-center justify-center rounded-lg border transition-colors cursor-pointer"
              :class="selectedSerials.includes(device.serial)
                ? 'border-blue-600 bg-blue-600 text-white'
                : 'border-slate-300 bg-white hover:border-blue-400 dark:border-white/20 dark:bg-white/10'"
              @click.stop="toggleSelect(device.serial)"
            >
              <CheckCircle2 v-if="selectedSerials.includes(device.serial)" class="h-4 w-4" />
              <Square v-else class="h-3.5 w-3.5 text-transparent" />
            </button>
          </div>

          <!-- 拟物手机外观模拟屏幕 -->
          <div class="my-4 relative flex flex-col justify-between overflow-hidden rounded-2xl border border-slate-800/10 bg-gradient-to-b from-slate-900 via-slate-950 to-black p-3.5 text-white shadow-inner min-h-[170px]">
            <!-- 手机微型刘海 / 挖孔 -->
            <div class="mx-auto h-3 w-12 rounded-full bg-black/60 border border-white/10 mb-2 flex items-center justify-center">
              <div class="h-1 w-1 rounded-full bg-cyan-400/80 mr-1" />
              <div class="h-1.5 w-1.5 rounded-full bg-slate-900 border border-white/20" />
            </div>

            <!-- 屏幕中间前台应用展示 -->
            <div class="my-auto text-center space-y-1">
              <div class="inline-flex h-10 w-10 items-center justify-center rounded-xl bg-gradient-to-tr from-blue-500/20 to-cyan-500/20 border border-white/15 text-white shadow-sm mb-1">
                <Smartphone class="h-5 w-5 text-blue-300" />
              </div>
              <div class="text-xs font-bold text-white tracking-wide truncate max-w-[200px] mx-auto">
                {{ device.telemetry.currentApp || '主屏幕' }}
              </div>
              <div class="font-mono text-[9px] text-slate-400">
                {{ device.telemetry.resolution }}
              </div>
            </div>

            <!-- 底部小横条 Home Bar -->
            <div class="mx-auto h-1 w-16 rounded-full bg-white/30 mt-2" />
          </div>

          <!-- 设备基础硬件指标条 -->
          <div class="space-y-2 pt-1 text-xs">
            <div class="flex items-center justify-between">
              <span class="font-semibold text-slate-800 dark:text-slate-100 truncate max-w-[140px]" :title="device.alias || device.telemetry.model">
                {{ device.alias || device.telemetry.model }}
              </span>
              <Badge variant="outline" class="h-4.5 px-1.5 text-[10px] font-mono">
                Android {{ device.telemetry.androidRelease }}
              </Badge>
            </div>

            <div class="grid grid-cols-2 gap-1.5 text-[11px] text-slate-500 dark:text-slate-400 font-mono">
              <div class="flex items-center gap-1.5">
                <BatteryCharging v-if="device.telemetry.batteryCharging" class="h-3.5 w-3.5 text-emerald-500" />
                <Battery v-else class="h-3.5 w-3.5 text-slate-400" />
                <span>{{ device.telemetry.batteryLevel }}%</span>
              </div>
              <div class="flex items-center gap-1.5">
                <Thermometer class="h-3.5 w-3.5 text-amber-500" />
                <span>{{ device.telemetry.batteryTempCelsius }}°C</span>
              </div>
            </div>
          </div>
        </div>
      </div>

      <!-- 视图 2：紧凑设备墙视图 (Phone View) -->
      <div
        v-else-if="viewMode === 'phone'"
        class="grid grid-cols-2 gap-3 sm:grid-cols-3 md:grid-cols-4 lg:grid-cols-5 xl:grid-cols-6 transition-all"
      >
        <div
          v-for="device in visibleDevices"
          :key="device.serial"
          class="relative flex flex-col overflow-hidden rounded-2xl border border-slate-200/90 bg-black text-white shadow-sm transition-all hover:scale-102 hover:shadow-lg cursor-pointer"
          :class="{
            'ring-2 ring-blue-500': selectedSerials.includes(device.serial),
            'opacity-50': device.state !== 'device',
          }"
          :style="{ minHeight: `${Math.round(200 * (phoneScale / 100))}px` }"
          @click="toggleSelect(device.serial)"
        >
          <!-- 悬浮角标 -->
          <div class="absolute top-2 left-2 z-10 flex items-center gap-1 rounded-md bg-black/70 px-1.5 py-0.5 font-mono text-[10px] font-bold text-white backdrop-blur-md">
            <span>{{ serialToNumberMap.get(device.serial) }}</span>
            <span class="h-1.5 w-1.5 rounded-full" :class="device.state === 'device' ? 'bg-emerald-400' : 'bg-amber-400'" />
          </div>

          <!-- 模拟投屏画面 -->
          <div class="flex-1 flex flex-col items-center justify-center p-3 text-center bg-radial from-slate-800 to-black">
            <Smartphone class="h-8 w-8 text-slate-500 mb-2" />
            <div class="text-[11px] font-bold text-white truncate max-w-[120px]">
              {{ device.alias || device.telemetry.model }}
            </div>
            <div class="text-[9px] text-slate-400 mt-0.5 font-mono">
              {{ device.telemetry.currentApp }}
            </div>
          </div>

          <!-- 勾选指示器 -->
          <div v-if="selectedSerials.includes(device.serial)" class="absolute top-2 right-2 z-10 flex h-5 w-5 items-center justify-center rounded-full bg-blue-600 text-white shadow-sm">
            <CheckCircle2 class="h-3.5 w-3.5" />
          </div>
        </div>
      </div>
    </main>

    <!-- iOS 风格悬浮底部操作栏组件 -->
    <IosBottomBar
      :keyword="keyword"
      :view-mode="viewMode"
      :phone-scale="phoneScale"
      :bitrate-mbps="bitrateMbps"
      :quality-percent="qualityPercent"
      :devices="devices"
      :selected-serials="selectedSerials"
      :batch-running="batchRunning"
      :is-sync-active="isSyncActive"
      @update:keyword="keyword = $event"
      @update:view-mode="viewMode = $event"
      @update:phone-scale="phoneScale = $event"
      @update:bitrate-mbps="bitrateMbps = $event"
      @update:quality-percent="qualityPercent = $event"
      @update:selected-serials="selectedSerials = $event"
      @toggle-select="toggleSelect"
      @select-all="selectAll"
      @clear-selection="clearSelection"
      @trigger-batch-action="handleBatchAction"
      @toggle-sync="toggleSync"
      @open-add="isAddDialogOpen = true"
      @open-config="toast.info('已触发：打开系统配置弹窗')"
      @open-ai="toast.info('已触发：前往 AI 技能集成页面')"
      @open-shortcuts="toast.info('已触发：查看键盘快捷键说明')"
      @open-docs="toast.info('已触发：查看 OpenAPI 接口文档')"
    />

    <!-- 连接网络设备对话框 -->
    <Dialog v-model:open="isAddDialogOpen">
      <DialogContent class="sm:max-w-md rounded-3xl border border-slate-200 bg-white/95 text-slate-900 dark:border-white/15 dark:bg-[#0e121d]/95 dark:text-white backdrop-blur-3xl shadow-2xl">
        <DialogHeader>
          <DialogTitle class="flex items-center gap-2 text-slate-900 dark:text-white font-bold">
            <Globe class="h-5 w-5 text-cyan-600 dark:text-cyan-400" />
            连接网络 Android 设备
          </DialogTitle>
          <DialogDescription class="text-slate-500 dark:text-slate-400 text-xs">
            输入局域网 Android 设备的 IP 地址与端口，建立 ADB 无线调试握手。
          </DialogDescription>
        </DialogHeader>

        <div class="space-y-4 py-2">
          <div class="grid grid-cols-3 gap-2">
            <div class="col-span-2 space-y-1.5">
              <label class="text-xs font-medium text-slate-700 dark:text-slate-300">设备 IP 地址</label>
              <Input
                v-model="newDeviceIp"
                placeholder="例如 192.168.1.100"
                class="rounded-xl border-slate-200 dark:border-white/10 dark:bg-black/20"
                @keydown.enter="handleAddDevice"
              />
            </div>
            <div class="space-y-1.5">
              <label class="text-xs font-medium text-slate-700 dark:text-slate-300">端口</label>
              <Input
                v-model="newDevicePort"
                placeholder="5555"
                class="rounded-xl border-slate-200 dark:border-white/10 dark:bg-black/20"
                @keydown.enter="handleAddDevice"
              />
            </div>
          </div>
        </div>

        <DialogFooter class="flex items-center justify-end gap-2 pt-2">
          <Button
            variant="ghost"
            class="rounded-xl text-xs"
            @click="isAddDialogOpen = false"
          >
            取消
          </Button>
          <Button
            class="rounded-xl bg-blue-600 text-xs text-white hover:bg-blue-700"
            @click="handleAddDevice"
          >
            确认连接
          </Button>
        </DialogFooter>
      </DialogContent>
    </Dialog>
  </div>
</template>
