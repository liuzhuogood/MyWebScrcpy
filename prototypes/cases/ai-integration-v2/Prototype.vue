<script setup lang="ts">
import type { AiTool, AiToolId, InstalledMap, SkillId, SkillItem } from './types'
import { computed, onMounted, onUnmounted, ref } from 'vue'
import { AlertCircle, ArrowLeft } from 'lucide-vue-next'
import { Button } from '@/components/ui/button'
import {
  Empty,
  EmptyContent,
  EmptyDescription,
  EmptyHeader,
  EmptyMedia,
  EmptyTitle,
} from '@/components/ui/empty'
import AiGuidePanel from './components/AiGuidePanel.vue'
import AiToolSelector from './components/AiToolSelector.vue'
import HeaderNav from './components/HeaderNav.vue'
import InstallDialog from './components/InstallDialog.vue'
import SkillDetailPage from './components/SkillDetailPage.vue'
import SkillMarket from './components/SkillMarket.vue'
import { aiToolMap, aiTools, initialInstalledMap, skills } from './data'

// 当前选中的 AI 工具，默认 Codex
const selectedToolId = ref<AiToolId>('codex')

// 安装状态记录表：Record<SkillId, AiToolId[]>
const installedMap = ref<InstalledMap>({
  'device-base': [...initialInstalledMap['device-base']],
  'kuaishou-daily': [...initialInstalledMap['kuaishou-daily']],
  'wechat-helper': [...initialInstalledMap['wechat-helper']],
})

// 当前选中的 AI 工具配置
const currentAiTool = computed<AiTool>(() => {
  return aiToolMap[selectedToolId.value]
})

// 路由与二级页状态
const currentSkillId = ref<SkillId | null>(null)
const unknownSkillParam = ref<string | null>(null)
const savedScrollY = ref(0)

const currentDetailSkill = computed<SkillItem | null>(() => {
  if (!currentSkillId.value)
    return null
  const found = skills.find(s => s.id === currentSkillId.value)
  return found ?? null
})

// 安装弹窗状态与目标技能
const installDialogOpen = ref(false)
const activeInstallSkill = ref<SkillItem | null>(null)

// 当前安装弹窗目标技能已安装工具列表
const activeInstallSkillInstalledTools = computed<AiToolId[]>(() => {
  if (!activeInstallSkill.value) {
    return installedMap.value['device-base']
  }
  return installedMap.value[activeInstallSkill.value.id]
})

function parseRouteFromUrl() {
  const params = new URLSearchParams(window.location.search)
  const skillParam = params.get('skill')
  if (!skillParam) {
    currentSkillId.value = null
    unknownSkillParam.value = null
    return
  }
  const matched = skills.find(s => s.id === skillParam)
  if (matched) {
    currentSkillId.value = matched.id
    unknownSkillParam.value = null
  }
  else {
    currentSkillId.value = null
    unknownSkillParam.value = skillParam
  }
}

function handlePopState() {
  parseRouteFromUrl()
  if (!currentSkillId.value && !unknownSkillParam.value) {
    requestAnimationFrame(() => {
      window.scrollTo({ top: savedScrollY.value, behavior: 'instant' })
    })
  }
}

function handleSelectTool(id: AiToolId) {
  selectedToolId.value = id
}

function handleViewDetail(skill: SkillItem) {
  savedScrollY.value = window.scrollY
  currentSkillId.value = skill.id
  unknownSkillParam.value = null

  const url = new URL(window.location.href)
  url.searchParams.set('skill', skill.id)
  window.history.pushState({ skillId: skill.id }, '', url.toString())
  window.scrollTo({ top: 0, behavior: 'instant' })
}

function handleBackToMarket() {
  const url = new URL(window.location.href)
  url.searchParams.delete('skill')
  window.history.pushState({}, '', url.toString())
  currentSkillId.value = null
  unknownSkillParam.value = null

  requestAnimationFrame(() => {
    window.scrollTo({ top: savedScrollY.value, behavior: 'instant' })
  })
}

function handleOpenInstall(skill: SkillItem) {
  activeInstallSkill.value = skill
  installDialogOpen.value = true
}

function handleOpenInstallBase() {
  const baseSkill = skills.find(s => s.id === 'device-base')
  activeInstallSkill.value = baseSkill ?? null
  installDialogOpen.value = true
}

function handleConfirmInstallMultiple(skillId: SkillId, targetToolIds: AiToolId[]) {
  const currentList = installedMap.value[skillId]
  const newTools = targetToolIds.filter(id => !currentList.includes(id))
  if (newTools.length > 0) {
    installedMap.value[skillId] = [...currentList, ...newTools]
  }
}

function handleConfirmInstallFromDialog(targetToolIds: AiToolId[]) {
  const targetSkillId: SkillId = activeInstallSkill.value ? activeInstallSkill.value.id : 'device-base'
  handleConfirmInstallMultiple(targetSkillId, targetToolIds)
}

onMounted(() => {
  parseRouteFromUrl()
  window.addEventListener('popstate', handlePopState)
})

onUnmounted(() => {
  window.removeEventListener('popstate', handlePopState)
})
</script>

<template>
  <div class="-m-4 -my-8 min-h-screen bg-background text-foreground sm:-m-6 selection:bg-primary/20">
    <!-- 顶部应用导航栏 -->
    <HeaderNav />

    <!-- 核心页面容器 -->
    <main class="mx-auto max-w-7xl px-3.5 py-5 sm:px-6 sm:py-6">
      <!-- 路由分支 1：未知技能 ID 缺省页面 -->
      <div v-if="unknownSkillParam" class="rounded-xl border border-dashed border-border py-12">
        <Empty>
          <EmptyMedia variant="icon">
            <AlertCircle class="h-6 w-6 text-destructive" />
          </EmptyMedia>
          <EmptyHeader>
            <EmptyTitle class="text-base font-semibold">未找到该技能</EmptyTitle>
            <EmptyDescription class="text-xs text-muted-foreground">
              未检索到技能标识为「{{ unknownSkillParam }}」的技能资料，请核对链接或返回技能市场浏览。
            </EmptyDescription>
          </EmptyHeader>
          <EmptyContent>
            <Button
              variant="outline"
              size="sm"
              class="h-8 text-xs"
              @click="handleBackToMarket"
            >
              <ArrowLeft class="mr-1.5 h-3.5 w-3.5" />
              返回技能市场
            </Button>
          </EmptyContent>
        </Empty>
      </div>

      <!-- 路由分支 2：已知技能二级详情页 -->
      <SkillDetailPage
        v-else-if="currentDetailSkill"
        :skill="currentDetailSkill"
        :current-tool-id="selectedToolId"
        :available-tools="aiTools"
        :installed-tools="installedMap[currentDetailSkill.id]"
        @back="handleBackToMarket"
        @install="handleConfirmInstallMultiple(currentDetailSkill.id, $event)"
      />

      <!-- 路由分支 3：市场主页（使用 v-show 保持搜索/筛选/滚动状态） -->
      <div v-show="!currentSkillId && !unknownSkillParam" class="space-y-6 sm:space-y-7">
        <!-- 页面标题与面包屑 -->
        <div class="space-y-1">
          <div class="text-xs text-muted-foreground">
            工作台 / AI 集成
          </div>
          <h1 class="text-xl font-bold tracking-tight text-foreground sm:text-2xl">
            AI 集成
          </h1>
          <p class="text-xs text-muted-foreground">
            连接 AI 助手，使用技能自动化操作手机
          </p>
        </div>

        <!-- 上半区：连接你的 AI 助手 -->
        <section class="space-y-3.5" aria-label="AI 助手接入引导">
          <AiToolSelector
            :tools="aiTools"
            :selected-id="selectedToolId"
            @select="handleSelectTool"
          />

          <AiGuidePanel
            :tool="currentAiTool"
            @install-base="handleOpenInstallBase"
          />
        </section>

        <!-- 下半区：技能市场 -->
        <SkillMarket
          :skills="skills"
          :current-tool-id="selectedToolId"
          :installed-map="installedMap"
          @view-detail="handleViewDetail"
          @install="handleOpenInstall"
        />
      </div>
    </main>

    <!-- 多选下载安装弹窗 -->
    <InstallDialog
      v-model:open="installDialogOpen"
      :skill="activeInstallSkill"
      :default-tool-id="selectedToolId"
      :available-tools="aiTools"
      :installed-tools="activeInstallSkillInstalledTools"
      @confirm="handleConfirmInstallFromDialog"
    />
  </div>
</template>
