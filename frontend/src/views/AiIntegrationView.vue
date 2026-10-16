<script setup lang="ts">
import type { AiTool, AiToolId, SkillCategory, SkillInstallTargetResult, SkillItem } from '@/api/aiIntegrations'
import { computed, nextTick, onMounted, onUnmounted, ref, watch } from 'vue'
import { useRoute, useRouter } from 'vue-router'
import { AlertCircle, Loader2, RotateCcw } from 'lucide-vue-next'
import { Button } from '@/components/ui/button'
import {
  Empty,
  EmptyContent,
  EmptyDescription,
  EmptyHeader,
  EmptyMedia,
  EmptyTitle,
} from '@/components/ui/empty'
import AppHeader from '@/components/layout/AppHeader.vue'
import AiGuidePanel from '@/components/ai-integration/AiGuidePanel.vue'
import AiToolSelector from '@/components/ai-integration/AiToolSelector.vue'
import InstallDialog from '@/components/ai-integration/InstallDialog.vue'
import SkillMarket from '@/components/ai-integration/SkillMarket.vue'
import { fetchAiIntegrations, fetchSkills, installSkill } from '@/api/aiIntegrations'

const route = useRoute()
const router = useRouter()

// 状态管理
const isLoading = ref(true)
const loadError = ref<string | null>(null)
const loadSeq = ref(0)

const tools = ref<AiTool[]>([])
const skills = ref<SkillItem[]>([])

// 筛选与选择状态（优先读取 URL query，支持浏览器前进后退与分享）
const selectedToolId = ref<AiToolId>('codex')
const searchQuery = ref('')
const selectedCategory = ref<SkillCategory>('all')
const onlyInstalled = ref(false)

let isSyncingFromRoute = false

function initFiltersFromRoute() {
  isSyncingFromRoute = true
  try {
    const qTool = route.query.tool
    if (typeof qTool === 'string' && ['codex', 'claude-code', 'workbuddy'].includes(qTool)) {
      selectedToolId.value = qTool as AiToolId
    }
    else {
      selectedToolId.value = 'codex'
    }

    const qSearch = route.query.q
    if (typeof qSearch === 'string') {
      searchQuery.value = qSearch
    }
    else {
      searchQuery.value = ''
    }

    const qCat = route.query.cat
    if (typeof qCat === 'string' && ['all', 'social', 'short-video', 'productivity'].includes(qCat)) {
      selectedCategory.value = qCat as SkillCategory
    }
    else {
      selectedCategory.value = 'all'
    }

    onlyInstalled.value = route.query.inst === '1'
  }
  finally {
    nextTick(() => {
      isSyncingFromRoute = false
    })
  }
}

function syncFiltersToRoute() {
  if (isSyncingFromRoute)
    return
  const query: Record<string, string | undefined> = {
    ...route.query,
    tool: selectedToolId.value !== 'codex' ? selectedToolId.value : undefined,
    q: searchQuery.value || undefined,
    cat: selectedCategory.value !== 'all' ? selectedCategory.value : undefined,
    inst: onlyInstalled.value ? '1' : undefined,
  }
  router.replace({ query })
}

watch([selectedToolId, searchQuery, selectedCategory, onlyInstalled], () => {
  syncFiltersToRoute()
})

watch(() => route.query, () => {
  initFiltersFromRoute()
})

// 安装映射表 Record<skillId, AiToolId[]>
const installedMap = computed<Record<string, AiToolId[]>>(() => {
  const map: Record<string, AiToolId[]> = {}
  for (const s of skills.value) {
    map[s.id] = s.installedTargets
  }
  return map
})

// 当前选中的 AI 工具
const currentAiTool = computed<AiTool | null>(() => {
  return tools.value.find(t => t.id === selectedToolId.value) || tools.value[0] || null
})

// 加载数据
async function loadData() {
  loadSeq.value++
  const currentSeq = loadSeq.value

  isLoading.value = true
  loadError.value = null
  try {
    const [loadedTools, loadedSkills] = await Promise.all([
      fetchAiIntegrations(),
      fetchSkills(),
    ])
    if (loadSeq.value !== currentSeq)
      return
    tools.value = loadedTools
    skills.value = loadedSkills
  }
  catch (err: unknown) {
    if (loadSeq.value !== currentSeq)
      return
    loadError.value = err instanceof Error ? err.message : '加载 AI 集成数据失败'
  }
  finally {
    if (loadSeq.value === currentSeq) {
      isLoading.value = false
    }
  }
}

// 切换工具
function handleSelectTool(id: AiToolId) {
  selectedToolId.value = id
}

// 跳转二级详情页（带上当前市场状态作为查询参数，以便返回时完全保留）
function handleViewDetail(skill: SkillItem) {
  router.push({
    path: `/ai-integration/skills/${encodeURIComponent(skill.id)}`,
    query: {
      tool: selectedToolId.value,
      q: searchQuery.value || undefined,
      cat: selectedCategory.value !== 'all' ? selectedCategory.value : undefined,
      inst: onlyInstalled.value ? '1' : undefined,
    },
  })
}

// 安装弹窗状态
const installDialogOpen = ref(false)
const activeInstallSkill = ref<SkillItem | null>(null)
const isInstalling = ref(false)
const lastInstallResults = ref<SkillInstallTargetResult[] | null>(null)
const installError = ref<string | null>(null)
const installSeq = ref(0)
let installCloseTimer: ReturnType<typeof setTimeout> | null = null

function clearInstallTimer() {
  if (installCloseTimer) {
    clearTimeout(installCloseTimer)
    installCloseTimer = null
  }
}

function resetInstallDialogState() {
  installSeq.value++
  clearInstallTimer()
  isInstalling.value = false
  installError.value = null
  lastInstallResults.value = null
}

watch(installDialogOpen, (open) => {
  if (!open) {
    resetInstallDialogState()
  }
})

function handleOpenInstall(skill: SkillItem) {
  resetInstallDialogState()
  activeInstallSkill.value = skill
  installDialogOpen.value = true
}

function handleOpenInstallBase() {
  resetInstallDialogState()
  const baseSkill = skills.value.find(s => s.id === 'mywebscrcpy-device-base')
  activeInstallSkill.value = baseSkill || null
  installDialogOpen.value = true
}

// 实际安装调用
async function handleConfirmInstall(targetToolIds: AiToolId[]) {
  const skillToInstall = activeInstallSkill.value || skills.value.find(s => s.id === 'mywebscrcpy-device-base')
  if (!skillToInstall || isInstalling.value)
    return

  installSeq.value++
  const currentActionSeq = installSeq.value
  const targetSkillId = skillToInstall.id

  isInstalling.value = true
  installError.value = null
  lastInstallResults.value = null

  try {
    const resp = await installSkill(targetSkillId, targetToolIds)
    if (installSeq.value !== currentActionSeq || activeInstallSkill.value?.id !== targetSkillId)
      return
    lastInstallResults.value = resp.results

    // 重新从文件系统刷新技能列表以保证最新真实状态
    const updatedSkills = await fetchSkills()
    if (installSeq.value !== currentActionSeq || activeInstallSkill.value?.id !== targetSkillId)
      return
    skills.value = updatedSkills

    // 检查是否有失败项目
    const hasFailures = resp.results.some(r => r.status === 'failed')
    if (!hasFailures) {
      clearInstallTimer()
      installCloseTimer = setTimeout(() => {
        if (installSeq.value === currentActionSeq && activeInstallSkill.value?.id === targetSkillId) {
          installDialogOpen.value = false
        }
        installCloseTimer = null
      }, 1000)
    }
  }
  catch (err: unknown) {
    if (installSeq.value !== currentActionSeq || activeInstallSkill.value?.id !== targetSkillId)
      return
    installError.value = err instanceof Error ? err.message : '安装请求失败'
  }
  finally {
    if (installSeq.value === currentActionSeq && activeInstallSkill.value?.id === targetSkillId) {
      isInstalling.value = false
    }
  }
}

onMounted(() => {
  initFiltersFromRoute()
  loadData()
})

onUnmounted(() => {
  loadSeq.value++
  resetInstallDialogState()
})
</script>

<template>
  <div class="flex min-h-screen flex-col bg-background text-foreground">
    <AppHeader device-count-text="" />

    <main class="flex-1 pb-16 pt-4">
      <div class="mx-auto max-w-6xl space-y-8 px-4 sm:px-6">
        <!-- 加载中状态 -->
        <div v-if="isLoading" class="flex flex-col items-center justify-center py-24 text-muted-foreground">
          <Loader2 class="h-8 w-8 animate-spin" />
          <p class="mt-3 text-sm">正在加载 AI 集成与技能市场...</p>
        </div>

        <!-- 加载失败状态 -->
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
              <Button variant="outline" size="sm" class="gap-1.5" @click="loadData">
                <RotateCcw class="h-4 w-4" />
                重新加载
              </Button>
            </EmptyContent>
          </Empty>
        </div>

        <!-- 正常内容 -->
        <template v-else>
          <!-- 上半区：AI 工具选择 -->
          <AiToolSelector
            :tools="tools"
            :selected-id="selectedToolId"
            @select="handleSelectTool"
          />

          <!-- 对应接入指引步骤 -->
          <AiGuidePanel
            v-if="currentAiTool"
            :tool="currentAiTool"
            @install-base="handleOpenInstallBase"
          />

          <!-- 下半区：技能市场 -->
          <SkillMarket
            v-model:search-query="searchQuery"
            v-model:selected-category="selectedCategory"
            v-model:only-installed="onlyInstalled"
            :skills="skills"
            :current-tool-id="selectedToolId"
            :installed-map="installedMap"
            @view-detail="handleViewDetail"
            @install="handleOpenInstall"
          />
        </template>
      </div>
    </main>

    <!-- 安装弹窗 -->
    <InstallDialog
      v-model:open="installDialogOpen"
      :skill="activeInstallSkill"
      :default-tool-id="selectedToolId"
      :available-tools="tools"
      :installed-tools="activeInstallSkill ? (installedMap[activeInstallSkill.id] || []) : (installedMap['mywebscrcpy-device-base'] || [])"
      :is-installing="isInstalling"
      :last-results="lastInstallResults"
      :general-error="installError"
      @confirm="handleConfirmInstall"
    />
  </div>
</template>
