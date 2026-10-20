<script setup lang="ts">
import type { AiTool, AiToolId, SkillInstallTargetResult, SkillItem } from '@/api/aiIntegrations'
import { computed, onMounted, onUnmounted, ref, watch } from 'vue'
import { useRoute, useRouter } from 'vue-router'
import {
  AlertCircle,
  ArrowLeft,
  Check,
  CheckCircle2,
  ChevronRight,
  Copy,
  Download,
  Folder,
  Info,
  Layers,
  ListOrdered,
  Loader2,
  Radio,
  RotateCcw,
  RotateCw,
  Smartphone,
  Sparkles,
  Zap,
  Bot,
} from 'lucide-vue-next'
import { Badge } from '@/components/ui/badge'
import { Button } from '@/components/ui/button'
import { Card, CardContent, CardHeader } from '@/components/ui/card'
import {
  Empty,
  EmptyContent,
  EmptyDescription,
  EmptyHeader,
  EmptyMedia,
  EmptyTitle,
} from '@/components/ui/empty'
import AppHeader from '@/components/layout/AppHeader.vue'
import HomeAuroraBackground from '@/components/home/HomeAuroraBackground.vue'
import TargetToolSelector from '@/components/ai-integration/TargetToolSelector.vue'
import { ApiError, downloadSkillZip, fetchAiIntegrations, fetchSkillDetail, installSkill } from '@/api/aiIntegrations'

const route = useRoute()
const router = useRouter()

const skillId = computed(() => String(route.params.id || ''))

// 加载状态
const isLoading = ref(true)
const skillNotFound = ref(false)
const loadError = ref<string | null>(null)

const skill = ref<SkillItem | null>(null)
const availableTools = ref<AiTool[]>([])

// 当前选中的主工具（从 query 或默认 codex）
const currentToolId = computed<AiToolId>(() => {
  const q = String(route.query.tool || '')
  if (['codex', 'claude-code', 'workbuddy'].includes(q)) {
    return q as AiToolId
  }
  return 'codex'
})

// 多选目标助手列表
const selectedToolIds = ref<AiToolId[]>([])

// 当前技能已安装的助手列表
const installedTools = computed<AiToolId[]>(() => {
  return skill.value ? skill.value.installedTargets : []
})

function initSelectedTools() {
  if (skill.value && !installedTools.value.includes(currentToolId.value)) {
    selectedToolIds.value = [currentToolId.value]
  }
  else {
    selectedToolIds.value = []
  }
}

// 示例提示词复制状态与清理
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

// 安装执行状态
const isInstalling = ref(false)
const lastInstallResults = ref<SkillInstallTargetResult[] | null>(null)
const installError = ref<string | null>(null)
const isDownloading = ref(false)
const downloadError = ref<string | null>(null)
const loadSeq = ref(0)
const installSeq = ref(0)
const downloadSeq = ref(0)

function resetDownloadState() {
  downloadSeq.value++
  isDownloading.value = false
  downloadError.value = null
}

function resetInstallState() {
  installSeq.value++
  isInstalling.value = false
  installError.value = null
  lastInstallResults.value = null
}

const hasFailures = computed(() => {
  return Boolean(lastInstallResults.value && lastInstallResults.value.some(r => r.status === 'failed'))
})

async function handleConfirmInstall() {
  if (!skill.value)
    return
  const targetSkillId = skill.value.id
  const targets = selectedToolIds.value.filter(
    id => !installedTools.value.includes(id),
  )
  if (targets.length === 0 || isInstalling.value)
    return

  installSeq.value++
  const currentActionSeq = installSeq.value

  isInstalling.value = true
  installError.value = null
  lastInstallResults.value = null

  try {
    const res = await installSkill(targetSkillId, targets)
    if (installSeq.value !== currentActionSeq || skillId.value !== targetSkillId)
      return
    lastInstallResults.value = res.results

    // 重新拉取最新安装状态
    const updated = await fetchSkillDetail(targetSkillId)
    if (installSeq.value !== currentActionSeq || skillId.value !== targetSkillId)
      return
    skill.value = updated

    // 仅保留失败目标供用户直接重试
    const failed = res.results.filter(r => r.status === 'failed').map(r => r.target)
    if (failed.length > 0) {
      selectedToolIds.value = failed
    }
    else {
      initSelectedTools()
    }
  }
  catch (err: unknown) {
    if (installSeq.value !== currentActionSeq || skillId.value !== targetSkillId)
      return
    installError.value = err instanceof Error ? err.message : '安装请求失败'
  }
  finally {
    if (installSeq.value === currentActionSeq && skillId.value === targetSkillId) {
      isInstalling.value = false
    }
  }
}

async function handleDownloadSkill() {
  if (!skill.value || isDownloading.value)
    return
  const targetSkillId = skill.value.id
  const actionSeq = ++downloadSeq.value
  isDownloading.value = true
  downloadError.value = null
  try {
    await downloadSkillZip(skill.value.id, skill.value.zipFileName)
  }
  catch (err: unknown) {
    if (downloadSeq.value !== actionSeq || skillId.value !== targetSkillId)
      return
    downloadError.value = err instanceof Error ? err.message : '下载失败'
  }
  finally {
    if (downloadSeq.value === actionSeq && skillId.value === targetSkillId)
      isDownloading.value = false
  }
}

// 加载技能数据
async function loadDetail() {
  if (!skillId.value) {
    skillNotFound.value = true
    isLoading.value = false
    return
  }

  loadSeq.value++
  const currentSeq = loadSeq.value

  isLoading.value = true
  skillNotFound.value = false
  loadError.value = null

  try {
    const [detail, tools] = await Promise.all([
      fetchSkillDetail(skillId.value),
      fetchAiIntegrations(),
    ])
    if (loadSeq.value !== currentSeq)
      return
    skill.value = detail
    availableTools.value = tools
    initSelectedTools()
  }
  catch (err: unknown) {
    if (loadSeq.value !== currentSeq)
      return
    if (err instanceof ApiError && err.status === 404) {
      skillNotFound.value = true
    }
    else {
      loadError.value = err instanceof Error ? err.message : '加载技能详情失败'
    }
  }
  finally {
    if (loadSeq.value === currentSeq) {
      isLoading.value = false
    }
  }
}

function handleBackToMarket() {
  router.push({
    path: '/ai-integration',
    query: route.query,
  })
}

watch(skillId, () => {
  resetCopyState()
  resetInstallState()
  resetDownloadState()
  skill.value = null
  selectedToolIds.value = []
  loadDetail()
})

onMounted(() => {
  loadDetail()
})

onUnmounted(() => {
  loadSeq.value++
  resetCopyState()
  resetInstallState()
  resetDownloadState()
})
</script>

<template>
  <div class="relative min-h-screen text-slate-900 dark:text-slate-100 select-none pb-20 overflow-x-hidden">
    <!-- 极光光斑渐变与星芒网格背景 -->
    <HomeAuroraBackground />

    <AppHeader
      back
      title="技能详情"
      :subtitle="skill?.name"
      :icon="Bot"
      @back="handleBackToMarket"
    />

    <main class="relative z-10 flex-1 pb-16 pt-5">
      <div class="mx-auto max-w-6xl px-4 sm:px-6">
        <!-- 加载中 -->
        <div v-if="isLoading" class="flex flex-col items-center justify-center py-24 text-muted-foreground">
          <Loader2 class="h-8 w-8 animate-spin" />
          <p class="mt-3 text-sm">正在加载技能详情...</p>
        </div>

        <!-- 未找到该技能 -->
        <div v-else-if="skillNotFound" class="py-24 text-center">
          <Empty>
            <EmptyMedia>
              <AlertCircle class="h-10 w-10 text-muted-foreground" />
            </EmptyMedia>
            <EmptyHeader>
              <EmptyTitle>未找到该技能</EmptyTitle>
              <EmptyDescription>该技能不存在或已下架，请返回技能市场查看其他技能。</EmptyDescription>
            </EmptyHeader>
            <EmptyContent>
              <Button variant="outline" class="gap-1.5" @click="handleBackToMarket">
                <ArrowLeft class="h-4 w-4" />
                返回技能市场
              </Button>
            </EmptyContent>
          </Empty>
        </div>

        <!-- 加载失败 -->
        <div v-else-if="loadError" class="py-24 text-center">
          <Empty>
            <EmptyMedia>
              <AlertCircle class="h-10 w-10 text-destructive" />
            </EmptyMedia>
            <EmptyHeader>
              <EmptyTitle>加载失败</EmptyTitle>
              <EmptyDescription>{{ loadError }}</EmptyDescription>
            </EmptyHeader>
            <EmptyContent>
              <Button variant="outline" class="gap-1.5" @click="loadDetail">
                <RotateCcw class="h-4 w-4" />
                重新加载
              </Button>
            </EmptyContent>
          </Empty>
        </div>

        <!-- 技能详情主体 -->
        <div v-else-if="skill" class="space-y-6">
          <!-- 面包屑与返回按钮 -->
          <div class="flex flex-wrap items-center justify-between gap-3 border-b pb-4">
            <nav class="flex items-center gap-1.5 text-xs text-muted-foreground" aria-label="面包屑导航">
              <button
                type="button"
                class="hover:text-foreground transition-colors cursor-pointer"
                @click="handleBackToMarket"
              >
                技能市场
              </button>
              <ChevronRight class="h-3.5 w-3.5" />
              <span class="text-foreground font-medium truncate max-w-[200px] sm:max-w-none">
                {{ skill.name }}
              </span>
            </nav>

            <Button
              variant="outline"
              size="sm"
              class="h-8 gap-1.5 text-xs font-normal"
              @click="handleBackToMarket"
            >
              <ArrowLeft class="h-3.5 w-3.5" />
              返回市场
            </Button>
          </div>

          <!-- 技能头部主卡片 -->
          <div class="rounded-xl border bg-card p-4 sm:p-6 shadow-sm">
            <div class="flex flex-col gap-4 sm:flex-row sm:items-start sm:justify-between">
              <div class="flex items-start gap-4">
                <!-- 图标 -->
                <div
                  v-if="skill.iconType === 'kuaishou'"
                  class="flex h-12 w-12 shrink-0 items-center justify-center rounded-xl bg-gradient-to-br from-amber-500 to-orange-500 text-white shadow-sm"
                >
                  <svg class="h-6 w-6" viewBox="0 0 24 24" fill="currentColor">
                    <rect x="2" y="5" width="14" height="14" rx="3" />
                    <path d="M16 10l5-3v10l-5-3v-4z" />
                  </svg>
                </div>
                <div
                  v-else-if="skill.iconType === 'wechat'"
                  class="flex h-12 w-12 shrink-0 items-center justify-center rounded-xl bg-emerald-500 text-white shadow-sm"
                >
                  <svg class="h-6 w-6" viewBox="0 0 24 24" fill="currentColor">
                    <path d="M9 4C5.13 4 2 6.69 2 10c0 1.88 1.02 3.55 2.62 4.67-.11.66-.46 2.05-.53 2.37-.1.44.17.43.36.3.26-.17 2.39-1.63 2.8-1.92.56.13 1.15.2 1.75.2 3.87 0 7-2.69 7-6s-3.13-6-7-6zm7 8c-.34 0-.67.03-.99.08.62.86.99 1.87.99 2.92 0 1.57-.84 2.99-2.19 3.96.09.55.39 1.7.45 1.97.08.37-.14.36-.3.25-.22-.14-2-1.36-2.34-1.6-.52.12-1.07.19-1.62.19-1.89 0-3.6-.66-4.83-1.74C7.03 19.35 9.38 20.5 12 20.5c.53 0 1.05-.05 1.55-.16.36.25 2.22 1.5 2.45 1.65.17.11.4.12.31-.26-.06-.27-.37-1.45-.46-2.02 1.4-1 2.29-2.45 2.29-4.08 0-2.88-2.73-5.23-6.14-5.63z" />
                  </svg>
                </div>
                <div
                  v-else
                  class="flex h-12 w-12 shrink-0 items-center justify-center rounded-xl bg-blue-600 text-white shadow-sm"
                >
                  <Smartphone class="h-6 w-6" />
                </div>

                <div class="space-y-1">
                  <div class="flex flex-wrap items-center gap-2">
                    <span class="text-xs font-semibold text-muted-foreground uppercase tracking-wider">
                      {{ skill.app }}
                    </span>
                    <Badge variant="secondary" class="text-xs font-normal">
                      {{ skill.categoryLabel }}
                    </Badge>
                  </div>
                  <h1 class="text-xl sm:text-2xl font-bold tracking-tight text-foreground">
                    {{ skill.name }}
                  </h1>
                  <p class="text-sm text-muted-foreground">
                    {{ skill.description }}
                  </p>
                </div>
              </div>
            </div>
          </div>

          <!-- 双栏主体：左侧富详情，右侧安装/下载操作面板 -->
          <div class="grid grid-cols-1 gap-6 lg:grid-cols-3">
            <!-- 左侧内容栏 (占 2 栏) -->
            <div class="space-y-6 lg:col-span-2">
              <!-- 概览与定位 -->
              <Card class="gap-0 py-0 overflow-hidden shadow-sm">
                <CardHeader class="p-4 pb-2 border-b bg-muted/20">
                  <div class="flex items-center gap-2">
                    <Info class="h-4 w-4 text-primary" />
                    <h2 class="text-sm font-semibold text-foreground">概览说明</h2>
                  </div>
                </CardHeader>
                <CardContent class="p-4 text-xs sm:text-sm text-muted-foreground leading-relaxed">
                  {{ skill.details?.overview }}
                </CardContent>
              </Card>

              <!-- 核心能力 -->
              <Card v-if="skill.details?.capabilities && skill.details.capabilities.length > 0" class="gap-0 py-0 overflow-hidden shadow-sm">
                <CardHeader class="p-4 pb-2 border-b bg-muted/20">
                  <div class="flex items-center gap-2">
                    <Zap class="h-4 w-4 text-primary" />
                    <h2 class="text-sm font-semibold text-foreground">核心能力</h2>
                  </div>
                </CardHeader>
                <CardContent class="p-4 space-y-3">
                  <div
                    v-for="(cap, i) in skill.details.capabilities"
                    :key="i"
                    class="rounded-lg border bg-card p-3 space-y-1"
                  >
                    <div class="font-medium text-xs sm:text-sm text-foreground flex items-center gap-2">
                      <span class="flex h-5 w-5 shrink-0 items-center justify-center rounded-full bg-primary/10 text-[11px] font-semibold text-primary">
                        {{ i + 1 }}
                      </span>
                      {{ cap.title }}
                    </div>
                    <p class="text-xs text-muted-foreground pl-7 leading-relaxed">
                      {{ cap.description }}
                    </p>
                  </div>
                </CardContent>
              </Card>

              <!-- 典型使用场景 -->
              <Card v-if="skill.details?.scenarios && skill.details.scenarios.length > 0" class="gap-0 py-0 overflow-hidden shadow-sm">
                <CardHeader class="p-4 pb-2 border-b bg-muted/20">
                  <div class="flex items-center gap-2">
                    <Radio class="h-4 w-4 text-primary" />
                    <h2 class="text-sm font-semibold text-foreground">典型场景</h2>
                  </div>
                </CardHeader>
                <CardContent class="p-4">
                  <ul class="space-y-2 text-xs sm:text-sm text-muted-foreground list-none">
                    <li
                      v-for="(scene, i) in skill.details.scenarios"
                      :key="i"
                      class="flex items-start gap-2.5"
                    >
                      <CheckCircle2 class="h-4 w-4 shrink-0 text-primary mt-0.5" />
                      <span>{{ scene }}</span>
                    </li>
                  </ul>
                </CardContent>
              </Card>

              <!-- 任务示例提示词 -->
              <Card v-if="skill.details?.samplePrompts && skill.details.samplePrompts.length > 0" class="gap-0 py-0 overflow-hidden shadow-sm">
                <CardHeader class="p-4 pb-2 border-b bg-muted/20">
                  <div class="flex items-center gap-2">
                    <Sparkles class="h-4 w-4 text-primary" />
                    <h2 class="text-sm font-semibold text-foreground">可复制任务示例</h2>
                  </div>
                </CardHeader>
                <CardContent class="p-4 space-y-2.5">
                  <div
                    v-for="(prompt, i) in skill.details.samplePrompts"
                    :key="i"
                    class="flex items-center justify-between gap-3 rounded-lg bg-muted/50 p-3 text-xs sm:text-sm"
                  >
                    <span class="text-foreground select-all leading-relaxed">{{ prompt }}</span>
                    <Button
                      variant="outline"
                      size="sm"
                      class="h-7 px-2 text-xs font-normal shrink-0"
                      @click="handleCopyPrompt(prompt, i)"
                    >
                      <Check v-if="copyingIndex === i" class="mr-1.5 h-3.5 w-3.5 text-green-600 dark:text-green-400" />
                      <Copy v-else class="mr-1.5 h-3.5 w-3.5" />
                      {{ copyingIndex === i ? '已复制' : '复制' }}
                    </Button>
                  </div>
                  <p v-if="copyError" role="alert" class="text-xs text-destructive">
                    {{ copyError }}
                  </p>
                </CardContent>
              </Card>

              <!-- 准备条件与步骤 -->
              <div class="grid grid-cols-1 sm:grid-cols-2 gap-4">
                <!-- 准备条件 -->
                <Card v-if="skill.details?.prerequisites && skill.details.prerequisites.length > 0" class="gap-0 py-0 overflow-hidden shadow-sm">
                  <CardHeader class="p-3.5 pb-2 border-b bg-muted/20">
                    <div class="flex items-center gap-2">
                      <Layers class="h-4 w-4 text-primary" />
                      <h3 class="text-xs font-semibold text-foreground">准备条件</h3>
                    </div>
                  </CardHeader>
                  <CardContent class="p-3.5">
                    <ul class="space-y-2 text-xs text-muted-foreground list-none">
                      <li
                        v-for="(pre, i) in skill.details.prerequisites"
                        :key="i"
                        class="flex items-start gap-2"
                      >
                        <span class="h-1.5 w-1.5 rounded-full bg-primary/80 mt-1.5 shrink-0" />
                        <span>{{ pre }}</span>
                      </li>
                    </ul>
                  </CardContent>
                </Card>

                <!-- 使用步骤 -->
                <Card v-if="skill.details?.usageSteps && skill.details.usageSteps.length > 0" class="gap-0 py-0 overflow-hidden shadow-sm">
                  <CardHeader class="p-3.5 pb-2 border-b bg-muted/20">
                    <div class="flex items-center gap-2">
                      <ListOrdered class="h-4 w-4 text-primary" />
                      <h3 class="text-xs font-semibold text-foreground">使用步骤</h3>
                    </div>
                  </CardHeader>
                  <CardContent class="p-3.5">
                    <ol class="space-y-2 text-xs text-muted-foreground list-none">
                      <li
                        v-for="(step, i) in skill.details.usageSteps"
                        :key="i"
                        class="flex items-start gap-2"
                      >
                        <span class="font-semibold text-foreground text-[11px] shrink-0 mt-0.5">{{ i + 1 }}.</span>
                        <span>{{ step }}</span>
                      </li>
                    </ol>
                  </CardContent>
                </Card>
              </div>
            </div>

            <!-- 右侧操作栏 (占 1 栏，桌面粘滞固定) -->
            <div class="space-y-4 lg:sticky lg:top-20 h-fit">
              <!-- 多选安装卡片 -->
              <Card class="gap-0 py-0 overflow-hidden shadow-sm border border-border">
                <CardHeader class="p-4 pb-2 border-b bg-muted/20">
                  <div class="flex items-center gap-2">
                    <Download class="h-4 w-4 text-primary" />
                    <h3 class="text-sm font-semibold text-foreground">安装至 AI 助手</h3>
                  </div>
                </CardHeader>
                <CardContent class="p-4 space-y-4">
                  <p class="text-xs text-muted-foreground">
                    选择一个或多个 AI 助手目标进行一键写入安装：
                  </p>

                  <TargetToolSelector
                    v-model="selectedToolIds"
                    :available-tools="availableTools"
                    :installed-tools="installedTools"
                    :disabled="isInstalling"
                  />

                  <!-- 宿主机说明提示 -->
                  <div class="rounded-lg bg-muted/40 p-2.5 text-[11px] text-muted-foreground leading-normal flex items-start gap-1.5">
                    <Folder class="h-3.5 w-3.5 shrink-0 mt-0.5 text-foreground" />
                    <span>安装写入运行 MyWebScrcpy 服务的电脑，已存在且被用户修改的文件将受保护不被覆盖。</span>
                  </div>

                  <!-- 最近安装反馈 -->
                  <div v-if="lastInstallResults && lastInstallResults.length > 0" class="space-y-1.5 rounded-lg border p-2.5 text-xs">
                    <div
                      v-for="res in lastInstallResults"
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
                          {{ availableTools.find(t => t.id === res.target)?.name || res.target }}:
                        </span>
                        <span v-if="res.status === 'installed'" class="text-green-600 ml-1">安装成功</span>
                        <span v-else-if="res.status === 'already_installed'" class="text-muted-foreground ml-1">已安装</span>
                        <span v-else class="text-destructive ml-1">{{ res.error || '安装失败' }}</span>
                      </div>
                    </div>
                  </div>

                  <!-- 错误信息 -->
                  <div v-if="installError" class="rounded-lg bg-destructive/10 p-2.5 text-xs text-destructive flex items-start gap-2">
                    <AlertCircle class="h-4 w-4 shrink-0 mt-0.5" />
                    <span class="break-all [overflow-wrap:anywhere]">{{ installError }}</span>
                  </div>

                  <Button
                    class="w-full h-9 text-xs font-medium"
                    :disabled="selectedToolIds.length === 0 || isInstalling"
                    @click="handleConfirmInstall"
                  >
                    <Loader2 v-if="isInstalling" class="mr-1.5 h-3.5 w-3.5 animate-spin" />
                    <RotateCw v-else-if="hasFailures" class="mr-1.5 h-3.5 w-3.5" />
                    <Download v-else class="mr-1.5 h-3.5 w-3.5" />
                    {{ isInstalling ? '正在安装...' : hasFailures ? '重试安装' : '确认安装' }}
                  </Button>
                </CardContent>
              </Card>

              <!-- 直接下载卡片 -->
              <Card class="gap-0 py-0 overflow-hidden shadow-sm border border-border">
                <CardHeader class="p-4 pb-2 border-b bg-muted/20">
                  <h3 class="text-xs font-semibold text-foreground">独立技能指令包</h3>
                </CardHeader>
                <CardContent class="p-4 space-y-3">
                  <p class="text-xs text-muted-foreground">
                    直接下载包含完整 <code>SKILL.md</code> 与通用请求客户端的 ZIP 压缩包，不改变任何安装记录。
                  </p>
                  <Button
                    variant="outline"
                    class="w-full h-8 text-xs font-normal"
                    :disabled="isDownloading"
                    @click="handleDownloadSkill"
                  >
                    <Loader2 v-if="isDownloading" class="mr-1.5 h-3.5 w-3.5 animate-spin" />
                    <Download v-else class="mr-1.5 h-3.5 w-3.5" />
                    直接下载技能包 (ZIP)
                  </Button>
                  <p v-if="downloadError" class="text-[11px] text-destructive break-all [overflow-wrap:anywhere]">
                    {{ downloadError }}
                  </p>
                </CardContent>
              </Card>
            </div>
          </div>
        </div>
      </div>
    </main>
  </div>
</template>
