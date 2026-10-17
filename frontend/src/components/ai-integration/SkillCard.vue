<script setup lang="ts">
import type { AiToolId, SkillItem } from '@/api/aiIntegrations'
import { computed, ref } from 'vue'
import { Check, Download, ExternalLink, Loader2, Smartphone } from 'lucide-vue-next'
import { Badge } from '@/components/ui/badge'
import { Button } from '@/components/ui/button'
import { Card, CardContent, CardFooter, CardHeader } from '@/components/ui/card'
import { downloadSkillZip } from '@/api/aiIntegrations'

const props = defineProps<{
  skill: SkillItem
  currentToolId: AiToolId
  installedTools: AiToolId[]
}>()

const emit = defineEmits<{
  (e: 'view-detail', skill: SkillItem): void
  (e: 'install', skill: SkillItem): void
}>()

const isInstalled = computed(() => {
  return props.installedTools.includes(props.currentToolId)
})

const isDownloading = ref(false)
const downloadError = ref<string | null>(null)

async function handleDownload() {
  if (isDownloading.value)
    return
  isDownloading.value = true
  downloadError.value = null
  try {
    await downloadSkillZip(props.skill.id, props.skill.zipFileName)
  }
  catch (err: unknown) {
    downloadError.value = err instanceof Error ? err.message : '下载失败'
  }
  finally {
    isDownloading.value = false
  }
}
</script>

<template>
  <Card class="gap-0 py-0 flex h-full flex-col justify-between overflow-hidden border border-border shadow-sm transition-all duration-150 hover:shadow-md">
    <CardHeader class="p-4 pb-2">
      <!-- 头部应用信息与分类 -->
      <div class="flex items-center justify-between gap-3">
        <div class="flex items-center gap-2.5">
          <!-- 快手图标 -->
          <div
            v-if="props.skill.iconType === 'kuaishou'"
            class="flex h-9 w-9 shrink-0 items-center justify-center rounded-lg bg-gradient-to-br from-amber-500 to-orange-500 text-white shadow-sm"
          >
            <svg class="h-4 w-4" viewBox="0 0 24 24" fill="currentColor">
              <rect x="2" y="5" width="14" height="14" rx="3" />
              <path d="M16 10l5-3v10l-5-3v-4z" />
            </svg>
          </div>

          <!-- 微信图标 -->
          <div
            v-else-if="props.skill.iconType === 'wechat'"
            class="flex h-9 w-9 shrink-0 items-center justify-center rounded-lg bg-emerald-500 text-white shadow-sm"
          >
            <svg class="h-4 w-4" viewBox="0 0 24 24" fill="currentColor">
              <path d="M9 4C5.13 4 2 6.69 2 10c0 1.88 1.02 3.55 2.62 4.67-.11.66-.46 2.05-.53 2.37-.1.44.17.43.36.3.26-.17 2.39-1.63 2.8-1.92.56.13 1.15.2 1.75.2 3.87 0 7-2.69 7-6s-3.13-6-7-6zm7 8c-.34 0-.67.03-.99.08.62.86.99 1.87.99 2.92 0 1.57-.84 2.99-2.19 3.96.09.55.39 1.7.45 1.97.08.37-.14.36-.3.25-.22-.14-2-1.36-2.34-1.6-.52.12-1.07.19-1.62.19-1.89 0-3.6-.66-4.83-1.74C7.03 19.35 9.38 20.5 12 20.5c.53 0 1.05-.05 1.55-.16.36.25 2.22 1.5 2.45 1.65.17.11.4.12.31-.26-.06-.27-.37-1.45-.46-2.02 1.4-1 2.29-2.45 2.29-4.08 0-2.88-2.73-5.23-6.14-5.63z" />
            </svg>
          </div>

          <!-- 通用设备图标 -->
          <div
            v-else
            class="flex h-9 w-9 shrink-0 items-center justify-center rounded-lg bg-blue-600 text-white shadow-sm"
          >
            <Smartphone class="h-4 w-4" />
          </div>

          <span class="font-medium text-foreground text-sm">{{ props.skill.app }}</span>
        </div>

        <Badge variant="secondary" class="text-xs font-normal">
          {{ props.skill.categoryLabel }}
        </Badge>
      </div>

      <!-- 技能名称与简介 -->
      <div class="mt-2.5 space-y-1">
        <h4 class="text-base font-semibold text-foreground tracking-tight">
          {{ props.skill.name }}
        </h4>
        <p class="text-xs text-muted-foreground leading-relaxed line-clamp-2">
          {{ props.skill.description }}
        </p>
      </div>
    </CardHeader>

    <CardContent class="p-4 pt-0" />

    <CardFooter class="grid grid-cols-3 gap-1.5 border-t bg-muted/20 p-2.5 sm:p-3">
      <Button
        variant="outline"
        size="sm"
        class="h-8 px-1.5 text-xs font-normal"
        @click="emit('view-detail', props.skill)"
      >
        <ExternalLink class="mr-1 h-3.5 w-3.5" />
        详情
      </Button>

      <Button
        variant="outline"
        size="sm"
        class="h-8 px-1.5 text-xs font-normal"
        :disabled="isDownloading"
        @click="handleDownload"
      >
        <Loader2 v-if="isDownloading" class="mr-1 h-3.5 w-3.5 animate-spin" />
        <Download v-else class="mr-1 h-3.5 w-3.5" />
        直接下载
      </Button>

      <Button
        v-if="isInstalled"
        variant="secondary"
        size="sm"
        class="h-8 px-1.5 text-xs font-normal text-muted-foreground cursor-default"
        disabled
      >
        <Check class="mr-1 h-3.5 w-3.5 text-green-600 dark:text-green-400" />
        已安装
      </Button>

      <Button
        v-else
        size="sm"
        class="h-8 px-1.5 text-xs font-normal"
        @click="emit('install', props.skill)"
      >
        <Download class="mr-1 h-3.5 w-3.5" />
        安装
      </Button>
    </CardFooter>
  </Card>
</template>
