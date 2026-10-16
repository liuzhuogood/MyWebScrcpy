<script setup lang="ts">
import type { AiTool, AiToolId } from '../types'
import { Check } from 'lucide-vue-next'
import { Button } from '@/components/ui/button'

const props = defineProps<{
  tools: AiTool[]
  selectedId: AiToolId
}>()

const emit = defineEmits<{
  (e: 'select', id: AiToolId): void
}>()
</script>

<template>
  <div class="space-y-3">
    <h2 class="text-base font-semibold tracking-tight text-foreground sm:text-lg">
      连接你的 AI 助手
    </h2>

    <div class="grid grid-cols-1 gap-3 sm:grid-cols-2 md:grid-cols-3">
      <Button
        v-for="tool in props.tools"
        :key="tool.id"
        variant="outline"
        :aria-pressed="props.selectedId === tool.id"
        :aria-label="`${tool.name}，${tool.subtitle}`"
        class="group relative flex h-auto w-full items-center justify-between rounded-xl border p-4 text-left font-normal transition-all duration-150"
        :class="[
          props.selectedId === tool.id
            ? 'border-primary bg-blue-50/70 shadow-sm hover:bg-blue-50/80 dark:bg-blue-950/25 dark:border-primary dark:hover:bg-blue-950/35'
            : 'border-border bg-card hover:border-border/80 hover:bg-muted/40',
        ]"
        @click="emit('select', tool.id)"
      >
        <div class="flex items-center gap-3.5">
          <!-- 工具图标 -->
          <div
            v-if="tool.iconType === 'workbuddy'"
            class="flex h-12 w-12 shrink-0 items-center justify-center rounded-xl bg-blue-600 text-white shadow-sm"
          >
            <span class="text-xl font-bold tracking-tight">W</span>
          </div>

          <div
            v-else-if="tool.iconType === 'codex'"
            class="flex h-12 w-12 shrink-0 items-center justify-center rounded-xl bg-neutral-900 text-white shadow-sm dark:bg-neutral-800"
          >
            <svg class="h-7 w-7" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="1.8">
              <path d="M12 2a10 10 0 1 0 10 10A10 10 0 0 0 12 2zm0 18a8 8 0 1 1 8-8 8 8 0 0 1-8 8z" stroke-dasharray="2 2" />
              <path d="M8 9l3 3-3 3M13 15h3" stroke-linecap="round" stroke-linejoin="round" />
            </svg>
          </div>

          <div
            v-else-if="tool.iconType === 'claude-code'"
            class="flex h-12 w-12 shrink-0 items-center justify-center rounded-xl bg-amber-600 text-white shadow-sm"
          >
            <svg class="h-6 w-6" viewBox="0 0 24 24" fill="currentColor">
              <path d="M12 2L13.5 8.5L20 7L15 12L20 17L13.5 15.5L12 22L10.5 15.5L4 17L9 12L4 7L10.5 8.5L12 2Z" />
            </svg>
          </div>

          <!-- 文字信息 -->
          <div class="min-w-0">
            <div class="font-semibold text-foreground text-sm">
              {{ tool.name }}
            </div>
            <div class="mt-0.5 text-xs text-muted-foreground">
              {{ tool.subtitle }}
            </div>
          </div>
        </div>

        <!-- 选中视觉 -->
        <div class="ml-2 shrink-0">
          <div
            v-if="props.selectedId === tool.id"
            class="flex h-5 w-5 items-center justify-center rounded-full bg-primary text-primary-foreground shadow-sm"
          >
            <Check class="h-3.5 w-3.5 stroke-[2.5]" />
          </div>
          <div
            v-else
            class="h-5 w-5 rounded-full border border-muted-foreground/30 group-hover:border-muted-foreground/50"
          />
        </div>
      </Button>
    </div>
  </div>
</template>
