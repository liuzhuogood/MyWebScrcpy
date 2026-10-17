<script setup lang="ts">
import type { AiTool } from '@/api/aiIntegrations'
import { onUnmounted, ref, watch } from 'vue'
import { Check, Copy, Download, ExternalLink, Folder, Loader2, Sparkles } from 'lucide-vue-next'
import { Badge } from '@/components/ui/badge'
import { Button } from '@/components/ui/button'
import { Card, CardContent } from '@/components/ui/card'
import { downloadSkillZip } from '@/api/aiIntegrations'

const props = defineProps<{
  tool: AiTool
}>()

const emit = defineEmits<{
  (e: 'install-base', tool: AiTool): void
}>()

const isDownloading = ref(false)
const downloadError = ref<string | null>(null)

async function handleDownloadBase() {
  if (isDownloading.value)
    return
  isDownloading.value = true
  downloadError.value = null
  try {
    await downloadSkillZip('mywebscrcpy-device-base', 'mywebscrcpy-device-base.zip')
  }
  catch (err: unknown) {
    downloadError.value = err instanceof Error ? err.message : '下载失败'
  }
  finally {
    isDownloading.value = false
  }
}

const copySuccess = ref(false)
const copyError = ref('')
const copySeq = ref(0)
let timer: ReturnType<typeof setTimeout> | null = null

function resetCopyState() {
  copySeq.value++
  copySuccess.value = false
  copyError.value = ''
  if (timer) {
    clearTimeout(timer)
    timer = null
  }
}

watch(
  () => props.tool.id,
  () => {
    resetCopyState()
  },
)

async function handleCopy(text: string) {
  copySeq.value++
  const currentSeq = copySeq.value
  if (timer) {
    clearTimeout(timer)
    timer = null
  }
  copySuccess.value = false
  copyError.value = ''

  if (!navigator?.clipboard?.writeText) {
    copyError.value = '剪贴板不可用，请手动选择文本复制。'
    return
  }

  try {
    await navigator.clipboard.writeText(text)
    if (copySeq.value !== currentSeq)
      return
    copySuccess.value = true
    timer = setTimeout(() => {
      if (copySeq.value === currentSeq) {
        copySuccess.value = false
      }
    }, 2000)
  }
  catch {
    if (copySeq.value !== currentSeq)
      return
    copyError.value = '复制失败，请手动选择文本复制。'
  }
}

onUnmounted(() => {
  resetCopyState()
})
</script>

<template>
  <Card class="gap-0 py-0 overflow-hidden border border-border shadow-sm">
    <CardContent class="p-4 sm:p-5">
      <!-- 头部：标题、标签与官方链接 -->
      <div class="flex flex-wrap items-center justify-between gap-2">
        <div class="flex items-center gap-2">
          <h3 class="text-base font-semibold tracking-tight text-foreground">
            {{ props.tool.guideTitle }}
          </h3>
          <Badge variant="secondary" class="font-normal text-xs text-muted-foreground">
            {{ props.tool.guideBadge }}
          </Badge>
        </div>

        <a
          v-if="props.tool.officialDocUrl"
          :href="props.tool.officialDocUrl"
          target="_blank"
          rel="noopener noreferrer"
          class="inline-flex items-center gap-1 text-xs text-muted-foreground hover:text-primary transition-colors"
        >
          <span>官方文档</span>
          <ExternalLink class="h-3 w-3" />
        </a>
      </div>

      <!-- 步骤流程：紧凑分栏，小屏纵向堆叠 -->
      <div class="mt-4 grid grid-cols-1 divide-y divide-border/60 rounded-lg md:grid-cols-3 md:divide-x md:divide-y-0">
        <!-- 步骤 1 -->
        <div class="flex flex-col justify-between py-2.5 px-3 first:pl-0 sm:py-3 sm:px-4">
          <div class="flex items-start gap-3">
            <span class="flex h-6 w-6 shrink-0 items-center justify-center rounded-full bg-blue-100 text-xs font-semibold text-blue-700 dark:bg-blue-950 dark:text-blue-300">
              1
            </span>
            <div class="space-y-0.5">
              <h4 class="text-sm font-medium text-foreground">
                {{ props.tool.step1Title }}
              </h4>
              <p class="text-xs text-muted-foreground leading-relaxed">
                {{ props.tool.step1Desc }}
              </p>
            </div>
          </div>
        </div>

        <!-- 步骤 2 -->
        <div class="flex flex-col justify-between py-2.5 px-3 sm:py-3 sm:px-4">
          <div class="flex items-start gap-3">
            <span class="flex h-6 w-6 shrink-0 items-center justify-center rounded-full bg-blue-100 text-xs font-semibold text-blue-700 dark:bg-blue-950 dark:text-blue-300">
              2
            </span>
            <div class="space-y-0.5">
              <h4 class="text-sm font-medium text-foreground">
                {{ props.tool.step2Title }}
              </h4>
              <p class="text-xs text-muted-foreground leading-relaxed">
                {{ props.tool.step2Desc }}
              </p>
            </div>
          </div>
          <div class="mt-3 flex flex-wrap items-center gap-2 pl-9">
            <Button
              size="sm"
              class="h-7 px-2.5 text-xs font-medium"
              @click="emit('install-base', props.tool)"
            >
              <Download class="mr-1.5 h-3.5 w-3.5" />
              安装基础技能
            </Button>
            <Button
              variant="outline"
              size="sm"
              class="h-7 px-2.5 text-xs font-medium"
              :disabled="isDownloading"
              @click="handleDownloadBase"
            >
              <Loader2 v-if="isDownloading" class="mr-1.5 h-3.5 w-3.5 animate-spin" />
              <Download v-else class="mr-1.5 h-3.5 w-3.5" />
              直接下载
            </Button>
            <span v-if="downloadError" class="text-[11px] text-destructive block">{{ downloadError }}</span>
          </div>
        </div>

        <!-- 步骤 3 -->
        <div class="flex flex-col justify-between py-2.5 px-3 last:pr-0 sm:py-3 sm:px-4">
          <div class="flex items-start gap-3">
            <span class="flex h-6 w-6 shrink-0 items-center justify-center rounded-full bg-blue-100 text-xs font-semibold text-blue-700 dark:bg-blue-950 dark:text-blue-300">
              3
            </span>
            <div class="space-y-0.5">
              <h4 class="text-sm font-medium text-foreground">
                {{ props.tool.step3Title }}
              </h4>
              <p class="text-xs text-muted-foreground leading-relaxed">
                {{ props.tool.step3Desc }}
              </p>
            </div>
          </div>
        </div>
      </div>

      <!-- 示例任务条 -->
      <div class="mt-3 flex flex-wrap items-center justify-between gap-3 rounded-lg bg-muted/60 px-3.5 py-2.5 text-xs sm:flex-nowrap">
        <div class="flex min-w-0 items-start gap-2.5">
          <Sparkles class="mt-0.5 h-4 w-4 shrink-0 text-primary" aria-hidden="true" />
          <span class="break-words text-foreground select-all leading-relaxed">{{ props.tool.samplePrompt }}</span>
        </div>
        <div class="flex shrink-0 items-center gap-2 self-center">
          <span v-if="copySuccess" class="text-xs text-green-600 dark:text-green-400 font-medium">已复制</span>
          <Button
            variant="ghost"
            size="icon"
            class="h-7 w-7 text-muted-foreground hover:text-foreground"
            aria-label="复制示例任务"
            @click="handleCopy(props.tool.samplePrompt)"
          >
            <Check v-if="copySuccess" class="h-3.5 w-3.5 text-green-600 dark:text-green-400" />
            <Copy v-else class="h-3.5 w-3.5" />
          </Button>
        </div>
      </div>

      <!-- 宿主机真实安装目录提示 -->
      <div class="mt-2.5 flex flex-wrap items-center gap-1.5 text-[11px] text-muted-foreground">
        <Folder class="h-3.5 w-3.5 shrink-0" />
        <span class="shrink-0">服务宿主机技能目录：</span>
        <code class="break-all rounded bg-muted/80 px-1 py-0.5 font-mono text-[11px] text-foreground">
          {{ props.tool.targetPath }}
        </code>
      </div>

      <!-- 复制异常提示 -->
      <p v-if="copyError" role="alert" class="mt-2 text-xs text-destructive">
        {{ copyError }}
      </p>
    </CardContent>
  </Card>
</template>
