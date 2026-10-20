<script setup lang="ts">
import {
  Grid3X3,
  LayoutGrid,
  Plus,
  RefreshCw,
  Search,
  Sparkles,
  Usb,
  Wifi,
} from 'lucide-vue-next'
import { Button } from '@/components/ui/button'
import { Input } from '@/components/ui/input'

defineProps<{
  keyword: string
  activeFilter: string
  viewMode: 'deck' | 'grid' | 'compact'
  totalCount: number
  onlineCount: number
}>()

const emit = defineEmits<{
  (e: 'update:keyword', val: string): void
  (e: 'update:activeFilter', val: string): void
  (e: 'update:viewMode', val: 'deck' | 'grid' | 'compact'): void
  (e: 'refresh'): void
  (e: 'openAdd'): void
}>()
</script>

<template>
  <div class="relative z-20 flex flex-col items-center justify-between gap-4 rounded-3xl border border-white/10 bg-white/[0.04] p-3 shadow-2xl backdrop-blur-2xl md:flex-row md:px-5">
    <!-- 左侧品牌与全局状态胶囊 -->
    <div class="flex items-center gap-3">
      <div class="flex h-10 w-10 items-center justify-center rounded-2xl bg-gradient-to-tr from-violet-600 to-cyan-500 shadow-lg shadow-violet-500/25">
        <Sparkles class="h-5 w-5 text-white animate-pulse" />
      </div>
      <div>
        <div class="flex items-center gap-2">
          <span class="text-sm font-semibold tracking-tight text-white">空间设备矩阵</span>
          <span class="rounded-full bg-emerald-500/20 px-2 py-0.5 text-[10px] font-medium text-emerald-300 border border-emerald-500/30">
            {{ onlineCount }} 在线
          </span>
        </div>
        <p class="text-[11px] text-slate-400">Vision 空间计算 · 毫秒级低时延投屏</p>
      </div>
    </div>

    <!-- 中间：全息搜索条与快速分类过滤胶囊 -->
    <div class="flex flex-1 flex-wrap items-center justify-center gap-2 max-w-xl w-full">
      <div class="relative flex-1 min-w-[200px]">
        <Search class="absolute left-3 top-1/2 h-4 w-4 -translate-y-1/2 text-slate-400" />
        <Input
          :model-value="keyword"
          placeholder="搜索设备名称、型号或序列号..."
          class="h-9 rounded-2xl border-white/10 bg-black/30 pl-9 text-xs text-white placeholder:text-slate-500 focus:border-violet-400"
          @update:model-value="emit('update:keyword', $event as string)"
        />
      </div>

      <!-- 分类胶囊群 -->
      <div class="flex items-center rounded-2xl border border-white/10 bg-black/20 p-1 text-xs">
        <button
          class="rounded-xl px-3 py-1 font-medium transition-all"
          :class="activeFilter === 'all' ? 'bg-white/15 text-white shadow-sm' : 'text-slate-400 hover:text-white'"
          @click="emit('update:activeFilter', 'all')"
        >
          全部
        </button>
        <button
          class="flex items-center gap-1 rounded-xl px-2.5 py-1 font-medium transition-all"
          :class="activeFilter === 'usb' ? 'bg-white/15 text-white shadow-sm' : 'text-slate-400 hover:text-white'"
          @click="emit('update:activeFilter', 'usb')"
        >
          <Usb class="h-3 w-3 text-cyan-400" />
          直连
        </button>
        <button
          class="flex items-center gap-1 rounded-xl px-2.5 py-1 font-medium transition-all"
          :class="activeFilter === 'wifi' ? 'bg-white/15 text-white shadow-sm' : 'text-slate-400 hover:text-white'"
          @click="emit('update:activeFilter', 'wifi')"
        >
          <Wifi class="h-3 w-3 text-violet-400" />
          无线
        </button>
      </div>
    </div>

    <!-- 右侧：布局切换与操作按钮 -->
    <div class="flex items-center gap-2">
      <!-- 视图模式切换 -->
      <div class="flex items-center rounded-2xl border border-white/10 bg-black/20 p-1 text-xs">
        <button
          class="rounded-xl p-1.5 transition-colors"
          :class="viewMode === 'deck' ? 'bg-white/15 text-white' : 'text-slate-400 hover:text-white'"
          title="3D 全息甲板"
          @click="emit('update:viewMode', 'deck')"
        >
          <LayoutGrid class="h-4 w-4" />
        </button>
        <button
          class="rounded-xl p-1.5 transition-colors"
          :class="viewMode === 'compact' ? 'bg-white/15 text-white' : 'text-slate-400 hover:text-white'"
          title="紧凑阵列"
          @click="emit('update:viewMode', 'compact')"
        >
          <Grid3X3 class="h-4 w-4" />
        </button>
      </div>

      <Button
        variant="ghost"
        size="icon"
        class="h-9 w-9 rounded-2xl border border-white/10 bg-white/5 text-slate-300 hover:bg-white/10 hover:text-white"
        title="刷新拓扑"
        @click="emit('refresh')"
      >
        <RefreshCw class="h-4 w-4" />
      </Button>

      <Button
        size="sm"
        class="h-9 rounded-2xl bg-white text-slate-950 font-medium hover:bg-white/90 shadow-lg px-3.5 text-xs"
        @click="emit('openAdd')"
      >
        <Plus class="mr-1 h-3.5 w-3.5" />
        连接设备
      </Button>
    </div>
  </div>
</template>
