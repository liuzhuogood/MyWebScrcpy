<script setup lang="ts">
import type { AiTool, AiToolId, SkillItem } from '../types'
import { onUnmounted, ref, watch } from 'vue'
import {
  ArrowLeft,
  Check,
  ChevronRight,
  Copy,
  Download,
  Info,
  Layers,
  ListOrdered,
  Radio,
  Smartphone,
  Sparkles,
  Zap,
} from 'lucide-vue-next'
import { Badge } from '@/components/ui/badge'
import { Button } from '@/components/ui/button'
import { Card, CardContent, CardHeader } from '@/components/ui/card'
import TargetToolSelector from './TargetToolSelector.vue'

const props = defineProps<{
  skill: SkillItem
  currentToolId: AiToolId
  availableTools: AiTool[]
  installedTools: AiToolId[]
}>()

const emit = defineEmits<{
  (e: 'back'): void
  (e: 'install', toolIds: AiToolId[]): void
}>()

const selectedToolIds = ref<AiToolId[]>([])

function initSelectedTools() {
  if (!props.installedTools.includes(props.currentToolId)) {
    selectedToolIds.value = [props.currentToolId]
  }
  else {
    selectedToolIds.value = []
  }
}

watch(
  [() => props.skill.id, () => props.currentToolId],
  () => {
    initSelectedTools()
  },
  { immediate: true },
)

function handleConfirmInstall() {
  const targets = selectedToolIds.value.filter(
    id => !props.installedTools.includes(id),
  )
  if (targets.length === 0)
    return
  emit('install', targets)
  selectedToolIds.value = []
}

// 示例提示词复制状态与竞态/失效控制
const copyingIndex = ref<number | null>(null)
const copyError = ref('')
const copySeq = ref(0)
let copyTimer: ReturnType<typeof setTimeout> | null = null

function resetCopyState() {
  copySeq.value++
  copyingIndex.value = null
  copyError.value = ''
  if (copyTimer) {
    clearTimeout(copyTimer)
    copyTimer = null
  }
}

watch(
  () => props.skill.id,
  () => {
    resetCopyState()
  },
)

async function handleCopyPrompt(text: string, index: number) {
  copySeq.value++
  const currentSeq = copySeq.value
  if (copyTimer) {
    clearTimeout(copyTimer)
    copyTimer = null
  }
  copyingIndex.value = null
  copyError.value = ''

  if (!navigator?.clipboard?.writeText) {
    copyError.value = '剪贴板不可用，请手动选择复制。'
    return
  }

  try {
    await navigator.clipboard.writeText(text)
    if (copySeq.value !== currentSeq)
      return
    copyingIndex.value = index
    copyTimer = setTimeout(() => {
      if (copySeq.value === currentSeq) {
        copyingIndex.value = null
      }
    }, 2000)
  }
  catch {
    if (copySeq.value !== currentSeq)
      return
    copyError.value = '复制失败，请手动选择复制。'
  }
}

onUnmounted(() => {
  resetCopyState()
})
</script>

<template>
  <div class="space-y-6" aria-label="技能详情">
    <!-- 顶部面包屑与返回操作 -->
    <div class="flex flex-wrap items-center justify-between gap-3 border-b pb-3">
      <nav class="flex items-center gap-1 text-xs text-muted-foreground" aria-label="面包屑导航">
        <Button
          variant="link"
          size="sm"
          class="h-auto p-0 text-xs text-muted-foreground hover:text-foreground hover:no-underline font-normal"
          @click="emit('back')"
        >
          AI 集成
        </Button>
        <ChevronRight class="h-3.5 w-3.5 text-muted-foreground/60 shrink-0" />
        <Button
          variant="link"
          size="sm"
          class="h-auto p-0 text-xs text-muted-foreground hover:text-foreground hover:no-underline font-normal"
          @click="emit('back')"
        >
          技能市场
        </Button>
        <ChevronRight class="h-3.5 w-3.5 text-muted-foreground/60 shrink-0" />
        <span class="font-medium text-foreground truncate">{{ props.skill.name }}</span>
      </nav>

      <Button
        variant="ghost"
        size="sm"
        class="h-7 gap-1 px-2 text-xs text-muted-foreground hover:text-foreground"
        @click="emit('back')"
      >
        <ArrowLeft class="h-3.5 w-3.5" />
        <span>返回技能市场</span>
      </Button>
    </div>

    <!-- 主体分栏：桌面 2/3 + 1/3，移动端纵向堆叠 -->
    <div class="grid grid-cols-1 gap-6 lg:grid-cols-3">
      <!-- 左侧主体详情内容 -->
      <div class="space-y-6 lg:col-span-2">
        <!-- 头部概览卡片 -->
        <Card class="overflow-hidden border border-border shadow-sm">
          <CardHeader class="p-5 sm:p-6">
            <div class="flex items-start gap-4">
              <!-- 应用图标 -->
              <div
                v-if="props.skill.iconType === 'kuaishou'"
                class="flex h-14 w-14 shrink-0 items-center justify-center rounded-2xl bg-gradient-to-br from-amber-500 to-orange-500 text-white shadow-sm"
              >
                <svg class="h-7 w-7" viewBox="0 0 24 24" fill="currentColor">
                  <rect x="2" y="5" width="14" height="14" rx="3" />
                  <path d="M16 10l5-3v10l-5-3v-4z" />
                </svg>
              </div>

              <div
                v-else-if="props.skill.iconType === 'wechat'"
                class="flex h-14 w-14 shrink-0 items-center justify-center rounded-2xl bg-emerald-500 text-white shadow-sm"
              >
                <svg class="h-7 w-7" viewBox="0 0 24 24" fill="currentColor">
                  <path d="M9 4C5.13 4 2 6.69 2 10c0 1.88 1.02 3.55 2.62 4.67-.11.66-.46 2.05-.53 2.37-.1.44.17.43.36.3.26-.17 2.39-1.63 2.8-1.92.56.13 1.15.2 1.75.2 3.87 0 7-2.69 7-6s-3.13-6-7-6zm7 8c-.34 0-.67.03-.99.08.62.86.99 1.87.99 2.92 0 1.57-.84 2.99-2.19 3.96.09.55.39 1.7.45 1.97.08.37-.14.36-.3.25-.22-.14-2-1.36-2.34-1.6-.52.12-1.07.19-1.62.19-1.89 0-3.6-.66-4.83-1.74C7.03 19.35 9.38 20.5 12 20.5c.53 0 1.05-.05 1.55-.16.36.25 2.22 1.5 2.45 1.65.17.11.4.12.31-.26-.06-.27-.37-1.45-.46-2.02 1.4-1 2.29-2.45 2.29-4.08 0-2.88-2.73-5.23-6.14-5.63z" />
                </svg>
              </div>

              <div
                v-else
                class="flex h-14 w-14 shrink-0 items-center justify-center rounded-2xl bg-blue-600 text-white shadow-sm"
              >
                <Smartphone class="h-7 w-7" />
              </div>

              <!-- 基础信息 -->
              <div class="min-w-0 space-y-1.5">
                <div class="flex flex-wrap items-center gap-2">
                  <h2 class="text-xl font-bold tracking-tight text-foreground sm:text-2xl">
                    {{ props.skill.name }}
                  </h2>
                  <Badge variant="secondary" class="font-normal text-xs">
                    {{ props.skill.categoryLabel }}
                  </Badge>
                  <span class="text-xs text-muted-foreground">· 归属应用 {{ props.skill.app }}</span>
                </div>
                <p class="text-xs text-muted-foreground sm:text-sm leading-relaxed">
                  {{ props.skill.description }}
                </p>
              </div>
            </div>
          </CardHeader>

          <CardContent class="border-t bg-muted/10 p-5 sm:p-6 space-y-3">
            <h3 class="flex items-center gap-2 text-sm font-semibold tracking-tight text-foreground">
              <Info class="h-4 w-4 text-primary" />
              <span>概览介绍</span>
            </h3>
            <p class="text-xs sm:text-sm text-muted-foreground leading-relaxed">
              {{ props.skill.details.overview }}
            </p>
          </CardContent>
        </Card>

        <!-- 核心能力 -->
        <Card class="border border-border shadow-sm">
          <CardHeader class="p-5 pb-3">
            <h3 class="flex items-center gap-2 text-base font-semibold tracking-tight text-foreground">
              <Zap class="h-4 w-4 text-primary" />
              <span>核心能力</span>
            </h3>
          </CardHeader>
          <CardContent class="p-5 pt-0 space-y-3">
            <div
              v-for="(cap, idx) in props.skill.details.capabilities"
              :key="idx"
              class="rounded-lg border bg-muted/20 p-3.5 space-y-1"
            >
              <div class="flex items-center gap-2 font-medium text-xs sm:text-sm text-foreground">
                <span class="flex h-5 w-5 shrink-0 items-center justify-center rounded-full bg-primary/10 text-xs font-semibold text-primary">
                  {{ idx + 1 }}
                </span>
                <span>{{ cap.title }}</span>
              </div>
              <p class="text-xs text-muted-foreground pl-7 leading-relaxed">
                {{ cap.description }}
              </p>
            </div>
          </CardContent>
        </Card>

        <!-- 典型使用场景 -->
        <Card class="border border-border shadow-sm">
          <CardHeader class="p-5 pb-3">
            <h3 class="flex items-center gap-2 text-base font-semibold tracking-tight text-foreground">
              <Radio class="h-4 w-4 text-primary" />
              <span>典型使用场景</span>
            </h3>
          </CardHeader>
          <CardContent class="p-5 pt-0">
            <ul class="space-y-2 text-xs sm:text-sm text-muted-foreground">
              <li
                v-for="(scenario, idx) in props.skill.details.scenarios"
                :key="idx"
                class="flex items-start gap-2.5"
              >
                <span class="mt-1.5 h-1.5 w-1.5 shrink-0 rounded-full bg-primary" />
                <span class="leading-relaxed text-foreground/90">{{ scenario }}</span>
              </li>
            </ul>
          </CardContent>
        </Card>

        <!-- 任务示例 -->
        <Card class="border border-border shadow-sm">
          <CardHeader class="p-5 pb-3">
            <h3 class="flex items-center gap-2 text-base font-semibold tracking-tight text-foreground">
              <Sparkles class="h-4 w-4 text-primary" />
              <span>任务示例（可复制到对话）</span>
            </h3>
          </CardHeader>
          <CardContent class="p-5 pt-0 space-y-2.5">
            <div
              v-for="(prompt, idx) in props.skill.details.samplePrompts"
              :key="idx"
              class="group flex flex-wrap items-center justify-between gap-3 rounded-lg border bg-muted/40 p-3 sm:flex-nowrap"
            >
              <span class="text-xs sm:text-sm text-foreground select-all leading-relaxed">
                {{ prompt }}
              </span>
              <div class="flex shrink-0 items-center gap-2 self-center">
                <Button
                  variant="outline"
                  size="sm"
                  class="h-7 px-2.5 text-xs font-medium"
                  :aria-label="`复制示例任务 ${idx + 1}`"
                  @click="handleCopyPrompt(prompt, idx)"
                >
                  <Check v-if="copyingIndex === idx" class="mr-1.5 h-3.5 w-3.5 text-green-600 dark:text-green-400" />
                  <Copy v-else class="mr-1.5 h-3.5 w-3.5" />
                  <span>{{ copyingIndex === idx ? '已复制' : '复制' }}</span>
                </Button>
              </div>
            </div>
            <p v-if="copyError" role="alert" class="mt-1 text-xs text-destructive">
              {{ copyError }}
            </p>
          </CardContent>
        </Card>

        <!-- 使用前准备与简洁使用步骤 -->
        <div class="grid grid-cols-1 gap-6 sm:grid-cols-2">
          <!-- 使用前准备 -->
          <Card class="border border-border shadow-sm">
            <CardHeader class="p-5 pb-3">
              <h3 class="flex items-center gap-2 text-sm font-semibold tracking-tight text-foreground">
                <Layers class="h-4 w-4 text-primary" />
                <span>使用前准备</span>
              </h3>
            </CardHeader>
            <CardContent class="p-5 pt-0">
              <ul class="space-y-2 text-xs text-muted-foreground">
                <li
                  v-for="(item, idx) in props.skill.details.prerequisites"
                  :key="idx"
                  class="flex items-start gap-2"
                >
                  <span class="mt-1.5 h-1.5 w-1.5 shrink-0 rounded-full bg-muted-foreground/60" />
                  <span class="leading-relaxed">{{ item }}</span>
                </li>
              </ul>
            </CardContent>
          </Card>

          <!-- 简洁使用步骤 -->
          <Card class="border border-border shadow-sm">
            <CardHeader class="p-5 pb-3">
              <h3 class="flex items-center gap-2 text-sm font-semibold tracking-tight text-foreground">
                <ListOrdered class="h-4 w-4 text-primary" />
                <span>简洁使用步骤</span>
              </h3>
            </CardHeader>
            <CardContent class="p-5 pt-0">
              <ol class="space-y-2 text-xs text-muted-foreground">
                <li
                  v-for="(step, idx) in props.skill.details.usageSteps"
                  :key="idx"
                  class="flex items-start gap-2.5"
                >
                  <span class="font-semibold text-primary">{{ idx + 1 }}.</span>
                  <span class="leading-relaxed">{{ step }}</span>
                </li>
              </ol>
            </CardContent>
          </Card>
        </div>
      </div>

      <!-- 右侧安装与下载卡片 -->
      <div class="space-y-4 lg:col-span-1">
        <Card class="sticky top-6 border border-border shadow-sm">
          <CardHeader class="p-5 pb-3">
            <h3 class="text-base font-semibold tracking-tight text-foreground">
              安装与获取
            </h3>
            <p class="text-xs text-muted-foreground">
              选择目标 AI 助手进行安装，或直接下载技能归档包。
            </p>
          </CardHeader>

          <CardContent class="p-5 pt-2 space-y-4">
            <!-- 目标助手多选 -->
            <div class="space-y-2">
              <label class="text-xs font-medium text-foreground">
                目标 AI 助手（支持多选）
              </label>
              <TargetToolSelector
                v-model="selectedToolIds"
                :available-tools="props.availableTools"
                :installed-tools="props.installedTools"
              />
            </div>

            <!-- 安装按钮 -->
            <Button
              class="w-full h-9 text-xs font-medium"
              :disabled="selectedToolIds.length === 0"
              @click="handleConfirmInstall"
            >
              <Download class="mr-1.5 h-4 w-4" />
              <span>安装到已选助手{{ selectedToolIds.length > 0 ? ` (${selectedToolIds.length})` : '' }}</span>
            </Button>

            <div class="relative flex items-center justify-center py-1">
              <div class="absolute inset-0 flex items-center">
                <div class="w-full border-t border-border/60" />
              </div>
              <span class="relative bg-card px-2 text-[11px] text-muted-foreground">或</span>
            </div>

            <!-- 直接下载链接 -->
            <Button
              as-child
              variant="outline"
              class="w-full h-9 text-xs font-normal"
            >
              <a :href="props.skill.zipDownloadUrl" :download="props.skill.zipFileName" aria-label="直接下载技能包">
                <Download class="mr-1.5 h-4 w-4" />
                <span>直接下载技能包 (.zip)</span>
              </a>
            </Button>

            <!-- 提示信息 -->
            <div class="rounded-lg bg-muted/40 p-3 text-[11px] text-muted-foreground leading-relaxed">
              安装后，可在目标 AI 助手对话中通过自然语言直接调用此技能控制设备。
            </div>
          </CardContent>
        </Card>
      </div>
    </div>
  </div>
</template>
