<script setup lang="ts">
import { computed, ref } from 'vue'
import {
  MonitorPlay,
  RefreshCw,
  Search,
  Terminal,
  Tv,
  Usb,
  Wifi,
  X,
} from 'lucide-vue-next'
import { Button } from '@/components/ui/button'
import { Input } from '@/components/ui/input'
import { toast } from 'vue-sonner'
import CyberBackground from './components/CyberBackground.vue'
import TelemetryBar from './components/TelemetryBar.vue'
import CyberDeviceCard from './components/CyberDeviceCard.vue'
import AddDeviceModal from './components/AddDeviceModal.vue'
import { type CyberDevice, INITIAL_CYBER_DEVICES } from './mockDevices'

const devices = ref<CyberDevice[]>(JSON.parse(JSON.stringify(INITIAL_CYBER_DEVICES)))
const keyword = ref('')
const activeFilter = ref<'all' | 'usb' | 'wifi' | 'alert'>('all')
const isAddModalOpen = ref(false)

// 浮动小窗状态模拟
const activePip = ref<string | null>(null)
// 终端抽屉状态模拟
const activeTerminal = ref<CyberDevice | null>(null)
const terminalInput = ref('')
const terminalLogs = ref<string[]>([
  'Connected to adb server daemon at 127.0.0.1:5037',
  'Device state: authenticated, transport: raw_scrcpy_stream',
  'Shell initialized. Ready for commands (e.g. dumpsys, pm, input)...',
])

const filteredDevices = computed(() => {
  return devices.value.filter((d) => {
    // 关键字搜索
    if (keyword.value.trim()) {
      const q = keyword.value.toLowerCase().trim()
      const match =
        d.name.toLowerCase().includes(q) ||
        d.model.toLowerCase().includes(q) ||
        d.serial.toLowerCase().includes(q) ||
        d.telemetry.currentApp.toLowerCase().includes(q)
      if (!match) return false
    }

    // 分类筛选
    if (activeFilter.value === 'usb') return d.source === 'auto'
    if (activeFilter.value === 'wifi') return d.source === 'manual'
    if (activeFilter.value === 'alert') return d.state !== 'device'
    return true
  })
})

function handleRefresh() {
  toast.info('正在全网段广播探测 ADB 节点拓扑...')
  setTimeout(() => {
    toast.success('拓扑刷新完成：发现 6 个活跃节点')
  }, 600)
}

function handleAddDevice(payload: { serial: string, alias: string }) {
  const newDev: CyberDevice = {
    serial: payload.serial,
    name: payload.alias,
    model: 'Android Network Node',
    source: 'manual',
    state: 'device',
    telemetry: {
      fps: 60,
      tempCelsius: 32.0,
      batteryPercent: 80,
      batteryCharging: false,
      latencyMs: 14,
      memoryUsageMb: 4200,
      totalMemoryMb: 8192,
      resolution: '1080 × 2400',
      androidVersion: 'Android 14',
      currentApp: '桌面主屏',
      currentPackage: 'com.android.launcher3',
      ip: payload.serial.split(':')[0],
      wifiSsid: 'MATRIX_LAB_5G',
    },
  }
  devices.value.unshift(newDev)
  toast.success(`已成功接入网络设备：${payload.serial}`)
}

function handlePlay(serial: string) {
  toast.success(`正在拉起 2K 高画质投屏控制台：${serial}`)
}

function handlePip(serial: string) {
  activePip.value = serial
  toast.info(`已开启原生画中画独立微窗：${serial}`)
}

function handleTerminal(serial: string) {
  const dev = devices.value.find(d => d.serial === serial)
  if (dev) {
    activeTerminal.value = dev
    terminalLogs.value.push(`\n$ adb -s ${serial} shell`)
    terminalLogs.value.push(`shell@${dev.model}:/$ `)
  }
}

function handleSendTerminalCmd() {
  if (!terminalInput.value.trim()) return
  const cmd = terminalInput.value.trim()
  terminalLogs.value.push(`$ ${cmd}`)
  if (cmd === 'clear') {
    terminalLogs.value = []
  } else if (cmd.startsWith('pm list')) {
    terminalLogs.value.push('package:com.kuaishou.nebula\npackage:com.zhiliaoapp.musically\npackage:com.tencent.mm')
  } else {
    terminalLogs.value.push(`[EXEC_SUCCESS] returncode=0: ${cmd}`)
  }
  terminalInput.value = ''
}

function handleReconnect(serial: string) {
  toast.info(`正在尝试重新协商 TCP/IP 握手：${serial}`)
  const dev = devices.value.find(d => d.serial === serial)
  if (dev) {
    setTimeout(() => {
      dev.state = 'device'
      dev.telemetry.fps = 60
      toast.success(`设备已恢复在线状态：${serial}`)
    }, 800)
  }
}

function handleDelete(serial: string) {
  devices.value = devices.value.filter(d => d.serial !== serial)
  toast.success(`已从控制台拓扑中移除节点：${serial}`)
}
</script>

<template>
  <div class="relative min-h-[90vh] pb-16 text-slate-100 select-none">
    <!-- 动态科技粒子背景 -->
    <CyberBackground />

    <div class="relative z-10 mx-auto max-w-7xl px-4 pt-4 sm:px-6 space-y-5">
      <!-- 遥测总控顶部仪表带 -->
      <TelemetryBar
        :devices="devices"
        :active-filter="activeFilter"
        @refresh="handleRefresh"
        @open-add="isAddModalOpen = true"
        @set-filter="activeFilter = $event as any"
      />

      <!-- 搜索与控制过滤条 -->
      <div class="flex flex-col gap-3 rounded-xl border border-cyan-500/20 bg-slate-950/70 p-3 backdrop-blur-md sm:flex-row sm:items-center sm:justify-between">
        <div class="flex flex-1 items-center gap-2">
          <div class="relative flex-1 max-w-md">
            <Search class="absolute left-3 top-1/2 h-4 w-4 -translate-y-1/2 text-cyan-400" />
            <Input
              v-model="keyword"
              placeholder="快速检索设备名称 / 序列号 / 型号 / 前台 App..."
              class="h-9 border-cyan-500/30 bg-slate-900/80 pl-9 font-mono text-xs text-cyan-200 placeholder:text-slate-500 focus:border-cyan-400"
            />
          </div>

          <!-- 分类筛选 Pill 群 -->
          <div class="flex items-center rounded-lg border border-cyan-500/20 bg-slate-900/80 p-0.5 text-xs font-mono">
            <button
              class="rounded-md px-3 py-1 transition-all"
              :class="activeFilter === 'all' ? 'bg-cyan-500 font-semibold text-slate-950 shadow-[0_0_10px_rgba(0,242,254,0.3)]' : 'text-slate-400 hover:text-white'"
              @click="activeFilter = 'all'"
            >
              全部 ({{ devices.length }})
            </button>
            <button
              class="flex items-center gap-1 rounded-md px-3 py-1 transition-all"
              :class="activeFilter === 'usb' ? 'bg-cyan-500 font-semibold text-slate-950 shadow-[0_0_10px_rgba(0,242,254,0.3)]' : 'text-slate-400 hover:text-white'"
              @click="activeFilter = 'usb'"
            >
              <Usb class="h-3 w-3" />
              USB ({{ devices.filter(d => d.source === 'auto').length }})
            </button>
            <button
              class="flex items-center gap-1 rounded-md px-3 py-1 transition-all"
              :class="activeFilter === 'wifi' ? 'bg-cyan-500 font-semibold text-slate-950 shadow-[0_0_10px_rgba(0,242,254,0.3)]' : 'text-slate-400 hover:text-white'"
              @click="activeFilter = 'wifi'"
            >
              <Wifi class="h-3 w-3" />
              Wi-Fi ({{ devices.filter(d => d.source === 'manual').length }})
            </button>
            <button
              class="rounded-md px-3 py-1 transition-all"
              :class="activeFilter === 'alert' ? 'bg-amber-500 font-semibold text-slate-950' : 'text-slate-400 hover:text-white'"
              @click="activeFilter = 'alert'"
            >
              异常 ({{ devices.filter(d => d.state !== 'device').length }})
            </button>
          </div>
        </div>

        <div class="flex items-center gap-2">
          <Button
            size="sm"
            variant="ghost"
            class="h-8 font-mono text-xs text-cyan-300 hover:bg-cyan-950/40"
            @click="handleRefresh"
          >
            <RefreshCw class="mr-1 h-3.5 w-3.5" />
            刷新
          </Button>
        </div>
      </div>

      <!-- 设备卡片矩阵 -->
      <div v-if="filteredDevices.length > 0" class="grid grid-cols-1 gap-5 md:grid-cols-2 lg:grid-cols-3">
        <CyberDeviceCard
          v-for="(device, idx) in filteredDevices"
          :key="device.serial"
          :device="device"
          :index="idx"
          @play="handlePlay"
          @pip="handlePip"
          @terminal="handleTerminal"
          @reconnect="handleReconnect"
          @delete="handleDelete"
        />
      </div>

      <!-- 空结果提示 -->
      <div
        v-else
        class="flex flex-col items-center justify-center rounded-2xl border border-dashed border-cyan-500/20 bg-slate-950/60 p-12 text-center backdrop-blur-sm"
      >
        <p class="font-mono text-sm text-slate-400">未检索到匹配的赛博矩阵设备节点</p>
        <Button
          size="sm"
          variant="outline"
          class="mt-4 border-cyan-500/30 text-xs font-mono text-cyan-300 hover:bg-cyan-950/40"
          @click="keyword = ''; activeFilter = 'all'"
        >
          重置检索条件
        </Button>
      </div>
    </div>

    <!-- 弹窗：网络设备接入 -->
    <AddDeviceModal
      :open="isAddModalOpen"
      @close="isAddModalOpen = false"
      @add="handleAddDevice"
    />

    <!-- 画中画小窗模拟浮层 -->
    <div
      v-if="activePip"
      class="fixed bottom-6 right-6 z-40 w-72 rounded-2xl border-2 border-cyan-400 bg-black/90 shadow-[0_0_40px_rgba(0,242,254,0.4)] backdrop-blur-xl animate-in slide-in-from-bottom-5"
    >
      <div class="flex items-center justify-between border-b border-cyan-500/20 bg-cyan-950/40 px-3 py-2">
        <div class="flex items-center gap-1.5 font-mono text-xs text-cyan-300">
          <Tv class="h-3.5 w-3.5" />
          <span>PIP // {{ activePip }}</span>
        </div>
        <Button variant="ghost" size="icon" class="h-5 w-5 text-slate-400 hover:text-white" @click="activePip = null">
          <X class="h-3.5 w-3.5" />
        </Button>
      </div>
      <div class="relative flex h-96 flex-col items-center justify-center p-4">
        <div class="text-center font-mono text-xs text-slate-400">
          <div class="mx-auto mb-2 h-10 w-10 rounded-full border border-cyan-500/40 flex items-center justify-center text-cyan-400 animate-spin">
            <MonitorPlay class="h-5 w-5" />
          </div>
          <span class="text-cyan-300 font-semibold block">H.264 60FPS 流畅传输中</span>
          <span class="text-[10px] text-slate-500 mt-1 block">双击展开全屏 · 支持滚轮触控</span>
        </div>
      </div>
    </div>

    <!-- 交互式 ADB Shell 终端抽屉 -->
    <div
      v-if="activeTerminal"
      class="fixed inset-y-0 right-0 z-50 flex w-full max-w-lg flex-col border-l border-cyan-500/30 bg-slate-950 shadow-[0_0_50px_rgba(0,242,254,0.25)] backdrop-blur-xl animate-in slide-in-from-right"
    >
      <div class="flex items-center justify-between border-b border-cyan-500/20 px-4 py-3 bg-slate-900/60">
        <div class="flex items-center gap-2">
          <Terminal class="h-4 w-4 text-cyan-400" />
          <h4 class="font-mono text-xs font-bold text-white">
            ADB SHELL // {{ activeTerminal.name }}
          </h4>
        </div>
        <Button variant="ghost" size="icon" class="h-6 w-6 text-slate-400 hover:text-white" @click="activeTerminal = null">
          <X class="h-4 w-4" />
        </Button>
      </div>

      <!-- 终端输出区 -->
      <div class="flex-1 overflow-y-auto p-4 font-mono text-xs text-cyan-300 space-y-1.5 bg-black/90">
        <div v-for="(log, idx) in terminalLogs" :key="idx" class="whitespace-pre-wrap leading-relaxed">
          {{ log }}
        </div>
      </div>

      <!-- 命令行输入 -->
      <div class="border-t border-cyan-500/20 p-3 bg-slate-900/70">
        <form class="flex items-center gap-2" @submit.prevent="handleSendTerminalCmd">
          <span class="font-mono text-xs text-emerald-400">$</span>
          <Input
            v-model="terminalInput"
            placeholder="输入 ADB shell 命令 (如: pm list packages, wm size)..."
            class="h-8 flex-1 border-cyan-500/30 bg-black font-mono text-xs text-cyan-200 placeholder:text-slate-600 focus:border-cyan-400"
          />
          <Button type="submit" size="sm" class="h-8 bg-cyan-500 font-mono text-xs font-semibold text-slate-950 hover:bg-cyan-400">
            执行
          </Button>
        </form>
      </div>
    </div>
  </div>
</template>
