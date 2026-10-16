<script setup lang="ts">
import type { AiTool, AiToolId, SkillItem } from '../types'
import { computed, ref, watch } from 'vue'
import { Check, Download } from 'lucide-vue-next'
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
import {
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from '@/components/ui/select'

const props = defineProps<{
  open: boolean
  skill: SkillItem | null
  targetToolId: AiToolId
  availableTools: AiTool[]
  installedTools: AiToolId[]
}>()

const emit = defineEmits<{
  (e: 'update:open', val: boolean): void
  (e: 'confirm', toolId: AiToolId): void
}>()

const selectedToolId = ref<AiToolId>(props.targetToolId)

watch(
  () => props.targetToolId,
  (val) => {
    selectedToolId.value = val
  },
  { immediate: true },
)

watch(
  () => props.open,
  (isOpen) => {
    if (isOpen) {
      selectedToolId.value = props.targetToolId
    }
  },
)

const isAlreadyInstalled = computed(() => {
  return props.installedTools.includes(selectedToolId.value)
})

function handleConfirm() {
  if (isAlreadyInstalled.value)
    return
  emit('confirm', selectedToolId.value)
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
        <!-- 技能基本信息简览（去掉了虚构的 supportedAgents） -->
        <div v-if="props.skill" class="flex items-center gap-2 rounded-lg bg-muted/40 p-3 text-xs">
          <span class="font-medium text-foreground">{{ props.skill.app }}</span>
          <Badge variant="secondary" class="font-normal text-[11px]">
            {{ props.skill.categoryLabel }}
          </Badge>
        </div>

        <!-- 选择安装目标 AI 工具 -->
        <div class="space-y-1.5">
          <label for="ai-tool-select" class="text-xs font-medium text-foreground">
            安装目标 AI 助手
          </label>
          <Select v-model="selectedToolId">
            <SelectTrigger id="ai-tool-select" class="w-full text-xs" aria-label="选择安装目标 AI 助手">
              <SelectValue placeholder="选择目标工具" />
            </SelectTrigger>
            <SelectContent>
              <SelectItem
                v-for="tool in props.availableTools"
                :key="tool.id"
                :value="tool.id"
                class="text-xs"
              >
                <div class="flex items-center gap-2">
                  <span>{{ tool.name }}</span>
                  <span v-if="props.installedTools.includes(tool.id)" class="text-[11px] text-muted-foreground">
                    (已安装)
                  </span>
                </div>
              </SelectItem>
            </SelectContent>
          </Select>
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
          :disabled="isAlreadyInstalled"
          @click="handleConfirm"
        >
          <Check v-if="isAlreadyInstalled" class="mr-1.5 h-3.5 w-3.5" />
          <Download v-else class="mr-1.5 h-3.5 w-3.5" />
          {{ isAlreadyInstalled ? '已安装' : '确认安装' }}
        </Button>
      </DialogFooter>
    </DialogContent>
  </Dialog>
</template>
