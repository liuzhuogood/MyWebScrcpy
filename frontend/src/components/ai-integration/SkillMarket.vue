<script setup lang="ts">
import type { AiToolId, SkillCategory, SkillItem } from '@/api/aiIntegrations'
import { computed } from 'vue'
import { Check, ListFilter, RotateCcw, Search, X } from 'lucide-vue-next'
import { Badge } from '@/components/ui/badge'
import { Button } from '@/components/ui/button'
import {
  Empty,
  EmptyContent,
  EmptyDescription,
  EmptyHeader,
  EmptyMedia,
  EmptyTitle,
} from '@/components/ui/empty'
import { Input } from '@/components/ui/input'
import SkillCard from './SkillCard.vue'

const props = withDefaults(
  defineProps<{
    skills: SkillItem[]
    currentToolId: AiToolId
    installedMap: Record<string, AiToolId[]>
    searchQuery?: string
    selectedCategory?: SkillCategory
    onlyInstalled?: boolean
  }>(),
  {
    searchQuery: '',
    selectedCategory: 'all',
    onlyInstalled: false,
  },
)

const emit = defineEmits<{
  (e: 'view-detail', skill: SkillItem): void
  (e: 'install', skill: SkillItem): void
  (e: 'update:searchQuery', val: string): void
  (e: 'update:selectedCategory', val: SkillCategory): void
  (e: 'update:onlyInstalled', val: boolean): void
}>()

const categories: Array<{ id: SkillCategory; label: string }> = [
  { id: 'all', label: '全部' },
  { id: 'social', label: '社交' },
  { id: 'short-video', label: '短视频' },
  { id: 'productivity', label: '基础能力' },
]

// 统计当前选中的 AI 工具下已安装的技能数量
const installedCountOnCurrentTool = computed(() => {
  return props.skills.filter((s) => {
    const list = props.installedMap[s.id] || []
    return list.includes(props.currentToolId)
  }).length
})

// 过滤技能列表
const filteredSkills = computed(() => {
  const q = props.searchQuery.trim().toLowerCase()

  return props.skills.filter((skill) => {
    // 1. 分类过滤
    if (props.selectedCategory !== 'all') {
      if (skill.category !== props.selectedCategory)
        return false
    }

    // 2. 仅看已安装过滤
    if (props.onlyInstalled) {
      const list = props.installedMap[skill.id] || []
      const isInstalled = list.includes(props.currentToolId)
      if (!isInstalled)
        return false
    }

    // 3. 搜索匹配（APP、名称、描述）
    if (q) {
      const matchApp = skill.app.toLowerCase().includes(q)
      const matchName = skill.name.toLowerCase().includes(q)
      const matchDesc = skill.description.toLowerCase().includes(q)
      if (!matchApp && !matchName && !matchDesc)
        return false
    }

    return true
  })
})

function clearFilters() {
  emit('update:searchQuery', '')
  emit('update:selectedCategory', 'all')
  emit('update:onlyInstalled', false)
}
</script>

<template>
  <section class="space-y-3.5" aria-label="技能市场">
    <!-- 标题与已安装快捷切换 -->
    <div class="flex flex-wrap items-center justify-between gap-3">
      <div>
        <h2 class="text-base font-semibold tracking-tight text-foreground sm:text-lg">
          技能市场
        </h2>
        <p class="text-xs text-muted-foreground">
          为常用 APP 添加自动化技能
        </p>
      </div>

      <!-- 已安装切换按钮 -->
      <Button
        :variant="props.onlyInstalled ? 'default' : 'outline'"
        size="sm"
        class="h-8 text-xs font-medium"
        :aria-pressed="props.onlyInstalled"
        @click="emit('update:onlyInstalled', !props.onlyInstalled)"
      >
        <Check v-if="props.onlyInstalled" class="mr-1.5 h-3.5 w-3.5" />
        <ListFilter v-else class="mr-1.5 h-3.5 w-3.5" />
        仅看当前助手已安装
        <Badge
          v-if="installedCountOnCurrentTool > 0"
          :variant="props.onlyInstalled ? 'secondary' : 'default'"
          class="ml-1.5 h-4 px-1 text-[10px]"
        >
          {{ installedCountOnCurrentTool }}
        </Badge>
      </Button>
    </div>

    <!-- 筛选搜索栏 -->
    <div class="flex flex-col gap-2.5 sm:flex-row sm:items-center sm:justify-between">
      <!-- 搜索框 -->
      <div class="relative w-full sm:w-72">
        <Search class="absolute left-2.5 top-1/2 h-3.5 w-3.5 -translate-y-1/2 text-muted-foreground" />
        <Input
          :model-value="props.searchQuery"
          type="text"
          placeholder="搜索技能或 APP..."
          aria-label="搜索技能或 APP"
          class="h-8 pl-8 pr-7 text-xs"
          @update:model-value="emit('update:searchQuery', String($event))"
        />
        <Button
          v-if="props.searchQuery"
          variant="ghost"
          size="icon"
          class="absolute right-1 top-1/2 h-6 w-6 -translate-y-1/2 text-muted-foreground hover:text-foreground"
          aria-label="清除搜索关键字"
          @click="emit('update:searchQuery', '')"
        >
          <X class="h-3 w-3" />
        </Button>
      </div>

      <!-- 分类标签组 -->
      <div class="flex flex-wrap items-center gap-1.5" role="tablist" aria-label="技能分类">
        <Button
          v-for="cat in categories"
          :key="cat.id"
          role="tab"
          :variant="props.selectedCategory === cat.id ? 'default' : 'ghost'"
          size="sm"
          :aria-selected="props.selectedCategory === cat.id"
          class="h-7 px-2.5 text-xs font-normal"
          @click="emit('update:selectedCategory', cat.id)"
        >
          {{ cat.label }}
        </Button>
      </div>
    </div>

    <!-- 技能卡片网格 -->
    <div
      v-if="filteredSkills.length > 0"
      class="grid grid-cols-1 gap-3 sm:grid-cols-2 lg:grid-cols-3"
    >
      <SkillCard
        v-for="skill in filteredSkills"
        :key="skill.id"
        :skill="skill"
        :current-tool-id="props.currentToolId"
        :installed-tools="props.installedMap[skill.id] || []"
        @view-detail="emit('view-detail', $event)"
        @install="emit('install', $event)"
      />
    </div>

    <!-- 空状态 -->
    <div v-else class="py-12 text-center">
      <Empty>
        <EmptyMedia>
          <Search class="h-8 w-8 text-muted-foreground" />
        </EmptyMedia>
        <EmptyHeader>
          <EmptyTitle>未找到相关技能</EmptyTitle>
          <EmptyDescription>没有匹配当前筛选条件的技能包，请尝试重置搜索或分类。</EmptyDescription>
        </EmptyHeader>
        <EmptyContent>
          <Button variant="outline" size="sm" class="gap-1.5 text-xs" @click="clearFilters">
            <RotateCcw class="h-3.5 w-3.5" />
            重置筛选条件
          </Button>
        </EmptyContent>
      </Empty>
    </div>
  </section>
</template>
