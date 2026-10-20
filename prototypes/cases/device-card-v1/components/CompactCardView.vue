<script setup lang="ts">
import { computed } from 'vue'
import {
  BatteryCharging,
  MonitorPlay,
  Pencil,
  Smartphone,
  Thermometer,
  Tv,
} from 'lucide-vue-next'
import { Button } from '@/components/ui/button'
import type { PrototypeDevice } from '../mockDevices'

const props = defineProps<{
  device: PrototypeDevice
}>()

const emit = defineEmits<{
  (e: 'play', serial: string): void
  (e: 'pip', serial: string): void
  (e: 'edit', device: PrototypeDevice): void
}>()

const displayName = computed(() => {
  if (props.device.alias?.trim()) return props.device.alias.trim()
  if (props.device.telemetry.brand?.trim()) return props.device.telemetry.brand.trim()
  return props.device.telemetry.model || 'Android'
})

const isOnline = computed(() => props.device.state === 'device')
</script>

<template>
  <div
    class="group flex items-center justify-between rounded-xl border border-slate-200/80 bg-white/90 p-3 text-slate-900 shadow-2xs hover:border-blue-500/40 hover:bg-white dark:border-white/10 dark:bg-slate-900/60 dark:text-white dark:hover:border-blue-400/30 transition-all select-none"
  >
    <div class="flex items-center gap-3 min-w-0 flex-1">
      <!-- 手机图标与在线指示 -->
      <div
        class="flex h-9 w-9 shrink-0 items-center justify-center rounded-lg"
        :class="isOnline ? 'bg-emerald-500/10 text-emerald-600 dark:text-emerald-400 border border-emerald-500/20' : 'bg-slate-100 text-slate-400 border border-slate-200 dark:bg-white/5 dark:border-white/10'"
      >
        <Smartphone class="h-4 w-4" />
      </div>

      <!-- 设备基础信息：品牌名优先 -->
      <div class="min-w-0 flex-1">
        <div class="flex items-center gap-2">
          <p class="truncate font-bold text-sm text-slate-900 dark:text-white">
            {{ displayName }}
          </p>
          <span
            class="h-1.5 w-1.5 rounded-full shrink-0"
            :class="isOnline ? 'bg-emerald-500 animate-pulse' : 'bg-slate-400'"
          />
          <span class="rounded bg-slate-100 dark:bg-white/10 px-1.5 py-0.2 text-[9px] font-mono text-slate-500 dark:text-slate-400">
            Android {{ device.telemetry.androidRelease }}
          </span>
        </div>

        <p class="truncate font-mono text-[11px] text-slate-500 dark:text-slate-400 mt-0.5">
          {{ device.telemetry.model }} · {{ device.serial }}
        </p>
      </div>

      <!-- 核心指标胶囊条 (电池、温度、前台) -->
      <div v-if="isOnline" class="hidden md:flex items-center gap-3 text-xs text-slate-600 dark:text-slate-300">
        <!-- 电池与电量 -->
        <div class="flex items-center gap-1 font-mono">
          <BatteryCharging v-if="device.telemetry.batteryCharging" class="h-3.5 w-3.5 text-emerald-500" />
          <span>{{ device.telemetry.batteryLevel }}%</span>
        </div>

        <!-- 电池温度 -->
        <div class="flex items-center gap-1 font-mono">
          <Thermometer class="h-3.5 w-3.5 text-blue-500" />
          <span>{{ device.telemetry.batteryTempCelsius ? `${device.telemetry.batteryTempCelsius.toFixed(1)}°C` : '--' }}</span>
        </div>

        <!-- 前台应用 -->
        <div class="max-w-[130px] truncate text-[11px] text-blue-600 dark:text-blue-400">
          {{ device.telemetry.currentApp }}
        </div>
      </div>
    </div>

    <!-- 紧凑操作按钮 -->
    <div class="flex items-center gap-1.5 shrink-0 ml-3">
      <Button
        v-if="isOnline"
        size="sm"
        class="h-8 rounded-lg bg-[#406BDB] hover:bg-[#3258be] text-white text-xs px-2.5 cursor-pointer"
        @click="emit('play', device.serial)"
      >
        <MonitorPlay class="mr-1 h-3.5 w-3.5" />
        投屏
      </Button>

      <Button
        v-if="isOnline"
        size="icon"
        variant="ghost"
        class="h-8 w-8 rounded-lg border border-slate-200 dark:border-white/10 p-0 text-slate-600 dark:text-slate-300 cursor-pointer"
        title="画中画微窗"
        @click="emit('pip', device.serial)"
      >
        <Tv class="h-3.5 w-3.5" />
      </Button>

      <Button
        size="icon"
        variant="ghost"
        class="h-8 w-8 rounded-lg text-slate-400 hover:text-slate-900 dark:hover:text-white cursor-pointer"
        title="重命名"
        @click="emit('edit', device)"
      >
        <Pencil class="h-3.5 w-3.5" />
      </Button>
    </div>
  </div>
</template>
