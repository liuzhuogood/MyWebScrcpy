<script setup lang="ts">
import type { AssistantActivity } from '@/scripts/useScriptAssistant'
defineProps<{ activities?: AssistantActivity[] }>()
const toolStatus = { running: '执行中', completed: '已完成', failed: '失败', interrupted: '已中断' }
</script>
<template>
  <template v-for="(activity, index) in activities" :key="index">
    <details v-if="activity.kind === 'thinking'" open class="rounded-lg border bg-muted/20 px-3 py-2">
      <summary class="cursor-pointer text-xs text-muted-foreground">{{ activity.running ? '思考中…' : '思考' }}</summary>
      <p v-if="activity.content" class="mt-2 text-xs leading-5 whitespace-pre-wrap text-muted-foreground">{{ activity.content }}</p>
    </details>
    <details v-else open class="rounded-lg border px-3 py-2">
      <summary class="cursor-pointer text-xs"><span class="font-medium">{{ activity.name }}</span><span class="ml-2 text-muted-foreground" :class="activity.status === 'failed' ? 'text-destructive' : ''">{{ toolStatus[activity.status] }}</span></summary>
      <pre v-if="activity.args" class="mt-2 max-h-40 overflow-auto text-xs leading-5 whitespace-pre-wrap break-all text-muted-foreground">{{ activity.args }}</pre>
      <pre v-if="activity.output" class="mt-2 max-h-56 overflow-auto border-t pt-2 text-xs leading-5 whitespace-pre-wrap break-all">{{ activity.output }}</pre>
    </details>
  </template>
</template>
