<script setup lang="ts">
import { ArrowLeft, LayoutGrid, Maximize, Minimize, MonitorSmartphone, Smartphone } from 'lucide-vue-next'
import { computed, onMounted, onUnmounted, ref } from 'vue'
import { RouterLink } from 'vue-router'
import { useElementSize, useFullscreen } from '@vueuse/core'
import DashTile from '@/components/dashboard/DashTile.vue'
import { Button } from '@/components/ui/button'
import { Empty, EmptyDescription, EmptyHeader, EmptyMedia, EmptyTitle } from '@/components/ui/empty'
import { Skeleton } from '@/components/ui/skeleton'
import { fetchDevices, type Device } from '@/api/devices'
import { computeMonitorGrid } from '@/utils/monitorGrid'

type DashSize = 'small' | 'medium' | 'large' | 'fit'
const SIZE_KEY = 'dash_size'
const POLL_MS = 5000

const containerRef = ref<HTMLElement | null>(null)
const { width: containerWidth, height: containerHeight } = useElementSize(containerRef)
const { isFullscreen: isPageFs, toggle: togglePageFs } = useFullscreen()

const devices = ref<Device[]>([])
const loading = ref(true)
const loadError = ref('')
const size = ref<DashSize>((localStorage.getItem(SIZE_KEY) as DashSize) || 'small')
let timer: number | undefined

const countText = computed(() => {
  if (loading.value && devices.value.length === 0)
    return '加载中...'
  if (loadError.value && devices.value.length === 0)
    return '加载失败'
  return devices.value.length > 0 ? `${devices.value.length} 台设备直播中` : '无设备'
})

const tileWidth = computed(() => {
  // 三档卡片宽度；高度由画面比例自适应
  if (size.value === 'small')
    return 'w-44'
  if (size.value === 'medium')
    return 'w-72'
  return 'w-[26rem]'
})

const gridStyle = computed(() => {
  if (size.value !== 'fit' || devices.value.length === 0)
    return {}

  const N = devices.value.length
  const W = containerWidth.value || window.innerWidth
  const H = containerHeight.value || (window.innerHeight - 56)
  const { rows, cols } = computeMonitorGrid(N, W, H)

  return {
    display: 'grid',
    gridTemplateColumns: `repeat(${cols}, minmax(0, 1fr))`,
    gridTemplateRows: `repeat(${rows}, minmax(0, 1fr))`,
    height: '100%',
    width: '100%',
  }
})

function setSize(s: DashSize) {
  size.value = s
  localStorage.setItem(SIZE_KEY, s)
}

async function load() {
  try {
    // 大屏只接真实设备：Mock 假 serial 建连必红屏，无意义
    devices.value = await fetchDevices()
    loadError.value = ''
  }
  catch (e) {
    if (devices.value.length === 0)
      loadError.value = e instanceof Error ? e.message : '加载失败'
  }
  finally {
    loading.value = false
  }
}

onMounted(() => {
  load()
  timer = window.setInterval(load, POLL_MS)
})

onUnmounted(() => {
  if (timer !== undefined)
    window.clearInterval(timer)
})
</script>

<template>
  <div class="flex min-h-screen flex-col bg-background" :class="{ 'h-screen overflow-hidden': size === 'fit' }">
    <header class="sticky top-0 z-10 border-b bg-background/80 backdrop-blur shrink-0">
      <div class="flex h-14 w-full items-center gap-3 px-4">
        <Button variant="ghost" size="icon" aria-label="返回设备列表" as-child>
          <RouterLink to="/"><ArrowLeft class="h-5 w-5" /></RouterLink>
        </Button>
        <span class="flex items-center gap-2 text-base font-semibold">
          <MonitorSmartphone class="h-5 w-5" />大屏监控
        </span>
        <span class="text-sm text-muted-foreground">{{ countText }}</span>
        <div class="ml-auto flex items-center gap-1" role="group" aria-label="卡片尺寸">
          <Button
            variant="outline" size="icon" title="小" aria-label="小尺寸卡片"
            :class="{ 'border-primary': size === 'small' }" @click="setSize('small')"
          >
            <Smartphone class="h-3.5 w-3.5" />
          </Button>
          <Button
            variant="outline" size="icon" title="中" aria-label="中尺寸卡片"
            :class="{ 'border-primary': size === 'medium' }" @click="setSize('medium')"
          >
            <Smartphone class="h-4 w-4" />
          </Button>
          <Button
            variant="outline" size="icon" title="大" aria-label="大尺寸卡片"
            :class="{ 'border-primary': size === 'large' }" @click="setSize('large')"
          >
            <Smartphone class="h-5 w-5" />
          </Button>
          <Button
            variant="outline" size="icon" title="监控铺满" aria-label="监控铺满模式"
            :class="{ 'border-primary text-primary': size === 'fit' }" @click="setSize('fit')"
          >
            <LayoutGrid class="h-4 w-4" />
          </Button>
          <Button
            variant="ghost" size="icon" :title="isPageFs ? '退出全屏' : '大屏全屏'" aria-label="大屏全屏"
            @click="togglePageFs"
          >
            <Minimize v-if="isPageFs" class="h-4 w-4" />
            <Maximize v-else class="h-4 w-4" />
          </Button>
        </div>
      </div>
    </header>

    <main
      class="w-full flex-1 min-h-0"
      :class="size === 'fit' ? 'p-1 overflow-hidden flex flex-col h-full' : 'px-4 py-6'"
    >
      <div v-if="loading && devices.length === 0" class="flex flex-wrap gap-4">
        <Skeleton v-for="i in 4" :key="i" class="h-72 w-44 rounded-xl" />
      </div>

      <Empty v-else-if="loadError && devices.length === 0">
        <EmptyHeader>
          <EmptyMedia variant="icon"><Smartphone /></EmptyMedia>
          <EmptyTitle>设备列表加载失败</EmptyTitle>
          <EmptyDescription>{{ loadError }}，请确认后端已启动。</EmptyDescription>
        </EmptyHeader>
        <Button variant="outline" @click="load()">重新加载</Button>
      </Empty>

      <Empty v-else-if="devices.length === 0">
        <EmptyHeader>
          <EmptyMedia variant="icon"><Smartphone /></EmptyMedia>
          <EmptyTitle>未检测到在线设备</EmptyTitle>
          <EmptyDescription>请用 adb connect IP:5555 连接设备</EmptyDescription>
        </EmptyHeader>
      </Empty>

      <div
        v-else
        ref="containerRef"
        class="w-full flex-1 min-h-0"
        :class="size === 'fit' ? 'grid gap-1' : 'flex flex-wrap items-start gap-4'"
        :style="gridStyle"
      >
        <DashTile
          v-for="d in devices" :key="d.serial" :device="d"
          :mode="size === 'fit' ? 'fit' : 'card'"
          :class="size === 'fit' ? 'w-full h-full min-h-0 min-w-0' : [tileWidth, 'shrink-0']"
        />
      </div>
    </main>
  </div>
</template>
