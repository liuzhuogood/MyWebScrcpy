<script setup lang="ts">
import { computed, onMounted, onUnmounted, ref } from 'vue'
import {
  Activity,
  Battery,
  BatteryCharging,
  Cpu,
  Film,
  Layers,
  Music,
  Pencil,
  RefreshCw,
  Settings,
  Smartphone,
  Sparkles,
  Thermometer,
  Tv,
  Usb,
  Wifi,
} from 'lucide-vue-next'
import { toast } from 'vue-sonner'
import { MOCK_PHONE_DEVICES, type MockPhoneDevice } from './mockDevices'

// 方案选择：方案 A (现代圆润微拟物手机) | 方案 B (表盘/圆环式手机卡片)
const activeDesign = ref<'rounded-phone' | 'circular-dial'>('rounded-phone')

// 图标形态开关：圆形图标 vs 圆角方块图标
const iconShape = ref<'circle' | 'squircle'>('circle')

// 尺寸模式：对齐直接显示屏幕规格（medium 270×508, small 200×380, large 348×648）
const cardSize = ref<'medium' | 'small' | 'large'>('medium')

// 模拟设备数据
const devices = ref<MockPhoneDevice[]>(JSON.parse(JSON.stringify(MOCK_PHONE_DEVICES)))
const keyword = ref('')
const filterStatus = ref<'all' | 'online' | 'offline'>('all')

// 别名编辑状态
const editingSerial = ref<string | null>(null)
const editingDraft = ref('')

// 自动后台遥测轮询模拟
const isAutoPolling = ref(true)
let pollTimer: ReturnType<typeof setInterval> | null = null

onMounted(() => {
  pollTimer = setInterval(() => {
    if (isAutoPolling.value) {
      devices.value.forEach((d) => {
        if (d.state === 'device') {
          // 微量波动电池温度
          const deltaB = (Math.random() - 0.48) * 0.4
          d.telemetry.batteryTempCelsius = Math.max(25, Math.min(48, Number((d.telemetry.batteryTempCelsius + deltaB).toFixed(1))))
          // 微量波动 CPU 温度
          if (d.telemetry.cpuTempSupported && d.telemetry.cpuTempCelsius) {
            const deltaC = (Math.random() - 0.48) * 0.8
            d.telemetry.cpuTempCelsius = Math.max(32, Math.min(68, Number((d.telemetry.cpuTempCelsius + deltaC).toFixed(1))))
          }
        }
      })
    }
  }, 3000)
})

onUnmounted(() => {
  if (pollTimer) clearInterval(pollTimer)
})

// 过滤后的设备
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

// 获取电池状态样式
function getBatteryStyle(d: MockPhoneDevice) {
  if (d.state !== 'device') {
    return { text: 'text-slate-400', icon: 'text-slate-400' }
  }
  if (d.telemetry.batteryCharging) {
    return { text: 'text-emerald-500 font-semibold', icon: 'text-emerald-500' }
  }
  const lvl = d.telemetry.batteryLevel
  if (lvl <= 10) {
    return { text: 'text-rose-500 font-bold animate-pulse', icon: 'text-rose-500' }
  }
  if (lvl <= 20) {
    return { text: 'text-amber-500 font-semibold', icon: 'text-amber-500' }
  }
  return { text: 'text-slate-700 dark:text-slate-200 font-medium', icon: 'text-slate-500 dark:text-slate-400' }
}

// 获取电池温度样式
function getBatteryTempStyle(d: MockPhoneDevice) {
  const t = d.telemetry.batteryTempCelsius
  if (d.state !== 'device' || !t || t <= 0) {
    return { text: 'text-slate-400', icon: 'text-slate-400' }
  }
  if (t < 38) {
    return { text: 'text-emerald-500 dark:text-emerald-400', icon: 'text-emerald-500' }
  }
  if (t < 42) {
    return { text: 'text-amber-500 dark:text-amber-400 font-semibold', icon: 'text-amber-500' }
  }
  return { text: 'text-rose-500 dark:text-rose-400 font-bold animate-pulse', icon: 'text-rose-500' }
}

// 获取 CPU 温度样式
function getCpuTempStyle(d: MockPhoneDevice) {
  const t = d.telemetry.cpuTempCelsius
  if (d.state !== 'device' || !d.telemetry.cpuTempSupported || !t) {
    return { text: 'text-slate-400', icon: 'text-slate-400' }
  }
  if (t < 48) return { text: 'text-emerald-500 dark:text-emerald-400', icon: 'text-emerald-500' }
  if (t < 58) return { text: 'text-amber-500 dark:text-amber-400 font-semibold', icon: 'text-amber-500' }
  return { text: 'text-rose-500 dark:text-rose-400 font-bold animate-pulse', icon: 'text-rose-500' }
}

// 获取应用图标与色调配置
function getAppVisual(iconType: string) {
  switch (iconType) {
    case 'wechat':
      return {
        bg: 'bg-[#07c160]/12 border-[#07c160]/25 text-[#07c160]',
      }
    case 'kuaishou':
      return {
        bg: 'bg-orange-500/12 border-orange-500/25 text-orange-400',
      }
    case 'bilibili':
      return {
        bg: 'bg-pink-500/12 border-pink-500/25 text-pink-400',
      }
    case 'neteasemusic':
      return {
        bg: 'bg-rose-500/12 border-rose-500/25 text-rose-400',
      }
    case 'settings':
      return {
        bg: 'bg-neutral-800/80 border-neutral-700 text-neutral-300',
      }
    default:
      return {
        bg: 'bg-sky-500/12 border-sky-500/25 text-sky-400',
      }
  }
}

function handleStartScrcpy(serial: string) {
  toast.success(`正在为设备 ${serial} 启动 Scrcpy 高清投屏`)
}

function handleToggleAlias(d: MockPhoneDevice) {
  if (editingSerial.value === d.serial) {
    d.alias = editingDraft.value.trim()
    editingSerial.value = null
    toast.success('别名已保存')
  } else {
    editingSerial.value = d.serial
    editingDraft.value = d.alias || d.telemetry.brand
  }
}

function randomizeAllApps() {
  devices.value.forEach((d) => {
    simulateAppSwitch(d)
  })
}

// 模拟切换前台应用
function simulateAppSwitch(d: MockPhoneDevice) {
  if (d.state !== 'device') return
  const appPool = [
    { name: '微信', pkg: 'com.tencent.mm', icon: 'wechat' },
    { name: '快手极速版', pkg: 'com.kuaishou.nebula', icon: 'kuaishou' },
    { name: '哔哩哔哩', pkg: 'tv.danmaku.bili', icon: 'bilibili' },
    { name: '网易云音乐', pkg: 'com.netease.cloudmusic', icon: 'neteasemusic' },
    { name: '系统设置', pkg: 'com.android.settings', icon: 'settings' },
  ]
  const next = appPool[Math.floor(Math.random() * appPool.length)]
  d.telemetry.currentApp = next.name
  d.telemetry.currentPackage = next.pkg
  d.telemetry.iconType = next.icon
  toast.info(`[${d.telemetry.brand}] 切换前台应用为: ${next.name}`)
}
</script>

<template>
  <div class="space-y-6">
    <!-- 原型控制中枢与设计对比栏 -->
    <div class="flex flex-col gap-4 rounded-3xl border border-border bg-card/60 p-5 backdrop-blur-xl shadow-lg">
      <div class="flex flex-wrap items-center justify-between gap-3">
        <div class="flex items-center gap-2.5">
          <div class="flex h-10 w-10 items-center justify-center rounded-2xl bg-primary/10 text-primary">
            <Smartphone class="h-5 w-5" />
          </div>
          <div>
            <h2 class="text-base font-bold text-foreground tracking-tight">首页手机拟物卡片重构原型</h2>
            <p class="text-xs text-muted-foreground">1:1 对齐直接显示屏幕规格（270×508 / 200×380） · 纯粹真机内屏 · 点击屏幕直接投屏</p>
          </div>
        </div>

        <!-- 方案切换 Tab -->
        <div class="flex items-center rounded-2xl border border-border bg-muted/40 p-1">
          <button
            type="button"
            class="flex items-center gap-1.5 rounded-xl px-3.5 py-1.5 text-xs font-semibold transition-all cursor-pointer"
            :class="activeDesign === 'rounded-phone' ? 'bg-primary text-primary-foreground shadow-sm' : 'text-muted-foreground hover:text-foreground'"
            @click="activeDesign = 'rounded-phone'"
          >
            <Smartphone class="h-3.5 w-3.5" />
            <span>方案 A: 圆润拟物手机卡片 (推荐)</span>
          </button>
          <button
            type="button"
            class="flex items-center gap-1.5 rounded-xl px-3.5 py-1.5 text-xs font-semibold transition-all cursor-pointer"
            :class="activeDesign === 'circular-dial' ? 'bg-primary text-primary-foreground shadow-sm' : 'text-muted-foreground hover:text-foreground'"
            @click="activeDesign = 'circular-dial'"
          >
            <Activity class="h-3.5 w-3.5" />
            <span>方案 B: 探索圆形表盘卡片</span>
          </button>
        </div>
      </div>

      <!-- 次级调控栏：图标形状、卡片密度、过滤与数据跳动 -->
      <div class="flex flex-wrap items-center justify-between gap-3 border-t border-border/60 pt-3 text-xs">
        <div class="flex flex-wrap items-center gap-2">
          <!-- 图标形态开关（对应用户“圆形”的两种理解探索） -->
          <span class="text-muted-foreground">应用图标形态:</span>
          <div class="flex items-center rounded-xl border border-border bg-muted/30 p-0.5">
            <button
              type="button"
              class="rounded-lg px-2 py-1 text-[11px] font-medium transition-colors cursor-pointer"
              :class="iconShape === 'circle' ? 'bg-background text-foreground shadow-xs' : 'text-muted-foreground'"
              @click="iconShape = 'circle'"
            >
              圆形 (Circle)
            </button>
            <button
              type="button"
              class="rounded-lg px-2 py-1 text-[11px] font-medium transition-colors cursor-pointer"
              :class="iconShape === 'squircle' ? 'bg-background text-foreground shadow-xs' : 'text-muted-foreground'"
              @click="iconShape = 'squircle'"
            >
              圆角方块 (Squircle)
            </button>
          </div>

          <!-- 尺寸档位（精确匹配直接显示屏幕规格 phoneGrid） -->
          <span class="ml-2 text-muted-foreground">屏幕尺寸:</span>
          <div class="flex items-center rounded-xl border border-border bg-muted/30 p-0.5">
            <button
              type="button"
              class="rounded-lg px-2 py-1 text-[11px] font-medium transition-colors cursor-pointer"
              :class="cardSize === 'medium' ? 'bg-background text-foreground shadow-xs' : 'text-muted-foreground'"
              @click="cardSize = 'medium'"
            >
              标准 270×508
            </button>
            <button
              type="button"
              class="rounded-lg px-2 py-1 text-[11px] font-medium transition-colors cursor-pointer"
              :class="cardSize === 'small' ? 'bg-background text-foreground shadow-xs' : 'text-muted-foreground'"
              @click="cardSize = 'small'"
            >
              紧凑 200×380
            </button>
            <button
              type="button"
              class="rounded-lg px-2 py-1 text-[11px] font-medium transition-colors cursor-pointer"
              :class="cardSize === 'large' ? 'bg-background text-foreground shadow-xs' : 'text-muted-foreground'"
              @click="cardSize = 'large'"
            >
              大号 348×648
            </button>
          </div>
        </div>

        <div class="flex items-center gap-2">
          <!-- 状态过滤 -->
          <div class="flex items-center rounded-xl border border-border bg-muted/30 p-0.5">
            <button
              type="button"
              class="rounded-lg px-2 py-1 text-[11px] font-medium cursor-pointer"
              :class="filterStatus === 'all' ? 'bg-background text-foreground' : 'text-muted-foreground'"
              @click="filterStatus = 'all'"
            >
              全部 ({{ devices.length }})
            </button>
            <button
              type="button"
              class="rounded-lg px-2 py-1 text-[11px] font-medium cursor-pointer"
              :class="filterStatus === 'online' ? 'bg-background text-foreground' : 'text-muted-foreground'"
              @click="filterStatus = 'online'"
            >
              在线
            </button>
            <button
              type="button"
              class="rounded-lg px-2 py-1 text-[11px] font-medium cursor-pointer"
              :class="filterStatus === 'offline' ? 'bg-background text-foreground' : 'text-muted-foreground'"
              @click="filterStatus = 'offline'"
            >
              离线
            </button>
          </div>

          <!-- 模拟跳动与换批应用 -->
          <button
            type="button"
            class="flex items-center gap-1 rounded-xl border border-border bg-muted/30 px-2 py-1 text-[11px] text-muted-foreground hover:text-foreground cursor-pointer"
            title="模拟切换随机前台应用"
            @click="randomizeAllApps"
          >
            <RefreshCw class="h-3 w-3" />
            <span>换批应用</span>
          </button>
          <button
            type="button"
            class="flex items-center gap-1 rounded-xl border border-border bg-muted/30 px-2.5 py-1 text-[11px] text-muted-foreground hover:text-foreground cursor-pointer"
            :class="{ 'text-primary border-primary/40': isAutoPolling }"
            @click="isAutoPolling = !isAutoPolling"
          >
            <Sparkles class="h-3 w-3" />
            <span>{{ isAutoPolling ? '动态跳动中' : '暂停动态' }}</span>
          </button>
        </div>
      </div>
    </div>

    <!-- 方案 A：现代拟物圆润手机卡片 (推荐) -->
    <div v-if="activeDesign === 'rounded-phone'" class="space-y-4">
      <div class="flex items-center justify-between text-xs text-muted-foreground px-1">
        <span>展示效果：1:1 对齐直接显示屏幕尺寸（270×508 / 200×380） · 纯粹真机内屏 · 点击屏幕直接投屏</span>
        <span>卡片总数：{{ filteredDevices.length }}</span>
      </div>

      <!-- 设备网格 -->
      <div
        class="flex flex-wrap items-start justify-center gap-6"
      >
        <div
          v-for="d in filteredDevices"
          :key="d.serial"
          class="group relative flex flex-col shrink-0 transition-all duration-200 select-none"
        >
          <!-- 手机拟物外壳：1:1 对齐直接显示屏幕规格 (medium 270×508, small 200×380, large 348×648) -->
          <div
            class="relative flex flex-col overflow-hidden bg-[#0d0e11] text-white shadow-xl shadow-black/60 transition-all duration-200 group-hover:-translate-y-1"
            :class="[
              cardSize === 'medium'
                ? 'w-[270px] h-[508px] rounded-[36px] p-2 border-[3px]'
                : cardSize === 'small'
                  ? 'w-[200px] h-[380px] rounded-[26px] p-1.5 border-[2px]'
                  : 'w-[348px] h-[648px] rounded-[44px] p-2.5 border-[4px]',
              d.state === 'device'
                ? 'border-neutral-800 hover:border-neutral-700 ring-1 ring-white/5'
                : 'border-neutral-900 opacity-55 grayscale',
            ]"
          >
            <!-- 手机左侧物理按键凹凸微雕 (音量键 + / -) -->
            <div
              class="absolute -left-[3px] rounded-r-sm bg-neutral-700"
              :class="cardSize === 'small' ? 'top-14 h-6 w-[2.5px]' : 'top-18 h-8 w-[3px]'"
            />
            <div
              class="absolute -left-[3px] rounded-r-sm bg-neutral-700"
              :class="cardSize === 'small' ? 'top-22 h-6 w-[2.5px]' : 'top-28 h-8 w-[3px]'"
            />
            <!-- 手机右侧物理按键凹凸微雕 (电源键) -->
            <div
              class="absolute -right-[3px] rounded-l-sm bg-neutral-700"
              :class="cardSize === 'small' ? 'top-16 h-8 w-[2.5px]' : 'top-22 h-10 w-[3px]'"
            />

            <!-- 手机内屏区域 (OLED 纯黑沉浸屏幕，整屏点击直接启动投屏) -->
            <div
              class="group/screen relative flex flex-col justify-between overflow-hidden bg-black flex-1 cursor-pointer transition-colors"
              :class="[
                cardSize === 'medium'
                  ? 'rounded-[28px] p-3'
                  : cardSize === 'small'
                    ? 'rounded-[20px] p-2'
                    : 'rounded-[36px] p-4',
              ]"
              title="点击屏幕直接启动投屏"
              @click="handleStartScrcpy(d.serial)"
            >
              <!-- 手机屏幕顶部：灵动岛/听筒打孔 + 顶栏遥测状态 (重点需求 2) -->
              <div class="relative z-10 flex flex-col gap-2 shrink-0">
                <!-- 顶部中央灵动岛 / 听筒打孔 -->
                <div class="mx-auto flex items-center justify-center">
                  <div
                    class="h-4 rounded-full bg-black/95 px-3 flex items-center gap-1.5 border border-white/15 shadow-inner"
                    :title="d.telemetry.model"
                  >
                    <!-- 前置摄像头打孔微点 -->
                    <span class="h-2 w-2 rounded-full bg-slate-900 border border-slate-700/80 flex items-center justify-center">
                      <span class="h-0.5 w-0.5 rounded-full bg-blue-400/60" />
                    </span>
                    <span class="text-[9px] font-mono text-slate-400 font-semibold tracking-wider scale-90">{{ d.telemetry.brand }}</span>
                  </div>
                </div>

                <!-- 顶栏状态信息：左侧连接模式，右侧遥测（电池、温度） -->
                <div class="flex items-center justify-between text-[11px] pt-0.5 px-0.5">
                  <!-- 左侧：连接模式 (USB / Wi-Fi) -->
                  <div class="flex items-center gap-1 font-mono">
                    <span
                      class="flex items-center gap-1 px-1.5 py-0.5 rounded-md text-[10px] font-medium"
                      :class="d.source === 'auto' ? 'bg-cyan-500/15 text-cyan-400 border border-cyan-500/30' : 'bg-purple-500/15 text-purple-400 border border-purple-500/30'"
                    >
                      <Usb v-if="d.source === 'auto'" class="h-2.5 w-2.5" />
                      <Wifi v-else class="h-2.5 w-2.5" />
                      <span>{{ d.source === 'auto' ? 'USB' : 'Wi-Fi' }}</span>
                    </span>
                  </div>

                  <!-- 右侧：电池与温度遥测核心区 (重点需求 2) -->
                  <div class="flex items-center gap-2 font-mono">
                    <!-- 电池温度指示 -->
                    <div
                      class="flex items-center gap-0.5 text-[10px]"
                      :class="getBatteryTempStyle(d).text"
                      :title="`电池温度: ${d.telemetry.batteryTempCelsius}°C`"
                    >
                      <Thermometer class="h-3 w-3" :class="getBatteryTempStyle(d).icon" />
                      <span>{{ d.state === 'device' && d.telemetry.batteryTempCelsius > 0 ? `${d.telemetry.batteryTempCelsius}°C` : '--' }}</span>
                    </div>

                    <!-- CPU 温度（若支持） -->
                    <div
                      v-if="d.telemetry.cpuTempSupported && d.telemetry.cpuTempCelsius"
                      class="flex items-center gap-0.5 text-[10px]"
                      :class="getCpuTempStyle(d).text"
                      :title="`CPU 核心温度: ${d.telemetry.cpuTempCelsius}°C`"
                    >
                      <Cpu class="h-2.5 w-2.5" :class="getCpuTempStyle(d).icon" />
                      <span>{{ d.telemetry.cpuTempCelsius.toFixed(0) }}°</span>
                    </div>

                    <!-- 电量百分比与充电标识 -->
                    <div
                      class="flex items-center gap-1 text-[10px]"
                      :class="getBatteryStyle(d).text"
                      :title="`当前电量: ${d.telemetry.batteryLevel}% ${d.telemetry.batteryCharging ? '(充电中)' : ''}`"
                    >
                      <span>{{ d.state === 'device' ? `${d.telemetry.batteryLevel}%` : '--' }}</span>
                      <BatteryCharging v-if="d.telemetry.batteryCharging" class="h-3.5 w-3.5" :class="getBatteryStyle(d).icon" />
                      <Battery v-else class="h-3.5 w-3.5" :class="getBatteryStyle(d).icon" />
                    </div>
                  </div>
                </div>
              </div>

              <!-- 手机中央屏幕区域：正在运行应用展示 (去弥散光斑，纯粹产品质感) -->
              <div class="relative my-auto flex flex-col items-center justify-center py-4 text-center">
                <!-- 应用图标主体：圆形 / 圆角矩形动态切换 -->
                <div
                  class="relative flex items-center justify-center border shadow-lg transition-transform duration-200 group-hover/screen:scale-102"
                  :class="[
                    iconShape === 'circle' ? 'rounded-full' : 'rounded-2xl',
                    cardSize === 'small' ? 'h-11 w-11' : cardSize === 'large' ? 'h-20 w-20' : 'h-15 w-15',
                    getAppVisual(d.telemetry.iconType).bg,
                  ]"
                >
                  <!-- 微信图标 -->
                  <svg
                    v-if="d.telemetry.iconType === 'wechat'"
                    class="text-[#07C160]"
                    :class="cardSize === 'small' ? 'h-6 w-6' : cardSize === 'large' ? 'h-10 w-10' : 'h-7.5 w-7.5'"
                    viewBox="0 0 24 24"
                    fill="currentColor"
                  >
                    <path d="M8.5 2C4.36 2 1 4.91 1 8.5c0 2.05 1.07 3.88 2.76 5.11L3 17.5l4.28-1.43c.4.08.8.13 1.22.13.23 0 .45-.02.67-.04-.26-.67-.42-1.39-.42-2.16 0-3.59 3.36-6.5 7.5-6.5.64 0 1.26.07 1.84.2C16.84 4.54 13.06 2 8.5 2zm-2.25 4a1.25 1.25 0 110 2.5 1.25 1.25 0 010-2.5zm4.5 0a1.25 1.25 0 110 2.5 1.25 1.25 0 010-2.5zM15.5 8.5c-3.59 0-6.5 2.46-6.5 5.5s2.91 5.5 6.5 5.5c.36 0 .71-.03 1.05-.09L20 20.5l-.9-3.08C20.9 16.38 22 14.8 22 13c0-3.04-2.91-5.5-6.5-5.5zm-2 3.5a1 1 0 110 2 1 1 0 010-2zm4 0a1 1 0 110 2 1 1 0 010-2z" />
                  </svg>
                  <Film
                    v-else-if="d.telemetry.iconType === 'kuaishou'"
                    class="text-orange-500"
                    :class="cardSize === 'small' ? 'h-6 w-6' : cardSize === 'large' ? 'h-10 w-10' : 'h-7.5 w-7.5'"
                  />
                  <Tv
                    v-else-if="d.telemetry.iconType === 'bilibili'"
                    class="text-pink-500"
                    :class="cardSize === 'small' ? 'h-6 w-6' : cardSize === 'large' ? 'h-10 w-10' : 'h-7.5 w-7.5'"
                  />
                  <Music
                    v-else-if="d.telemetry.iconType === 'neteasemusic'"
                    class="text-rose-500"
                    :class="cardSize === 'small' ? 'h-6 w-6' : cardSize === 'large' ? 'h-10 w-10' : 'h-7.5 w-7.5'"
                  />
                  <Settings
                    v-else-if="d.telemetry.iconType === 'settings'"
                    class="text-slate-400"
                    :class="cardSize === 'small' ? 'h-6 w-6' : cardSize === 'large' ? 'h-10 w-10' : 'h-7.5 w-7.5'"
                  />
                  <Layers
                    v-else
                    class="text-blue-500"
                    :class="cardSize === 'small' ? 'h-6 w-6' : cardSize === 'large' ? 'h-10 w-10' : 'h-7.5 w-7.5'"
                  />
                </div>

                <!-- 运行应用文字信息 -->
                <div class="mt-3 w-full px-2">
                  <h4 class="truncate text-sm font-medium tracking-tight text-neutral-100 group-hover/screen:text-white transition-colors">
                    {{ d.state === 'device' ? d.telemetry.currentApp : '设备离线' }}
                  </h4>
                  <p class="truncate font-mono text-[10px] text-neutral-500 mt-0.5">
                    {{ d.state === 'device' && d.telemetry.currentPackage ? d.telemetry.currentPackage : '休眠中' }}
                  </p>
                </div>
              </div>

              <!-- 手机底部区域：纯粹设备信息与 Android 手势条 (彻底去按钮，保留呼吸间距) -->
              <div class="relative z-10 shrink-0 border-t border-neutral-800/80 pt-2 pb-0.5 flex flex-col gap-1.5" @click.stop>
                <div class="flex items-center justify-between gap-1.5 px-0.5">
                  <div class="min-w-0 flex-1 truncate">
                    <div class="flex items-center gap-1.5">
                      <span
                        class="font-medium text-neutral-200 truncate"
                        :class="cardSize === 'small' ? 'text-[11px]' : 'text-xs'"
                      >
                        {{ d.alias || d.telemetry.brand }}
                      </span>
                      <button
                        type="button"
                        class="text-neutral-500 hover:text-neutral-200 transition-colors cursor-pointer"
                        title="编辑别名"
                        @click.stop="handleToggleAlias(d)"
                      >
                        <Pencil class="h-2.5 w-2.5" />
                      </button>
                    </div>
                    <p
                      class="truncate font-mono text-neutral-500 mt-0.5"
                      :class="cardSize === 'small' ? 'text-[8px]' : 'text-[9px]'"
                    >
                      {{ d.telemetry.model }} · {{ d.serial }}
                    </p>
                  </div>
                </div>

                <!-- 手机底部 Android 手势条 (Home Indicator) -->
                <div
                  class="mx-auto rounded-full bg-neutral-800 transition-colors group-hover/screen:bg-neutral-600"
                  :class="cardSize === 'small' ? 'h-0.5 w-10 mt-0.5 mb-0.5' : 'h-1 w-14 mt-1 mb-0.5'"
                />
              </div>
            </div>
          </div>
        </div>
      </div>
    </div>

    <!-- 方案 B：探索圆形表盘卡片 (验证字面“圆形”版本) -->
    <div v-else class="space-y-4">
      <div class="flex items-center justify-between text-xs text-muted-foreground px-1">
        <span>探索方案：字面“圆形”卡片（正圆表盘形态），对比屏幕空间利用率与边缘留白</span>
        <span>卡片总数：{{ filteredDevices.length }}</span>
      </div>

      <div class="grid grid-cols-1 sm:grid-cols-2 md:grid-cols-3 lg:grid-cols-4 gap-6">
        <div
          v-for="d in filteredDevices"
          :key="d.serial"
          class="group relative flex flex-col items-center justify-center transition-all duration-200 select-none"
        >
          <!-- 正圆形表盘卡片 -->
          <div
            class="relative flex flex-col items-center justify-between rounded-full border-4 p-5 shadow-2xl transition-all duration-300 group-hover:scale-102 cursor-pointer"
            :class="[
              d.state === 'device'
                ? 'border-violet-500/40 bg-gradient-to-b from-slate-900 via-slate-950 to-black text-white hover:border-violet-400'
                : 'border-slate-800 bg-slate-950/60 opacity-60 grayscale',
            ]"
            style="width: 270px; height: 270px;"
            title="点击表盘直接启动投屏"
            @click="handleStartScrcpy(d.serial)"
          >
            <!-- 圆形顶环：右上角遥测（电量与温度） -->
            <div class="flex items-center justify-between w-full px-4 text-[10px] font-mono text-slate-300">
              <span class="text-cyan-400">{{ d.telemetry.brand }}</span>
              <div class="flex items-center gap-2">
                <span :class="getBatteryTempStyle(d).text">{{ d.telemetry.batteryTempCelsius }}°C</span>
                <span :class="getBatteryStyle(d).text">{{ d.telemetry.batteryLevel }}%</span>
              </div>
            </div>

            <!-- 中央运行应用 -->
            <div
              class="flex flex-col items-center text-center my-auto"
            >
              <div
                class="flex h-14 w-14 items-center justify-center rounded-full border shadow-lg"
                :class="getAppVisual(d.telemetry.iconType).bg"
              >
                <svg v-if="d.telemetry.iconType === 'wechat'" class="h-7 w-7 text-[#07C160]" viewBox="0 0 24 24" fill="currentColor">
                  <path d="M8.5 2C4.36 2 1 4.91 1 8.5c0 2.05 1.07 3.88 2.76 5.11L3 17.5l4.28-1.43c.4.08.8.13 1.22.13.23 0 .45-.02.67-.04-.26-.67-.42-1.39-.42-2.16 0-3.59 3.36-6.5 7.5-6.5.64 0 1.26.07 1.84.2C16.84 4.54 13.06 2 8.5 2zm-2.25 4a1.25 1.25 0 110 2.5 1.25 1.25 0 010-2.5zm4.5 0a1.25 1.25 0 110 2.5 1.25 1.25 0 010-2.5zM15.5 8.5c-3.59 0-6.5 2.46-6.5 5.5s2.91 5.5 6.5 5.5c.36 0 .71-.03 1.05-.09L20 20.5l-.9-3.08C20.9 16.38 22 14.8 22 13c0-3.04-2.91-5.5-6.5-5.5zm-2 3.5a1 1 0 110 2 1 1 0 010-2zm4 0a1 1 0 110 2 1 1 0 010-2z" />
                </svg>
                <Film v-else-if="d.telemetry.iconType === 'kuaishou'" class="h-7 w-7 text-orange-500" />
                <Tv v-else-if="d.telemetry.iconType === 'bilibili'" class="h-7 w-7 text-pink-500" />
                <Music v-else-if="d.telemetry.iconType === 'neteasemusic'" class="h-7 w-7 text-rose-500" />
                <Settings v-else-if="d.telemetry.iconType === 'settings'" class="h-7 w-7 text-slate-400" />
                <Layers v-else class="h-7 w-7 text-blue-500" />
              </div>
              <h4 class="mt-1.5 text-xs font-bold truncate max-w-[140px] text-white">{{ d.telemetry.currentApp }}</h4>
              <p class="font-mono text-[9px] text-slate-400 truncate max-w-[120px]">{{ d.alias || d.telemetry.model }}</p>
            </div>
          </div>
        </div>
      </div>

      <!-- 独立判断与架构风险说明卡片 -->
      <div class="rounded-2xl border border-amber-500/30 bg-amber-500/5 p-4 text-xs text-amber-200/90 space-y-1.5">
        <div class="font-bold text-amber-300 flex items-center gap-1.5">
          <span>⚠️ 架构与交互独立评估提示</span>
        </div>
        <p>
          1. <strong>直接显示屏幕对齐</strong>：拟物卡片规格已 1:1 对齐系统中真实投屏规格（标准 270×508 / 紧凑 200×380），并采用标准 9:16.9 纵向比例；
        </p>
        <p>
          2. <strong>极简沉浸交互</strong>：彻底移除一切详情与操作按钮，点击整屏任何位置即直接唤醒 Scrcpy 投屏，达到与真实手机一致的直接视觉体验。
        </p>
      </div>
    </div>
  </div>
</template>
