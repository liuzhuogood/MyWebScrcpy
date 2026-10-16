<script setup lang="ts">
import type { AiTool, AiToolId, SkillItem } from '../types'
import { computed } from 'vue'
import { Check, CheckCircle2, Download } from 'lucide-vue-next'
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
import { aiToolMap } from '../data'

const props = defineProps<{
  open: boolean
  skill: SkillItem | null
  currentToolId: AiToolId
  installedTools: AiToolId[]
  availableTools: AiTool[]
}>()

const emit = defineEmits<{
  (e: 'update:open', val: boolean): void
  (e: 'request-install', skill: SkillItem, toolId: AiToolId): void
}>()

const isInstalledOnCurrent = computed(() => {
  if (!props.skill)
    return false
  return props.installedTools.includes(props.currentToolId)
})

const currentToolName = computed(() => {
  return aiToolMap[props.currentToolId].name
})

function handleInstall(toolId: AiToolId) {
  if (!props.skill)
    return
  emit('update:open', false)
  emit('request-install', props.skill, toolId)
}
</script>

<template>
  <Dialog :open="props.open" @update:open="emit('update:open', $event)">
    <DialogContent class="max-w-md sm:max-w-lg">
      <DialogHeader v-if="props.skill">
        <div class="flex items-center gap-2">
          <Badge variant="secondary" class="text-xs font-normal">
            {{ props.skill.app }}
          </Badge>
          <Badge variant="outline" class="text-xs font-normal">
            {{ props.skill.categoryLabel }}
          </Badge>
        </div>
        <DialogTitle class="text-lg font-bold tracking-tight">
          {{ props.skill.name }}
        </DialogTitle>
        <DialogDescription class="text-xs text-muted-foreground">
          {{ props.skill.description }}
        </DialogDescription>
      </DialogHeader>

      <div v-if="props.skill" class="space-y-4 py-2 text-sm">
        <!-- 概述说明 -->
        <div class="rounded-lg bg-muted/40 p-3 text-xs text-muted-foreground leading-relaxed">
          {{ props.skill.details.summary }}
        </div>

        <!-- 核心能力列表 -->
        <div class="space-y-2">
          <h4 class="text-xs font-semibold uppercase tracking-wider text-muted-foreground">
            核心能力
          </h4>
          <ul class="space-y-1.5 rounded-lg border p-3 text-xs text-foreground">
            <li
              v-for="(cap, idx) in props.skill.details.capabilities"
              :key="idx"
              class="flex items-start gap-2"
            >
              <CheckCircle2 class="mt-0.5 h-3.5 w-3.5 shrink-0 text-primary" />
              <span>{{ cap }}</span>
            </li>
          </ul>
        </div>

        <!-- 各助手安装状态与配置 -->
        <div class="space-y-2">
          <h4 class="text-xs font-semibold uppercase tracking-wider text-muted-foreground">
            AI 助手配置状态
          </h4>
          <div class="grid grid-cols-1 gap-2 sm:grid-cols-3">
            <div
              v-for="tool in props.availableTools"
              :key="tool.id"
              class="flex items-center justify-between rounded-md border p-2.5 text-xs"
              :class="[
                props.installedTools.includes(tool.id)
                  ? 'border-green-200 bg-green-50/40 dark:border-green-900/40 dark:bg-green-950/20'
                  : 'border-border bg-card',
              ]"
            >
              <div class="font-medium text-foreground">
                {{ tool.name }}
              </div>
              <div v-if="props.installedTools.includes(tool.id)" class="flex items-center gap-1 text-green-700 dark:text-green-400">
                <Check class="h-3.5 w-3.5" />
                <span>已安装</span>
              </div>
              <Button
                v-else
                variant="ghost"
                size="sm"
                class="h-6 px-2 text-[11px] text-primary hover:text-primary"
                @click="handleInstall(tool.id)"
              >
                安装
              </Button>
            </div>
          </div>
        </div>
      </div>

      <DialogFooter class="flex items-center justify-between gap-2 border-t pt-3">
        <Button
          variant="outline"
          size="sm"
          class="h-8 text-xs"
          @click="emit('update:open', false)"
        >
          关闭
        </Button>

        <div v-if="props.skill" class="flex items-center gap-2">
          <Button
            v-if="!isInstalledOnCurrent"
            variant="default"
            size="sm"
            class="h-8 text-xs"
            @click="handleInstall(props.currentToolId)"
          >
            <Download class="mr-1.5 h-3.5 w-3.5" />
            安装到 {{ currentToolName }}
          </Button>
          <div
            v-else
            class="flex h-8 items-center gap-1.5 rounded-md bg-muted px-3 text-xs font-medium text-muted-foreground"
          >
            <Check class="h-3.5 w-3.5" />
            已安装至 {{ currentToolName }}
          </div>
        </div>
      </DialogFooter>
    </DialogContent>
  </Dialog>
</template>
