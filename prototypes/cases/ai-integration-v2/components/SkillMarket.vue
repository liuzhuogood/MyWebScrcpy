<script setup lang="ts">
import type { AiToolId, InstalledMap, SkillCategory, SkillItem } from '../types'
import { computed, ref } from 'vue'
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

const props = defineProps<{
  skills: SkillItem[]
  currentToolId: AiToolId
  installedMap: InstalledMap
}>()

const emit = defineEmits<{
  (e: 'view-detail', skill: SkillItem): void
  (e: 'install', skill: SkillItem): void
}>()

const searchQuery = ref('')
const selectedCategory = ref<SkillCategory>('all')
const onlyInstalled = ref(false)

const categories: Array<{ id: SkillCategory; label: string }> = [
  { id: 'all', label: '全部' },
  { id: 'social', label: '社交' },
  { id: 'short-video', label: '短视频' },
  { id: 'productivity', label: '基础能力' },
]

// 统计当前选中的 AI 工具下已安装的技能数量
const installedCountOnCurrentTool = computed(() => {
  return props.skills.filter(s => props.installedMap[s.id].includes(props.currentToolId)).length
})

// 过滤技能列表
const filteredSkills = computed(() => {
  const q = searchQuery.value.trim().toLowerCase()

  return props.skills.filter((skill) => {
    // 1. 分类过滤
    if (selectedCategory.value !== 'all') {
      if (skill.category !== selectedCategory.value)
        return false
    }

    // 2. 仅看已安装过滤
    if (onlyInstalled.value) {
      const isInstalled = props.installedMap[skill.id].includes(props.currentToolId)
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
  searchQuery.value = ''
  selectedCategory.value = 'all'
  onlyInstalled.value = false
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
        :variant="onlyInstalled ? 'default' : 'outline'"
        size="sm"
        class="h-8 text-xs font-medium"
        :aria-pressed="onlyInstalled"
        @click="onlyInstalled = !onlyInstalled"
      >
        <Check v-if="onlyInstalled" class="mr-1.5 h-3.5 w-3.5" />
        <ListFilter v-else class="mr-1.5 h-3.5 w-3.5" />
        <span>已安装</span>
        <Badge
          v-if="installedCountOnCurrentTool > 0"
          :variant="onlyInstalled ? 'secondary' : 'outline'"
          class="ml-1.5 px-1.5 py-0 text-[10px]"
        >
          {{ installedCountOnCurrentTool }}
        </Badge>
      </Button>
    </div>

    <!-- 工具栏：搜索与分类标签 -->
    <div class="flex flex-col gap-2.5 sm:flex-row sm:items-center sm:justify-between">
      <!-- 搜索框 -->
      <div class="relative w-full sm:max-w-xs">
        <Search class="pointer-events-none absolute top-2.5 left-3 h-4 w-4 text-muted-foreground" aria-hidden="true" />
        <Input
          v-model="searchQuery"
          class="h-9 pl-9 pr-8 text-xs"
          placeholder="搜索 APP 或技能"
          aria-label="搜索 APP 或技能"
        />
        <Button
          v-if="searchQuery"
          variant="ghost"
          size="icon"
          class="absolute top-1 right-1 h-7 w-7 text-muted-foreground hover:text-foreground"
          aria-label="清除搜索"
          @click="searchQuery = ''"
        >
          <X class="h-3.5 w-3.5" />
        </Button>
      </div>

      <!-- 分类筛选按钮 -->
      <div class="flex flex-wrap items-center gap-1.5" aria-label="技能分类筛选">
        <Button
          v-for="cat in categories"
          :key="cat.id"
          variant="ghost"
          size="sm"
          :aria-pressed="selectedCategory === cat.id"
          class="h-7 rounded-full px-3 text-xs font-medium"
          :class="[
            selectedCategory === cat.id
              ? 'bg-primary text-primary-foreground hover:bg-primary/90 hover:text-primary-foreground shadow-sm'
              : 'bg-muted/70 text-muted-foreground hover:bg-muted hover:text-foreground',
          ]"
          @click="selectedCategory = cat.id"
        >
          {{ cat.label }}
        </Button>
      </div>
    </div>

    <!-- 技能卡片列表 -->
    <div
      v-if="filteredSkills.length > 0"
      class="grid grid-cols-1 gap-4 sm:grid-cols-2 lg:grid-cols-3"
    >
      <SkillCard
        v-for="skill in filteredSkills"
        :key="skill.id"
        :skill="skill"
        :current-tool-id="props.currentToolId"
        :installed-tools="props.installedMap[skill.id]"
        @view-detail="emit('view-detail', $event)"
        @install="emit('install', $event)"
      />
    </div>

    <!-- 空结果状态 -->
    <div v-else class="rounded-xl border border-dashed border-border py-8">
      <Empty>
        <EmptyMedia variant="icon">
          <Search class="h-5 w-5 text-muted-foreground" />
        </EmptyMedia>
        <EmptyHeader>
          <EmptyTitle class="text-base font-semibold">未找到匹配的技能</EmptyTitle>
          <EmptyDescription class="text-xs text-muted-foreground">
            没有符合当前搜索或筛选条件的技能，可尝试调整关键词或查看全部技能。
          </EmptyDescription>
        </EmptyHeader>
        <EmptyContent>
          <Button
            variant="outline"
            size="sm"
            class="h-8 text-xs"
            @click="clearFilters"
          >
            <RotateCcw class="mr-1.5 h-3.5 w-3.5" />
            重置筛选条件
          </Button>
        </EmptyContent>
      </Empty>
    </div>
  </section>
</template>
