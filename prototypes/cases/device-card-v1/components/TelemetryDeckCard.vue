<script setup lang="ts">
import { computed, nextTick, ref } from 'vue'
import {
  Activity,
  Battery,
  BatteryCharging,
  Check,
  Cpu,
  Layers,
  MonitorPlay,
  Pencil,
  RefreshCw,
  Thermometer,
  Tv,
  Usb,
  Wifi,
  X,
} from 'lucide-vue-next'
import { Button } from '@/components/ui/button'
import { Input } from '@/components/ui/input'
import type { PrototypeDevice } from '../mockDevices'

const props = defineProps<{
  device: PrototypeDevice
}>()

const emit = defineEmits<{
  (e: 'play', serial: string): void
  (e: 'pip', serial: string): void
  (e: 'updateAlias', serial: string, alias: string): void
  (e: 'reconnect', serial: string): void
}>()

const isEditing = ref(false)
const draft = ref('')
const inputRef = ref<HTMLInputElement | null>(null)

// 设备展示名称逻辑：用户强调“设备品名称，默认为品牌名”
// 优先级：用户自定义别名 > 设备品牌名(ro.product.brand) > 型号 > 默认标识
const displayName = computed(() => {
  if (props.device.alias?.trim()) return props.device.alias.trim()
  if (props.device.telemetry.brand?.trim()) return props.device.telemetry.brand.trim()
  return props.device.telemetry.model || 'Android'
})

const isOnline = computed(() => props.device.state === 'device')
const isManual = computed(() => props.device.source === 'manual')

// 温度健康度色彩映射
const batteryTempStatus = computed(() => {
  const t = props.device.telemetry.batteryTempCelsius
  if (!isOnline.value || !t) return { color: 'text-slate-400', badge: 'bg-slate-500/10 text-slate-500 border-slate-500/20', text: '离线' }
  if (t < 38) return { color: 'text-emerald-500 dark:text-emerald-400', badge: 'bg-emerald-500/10 text-emerald-600 dark:text-emerald-400 border-emerald-500/20', text: '良好' }
  if (t < 42) return { color: 'text-amber-500 dark:text-amber-400', badge: 'bg-amber-500/10 text-amber-600 dark:text-amber-400 border-amber-500/20', text: '温热' }
  return { color: 'text-rose-500 dark:text-rose-400', badge: 'bg-rose-500/10 text-rose-600 dark:text-rose-400 border-rose-500/20', text: '过热' }
})

const cpuTempStatus = computed(() => {
  const t = props.device.telemetry.cpuTempCelsius
  if (!isOnline.value || !props.device.telemetry.cpuTempSupported || t === undefined) {
    return { color: 'text-slate-400', badge: 'bg-slate-500/10 text-slate-500 border-slate-500/20', text: '未开放' }
  }
  if (t < 45) return { color: 'text-emerald-500 dark:text-emerald-400', badge: 'bg-emerald-500/10 text-emerald-600 dark:text-emerald-400 border-emerald-500/20', text: '常温' }
  if (t < 55) return { color: 'text-amber-500 dark:text-amber-400', badge: 'bg-amber-500/10 text-amber-600 dark:text-amber-400 border-amber-500/20', text: '较高' }
  return { color: 'text-rose-500 dark:text-rose-400', badge: 'bg-rose-500/10 text-rose-600 dark:text-rose-400 border-rose-500/20', text: '警戒' }
})

function startEdit() {
  draft.value = props.device.alias || props.device.telemetry.brand || ''
  isEditing.value = true
  nextTick(() => {
    inputRef.value?.focus()
    inputRef.value?.select()
  })
}

function saveEdit() {
  emit('updateAlias', props.device.serial, draft.value)
  isEditing.value = false
}

function cancelEdit() {
  isEditing.value = false
}
</script>

<template>
  <div
    class="relative flex flex-col justify-between overflow-hidden rounded-2xl border transition-all duration-200 select-none shadow-xs"
    :class="[
      isOnline
        ? 'border-slate-200/90 bg-white/90 text-slate-900 shadow-sm hover:border-blue-500/40 hover:shadow-md dark:border-white/10 dark:bg-slate-900/70 dark:text-white dark:hover:border-blue-400/30'
        : 'border-slate-200/70 bg-white/60 text-slate-600 opacity-80 dark:border-white/5 dark:bg-slate-900/30 dark:text-slate-400',
    ]"
  >
    <!-- 顶栏状态与来源 -->
    <div class="p-4 pb-3 border-b border-slate-100 dark:border-white/5">
      <div class="flex items-center justify-between gap-2">
        <!-- 在线状态与系统架构徽标 -->
        <div class="flex items-center gap-1.5 flex-wrap">
          <span
            class="inline-flex items-center gap-1.5 rounded-full px-2 py-0.5 text-[11px] font-medium"
            :class="[
              isOnline
                ? 'bg-emerald-500/10 text-emerald-700 dark:text-emerald-300 border border-emerald-500/20'
                : 'bg-slate-200/60 text-slate-600 dark:bg-white/5 dark:text-slate-400 border border-slate-300/40 dark:border-white/10',
            ]"
          >
            <span
              class="h-1.5 w-1.5 rounded-full"
              :class="isOnline ? 'bg-emerald-500 animate-pulse' : 'bg-slate-400'"
            />
            {{ isOnline ? '在线' : '离线' }}
          </span>

          <!-- Android 版本 & SDK 胶囊 -->
          <span class="inline-flex items-center rounded-md bg-blue-500/10 text-blue-700 dark:text-blue-300 border border-blue-500/20 px-1.5 py-0.5 text-[10px] font-medium font-mono">
            Android {{ device.telemetry.androidRelease }} (API {{ device.telemetry.sdkVersion }})
          </span>

          <!-- CPU ABI 架构 -->
          <span class="inline-flex items-center rounded-md bg-slate-100 text-slate-600 dark:bg-white/5 dark:text-slate-400 border border-slate-200/60 dark:border-white/10 px-1.5 py-0.5 text-[10px] font-mono">
            {{ device.telemetry.cpuAbi }}
          </span>
        </div>

        <!-- 连接介质 -->
        <div class="flex items-center gap-1 text-[11px] text-slate-500 dark:text-slate-400 shrink-0 font-medium">
          <Usb v-if="!isManual" class="h-3.5 w-3.5 text-cyan-600 dark:text-cyan-400" />
          <Wifi v-else class="h-3.5 w-3.5 text-violet-600 dark:text-violet-400" />
          <span>{{ !isManual ? 'USB' : 'Wi-Fi' }}</span>
        </div>
      </div>

      <!-- 设备标题（品牌名优先）与型号副标 -->
      <div class="mt-3">
        <div v-if="!isEditing" class="flex items-center justify-between gap-2">
          <div class="min-w-0 flex-1">
            <div class="flex items-center gap-2">
              <h3 class="truncate text-base font-bold tracking-tight text-slate-900 dark:text-white">
                {{ displayName }}
              </h3>
              <!-- 标识是否采用了品牌名默认命名 -->
              <span
                v-if="!device.alias"
                class="rounded bg-slate-100 px-1.5 py-0.5 text-[10px] font-normal text-slate-500 dark:bg-white/10 dark:text-slate-400 shrink-0"
              >
                品牌默认
              </span>
            </div>
            <p class="mt-0.5 truncate text-[11px] text-slate-500 dark:text-slate-400 font-mono">
              {{ device.telemetry.model }} · {{ device.serial }}
            </p>
          </div>

          <Button
            variant="ghost"
            size="icon"
            class="h-7 w-7 rounded-lg text-slate-400 hover:text-slate-900 dark:hover:text-white shrink-0 cursor-pointer"
            title="重命名设备"
            @click.stop="startEdit"
          >
            <Pencil class="h-3.5 w-3.5" />
          </Button>
        </div>

        <!-- 重命名编辑模式 -->
        <div v-else class="flex items-center gap-1.5 py-0.5" @click.stop>
          <Input
            ref="inputRef"
            v-model="draft"
            maxlength="20"
            :placeholder="device.telemetry.brand || '设备名称'"
            class="h-8 rounded-lg text-xs"
            @keydown.enter.prevent="saveEdit"
            @keydown.esc.prevent="cancelEdit"
          />
          <Button size="sm" class="h-8 rounded-lg bg-[#406BDB] hover:bg-[#3258be] text-white px-2.5" @click="saveEdit">
            <Check class="h-3.5 w-3.5" />
          </Button>
          <Button size="sm" variant="ghost" class="h-8 rounded-lg text-slate-500 px-2" @click="cancelEdit">
            <X class="h-3.5 w-3.5" />
          </Button>
        </div>
      </div>
    </div>

    <!-- 实时遥测核心指标区 (电池电量、电池温度、CPU 温度、负载) -->
    <div class="p-4 py-3 space-y-2.5">
      <!-- 双温感与电量栅格 -->
      <div class="grid grid-cols-2 gap-2">
        <!-- 电池状态与温度 -->
        <div class="rounded-xl border border-slate-200/70 bg-slate-50/70 p-2.5 dark:border-white/5 dark:bg-white/[0.03]">
          <div class="flex items-center justify-between text-[11px] text-slate-500 dark:text-slate-400">
            <span class="flex items-center gap-1">
              <BatteryCharging v-if="device.telemetry.batteryCharging" class="h-3.5 w-3.5 text-emerald-500" />
              <Battery v-else class="h-3.5 w-3.5" />
              电池
            </span>
            <span class="font-mono font-semibold" :class="device.telemetry.batteryCharging ? 'text-emerald-600 dark:text-emerald-400' : 'text-slate-700 dark:text-slate-300'">
              {{ isOnline ? `${device.telemetry.batteryLevel}%` : '--' }}
            </span>
          </div>

          <!-- 电量进度指示微条 -->
          <div class="mt-1.5 h-1.5 w-full overflow-hidden rounded-full bg-slate-200/80 dark:bg-white/10">
            <div
              class="h-full rounded-full transition-all duration-300"
              :class="device.telemetry.batteryCharging ? 'bg-emerald-500' : 'bg-[#406BDB]'"
              :style="{ width: `${isOnline ? device.telemetry.batteryLevel : 0}%` }"
            />
          </div>

          <!-- 电池温度 -->
          <div class="mt-2 flex items-center justify-between text-[10px]">
            <span class="text-slate-500 dark:text-slate-400 flex items-center gap-0.5">
              <Thermometer class="h-3 w-3" />
              电池温度
            </span>
            <span class="font-mono font-medium" :class="batteryTempStatus.color">
              {{ isOnline && device.telemetry.batteryTempCelsius ? `${device.telemetry.batteryTempCelsius.toFixed(1)}°C` : 'N/A' }}
            </span>
          </div>
        </div>

        <!-- CPU 状态与温度 -->
        <div class="rounded-xl border border-slate-200/70 bg-slate-50/70 p-2.5 dark:border-white/5 dark:bg-white/[0.03]">
          <div class="flex items-center justify-between text-[11px] text-slate-500 dark:text-slate-400">
            <span class="flex items-center gap-1">
              <Cpu class="h-3.5 w-3.5 text-blue-500" />
              CPU 温度
            </span>
            <span
              v-if="device.telemetry.cpuTempSupported && device.telemetry.cpuTempCelsius"
              class="font-mono font-semibold"
              :class="cpuTempStatus.color"
            >
              {{ isOnline ? `${device.telemetry.cpuTempCelsius.toFixed(1)}°C` : '--' }}
            </span>
            <span
              v-else
              class="text-[10px] font-normal text-slate-400"
              title="设备未开放 thermal zone 或无需 root 接口"
            >
              未开放
            </span>
          </div>

          <!-- 负载占用微条 -->
          <div class="mt-1.5 h-1.5 w-full overflow-hidden rounded-full bg-slate-200/80 dark:bg-white/10">
            <div
              class="h-full rounded-full bg-blue-500 transition-all duration-300"
              :style="{ width: `${isOnline ? device.telemetry.cpuUsagePercent : 0}%` }"
            />
          </div>

          <!-- CPU 负载率与内存 -->
          <div class="mt-2 flex items-center justify-between text-[10px]">
            <span class="text-slate-500 dark:text-slate-400 flex items-center gap-0.5">
              <Activity class="h-3 w-3" />
              CPU负载
            </span>
            <span class="font-mono font-medium text-slate-700 dark:text-slate-300">
              {{ isOnline ? `${device.telemetry.cpuUsagePercent}%` : '--' }}
            </span>
          </div>
        </div>
      </div>

      <!-- 前台活跃应用小卡片 -->
      <div class="flex items-center justify-between rounded-xl border border-slate-200/60 bg-white/60 p-2 text-xs dark:border-white/5 dark:bg-white/[0.02]">
        <div class="flex items-center gap-2 min-w-0 flex-1">
          <Layers class="h-3.5 w-3.5 text-blue-500 shrink-0" />
          <div class="min-w-0 flex-1">
            <p class="truncate text-[11px] font-medium text-slate-800 dark:text-slate-200">
              {{ isOnline ? device.telemetry.currentApp : '设备待机中' }}
            </p>
            <p class="truncate text-[9px] font-mono text-slate-400">
              {{ isOnline ? device.telemetry.currentPackage : '未运行前台前任务' }}
            </p>
          </div>
        </div>
        <span class="rounded bg-slate-100 dark:bg-white/5 px-1.5 py-0.5 text-[9px] font-mono text-slate-500 shrink-0">
          {{ device.telemetry.resolution }}
        </span>
      </div>
    </div>

    <!-- 底部操作条 -->
    <div class="p-3 border-t border-slate-100 dark:border-white/5 bg-slate-50/50 dark:bg-white/[0.01]">
      <div class="flex items-center gap-2">
        <Button
          size="sm"
          :disabled="!isOnline"
          class="flex-1 rounded-xl bg-[#406BDB] hover:bg-[#3258be] font-medium text-white shadow-xs text-xs cursor-pointer"
          @click="emit('play', device.serial)"
        >
          <MonitorPlay class="mr-1.5 h-3.5 w-3.5" />
          设备投屏
        </Button>

        <Button
          size="sm"
          variant="outline"
          :disabled="!isOnline"
          class="h-8 w-8 rounded-xl p-0 cursor-pointer text-slate-700 dark:text-slate-200"
          title="画中画微窗"
          @click="emit('pip', device.serial)"
        >
          <Tv class="h-3.5 w-3.5" />
        </Button>

        <Button
          v-if="!isOnline && isManual"
          size="sm"
          variant="outline"
          class="h-8 w-8 rounded-xl p-0 cursor-pointer"
          title="重连"
          @click="emit('reconnect', device.serial)"
        >
          <RefreshCw class="h-3.5 w-3.5" />
        </Button>
      </div>
    </div>
  </div>
</template>
