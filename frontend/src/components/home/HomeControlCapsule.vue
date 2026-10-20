<script setup lang="ts">
import {
  Grid3X3,
  LayoutGrid,
  Plus,
  RefreshCw,
  Search,
  Usb,
  Wifi,
} from 'lucide-vue-next'
import { Button } from '@/components/ui/button'
import { Input } from '@/components/ui/input'
import { Switch } from '@/components/ui/switch'

defineProps<{
  keyword: string
  sourceFilter: 'all' | 'auto' | 'manual'
  viewMode: 'deck' | 'compact'
  totalCount: number
  onlineCount: number
  isDevelopment?: boolean
  mockEnabled?: boolean
}>()

const emit = defineEmits<{
  (e: 'update:keyword', val: string): void
  (e: 'update:sourceFilter', val: 'all' | 'auto' | 'manual'): void
  (e: 'update:viewMode', val: 'deck' | 'compact'): void
  (e: 'refresh'): void
  (e: 'openAdd'): void
  (e: 'toggleMock', val: boolean): void
}>()
</script>

<template>
  <div class="relative z-20 flex flex-col items-center justify-between gap-4 rounded-3xl border border-slate-200/80 bg-white/75 p-3.5 shadow-lg backdrop-blur-2xl text-slate-900 dark:border-white/10 dark:bg-white/[0.04] dark:shadow-2xl dark:text-white md:flex-row md:px-5 transition-colors">
    <!-- 左侧设备状态标签（已去除动态图标，并命名为设备列表） -->
    <div class="flex items-center gap-3">
      <div>
        <div class="flex items-center gap-2.5">
          <span class="text-base font-bold tracking-tight text-slate-900 dark:text-white">设备列表</span>
          <span class="rounded-full bg-emerald-500/15 px-2.5 py-0.5 text-[10px] font-semibold text-emerald-700 dark:bg-emerald-500/20 dark:text-emerald-300 border border-emerald-500/30">
            {{ onlineCount }} 在线
          </span>
          <span v-if="mockEnabled" class="rounded-full bg-amber-500/15 px-2 py-0.5 text-[10px] font-medium text-amber-700 dark:bg-amber-500/20 dark:text-amber-300 border border-amber-500/30">
            Mock
          </span>
        </div>
        <p class="text-[11px] text-slate-500 dark:text-slate-400 mt-0.5">毫秒级超低时延投屏 · 多端无感互联</p>
      </div>
    </div>

    <!-- 中间：全息搜索条与快速分类过滤胶囊 -->
    <div class="flex flex-1 flex-wrap items-center justify-center gap-2 max-w-xl w-full">
      <div class="relative flex-1 min-w-[200px]">
        <Search class="absolute left-3 top-1/2 h-4 w-4 -translate-y-1/2 text-slate-400 dark:text-slate-500" />
        <Input
          :model-value="keyword"
          placeholder="搜索设备名称、型号或序列号..."
          aria-label="搜索设备"
          class="h-9 rounded-2xl border-slate-200/90 bg-slate-100/80 pl-9 text-xs text-slate-900 placeholder:text-slate-400 focus:border-violet-500 dark:border-white/10 dark:bg-black/30 dark:text-white dark:placeholder:text-slate-500 dark:focus:border-violet-400 transition-colors"
          @update:model-value="emit('update:keyword', $event as string)"
        />
      </div>

      <!-- 分类胶囊群 -->
      <div class="flex items-center rounded-2xl border border-slate-200/80 bg-slate-100/80 dark:border-white/10 dark:bg-black/20 p-1 text-xs">
        <button
          type="button"
          class="rounded-xl px-3 py-1 font-medium transition-all cursor-pointer"
          :class="sourceFilter === 'all'
            ? 'bg-white text-slate-950 shadow-xs font-semibold dark:bg-white/15 dark:text-white'
            : 'text-slate-600 hover:text-slate-950 dark:text-slate-400 dark:hover:text-white'"
          @click="emit('update:sourceFilter', 'all')"
        >
          全部
        </button>
        <button
          type="button"
          class="flex items-center gap-1 rounded-xl px-2.5 py-1 font-medium transition-all cursor-pointer"
          :class="sourceFilter === 'auto'
            ? 'bg-white text-slate-950 shadow-xs font-semibold dark:bg-white/15 dark:text-white'
            : 'text-slate-600 hover:text-slate-950 dark:text-slate-400 dark:hover:text-white'"
          @click="emit('update:sourceFilter', 'auto')"
        >
          <Usb class="h-3 w-3 text-cyan-600 dark:text-cyan-400" />
          直连
        </button>
        <button
          type="button"
          class="flex items-center gap-1 rounded-xl px-2.5 py-1 font-medium transition-all cursor-pointer"
          :class="sourceFilter === 'manual'
            ? 'bg-white text-slate-950 shadow-xs font-semibold dark:bg-white/15 dark:text-white'
            : 'text-slate-600 hover:text-slate-950 dark:text-slate-400 dark:hover:text-white'"
          @click="emit('update:sourceFilter', 'manual')"
        >
          <Wifi class="h-3 w-3 text-violet-600 dark:text-violet-400" />
          无线
        </button>
      </div>
    </div>

    <!-- 右侧：布局切换与操作按钮 -->
    <div class="flex items-center gap-2">
      <!-- 视图模式切换 -->
      <div class="flex items-center rounded-2xl border border-slate-200/80 bg-slate-100/80 dark:border-white/10 dark:bg-black/20 p-1 text-xs">
        <button
          type="button"
          class="rounded-xl p-1.5 transition-colors cursor-pointer"
          :class="viewMode === 'deck'
            ? 'bg-white text-slate-950 shadow-xs dark:bg-white/15 dark:text-white'
            : 'text-slate-600 hover:text-slate-950 dark:text-slate-400 dark:hover:text-white'"
          title="3D 全息甲板"
          aria-label="3D 全息甲板"
          @click="emit('update:viewMode', 'deck')"
        >
          <LayoutGrid class="h-4 w-4" />
        </button>
        <button
          type="button"
          class="rounded-xl p-1.5 transition-colors cursor-pointer"
          :class="viewMode === 'compact'
            ? 'bg-white text-slate-950 shadow-xs dark:bg-white/15 dark:text-white'
            : 'text-slate-600 hover:text-slate-950 dark:text-slate-400 dark:hover:text-white'"
          title="紧凑阵列"
          aria-label="紧凑阵列"
          @click="emit('update:viewMode', 'compact')"
        >
          <Grid3X3 class="h-4 w-4" />
        </button>
      </div>

      <Button
        variant="ghost"
        size="icon"
        class="h-9 w-9 rounded-2xl border border-slate-200/80 bg-white/70 text-slate-700 hover:bg-slate-100 hover:text-slate-950 dark:border-white/10 dark:bg-white/5 dark:text-slate-300 dark:hover:bg-white/10 dark:hover:text-white cursor-pointer transition-colors"
        title="刷新设备列表"
        aria-label="刷新设备列表"
        @click="emit('refresh')"
      >
        <RefreshCw class="h-4 w-4" />
      </Button>

      <Button
        size="sm"
        class="h-9 rounded-2xl bg-gradient-to-r from-violet-600 to-indigo-600 text-white font-medium hover:opacity-95 shadow-md px-3.5 text-xs cursor-pointer"
        @click="emit('openAdd')"
      >
        <Plus class="mr-1 h-3.5 w-3.5" />
        连接设备
      </Button>

      <!-- 开发模式 Mock 开关 -->
      <label v-if="isDevelopment" class="flex cursor-pointer items-center gap-1.5 text-xs text-slate-500 dark:text-slate-400 pl-1">
        <span>Mock</span>
        <Switch
          :model-value="mockEnabled"
          aria-label="Mock 数据开关"
          @update:model-value="(v) => emit('toggleMock', !!v)"
        />
      </label>
    </div>
  </div>
</template>
