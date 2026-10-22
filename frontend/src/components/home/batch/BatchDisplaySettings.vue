<script setup lang="ts">
import { Maximize, Minimize } from 'lucide-vue-next'
import { Button } from '@/components/ui/button'
import type { DashSize } from '@/composables/useHomePhoneView'

defineProps<{
  dashSize?: DashSize
  viewMode: 'deck' | 'compact' | 'phone'
  isPageFullscreen?: boolean
}>()

const emit = defineEmits<{
  (e: 'update:dashSize', val: DashSize): void
  (e: 'togglePageFullscreen'): void
}>()

const sizes: { key: DashSize, label: string }[] = [
  { key: 'small', label: '小' },
  { key: 'medium', label: '中' },
  { key: 'large', label: '大' },
  { key: 'fit', label: '铺满' },
]
</script>

<template>
  <div class="space-y-2 border-t border-slate-200/80 dark:border-white/10 pt-3">
    <div class="flex items-center justify-between text-xs">
      <span class="text-slate-600 dark:text-slate-400 font-medium">显示</span>
      <Button
        variant="ghost"
        size="sm"
        class="h-6 px-1.5 text-[11px] rounded-lg text-slate-600 dark:text-slate-300 gap-1 cursor-pointer hover:bg-slate-100 dark:hover:bg-white/10"
        :title="isPageFullscreen ? '退出全屏' : '页面全屏'"
        @click="emit('togglePageFullscreen')"
      >
        <Minimize v-if="isPageFullscreen" class="h-3 w-3" />
        <Maximize v-else class="h-3 w-3" />
        <span>{{ isPageFullscreen ? '退出全屏' : '全屏' }}</span>
      </Button>
    </div>

    <!-- 手机视图卡片尺寸档位 -->
    <div
      v-if="viewMode === 'phone'"
      class="flex items-center gap-1 rounded-xl border border-slate-200/80 bg-slate-100/80 p-0.5 dark:border-white/10 dark:bg-black/20"
    >
      <button
        v-for="s in sizes"
        :key="s.key"
        type="button"
        class="flex-1 h-7 rounded-lg text-xs font-medium transition-colors cursor-pointer"
        :class="dashSize === s.key
          ? 'bg-white text-slate-950 shadow-2xs font-semibold dark:bg-white/15 dark:text-white'
          : 'text-slate-600 hover:text-slate-950 dark:text-slate-400 dark:hover:text-white'"
        @click="emit('update:dashSize', s.key)"
      >
        {{ s.label }}
      </button>
    </div>
  </div>
</template>
