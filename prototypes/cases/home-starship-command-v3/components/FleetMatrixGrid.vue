<script setup lang="ts">
import {
  Battery,
  BatteryCharging,
  CheckSquare,
  Crosshair,
  MonitorPlay,
  Square,
  Tv,
  Usb,
  Wifi,
} from 'lucide-vue-next'
import { Button } from '@/components/ui/button'
import type { FleetDevice } from '../mockDevices'

const props = defineProps<{
  devices: FleetDevice[]
  selectedSerial: string | null
  batchSelectedSerials: string[]
}>()

const emit = defineEmits<{
  (e: 'select', device: FleetDevice): void
  (e: 'toggleBatch', serial: string): void
  (e: 'play', serial: string): void
  (e: 'pip', serial: string): void
}>()
</script>

<template>
  <div class="grid grid-cols-1 gap-4 sm:grid-cols-2">
    <div
      v-for="dev in devices"
      :key="dev.serial"
      class="group relative flex flex-col justify-between overflow-hidden rounded-2xl border p-4.5 transition-all duration-200 cursor-pointer"
      :class="[
        selectedSerial === dev.serial
          ? 'border-cyan-400 bg-blue-950/40 shadow-[0_0_25px_rgba(0,242,254,0.25)] ring-1 ring-cyan-400'
          : dev.state === 'device'
            ? 'border-blue-500/25 bg-[#0a0f1d]/80 hover:border-blue-400/50 hover:bg-blue-950/30'
            : 'border-slate-800 bg-slate-950/40 opacity-70 hover:opacity-90',
      ]"
      @click="emit('select', dev)"
    >
      <!-- 顶栏状态与批选框 -->
      <div>
        <div class="flex items-center justify-between border-b border-blue-500/10 pb-2.5">
          <div class="flex items-center gap-2">
            <!-- 批选勾选框 -->
            <button
              class="text-slate-400 hover:text-cyan-300"
              @click.stop="emit('toggleBatch', dev.serial)"
            >
              <CheckSquare
                v-if="batchSelectedSerials.includes(dev.serial)"
                class="h-4 w-4 text-cyan-400"
              />
              <Square v-else class="h-4 w-4" />
            </button>

            <span
              class="h-2 w-2 rounded-full"
              :class="dev.state === 'device' ? 'bg-emerald-400 animate-pulse' : dev.state === 'unauthorized' ? 'bg-amber-400' : 'bg-slate-500'"
            />
            <span class="font-mono text-xs font-semibold text-white">
              {{ dev.name }}
            </span>
          </div>

          <div class="flex items-center gap-1.5 text-xs font-mono text-slate-400">
            <Usb v-if="dev.source === 'auto'" class="h-3 w-3 text-cyan-400" />
            <Wifi v-else class="h-3 w-3 text-blue-400" />
            <span class="text-[10px]">{{ dev.source === 'auto' ? 'USB' : 'TCP' }}</span>
          </div>
        </div>

        <!-- 战位型号与序列号 -->
        <div class="mt-2.5 flex items-baseline justify-between text-xs font-mono text-slate-400">
          <span>{{ dev.model }}</span>
          <span class="text-cyan-300/80">{{ dev.serial }}</span>
        </div>

        <!-- 模拟监视大屏微缩帧 -->
        <div class="relative mt-3 overflow-hidden rounded-xl border border-blue-500/20 bg-black/80 p-3">
          <div class="flex items-center justify-between text-xs font-mono">
            <span class="text-slate-400">前台进程:</span>
            <span class="font-semibold text-emerald-300 truncate max-w-[140px]">{{ dev.currentApp }}</span>
          </div>

          <!-- 实时遥测三维参数 -->
          <div class="mt-2.5 grid grid-cols-3 gap-2 border-t border-slate-800 pt-2 text-[10px] font-mono">
            <div>
              <span class="text-slate-500 block">流码率</span>
              <span class="text-cyan-300 font-semibold">{{ dev.streamBitrateMbps }} Mbps</span>
            </div>
            <div>
              <span class="text-slate-500 block">帧率</span>
              <span class="text-white font-semibold">{{ dev.fps }} FPS</span>
            </div>
            <div>
              <span class="text-slate-500 block">时延</span>
              <span class="text-emerald-300 font-semibold">{{ dev.latencyMs }} ms</span>
            </div>
          </div>
        </div>

        <!-- 电量与分辨率底标 -->
        <div class="mt-2.5 flex items-center justify-between text-xs font-mono text-slate-400">
          <div class="flex items-center gap-1">
            <BatteryCharging v-if="dev.batteryCharging" class="h-3.5 w-3.5 text-emerald-400" />
            <Battery v-else class="h-3.5 w-3.5 text-slate-400" />
            <span>{{ dev.batteryPercent }}% ({{ dev.tempCelsius }}°C)</span>
          </div>
          <span class="text-[10px] text-slate-500">{{ dev.resolution }}</span>
        </div>
      </div>

      <!-- 快速操作条 -->
      <div class="mt-3.5 flex items-center gap-2 border-t border-blue-500/10 pt-2.5">
        <Button
          size="sm"
          :disabled="dev.state !== 'device'"
          class="flex-1 h-7.5 bg-blue-600/80 font-mono text-xs font-medium text-white hover:bg-blue-500 shadow-sm"
          @click.stop="emit('play', dev.serial)"
        >
          <MonitorPlay class="mr-1 h-3 w-3" />
          投屏
        </Button>
        <Button
          size="sm"
          variant="outline"
          :disabled="dev.state !== 'device'"
          class="h-7.5 border-slate-700 bg-slate-900/60 px-2 text-xs text-slate-300 hover:bg-slate-800"
          title="画中画微窗"
          @click.stop="emit('pip', dev.serial)"
        >
          <Tv class="h-3 w-3" />
        </Button>
        <Button
          size="sm"
          variant="ghost"
          class="h-7.5 px-2 text-xs text-cyan-300 hover:bg-cyan-950/40"
          title="锁定战术深研舱"
          @click.stop="emit('select', dev)"
        >
          <Crosshair class="h-3 w-3" />
        </Button>
      </div>
    </div>
  </div>
</template>
