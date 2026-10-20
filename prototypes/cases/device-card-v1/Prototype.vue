<script setup lang="ts">
import { computed, onMounted, onUnmounted, ref } from 'vue'
import {
  Activity,
  Battery,
  Layers,
  RotateCcw,
  Sparkles,
  Thermometer,
} from 'lucide-vue-next'
import { Button } from '@/components/ui/button'
import { Input } from '@/components/ui/input'
import { toast } from 'vue-sonner'
import TelemetryDeckCard from './components/TelemetryDeckCard.vue'
import GlassAeroCard from './components/GlassAeroCard.vue'
import AdaptiveExpandCard from './components/AdaptiveExpandCard.vue'
import CompactCardView from './components/CompactCardView.vue'
import { MOCK_DEVICES, type PrototypeDevice } from './mockDevices'

const devices = ref<PrototypeDevice[]>(JSON.parse(JSON.stringify(MOCK_DEVICES)))
// 默认定位到用户选定的“方案 C”
const activeDesign = ref<'deck' | 'glass' | 'adaptive' | 'compact'>('adaptive')
const keyword = ref('')
const filterStatus = ref<'all' | 'online' | 'offline'>('all')

// 后台自动更新状态演示
const isAutoPolling = ref(true)
let pollTimer: ReturnType<typeof setInterval> | null = null

function toggleAutoPolling() {
  isAutoPolling.value = !isAutoPolling.value
  if (isAutoPolling.value) {
    toast.success('后台静默自动采集已开启：由后台独立协程异步定时更新，不阻塞列表刷新')
  } else {
    toast.info('后台静默自动采集已暂停')
  }
}

onMounted(() => {
  pollTimer = setInterval(() => {
    if (isAutoPolling.value) {
      // 模拟后台异步更新内存快照，无感刷新
      devices.value.forEach((d) => {
        if (d.state === 'device') {
          // 微量波动
          const deltaTemp = (Math.random() - 0.48) * 0.4
          d.telemetry.batteryTempCelsius = Math.max(25, Math.min(46, Number((d.telemetry.batteryTempCelsius + deltaTemp).toFixed(1))))
          if (d.telemetry.cpuTempSupported && d.telemetry.cpuTempCelsius) {
            d.telemetry.cpuTempCelsius = Math.max(32, Math.min(65, Number((d.telemetry.cpuTempCelsius + deltaTemp * 1.6).toFixed(1))))
          }
        }
      })
    }
  }, 3500)
})

onUnmounted(() => {
  if (pollTimer) clearInterval(pollTimer)
})

// 过滤列表
const filteredDevices = computed(() => {
  return devices.value.filter((d) => {
    if (filterStatus.value === 'online' && d.state !== 'device') return false
    if (filterStatus.value === 'offline' && d.state === 'device') return false
    if (!keyword.value.trim()) return true
    const q = keyword.value.toLowerCase().trim()
    const name = d.alias || d.telemetry.brand || ''
    return (
      name.toLowerCase().includes(q) ||
      d.telemetry.model.toLowerCase().includes(q) ||
      d.serial.toLowerCase().includes(q) ||
      d.telemetry.currentApp.toLowerCase().includes(q)
    )
  })
})

function handlePlay(serial: string) {
  toast.success(`触发设备投屏：${serial}`)
}

function handlePip(serial: string) {
  toast.info(`开启画中画微窗：${serial}`)
}

function handleUpdateAlias(serial: string, alias: string) {
  const d = devices.value.find(item => item.serial === serial)
  if (d) {
    d.alias = alias
    toast.success(alias ? `别名已更新为: ${alias}` : `已清除别名，默认恢复显示品牌名: ${d.telemetry.brand}`)
  }
}

function handleReconnect(serial: string) {
  const d = devices.value.find(item => item.serial === serial)
  if (d) {
    d.state = 'device'
    toast.success(`设备已恢复握手在线：${serial}`)
  }
}

// 模拟动态温度与负载跳动
function simulateTelemetryPulse() {
  devices.value.forEach((d) => {
    if (d.state === 'device') {
      // 模拟微小波动
      const deltaTemp = (Math.random() - 0.45) * 0.8
      d.telemetry.batteryTempCelsius = Math.max(25, Math.min(48, d.telemetry.batteryTempCelsius + deltaTemp))
      if (d.telemetry.cpuTempSupported && d.telemetry.cpuTempCelsius) {
        d.telemetry.cpuTempCelsius = Math.max(30, Math.min(65, d.telemetry.cpuTempCelsius + deltaTemp * 1.5))
      }
      d.telemetry.cpuUsagePercent = Math.floor(Math.random() * 60 + 10)
    }
  })
  toast.info('已模拟设备温升与负载实时跳动')
}

// 一键重置所有设备的别名，测试“默认显示品牌名”
function resetAllAliases() {
  devices.value.forEach((d) => {
    d.alias = ''
  })
  toast.success('已清空全部自定义别名，当前所有设备卡均默认展示品牌名')
}
function handleEditPrompt(dev: PrototypeDevice) {
  const next = window.prompt('请输入新设备名称/别名（清空则恢复品牌默认名）', dev.alias || dev.telemetry.brand)
  if (next !== null) {
    handleUpdateAlias(dev.serial, next)
  }
}
</script>

<template>
  <div class="min-h-screen bg-slate-50 text-slate-900 dark:bg-slate-950 dark:text-slate-100 p-6 md:p-8">
    <div class="mx-auto max-w-7xl space-y-6">
      <!-- 顶部控制与方案切换栏 -->
      <div class="rounded-2xl border border-slate-200 bg-white p-5 shadow-xs dark:border-white/10 dark:bg-slate-900">
        <div class="flex flex-col md:flex-row items-start md:items-center justify-between gap-4">
          <div>
            <div class="flex items-center gap-2">
              <h1 class="text-xl font-bold tracking-tight">首页设备卡重构设计原型</h1>
              <span class="rounded-full bg-blue-500/10 px-2.5 py-0.5 text-xs font-medium text-blue-600 dark:text-blue-400 border border-blue-500/20">
                品牌优先 · 硬件遥测
              </span>
            </div>
            <p class="text-xs text-slate-500 dark:text-slate-400 mt-1">
              设备名称默认取 ADB 品牌名 (<code class="font-mono">ro.product.brand</code>)；集成 Android 版本/SDK、CPU 架构、电池与 CPU 温度监控
            </p>
          </div>

          <!-- 右侧交互辅助工具 -->
          <div class="flex items-center gap-2 flex-wrap">
            <Button
              :variant="isAutoPolling ? 'default' : 'outline'"
              size="sm"
              class="h-8 rounded-xl text-xs gap-1.5 cursor-pointer"
              :class="isAutoPolling ? 'bg-emerald-600 hover:bg-emerald-700 text-white' : ''"
              @click="toggleAutoPolling"
            >
              <Activity class="h-3.5 w-3.5" :class="isAutoPolling ? 'animate-pulse' : ''" />
              后台自动更新: {{ isAutoPolling ? '已启用 (异步无感刷新)' : '已暂停' }}
            </Button>
            <Button
              variant="outline"
              size="sm"
              class="h-8 rounded-xl text-xs gap-1.5 cursor-pointer"
              @click="simulateTelemetryPulse"
            >
              模拟温升与电量波动
            </Button>
            <Button
              variant="outline"
              size="sm"
              class="h-8 rounded-xl text-xs gap-1.5 cursor-pointer"
              @click="resetAllAliases"
            >
              <RotateCcw class="h-3.5 w-3.5" />
              清空别名(验证品牌名默认)
            </Button>
          </div>
        </div>

        <!-- 方案选择卡片 Tabs -->
        <div class="mt-5 flex flex-wrap items-center justify-between gap-3 border-t border-slate-100 dark:border-white/5 pt-4">
          <div class="flex items-center gap-1.5 bg-slate-100 p-1 rounded-xl dark:bg-white/5">
            <button
              class="rounded-lg px-3 py-1.5 text-xs font-medium transition-colors cursor-pointer"
              :class="activeDesign === 'deck' ? 'bg-white text-blue-600 shadow-xs dark:bg-slate-800 dark:text-white' : 'text-slate-600 hover:text-slate-900 dark:text-slate-400'"
              @click="activeDesign = 'deck'"
            >
              方案 A：极客遥测面板 (Telemetry Deck)
            </button>
            <button
              class="rounded-lg px-3 py-1.5 text-xs font-medium transition-colors cursor-pointer"
              :class="activeDesign === 'glass' ? 'bg-white text-blue-600 shadow-xs dark:bg-slate-800 dark:text-white' : 'text-slate-600 hover:text-slate-900 dark:text-slate-400'"
              @click="activeDesign = 'glass'"
            >
              方案 B：现代全息流线 (Glass Aero)
            </button>
            <button
              class="rounded-lg px-3 py-1.5 text-xs font-medium transition-colors cursor-pointer"
              :class="activeDesign === 'adaptive' ? 'bg-white text-blue-600 shadow-xs dark:bg-slate-800 dark:text-white' : 'text-slate-600 hover:text-slate-900 dark:text-slate-400'"
              @click="activeDesign = 'adaptive'"
            >
              方案 C：自适应折叠 (Adaptive Expand)
            </button>
            <button
              class="rounded-lg px-3 py-1.5 text-xs font-medium transition-colors cursor-pointer"
              :class="activeDesign === 'compact' ? 'bg-white text-blue-600 shadow-xs dark:bg-slate-800 dark:text-white' : 'text-slate-600 hover:text-slate-900 dark:text-slate-400'"
              @click="activeDesign = 'compact'"
            >
              模式对比：紧凑列表 (Compact)
            </button>
          </div>

          <!-- 搜索与在线过滤 -->
          <div class="flex items-center gap-2">
            <Input
              v-model="keyword"
              placeholder="搜索品牌、型号或序列号..."
              class="h-8 w-44 rounded-xl text-xs"
            />
            <select
              v-model="filterStatus"
              class="h-8 rounded-xl border border-slate-200 bg-white px-2.5 text-xs text-slate-700 dark:border-white/10 dark:bg-slate-800 dark:text-slate-200 cursor-pointer"
            >
              <option value="all">全部设备 ({{ devices.length }})</option>
              <option value="online">在线设备</option>
              <option value="offline">离线设备</option>
            </select>
          </div>
        </div>
      </div>

      <!-- 电量与温度阈值变色规则图例 -->
      <div class="grid grid-cols-1 md:grid-cols-2 gap-3 text-xs">
        <!-- 电量变色规则 -->
        <div class="rounded-xl border border-slate-200 bg-white p-3 dark:border-white/10 dark:bg-slate-900 flex items-center justify-between">
          <div class="flex items-center gap-2 font-medium text-slate-700 dark:text-slate-300">
            <Battery class="h-4 w-4 text-emerald-500" />
            <span>电量颜色阶梯:</span>
          </div>
          <div class="flex items-center gap-2 font-mono text-[11px]">
            <span class="rounded px-2 py-0.5 bg-emerald-500/10 text-emerald-600 dark:text-emerald-400 font-medium">充电中 / &gt;20% 充足</span>
            <span class="rounded px-2 py-0.5 bg-amber-500/10 text-amber-600 dark:text-amber-400 font-medium">&le;20% 预警</span>
            <span class="rounded px-2 py-0.5 bg-rose-500/10 text-rose-600 dark:text-rose-400 font-bold animate-pulse">&le;10% 告警</span>
          </div>
        </div>

        <!-- 温度变色规则 -->
        <div class="rounded-xl border border-slate-200 bg-white p-3 dark:border-white/10 dark:bg-slate-900 flex items-center justify-between">
          <div class="flex items-center gap-2 font-medium text-slate-700 dark:text-slate-300">
            <Thermometer class="h-4 w-4 text-blue-500" />
            <span>温度颜色阶梯:</span>
          </div>
          <div class="flex items-center gap-2 font-mono text-[11px]">
            <span class="rounded px-2 py-0.5 bg-emerald-500/10 text-emerald-600 dark:text-emerald-400 font-medium">&lt;38°C 常温</span>
            <span class="rounded px-2 py-0.5 bg-amber-500/10 text-amber-600 dark:text-amber-400 font-medium">38~42°C 温热</span>
            <span class="rounded px-2 py-0.5 bg-rose-500/10 text-rose-600 dark:text-rose-400 font-bold animate-pulse">&ge;42°C 过热</span>
          </div>
        </div>
      </div>
      <div class="flex items-center justify-between rounded-xl bg-blue-500/5 border border-blue-500/15 p-3.5 text-xs text-blue-800 dark:text-blue-300">
        <div class="flex items-center gap-2">
          <Sparkles class="h-4 w-4 text-blue-500 shrink-0" />
          <span>
            <strong v-if="activeDesign === 'deck'">方案 A 特点：</strong>
            <strong v-else-if="activeDesign === 'glass'">方案 B 特点：</strong>
            <strong v-else-if="activeDesign === 'adaptive'">方案 C 特点：</strong>
            <strong v-else>紧凑列表模式特点：</strong>
            <span v-if="activeDesign === 'deck'">双温感（电池温度+CPU温度）工控式微型仪表板，带 CPU/内存负载条与完整 ABI/SDK 胶囊，信息一览无余。</span>
            <span v-else-if="activeDesign === 'glass'">3D 微悬浮与光影动态跟随，大字号品牌名优先，3 联精炼指标胶囊，层级清晰美观。</span>
            <span v-else-if="activeDesign === 'adaptive'">常规视图保持极简利落，点击“详情”展开完整 ADB 系统硬件清单（构建ID、ABI、CPU温度、厂商等）。</span>
            <span v-else>高密度设备阵列排布，横向展示品牌名、Android 版本、电池电量与温升指标，适合批量多设备运维。</span>
          </span>
        </div>
        <div class="font-mono text-[11px] text-blue-600/80 dark:text-blue-400/80 hidden sm:block">
          默认展示名称: device.alias || device.brand
        </div>
      </div>

      <!-- 设备卡片展示区 -->
      <!-- 方案 A：极客遥测面板 -->
      <div v-if="activeDesign === 'deck'" class="grid grid-cols-1 md:grid-cols-2 lg:grid-cols-3 gap-5">
        <TelemetryDeckCard
          v-for="d in filteredDevices"
          :key="d.serial"
          :device="d"
          @play="handlePlay"
          @pip="handlePip"
          @update-alias="handleUpdateAlias"
          @reconnect="handleReconnect"
        />
      </div>

      <!-- 方案 B：现代全息流线 -->
      <div v-else-if="activeDesign === 'glass'" class="grid grid-cols-1 md:grid-cols-2 lg:grid-cols-3 gap-5">
        <GlassAeroCard
          v-for="d in filteredDevices"
          :key="d.serial"
          :device="d"
          @play="handlePlay"
          @pip="handlePip"
          @update-alias="handleUpdateAlias"
        />
      </div>

      <!-- 方案 C：自适应折叠 -->
      <div v-else-if="activeDesign === 'adaptive'" class="grid grid-cols-1 md:grid-cols-2 lg:grid-cols-3 gap-5">
        <AdaptiveExpandCard
          v-for="d in filteredDevices"
          :key="d.serial"
          :device="d"
          @play="handlePlay"
          @pip="handlePip"
          @update-alias="handleUpdateAlias"
        />
      </div>

      <!-- 模式对比：紧凑列表 -->
      <div v-else class="space-y-2.5">
        <CompactCardView
          v-for="d in filteredDevices"
          :key="d.serial"
          :device="d"
          @play="handlePlay"
          @pip="handlePip"
          @edit="handleEditPrompt"
        />
      </div>

      <!-- 底部：ADB 采集指标与字段对照参考表 -->
      <div class="rounded-2xl border border-slate-200 bg-white p-5 shadow-xs dark:border-white/10 dark:bg-slate-900">
        <div class="flex items-center gap-2 mb-3">
          <Layers class="h-4 w-4 text-blue-500" />
          <h2 class="text-sm font-bold">ADB 采集指标与前端卡片呈现映射说明</h2>
        </div>
        <div class="overflow-x-auto">
          <table class="w-full text-left text-xs border-collapse">
            <thead>
              <tr class="border-b border-slate-200 dark:border-white/10 text-slate-500 dark:text-slate-400">
                <th class="py-2 px-3 font-medium">设备信息指标</th>
                <th class="py-2 px-3 font-medium">底层 ADB 采集来源</th>
                <th class="py-2 px-3 font-medium">卡片位置与展示规则</th>
                <th class="py-2 px-3 font-medium">兼容与降级处理</th>
              </tr>
            </thead>
            <tbody class="divide-y divide-slate-100 dark:divide-white/5 font-mono text-[11px]">
              <tr>
                <td class="py-2 px-3 font-sans font-medium text-slate-800 dark:text-slate-200">设备名称 (默认)</td>
                <td class="py-2 px-3 text-slate-600 dark:text-slate-400">ro.product.brand</td>
                <td class="py-2 px-3 text-blue-600 dark:text-blue-400">卡片主标题（默认展示品牌名，如 Xiaomi）</td>
                <td class="py-2 px-3 text-slate-500">若有用户别名优先使用别名；无品牌名兜底使用型号</td>
              </tr>
              <tr>
                <td class="py-2 px-3 font-sans font-medium text-slate-800 dark:text-slate-200">型号与序列号</td>
                <td class="py-2 px-3 text-slate-600 dark:text-slate-400">ro.product.model / get-serialno</td>
                <td class="py-2 px-3">标题下方副标题，小字浅灰排布</td>
                <td class="py-2 px-3 text-slate-500">所有设备通用支持</td>
              </tr>
              <tr>
                <td class="py-2 px-3 font-sans font-medium text-slate-800 dark:text-slate-200">Android 版本 & SDK</td>
                <td class="py-2 px-3 text-slate-600 dark:text-slate-400">ro.build.version.release / sdk</td>
                <td class="py-2 px-3">顶部系统胶囊标签 (例: Android 14 API 34)</td>
                <td class="py-2 px-3 text-slate-500">原生标准属性</td>
              </tr>
              <tr>
                <td class="py-2 px-3 font-sans font-medium text-slate-800 dark:text-slate-200">CPU 架构</td>
                <td class="py-2 px-3 text-slate-600 dark:text-slate-400">ro.product.cpu.abi</td>
                <td class="py-2 px-3">硬件胶囊标签 (例: arm64-v8a / x86_64)</td>
                <td class="py-2 px-3 text-slate-500">原生标准属性</td>
              </tr>
              <tr>
                <td class="py-2 px-3 font-sans font-medium text-slate-800 dark:text-slate-200">电池电量 & 电池温度</td>
                <td class="py-2 px-3 text-slate-600 dark:text-slate-400">dumpsys battery (level & temperature)</td>
                <td class="py-2 px-3">遥测仪表左侧：电量进度条 + 36.5°C 状态颜色</td>
                <td class="py-2 px-3 text-slate-500">免 Root，除少数定制模拟器外绝大部分真机均支持</td>
              </tr>
              <tr>
                <td class="py-2 px-3 font-sans font-medium text-slate-800 dark:text-slate-200">CPU 温度</td>
                <td class="py-2 px-3 text-slate-600 dark:text-slate-400">/sys/class/thermal 或 dumpsys thermalservice</td>
                <td class="py-2 px-3">遥测仪表右侧：实时 CPU 温度与常温/警戒标识</td>
                <td class="py-2 px-3 text-amber-600 dark:text-amber-400 font-sans">若厂商系统限制无权限，自动平滑显示“未开放”，不影响整卡展示</td>
              </tr>
            </tbody>
          </table>
        </div>
      </div>
    </div>
  </div>
</template>
