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
  <div class="group relative flex items-center justify-center transition-all duration-150">
    <div
      class="relative"
      :class="viewMode === 'deck' ? { 'ring-2 ring-violet-500 rounded-[38px]': selected } : { 'ring-2 ring-violet-500 rounded-2xl w-full': selected, 'w-full': true }"
    >
      <!-- 宿主独立勾选按钮（点击不穿透，跟随卡片左上角） -->
      <button
        v-if="device.state === 'device' && !mockEnabled"
        type="button"
        class="absolute -left-2 -top-2 z-20 flex h-6 w-6 items-center justify-center rounded-lg border border-slate-200/90 bg-white shadow-md backdrop-blur-md transition-all cursor-pointer dark:border-white/20 dark:bg-slate-900 text-slate-400 hover:text-slate-900 dark:hover:text-white"
        :class="{ '!border-violet-500 !bg-violet-600 !text-white': selected }"
        :title="selected ? '取消勾选' : '勾选此设备进行批量操作'"
        @click.stop="emit('toggleSelect', device.serial)"
      >
        <component :is="selected ? CheckSquare : Square" class="h-3.5 w-3.5" />
      </button>

      <DeviceCard
        :device="device"
        :view-mode="viewMode"
        @open-floating="emit('openFloating', device.serial)"
      />
    </div>
  </div>
</template>
