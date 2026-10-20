<script setup lang="ts">
import { computed, ref } from 'vue'
import {
  CheckSquare,
  LayoutGrid,
  Search,
  Tv,
  X,
} from 'lucide-vue-next'
import { Button } from '@/components/ui/button'
import { Input } from '@/components/ui/input'
import { toast } from 'vue-sonner'
import StarshipHeader from './components/StarshipHeader.vue'
import FleetMatrixGrid from './components/FleetMatrixGrid.vue'
import TacticalCockpit from './components/TacticalCockpit.vue'
import { type FleetDevice, INITIAL_FLEET_DEVICES } from './mockDevices'

const devices = ref<FleetDevice[]>(JSON.parse(JSON.stringify(INITIAL_FLEET_DEVICES)))
const selectedDevice = ref<FleetDevice | null>(devices.value[0] || null)
const batchSelectedSerials = ref<string[]>([devices.value[0]?.serial || ''])
const keyword = ref('')
const activeFilter = ref<'all' | 'usb' | 'wifi' | 'alert'>('all')

const isTileModalOpen = ref(false)
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
        d.currentApp.toLowerCase().includes(q)
      if (!match) return false
    }
    if (activeFilter.value === 'usb') return d.source === 'auto'
    if (activeFilter.value === 'wifi') return d.source === 'manual'
    if (activeFilter.value === 'alert') return d.state !== 'device'
    return true
  })
})

function handleToggleBatch(serial: string) {
  if (batchSelectedSerials.value.includes(serial)) {
    batchSelectedSerials.value = batchSelectedSerials.value.filter(s => s !== serial)
  } else {
    batchSelectedSerials.value.push(serial)
  }
}

function handleSelectAll() {
  if (batchSelectedSerials.value.length === filteredDevices.value.length) {
    batchSelectedSerials.value = []
  } else {
    batchSelectedSerials.value = filteredDevices.value.map(d => d.serial)
  }
}

function handleRefresh() {
  toast.info('正在扫描星舰 ADB 集群战位...')
  setTimeout(() => {
    toast.success('全域战位同步完成：6 个节点就绪')
  }, 500)
}

function handleTileAll() {
  isTileModalOpen.value = true
  toast.success('已拉起全量小窗平铺阵列')
}

function handleSyncBroadcast() {
  const count = batchSelectedSerials.value.length || devices.value.length
  toast.success(`[群控广播] 已向 ${count} 台战位同步广播触控与按键流`)
}

function handleScreenOff() {
  toast.info('已向全部在线战位发送息屏省电指令 (KEYCODE_POWER)')
}

function handlePlay(serial: string) {
  toast.success(`正在激活高帧率视频流投屏: ${serial}`)
}

function handlePip(serial: string) {
  activePip.value = serial
  toast.info(`已开启战位画中画悬浮窗: ${serial}`)
}

function handleOpenAdd() {
  const ip = `192.168.1.${Math.floor(Math.random() * 80 + 130)}:5555`
  const newDev: FleetDevice = {
    serial: ip,
    name: '新加入战备战位',
    model: 'Combat Node Pro',
    source: 'manual',
    state: 'device',
    fps: 90,
    tempCelsius: 32.5,
    batteryPercent: 85,
    batteryCharging: true,
    latencyMs: 12,
    resolution: '1440 × 3120',
    dpi: 520,
    androidVersion: 'Android 14',
    currentApp: '战术控制台',
    currentPackage: 'com.combat.control',
    currentActivity: 'com.combat.MainActivity',
    ip: ip.split(':')[0],
    streamBitrateMbps: 9.8,
  }
  devices.value.unshift(newDev)
  selectedDevice.value = newDev
  toast.success(`已加入新战位: ${ip}`)
}
</script>

<template>
  <div class="relative min-h-[90vh] pb-16 text-slate-100 select-none bg-[#03060c]">
    <!-- 背景科技星际微光网格 -->
    <div
      class="pointer-events-none fixed inset-0 opacity-[0.07]"
      style="background-image: linear-gradient(#3b82f6 1px, transparent 1px), linear-gradient(90deg, #3b82f6 1px, transparent 1px); background-size: 40px 40px;"
    />

    <div class="relative z-10 mx-auto max-w-7xl px-4 pt-4 sm:px-6 space-y-5">
      <!-- 顶部星舰总控台顶栏 -->
      <StarshipHeader
        :total-count="devices.length"
        :online-count="onlineCount"
        :selected-count="batchSelectedSerials.length"
        @refresh="handleRefresh"
        @tile-all="handleTileAll"
        @sync-broadcast="handleSyncBroadcast"
        @screen-off="handleScreenOff"
        @open-add="handleOpenAdd"
      />

      <!-- 主指挥舱双栏布局 -->
      <div class="grid grid-cols-1 gap-5 lg:grid-cols-12 items-start">
        <!-- 左侧：多机矩阵阵列 (8 栏) -->
        <div class="lg:col-span-8 space-y-4">
          <!-- 检索与快捷批选条 -->
          <div class="flex flex-col gap-3 rounded-2xl border border-blue-500/20 bg-[#070b14]/90 p-3.5 backdrop-blur-md sm:flex-row sm:items-center sm:justify-between font-mono text-xs">
            <div class="flex flex-1 items-center gap-2">
              <div class="relative flex-1 max-w-sm">
                <Search class="absolute left-3 top-1/2 h-3.5 w-3.5 -translate-y-1/2 text-blue-400" />
                <Input
                  v-model="keyword"
                  placeholder="检索战位 / 序列号 / 型号 / Activity..."
                  class="h-8 border-blue-500/30 bg-black/60 pl-8 text-xs text-blue-200 placeholder:text-slate-600 focus:border-blue-400"
                />
              </div>

              <!-- 分类过滤 Tab -->
              <div class="flex items-center rounded-lg border border-blue-500/20 bg-blue-950/30 p-0.5">
                <button
                  class="rounded px-2.5 py-1 transition-all"
                  :class="activeFilter === 'all' ? 'bg-blue-600 font-semibold text-white' : 'text-slate-400 hover:text-white'"
                  @click="activeFilter = 'all'"
                >
                  全部
                </button>
                <button
                  class="rounded px-2.5 py-1 transition-all"
                  :class="activeFilter === 'usb' ? 'bg-blue-600 font-semibold text-white' : 'text-slate-400 hover:text-white'"
                  @click="activeFilter = 'usb'"
                >
                  USB
                </button>
                <button
                  class="rounded px-2.5 py-1 transition-all"
                  :class="activeFilter === 'wifi' ? 'bg-blue-600 font-semibold text-white' : 'text-slate-400 hover:text-white'"
                  @click="activeFilter = 'wifi'"
                >
                  Wi-Fi
                </button>
              </div>
            </div>

            <div class="flex items-center gap-3 text-slate-400">
              <button
                class="flex items-center gap-1.5 text-blue-300 hover:text-white"
                @click="handleSelectAll"
              >
                <CheckSquare class="h-3.5 w-3.5" />
                <span>{{ batchSelectedSerials.length === filteredDevices.length ? '取消全选' : '全选阵列' }}</span>
              </button>
              <span>已选: <strong class="text-cyan-300">{{ batchSelectedSerials.length }}</strong> 台</span>
            </div>
          </div>

          <!-- 设备矩阵卡片网格 -->
          <FleetMatrixGrid
            :devices="filteredDevices"
            :selected-serial="selectedDevice?.serial || null"
            :batch-selected-serials="batchSelectedSerials"
            @select="selectedDevice = $event"
            @toggle-batch="handleToggleBatch"
            @play="handlePlay"
            @pip="handlePip"
          />
        </div>

        <!-- 右侧：战术深研舱 (4 栏) -->
        <div class="lg:col-span-4 sticky top-4">
          <TacticalCockpit
            :device="selectedDevice"
            @play="handlePlay"
          />
        </div>
      </div>
    </div>

    <!-- 平铺大屏预览浮层 -->
    <div
      v-if="isTileModalOpen"
      class="fixed inset-0 z-50 flex flex-col bg-black/95 p-6 backdrop-blur-2xl animate-in fade-in"
    >
      <div class="flex items-center justify-between border-b border-blue-500/20 pb-3 font-mono">
        <div class="flex items-center gap-2 text-sm text-cyan-400 font-bold">
          <LayoutGrid class="h-4 w-4" />
          <span>全域战位小窗阵列平铺监控墙 ({{ onlineCount }} 台运行中)</span>
        </div>
        <Button variant="ghost" size="icon" class="text-slate-400 hover:text-white" @click="isTileModalOpen = false">
          <X class="h-4 w-4" />
        </Button>
      </div>

      <div class="flex-1 grid grid-cols-2 sm:grid-cols-3 gap-4 p-4 overflow-y-auto">
        <div
          v-for="d in devices.filter(d => d.state === 'device')"
          :key="d.serial"
          class="relative flex flex-col justify-between rounded-xl border border-blue-500/30 bg-[#070b14] p-3 text-xs font-mono"
        >
          <div class="flex items-center justify-between text-slate-400 border-b border-slate-800 pb-1.5">
            <span class="truncate font-semibold text-white">{{ d.name }}</span>
            <span class="text-cyan-400 text-[10px]">{{ d.fps }} FPS</span>
          </div>
          <div class="flex-1 flex flex-col items-center justify-center py-6 text-center text-slate-500">
            <Tv class="h-8 w-8 text-blue-500 mb-2 animate-pulse" />
            <span class="text-[11px] text-slate-300 font-sans">{{ d.currentApp }}</span>
            <span class="text-[10px] text-slate-500">{{ d.resolution }}</span>
          </div>
          <div class="flex items-center justify-between border-t border-slate-800 pt-1.5 text-[10px] text-slate-400">
            <span>码率 {{ d.streamBitrateMbps }} Mbps</span>
            <span class="text-emerald-400">{{ d.latencyMs }} ms</span>
          </div>
        </div>
      </div>
    </div>
  </div>
</template>
