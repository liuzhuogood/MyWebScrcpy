<script setup lang="ts">
import { computed } from 'vue'
import {
  Battery,
  BatteryCharging,
  MonitorPlay,
  RotateCcw,
  Terminal,
  Tv,
  Usb,
  Wifi,
} from 'lucide-vue-next'
import { Badge } from '@/components/ui/badge'
import { Button } from '@/components/ui/button'
import type { CyberDevice } from '../mockDevices'

const props = defineProps<{
  device: CyberDevice
  index: number
}>()

const emit = defineEmits<{
  (e: 'play', serial: string): void
  (e: 'pip', serial: string): void
  (e: 'terminal', serial: string): void
  (e: 'reconnect', serial: string): void
  (e: 'delete', serial: string): void
}>()

const isOnline = computed(() => props.device.state === 'device')
const isUnauthorized = computed(() => props.device.state === 'unauthorized')

const indexCode = computed(() => String(props.index + 1).padStart(2, '0'))
</script>

<template>
  <div
    class="group relative flex flex-col justify-between overflow-hidden rounded-2xl border bg-slate-950/70 p-5 transition-all duration-300 backdrop-blur-md"
    :class="[
      isOnline
        ? 'border-cyan-500/30 hover:border-cyan-400 hover:shadow-[0_0_30px_rgba(0,242,254,0.2)]'
        : isUnauthorized
          ? 'border-amber-500/40 hover:border-amber-400'
          : 'border-slate-800/80 opacity-75 hover:opacity-100',
    ]"
  >
    <!-- 赛博 HUD 四角折角装饰标 -->
    <div class="pointer-events-none absolute left-0 top-0 h-3 w-3 border-l-2 border-t-2 border-cyan-400/70" />
    <div class="pointer-events-none absolute right-0 top-0 h-3 w-3 border-r-2 border-t-2 border-cyan-400/70" />
    <div class="pointer-events-none absolute bottom-0 left-0 h-3 w-3 border-b-2 border-l-2 border-cyan-400/70" />
    <div class="pointer-events-none absolute bottom-0 right-0 h-3 w-3 border-b-2 border-r-2 border-cyan-400/70" />

    <!-- 顶栏状态与编号 -->
    <div>
      <div class="flex items-center justify-between gap-2 border-b border-cyan-500/10 pb-3">
        <div class="flex items-center gap-2">
          <span class="font-mono text-xs font-bold text-cyan-400/80">#NODE_{{ indexCode }}</span>
          <Badge
            variant="outline"
            class="font-mono text-[10px] tracking-wider"
            :class="[
              isOnline
                ? 'border-emerald-500/40 bg-emerald-950/40 text-emerald-300'
                : isUnauthorized
                  ? 'border-amber-500/40 bg-amber-950/40 text-amber-300'
                  : 'border-slate-700 bg-slate-900 text-slate-400',
            ]"
          >
            <span
              class="mr-1.5 inline-block h-1.5 w-1.5 rounded-full"
              :class="isOnline ? 'bg-emerald-400 animate-pulse' : isUnauthorized ? 'bg-amber-400' : 'bg-slate-500'"
            />
            {{ isOnline ? 'ONLINE // READY' : isUnauthorized ? 'UNAUTHORIZED' : 'OFFLINE' }}
          </Badge>
        </div>

        <!-- 传输介质徽标 -->
        <div class="flex items-center gap-1 font-mono text-[11px] text-slate-400">
          <Usb v-if="device.source === 'auto'" class="h-3.5 w-3.5 text-cyan-400" />
          <Wifi v-else class="h-3.5 w-3.5 text-emerald-400" />
          <span>{{ device.source === 'auto' ? 'USB 3.2' : 'WI-FI 6' }}</span>
        </div>
      </div>

      <!-- 设备主体信息 -->
      <div class="mt-3.5 flex items-start justify-between gap-3">
        <div>
          <h3 class="font-sans text-base font-semibold tracking-tight text-white group-hover:text-cyan-200 transition-colors">
            {{ device.name }}
          </h3>
          <p class="font-mono text-xs text-slate-400 mt-0.5">
            {{ device.model }} · <span class="text-cyan-300/90">{{ device.serial }}</span>
          </p>
        </div>
      </div>

      <!-- 拟真机身迷你全息屏预览区 -->
      <div class="relative mt-4 overflow-hidden rounded-xl border border-cyan-500/20 bg-gradient-to-b from-slate-900 to-black p-3.5">
        <!-- 扫光动画线 -->
        <div
          v-if="isOnline"
          class="pointer-events-none absolute inset-x-0 h-10 -translate-y-full bg-gradient-to-b from-transparent via-cyan-400/10 to-transparent animate-scan"
        />

        <div class="flex items-center justify-between text-xs font-mono">
          <span class="text-slate-400">前台活跃 APP:</span>
          <span class="font-medium text-emerald-300 flex items-center gap-1">
            <span class="h-1.5 w-1.5 rounded-full bg-emerald-400 animate-ping" />
            {{ device.telemetry.currentApp }}
          </span>
        </div>
        <div class="mt-1 truncate font-mono text-[10px] text-slate-500">
          {{ device.telemetry.currentPackage }}
        </div>

        <!-- 硬件参数横向条 -->
        <div class="mt-3 grid grid-cols-3 gap-2 border-t border-slate-800/80 pt-2 text-[11px] font-mono">
          <div>
            <span class="text-slate-500 text-[10px] block">刷新率</span>
            <span class="text-cyan-300 font-semibold">{{ device.telemetry.fps }} FPS</span>
          </div>
          <div>
            <span class="text-slate-500 text-[10px] block">机身温度</span>
            <span :class="device.telemetry.tempCelsius > 36 ? 'text-amber-300' : 'text-slate-300'">
              {{ device.telemetry.tempCelsius }}°C
            </span>
          </div>
          <div>
            <span class="text-slate-500 text-[10px] block">投屏时延</span>
            <span class="text-emerald-300 font-semibold">{{ device.telemetry.latencyMs }} ms</span>
          </div>
        </div>
      </div>

      <!-- 电池与分辨率底行 -->
      <div class="mt-3 flex items-center justify-between text-xs font-mono text-slate-400">
        <div class="flex items-center gap-1.5">
          <BatteryCharging v-if="device.telemetry.batteryCharging" class="h-4 w-4 text-emerald-400" />
          <Battery v-else class="h-4 w-4 text-slate-300" />
          <span>{{ device.telemetry.batteryPercent }}%</span>
        </div>
        <div class="rounded bg-slate-900/80 px-1.5 py-0.5 text-[10px] text-slate-300 border border-slate-800">
          {{ device.telemetry.resolution }}
        </div>
      </div>
    </div>

    <!-- 底部操作按钮群 -->
    <div class="mt-5 border-t border-cyan-500/10 pt-3.5">
      <div class="flex items-center gap-2">
        <!-- 主控投屏 -->
        <Button
          size="sm"
          :disabled="!isOnline"
          class="flex-1 bg-cyan-500 text-slate-950 font-semibold hover:bg-cyan-400 shadow-[0_0_15px_rgba(0,242,254,0.25)] transition-all font-mono text-xs"
          @click="emit('play', device.serial)"
        >
          <MonitorPlay class="mr-1.5 h-3.5 w-3.5" />
          投屏操控
        </Button>

        <!-- 浮动小窗画中画 -->
        <Button
          size="sm"
          variant="outline"
          :disabled="!isOnline"
          class="border-cyan-500/30 bg-cyan-950/30 text-cyan-300 hover:bg-cyan-900/40 hover:text-white px-2.5"
          title="独立画中画"
          @click="emit('pip', device.serial)"
        >
          <Tv class="h-3.5 w-3.5" />
        </Button>

        <!-- ADB 终端 -->
        <Button
          size="sm"
          variant="outline"
          :disabled="!isOnline"
          class="border-slate-700 bg-slate-900/50 text-slate-300 hover:bg-slate-800 px-2.5"
          title="ADB 终端"
          @click="emit('terminal', device.serial)"
        >
          <Terminal class="h-3.5 w-3.5" />
        </Button>

        <!-- 异常设备重连 / 移除 -->
        <Button
          v-if="!isOnline"
          size="sm"
          variant="outline"
          class="border-amber-500/30 bg-amber-950/30 text-amber-300 hover:bg-amber-900/40 px-2.5"
          title="重连"
          @click="emit('reconnect', device.serial)"
        >
          <RotateCcw class="h-3.5 w-3.5" />
        </Button>
      </div>
    </div>
  </div>
</template>

<style scoped>
@keyframes scan {
  0% { transform: translateY(-100%); }
  100% { transform: translateY(400%); }
}
.animate-scan {
  animation: scan 3.5s cubic-bezier(0.4, 0, 0.2, 1) infinite;
}
</style>
