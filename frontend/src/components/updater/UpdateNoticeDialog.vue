<script setup lang="ts">
import { computed, watch } from 'vue'
import {
  Dialog,
  DialogContent,
  DialogDescription,
  DialogFooter,
  DialogHeader,
  DialogTitle,
} from '@/components/ui/dialog'
import { Button } from '@/components/ui/button'
import { Progress } from '@/components/ui/progress'
import { Badge } from '@/components/ui/badge'
import { useUpdaterStore } from '@/stores/updater'
import { Download, RefreshCw, CheckCircle2, AlertCircle, Sparkles, Info } from 'lucide-vue-next'

const updater = useUpdaterStore()

const hasUpdate = computed(() => !!updater.checkResult?.has_update)

const stateText = computed(() => {
  switch (updater.progress.state) {
    case 'downloading':
      return '正在下载新版本...'
    case 'verifying':
      return '正在验证安装包...'
    case 'applying':
      return '正在替换可执行文件...'
    case 'success':
      return '更新完成，服务正在重启...'
    case 'error':
      return '更新失败'
    default:
      return ''
  }
})

function formatSize(bytes: number) {
  if (!bytes || bytes <= 0) return '0 B'
  const units = ['B', 'KB', 'MB', 'GB']
  let b = bytes
  let i = 0
  while (b >= 1024 && i < units.length - 1) {
    b /= 1024
    i++
  }
  return `${b.toFixed(2)} ${units[i]}`
}

function handleRefresh() {
  window.location.reload()
}

// 成功后自动尝试探测后端并刷新
watch(
  () => updater.progress.state,
  (newState) => {
    if (newState === 'success') {
      let attempts = 0
      const timer = setInterval(async () => {
        attempts++
        try {
          const res = await fetch('/api/version/check')
          if (res.ok) {
            clearInterval(timer)
            setTimeout(() => {
              window.location.reload()
            }, 800)
          }
        } catch {
          // 服务还在重启中
        }
        if (attempts > 30) {
          clearInterval(timer)
        }
      }, 1000)
    }
  }
)
</script>

<template>
  <Dialog :open="updater.dialogOpen" @update:open="(val) => (updater.dialogOpen = val)">
    <DialogContent class="sm:max-w-[620px]">
      <DialogHeader>
        <div class="flex items-center gap-2">
          <Sparkles v-if="hasUpdate" class="h-5 w-5 text-primary" />
          <Info v-else class="h-5 w-5 text-muted-foreground" />
          <DialogTitle class="text-lg">{{ hasUpdate ? '软件更新' : '版本信息' }}</DialogTitle>
        </div>
        <DialogDescription class="text-sm">
          {{ hasUpdate ? '发现新版本，建议及时更新以获取最新功能和修复。' : '当前已是最新版本，无需更新。' }}
        </DialogDescription>
      </DialogHeader>

      <div class="space-y-4 py-2">
        <!-- 版本对比 -->
        <div class="flex items-center justify-between rounded-lg border bg-muted/40 p-3.5">
          <div class="space-y-1">
            <span class="text-xs text-muted-foreground">当前版本</span>
            <div class="font-mono text-base font-medium">
              {{ updater.checkResult?.current_version || '未知' }}
            </div>
          </div>
          <div v-if="hasUpdate" class="text-muted-foreground font-mono">→</div>
          <div v-if="hasUpdate" class="space-y-1 text-right">
            <span class="text-xs text-muted-foreground">最新版本</span>
            <div class="flex items-center gap-1.5 font-mono text-base font-semibold text-primary">
              {{ updater.checkResult?.latest_version || '未知' }}
              <Badge variant="default" class="h-5 text-[10px] px-1.5">New</Badge>
            </div>
          </div>
          <div v-else class="text-right">
            <Badge variant="secondary" class="gap-1 border-green-500/30 bg-green-500/10 text-green-600 dark:text-green-400">
              <CheckCircle2 class="h-3.5 w-3.5" />
              已是最新
            </Badge>
          </div>
        </div>

        <!-- Release 描述 / 日志 -->
        <div v-if="updater.checkResult?.release_notes && updater.progress.state === 'idle'" class="space-y-1.5">
          <span class="text-xs font-medium text-muted-foreground">{{ hasUpdate ? '更新日志' : '版本说明' }}</span>
          <div class="max-h-60 overflow-y-auto rounded-md border bg-background p-3.5 text-xs leading-relaxed text-foreground/90 whitespace-pre-wrap font-sans">
            {{ updater.checkResult.release_notes }}
          </div>
        </div>

        <!-- 进度条展示 -->
        <div v-if="updater.progress.state !== 'idle'" class="space-y-2 pt-1">
          <div class="flex items-center justify-between text-xs">
            <span class="font-medium flex items-center gap-1.5">
              <RefreshCw v-if="['downloading', 'verifying', 'applying'].includes(updater.progress.state)" class="h-3.5 w-3.5 animate-spin text-primary" />
              <CheckCircle2 v-else-if="updater.progress.state === 'success'" class="h-3.5 w-3.5 text-green-500" />
              <AlertCircle v-else-if="updater.progress.state === 'error'" class="h-3.5 w-3.5 text-destructive" />
              {{ stateText }}
            </span>
            <span class="font-mono text-muted-foreground">
              {{ updater.progress.percent.toFixed(1) }}%
            </span>
          </div>

          <Progress :model-value="updater.progress.percent" class="h-2" />

          <div class="flex items-center justify-between text-[11px] text-muted-foreground font-mono">
            <span>{{ formatSize(updater.progress.downloaded) }} / {{ formatSize(updater.progress.total) }}</span>
            <span v-if="updater.progress.state === 'downloading'">{{ updater.progress.speed }}</span>
          </div>
        </div>

        <!-- 错误提示 -->
        <div v-if="updater.errorMessage" class="rounded-md border border-destructive/30 bg-destructive/10 p-2.5 text-xs text-destructive flex items-center gap-2">
          <AlertCircle class="h-4 w-4 shrink-0" />
          <span>{{ updater.errorMessage }}</span>
        </div>
      </div>

      <DialogFooter class="gap-2 sm:gap-2">
        <template v-if="updater.progress.state === 'idle' || updater.progress.state === 'error'">
          <template v-if="hasUpdate">
            <Button variant="ghost" size="sm" @click="updater.closeDialog">稍后提醒</Button>
            <Button size="sm" class="gap-1.5" @click="updater.upgrade">
              <Download class="h-4 w-4" />
              立即更新
            </Button>
          </template>
          <template v-else>
            <Button variant="outline" size="sm" class="gap-1.5" :disabled="updater.isChecking" @click="updater.check">
              <RefreshCw class="h-3.5 w-3.5" :class="{ 'animate-spin': updater.isChecking }" />
              检查更新
            </Button>
            <Button size="sm" @click="updater.closeDialog">确定</Button>
          </template>
        </template>
        <template v-else-if="updater.progress.state === 'success'">
          <Button size="sm" class="w-full gap-1.5" @click="handleRefresh">
            <RefreshCw class="h-4 w-4" />
            刷新网页
          </Button>
        </template>
        <template v-else>
          <Button variant="outline" size="sm" class="w-full" disabled>
            更新进行中，请勿关闭服务...
          </Button>
        </template>
      </DialogFooter>
    </DialogContent>
  </Dialog>
</template>
