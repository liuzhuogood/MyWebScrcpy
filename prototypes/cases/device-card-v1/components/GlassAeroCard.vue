<script setup lang="ts">
import { computed, nextTick, ref } from 'vue'
import {
  BatteryCharging,
  Check,
  Cpu,
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

const cardRef = ref<HTMLDivElement | null>(null)
const rotateX = ref(0)
const rotateY = ref(0)
const glowX = ref(50)
const glowY = ref(50)
const isHovering = ref(false)

const isEditing = ref(false)
const draft = ref('')
const inputRef = ref<HTMLInputElement | null>(null)

// 品牌名称优先展示
const displayName = computed(() => {
  if (props.device.alias?.trim()) return props.device.alias.trim()
  if (props.device.telemetry.brand?.trim()) return props.device.telemetry.brand.trim()
  return props.device.telemetry.model || 'Android'
})

const isOnline = computed(() => props.device.state === 'device')
const isManual = computed(() => props.device.source === 'manual')

function handleMouseMove(e: MouseEvent) {
  if (!cardRef.value) return
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
  <div class="perspective-1000" style="perspective: 1000px;">
    <div
      ref="cardRef"
      class="group relative flex flex-col justify-between overflow-hidden rounded-3xl border p-5 transition-all duration-200 ease-out will-change-transform select-none"
      :class="[
        isOnline
          ? 'border-slate-200/90 bg-white/85 shadow-sm backdrop-blur-xl hover:border-blue-500/50 hover:bg-white/95 hover:shadow-lg text-slate-900 dark:border-white/10 dark:bg-slate-900/60 dark:shadow-md dark:backdrop-blur-2xl dark:hover:border-blue-400/40 dark:text-white'
          : 'border-slate-200/70 bg-white/60 text-slate-600 opacity-75 dark:border-white/5 dark:bg-slate-900/30 dark:text-slate-300',
      ]"
      :style="{
        transform: `rotateX(${rotateX}deg) rotateY(${rotateY}deg) ${isHovering ? 'scale3d(1.015, 1.015, 1.015)' : 'scale3d(1, 1, 1)'}`,
      }"
      @mousemove="handleMouseMove"
      @mouseleave="handleMouseLeave"
    >
      <!-- 动态环境高光 -->
      <div
        class="pointer-events-none absolute inset-0 opacity-0 transition-opacity duration-300 group-hover:opacity-100"
        :style="{
          background: `radial-gradient(circle at ${glowX}% ${glowY}%, rgba(64, 107, 219, 0.12) 0%, rgba(59, 130, 246, 0.05) 40%, transparent 70%)`,
        }"
      />

      <!-- 顶栏状态与硬件品牌 -->
      <div class="relative z-10 flex items-center justify-between">
        <div class="flex items-center gap-2">
          <!-- 状态徽章 -->
          <span
            class="inline-flex items-center gap-1.5 rounded-full px-2.5 py-1 text-[11px] font-semibold"
            :class="isOnline ? 'bg-emerald-500/15 text-emerald-700 dark:text-emerald-300 border border-emerald-500/30' : 'bg-slate-200/70 text-slate-600 dark:bg-white/5 dark:text-slate-400'"
          >
            <span class="h-1.5 w-1.5 rounded-full" :class="isOnline ? 'bg-emerald-500 animate-pulse' : 'bg-slate-400'" />
            {{ isOnline ? '在线' : '离线' }}
          </span>

          <!-- Android 版本胶囊 -->
          <span class="rounded-full bg-slate-100 px-2.5 py-0.5 text-[10px] font-mono text-slate-700 border border-slate-200/80 dark:bg-white/5 dark:text-slate-300 dark:border-white/10">
            Android {{ device.telemetry.androidRelease }}
          </span>
        </div>

        <div class="flex items-center gap-1.5 text-xs text-slate-500 dark:text-slate-400">
          <Usb v-if="!isManual" class="h-3.5 w-3.5 text-cyan-600 dark:text-cyan-400" />
          <Wifi v-else class="h-3.5 w-3.5 text-violet-600 dark:text-violet-400" />
          <span class="text-[11px] font-medium">{{ !isManual ? 'USB' : 'Wi-Fi' }}</span>
        </div>
      </div>

      <!-- 核心名称：默认为品牌名 -->
      <div class="relative z-10 mt-4">
        <div v-if="!isEditing" class="flex items-center justify-between gap-2">
          <div class="min-w-0 flex-1">
            <h3 class="truncate text-lg font-bold tracking-tight text-slate-900 dark:text-white group-hover:text-blue-600 dark:group-hover:text-blue-400 transition-colors">
              {{ displayName }}
            </h3>
            <p class="mt-0.5 truncate text-xs text-slate-500 dark:text-slate-400 font-mono">
              {{ device.telemetry.model }} · {{ device.serial }}
            </p>
          </div>
          <Button
            variant="ghost"
            size="icon"
            class="h-7 w-7 rounded-lg text-slate-400 hover:text-slate-900 dark:hover:text-white opacity-0 group-hover:opacity-100 transition-opacity cursor-pointer shrink-0"
            title="重命名设备"
            @click.stop="startEdit"
          >
            <Pencil class="h-3.5 w-3.5" />
          </Button>
        </div>

        <!-- 重命名输入框 -->
        <div v-else class="flex items-center gap-1.5 py-1" @click.stop>
          <Input
            ref="inputRef"
            v-model="draft"
            maxlength="20"
            :placeholder="device.telemetry.brand || '设备名称'"
            class="h-8 rounded-xl text-xs"
            @keydown.enter.prevent="saveEdit"
            @keydown.esc.prevent="cancelEdit"
          />
          <Button size="sm" class="h-8 rounded-xl bg-[#406BDB] hover:bg-[#3258be] text-white px-2.5" @click="saveEdit">
            <Check class="h-3.5 w-3.5" />
          </Button>
          <Button size="sm" variant="ghost" class="h-8 rounded-xl text-slate-500 px-2" @click="cancelEdit">
            <X class="h-3.5 w-3.5" />
          </Button>
        </div>

        <!-- 3 组精炼指标胶囊栏（电池与温升、CPU与架构、当前前台） -->
        <div class="mt-3.5 grid grid-cols-3 gap-2">
          <!-- 电池指标 -->
          <div class="rounded-xl border border-slate-200/80 bg-slate-50/70 p-2 dark:border-white/5 dark:bg-white/[0.03]">
            <div class="flex items-center gap-1 text-[10px] text-slate-500 dark:text-slate-400">
              <BatteryCharging v-if="device.telemetry.batteryCharging" class="h-3 w-3 text-emerald-500" />
              <Thermometer v-else class="h-3 w-3 text-blue-500" />
              电池 / 电量
            </div>
            <div class="mt-1 flex items-baseline justify-between">
              <span class="text-xs font-bold font-mono text-slate-800 dark:text-slate-200">
                {{ isOnline ? `${device.telemetry.batteryLevel}%` : '--' }}
              </span>
              <span class="text-[10px] font-mono text-emerald-600 dark:text-emerald-400">
                {{ isOnline && device.telemetry.batteryTempCelsius ? `${device.telemetry.batteryTempCelsius.toFixed(1)}°` : '--' }}
              </span>
            </div>
          </div>

          <!-- CPU 指标 -->
          <div class="rounded-xl border border-slate-200/80 bg-slate-50/70 p-2 dark:border-white/5 dark:bg-white/[0.03]">
            <div class="flex items-center gap-1 text-[10px] text-slate-500 dark:text-slate-400">
              <Cpu class="h-3 w-3 text-violet-500" />
              CPU / 温度
            </div>
            <div class="mt-1 flex items-baseline justify-between">
              <span class="text-xs font-bold font-mono text-slate-800 dark:text-slate-200">
                {{ device.telemetry.cpuAbi.replace('-v8a', '') }}
              </span>
              <span class="text-[10px] font-mono text-slate-600 dark:text-slate-300">
                {{ isOnline && device.telemetry.cpuTempSupported && device.telemetry.cpuTempCelsius ? `${device.telemetry.cpuTempCelsius.toFixed(0)}°C` : '未开放' }}
              </span>
            </div>
          </div>

          <!-- 屏幕与架构 -->
          <div class="rounded-xl border border-slate-200/80 bg-slate-50/70 p-2 dark:border-white/5 dark:bg-white/[0.03]">
            <div class="flex items-center gap-1 text-[10px] text-slate-500 dark:text-slate-400">
              API {{ device.telemetry.sdkVersion }}
            </div>
            <div class="mt-1 truncate text-xs font-bold font-mono text-slate-800 dark:text-slate-200">
              {{ device.telemetry.resolution.split(' ')[0] }}P
            </div>
          </div>
        </div>

        <!-- 前台活跃应用小卡片 -->
        <div class="mt-3 flex items-center justify-between rounded-xl border border-slate-200/60 bg-white/70 p-2.5 text-xs dark:border-white/5 dark:bg-white/[0.02]">
          <div class="flex items-center gap-2 min-w-0 flex-1">
            <Layers class="h-3.5 w-3.5 text-blue-500 shrink-0" />
            <div class="min-w-0 flex-1">
              <span class="truncate block text-xs font-semibold text-slate-900 dark:text-white">
                {{ isOnline ? device.telemetry.currentApp : '设备离线休眠' }}
              </span>
              <span class="truncate block text-[10px] font-mono text-slate-400">
                {{ isOnline ? device.telemetry.currentPackage : '无活跃任务' }}
              </span>
            </div>
          </div>
          <span class="rounded bg-blue-500/10 px-1.5 py-0.5 text-[9px] font-medium text-blue-600 dark:text-blue-400 shrink-0">
            前台
          </span>
        </div>
      </div>

      <!-- 底部操作条 -->
      <div class="relative z-10 mt-4 border-t border-slate-200/80 dark:border-white/10 pt-3">
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
  </div>
</template>
