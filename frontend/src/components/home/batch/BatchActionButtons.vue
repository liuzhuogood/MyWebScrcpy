<script setup lang="ts">
import {
  AppWindow,
  ChevronLeft,
  Home,
  Loader2,
} from 'lucide-vue-next'
import { Button } from '@/components/ui/button'

defineProps<{
  batchRunning: boolean
  currentAction: string
  selectedCount: number
}>()

const emit = defineEmits<{
  (e: 'triggerAction', action: 'back' | 'home' | 'app_switch'): void
  (e: 'cancelBatch'): void
}>()

function formatActionName(action: string): string {
  if (action === 'back')
    return '返回'
  if (action === 'home')
    return '主页'
  if (action === 'app_switch')
    return '任务'
  return action
}
</script>

<template>
  <div class="space-y-2 border-t border-slate-200/80 dark:border-white/10 pt-3">
    <div class="grid grid-cols-3 gap-2">
      <Button
        variant="outline"
        size="sm"
        class="h-8 rounded-xl text-xs gap-1 cursor-pointer"
        :disabled="selectedCount === 0 || batchRunning"
        title="返回"
        @click="emit('triggerAction', 'back')"
      >
        <ChevronLeft class="h-3.5 w-3.5" />
        返回
      </Button>
      <Button
        variant="outline"
        size="sm"
        class="h-8 rounded-xl text-xs gap-1 cursor-pointer"
        :disabled="selectedCount === 0 || batchRunning"
        title="主页"
        @click="emit('triggerAction', 'home')"
      >
        <Home class="h-3.5 w-3.5" />
        主页
      </Button>
      <Button
        variant="outline"
        size="sm"
        class="h-8 rounded-xl text-xs gap-1 cursor-pointer"
        :disabled="selectedCount === 0 || batchRunning"
        title="任务"
        @click="emit('triggerAction', 'app_switch')"
      >
        <AppWindow class="h-3.5 w-3.5" />
        任务
      </Button>
    </div>

    <!-- 运行中指示与取消按键 -->
    <div
      v-if="batchRunning"
      class="flex items-center justify-between rounded-xl bg-violet-50 p-2 dark:bg-violet-950/30 text-xs text-violet-700 dark:text-violet-300"
    >
      <div class="flex items-center gap-2">
        <Loader2 class="h-3.5 w-3.5 animate-spin text-violet-600" />
        <span>正在执行 {{ formatActionName(currentAction) }}...</span>
      </div>
      <Button
        variant="destructive"
        size="sm"
        class="h-6 rounded-lg px-2 text-[11px] cursor-pointer"
        @click="emit('cancelBatch')"
      >
        取消
      </Button>
    </div>
  </div>
</template>
