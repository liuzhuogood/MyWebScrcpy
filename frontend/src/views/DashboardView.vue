<script setup lang="ts">
import { ArrowLeft, Eye, EyeOff, LayoutGrid, Maximize, Minimize, MonitorSmartphone, Smartphone } from 'lucide-vue-next'
import { computed, onMounted, onUnmounted, ref } from 'vue'
import { RouterLink } from 'vue-router'
import { useElementSize, useFullscreen } from '@vueuse/core'
import DashTile from '@/components/dashboard/DashTile.vue'
import { Badge } from '@/components/ui/badge'
import { Button } from '@/components/ui/button'
import { Checkbox } from '@/components/ui/checkbox'
import { Empty, EmptyDescription, EmptyHeader, EmptyMedia, EmptyTitle } from '@/components/ui/empty'
import { Popover, PopoverContent, PopoverTrigger } from '@/components/ui/popover'
import { Skeleton } from '@/components/ui/skeleton'
import { fetchDevices, type Device } from '@/api/devices'
import { useDeviceStore } from '@/stores/devices'
import { computeMonitorGrid } from '@/utils/monitorGrid'

type DashSize = 'small' | 'medium' | 'large' | 'fit'
const SIZE_KEY = 'dash_size'
const HIDDEN_KEY = 'dash_hidden_devices'
const POLL_MS = 5000

const deviceStore = useDeviceStore()
const containerRef = ref<HTMLElement | null>(null)
const { width: containerWidth, height: containerHeight } = useElementSize(containerRef)
const { isFullscreen: isPageFs, toggle: togglePageFs } = useFullscreen()

const devices = ref<Device[]>([])
const loading = ref(true)
const loadError = ref('')
const size = ref<DashSize>((localStorage.getItem(SIZE_KEY) as DashSize) || 'small')
let timer: number | undefined

function loadHiddenSerials(): string[] {
  try {
    const raw = localStorage.getItem(HIDDEN_KEY)
    if (!raw)
      return []
    const parsed = JSON.parse(raw)
    return Array.isArray(parsed) ? parsed : []
  }
  catch {
    return []
  }
}

const hiddenSerials = ref<string[]>(loadHiddenSerials())

function saveHiddenSerials() {
  localStorage.setItem(HIDDEN_KEY, JSON.stringify(hiddenSerials.value))
}

function isDeviceVisible(serial: string): boolean {
  return !hiddenSerials.value.includes(serial)
}

function toggleDeviceVisibility(serial: string) {
  const idx = hiddenSerials.value.indexOf(serial)
  if (idx >= 0) {
    hiddenSerials.value.splice(idx, 1)
  }
  else {
    hiddenSerials.value.push(serial)
  }
  saveHiddenSerials()
}

function showAllDevices() {
  hiddenSerials.value = []
  saveHiddenSerials()
}

function hideAllDevices() {
  hiddenSerials.value = devices.value.map(d => d.serial)
  saveHiddenSerials()
}

const visibleDevices = computed(() => {
  return devices.value.filter(d => !hiddenSerials.value.includes(d.serial))
})

const countText = computed(() => {
  if (loading.value && devices.value.length === 0)
    return '加载中...'
  if (loadError.value && devices.value.length === 0)
    return '加载失败'
  if (devices.value.length === 0)
    return '无设备'
  if (hiddenSerials.value.length > 0)
    return `${visibleDevices.value.length}/${devices.value.length} 台设备监控中`
  return `${devices.value.length} 台设备监控中`
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
  if (size.value !== 'fit' || visibleDevices.value.length === 0)
    return {}

  const N = visibleDevices.value.length
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
        <div class="ml-auto flex items-center gap-2" role="toolbar" aria-label="大屏控制">
          <!-- 设备显隐选择 -->
          <Popover>
            <PopoverTrigger as-child>
              <Button
                variant="outline"
                size="sm"
                class="h-8 gap-1.5 px-2.5 text-xs font-normal"
                :class="{ 'border-primary text-primary': hiddenSerials.length > 0 }"
                title="选择显示与隐藏设备"
              >
                <Eye class="h-3.5 w-3.5" />
                <span>设备显隐</span>
                <Badge
                  v-if="hiddenSerials.length > 0"
                  variant="secondary"
                  class="h-4 px-1 text-[10px] leading-none"
                >
                  {{ visibleDevices.length }}/{{ devices.length }}
                </Badge>
              </Button>
            </PopoverTrigger>
            <PopoverContent align="end" class="w-72 p-3 space-y-2.5">
              <div class="flex items-center justify-between border-b pb-2">
                <span class="text-xs font-medium">设备显示设置</span>
                <div class="flex items-center gap-1">
                  <Button
                    variant="ghost"
                    size="sm"
                    class="h-6 px-1.5 text-[11px]"
                    :disabled="hiddenSerials.length === 0"
                    @click="showAllDevices"
                  >
                    全显
                  </Button>
                  <Button
                    variant="ghost"
                    size="sm"
                    class="h-6 px-1.5 text-[11px]"
                    :disabled="visibleDevices.length === 0"
                    @click="hideAllDevices"
                  >
                    全隐
                  </Button>
                </div>
              </div>

              <div v-if="devices.length === 0" class="py-4 text-center text-xs text-muted-foreground">
                暂无在线设备
              </div>

              <div v-else class="max-h-64 space-y-1 overflow-y-auto pr-1">
                <div
                  v-for="d in devices"
                  :key="d.serial"
                  class="flex cursor-pointer items-center gap-2 rounded px-2 py-1.5 hover:bg-muted/50 transition-colors select-none"
                  @click="toggleDeviceVisibility(d.serial)"
                >
                  <Checkbox
                    :checked="isDeviceVisible(d.serial)"
                    class="pointer-events-none"
                  />
                  <div class="min-w-0 flex-1">
                    <div class="truncate text-xs font-medium" :title="d.serial">
                      {{ deviceStore.displayName(d) }}
                    </div>
                    <div class="truncate font-mono text-[10px] text-muted-foreground">
                      {{ d.serial }}
                    </div>
                  </div>
                </div>
              </div>
            </PopoverContent>
          </Popover>

          <div class="h-4 w-px bg-border" />

          <div class="flex items-center gap-1" role="group" aria-label="卡片尺寸">
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
        v-else-if="visibleDevices.length > 0"
        ref="containerRef"
        class="w-full flex-1 min-h-0"
        :class="size === 'fit' ? 'grid gap-1' : 'flex flex-wrap items-start gap-4'"
        :style="gridStyle"
      >
        <DashTile
          v-for="d in visibleDevices" :key="d.serial" :device="d"
          :mode="size === 'fit' ? 'fit' : 'card'"
          :class="size === 'fit' ? 'w-full h-full min-h-0 min-w-0' : [tileWidth, 'shrink-0']"
        />
      </div>

      <Empty v-else>
        <EmptyHeader>
          <EmptyMedia variant="icon"><EyeOff /></EmptyMedia>
          <EmptyTitle>所有设备已隐藏</EmptyTitle>
          <EmptyDescription>已隐藏所有在线设备，请在顶栏「设备显隐」中勾选要显示的设备</EmptyDescription>
        </EmptyHeader>
        <Button variant="outline" size="sm" @click="showAllDevices">显示全部设备</Button>
      </Empty>
    </main>
  </div>
</template>
