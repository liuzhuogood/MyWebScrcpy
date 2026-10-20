<script setup lang="ts">
import { computed, ref } from 'vue'
import {
  Battery,
  BatteryCharging,
  Info,
  MonitorPlay,
  Sparkles,
  Terminal,
  Tv,
  Usb,
  Wifi,
} from 'lucide-vue-next'
import { Button } from '@/components/ui/button'
import type { SpatialDevice } from '../mockDevices'

const props = defineProps<{
  device: SpatialDevice
  index: number
}>()

const emit = defineEmits<{
  (e: 'play', serial: string): void
  (e: 'pip', serial: string): void
  (e: 'inspect', device: SpatialDevice): void
  (e: 'terminal', serial: string): void
  (e: 'reconnect', serial: string): void
}>()

const cardRef = ref<HTMLDivElement | null>(null)
const rotateX = ref(0)
const rotateY = ref(0)
const glowX = ref(50)
const glowY = ref(50)
const isHovering = ref(false)

const isOnline = computed(() => props.device.state === 'device')
const isUnauthorized = computed(() => props.device.state === 'unauthorized')

function handleMouseMove(e: MouseEvent) {
  if (!cardRef.value) return
  const rect = cardRef.value.getBoundingClientRect()
  const x = e.clientX - rect.left
  const y = e.clientY - rect.top
  const centerX = rect.width / 2
  const centerY = rect.height / 2

  // 限制最大旋转 8 度，呈现精致 3D 浮雕感
  rotateX.value = ((y - centerY) / centerY) * -8
  rotateY.value = ((x - centerX) / centerX) * 8

  glowX.value = (x / rect.width) * 100
  glowY.value = (y / rect.height) * 100
  isHovering.value = true
}

function handleMouseLeave() {
  rotateX.value = 0
  rotateY.value = 0
  isHovering.value = false
}
</script>

<template>
  <div
    class="perspective-1000"
    style="perspective: 1000px;"
  >
    <div
      ref="cardRef"
      class="group relative flex flex-col justify-between overflow-hidden rounded-3xl border p-6 transition-transform duration-200 ease-out will-change-transform"
      :class="[
        isOnline
          ? 'border-white/10 bg-white/[0.03] shadow-[0_20px_50px_rgba(0,0,0,0.5)] backdrop-blur-2xl hover:border-violet-400/40 hover:bg-white/[0.06]'
          : isUnauthorized
            ? 'border-amber-500/20 bg-amber-950/10 backdrop-blur-2xl'
            : 'border-white/5 bg-white/[0.01] opacity-70 backdrop-blur-xl hover:opacity-100',
      ]"
      :style="{
        transform: `rotateX(${rotateX}deg) rotateY(${rotateY}deg) ${isHovering ? 'scale3d(1.02, 1.02, 1.02)' : 'scale3d(1, 1, 1)'}`,
      }"
      @mousemove="handleMouseMove"
      @mouseleave="handleMouseLeave"
    >
      <!-- 3D 玻璃动态反光高光层 (Specular Highlight) -->
      <div
        class="pointer-events-none absolute inset-0 opacity-0 transition-opacity duration-300 group-hover:opacity-100"
        :style="{
          background: `radial-gradient(circle at ${glowX}% ${glowY}%, rgba(255, 255, 255, 0.12) 0%, rgba(139, 92, 246, 0.08) 35%, transparent 70%)`,
        }"
      />

      <!-- 顶栏状态与标签 -->
      <div class="relative z-10 flex items-center justify-between">
        <div class="flex items-center gap-2">
          <span
            class="inline-flex items-center gap-1.5 rounded-full px-2.5 py-1 text-[11px] font-medium backdrop-blur-md"
            :class="[
              isOnline
                ? 'bg-emerald-500/15 text-emerald-300 border border-emerald-500/30'
                : isUnauthorized
                  ? 'bg-amber-500/15 text-amber-300 border border-amber-500/30'
                  : 'bg-white/5 text-slate-400 border border-white/10',
            ]"
          >
            <span
              class="h-1.5 w-1.5 rounded-full"
              :class="isOnline ? 'bg-emerald-400 animate-pulse' : isUnauthorized ? 'bg-amber-400' : 'bg-slate-500'"
            />
            {{ isOnline ? '空间在线' : isUnauthorized ? '等待授权' : '离线待机' }}
          </span>

          <span class="rounded-full bg-white/5 px-2.5 py-1 text-[10px] text-slate-300 border border-white/10">
            {{ device.telemetry.resolution }}
          </span>
        </div>

        <div class="flex items-center gap-1.5 text-xs text-slate-400">
          <Usb v-if="device.source === 'auto'" class="h-3.5 w-3.5 text-cyan-400" />
          <Wifi v-else class="h-3.5 w-3.5 text-violet-400" />
          <span class="text-[11px]">{{ device.source === 'auto' ? '极速直连' : '空间无线' }}</span>
        </div>
      </div>

      <!-- 设备核心信息 -->
      <div class="relative z-10 mt-5">
        <h3 class="text-lg font-semibold tracking-tight text-white group-hover:text-violet-200 transition-colors">
          {{ device.name }}
        </h3>
        <p class="mt-0.5 text-xs text-slate-400 font-mono">
          {{ device.model }} · {{ device.serial }}
        </p>

        <!-- 空间全息微缩屏幕展示盒 -->
        <div class="relative mt-4 overflow-hidden rounded-2xl border border-white/10 bg-gradient-to-b from-white/[0.08] to-black/40 p-4 backdrop-blur-md shadow-inner">
          <div class="flex items-center justify-between text-xs">
            <span class="text-slate-400">当前活跃进程</span>
            <span class="font-medium text-violet-300 flex items-center gap-1">
              <Sparkles class="h-3 w-3 text-violet-400 animate-spin" />
              {{ device.telemetry.currentApp }}
            </span>
          </div>

          <!-- 遥测参数横条 -->
          <div class="mt-3.5 grid grid-cols-3 gap-2 border-t border-white/10 pt-3 text-center">
            <div class="rounded-xl bg-white/[0.03] p-1.5">
              <span class="text-[10px] text-slate-400 block">帧率</span>
              <span class="text-xs font-semibold text-cyan-300">{{ device.telemetry.fps }} FPS</span>
            </div>
            <div class="rounded-xl bg-white/[0.03] p-1.5">
              <span class="text-[10px] text-slate-400 block">时延</span>
              <span class="text-xs font-semibold text-emerald-300">{{ device.telemetry.latencyMs }} ms</span>
            </div>
            <div class="rounded-xl bg-white/[0.03] p-1.5">
              <span class="text-[10px] text-slate-400 block">机温</span>
              <span class="text-xs font-semibold text-violet-300">{{ device.telemetry.tempCelsius }}°C</span>
            </div>
          </div>
        </div>

        <!-- 电量指示行 -->
        <div class="mt-3.5 flex items-center justify-between text-xs text-slate-400 px-1">
          <div class="flex items-center gap-1.5">
            <BatteryCharging v-if="device.telemetry.batteryCharging" class="h-4 w-4 text-emerald-400" />
            <Battery v-else class="h-4 w-4 text-slate-400" />
            <span>电量 {{ device.telemetry.batteryPercent }}%</span>
          </div>
          <span class="text-[11px] text-slate-500">{{ device.telemetry.androidVersion }}</span>
        </div>
      </div>

      <!-- 底部悬浮玻璃操作条 -->
      <div class="relative z-10 mt-6 border-t border-white/10 pt-4">
        <div class="flex items-center gap-2">
          <!-- 投屏主按钮 -->
          <Button
            size="sm"
            :disabled="!isOnline"
            class="flex-1 rounded-xl bg-gradient-to-r from-violet-600 via-indigo-600 to-cyan-500 font-medium text-white shadow-[0_0_20px_rgba(139,92,246,0.3)] hover:opacity-95 transition-opacity"
            @click="emit('play', device.serial)"
          >
            <MonitorPlay class="mr-1.5 h-3.5 w-3.5" />
            空间投屏
          </Button>

          <!-- 画中画独立窗 -->
          <Button
            size="sm"
            variant="ghost"
            :disabled="!isOnline"
            class="rounded-xl border border-white/10 bg-white/5 text-slate-200 hover:bg-white/10 hover:text-white px-2.5"
            title="画中画"
            @click="emit('pip', device.serial)"
          >
            <Tv class="h-3.5 w-3.5" />
          </Button>

          <!-- 快速终端 -->
          <Button
            size="sm"
            variant="ghost"
            :disabled="!isOnline"
            class="rounded-xl border border-white/10 bg-white/5 text-slate-200 hover:bg-white/10 hover:text-white px-2.5"
            title="ADB 终端"
            @click="emit('terminal', device.serial)"
          >
            <Terminal class="h-3.5 w-3.5" />
          </Button>

          <!-- 设备深度详情面板 -->
          <Button
            size="sm"
            variant="ghost"
            class="rounded-xl border border-white/10 bg-white/5 text-slate-200 hover:bg-white/10 hover:text-white px-2.5"
            title="硬件详情"
            @click="emit('inspect', device)"
          >
            <Info class="h-3.5 w-3.5" />
          </Button>
        </div>
      </div>
    </div>
  </div>
</template>
