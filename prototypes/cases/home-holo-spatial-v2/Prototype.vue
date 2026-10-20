<script setup lang="ts">
import { computed, ref } from 'vue'
import {
  MonitorPlay,
  Tv,
  X,
} from 'lucide-vue-next'
import { Button } from '@/components/ui/button'
import { toast } from 'vue-sonner'
import AuroraBackground from './components/AuroraBackground.vue'
import FloatingIslandBar from './components/FloatingIslandBar.vue'
import SpatialCard from './components/SpatialCard.vue'
import DeviceDetailDrawer from './components/DeviceDetailDrawer.vue'
import { type SpatialDevice, INITIAL_SPATIAL_DEVICES } from './mockDevices'

const devices = ref<SpatialDevice[]>(JSON.parse(JSON.stringify(INITIAL_SPATIAL_DEVICES)))
const keyword = ref('')
const activeFilter = ref<'all' | 'usb' | 'wifi'>('all')
const viewMode = ref<'deck' | 'grid' | 'compact'>('deck')

const inspectingDevice = ref<SpatialDevice | null>(null)
const activePip = ref<string | null>(null)

const onlineCount = computed(() => devices.value.filter(d => d.state === 'device').length)

const filteredDevices = computed(() => {
  return devices.value.filter((d) => {
    if (keyword.value.trim()) {
      const q = keyword.value.toLowerCase().trim()
      const match =
        d.name.toLowerCase().includes(q) ||
        d.model.toLowerCase().includes(q) ||
        d.serial.toLowerCase().includes(q) ||
        d.telemetry.currentApp.toLowerCase().includes(q)
      if (!match) return false
    }
    if (activeFilter.value === 'usb') return d.source === 'auto'
    if (activeFilter.value === 'wifi') return d.source === 'manual'
    return true
  })
})

function handleRefresh() {
  toast.info('正在扫描局域网与 USB 空间节点...')
  setTimeout(() => {
    toast.success('空间设备阵列同步就绪')
  }, 500)
}

function handlePlay(serial: string) {
  toast.success(`已激活超高清低时延空间投屏: ${serial}`)
}

function handlePip(serial: string) {
  activePip.value = serial
  toast.info(`已开启全息悬浮微窗: ${serial}`)
}

function handleTerminal(serial: string) {
  toast.info(`正在为 ${serial} 分配交互式 Web PTY...`)
}

function handleReconnect(serial: string) {
  const dev = devices.value.find(d => d.serial === serial)
  if (dev) {
    dev.state = 'device'
    toast.success(`设备已恢复握手: ${serial}`)
  }
}

function handleOpenAdd() {
  const ip = `192.168.1.${Math.floor(Math.random() * 80 + 120)}:5555`
  const newDev: SpatialDevice = {
    serial: ip,
    name: '新增无线平板 (iPad / Android)',
    model: 'Spatial Pad Pro',
    source: 'manual',
    state: 'device',
    telemetry: {
      fps: 120,
      tempCelsius: 31.0,
      batteryPercent: 90,
      batteryCharging: true,
      latencyMs: 9,
      resolution: '1600 × 2560',
      androidVersion: 'Android 14',
      currentApp: '控制台主页',
      currentPackage: 'com.mywebscrcpy.desktop',
      ip: ip.split(':')[0],
      aspectRatio: '16:10',
    },
  }
  devices.value.unshift(newDev)
  toast.success(`已连接并发现新空间设备: ${ip}`)
}
</script>

<template>
  <div class="relative min-h-[90vh] pb-20 text-white select-none">
    <!-- 极光背景 -->
    <AuroraBackground />

    <div class="relative z-10 mx-auto max-w-7xl px-4 pt-4 sm:px-6 space-y-6">
      <!-- 顶部悬浮动态岛控制胶囊 -->
      <FloatingIslandBar
        :keyword="keyword"
        :active-filter="activeFilter"
        :view-mode="viewMode"
        :total-count="devices.length"
        :online-count="onlineCount"
        @update:keyword="keyword = $event"
        @update:active-filter="activeFilter = $event as any"
        @update:view-mode="viewMode = $event"
        @refresh="handleRefresh"
        @open-add="handleOpenAdd"
      />

      <!-- 视图模式一：3D 全息甲板视图 (Holo Deck) -->
      <div
        v-if="viewMode === 'deck'"
        class="grid grid-cols-1 gap-6 sm:grid-cols-2 lg:grid-cols-3"
      >
        <SpatialCard
          v-for="(device, idx) in filteredDevices"
          :key="device.serial"
          :device="device"
          :index="idx"
          @play="handlePlay"
          @pip="handlePip"
          @inspect="inspectingDevice = $event"
          @terminal="handleTerminal"
          @reconnect="handleReconnect"
        />
      </div>

      <!-- 视图模式二：紧凑型空间阵列 (Compact Matrix) -->
      <div
        v-else-if="viewMode === 'compact'"
        class="grid grid-cols-1 gap-3 sm:grid-cols-2 lg:grid-cols-4"
      >
        <div
          v-for="device in filteredDevices"
          :key="device.serial"
          class="group rounded-2xl border border-white/10 bg-white/[0.03] p-4 backdrop-blur-xl hover:border-violet-400/40 hover:bg-white/[0.06] transition-all flex flex-col justify-between"
        >
          <div class="flex items-center justify-between">
            <span
              class="h-2 w-2 rounded-full"
              :class="device.state === 'device' ? 'bg-emerald-400 animate-pulse' : 'bg-slate-500'"
            />
            <span class="text-[10px] text-slate-400 font-mono">{{ device.telemetry.fps }} FPS</span>
          </div>
          <div class="mt-2">
            <h4 class="font-medium text-sm text-white truncate">{{ device.name }}</h4>
            <p class="text-[11px] text-slate-400 font-mono truncate">{{ device.serial }}</p>
          </div>
          <div class="mt-3 flex items-center justify-between pt-2 border-t border-white/5">
            <span class="text-[10px] text-violet-300 truncate max-w-[120px]">{{ device.telemetry.currentApp }}</span>
            <Button
              size="sm"
              variant="ghost"
              class="h-7 text-xs text-white hover:bg-violet-600/30"
              @click="handlePlay(device.serial)"
            >
              投屏
            </Button>
          </div>
        </div>
      </div>

      <!-- 空结果提示 -->
      <div
        v-if="filteredDevices.length === 0"
        class="flex flex-col items-center justify-center rounded-3xl border border-dashed border-white/10 bg-white/[0.02] p-12 text-center backdrop-blur-md"
      >
        <p class="text-sm text-slate-400">未检索到符合条件的设备</p>
        <Button
          size="sm"
          variant="outline"
          class="mt-4 rounded-xl border-white/10 text-xs text-white hover:bg-white/10"
          @click="keyword = ''; activeFilter = 'all'"
        >
          重置检索条件
        </Button>
      </div>
    </div>

    <!-- 侧边设备详情抽屉 -->
    <DeviceDetailDrawer
      :device="inspectingDevice"
      @close="inspectingDevice = null"
      @play="handlePlay"
    />

    <!-- 画中画独立微窗 -->
    <div
      v-if="activePip"
      class="fixed bottom-6 right-6 z-40 w-72 rounded-3xl border border-white/20 bg-black/85 p-4 shadow-2xl backdrop-blur-2xl animate-in slide-in-from-bottom-5"
    >
      <div class="flex items-center justify-between border-b border-white/10 pb-2 mb-3">
        <div class="flex items-center gap-1.5 text-xs font-medium text-violet-300">
          <Tv class="h-3.5 w-3.5" />
          <span>全息画中画微窗</span>
        </div>
        <Button variant="ghost" size="icon" class="h-6 w-6 text-slate-400 hover:text-white" @click="activePip = null">
          <X class="h-3.5 w-3.5" />
        </Button>
      </div>
      <div class="flex h-56 flex-col items-center justify-center rounded-2xl bg-white/[0.03] border border-white/5 text-center p-3">
        <MonitorPlay class="h-8 w-8 text-violet-400 mb-2 animate-bounce" />
        <span class="text-xs text-white font-medium">{{ activePip }}</span>
        <span class="text-[10px] text-slate-400 mt-1">支持鼠标手势与键盘快捷注入</span>
      </div>
    </div>
  </div>
</template>
