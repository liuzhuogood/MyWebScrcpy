<script setup lang="ts">
import type { AiTool, AiToolId } from '@/api/aiIntegrations'
import { useId } from 'vue'
import { Badge } from '@/components/ui/badge'
import { Checkbox } from '@/components/ui/checkbox'

const props = defineProps<{
  availableTools: AiTool[]
  installedTools: AiToolId[]
  modelValue: AiToolId[]
  disabled?: boolean
}>()

const emit = defineEmits<{
  (e: 'update:modelValue', value: AiToolId[]): void
}>()

const instanceId = useId()

function isToolInstalled(id: AiToolId): boolean {
  return props.installedTools.includes(id)
}

function isToolDisabled(id: AiToolId): boolean {
  return props.disabled === true || isToolInstalled(id)
}

function isToolChecked(id: AiToolId): boolean {
  if (isToolInstalled(id)) {
    return true
  }
  return props.modelValue.includes(id)
}

function handleToggle(id: AiToolId, checked: boolean) {
  if (isToolDisabled(id)) {
    return
  }
  if (checked) {
    if (!props.modelValue.includes(id)) {
      emit('update:modelValue', [...props.modelValue, id])
    }
  }
  else {
    emit('update:modelValue', props.modelValue.filter(item => item !== id))
  }
}
</script>

<template>
  <div class="space-y-2" role="group" aria-label="选择安装目标 AI 助手">
    <div
      v-for="tool in props.availableTools"
      :key="tool.id"
      class="flex items-center justify-between rounded-lg border p-2.5 transition-colors"
      :class="[
        isToolInstalled(tool.id)
          ? 'border-border/60 bg-muted/40 opacity-80'
          : isToolChecked(tool.id)
            ? 'border-primary/50 bg-primary/5'
            : 'border-border bg-card hover:bg-muted/30',
      ]"
    >
      <label
        :for="`${instanceId}-tool-${tool.id}`"
        class="flex flex-1 items-center gap-2.5 text-xs font-medium break-all [overflow-wrap:anywhere]"
        :class="[isToolDisabled(tool.id) ? 'cursor-not-allowed text-muted-foreground' : 'cursor-pointer text-foreground']"
      >
        <Checkbox
          :id="`${instanceId}-tool-${tool.id}`"
          :model-value="isToolChecked(tool.id)"
          :disabled="isToolDisabled(tool.id)"
          @update:model-value="handleToggle(tool.id, $event === true)"
        />
        <span>{{ tool.name }}</span>
      </label>

      <Badge
        v-if="isToolInstalled(tool.id)"
        variant="secondary"
        class="text-[11px] font-normal shrink-0"
      >
        已安装
      </Badge>
    </div>
  </div>
</template>
