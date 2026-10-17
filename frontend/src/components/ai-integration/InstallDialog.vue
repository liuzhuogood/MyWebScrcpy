<script setup lang="ts">
import type { AiTool, AiToolId, SkillInstallTargetResult, SkillItem } from '@/api/aiIntegrations'
import { computed, ref, watch } from 'vue'
import { AlertCircle, CheckCircle2, Download, Loader2, RotateCw } from 'lucide-vue-next'
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
  isInstalling?: boolean
  lastResults?: SkillInstallTargetResult[] | null
  generalError?: string | null
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

const hasFailures = computed(() => {
  return Boolean(props.lastResults && props.lastResults.some(r => r.status === 'failed'))
})

watch(
  () => props.lastResults,
  (results) => {
    if (results && results.length > 0) {
      const failed = results.filter(r => r.status === 'failed').map(r => r.target)
      if (failed.length > 0) {
        selectedToolIds.value = failed
      }
    }
  },
)

function handleConfirm() {
  const targetsToInstall = selectedToolIds.value.filter(
    id => !props.installedTools.includes(id),
  )
  if (targetsToInstall.length === 0 || props.isInstalling) {
    return
  }
  emit('confirm', targetsToInstall)
}
</script>

<template>
  <Dialog :open="props.open" @update:open="(val) => { if (!props.isInstalling) emit('update:open', val) }">
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
            :disabled="props.isInstalling"
          />
        </div>

        <!-- 宿主机说明提示 -->
        <p class="text-[11px] text-muted-foreground leading-normal">
          * 技能文件将写入运行 MyWebScrcpy 服务的宿主机对应助手目录，已存在且被用户修改的文件将受保护不被覆盖。
        </p>

        <!-- 最近安装结果反馈 -->
        <div v-if="props.lastResults && props.lastResults.length > 0" class="space-y-1.5 rounded-lg border p-2.5 text-xs">
          <div
            v-for="res in props.lastResults"
            :key="res.target"
            class="flex items-start gap-2"
          >
            <CheckCircle2
              v-if="res.status === 'installed' || res.status === 'already_installed'"
              class="h-4 w-4 shrink-0 text-green-600 mt-0.5"
            />
            <AlertCircle
              v-else
              class="h-4 w-4 shrink-0 text-destructive mt-0.5"
            />
            <div class="flex-1 break-all [overflow-wrap:anywhere]">
              <span class="font-medium">
                {{ props.availableTools.find(t => t.id === res.target)?.name || res.target }}:
              </span>
              <span v-if="res.status === 'installed'" class="text-green-600 ml-1">安装成功</span>
              <span v-else-if="res.status === 'already_installed'" class="text-muted-foreground ml-1">已安装，无需重复写入</span>
              <span v-else class="text-destructive ml-1">{{ res.error || '安装失败' }}</span>
            </div>
          </div>
        </div>

        <!-- 通用错误提示 -->
        <div v-if="props.generalError" class="rounded-lg bg-destructive/10 p-2.5 text-xs text-destructive flex items-start gap-2">
          <AlertCircle class="h-4 w-4 shrink-0 mt-0.5" />
          <span class="break-all [overflow-wrap:anywhere]">{{ props.generalError }}</span>
        </div>
      </div>

      <DialogFooter class="flex items-center justify-end gap-2 border-t pt-3">
        <Button
          variant="outline"
          size="sm"
          :disabled="props.isInstalling"
          @click="emit('update:open', false)"
        >
          取消
        </Button>
        <Button
          size="sm"
          :disabled="selectedToolIds.length === 0 || props.isInstalling"
          @click="handleConfirm"
        >
          <Loader2 v-if="props.isInstalling" class="mr-1.5 h-3.5 w-3.5 animate-spin" />
          <RotateCw v-else-if="hasFailures" class="mr-1.5 h-3.5 w-3.5" />
          <Download v-else class="mr-1.5 h-3.5 w-3.5" />
          {{ props.isInstalling ? '正在安装...' : hasFailures ? '重试安装' : '确认安装' }}
        </Button>
      </DialogFooter>
    </DialogContent>
  </Dialog>
</template>
