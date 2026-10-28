<script setup lang="ts">
import { CheckSquare, Square } from 'lucide-vue-next'
import DeviceCard from '@/components/device/DeviceCard.vue'
import type { Device } from '@/api/devices'

defineProps<{
  device: Device
  viewMode: 'deck' | 'compact'
  selected: boolean
  mockEnabled: boolean
}>()

const emit = defineEmits<{
  (e: 'toggleSelect', serial: string): void
  (e: 'openFloating', serial: string): void
}>()
</script>

<template>
  <div class="group relative transition-all duration-150">
    <!-- 宿主独立勾选按钮（左上角悬浮，点击不穿透，独立多选） -->
    <button
      v-if="device.state === 'device' && !mockEnabled"
      type="button"
      class="absolute left-3 top-3 z-20 flex h-6 w-6 items-center justify-center rounded-lg border border-slate-200/80 bg-white/90 shadow-sm backdrop-blur-md transition-all cursor-pointer dark:border-white/15 dark:bg-slate-900/90 text-slate-400 hover:text-slate-900 dark:hover:text-white"
      :class="{ '!border-violet-500 !bg-violet-600 !text-white': selected }"
      :title="selected ? '取消勾选' : '勾选此设备进行批量操作'"
      @click.stop="emit('toggleSelect', device.serial)"
    >
      <component :is="selected ? CheckSquare : Square" class="h-3.5 w-3.5" />
    </button>

    <div :class="viewMode === 'deck' ? { 'ring-2 ring-violet-500 rounded-3xl': selected } : { 'ring-2 ring-violet-500 rounded-2xl': selected }">
      <DeviceCard
        :device="device"
        :view-mode="viewMode"
        @open-floating="emit('openFloating', device.serial)"
      />
    </div>
  </div>
</template>
