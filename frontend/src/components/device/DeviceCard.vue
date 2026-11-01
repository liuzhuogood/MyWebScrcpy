<script setup lang="ts">
import {
  Battery,
  BatteryCharging,
  Camera,
  Check,
  Film,
  Globe,
  Image,
  Layers,
  LayoutGrid,
  Loader2,
  Lock,
  MapPin,
  MonitorPlay,
  Music,
  Pencil,
  Phone,
  RefreshCw,
  RotateCcw,
  Settings,
  ShoppingBag,
  Smartphone,
  Thermometer,
  Trash2,
  Tv,
  Usb,
  Wifi,
  X,
} from 'lucide-vue-next'
import { computed, nextTick, ref } from 'vue'
import type { ComponentPublicInstance } from 'vue'
import { useRouter } from 'vue-router'
import { invoke } from '@tauri-apps/api/core'
import { Button } from '@/components/ui/button'
import { Input } from '@/components/ui/input'
import type { Device } from '@/api/devices'
import { useDeviceStore } from '@/stores/devices'
import { usePlayerSettings } from '@/composables/usePlayerSettings'
import { usePlayerWindows } from '@/composables/usePlayerWindows'
import { toast } from 'vue-sonner'

const props = withDefaults(
  defineProps<{
    device: Device
    viewMode?: 'deck' | 'compact'
  }>(),
  {
    viewMode: 'deck',
  },
)

const emit = defineEmits<{
  (e: 'openFloating', serial: string): void
}>()

const store = useDeviceStore()
const router = useRouter()
const playerSettings = usePlayerSettings()
const windowEffects = usePlayerWindows(computed(() => props.device.serial))

const isEditing = computed(() => store.editingSerial === props.device.serial)
const draft = ref('')
const inputRef = ref<ComponentPublicInstance | null>(null)
const isReconnecting = ref(false)
const isDeleting = ref(false)

const isManual = computed(() => props.device.source === 'manual')
const isOnline = computed(() => props.device.state === 'device')
const isConnecting = computed(() => props.device.state === 'connecting' || isReconnecting.value)
const isUnauthorized = computed(() => props.device.state === 'unauthorized')

// 手机黑屏或锁屏状态感知：黑屏 (screen_on === false) 或锁屏 (screen_locked === true) 时判定为锁定
const isLocked = computed(() => {
  if (!isOnline.value) return false
  return props.device.screen_on === false || props.device.screen_locked === true
})

// 电量状态变色：充电中翡翠绿；放电模式下 <=10% 红色告警，<=20% 暖黄预警，>20% 正常
const batteryLevelStyle = computed(() => {
  if (!isOnline.value || props.device.battery_level === undefined) {
    return { text: 'text-slate-400', icon: 'text-slate-400' }
  }
  if (props.device.battery_charging) {
    return {
      text: 'text-emerald-600 dark:text-emerald-400 font-semibold',
      icon: 'text-emerald-500',
    }
  }
  const lvl = props.device.battery_level
  if (lvl <= 10) {
    return {
      text: 'text-rose-600 dark:text-rose-400 font-bold animate-pulse',
      icon: 'text-rose-500',
    }
  }
  if (lvl <= 20) {
    return {
      text: 'text-amber-600 dark:text-amber-400 font-semibold',
      icon: 'text-amber-500',
    }
  }
  return {
    text: 'text-slate-700 dark:text-slate-200 font-medium',
    icon: 'text-slate-500',
  }
})

// 电池温度状态变色：< 38°C 良好绿色/默认，38~42°C 暖黄预警，>= 42°C 红色高热告警
const batteryTempStyle = computed(() => {
  const t = props.device.battery_temp
  if (!isOnline.value || t === undefined || t <= 0) {
    return { text: 'text-slate-400', icon: 'text-slate-400' }
  }
  if (t < 38) {
    return {
      text: 'text-emerald-600 dark:text-emerald-400',
      icon: 'text-emerald-500',
    }
  }
  if (t < 42) {
    return {
      text: 'text-amber-600 dark:text-amber-400 font-semibold',
      icon: 'text-amber-500',
    }
  }
  return {
    text: 'text-rose-600 dark:text-rose-400 font-bold animate-pulse',
    icon: 'text-rose-500',
  }
})

// CPU 温度状态变色：< 48°C 常温，48~58°C 温升，>= 58°C 告警
const cpuTempStyle = computed(() => {
  const t = props.device.cpu_temp
  if (!isOnline.value || !props.device.cpu_temp_supported || t === undefined) {
    return { text: 'text-slate-400' }
  }
  if (t < 48) return { text: 'text-emerald-600 dark:text-emerald-400' }
  if (t < 58) return { text: 'text-amber-600 dark:text-amber-400 font-semibold' }
  return { text: 'text-rose-600 dark:text-rose-400 font-bold animate-pulse' }
})

// 前台活跃应用
interface ForegroundApp {
  name: string
  pkg: string
  iconType?: string
}

const foregroundApp = computed<ForegroundApp>(() => {
  if (!isOnline.value) {
    return { name: '待机', pkg: '设备处于离线状态', iconType: 'standby' }
  }
  if (props.device.foreground_pkg) {
    return {
      name: props.device.foreground_app_name || props.device.foreground_pkg,
      pkg: props.device.foreground_pkg,
      iconType: props.device.foreground_app_icon || 'app_default',
    }
  }
  return { name: '待机', pkg: '系统待机中', iconType: 'standby' }
})

function getAppVisual(iconType?: string) {
  switch (iconType) {
    case 'wechat':
      return 'bg-[#07c160]/12 border-[#07c160]/25 text-[#07c160]'
    case 'kuaishou':
    case 'douyin':
      return 'bg-orange-500/12 border-orange-500/25 text-orange-400'
    case 'bilibili':
    case 'iqiyi':
    case 'qqvideo':
    case 'youku':
      return 'bg-pink-500/12 border-pink-500/25 text-pink-400'
    case 'neteasemusic':
    case 'qqmusic':
      return 'bg-rose-500/12 border-rose-500/25 text-rose-400'
    case 'settings':
      return 'bg-neutral-800/80 border-neutral-700 text-neutral-300'
    case 'camera':
      return 'bg-amber-500/12 border-amber-500/25 text-amber-400'
    case 'gallery':
      return 'bg-violet-500/12 border-violet-500/25 text-violet-400'
    case 'phone':
      return 'bg-emerald-500/12 border-emerald-500/25 text-emerald-400'
    case 'browser':
      return 'bg-cyan-500/12 border-cyan-500/25 text-cyan-400'
    case 'alipay':
    case 'taobao':
    case 'jd':
    case 'pinduoduo':
    case 'market':
      return 'bg-amber-500/12 border-amber-500/25 text-amber-400'
    case 'amap':
    case 'baidumap':
      return 'bg-blue-500/12 border-blue-500/25 text-blue-400'
    default:
      return 'bg-sky-500/12 border-sky-500/25 text-sky-400'
  }
}

async function startEdit(e: Event) {
  e.stopPropagation()
  store.editingSerial = props.device.serial
  draft.value = props.device.alias || store.aliases[props.device.serial] || props.device.brand || props.device.model || ''
  await nextTick()
  const el = inputRef.value?.$el as HTMLInputElement | undefined
  el?.focus()
  el?.select()
}

function save() {
  store.saveAlias(props.device.serial, draft.value, props.device.brand || props.device.model || 'Android')
  store.editingSerial = null
}

function cancel() {
  store.editingSerial = null
}

async function handleReconnect(e: Event) {
  e.stopPropagation()
  if (isConnecting.value)
    return
  isReconnecting.value = true
  toast.info('正在尝试重新握手连接...')
  try {
    await store.reconnect(props.device.serial)
    toast.success('设备连接指令已发送')
  }
  catch (err) {
    toast.error(err instanceof Error ? err.message : '连接设备失败')
  }
  finally {
    setTimeout(() => {
      isReconnecting.value = false
    }, 1500)
  }
}

async function handleDelete(e: Event) {
  e.stopPropagation()
  if (isDeleting.value)
    return
  isDeleting.value = true
  try {
    await store.removeDevice(props.device.serial)
    toast.success('已删除设备')
  }
  catch (err) {
    isDeleting.value = false
    toast.error(err instanceof Error ? err.message : '删除设备失败')
  }
}

function open() {
  if (isEditing.value || !isOnline.value)
    return
  router.push({ name: 'player', query: { serial: props.device.serial } })
}

async function openFloatingWindow(e?: Event) {
  e?.stopPropagation()
  if (isEditing.value || !isOnline.value)
    return
  if (windowEffects.native) {
    if (windowEffects.player.value) {
      await windowEffects.restore()
      toast.success('已恢复小窗显示')
      return
    }
    try {
      const HEADER_HEIGHT = 56
      const targetW = playerSettings.floatingSize.value === 'compact' ? 320 : playerSettings.floatingSize.value === 'large' ? 440 : 380
      const baseH = playerSettings.floatingSize.value === 'compact' ? 680 : playerSettings.floatingSize.value === 'large' ? 880 : 780
      const targetH = baseH + HEADER_HEIGHT
      await invoke('open_device_window', {
        serial: props.device.serial,
        title: store.displayName(props.device),
        width: targetW,
        height: targetH,
        borderless: playerSettings.borderless.value,
      })
      return
    }
    catch (e) {
      console.warn('打开桌面小窗失败，降级为网页小窗', e)
    }
  }
  emit('openFloating', props.device.serial)
}
</script>

<template>
  <!-- 模式一：真实手机拟物卡片 (Deck View, 1:1 对齐直接显示屏幕 270×508 规格) -->
  <div
    v-if="viewMode === 'deck'"
    class="flex items-center justify-center w-full select-none"
  >
    <div
      class="group relative flex flex-col w-[270px] h-[508px] overflow-hidden bg-[#0d0e11] text-white rounded-[36px] p-2 border-[3px] shadow-xl shadow-black/60 transition-all duration-200 hover:-translate-y-1 cursor-pointer select-none"
      :class="[
        isOnline
          ? 'border-neutral-800 hover:border-neutral-700 ring-1 ring-white/5'
          : isConnecting
            ? 'border-blue-500/40 opacity-80'
            : isUnauthorized
              ? 'border-amber-500/40 opacity-80'
              : 'border-neutral-900 opacity-55 grayscale',
      ]"
      title="点击手机屏幕直接启动投屏"
      @click="open"
    >
      <!-- 手机左侧物理按键凹凸微雕 (音量键 + / -) -->
      <div class="absolute -left-[3px] top-18 h-8 w-[3px] rounded-r-sm bg-neutral-700" />
      <div class="absolute -left-[3px] top-28 h-8 w-[3px] rounded-r-sm bg-neutral-700" />
      <!-- 手机右侧物理按键凹凸微雕 (电源锁屏键) -->
      <div class="absolute -right-[3px] top-22 h-10 w-[3px] rounded-l-sm bg-neutral-700" />

      <!-- 手机内屏区域 (OLED 纯黑沉浸屏幕，整屏点击直接启动投屏) -->
      <div
        class="group/screen relative flex flex-col justify-between overflow-hidden bg-black rounded-[28px] p-3 flex-1 transition-colors"
      >
        <!-- 屏幕顶部：灵动岛/听筒打孔 + 顶栏遥测状态 -->
        <div class="relative z-10 flex flex-col gap-2 shrink-0">
          <!-- 顶部中央灵动岛 / 听筒打孔 -->
          <div class="mx-auto flex items-center justify-center">
            <div
              class="h-4 rounded-full bg-black/95 px-3 flex items-center gap-1.5 border border-white/15 shadow-inner"
              :title="device.model || 'Android'"
            >
              <!-- 前置摄像头打孔微点 -->
              <span class="h-2 w-2 rounded-full bg-slate-900 border border-slate-700/80 flex items-center justify-center">
                <span class="h-0.5 w-0.5 rounded-full bg-blue-400/60" />
              </span>
              <span class="text-[9px] font-mono text-slate-400 font-semibold tracking-wider scale-90">
                {{ device.brand || 'Android' }}
              </span>
            </div>
          </div>

          <!-- 顶栏状态信息：左侧连接模式/状态，右侧遥测（电池、温度） -->
          <div class="flex items-center justify-between text-[11px] pt-0.5 px-1.5">
            <!-- 左侧：连接模式 (USB / Wi-Fi) 与状态指示 -->
            <div class="flex items-center gap-1 font-mono">
              <span
                class="flex items-center gap-1 px-1.5 py-0.5 rounded-md text-[10px] font-medium"
                :class="isManual ? 'bg-purple-500/15 text-purple-400 border border-purple-500/30' : 'bg-cyan-500/15 text-cyan-400 border border-cyan-500/30'"
              >
                <Wifi v-if="isManual" class="h-2.5 w-2.5" />
                <Usb v-else class="h-2.5 w-2.5" />
                <span>{{ isManual ? 'Wi-Fi' : 'USB' }}</span>
              </span>

              <span
                v-if="!isOnline"
                class="text-[9px] font-mono px-1 rounded"
                :class="isConnecting ? 'text-blue-400 animate-pulse' : isUnauthorized ? 'text-amber-400' : 'text-neutral-500'"
              >
                {{ isConnecting ? '连接中' : isUnauthorized ? '待授权' : '离线' }}
              </span>
            </div>

            <!-- 右侧：电池与温度遥测核心区 -->
            <div class="flex items-center gap-2 font-mono">
              <!-- 电池温度指示 -->
              <div
                class="flex items-center gap-0.5 text-[10px]"
                :class="batteryTempStyle.text"
                :title="`电池温度: ${device.battery_temp && device.battery_temp > 0 ? `${device.battery_temp.toFixed(1)}°C` : '--'}`"
              >
                <Thermometer class="h-3 w-3" :class="batteryTempStyle.icon" />
                <span>{{ isOnline && device.battery_temp && device.battery_temp > 0 ? `${device.battery_temp.toFixed(0)}°C` : '--' }}</span>
              </div>

              <!-- CPU 温度（若硬件支持） -->
              <div
                v-if="device.cpu_temp_supported && device.cpu_temp"
                class="flex items-center gap-0.5 text-[10px]"
                :class="cpuTempStyle.text"
                :title="`CPU 核心温度: ${device.cpu_temp.toFixed(1)}°C`"
              >
                <span class="text-[9px] text-neutral-500">CPU</span>
                <span>{{ device.cpu_temp.toFixed(0) }}°</span>
              </div>

              <!-- 电量百分比与充电标识 -->
              <div
                class="flex items-center gap-1 text-[10px]"
                :class="batteryLevelStyle.text"
                :title="`当前电量: ${device.battery_level !== undefined ? `${device.battery_level}%` : '--'} ${device.battery_charging ? '(充电中)' : ''}`"
              >
                <span>{{ isOnline && device.battery_level !== undefined ? `${device.battery_level}%` : '--' }}</span>
                <BatteryCharging v-if="device.battery_charging" class="h-3.5 w-3.5" :class="batteryLevelStyle.icon" />
                <Battery v-else class="h-3.5 w-3.5" :class="batteryLevelStyle.icon" />
              </div>
            </div>
          </div>
        </div>

        <!-- 手机中央屏幕区域：支持黑屏已锁定 vs 在线运行应用 vs 设备离线 -->
        <!-- 场景 1：手机黑屏/锁屏 -> 显示已锁定，不再显示运行的应用 -->
        <div v-if="isOnline && isLocked" class="relative my-auto flex flex-col items-center justify-center py-4 text-center">
          <div class="relative flex items-center justify-center rounded-2xl h-15 w-15 bg-neutral-900/90 border border-neutral-700/60 shadow-inner transition-transform duration-200 group-hover/screen:scale-105">
            <Lock class="h-7 w-7 text-neutral-400" />
          </div>
          <div class="mt-3 w-full px-2">
            <h4 class="text-sm font-medium tracking-tight text-neutral-300">
              已锁定
            </h4>
          </div>
        </div>

        <!-- 场景 2：设备在线且未锁定 -> 显示当前正在运行的应用 -->
        <div v-else-if="isOnline" class="relative my-auto flex flex-col items-center justify-center py-4 text-center">
          <div
            class="relative flex items-center justify-center border shadow-lg transition-transform duration-200 group-hover/screen:scale-102 rounded-2xl h-15 w-15"
            :class="getAppVisual(foregroundApp.iconType)"
          >
            <!-- 微信图标 -->
            <svg
              v-if="foregroundApp.iconType === 'wechat'"
              class="h-7.5 w-7.5 text-[#07C160]"
              viewBox="0 0 24 24"
              fill="currentColor"
            >
              <path d="M8.5 2C4.36 2 1 4.91 1 8.5c0 2.05 1.07 3.88 2.76 5.11L3 17.5l4.28-1.43c.4.08.8.13 1.22.13.23 0 .45-.02.67-.04-.26-.67-.42-1.39-.42-2.16 0-3.59 3.36-6.5 7.5-6.5.64 0 1.26.07 1.84.2C16.84 4.54 13.06 2 8.5 2zm-2.25 4a1.25 1.25 0 110 2.5 1.25 1.25 0 010-2.5zm4.5 0a1.25 1.25 0 110 2.5 1.25 1.25 0 010-2.5zM15.5 8.5c-3.59 0-6.5 2.46-6.5 5.5s2.91 5.5 6.5 5.5c.36 0 .71-.03 1.05-.09L20 20.5l-.9-3.08C20.9 16.38 22 14.8 22 13c0-3.04-2.91-5.5-6.5-5.5zm-2 3.5a1 1 0 110 2 1 1 0 010-2zm4 0a1 1 0 110 2 1 1 0 010-2z" />
            </svg>
            <Settings v-else-if="foregroundApp.iconType === 'settings'" class="h-7.5 w-7.5" />
            <LayoutGrid v-else-if="foregroundApp.iconType === 'launcher'" class="h-7.5 w-7.5" />
            <Camera v-else-if="foregroundApp.iconType === 'camera'" class="h-7.5 w-7.5" />
            <Image v-else-if="foregroundApp.iconType === 'gallery'" class="h-7.5 w-7.5" />
            <Phone v-else-if="foregroundApp.iconType === 'phone'" class="h-7.5 w-7.5" />
            <Globe v-else-if="foregroundApp.iconType === 'browser'" class="h-7.5 w-7.5" />
            <Film v-else-if="foregroundApp.iconType === 'kuaishou' || foregroundApp.iconType === 'douyin'" class="h-7.5 w-7.5" />
            <Tv v-else-if="foregroundApp.iconType === 'bilibili' || foregroundApp.iconType === 'iqiyi' || foregroundApp.iconType === 'qqvideo' || foregroundApp.iconType === 'youku'" class="h-7.5 w-7.5" />
            <ShoppingBag v-else-if="foregroundApp.iconType === 'alipay' || foregroundApp.iconType === 'taobao' || foregroundApp.iconType === 'jd' || foregroundApp.iconType === 'pinduoduo' || foregroundApp.iconType === 'market'" class="h-7.5 w-7.5" />
            <Music v-else-if="foregroundApp.iconType === 'neteasemusic' || foregroundApp.iconType === 'qqmusic'" class="h-7.5 w-7.5" />
            <MapPin v-else-if="foregroundApp.iconType === 'amap' || foregroundApp.iconType === 'baidumap'" class="h-7.5 w-7.5" />
            <Layers v-else class="h-7.5 w-7.5" />
          </div>

          <!-- 运行应用文字信息 -->
          <div class="mt-3 w-full px-2">
            <h4 class="truncate text-sm font-medium tracking-tight text-neutral-100 group-hover/screen:text-white transition-colors">
              {{ foregroundApp.name }}
            </h4>
            <p v-if="foregroundApp.pkg" class="truncate font-mono text-[10px] text-neutral-500 mt-0.5">
              {{ foregroundApp.pkg }}
            </p>
          </div>
        </div>

        <!-- 场景 3：设备离线/连接中 -->
        <div v-else class="relative my-auto flex flex-col items-center justify-center py-4 text-center">
          <div class="relative flex items-center justify-center rounded-2xl h-15 w-15 bg-neutral-900/60 border border-neutral-800 text-neutral-600">
            <Smartphone class="h-7 w-7" />
          </div>
          <div class="mt-3 w-full px-2">
            <h4 class="text-sm font-medium text-neutral-500">
              设备离线
            </h4>
          </div>
        </div>

        <!-- 手机底部区域：纯粹设备信息与 Android 手势条 (彻底去按钮，保留呼吸间距) -->
        <div class="relative z-10 shrink-0 border-t border-neutral-800/80 pt-2 pb-0.5 flex flex-col gap-1.5" @click.stop>
          <div class="flex items-center justify-between gap-1.5 px-0.5">
            <div class="min-w-0 flex-1 truncate">
              <!-- 编辑别名状态 -->
              <div v-if="isEditing" class="flex items-center gap-1" @click.stop>
                <Input
                  ref="inputRef"
                  v-model="draft"
                  class="h-6 rounded-md bg-neutral-900 border-neutral-700 text-xs px-1.5 text-white"
                  @keydown.enter="save"
                  @keydown.esc="cancel"
                />
                <button type="button" class="text-emerald-400 hover:text-emerald-300 p-0.5" @click.stop="save">
                  <Check class="h-3 w-3" />
                </button>
                <button type="button" class="text-neutral-400 hover:text-neutral-300 p-0.5" @click.stop="cancel">
                  <X class="h-3 w-3" />
                </button>
              </div>

              <!-- 显示别名状态 -->
              <div v-else class="flex items-center gap-1.5">
                <span class="font-medium text-neutral-200 truncate text-xs">
                  {{ store.displayName(device) }}
                </span>
                <button
                  type="button"
                  class="text-neutral-500 hover:text-neutral-200 transition-colors cursor-pointer"
                  title="编辑别名"
                  @click.stop="startEdit"
                >
                  <Pencil class="h-2.5 w-2.5" />
                </button>
              </div>

              <p class="truncate font-mono text-[9px] text-neutral-500 mt-0.5">
                {{ device.model || 'Android' }} · {{ device.serial }}
              </p>
            </div>

            <!-- 网络设备快捷重连 / 删除操作 -->
            <div v-if="isManual && !isOnline" class="flex items-center gap-1 shrink-0" @click.stop>
              <button
                type="button"
                class="p-1 rounded text-neutral-400 hover:text-blue-400 transition-colors cursor-pointer"
                title="重新连接"
                :disabled="isConnecting"
                @click.stop="handleReconnect"
              >
                <Loader2 v-if="isConnecting" class="h-3 w-3 animate-spin text-blue-400" />
                <RefreshCw v-else class="h-3 w-3" />
              </button>
              <button
                type="button"
                class="p-1 rounded text-neutral-400 hover:text-rose-400 transition-colors cursor-pointer"
                title="删除设备"
                :disabled="isDeleting"
                @click.stop="handleDelete"
              >
                <Trash2 class="h-3 w-3" />
              </button>
            </div>
          </div>

          <!-- 手机底部 Android 手势条 (Home Indicator) -->
          <div
            class="mx-auto rounded-full bg-neutral-800 transition-colors group-hover/screen:bg-neutral-600 h-1 w-14 mt-1 mb-0.5"
          />
        </div>
      </div>
    </div>
  </div>

  <!-- 模式二：紧凑型设备阵列 (Compact View) -->
  <div
    v-else
    class="group relative flex items-center justify-between rounded-2xl border border-slate-200/80 bg-white/85 p-3.5 text-slate-900 backdrop-blur-xl transition-all hover:border-blue-500/50 hover:bg-white/95 shadow-sm dark:border-white/10 dark:bg-slate-900/60 dark:text-white dark:hover:border-blue-400/40 dark:hover:bg-slate-900/80 select-none cursor-pointer"
    @click="open"
  >
    <div class="flex items-center gap-3 min-w-0 flex-1">
      <div
        class="flex h-10 w-10 shrink-0 items-center justify-center rounded-xl"
        :class="isOnline ? 'bg-emerald-500/15 text-emerald-600 border border-emerald-500/30' : 'bg-slate-100 text-slate-500 border border-slate-200 dark:bg-white/5 dark:text-slate-400 dark:border-white/10'"
      >
        <Smartphone class="h-5 w-5" />
      </div>

      <div class="min-w-0 flex-1">
        <div class="flex items-center gap-2">
          <p class="truncate font-semibold text-sm text-slate-900 group-hover:text-blue-600 dark:text-white dark:group-hover:text-blue-300 transition-colors">
            {{ store.displayName(device) }}
          </p>
          <span
            class="h-1.5 w-1.5 rounded-full shrink-0"
            :class="isOnline ? 'bg-emerald-500' : isConnecting ? 'bg-blue-500 animate-pulse' : 'bg-slate-400'"
          />
          <span v-if="device.android_release" class="rounded bg-slate-100 dark:bg-white/10 px-1.5 py-0.2 text-[9px] font-mono text-slate-500 dark:text-slate-400">
            Android {{ device.android_release }}
          </span>
        </div>
        <p class="truncate font-mono text-[11px] text-slate-500 dark:text-slate-400 mt-0.5">
          {{ device.model || 'Android' }} · {{ device.serial }}
        </p>
      </div>

      <!-- 紧凑指标条（电量与电池温度） -->
      <div v-if="isOnline" class="hidden md:flex items-center gap-3 text-xs">
        <div class="flex items-center gap-1 font-mono" :class="batteryLevelStyle.text">
          <BatteryCharging v-if="device.battery_charging" class="h-3.5 w-3.5" :class="batteryLevelStyle.icon" />
          <Battery v-else class="h-3.5 w-3.5" :class="batteryLevelStyle.icon" />
          <span>{{ device.battery_level !== undefined ? `${device.battery_level}%` : '--' }}</span>
        </div>
        <div class="flex items-center gap-1 font-mono" :class="batteryTempStyle.text">
          <Thermometer class="h-3.5 w-3.5" :class="batteryTempStyle.icon" />
          <span>{{ device.battery_temp && device.battery_temp > 0 ? `${device.battery_temp.toFixed(1)}°C` : '--' }}</span>
        </div>
      </div>
    </div>

    <!-- 紧凑右侧操作（纯蓝色 #406BDB） -->
    <div class="flex items-center gap-1.5 shrink-0 ml-3" @click.stop>
      <Button
        v-if="isOnline"
        size="sm"
        class="h-8 rounded-xl bg-[#406BDB] hover:bg-[#3258be] text-white text-xs px-3 shadow-xs cursor-pointer"
        @click.stop="open"
      >
        <MonitorPlay class="mr-1 h-3.5 w-3.5" />
        投屏
      </Button>
      <Button
        v-if="isOnline"
        size="icon"
        variant="ghost"
        class="h-8 w-8 rounded-xl border p-0 cursor-pointer shadow-2xs transition-all"
        :class="[
          windowEffects.player.value
            ? 'border-blue-500/50 bg-blue-500/10 text-blue-600 dark:border-blue-400/40 dark:bg-blue-500/20 dark:text-blue-300 hover:bg-blue-500/20'
            : 'border-slate-200/80 bg-white/80 text-slate-700 hover:text-slate-950 dark:border-white/10 dark:bg-white/5 dark:text-slate-300 dark:hover:text-white',
        ]"
        :title="windowEffects.player.value ? '恢复画中画小窗' : '画中画'"
        :aria-label="windowEffects.player.value ? '恢复画中画小窗' : '画中画'"
        @click.stop="openFloatingWindow"
      >
        <RotateCcw v-if="windowEffects.needsRecovery.value" class="h-3.5 w-3.5 text-amber-600 dark:text-amber-400" />
        <Tv v-else class="h-3.5 w-3.5" />
      </Button>
      <Button
        size="icon"
        variant="ghost"
        class="h-8 w-8 rounded-xl text-slate-500 hover:text-slate-950 dark:text-slate-400 dark:hover:text-white cursor-pointer"
        title="重命名"
        @click.stop="startEdit"
      >
        <Pencil class="h-3.5 w-3.5" />
      </Button>
      <Button
        v-if="isManual"
        size="icon"
        variant="ghost"
        class="h-8 w-8 rounded-xl text-slate-500 hover:text-red-600 dark:text-slate-400 dark:hover:text-red-400 cursor-pointer"
        title="删除"
        @click.stop="handleDelete"
      >
        <Trash2 class="h-3.5 w-3.5" />
      </Button>
    </div>
  </div>
</template>
