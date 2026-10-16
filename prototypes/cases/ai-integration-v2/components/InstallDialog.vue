<script setup lang="ts">
import type { AiTool, AiToolId, SkillItem } from '../types'
import { ref, watch } from 'vue'
import { Download } from 'lucide-vue-next'
import { Badge } from '@/components/ui/badge'
import { Button } from '@/components/ui/button'
import {
  Dialog,
  DialogContent,
  DialogDescription,
  DialogFooter,
  DialogHeader,
  DialogTitle,
} from '@/components/ui/dialog'
import TargetToolSelector from './TargetToolSelector.vue'

const props = defineProps<{
  open: boolean
  skill: SkillItem | null
  defaultToolId: AiToolId
  availableTools: AiTool[]
  installedTools: AiToolId[]
}>()

const emit = defineEmits<{
  (e: 'update:open', val: boolean): void
  (e: 'confirm', toolIds: AiToolId[]): void
}>()

const selectedToolIds = ref<AiToolId[]>([])

function initSelection() {
  if (!props.installedTools.includes(props.defaultToolId)) {
    selectedToolIds.value = [props.defaultToolId]
  }
  else {
    selectedToolIds.value = []
  }
}

watch(
  () => props.open,
  (isOpen) => {
    if (isOpen) {
      initSelection()
    }
  },
  { immediate: true },
)

function handleConfirm() {
  const targetsToInstall = selectedToolIds.value.filter(
    id => !props.installedTools.includes(id),
  )
  if (targetsToInstall.length === 0) {
    return
  }
  emit('confirm', targetsToInstall)
  emit('update:open', false)
}
</script>

<template>
  <Dialog :open="props.open" @update:open="emit('update:open', $event)">
    <DialogContent class="max-w-md">
      <DialogHeader>
        <DialogTitle class="text-lg font-semibold tracking-tight">
          {{ props.skill ? `安装技能 · ${props.skill.name}` : '安装基础设备操作技能' }}
        </DialogTitle>
        <DialogDescription class="text-xs text-muted-foreground">
          {{ props.skill ? props.skill.description : '将底层设备连接与基础操作能力加入外部 AI 助手' }}
        </DialogDescription>
      </DialogHeader>

      <div class="space-y-4 py-2">
        <!-- 技能基本信息简览 -->
        <div v-if="props.skill" class="flex items-center gap-2 rounded-lg bg-muted/40 p-3 text-xs">
          <span class="font-medium text-foreground">{{ props.skill.app }}</span>
          <Badge variant="secondary" class="font-normal text-[11px]">
            {{ props.skill.categoryLabel }}
          </Badge>
        </div>

        <!-- 多选目标助手 -->
        <div class="space-y-2">
          <label class="text-xs font-medium text-foreground">
            安装目标 AI 助手（支持多选）
          </label>
          <TargetToolSelector
            v-model="selectedToolIds"
            :available-tools="props.availableTools"
            :installed-tools="props.installedTools"
          />
        </div>
      </div>

      <DialogFooter class="flex items-center justify-end gap-2 border-t pt-3">
        <Button
          variant="outline"
          size="sm"
          class="h-8 text-xs"
          @click="emit('update:open', false)"
        >
          取消
        </Button>
        <Button
          variant="default"
          size="sm"
          class="h-8 text-xs"
          :disabled="selectedToolIds.length === 0"
          @click="handleConfirm"
        >
          <Download class="mr-1.5 h-3.5 w-3.5" />
          确认安装{{ selectedToolIds.length > 0 ? ` (${selectedToolIds.length})` : '' }}
        </Button>
      </DialogFooter>
    </DialogContent>
  </Dialog>
</template>
