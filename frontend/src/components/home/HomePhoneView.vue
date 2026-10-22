<script setup lang="ts">
import {
  CheckSquare,
  EyeOff,
  Smartphone,
  Square,
} from 'lucide-vue-next'
import { computed, ref } from 'vue'
import { useElementSize } from '@vueuse/core'
import DashTile from '@/components/dashboard/DashTile.vue'
import { Button } from '@/components/ui/button'
import { Empty, EmptyDescription, EmptyHeader, EmptyMedia, EmptyTitle } from '@/components/ui/empty'
import type { Device } from '@/api/devices'
import { useDeviceStore } from '@/stores/devices'
import { computeMonitorGrid } from '@/utils/monitorGrid'
import { getPhoneTileDimensionClasses } from '@/utils/phoneGrid'
import {
  type DashSize,
  isDeviceOnlineForStream,
} from '@/composables/useHomePhoneView'

const props = defineProps<{
  visibleDevices: Device[]
  dashSize: DashSize
  serialToNumberMap: Map<string, string>
  selectedSerials: string[]
  mockEnabled?: boolean
}>()

const emit = defineEmits<{
  (e: 'toggleSelect', serial: string): void
  (e: 'showAll'): void
}>()

const deviceStore = useDeviceStore()
const containerRef = ref<HTMLElement | null>(null)
const { width: containerWidth, height: containerHeight } = useElementSize(containerRef)

const dimensionConfig = computed(() => getPhoneTileDimensionClasses(props.dashSize))

const gridStyle = computed(() => {
  if (props.dashSize !== 'fit' || props.visibleDevices.length === 0)
    return {}

  const N = props.visibleDevices.length
  const W = containerWidth.value || window.innerWidth
  const H = containerHeight.value || (window.innerHeight - 220)
  const { rows, cols } = computeMonitorGrid(N, W, H)

  return {
    display: 'grid',
    gridTemplateColumns: `repeat(${cols}, minmax(0, 1fr))`,
    gridTemplateRows: `repeat(${rows}, minmax(0, 1fr))`,
    gap: '2px',
    height: '100%',
    width: '100%',
  }
})
</script>

<template>
  <div
    class="flex flex-col"
    :class="{ 'h-[calc(100vh-140px)] min-h-[500px] overflow-hidden': dashSize === 'fit' }"
  >
    <!-- 全隐藏空状态展示与恢复入口 -->
    <div
      v-if="visibleDevices.length === 0"
      class="flex flex-col items-center justify-center p-12 text-slate-400 space-y-3 rounded-3xl border border-dashed border-slate-200 dark:border-white/10 bg-white/40 dark:bg-white/[0.01]"
    >
      <Empty>
        <EmptyHeader>
          <EmptyMedia variant="icon"><EyeOff class="h-8 w-8 text-slate-400" /></EmptyMedia>
          <EmptyTitle>所有设备已隐藏</EmptyTitle>
          <EmptyDescription>
            当前所有设备均已隐藏，可通过右侧编号面板或点击下方按钮重新显示
          </EmptyDescription>
        </EmptyHeader>
        <Button
          variant="outline"
          size="sm"
          class="rounded-xl mt-3 cursor-pointer"
          @click="emit('showAll')"
        >
          全显
        </Button>
      </Empty>
    </div>

    <!-- 手机视图卡片主体（顶端与右侧面板自然对齐） -->
    <div
      v-else
      ref="containerRef"
      class="w-full flex-1 min-h-0"
      :class="dimensionConfig.containerClass"
      :style="gridStyle"
    >
      <div
        v-for="d in visibleDevices"
        :key="d.serial"
        class="group relative flex flex-col rounded-[2px] border border-neutral-800 bg-slate-900 overflow-hidden transition-all duration-150 select-none"
        :class="[
          dimensionConfig.cardClass,
          selectedSerials.includes(d.serial) ? 'ring-1 ring-violet-500 border-violet-500' : 'hover:border-neutral-600',
        ]"
      >
        <!-- 手机外框顶栏：编号标识、多选勾选、别名与状态（显隐统一由右侧面板编号方块控制） -->
        <div class="flex h-7 items-center justify-between border-b border-white/10 bg-slate-950/80 px-2.5 py-1 text-xs text-white shrink-0 select-none z-10">
          <div class="flex min-w-0 items-center gap-1.5">
            <button
              v-if="d.state === 'device' && !mockEnabled"
              type="button"
              class="flex h-5 w-5 shrink-0 items-center justify-center rounded text-white transition-colors cursor-pointer"
              :class="selectedSerials.includes(d.serial) ? 'text-violet-400' : 'text-slate-400 hover:text-white'"
              :title="selectedSerials.includes(d.serial) ? '取消勾选' : '勾选此设备'"
              @click.stop="emit('toggleSelect', d.serial)"
            >
              <component :is="selectedSerials.includes(d.serial) ? CheckSquare : Square" class="h-4 w-4" />
            </button>

            <!-- 稳定编号标识徽章 -->
            <span
              v-if="serialToNumberMap.has(d.serial)"
              class="px-1.5 py-0.2 rounded font-mono text-[10px] font-bold bg-violet-500/20 text-violet-300 border border-violet-500/30 shrink-0"
              :title="`设备编号: ${serialToNumberMap.get(d.serial)}`"
            >
              {{ serialToNumberMap.get(d.serial) }}
            </span>

            <div class="min-w-0 truncate">
              <span class="truncate font-semibold text-[11px] text-slate-100">
                {{ deviceStore.displayName(d) }}
              </span>
            </div>
          </div>

          <div class="flex items-center gap-1.5 shrink-0">
            <span
              class="h-2 w-2 rounded-full"
              :class="d.state === 'device' ? 'bg-emerald-500' : 'bg-amber-500'"
              :title="d.state === 'device' ? '在线' : d.state"
            />
          </div>
        </div>

        <!-- 真实在线设备且非 Mock 挂载投屏；Mock 或离线展示占位 -->
        <div class="relative w-full flex-1 min-h-0 bg-black overflow-hidden flex items-center justify-center">
          <DashTile
            v-if="isDeviceOnlineForStream(d, mockEnabled)"
            :device="d"
            mode="contain"
            class="w-full h-full"
          />
          <div v-else class="flex flex-col items-center justify-center p-6 text-center space-y-2">
            <Smartphone class="h-8 w-8 text-amber-500/60" />
            <span class="text-xs text-amber-400 font-medium">
              {{ mockEnabled ? 'Mock 模拟设备' : d.state }}
            </span>
            <p class="text-[10px] text-slate-500">
              {{ mockEnabled ? '已排除模拟设备，不向服务端发起真机连接' : '设备离线，未订阅实时投屏' }}
            </p>
          </div>
        </div>
      </div>
    </div>
  </div>
</template>
