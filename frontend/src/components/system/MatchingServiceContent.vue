<script setup lang="ts">
import { computed } from 'vue'
import { Loader2, RefreshCw } from 'lucide-vue-next'
import { Button } from '@/components/ui/button'
import { Badge } from '@/components/ui/badge'
import type { MatchingStatus } from '@/composables/useMatchingService'
const props = defineProps<{ status?: MatchingStatus; busy: boolean; error: string }>()
const emit = defineEmits<{ action: [operation: 'check' | 'start' | 'stop'] }>()
const labels = { stopped: '未启动', checking: '检测中', starting: '启动中', running: '运行中', stopping: '停止中', failed: '服务异常' }
const deviceLabels = { waiting: '等待画面', matching: '匹配中', failed: '匹配异常' }
const missing = computed(() => props.status?.environment.dependencies.filter(item => !item.available) ?? [])
</script>
<template>
  <div class="space-y-4">
    <div class="flex items-center justify-between gap-3"><h2 class="text-sm font-medium">模板匹配服务</h2><Badge variant="outline">{{ status ? labels[status.state] : '读取中' }}</Badge></div>
    <p class="text-xs leading-relaxed text-muted-foreground">调试时按需启动模板匹配服务；正式执行任务时，由任务脚本负责模板匹配。</p>
    <div class="space-y-3 rounded-lg border p-4">
      <div class="flex items-center justify-between gap-3"><span class="text-sm">Python 环境</span><Button variant="ghost" size="sm" :disabled="!status || busy || status.state === 'running'" @click="emit('action', 'check')"><RefreshCw class="size-3.5" :class="{ 'animate-spin': status?.state === 'checking' }" />检测依赖</Button></div>
      <template v-if="status?.environment.version"><p class="text-xs" :class="status.environment.ready ? 'text-muted-foreground' : 'text-destructive'">Python {{ status.environment.version }} · {{ status.environment.ready ? '依赖可用' : '环境不可用' }}</p><p class="break-all text-xs text-muted-foreground">{{ status.environment.path }}</p></template>
      <p v-else-if="!status?.environment.error" class="text-xs text-muted-foreground">请先检测 Python 与识图依赖。</p>
      <p v-if="status?.environment.error" role="alert" class="text-xs text-destructive">{{ status.environment.error }}</p>
      <p v-if="missing.length" role="alert" class="text-xs text-destructive">缺少依赖：{{ missing.map(item => item.name).join('、') }}</p>
    </div>
    <div class="flex items-center justify-between gap-3"><span class="text-xs text-muted-foreground">{{ status?.state === 'running' ? `匹配设备 · ${status.devices.length} 台` : '启用后自动匹配当前投屏设备的模板。' }}</span><Button size="sm" :disabled="!status || busy || (!status.environment.ready && status.state !== 'running')" @click="emit('action', status?.state === 'running' ? 'stop' : 'start')"><Loader2 v-if="busy" class="size-3.5 animate-spin" />{{ status?.state === 'running' ? '停止' : '启动' }}</Button></div>
    <div v-if="status?.devices.length" class="divide-y rounded-lg border px-4"><div v-for="device in status.devices" :key="device.serial" class="space-y-1 py-3"><div class="flex items-center justify-between gap-3"><span class="min-w-0 break-all text-xs">{{ device.serial }}</span><Badge variant="secondary">{{ deviceLabels[device.state] }}</Badge></div><p v-if="device.error" class="break-all text-xs text-destructive">{{ device.error }}</p></div></div>
    <p v-if="error || status?.error" role="alert" class="break-all text-xs text-destructive">{{ error || status?.error }}</p>
  </div>
</template>
