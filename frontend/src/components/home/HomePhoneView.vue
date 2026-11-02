<script setup lang="ts">
import {
  Battery,
  BatteryCharging,
  Camera,
  CheckSquare,
  EyeOff,
  Film,
  Globe,
  Image,
  Layers,
  LayoutGrid,
  Lock,
  MapPin,
  Music,
  Phone,
  Settings,
  ShoppingBag,
  Smartphone,
  Square,
  Thermometer,
  Tv,
  Usb,
  Wifi,
} from 'lucide-vue-next'
import { computed, ref } from 'vue'
import { useRouter } from 'vue-router'
import { useElementSize } from '@vueuse/core'
import DashTile from '@/components/dashboard/DashTile.vue'
import { Button } from '@/components/ui/button'
import { Empty, EmptyDescription, EmptyHeader, EmptyMedia, EmptyTitle } from '@/components/ui/empty'
import type { Device } from '@/api/devices'
import {
  HOME_PHONE_BASE_WIDTH,
  type HomeViewMode,
  usePlayerSettings,
} from '@/composables/usePlayerSettings'
import { useDeviceStore } from '@/stores/devices'
import { computeMonitorGrid } from '@/utils/monitorGrid'
import { getPhoneTileDimensionClasses } from '@/utils/phoneGrid'
import {
  type DashSize,
  isDeviceOnlineForStream,
} from '@/composables/useHomePhoneView'

const props = withDefaults(
  defineProps<{
    visibleDevices: Device[]
    dashSize: DashSize
    serialToNumberMap: Map<string, string>
    selectedSerials: string[]
    mockEnabled?: boolean
    viewMode?: HomeViewMode
  }>(),
  {
    viewMode: 'phone',
  },
)

const emit = defineEmits<{
  (e: 'toggleSelect', serial: string): void
  (e: 'showAll'): void
}>()

const router = useRouter()
const deviceStore = useDeviceStore()
const { homePhoneScale } = usePlayerSettings()
const cardWidth = computed(() => Math.round(HOME_PHONE_BASE_WIDTH * homePhoneScale.value / 100))
const cardHeight = computed(() => Math.round(cardWidth.value * 1.88))
const containerRef = ref<HTMLElement | null>(null)
const { width: containerWidth, height: containerHeight } = useElementSize(containerRef)

const dimensionConfig = computed(() => getPhoneTileDimensionClasses(props.dashSize))

const gridStyle = computed(() => {
  if (props.dashSize === 'fit') {
    return {
      display: 'grid',
      gridTemplateColumns: `repeat(auto-fill, minmax(${cardWidth.value}px, 1fr))`,
      gridAutoRows: `${cardHeight.value}px`,
      gap: '2px',
      width: '100%',
    }
  }

  if (props.visibleDevices.length === 0)
    return {}

  const N = props.visibleDevices.length
  const W = containerWidth.value || window.innerWidth
  const H = containerHeight.value || (window.innerHeight - 220)
  const { rows, cols } = computeMonitorGrid(N, W, H)

  return {
    display: 'grid',
    gridTemplateColumns: `repeat(${cols}, minmax(0, 1fr))`,
    gridTemplateRows: `repeat(${rows}, minmax(0, 1fr))`,
    gap: '2px',
    height: '100%',
    width: '100%',
  }
})

// 判定设备是否黑屏休眠或处于锁屏状态
function isLocked(d: Device): boolean {
  if (d.state !== 'device') return false
  return d.screen_on === false || d.screen_locked === true
}

// 电池温度状态色
function getBatteryTempClass(d: Device) {
  const t = d.battery_temp
  if (d.state !== 'device' || !t || t <= 0) return 'text-slate-400'
  if (t < 38) return 'text-emerald-400'
  if (t < 42) return 'text-amber-400 font-semibold'
  return 'text-rose-400 font-bold animate-pulse'
}

// CPU 核心温度状态色
function getCpuTempClass(d: Device) {
  const t = d.cpu_temp
  if (d.state !== 'device' || !d.cpu_temp_supported || !t) return 'text-slate-400'
  if (t < 48) return 'text-emerald-400'
  if (t < 58) return 'text-amber-400 font-semibold'
  return 'text-rose-400 font-bold animate-pulse'
}

// 电池电量状态色
function getBatteryLevelClass(d: Device) {
  if (d.state !== 'device' || d.battery_level === undefined) return 'text-slate-400'
  if (d.battery_charging) return 'text-emerald-400 font-semibold'
  const lvl = d.battery_level
  if (lvl <= 10) return 'text-rose-400 font-bold animate-pulse'
  if (lvl <= 20) return 'text-amber-400 font-semibold'
  return 'text-neutral-200 font-medium'
}

// 前台应用视觉色阶
function getAppVisual(iconType?: string) {
  switch (iconType) {
    case 'wechat':
      return 'bg-[#07c160]/15 border-[#07c160]/30 text-[#07c160]'
    case 'kuaishou':
    case 'douyin':
      return 'bg-orange-500/15 border-orange-500/30 text-orange-400'
    case 'bilibili':
    case 'iqiyi':
    case 'qqvideo':
    case 'youku':
      return 'bg-pink-500/15 border-pink-500/30 text-pink-400'
    case 'neteasemusic':
    case 'qqmusic':
      return 'bg-rose-500/15 border-rose-500/30 text-rose-400'
    case 'settings':
      return 'bg-neutral-800/80 border-neutral-700 text-neutral-300'
    case 'camera':
      return 'bg-amber-500/15 border-amber-500/30 text-amber-400'
    case 'gallery':
      return 'bg-violet-500/15 border-violet-500/30 text-violet-400'
    case 'phone':
      return 'bg-emerald-500/15 border-emerald-500/30 text-emerald-400'
    case 'browser':
      return 'bg-cyan-500/15 border-cyan-500/30 text-cyan-400'
    case 'alipay':
    case 'taobao':
    case 'jd':
    case 'pinduoduo':
    case 'market':
      return 'bg-amber-500/15 border-amber-500/30 text-amber-400'
    case 'amap':
    case 'baidumap':
      return 'bg-blue-500/15 border-blue-500/30 text-blue-400'
    default:
      return 'bg-sky-500/15 border-sky-500/30 text-sky-400'
  }
}

function openPlayer(serial: string) {
  router.push({ name: 'player', query: { serial } })
}
</script>

<template>
  <div
    class="flex flex-col"
    :class="{ 'h-[calc(100vh-140px)] min-h-[500px] overflow-hidden': dashSize === 'fit' }"
  >
    <!-- 全隐藏空状态展示与恢复入口 -->
    <div
      v-if="visibleDevices.length === 0"
      class="flex flex-col items-center justify-center p-12 text-slate-400 space-y-3 rounded-3xl border border-dashed border-slate-200 dark:border-white/10 bg-white/40 dark:bg-white/[0.01]"
    >
      <Empty>
        <EmptyHeader>
          <EmptyMedia variant="icon"><EyeOff class="h-8 w-8 text-slate-400" /></EmptyMedia>
          <EmptyTitle>所有设备已隐藏</EmptyTitle>
          <EmptyDescription>
            当前所有设备均已隐藏，可通过右侧编号面板或点击下方按钮重新显示
          </EmptyDescription>
        </EmptyHeader>
        <Button
          variant="outline"
          size="sm"
          class="rounded-xl mt-3 cursor-pointer"
          @click="emit('showAll')"
        >
          全显
        </Button>
      </Empty>
    </div>

    <!-- 手机视图卡片主体（顶端与右侧面板自然对齐） -->
    <div
      v-else
      ref="containerRef"
      class="w-full flex-1 min-h-0"
      :class="dimensionConfig.containerClass"
      :style="gridStyle"
    >
      <div
        v-for="d in visibleDevices"
        :key="d.serial"
        class="group relative flex flex-col rounded-[2px] border border-neutral-800 bg-slate-900 overflow-hidden transition-all duration-150 select-none"
        :class="[
          props.dashSize === 'fit' ? 'min-h-0 min-w-0' : dimensionConfig.cardClass,
          selectedSerials.includes(d.serial) ? 'ring-1 ring-violet-500 border-violet-500' : 'hover:border-neutral-600',
        ]"
      >
        <!-- 手机外框顶栏：编号标识、多选勾选、别名与状态（显隐统一由右侧面板编号方块控制） -->
        <div class="flex h-7 items-center justify-between border-b border-white/10 bg-slate-950/80 px-2.5 py-1 text-xs text-white shrink-0 select-none z-10">
          <div class="flex min-w-0 items-center gap-1.5">
            <button
              v-if="d.state === 'device' && !mockEnabled"
              type="button"
              class="flex h-5 w-5 shrink-0 items-center justify-center rounded text-white transition-colors cursor-pointer"
              :class="selectedSerials.includes(d.serial) ? 'text-violet-400' : 'text-slate-400 hover:text-white'"
              :title="selectedSerials.includes(d.serial) ? '取消勾选' : '勾选此设备'"
              @click.stop="emit('toggleSelect', d.serial)"
            >
              <component :is="selectedSerials.includes(d.serial) ? CheckSquare : Square" class="h-4 w-4" />
            </button>

            <!-- 稳定编号标识徽章 -->
            <span
              v-if="serialToNumberMap.has(d.serial)"
              class="px-1.5 py-0.2 rounded font-mono text-[10px] font-bold bg-violet-500/20 text-violet-300 border border-violet-500/30 shrink-0"
              :title="`设备编号: ${serialToNumberMap.get(d.serial)}`"
            >
              {{ serialToNumberMap.get(d.serial) }}
            </span>

            <div class="min-w-0 truncate">
              <span class="truncate font-semibold text-[11px] text-slate-100">
                {{ deviceStore.displayName(d) }}
              </span>
            </div>
          </div>

          <div class="flex items-center gap-1.5 shrink-0">
            <span
              class="h-2 w-2 rounded-full"
              :class="d.state === 'device' ? 'bg-emerald-500' : 'bg-amber-500'"
              :title="d.state === 'device' ? '在线' : d.state"
            />
          </div>
        </div>

        <!-- 模式一：免投屏卡片模式 (Deck View) -> 展示温度、电量、CPU、应用图标/已锁定 -->
        <div
          v-if="viewMode === 'deck'"
          class="relative w-full flex-1 min-h-0 bg-black overflow-hidden flex flex-col justify-between p-3 select-none cursor-pointer group/card"
          title="点击进入设备投屏控制"
          @click="openPlayer(d.serial)"
        >
          <!-- 顶部状态遥测栏：左侧连接模式 (USB / Wi-Fi)，右侧温度 (电池+CPU) 与电量 -->
          <div class="flex items-center justify-between text-[11px] font-mono shrink-0 pt-1 px-1.5 z-10">
            <!-- 左侧：连接模式 (USB / Wi-Fi) -->
            <div class="flex items-center gap-1 shrink-0">
              <span
                class="flex items-center gap-1 px-1.5 py-0.5 rounded text-[10px] font-medium whitespace-nowrap"
                :class="d.source === 'manual' ? 'bg-purple-500/15 text-purple-400 border border-purple-500/30' : 'bg-cyan-500/15 text-cyan-400 border border-cyan-500/30'"
              >
                <Wifi v-if="d.source === 'manual'" class="h-2.5 w-2.5 shrink-0" />
                <Usb v-else class="h-2.5 w-2.5 shrink-0" />
                <span>{{ d.source === 'manual' ? 'Wi-Fi' : 'USB' }}</span>
              </span>
            </div>

            <!-- 右侧：温度 (电池+CPU) 与电量 -->
            <div class="flex items-center gap-2 shrink-0 whitespace-nowrap">
              <!-- 电池温度 -->
              <div
                class="flex items-center gap-0.5 text-[10px]"
                :class="getBatteryTempClass(d)"
                :title="`电池温度: ${d.battery_temp && d.battery_temp > 0 ? `${d.battery_temp.toFixed(1)}°C` : '--'}`"
              >
                <Thermometer class="h-3 w-3" />
                <span>{{ d.battery_temp && d.battery_temp > 0 ? `${d.battery_temp.toFixed(0)}°C` : '--' }}</span>
              </div>

              <!-- CPU 温度（若硬件支持） -->
              <div
                v-if="d.cpu_temp_supported && d.cpu_temp"
                class="flex items-center gap-0.5 text-[10px]"
                :class="getCpuTempClass(d)"
                :title="`CPU 核心温度: ${d.cpu_temp.toFixed(1)}°C`"
              >
                <span class="text-[9px] text-neutral-500">CPU</span>
                <span>{{ d.cpu_temp.toFixed(0) }}°</span>
              </div>

              <!-- 电量百分比与充放电图标 -->
              <div
                class="flex items-center gap-1 text-[10px]"
                :class="getBatteryLevelClass(d)"
                :title="`电量: ${d.battery_level !== undefined ? `${d.battery_level}%` : '--'} ${d.battery_charging ? '(充电中)' : ''}`"
              >
                <span>{{ d.battery_level !== undefined ? `${d.battery_level}%` : '--' }}</span>
                <BatteryCharging v-if="d.battery_charging" class="h-3.5 w-3.5" />
                <Battery v-else class="h-3.5 w-3.5" />
              </div>
            </div>
          </div>

          <!-- 屏幕中间核心区：黑屏已锁定 vs 正常运行应用 vs 设备离线 -->
          <!-- 场景 1：手机黑屏/锁屏 -> 显示已锁定，不再显示运行的应用 -->
          <div
            v-if="d.state === 'device' && isLocked(d)"
            class="relative my-auto flex flex-col items-center justify-center py-4 text-center"
          >
            <div class="relative flex items-center justify-center rounded-2xl h-14 w-14 bg-neutral-900/90 border border-neutral-700/60 shadow-inner transition-transform duration-200 group-hover/card:scale-105">
              <Lock class="h-7 w-7 text-neutral-400" />
            </div>
            <div class="mt-3 w-full px-2">
              <h4 class="text-sm font-medium tracking-tight text-neutral-300">
                已锁定
              </h4>
            </div>
          </div>

          <!-- 场景 2：设备在线且未锁定 -> 显示当前正在运行的应用 -->
          <div
            v-else-if="d.state === 'device'"
            class="relative my-auto flex flex-col items-center justify-center py-4 text-center"
          >
            <div
              class="relative flex items-center justify-center border shadow-lg transition-transform duration-200 group-hover/card:scale-105 rounded-2xl h-14 w-14"
              :class="getAppVisual(d.foreground_app_icon)"
            >
              <!-- 微信图标 -->
              <svg
                v-if="d.foreground_app_icon === 'wechat'"
                class="h-7 w-7 text-[#07C160]"
                viewBox="0 0 24 24"
                fill="currentColor"
              >
                <path d="M8.5 2C4.36 2 1 4.91 1 8.5c0 2.05 1.07 3.88 2.76 5.11L3 17.5l4.28-1.43c.4.08.8.13 1.22.13.23 0 .45-.02.67-.04-.26-.67-.42-1.39-.42-2.16 0-3.59 3.36-6.5 7.5-6.5.64 0 1.26.07 1.84.2C16.84 4.54 13.06 2 8.5 2zm-2.25 4a1.25 1.25 0 110 2.5 1.25 1.25 0 010-2.5zm4.5 0a1.25 1.25 0 110 2.5 1.25 1.25 0 010-2.5zM15.5 8.5c-3.59 0-6.5 2.46-6.5 5.5s2.91 5.5 6.5 5.5c.36 0 .71-.03 1.05-.09L20 20.5l-.9-3.08C20.9 16.38 22 14.8 22 13c0-3.04-2.91-5.5-6.5-5.5zm-2 3.5a1 1 0 110 2 1 1 0 010-2zm4 0a1 1 0 110 2 1 1 0 010-2z" />
              </svg>
              <Settings v-else-if="d.foreground_app_icon === 'settings'" class="h-7 w-7" />
              <LayoutGrid v-else-if="d.foreground_app_icon === 'launcher'" class="h-7 w-7" />
              <Camera v-else-if="d.foreground_app_icon === 'camera'" class="h-7 w-7" />
              <Image v-else-if="d.foreground_app_icon === 'gallery'" class="h-7 w-7" />
              <Phone v-else-if="d.foreground_app_icon === 'phone'" class="h-7 w-7" />
              <Globe v-else-if="d.foreground_app_icon === 'browser'" class="h-7 w-7" />
              <Film v-else-if="d.foreground_app_icon === 'kuaishou' || d.foreground_app_icon === 'douyin'" class="h-7 w-7" />
              <Tv v-else-if="d.foreground_app_icon === 'bilibili' || d.foreground_app_icon === 'iqiyi' || d.foreground_app_icon === 'qqvideo' || d.foreground_app_icon === 'youku'" class="h-7 w-7" />
              <ShoppingBag v-else-if="d.foreground_app_icon === 'alipay' || d.foreground_app_icon === 'taobao' || d.foreground_app_icon === 'jd' || d.foreground_app_icon === 'pinduoduo' || d.foreground_app_icon === 'market'" class="h-7 w-7" />
              <Music v-else-if="d.foreground_app_icon === 'neteasemusic' || d.foreground_app_icon === 'qqmusic'" class="h-7 w-7" />
              <MapPin v-else-if="d.foreground_app_icon === 'amap' || d.foreground_app_icon === 'baidumap'" class="h-7 w-7" />
              <Layers v-else class="h-7 w-7" />
            </div>

            <!-- 运行应用文字信息 -->
            <div class="mt-3 w-full px-2">
              <h4 class="truncate text-sm font-medium tracking-tight text-neutral-100 group-hover/card:text-white transition-colors">
                {{ d.foreground_app_name || '系统待机' }}
              </h4>
              <p v-if="d.foreground_pkg" class="truncate font-mono text-[10px] text-neutral-500 mt-0.5">
                {{ d.foreground_pkg }}
              </p>
            </div>
          </div>

          <!-- 场景 3：设备离线/连接中 -->
          <div
            v-else
            class="relative my-auto flex flex-col items-center justify-center py-4 text-center"
          >
            <div class="relative flex items-center justify-center rounded-2xl h-14 w-14 bg-neutral-900/60 border border-neutral-800 text-neutral-600">
              <Smartphone class="h-7 w-7" />
            </div>
            <div class="mt-3 w-full px-2">
              <h4 class="text-sm font-medium text-neutral-500">
                设备离线
              </h4>
            </div>
          </div>

          <!-- 底部微型型号/序列号与手势条 -->
          <div class="shrink-0 pt-2 pb-0.5 flex flex-col gap-1 border-t border-neutral-800/60">
            <p class="truncate font-mono text-[9px] text-neutral-500 text-center">
              {{ d.model || 'Android' }} · {{ d.serial }}
            </p>
            <div class="mx-auto rounded-full bg-neutral-800 transition-colors group-hover/card:bg-neutral-600 h-0.5 w-10 mt-0.5" />
          </div>
        </div>

        <!-- 模式二：实时投屏流模式 (Phone View) -->
        <div
          v-else
          class="relative w-full flex-1 min-h-0 bg-black overflow-hidden flex items-center justify-center"
        >
          <DashTile
            v-if="isDeviceOnlineForStream(d, mockEnabled)"
            :device="d"
            mode="contain"
            class="w-full h-full"
          />
          <div v-else class="flex flex-col items-center justify-center p-6 text-center space-y-2">
            <Smartphone class="h-8 w-8 text-amber-500/60" />
            <span class="text-xs text-amber-400 font-medium">
              {{ mockEnabled ? 'Mock 模拟设备' : d.state }}
            </span>
            <p class="text-[10px] text-slate-500">
              {{ mockEnabled ? '已排除模拟设备，不向服务端发起真机连接' : '设备离线，未订阅实时投屏' }}
            </p>
          </div>
        </div>
      </div>
    </div>
  </div>
</template>
