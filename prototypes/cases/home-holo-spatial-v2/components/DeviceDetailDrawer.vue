<script setup lang="ts">
import {
  Cpu,
  Monitor,
  Power,
  RotateCw,
  Smartphone,
  Volume2,
  VolumeX,
  X,
  Zap,
} from 'lucide-vue-next'
import { Button } from '@/components/ui/button'
import { toast } from 'vue-sonner'
import type { SpatialDevice } from '../mockDevices'

const props = defineProps<{
  device: SpatialDevice | null
}>()

const emit = defineEmits<{
  (e: 'close'): void
  (e: 'play', serial: string): void
}>()

function triggerKey(keyName: string) {
  toast.success(`已向设备注入模拟键值: ${keyName}`)
}
</script>

<template>
  <div
    v-if="device"
    class="fixed inset-y-0 right-0 z-50 flex w-full max-w-md flex-col border-l border-white/10 bg-[#0b0e17]/95 p-6 shadow-2xl backdrop-blur-3xl animate-in slide-in-from-right"
  >
    <!-- 顶栏 -->
    <div class="flex items-center justify-between border-b border-white/10 pb-4">
      <div class="flex items-center gap-2.5">
        <div class="flex h-8 w-8 items-center justify-center rounded-xl bg-violet-600/20 text-violet-400 border border-violet-500/30">
          <Smartphone class="h-4 w-4" />
        </div>
        <div>
          <h3 class="text-sm font-semibold text-white">{{ device.name }}</h3>
          <p class="text-[11px] font-mono text-slate-400">{{ device.serial }}</p>
        </div>
      </div>
      <Button variant="ghost" size="icon" class="h-8 w-8 rounded-full text-slate-400 hover:text-white" @click="emit('close')">
        <X class="h-4 w-4" />
      </Button>
    </div>

    <!-- 抽屉滚动内容 -->
    <div class="flex-1 overflow-y-auto space-y-5 py-4 text-xs">
      <!-- 硬件参数卡片 -->
      <div class="rounded-2xl border border-white/10 bg-white/[0.02] p-4 space-y-3">
        <h4 class="font-medium text-slate-300 flex items-center gap-1.5">
          <Cpu class="h-3.5 w-3.5 text-violet-400" />
          硬件参数遥测
        </h4>
        <div class="grid grid-cols-2 gap-2.5 font-mono text-slate-400">
          <div class="rounded-xl bg-white/[0.02] p-2.5 border border-white/5">
            <span class="text-[10px] text-slate-500 block">原生分辨率</span>
            <span class="text-white font-semibold">{{ device.telemetry.resolution }}</span>
          </div>
          <div class="rounded-xl bg-white/[0.02] p-2.5 border border-white/5">
            <span class="text-[10px] text-slate-500 block">画面比例</span>
            <span class="text-white font-semibold">{{ device.telemetry.aspectRatio }}</span>
          </div>
          <div class="rounded-xl bg-white/[0.02] p-2.5 border border-white/5">
            <span class="text-[10px] text-slate-500 block">系统版本</span>
            <span class="text-violet-300 font-semibold truncate block">{{ device.telemetry.androidVersion }}</span>
          </div>
          <div class="rounded-xl bg-white/[0.02] p-2.5 border border-white/5">
            <span class="text-[10px] text-slate-500 block">传输模式</span>
            <span class="text-cyan-300 font-semibold">{{ device.source === 'auto' ? 'USB 极速直连' : 'Wi-Fi 局域网' }}</span>
          </div>
        </div>
      </div>

      <!-- 快速远程硬件按键模拟 -->
      <div class="rounded-2xl border border-white/10 bg-white/[0.02] p-4 space-y-3">
        <h4 class="font-medium text-slate-300 flex items-center gap-1.5">
          <Zap class="h-3.5 w-3.5 text-cyan-400" />
          物理按键快速注入 (ADB KEYCODE)
        </h4>
        <div class="grid grid-cols-4 gap-2">
          <Button
            size="sm"
            variant="ghost"
            class="h-9 rounded-xl border border-white/10 bg-white/5 text-slate-200 hover:bg-white/10"
            @click="triggerKey('KEYCODE_BACK (返回)')"
          >
            返回
          </Button>
          <Button
            size="sm"
            variant="ghost"
            class="h-9 rounded-xl border border-white/10 bg-white/5 text-slate-200 hover:bg-white/10"
            @click="triggerKey('KEYCODE_HOME (主屏)')"
          >
            主屏
          </Button>
          <Button
            size="sm"
            variant="ghost"
            class="h-9 rounded-xl border border-white/10 bg-white/5 text-slate-200 hover:bg-white/10"
            @click="triggerKey('KEYCODE_APP_SWITCH (任务)')"
          >
            多任务
          </Button>
          <Button
            size="sm"
            variant="ghost"
            class="h-9 rounded-xl border border-white/10 bg-white/5 text-rose-300 hover:bg-rose-950/30"
            @click="triggerKey('KEYCODE_POWER (电源)')"
          >
            <Power class="h-3.5 w-3.5" />
          </Button>
        </div>

        <div class="grid grid-cols-3 gap-2 pt-1">
          <Button
            size="sm"
            variant="ghost"
            class="h-8 rounded-xl border border-white/10 bg-white/5 text-slate-300 hover:bg-white/10"
            @click="triggerKey('VOLUME_UP (+)')"
          >
            <Volume2 class="mr-1 h-3.5 w-3.5" /> 音量+
          </Button>
          <Button
            size="sm"
            variant="ghost"
            class="h-8 rounded-xl border border-white/10 bg-white/5 text-slate-300 hover:bg-white/10"
            @click="triggerKey('VOLUME_DOWN (-)')"
          >
            <VolumeX class="mr-1 h-3.5 w-3.5" /> 音量-
          </Button>
          <Button
            size="sm"
            variant="ghost"
            class="h-8 rounded-xl border border-white/10 bg-white/5 text-slate-300 hover:bg-white/10"
            @click="triggerKey('ROTATE (横竖屏切换)')"
          >
            <RotateCw class="mr-1 h-3.5 w-3.5" /> 旋转
          </Button>
        </div>
      </div>
    </div>

    <!-- 底部投屏直达 -->
    <div class="border-t border-white/10 pt-4">
      <Button
        class="w-full h-10 rounded-xl bg-gradient-to-r from-violet-600 to-cyan-500 font-semibold text-white shadow-xl hover:opacity-95"
        @click="emit('play', device.serial)"
      >
        <Monitor class="mr-2 h-4 w-4" />
        拉起空间超清投屏
      </Button>
    </div>
  </div>
</template>
