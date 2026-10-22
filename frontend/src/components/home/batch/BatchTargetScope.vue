<script setup lang="ts">
import { Button } from '@/components/ui/button'

defineProps<{
  selectedCount: number
  isAllSelected: boolean
  hasSelectable: boolean
  batchRunning: boolean
}>()

const emit = defineEmits<{
  (e: 'selectAll'): void
  (e: 'clearSelection'): void
}>()
</script>

<template>
  <div class="space-y-2 border-t border-slate-200/80 dark:border-white/10 pt-3">
    <div class="flex items-center justify-between text-xs">
      <span class="text-slate-600 dark:text-slate-400 font-medium">批量选择</span>
      <span class="font-semibold text-violet-600 dark:text-violet-400">已选 {{ selectedCount }} 台</span>
    </div>
    <div class="flex items-center gap-2">
      <Button
        variant="outline"
        size="sm"
        class="flex-1 h-7.5 rounded-xl text-xs cursor-pointer"
        :disabled="!hasSelectable || batchRunning"
        @click="emit('selectAll')"
      >
        {{ isAllSelected ? '取消全选' : '全选' }}
      </Button>
      <Button
        variant="outline"
        size="sm"
        class="flex-1 h-7.5 rounded-xl text-xs cursor-pointer"
        :disabled="selectedCount === 0 || batchRunning"
        @click="emit('clearSelection')"
      >
        清空
      </Button>
    </div>
  </div>
</template>
