<script setup lang="ts">
import type { Device } from '@/api/devices'
import type { DashSize } from '@/composables/useHomePhoneView'
import type { BatchItemResult, BatchSummary } from '@/composables/useDeviceBatch'
import type { HomeViewMode } from '@/composables/usePlayerSettings'
import { computed, nextTick, ref } from 'vue'
import {
  ArrowLeft,
  Home,
  Layers,
  LayoutGrid,
  Maximize,
  Minimize,
  PanelRightClose,
  Plus,
  Radio,
  RefreshCw,
  Search,
  SlidersHorizontal,
  Smartphone,
  X,
} from 'lucide-vue-next'
import { Badge } from '@/components/ui/badge'
import { Button } from '@/components/ui/button'
import { Input } from '@/components/ui/input'
import { Slider } from '@/components/ui/slider'
import {
  HOME_PHONE_SCALE_MAX,
  HOME_PHONE_SCALE_MIN,
  HOME_QUALITY_MAX,
  HOME_QUALITY_MIN,
  usePlayerSettings,
} from '@/composables/usePlayerSettings'

const props = defineProps<{
  keyword: string
  isOpen: boolean
  viewMode: HomeViewMode
  devices: Device[]
  hiddenSerials: string[]
  selectedSerials: string[]
  displayName: (device: Device) => string
  onlineCount: number
  isAllSelected: boolean
  hasSelectable: boolean
  batchRunning: boolean
  currentAction: string
  batchResults: BatchItemResult[]
  batchSummary: BatchSummary
  dashSize: DashSize
  isPageFullscreen: boolean
  isRefreshing?: boolean
  isSyncActive?: boolean
}>()

const emit = defineEmits<{
  (e: 'update:isOpen', value: boolean): void
  (e: 'update:keyword', value: string): void
  (e: 'update:view-mode', value: HomeViewMode): void
  (e: 'toggleHide', serial: string): void
  (e: 'showAll'): void
  (e: 'showOnlySelected'): void
  (e: 'selectAll'): void
  (e: 'clearSelection'): void
  (e: 'triggerAction', action: 'back' | 'home' | 'app_switch'): void
  (e: 'toggleSync'): void
  (e: 'cancelBatch'): void
  (e: 'clearResults'): void
  (e: 'refresh'): void
  (e: 'openAdd'): void
  (e: 'togglePageFullscreen'): void
}>()

const settings = usePlayerSettings()
const { homePhoneScale, homeQuality } = settings
const searchInput = ref<HTMLInputElement | null>(null)
const cardWidth = computed(() => Math.round(270 * homePhoneScale.value / 100))

const numberedDevices = computed(() => {
  return [...props.devices]
    .sort((a, b) => a.serial.localeCompare(b.serial))
    .map((device, index) => {
      const label = index + 1 < 10 ? `0${index + 1}` : String(index + 1)
      return { device, label, serial: device.serial }
    })
})

const hiddenSet = computed(() => new Set(props.hiddenSerials))
const selectedSet = computed(() => new Set(props.selectedSerials))
const visibleCount = computed(() => props.devices.filter(d => !hiddenSet.value.has(d.serial)).length)
const hasHidden = computed(() => props.devices.some(d => hiddenSet.value.has(d.serial)))

const qualityMarks = [
  { label: '720P', value: 60 },
  { label: '1080P', value: 80 },
  { label: '100%', value: 100 },
]

const { homeBitrate: bitrateValue } = settings

async function expandAndFocusSearch() {
  emit('update:isOpen', true)
  await nextTick()
  searchInput.value?.focus()
}

function handleRangeUpdate(value: number | Array<number> | undefined, apply: (value: number) => void) {
  const next = Array.isArray(value) ? value[0] : value
  if (typeof next === 'number')
    apply(next)
}
</script>

<template>
  <div class="w-full lg:w-80 shrink-0 order-first lg:order-last">
    <aside
      v-if="isOpen"
      aria-label="操作"
      class="flex flex-col max-h-[calc(100vh-7rem)] overflow-hidden rounded-3xl border border-slate-200/90 bg-white/95 shadow-xl backdrop-blur-3xl dark:border-white/15 dark:bg-[#0e121d]/95"
    >
      <header class="flex shrink-0 items-center justify-between border-b border-slate-200/80 px-4 py-3 dark:border-white/10">
        <div class="flex items-center gap-2">
          <SlidersHorizontal class="h-4 w-4 text-violet-600 dark:text-violet-400" />
          <h2 class="text-sm font-bold text-slate-900 dark:text-white">操作</h2>
        </div>
        <div class="flex items-center gap-0.5">
          <button
            type="button"
            class="rounded-xl p-1.5 text-slate-400 transition-colors hover:bg-slate-100 hover:text-slate-900 dark:hover:bg-white/10 dark:hover:text-white"
            :disabled="isRefreshing"
            title="刷新设备列表"
            aria-label="刷新设备列表"
            @click="emit('refresh')"
          >
            <RefreshCw class="h-3.5 w-3.5" :class="{ 'animate-spin': isRefreshing }" />
          </button>
          <button
            type="button"
            class="rounded-xl p-1.5 text-slate-400 transition-colors hover:bg-slate-100 hover:text-slate-900 dark:hover:bg-white/10 dark:hover:text-white"
            title="连接设备"
            aria-label="连接设备"
            @click="emit('openAdd')"
          >
            <Plus class="h-3.5 w-3.5" />
          </button>
          <button
            type="button"
            class="rounded-xl p-1.5 text-slate-400 transition-colors hover:bg-slate-100 hover:text-slate-900 dark:hover:bg-white/10 dark:hover:text-white"
            :title="isPageFullscreen ? '退出全屏' : '全屏'"
            :aria-label="isPageFullscreen ? '退出全屏' : '全屏'"
            @click="emit('togglePageFullscreen')"
          >
            <component :is="isPageFullscreen ? Minimize : Maximize" class="h-3.5 w-3.5" />
          </button>
          <button
            type="button"
            class="ml-1 rounded-xl p-1.5 text-slate-400 transition-colors hover:bg-slate-100 hover:text-slate-900 dark:hover:bg-white/10 dark:hover:text-white"
            title="收起面板"
            aria-label="收起操作面板"
            @click="emit('update:isOpen', false)"
          >
            <PanelRightClose class="h-4 w-4" />
          </button>
        </div>
      </header>

      <div class="flex-1 space-y-4 overflow-y-auto overscroll-contain p-4">
        <section class="space-y-3 border-b border-slate-200/80 pb-4 dark:border-white/10">
          <div class="relative">
            <Search class="absolute left-2.5 top-1/2 h-3.5 w-3.5 -translate-y-1/2 text-slate-400" />
            <Input
              id="home-action-search"
              ref="searchInput"
              :model-value="keyword"
              placeholder="搜索设备名称、型号或序列号"
              aria-label="搜索设备"
              autocomplete="off"
              class="h-8 rounded-xl pl-8 pr-7 text-xs"
              @update:model-value="emit('update:keyword', String($event))"
            />
            <button
              v-if="keyword"
              type="button"
              class="absolute right-2 top-1/2 -translate-y-1/2 text-slate-400 hover:text-slate-700 dark:hover:text-white"
              title="清除搜索"
              aria-label="清除搜索"
              @click="emit('update:keyword', '')"
            >
              <X class="h-3 w-3" />
            </button>
          </div>

          <div class="flex items-center justify-between">
            <span class="text-xs text-slate-600 dark:text-slate-400">视图模式</span>
            <div class="flex rounded-xl border border-slate-200/80 bg-slate-100/80 p-0.5 dark:border-white/10 dark:bg-black/20">
              <button
                v-for="mode in [{ key: 'deck', icon: LayoutGrid, title: '手机卡片视图' }, { key: 'phone', icon: Smartphone, title: '手机视图' }] as const"
                :key="mode.key"
                type="button"
                class="flex h-7 items-center gap-1 rounded-lg px-2.5 text-xs font-medium transition-all"
                :class="viewMode === mode.key
                  ? 'bg-white font-semibold text-slate-950 shadow-xs dark:bg-white/15 dark:text-white'
                  : 'cursor-pointer text-slate-500 hover:text-slate-950 dark:text-slate-400 dark:hover:text-white'"
                :title="mode.title"
                :aria-label="mode.title"
                :aria-pressed="viewMode === mode.key"
                @click="emit('update:view-mode', mode.key)"
              >
                <component :is="mode.icon" class="h-3 w-3" />
                <span>{{ mode.key === 'deck' ? '卡片' : '手机' }}</span>
              </button>
            </div>
          </div>

          <div class="space-y-1.5">
            <div class="flex items-center justify-between text-xs">
              <span class="font-medium text-slate-700 dark:text-slate-300">手机大小</span>
              <span class="font-mono text-[11px] font-semibold text-violet-600 dark:text-violet-400">{{ homePhoneScale }}% · {{ cardWidth }}px</span>
            </div>
            <Slider
              :model-value="[homePhoneScale]"
              :min="HOME_PHONE_SCALE_MIN"
              :max="HOME_PHONE_SCALE_MAX"
              :step="5"
              aria-label="手机大小"
              @update:model-value="handleRangeUpdate($event, value => homePhoneScale = value)"
            />
            <div class="flex justify-between text-[10px] text-slate-400">
              <button type="button" class="cursor-pointer hover:text-violet-600" @click="homePhoneScale = 75">紧凑</button>
              <button type="button" class="cursor-pointer hover:text-violet-600" @click="homePhoneScale = 100">标准</button>
              <button type="button" class="cursor-pointer hover:text-violet-600" @click="homePhoneScale = 130">超大</button>
            </div>
          </div>

          <div class="space-y-1.5">
            <div class="flex items-center justify-between text-xs">
              <span class="font-medium text-slate-700 dark:text-slate-300">投屏码率</span>
              <span class="font-mono text-[11px] font-semibold text-violet-600 dark:text-violet-400">{{ bitrateValue }} Mbps</span>
            </div>
            <Slider
              :model-value="[bitrateValue]"
              :min="2"
              :max="16"
              :step="1"
              aria-label="投屏码率"
              @update:model-value="handleRangeUpdate($event, value => bitrateValue = value)"
            />
            <div class="flex justify-between text-[10px] text-slate-400">
              <button type="button" class="cursor-pointer hover:text-violet-600" @click="bitrateValue = 2">流畅</button>
              <button type="button" class="cursor-pointer hover:text-violet-600" @click="bitrateValue = 8">超清</button>
              <button type="button" class="cursor-pointer hover:text-violet-600" @click="bitrateValue = 16">极致</button>
            </div>
          </div>

          <div class="space-y-1.5">
            <div class="flex items-center justify-between text-xs">
              <span class="font-medium text-slate-700 dark:text-slate-300">投屏质量</span>
              <span class="font-mono text-[11px] font-semibold text-violet-600 dark:text-violet-400">{{ homeQuality }}%</span>
            </div>
            <Slider
              :model-value="[homeQuality]"
              :min="HOME_QUALITY_MIN"
              :max="HOME_QUALITY_MAX"
              :step="5"
              aria-label="投屏质量"
              @update:model-value="handleRangeUpdate($event, value => homeQuality = value)"
            />
            <div class="flex justify-between text-[10px] text-slate-400">
              <button
                v-for="mark in qualityMarks"
                :key="mark.value"
                type="button"
                class="cursor-pointer hover:text-violet-600"
                @click="homeQuality = mark.value"
              >
                {{ mark.label }}
              </button>
            </div>
          </div>
        </section>

        <section class="space-y-2">
          <div class="flex items-center justify-between">
            <div class="flex items-center gap-2">
              <h3 class="text-[11px] font-bold uppercase tracking-wider text-slate-400">设备显隐</h3>
              <Badge variant="secondary" class="h-4.5 px-1.5 font-mono text-[10px] leading-none">
                {{ visibleCount }}/{{ devices.length }}
              </Badge>
              <Badge variant="outline" class="h-4.5 px-1.5 font-mono text-[10px] leading-none">
                {{ onlineCount }} 在线
            </Badge>
            </div>
            <div class="flex items-center gap-1.5 text-[11px]">
              <button type="button" class="cursor-pointer text-violet-600 hover:underline" :disabled="!hasHidden" @click="emit('showAll')">全显</button>
              <span class="text-slate-300">|</span>
              <button type="button" class="cursor-pointer text-violet-600 hover:underline" :disabled="selectedSerials.length === 0" @click="emit('showOnlySelected')">只显已选</button>
            </div>
          </div>
          <div class="grid max-h-52 grid-cols-6 gap-1.5 overflow-y-auto rounded-2xl border border-slate-200/80 bg-slate-50/50 p-1 dark:border-white/10 dark:bg-black/20">
            <button
              v-for="item in numberedDevices"
              :key="item.serial"
              type="button"
              class="relative flex h-8 items-center justify-center rounded-lg border font-mono text-xs font-bold transition-all"
              :class="hiddenSet.has(item.serial)
                ? 'cursor-pointer border-dashed border-slate-300 bg-slate-200/40 text-slate-400 line-through dark:border-white/10 dark:bg-white/5'
                : selectedSet.has(item.serial)
                  ? 'cursor-pointer border-2 border-violet-500 bg-violet-600 text-white'
                  : 'cursor-pointer border-slate-300 bg-white text-slate-800 hover:border-violet-400 dark:border-white/20 dark:bg-white/10 dark:text-slate-100'"
              :title="`${item.label} · ${displayName(item.device)} · ${item.device.state === 'device' ? '在线' : item.device.state}`"
              :aria-label="`切换 ${item.label} ${displayName(item.device)} 显示状态`"
              @click="emit('toggleHide', item.serial)"
            >
              {{ item.label }}
              <span class="absolute right-1 top-1 h-1.5 w-1.5 rounded-full" :class="item.device.state === 'device' ? 'bg-emerald-400' : 'bg-amber-400'" />
            </button>
          </div>
        </section>

        <section class="space-y-2 border-t border-slate-200/80 pt-4 dark:border-white/10">
          <div class="flex items-center justify-between text-xs">
            <h3 class="font-bold uppercase tracking-wider text-slate-400">批量控制</h3>
            <span class="font-semibold text-violet-600 dark:text-violet-400">已选 {{ selectedSerials.length }} 台</span>
          </div>
          <div class="grid grid-cols-2 gap-2">
            <Button variant="outline" size="sm" class="h-7 rounded-xl text-xs" :disabled="!hasSelectable || batchRunning" @click="emit('selectAll')">
              {{ isAllSelected ? '取消全选' : '全选在线' }}
            </Button>
            <Button variant="outline" size="sm" class="h-7 rounded-xl text-xs" :disabled="selectedSerials.length === 0 || batchRunning" @click="emit('clearSelection')">
              清空
            </Button>
          </div>
          <div class="grid grid-cols-4 gap-2">
            <Button variant="secondary" size="sm" class="h-8.5 rounded-xl" :disabled="selectedSerials.length === 0 || batchRunning" title="返回" aria-label="返回" @click="emit('triggerAction', 'back')">
              <ArrowLeft class="h-4 w-4" />
            </Button>
            <Button variant="secondary" size="sm" class="h-8.5 rounded-xl" :disabled="selectedSerials.length === 0 || batchRunning" title="主页" aria-label="主页" @click="emit('triggerAction', 'home')">
              <Home class="h-4 w-4" />
            </Button>
            <Button variant="secondary" size="sm" class="h-8.5 rounded-xl" :disabled="selectedSerials.length === 0 || batchRunning" title="多任务" aria-label="多任务" @click="emit('triggerAction', 'app_switch')">
              <Layers class="h-4 w-4" />
            </Button>
            <Button
              size="sm"
              :variant="isSyncActive ? 'default' : 'secondary'"
              class="h-8.5 rounded-xl cursor-pointer transition-all"
              :class="{ 'bg-violet-600 text-white hover:bg-violet-700 shadow-xs ring-1 ring-violet-500': isSyncActive }"
              :disabled="selectedSerials.length === 0 || batchRunning"
              title="同步操作：通过一台手机，把对它的操作同步到所有已选设备"
              aria-label="同步操作"
              @click="emit('toggleSync')"
            >
              <Radio class="h-4 w-4" :class="{ 'animate-pulse': isSyncActive }" />
            </Button>
          </div>
          <div v-if="batchRunning" class="flex items-center justify-between rounded-xl bg-violet-50 p-2 text-xs text-violet-700 dark:bg-violet-950/30 dark:text-violet-300">
            <span>正在执行 {{ currentAction }}</span>
            <Button variant="destructive" size="sm" class="h-6 rounded-lg px-2 text-[11px]" @click="emit('cancelBatch')">取消</Button>
          </div>
          <div v-else-if="batchResults.length" class="flex items-center justify-between text-[10px] text-slate-500">
            <span>成功 {{ batchSummary.success }} / 失败 {{ batchSummary.failed }} / 待确认 {{ batchSummary.unknown }}</span>
            <button type="button" class="cursor-pointer hover:text-slate-900 dark:hover:text-white" @click="emit('clearResults')">清除</button>
          </div>
        </section>
      </div>
    </aside>

    <aside
      v-else
      aria-label="操作（已收起）"
      class="flex w-13 flex-col items-center gap-4 rounded-3xl border border-slate-200/90 bg-white/95 py-3.5 shadow-lg backdrop-blur-3xl dark:border-white/15 dark:bg-[#0e121d]/95"
    >
      <button type="button" class="cursor-pointer rounded-2xl bg-violet-50 p-2 text-violet-600 transition-colors hover:bg-violet-100 dark:bg-violet-500/10 dark:text-violet-400 dark:hover:bg-violet-500/20" title="展开操作面板" aria-label="展开操作面板" @click="emit('update:isOpen', true)">
        <PanelRightClose class="h-5 w-5 rotate-180" />
      </button>
      <button type="button" class="cursor-pointer rounded-xl p-2 text-slate-400 hover:bg-slate-100 hover:text-slate-900 dark:hover:bg-white/10 dark:hover:text-white" title="搜索设备" aria-label="搜索设备" @click="expandAndFocusSearch">
        <Search class="h-4 w-4" />
      </button>
      <button type="button" class="cursor-pointer rounded-xl p-2 text-slate-400 hover:bg-slate-100 hover:text-slate-900 dark:hover:bg-white/10 dark:hover:text-white" title="刷新设备列表" aria-label="刷新设备列表" @click="emit('refresh')">
        <RefreshCw class="h-4 w-4" :class="{ 'animate-spin': isRefreshing }" />
      </button>
      <button type="button" class="cursor-pointer rounded-xl p-2 text-slate-400 hover:bg-slate-100 hover:text-slate-900 dark:hover:bg-white/10 dark:hover:text-white" :title="`切换视图（当前：${viewMode === 'deck' ? '卡片' : '手机'}）`" @click="emit('update:view-mode', viewMode === 'deck' ? 'phone' : 'deck')">
        <component :is="viewMode === 'deck' ? LayoutGrid : Smartphone" class="h-4 w-4 text-violet-600 dark:text-violet-400" />
      </button>
      <div v-if="selectedSerials.length" class="flex cursor-pointer flex-col items-center gap-1" title="已选设备" @click="emit('update:isOpen', true)">
        <span class="flex h-6 w-6 items-center justify-center rounded-full bg-violet-600 text-[10px] font-bold text-white">{{ selectedSerials.length }}</span>
        <span class="text-[9px] text-slate-400">已选</span>
      </div>
      <div class="my-1 h-px w-6 bg-slate-200 dark:bg-white/10" />
      <div class="cursor-pointer py-1 text-xs font-bold tracking-widest text-slate-400 hover:text-violet-600 dark:hover:text-violet-400" title="展开操作面板" @click="emit('update:isOpen', true)">
        <span>操</span>
        <span class="mt-1 block">作</span>
      </div>
    </aside>
  </div>
</template>
