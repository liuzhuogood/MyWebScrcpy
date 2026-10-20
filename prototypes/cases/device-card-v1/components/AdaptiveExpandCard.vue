<script setup lang="ts">
import { computed, nextTick, ref } from 'vue'
import {
  Battery,
  BatteryCharging,
  Check,
  ChevronDown,
  ChevronUp,
  Layers,
  MonitorPlay,
  Pencil,
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
}>()

const isExpanded = ref(false)
const isEditing = ref(false)
const draft = ref('')
const inputRef = ref<HTMLInputElement | null>(null)

// 品牌名优先
const displayName = computed(() => {
  if (props.device.alias?.trim()) return props.device.alias.trim()
  if (props.device.telemetry.brand?.trim()) return props.device.telemetry.brand.trim()
  return props.device.telemetry.model || 'Android'
})

const isOnline = computed(() => props.device.state === 'device')
const isManual = computed(() => props.device.source === 'manual')

// 电量状态变色：充电中翡翠绿；放电模式下 <=10% 红色告警，<=20% 暖黄预警，>20% 正常
const batteryLevelStyle = computed(() => {
  if (!isOnline.value) return { text: 'text-slate-400', icon: 'text-slate-400' }
  if (props.device.telemetry.batteryCharging) {
    return {
      text: 'text-emerald-600 dark:text-emerald-400 font-semibold',
      icon: 'text-emerald-500',
    }
  }
  const lvl = props.device.telemetry.batteryLevel
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
  const t = props.device.telemetry.batteryTempCelsius
  if (!isOnline.value || !t) return { text: 'text-slate-400', icon: 'text-slate-400', bg: 'bg-slate-100 dark:bg-white/5' }
  if (t < 38) {
    return {
      text: 'text-emerald-600 dark:text-emerald-400',
      icon: 'text-emerald-500',
      bg: 'bg-emerald-500/10 border-emerald-500/20',
    }
  }
  if (t < 42) {
    return {
      text: 'text-amber-600 dark:text-amber-400 font-semibold',
      icon: 'text-amber-500',
      bg: 'bg-amber-500/10 border-amber-500/20',
    }
  }
  return {
    text: 'text-rose-600 dark:text-rose-400 font-bold animate-pulse',
    icon: 'text-rose-500',
    bg: 'bg-rose-500/10 border-rose-500/30',
  }
})

// CPU 温度状态变色：< 48°C 常温，48~58°C 温升，>= 58°C 告警
const cpuTempStyle = computed(() => {
  const t = props.device.telemetry.cpuTempCelsius
  if (!isOnline.value || !props.device.telemetry.cpuTempSupported || t === undefined) {
    return { text: 'text-slate-400' }
  }
  if (t < 48) return { text: 'text-emerald-600 dark:text-emerald-400' }
  if (t < 58) return { text: 'text-amber-600 dark:text-amber-400 font-semibold' }
  return { text: 'text-rose-600 dark:text-rose-400 font-bold' }
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
        ? 'border-slate-200/90 bg-white/90 text-slate-900 dark:border-white/10 dark:bg-slate-900/70 dark:text-white'
        : 'border-slate-200/70 bg-white/60 text-slate-600 opacity-80 dark:border-white/5 dark:bg-slate-900/30 dark:text-slate-400',
    ]"
  >
    <!-- 主体区域 -->
    <div class="p-4 pb-3">
      <!-- 顶栏状态 -->
      <div class="flex items-center justify-between">
        <div class="flex items-center gap-2">
          <span
            class="inline-flex items-center gap-1.5 rounded-full px-2 py-0.5 text-[11px] font-medium"
            :class="isOnline ? 'bg-emerald-500/10 text-emerald-700 dark:text-emerald-300 border border-emerald-500/20' : 'bg-slate-200/60 text-slate-600 dark:bg-white/5 dark:text-slate-400'"
          >
            <span class="h-1.5 w-1.5 rounded-full" :class="isOnline ? 'bg-emerald-500 animate-pulse' : 'bg-slate-400'" />
            {{ isOnline ? '在线' : '离线' }}
          </span>

          <span class="rounded bg-slate-100 dark:bg-white/10 px-1.5 py-0.5 text-[10px] font-mono text-slate-600 dark:text-slate-300">
            Android {{ device.telemetry.androidRelease }}
          </span>
        </div>

        <div class="flex items-center gap-2">
          <div class="flex items-center gap-1 text-[11px] text-slate-500 dark:text-slate-400">
            <Usb v-if="!isManual" class="h-3.5 w-3.5 text-cyan-600 dark:text-cyan-400" />
            <Wifi v-else class="h-3.5 w-3.5 text-violet-600 dark:text-violet-400" />
            <span>{{ !isManual ? 'USB' : 'Wi-Fi' }}</span>
          </div>

          <!-- 展开详情切换 -->
          <Button
            variant="ghost"
            size="sm"
            class="h-7 px-2 text-[11px] text-slate-500 hover:text-slate-900 dark:text-slate-400 dark:hover:text-white cursor-pointer"
            @click="isExpanded = !isExpanded"
          >
            <span class="mr-1">{{ isExpanded ? '收起' : '详情' }}</span>
            <ChevronUp v-if="isExpanded" class="h-3.5 w-3.5" />
            <ChevronDown v-else class="h-3.5 w-3.5" />
          </Button>
        </div>
      </div>

      <!-- 设备标题（品牌名优先） -->
      <div class="mt-3">
        <div v-if="!isEditing" class="flex items-center justify-between gap-2">
          <div class="min-w-0 flex-1">
            <h3 class="truncate text-base font-bold tracking-tight text-slate-900 dark:text-white">
              {{ displayName }}
            </h3>
            <p class="mt-0.5 truncate text-[11px] text-slate-500 dark:text-slate-400 font-mono">
              {{ device.telemetry.model }} · {{ device.serial }}
            </p>
          </div>
          <Button
            variant="ghost"
            size="icon"
            class="h-7 w-7 rounded-lg text-slate-400 hover:text-slate-900 dark:hover:text-white cursor-pointer shrink-0"
            title="重命名设备"
            @click.stop="startEdit"
          >
            <Pencil class="h-3.5 w-3.5" />
          </Button>
        </div>

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
          <Button size="sm" class="h-8 rounded-lg bg-[#406BDB] text-white px-2.5" @click="saveEdit">
            <Check class="h-3.5 w-3.5" />
          </Button>
          <Button size="sm" variant="ghost" class="h-8 rounded-lg text-slate-500 px-2" @click="cancelEdit">
            <X class="h-3.5 w-3.5" />
          </Button>
        </div>
      </div>

      <!-- 核心指标摘要（电量、电池温度、前台） -->
      <div class="mt-3 flex items-center justify-between rounded-xl bg-slate-50 dark:bg-white/[0.03] p-2.5 border border-slate-200/60 dark:border-white/5">
        <div class="flex items-center gap-3">
          <!-- 电量指示（分级变色） -->
          <div class="flex items-center gap-1 text-xs font-mono" :class="batteryLevelStyle.text">
            <BatteryCharging v-if="device.telemetry.batteryCharging" class="h-3.5 w-3.5" :class="batteryLevelStyle.icon" />
            <Battery v-else class="h-3.5 w-3.5" :class="batteryLevelStyle.icon" />
            {{ isOnline ? `${device.telemetry.batteryLevel}%` : '--' }}
          </div>

          <!-- 电池温度（分级变色） -->
          <div class="flex items-center gap-1 text-xs font-mono" :class="batteryTempStyle.text">
            <Thermometer class="h-3.5 w-3.5" :class="batteryTempStyle.icon" />
            {{ isOnline && device.telemetry.batteryTempCelsius ? `${device.telemetry.batteryTempCelsius.toFixed(1)}°C` : '--' }}
          </div>
        </div>

        <div class="flex items-center gap-1.5 text-xs text-slate-600 dark:text-slate-300 min-w-0 max-w-[140px] truncate">
          <Layers class="h-3 w-3 text-blue-500 shrink-0" />
          <span class="truncate text-[11px]">{{ isOnline ? device.telemetry.currentApp : '离线' }}</span>
        </div>
      </div>

      <!-- 展开的 ADB 完整硬件明细抽屉面板 -->
      <div
        v-if="isExpanded"
        class="mt-3 rounded-xl border border-blue-500/20 bg-blue-50/50 p-3 text-xs dark:border-blue-500/20 dark:bg-blue-950/20 space-y-2 animate-in fade-in slide-in-from-top-2 duration-150"
      >
        <div class="text-[11px] font-semibold text-blue-700 dark:text-blue-300 flex items-center justify-between">
          <span>ADB 硬件与系统明细</span>
          <span class="font-mono text-[10px] text-slate-400">ro.* & dumpsys</span>
        </div>

        <div class="grid grid-cols-2 gap-x-3 gap-y-1.5 text-[11px] text-slate-600 dark:text-slate-300">
          <div><span class="text-slate-400">品牌:</span> {{ device.telemetry.brand }}</div>
          <div><span class="text-slate-400">厂商:</span> {{ device.telemetry.manufacturer }}</div>
          <div><span class="text-slate-400">型号:</span> {{ device.telemetry.model.split(' ')[0] }}</div>
          <div><span class="text-slate-400">系统:</span> Android {{ device.telemetry.androidRelease }} (API {{ device.telemetry.sdkVersion }})</div>
          <div><span class="text-slate-400">CPU ABI:</span> {{ device.telemetry.cpuAbi }}</div>
          <div>
            <span class="text-slate-400">CPU 温度:</span>
            <span
              v-if="device.telemetry.cpuTempSupported && device.telemetry.cpuTempCelsius"
              class="font-mono ml-1"
              :class="cpuTempStyle.text"
            >
              {{ device.telemetry.cpuTempCelsius.toFixed(1) }}°C
            </span>
            <span v-else class="text-slate-400 ml-1">未开放</span>
          </div>
          <div><span class="text-slate-400">分辨率:</span> {{ device.telemetry.resolution }}</div>
          <div><span class="text-slate-400">内存:</span> {{ device.telemetry.ramUsedGb }}/{{ device.telemetry.ramTotalGb }} GB</div>
        </div>

        <div v-if="device.telemetry.buildDisplayId" class="pt-1 border-t border-blue-200/50 dark:border-white/5 text-[10px] text-slate-400 truncate font-mono">
          构建 ID: {{ device.telemetry.buildDisplayId }}
        </div>
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
      </div>
    </div>
  </div>
</template>
