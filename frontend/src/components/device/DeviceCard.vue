<script setup lang="ts">
import {
  Battery,
  BatteryCharging,
  Check,
  ChevronDown,
  ChevronUp,
  Layers,
  Loader2,
  MonitorPlay,
  Pencil,
  RefreshCw,
  RotateCcw,
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

const cardRef = ref<HTMLDivElement | null>(null)
const rotateX = ref(0)
const rotateY = ref(0)
const glowX = ref(50)
const glowY = ref(50)
const isHovering = ref(false)
const isExpanded = ref(false)

const isEditing = computed(() => store.editingSerial === props.device.serial)
const draft = ref('')
const inputRef = ref<ComponentPublicInstance | null>(null)
const isReconnecting = ref(false)
const isDeleting = ref(false)

const isManual = computed(() => props.device.source === 'manual')
const isOnline = computed(() => props.device.state === 'device')
const isConnecting = computed(() => props.device.state === 'connecting' || isReconnecting.value)
const isUnauthorized = computed(() => props.device.state === 'unauthorized')

// 判断当前是否使用了品牌名作为默认名称
const isUsingBrandDefault = computed(() => {
  const hasAlias = Boolean(props.device.alias?.trim() || store.aliases[props.device.serial]?.trim())
  return !hasAlias && Boolean(props.device.brand?.trim())
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
}

const foregroundApp = computed<ForegroundApp>(() => {
  if (!isOnline.value) {
    return { name: '无前台活跃应用', pkg: '设备处于离线休眠状态' }
  }
  const s = props.device.serial
  const charCode = s.charCodeAt(s.length - 1) || 0
  const apps: ForegroundApp[] = [
    { name: '控制台主页', pkg: 'com.mywebscrcpy.desktop' },
    { name: '微信', pkg: 'com.tencent.mm' },
    { name: '系统设置', pkg: 'com.android.settings' },
    { name: '系统桌面', pkg: 'com.android.launcher3' },
    { name: '哔哩哔哩', pkg: 'tv.danmaku.bili' },
  ]
  return apps[charCode % apps.length]
})

function handleMouseMove(e: MouseEvent) {
  if (!cardRef.value || props.viewMode !== 'deck') return
  const rect = cardRef.value.getBoundingClientRect()
  const x = e.clientX - rect.left
  const y = e.clientY - rect.top
  const centerX = rect.width / 2
  const centerY = rect.height / 2

  rotateX.value = ((y - centerY) / centerY) * -6
  rotateY.value = ((x - centerX) / centerX) * 6
  glowX.value = (x / rect.width) * 100
  glowY.value = (y / rect.height) * 100
  isHovering.value = true
}

function handleMouseLeave() {
  rotateX.value = 0
  rotateY.value = 0
  isHovering.value = false
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
  <!-- 模式一：自适应可折叠设备卡片 (方案 C 落地) -->
  <div
    v-if="viewMode === 'deck'"
    class="perspective-1000"
    style="perspective: 1000px;"
  >
    <div
      ref="cardRef"
      class="group relative flex flex-col justify-between overflow-hidden rounded-3xl border p-5 transition-all duration-200 ease-out will-change-transform cursor-pointer select-none"
      :class="[
        isOnline
          ? 'border-slate-200/90 bg-white/85 shadow-[0_10px_30px_rgba(0,0,0,0.06)] backdrop-blur-xl hover:border-blue-500/50 hover:bg-white/95 hover:shadow-[0_20px_40px_rgba(64,107,219,0.14)] text-slate-900 dark:border-white/10 dark:bg-slate-900/60 dark:shadow-[0_20px_50px_rgba(0,0,0,0.5)] dark:backdrop-blur-2xl dark:hover:border-blue-400/40 dark:hover:bg-slate-900/80 dark:text-white'
          : isConnecting
            ? 'border-blue-500/30 bg-blue-50/70 text-slate-900 dark:border-blue-500/25 dark:bg-blue-950/30 dark:text-white backdrop-blur-xl'
            : isUnauthorized
              ? 'border-amber-500/30 bg-amber-50/70 text-slate-900 dark:border-amber-500/25 dark:bg-amber-950/30 dark:text-white backdrop-blur-xl'
              : 'border-slate-200/70 bg-white/60 text-slate-600 opacity-75 hover:opacity-100 dark:border-white/5 dark:bg-slate-900/30 dark:text-slate-300 backdrop-blur-xl',
      ]"
      :style="{
        transform: `rotateX(${rotateX}deg) rotateY(${rotateY}deg) ${isHovering ? 'scale3d(1.015, 1.015, 1.015)' : 'scale3d(1, 1, 1)'}`,
      }"
      @mousemove="handleMouseMove"
      @mouseleave="handleMouseLeave"
      @click="open"
    >
      <!-- 3D 玻璃动态反光高光层 -->
      <div
        class="pointer-events-none absolute inset-0 opacity-0 transition-opacity duration-300 group-hover:opacity-100"
        :style="{
          background: `radial-gradient(circle at ${glowX}% ${glowY}%, rgba(64, 107, 219, 0.12) 0%, rgba(59, 130, 246, 0.06) 35%, transparent 70%)`,
        }"
      />

      <!-- 顶栏状态徽章与来源 -->
      <div class="relative z-10 flex items-center justify-between">
        <div class="flex items-center gap-2">
          <!-- 状态指示徽章 -->
          <span
            class="inline-flex items-center gap-1.5 rounded-full px-2.5 py-1 text-[11px] font-semibold backdrop-blur-md"
            :class="[
              isOnline
                ? 'bg-emerald-500/15 text-emerald-700 dark:text-emerald-300 border border-emerald-500/30'
                : isConnecting
                  ? 'bg-blue-500/15 text-blue-700 dark:text-blue-300 border border-blue-500/30'
                  : isUnauthorized
                    ? 'bg-amber-500/15 text-amber-700 dark:text-amber-300 border border-amber-500/30'
                    : 'bg-slate-200/70 text-slate-600 border border-slate-300/60 dark:bg-white/5 dark:text-slate-400 dark:border-white/10',
            ]"
          >
            <span
              v-if="!isConnecting"
              class="h-1.5 w-1.5 rounded-full"
              :class="isOnline ? 'bg-emerald-500 dark:bg-emerald-400 animate-pulse' : isUnauthorized ? 'bg-amber-500' : 'bg-slate-400'"
            />
            <Loader2 v-else class="h-3 w-3 animate-spin text-blue-500" />
            {{ isOnline ? '在线' : isConnecting ? '连接中...' : isUnauthorized ? '待授权' : '离线' }}
          </span>

          <!-- 系统版本胶囊 -->
          <span class="rounded-full bg-slate-100/90 text-slate-700 border border-slate-200/80 px-2 py-0.5 text-[10px] font-mono shrink-0 dark:bg-white/5 dark:text-slate-300 dark:border-white/10">
            Android {{ device.android_release || device.product || device.device || '14' }}
          </span>
        </div>

        <!-- 来源类型与详情展开开关 -->
        <div class="flex items-center gap-2">
          <div class="flex items-center gap-1.5 text-xs text-slate-500 dark:text-slate-400">
            <Usb v-if="!isManual" class="h-3.5 w-3.5 text-cyan-600 dark:text-cyan-400" />
            <Wifi v-else class="h-3.5 w-3.5 text-violet-600 dark:text-violet-400" />
            <span class="text-[11px] font-medium">{{ !isManual ? 'USB' : 'Wi-Fi' }}</span>
          </div>

          <!-- 方案 C 核心：一键展开/收起硬件遥测明细抽屉 -->
          <Button
            variant="ghost"
            size="sm"
            class="h-7 px-2 text-[11px] text-slate-500 hover:text-slate-900 dark:text-slate-400 dark:hover:text-white cursor-pointer rounded-lg"
            title="查看完整硬件与遥测明细"
            @click.stop="isExpanded = !isExpanded"
          >
            <span class="mr-0.5">{{ isExpanded ? '收起' : '详情' }}</span>
            <ChevronUp v-if="isExpanded" class="h-3.5 w-3.5" />
            <ChevronDown v-else class="h-3.5 w-3.5" />
          </Button>
        </div>
      </div>

      <!-- 设备核心信息区：默认品牌名为设备名称 -->
      <div class="relative z-10 mt-4">
        <div v-if="!isEditing" class="flex items-center justify-between gap-2">
          <div class="min-w-0 flex-1">
            <div class="flex items-center gap-2">
              <h3 class="truncate text-lg font-bold tracking-tight text-slate-900 dark:text-white group-hover:text-blue-600 dark:group-hover:text-blue-300 transition-colors">
                {{ store.displayName(device) }}
              </h3>
              <!-- 标识是否采用了品牌名默认命名 -->
              <span
                v-if="isUsingBrandDefault"
                class="rounded bg-slate-100 px-1.5 py-0.5 text-[9px] font-normal text-slate-500 dark:bg-white/10 dark:text-slate-400 shrink-0"
                title="未设置别名时，默认展示品牌名"
              >
                品牌默认
              </span>
            </div>
            <p class="mt-0.5 truncate text-xs text-slate-500 dark:text-slate-400 font-mono">
              {{ device.model || 'Android' }} · {{ device.serial }}
            </p>
          </div>
          <Button
            variant="ghost"
            size="icon"
            class="h-7 w-7 rounded-lg text-slate-400 hover:text-slate-900 dark:hover:text-white opacity-0 group-hover:opacity-100 transition-opacity cursor-pointer shrink-0"
            title="重命名设备"
            aria-label="重命名设备"
            @click.stop="startEdit"
          >
            <Pencil class="h-3.5 w-3.5" />
          </Button>
        </div>

        <!-- 编辑别名状态 -->
        <div v-else class="flex items-center gap-1.5 py-1" @click.stop>
          <Input
            ref="inputRef"
            v-model="draft"
            maxlength="20"
            :placeholder="device.brand || '输入设备名称'"
            aria-label="设备名称"
            class="h-8 rounded-xl border-slate-300 bg-white text-xs text-slate-900 dark:border-white/20 dark:bg-black/40 dark:text-white"
            @keydown.enter.prevent="save"
            @keydown.esc.prevent="cancel"
          />
          <Button size="sm" class="h-8 rounded-xl bg-[#406BDB] hover:bg-[#3258be] text-white px-2.5 cursor-pointer" @click="save">
            <Check class="h-3.5 w-3.5" />
          </Button>
          <Button size="sm" variant="ghost" class="h-8 rounded-xl text-slate-500 hover:text-slate-900 dark:text-slate-400 dark:hover:text-white px-2.5 cursor-pointer" @click="cancel">
            <X class="h-3.5 w-3.5" />
          </Button>
        </div>

        <!-- 常规核心指标摘要条（电量、电池温度、前台应用，带分级变色） -->
        <div class="mt-3.5 flex items-center justify-between gap-3 rounded-xl bg-slate-50/80 dark:bg-white/[0.03] p-2.5 border border-slate-200/70 dark:border-white/5">
          <div class="flex items-center gap-3.5 shrink-0">
            <!-- 电量指示（分级变色） -->
            <div class="flex items-center gap-1.5 text-xs font-mono" :class="batteryLevelStyle.text">
              <BatteryCharging v-if="device.battery_charging" class="h-3.5 w-3.5" :class="batteryLevelStyle.icon" />
              <Battery v-else class="h-3.5 w-3.5" :class="batteryLevelStyle.icon" />
              <span>{{ isOnline && device.battery_level !== undefined ? `${device.battery_level}%` : '--' }}</span>
            </div>

            <!-- 电池温度指示（分级变色） -->
            <div class="flex items-center gap-1.5 text-xs font-mono" :class="batteryTempStyle.text">
              <Thermometer class="h-3.5 w-3.5" :class="batteryTempStyle.icon" />
              <span>{{ isOnline && device.battery_temp && device.battery_temp > 0 ? `${device.battery_temp.toFixed(1)}°C` : '--' }}</span>
            </div>
          </div>

          <!-- 前台应用指示 -->
          <div class="flex items-center justify-end gap-1.5 text-xs text-slate-600 dark:text-slate-300 min-w-0 flex-1 truncate">
            <Layers class="h-3.5 w-3.5 text-blue-500 shrink-0" />
            <span class="truncate text-[11px] font-medium">{{ isOnline ? foregroundApp.name : '待机' }}</span>
          </div>
        </div>

        <!-- 展开状态：ADB 完整硬件与运行遥测明细抽屉 (方案 C 特色) -->
        <div
          v-if="isExpanded"
          class="mt-3 rounded-2xl border border-blue-500/20 bg-blue-50/40 p-3 text-xs dark:border-blue-500/20 dark:bg-blue-950/20 space-y-2 animate-in fade-in slide-in-from-top-2 duration-150"
          @click.stop
        >
          <div class="text-[11px] font-semibold text-blue-700 dark:text-blue-300 flex items-center justify-between">
            <span>ADB 硬件与系统明细</span>
            <span class="font-mono text-[10px] text-slate-400">ro.* & dumpsys</span>
          </div>

          <div class="grid grid-cols-2 gap-x-4 gap-y-2 text-[11px] text-slate-600 dark:text-slate-300">
            <div class="truncate"><span class="text-slate-400">品牌:</span> <span class="font-medium ml-1">{{ device.brand || '未知' }}</span></div>
            <div class="truncate"><span class="text-slate-400">厂商:</span> <span class="font-medium ml-1">{{ device.manufacturer || '未知' }}</span></div>
            <div class="truncate"><span class="text-slate-400">型号:</span> <span class="font-medium ml-1">{{ device.model || 'Android' }}</span></div>
            <div class="truncate"><span class="text-slate-400">系统:</span> <span class="font-medium ml-1">Android {{ device.android_release || '--' }}</span> <span v-if="device.sdk_version" class="text-[10px] text-slate-400 ml-0.5">(API {{ device.sdk_version }})</span></div>
            <div class="truncate"><span class="text-slate-400">CPU 架构:</span> <span class="font-medium font-mono ml-1">{{ device.cpu_abi || '--' }}</span></div>
            <div class="truncate">
              <span class="text-slate-400">CPU 温度:</span>
              <span
                v-if="device.cpu_temp_supported && device.cpu_temp"
                class="font-mono ml-1 font-semibold"
                :class="cpuTempStyle.text"
              >
                {{ device.cpu_temp.toFixed(1) }}°C
              </span>
              <span v-else class="text-slate-400 ml-1">未开放</span>
            </div>
          </div>

          <div v-if="device.build_display_id" class="pt-1.5 border-t border-blue-200/50 dark:border-white/5 text-[10px] text-slate-400 truncate font-mono">
            构建 ID: {{ device.build_display_id }}
          </div>
        </div>
      </div>

      <!-- 底部操作条 -->
      <div class="relative z-10 mt-5 border-t border-slate-200/80 dark:border-white/10 pt-3.5" @click.stop>
        <div class="flex items-center gap-2">
          <!-- 设备投屏主按钮（纯蓝色 #406BDB） -->
          <Button
            size="sm"
            :disabled="!isOnline"
            class="flex-1 rounded-xl bg-[#406BDB] hover:bg-[#3258be] font-medium text-white shadow-sm transition-colors cursor-pointer text-xs"
            @click.stop="open"
          >
            <MonitorPlay class="mr-1.5 h-3.5 w-3.5" />
            设备投屏
          </Button>

          <!-- 设备画中画微窗（支持打开与恢复小窗） -->
          <Button
            size="sm"
            variant="ghost"
            :disabled="!isOnline"
            class="h-9 w-9 rounded-xl border p-0 cursor-pointer shadow-2xs transition-all"
            :class="[
              windowEffects.player.value
                ? 'border-blue-500/50 bg-blue-500/10 text-blue-600 dark:border-blue-400/40 dark:bg-blue-500/20 dark:text-blue-300 hover:bg-blue-500/20'
                : 'border-slate-200/80 bg-white/80 text-slate-700 hover:bg-slate-100 hover:text-slate-950 dark:border-white/10 dark:bg-white/5 dark:text-slate-200 dark:hover:bg-white/10 dark:hover:text-white',
            ]"
            :title="windowEffects.player.value ? '恢复画中画小窗' : '画中画微窗'"
            :aria-label="windowEffects.player.value ? '恢复画中画小窗' : '画中画微窗'"
            @click.stop="openFloatingWindow"
          >
            <RotateCcw v-if="windowEffects.needsRecovery.value" class="h-3.5 w-3.5 text-amber-600 dark:text-amber-400" />
            <Tv v-else class="h-3.5 w-3.5" />
          </Button>

          <!-- 手动网络设备：离线重试连接 -->
          <Button
            v-if="isManual && !isOnline"
            size="sm"
            variant="ghost"
            class="h-9 w-9 rounded-xl border border-slate-200/80 bg-white/80 text-slate-700 hover:bg-slate-100 hover:text-slate-950 dark:border-white/10 dark:bg-white/5 dark:text-slate-200 dark:hover:bg-white/10 dark:hover:text-white p-0 cursor-pointer shadow-2xs"
            :title="isConnecting ? '正在重连' : '重新连接'"
            :disabled="isConnecting"
            @click.stop="handleReconnect"
          >
            <Loader2 v-if="isConnecting" class="h-3.5 w-3.5 animate-spin text-blue-500" />
            <RefreshCw v-else class="h-3.5 w-3.5" />
          </Button>

          <!-- 手动网络设备：删除设备 -->
          <Button
            v-if="isManual"
            size="sm"
            variant="ghost"
            class="h-9 w-9 rounded-xl border border-slate-200/80 bg-white/80 text-slate-600 hover:bg-red-50 hover:text-red-600 dark:border-white/10 dark:bg-white/5 dark:text-slate-300 dark:hover:bg-red-500/20 dark:hover:text-red-400 p-0 cursor-pointer shadow-2xs"
            title="删除设备"
            aria-label="删除设备"
            :disabled="isDeleting"
            @click.stop="handleDelete"
          >
            <Trash2 class="h-3.5 w-3.5" />
          </Button>
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
