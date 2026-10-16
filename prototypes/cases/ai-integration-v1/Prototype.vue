<script setup lang="ts">
import type { AiTool, AiToolId, InstalledMap, SkillItem } from './types'
import { computed, ref } from 'vue'
import AiGuidePanel from './components/AiGuidePanel.vue'
import AiToolSelector from './components/AiToolSelector.vue'
import HeaderNav from './components/HeaderNav.vue'
import InstallDialog from './components/InstallDialog.vue'
import SkillDetailDialog from './components/SkillDetailDialog.vue'
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

// 当前选中的 AI 工具配置，直接读取确定的映射键
const currentAiTool = computed<AiTool>(() => {
  return aiToolMap[selectedToolId.value]
})

// 详情弹窗状态
const detailDialogOpen = ref(false)
const activeDetailSkill = ref<SkillItem | null>(null)

// 安装弹窗状态与预选目标
const installDialogOpen = ref(false)
const activeInstallSkill = ref<SkillItem | null>(null)
const installTargetToolId = ref<AiToolId>('codex')

// 详情弹窗当前技能已安装工具列表
const activeDetailSkillInstalledTools = computed<AiToolId[]>(() => {
  if (!activeDetailSkill.value)
    return []
  return installedMap.value[activeDetailSkill.value.id]
})

// 安装弹窗目标技能已安装工具列表
const activeInstallSkillInstalledTools = computed<AiToolId[]>(() => {
  if (!activeInstallSkill.value) {
    return installedMap.value['device-base']
  }
  return installedMap.value[activeInstallSkill.value.id]
})

function handleSelectTool(id: AiToolId) {
  selectedToolId.value = id
}

function handleViewDetail(skill: SkillItem) {
  activeDetailSkill.value = skill
  detailDialogOpen.value = true
}

function handleOpenInstall(skill: SkillItem) {
  activeInstallSkill.value = skill
  installTargetToolId.value = selectedToolId.value
  installDialogOpen.value = true
}

function handleOpenInstallBase() {
  activeInstallSkill.value = null
  installTargetToolId.value = selectedToolId.value
  installDialogOpen.value = true
}

// 统一路径：详情弹窗请求安装特定目标时关闭详情，打开确认弹窗并预选该目标
function handleRequestInstallFromDetail(skill: SkillItem, toolId: AiToolId) {
  activeInstallSkill.value = skill
  installTargetToolId.value = toolId
  installDialogOpen.value = true
}

function handleConfirmInstall(targetToolId: AiToolId) {
  const skillId = activeInstallSkill.value ? activeInstallSkill.value.id : 'device-base'
  const list = installedMap.value[skillId]
  if (!list.includes(targetToolId)) {
    installedMap.value[skillId] = [...list, targetToolId]
  }
}
</script>

<template>
  <div class="-m-4 -my-8 min-h-screen bg-background text-foreground sm:-m-6 selection:bg-primary/20">
    <!-- 顶部应用导航栏（纯 UI 视觉呈现，无副作用） -->
    <HeaderNav />

    <!-- 页面核心内容区域 -->
    <main class="mx-auto max-w-7xl px-3.5 py-5 sm:px-6 sm:py-6">
      <div class="space-y-6 sm:space-y-7">
        <!-- 页面标题与面包屑（已清理无响应的查看文档入口） -->
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
          <!-- 三工具选择卡 -->
          <AiToolSelector
            :tools="aiTools"
            :selected-id="selectedToolId"
            @select="handleSelectTool"
          />

          <!-- 对应接入指南面板 -->
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

    <!-- 技能详情弹窗 -->
    <SkillDetailDialog
      v-model:open="detailDialogOpen"
      :skill="activeDetailSkill"
      :current-tool-id="selectedToolId"
      :installed-tools="activeDetailSkillInstalledTools"
      :available-tools="aiTools"
      @request-install="handleRequestInstallFromDetail"
    />

    <!-- 下载安装 / 基础技能安装弹窗 -->
    <InstallDialog
      v-model:open="installDialogOpen"
      :skill="activeInstallSkill"
      :target-tool-id="installTargetToolId"
      :available-tools="aiTools"
      :installed-tools="activeInstallSkillInstalledTools"
      @confirm="handleConfirmInstall"
    />
  </div>
</template>
