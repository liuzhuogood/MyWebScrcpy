<script setup lang="ts">
import { Loader2 } from 'lucide-vue-next'
import type { BatchItemResult, BatchSummary } from '@/composables/useDeviceBatch'

defineProps<{
  batchResults: BatchItemResult[]
  batchSummary: BatchSummary
}>()

const emit = defineEmits<{
  (e: 'clearResults'): void
}>()
</script>

<template>
  <div v-if="batchResults.length > 0" class="space-y-2 border-t border-slate-200/80 dark:border-white/10 pt-3">
    <div class="flex items-center justify-between text-xs">
      <span class="text-slate-600 dark:text-slate-400 font-medium">执行结果</span>
      <div class="flex items-center gap-2">
        <span class="text-[10px] text-emerald-600 dark:text-emerald-400">成功 {{ batchSummary.success }}</span>
        <span v-if="batchSummary.failed > 0" class="text-[10px] text-rose-600 dark:text-rose-400">失败 {{ batchSummary.failed }}</span>
        <span v-if="batchSummary.unknown > 0" class="text-[10px] text-amber-600 dark:text-amber-400">待确认 {{ batchSummary.unknown }}</span>
        <button
          type="button"
          class="text-[10px] text-slate-400 hover:text-slate-900 dark:hover:text-white cursor-pointer ml-1"
          @click="emit('clearResults')"
        >
          清除
        </button>
      </div>
    </div>

    <div class="max-h-48 overflow-y-auto space-y-1.5 pr-1 overscroll-contain">
      <div
        v-for="r in batchResults"
        :key="r.serial"
        class="rounded-xl border border-slate-200/80 bg-slate-50/70 p-2 text-xs flex items-center justify-between gap-2 dark:border-white/5 dark:bg-white/[0.02]"
      >
        <div class="min-w-0">
          <div class="truncate text-[11px] font-medium text-slate-800 dark:text-slate-200">
            {{ r.name || r.serial }}
          </div>
          <div class="truncate font-mono text-[9px] text-slate-400">
            {{ r.serial }}
          </div>
          <div v-if="r.error" class="text-[9px] text-rose-500 truncate">{{ r.error }}</div>
        </div>
        <div class="shrink-0 text-right">
          <span
            v-if="r.status === 'success'"
            class="inline-flex items-center gap-1 text-[10px] text-emerald-600 dark:text-emerald-400 font-medium"
          >
            成功
            <span v-if="r.elapsedMs" class="font-mono text-[8px] text-slate-400">{{ r.elapsedMs }}ms</span>
          </span>
          <span v-else-if="r.status === 'failed'" class="text-[10px] text-rose-600 dark:text-rose-400 font-medium">
            失败
          </span>
          <span v-else-if="r.status === 'running'" class="inline-flex items-center gap-1 text-[10px] text-violet-600 dark:text-violet-400">
            <Loader2 class="h-3 w-3 animate-spin" />
            执行中
          </span>
          <span v-else-if="r.status === 'unknown'" class="text-[10px] text-amber-600 dark:text-amber-400 font-medium" title="请求超时或网络异常，需检查设备实际状态">
            待确认
          </span>
          <span v-else-if="r.status === 'cancelled'" class="text-[10px] text-slate-400 font-medium">
            已取消
          </span>
          <span v-else class="text-[10px] text-slate-400">
            等待
          </span>
        </div>
      </div>
    </div>
  </div>
</template>
