<script setup lang="ts">
import { computed } from 'vue'
import { Badge } from '@/components/ui/badge'
import { Button } from '@/components/ui/button'
import type { Device } from '@/api/devices'
import { buildStableNumberedDevices } from '@/composables/useHomePhoneView'

const props = defineProps<{
  devices: Device[]
  hiddenSerials: string[]
  selectedSerials: string[]
  displayName: (d: Device) => string
}>()

const emit = defineEmits<{
  (e: 'toggleHide', serial: string): void
  (e: 'showAll'): void
  (e: 'showOnlySelected'): void
}>()

const numberedDevices = computed(() => {
  return buildStableNumberedDevices(props.devices)
})

const hiddenSet = computed(() => new Set(props.hiddenSerials))
const selectedSet = computed(() => new Set(props.selectedSerials))

const visibleCount = computed(() => {
  return props.devices.filter(d => !hiddenSet.value.has(d.serial)).length
})

const hasHidden = computed(() => {
  return props.devices.some(d => hiddenSet.value.has(d.serial))
})

const canShowOnlySelected = computed(() => {
  return props.selectedSerials.length > 0
})
</script>

<template>
  <div class="space-y-2">
    <!-- 头部工具行：设备统计与全显/只显已选 -->
    <div class="flex items-center justify-between gap-1 text-xs">
      <div class="flex items-center gap-1.5 min-w-0">
        <span class="font-medium text-slate-700 dark:text-slate-300">设备</span>
        <Badge variant="secondary" class="h-4.5 px-1.5 text-[10px] font-mono leading-none">
          {{ visibleCount }}/{{ devices.length }}
        </Badge>
      </div>
      <div class="flex items-center gap-1 shrink-0">
        <Button
          variant="outline"
          size="sm"
          class="h-6 px-2 text-[11px] rounded-lg cursor-pointer"
          :disabled="!hasHidden"
          title="全显"
          @click="emit('showAll')"
        >
          全显
        </Button>
        <Button
          variant="outline"
          size="sm"
          class="h-6 px-2 text-[11px] rounded-lg cursor-pointer"
          :disabled="!canShowOnlySelected"
          title="只显已选"
          @click="emit('showOnlySelected')"
        >
          只显已选
        </Button>
      </div>
    </div>

    <!-- 无设备空提示 -->
    <div
      v-if="devices.length === 0"
      class="py-4 text-center text-xs text-slate-400 border border-dashed border-slate-200 dark:border-white/10 rounded-2xl"
    >
      暂无匹配设备
    </div>

    <!-- 01..N 真实编号小方块选择器 -->
    <div
      v-else
      class="grid grid-cols-6 sm:grid-cols-6 md:grid-cols-8 lg:grid-cols-6 xl:grid-cols-8 gap-1.5 max-h-52 overflow-y-auto p-1 rounded-2xl border border-slate-200/80 bg-slate-50/50 dark:border-white/10 dark:bg-black/20"
    >
      <button
        v-for="item in numberedDevices"
        :key="item.serial"
        type="button"
        class="relative flex flex-col items-center justify-center h-8 rounded-lg text-xs font-mono font-bold transition-all cursor-pointer select-none"
        :class="[
          !hiddenSet.has(item.serial)
            ? (selectedSet.has(item.serial)
                ? 'border-2 border-violet-500 bg-violet-600 text-white shadow-xs'
                : 'border border-slate-300 dark:border-white/20 bg-white dark:bg-white/10 text-slate-800 dark:text-slate-100 hover:border-violet-400')
            : 'opacity-40 border border-dashed border-slate-300 dark:border-white/10 bg-slate-200/40 dark:bg-white/5 text-slate-400 line-through',
        ]"
        :title="`[${item.label}] ${displayName(item.device)} (${item.serial}) · ${item.device.state === 'device' ? '在线' : '离线'}`"
        @click="emit('toggleHide', item.serial)"
      >
        <span
          class="absolute top-1 right-1 h-1.5 w-1.5 rounded-full"
          :class="item.device.state === 'device' ? 'bg-emerald-400' : 'bg-amber-400'"
        />
        <span>{{ item.label }}</span>
      </button>
    </div>
  </div>
</template>
