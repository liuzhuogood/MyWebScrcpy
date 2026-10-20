<script setup lang="ts">
import { Eye, EyeOff, LayoutGrid, Maximize, Minimize, MonitorSmartphone, Smartphone } from 'lucide-vue-next'
import { computed, onMounted, onUnmounted, ref } from 'vue'
import { useElementSize, useFullscreen } from '@vueuse/core'
import DashTile from '@/components/dashboard/DashTile.vue'
import AppHeader from '@/components/layout/AppHeader.vue'
import { Badge } from '@/components/ui/badge'
import { Button } from '@/components/ui/button'
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
  if (hiddenSerials.value.includes(serial)) {
    hiddenSerials.value = hiddenSerials.value.filter(s => s !== serial)
  }
  else {
    hiddenSerials.value = [...hiddenSerials.value, serial]
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
  // 三档投屏屏幕宽度；高度由画面比例真实自适应撑开
  if (size.value === 'small')
    return 'w-48'
  if (size.value === 'medium')
    return 'w-80'
  return 'w-[32rem]'
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
    <AppHeader
      back
      back-to="/"
      title="大屏监控"
      :subtitle="countText"
      :icon="MonitorSmartphone"
    >
      <template #actions>
        <!-- 设备显隐选择 -->
        <Popover>
          <PopoverTrigger as-child>
            <Button
              variant="outline"
              size="sm"
              class="h-8 rounded-xl border border-slate-200/80 bg-slate-100/70 text-slate-800 hover:bg-slate-200/80 hover:text-slate-950 dark:border-white/15 dark:bg-white/10 dark:text-slate-100 dark:hover:bg-white/20 dark:hover:text-white text-xs font-medium px-2.5 gap-1.5 shadow-2xs backdrop-blur-md transition-all cursor-pointer"
              :class="{ '!border-primary !text-primary': hiddenSerials.length > 0 }"
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
          <PopoverContent align="end" class="w-72 p-3 space-y-2.5 rounded-2xl border-slate-200 bg-white/95 dark:border-white/10 dark:bg-slate-900/95 backdrop-blur-xl">
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

            <div v-else class="max-h-72 space-y-1 overflow-y-auto pr-1">
              <div
                v-for="d in devices"
                :key="d.serial"
                class="flex cursor-pointer items-center justify-between gap-2.5 rounded-md px-2.5 py-1.5 text-xs transition-colors select-none"
                :class="isDeviceVisible(d.serial) ? 'bg-muted/40 hover:bg-muted/70 text-foreground' : 'text-muted-foreground/60 hover:bg-muted/30'"
                :title="isDeviceVisible(d.serial) ? '点击在大屏中隐藏' : '点击在大屏中显示'"
                @click="toggleDeviceVisibility(d.serial)"
              >
                <div class="flex min-w-0 items-center gap-2">
                  <span
                    class="inline-block h-2 w-2 shrink-0 rounded-full"
                    :class="d.state === 'device' ? 'bg-emerald-500' : 'bg-amber-500'"
                  />
                  <div class="min-w-0">
                    <div class="truncate font-medium leading-tight">
                      {{ deviceStore.displayName(d) }}
                    </div>
                    <div class="truncate font-mono text-[10px] text-muted-foreground/70">
                      {{ d.serial }}
                    </div>
                  </div>
                </div>

                <!-- 显隐状态图标与提示：显示时高亮 Eye 图标，隐藏时暗色 EyeOff 图标 -->
                <div class="shrink-0">
                  <span
                    v-if="isDeviceVisible(d.serial)"
                    class="inline-flex items-center gap-1 rounded bg-primary/10 px-1.5 py-0.5 text-[11px] font-medium text-primary"
                  >
                    <Eye class="h-3.5 w-3.5" />
                    显示
                  </span>
                  <span
                    v-else
                    class="inline-flex items-center gap-1 rounded bg-muted px-1.5 py-0.5 text-[11px] text-muted-foreground/70"
                  >
                    <EyeOff class="h-3.5 w-3.5" />
                    隐藏
                  </span>
                </div>
              </div>
            </div>
          </PopoverContent>
        </Popover>

        <!-- 尺寸与全屏组合 -->
        <div class="flex items-center gap-1 rounded-xl border border-slate-200/80 bg-slate-100/60 p-0.5 dark:border-white/10 dark:bg-white/5 backdrop-blur-md" role="group" aria-label="卡片尺寸">
          <Button
            variant="ghost" size="icon" title="小" aria-label="小尺寸卡片"
            class="h-7 w-7 rounded-lg text-slate-600 dark:text-slate-400 hover:text-slate-900 dark:hover:text-white cursor-pointer"
            :class="{ '!bg-white dark:!bg-white/15 !text-slate-900 dark:!text-white shadow-2xs': size === 'small' }"
            @click="setSize('small')"
          >
            <Smartphone class="h-3.5 w-3.5" />
          </Button>
          <Button
            variant="ghost" size="icon" title="中" aria-label="中尺寸卡片"
            class="h-7 w-7 rounded-lg text-slate-600 dark:text-slate-400 hover:text-slate-900 dark:hover:text-white cursor-pointer"
            :class="{ '!bg-white dark:!bg-white/15 !text-slate-900 dark:!text-white shadow-2xs': size === 'medium' }"
            @click="setSize('medium')"
          >
            <Smartphone class="h-4 w-4" />
          </Button>
          <Button
            variant="ghost" size="icon" title="大" aria-label="大尺寸卡片"
            class="h-7 w-7 rounded-lg text-slate-600 dark:text-slate-400 hover:text-slate-900 dark:hover:text-white cursor-pointer"
            :class="{ '!bg-white dark:!bg-white/15 !text-slate-900 dark:!text-white shadow-2xs': size === 'large' }"
            @click="setSize('large')"
          >
            <Smartphone class="h-5 w-5" />
          </Button>
          <Button
            variant="ghost" size="icon" title="监控铺满" aria-label="监控铺满模式"
            class="h-7 w-7 rounded-lg text-slate-600 dark:text-slate-400 hover:text-slate-900 dark:hover:text-white cursor-pointer"
            :class="{ '!bg-white dark:!bg-white/15 !text-primary shadow-2xs': size === 'fit' }"
            @click="setSize('fit')"
          >
            <LayoutGrid class="h-4 w-4" />
          </Button>
          <Button
            variant="ghost" size="icon" :title="isPageFs ? '退出全屏' : '大屏全屏'" aria-label="大屏全屏"
            class="h-7 w-7 rounded-lg text-slate-600 dark:text-slate-400 hover:text-slate-900 dark:hover:text-white cursor-pointer"
            @click="togglePageFs"
          >
            <Minimize v-if="isPageFs" class="h-4 w-4" />
            <Maximize v-else class="h-4 w-4" />
          </Button>
        </div>
      </template>
    </AppHeader>

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
