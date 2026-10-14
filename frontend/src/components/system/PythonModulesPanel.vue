<script setup lang="ts">
import { Loader2 } from 'lucide-vue-next'
import { Button } from '@/components/ui/button'
import { Badge } from '@/components/ui/badge'
import type { DependencyInfo } from '@/types/dependencies'
import type { InstallTask } from '@/composables/useDependencies'

defineProps<{ runtime?: DependencyInfo; task?: InstallTask; disabled: boolean }>()
defineEmits<{ install: []; cancel: [] }>()
</script>

<template>
  <div class="space-y-2 border-t pt-3">
    <div class="flex flex-wrap items-center justify-between gap-2">
      <div class="flex min-w-0 flex-wrap items-center gap-2">
        <span class="text-xs font-medium">mywebscrcpy-sdk</span>
        <Badge variant="outline">{{ runtime?.python?.sdk_version || '未就绪' }}</Badge>
      </div>
      <Button v-if="task?.stage === 'installing'" variant="ghost" size="sm" @click="$emit('cancel')"><Loader2 class="size-3.5 animate-spin" />取消</Button>
      <Button v-else variant="outline" size="sm" :disabled="disabled || !runtime?.installed || !runtime.python?.pip_available" @click="$emit('install')">安装依赖</Button>
    </div>
    <p v-if="task?.stage === 'installing'" role="status" class="text-xs text-muted-foreground">正在安装依赖…</p>
    <p v-else-if="runtime?.python && !runtime.python.pip_available" role="alert" class="text-xs text-destructive">所选 Python 缺少 pip</p>
    <p v-else-if="runtime?.python?.sdk_error" role="alert" class="text-xs text-destructive">{{ runtime.python.sdk_error }}</p>
    <p v-else-if="runtime?.python?.sdk_version && !runtime.python.ready" class="text-xs text-muted-foreground">需要 {{ runtime.python.required_version }}</p>
    <p v-if="task?.error" role="alert" class="whitespace-pre-wrap break-all text-xs text-destructive">{{ task.error }}</p>
  </div>
</template>
