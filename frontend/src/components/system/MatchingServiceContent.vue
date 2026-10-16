<script setup lang="ts">
import { computed } from 'vue'
import { Loader2 } from 'lucide-vue-next'
import { Badge } from '@/components/ui/badge'
import { Card } from '@/components/ui/card'
import { Switch } from '@/components/ui/switch'
import type { MatchingStatus } from '@/composables/useMatchingService'

const props = defineProps<{ status?: MatchingStatus; busy: boolean; error: string }>()
const emit = defineEmits<{ action: [operation: 'check' | 'start' | 'stop'] }>()

const isRunning = computed(() => props.status?.state === 'running')
const labels = { stopped: '未启动', checking: '检测中', installing: '准备依赖中', starting: '启动中', running: '运行中', stopping: '停止中', failed: '服务异常' }
const deviceLabels = { waiting: '等待画面', matching: '匹配中', failed: '匹配异常' }

function onToggle(enabled: boolean) {
  emit('action', enabled ? 'start' : 'stop')
}
</script>

<template>
  <Card class="gap-0 p-4">
    <div class="flex items-center justify-between gap-3">
      <div>
        <div class="flex flex-wrap items-center gap-2">
          <h2 class="text-sm font-medium">模板匹配服务</h2>
          <Badge variant="outline">{{ status ? labels[status.state] : '读取中' }}</Badge>
          <Loader2 v-if="busy" class="size-3.5 animate-spin text-muted-foreground" />
        </div>
        <p class="mt-1 text-xs text-muted-foreground">调试时按需启动模板匹配服务；正式执行任务时，由任务脚本负责模板匹配。</p>
      </div>
      <Switch
        :model-value="isRunning"
        :disabled="!status || busy"
        aria-label="模板匹配服务开关"
        @update:model-value="onToggle"
      />
    </div>

    <div v-if="status?.devices.length" class="mt-4 divide-y rounded-lg border px-4">
      <div v-for="device in status.devices" :key="device.serial" class="space-y-1 py-3">
        <div class="flex items-center justify-between gap-3">
          <span class="min-w-0 break-all text-xs">{{ device.serial }}</span>
          <Badge variant="secondary">{{ deviceLabels[device.state] }}</Badge>
        </div>
        <p v-if="device.error" class="break-all text-xs text-destructive">{{ device.error }}</p>
      </div>
    </div>

    <p v-if="error || status?.error" role="alert" class="mt-4 break-all text-xs text-destructive">{{ error || status?.error }}</p>
  </Card>
</template>
