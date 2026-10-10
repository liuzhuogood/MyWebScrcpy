<script setup lang="ts">
import { onMounted } from 'vue'
import { Github, LayoutGrid, Sparkles } from 'lucide-vue-next'
import { RouterLink } from 'vue-router'
import { Button } from '@/components/ui/button'
import { Badge } from '@/components/ui/badge'
import { useUpdaterStore } from '@/stores/updater'
import UpdateNoticeDialog from '@/components/updater/UpdateNoticeDialog.vue'

defineProps<{ deviceCountText: string }>()

const updater = useUpdaterStore()
const appIconUrl = `${import.meta.env.BASE_URL}app-icon.png`

onMounted(() => {
  // 进入页面自动检查一次更新
  updater.check()
})
</script>

<template>
  <header class="sticky top-0 z-10 border-b bg-background/80 backdrop-blur">
    <div class="flex h-14 w-full items-center gap-3 px-4">
      <div class="flex items-center gap-2">
        <RouterLink to="/" class="flex items-center gap-2">
          <img :src="appIconUrl" alt="" class="h-8 w-8 shrink-0 object-contain" width="32" height="32" />
          <span class="text-base font-semibold">MyWebScrcpy</span>
        </RouterLink>

        <!-- 版本号常显（点击可打开版本信息弹窗） -->
        <button
          v-if="updater.checkResult?.current_version"
          type="button"
          class="inline-flex items-center gap-1.5 rounded px-1.5 py-0.5 font-mono text-xs text-muted-foreground hover:bg-muted/60 hover:text-foreground transition-colors cursor-pointer"
          title="点击查看版本信息"
          @click="updater.openDialog"
        >
          <span>{{ updater.checkResult.current_version }}</span>
          <span
            v-if="updater.checkResult?.has_update"
            class="flex h-2 w-2 relative"
            title="有新版本可用"
          >
            <span class="animate-ping absolute inline-flex h-full w-full rounded-full bg-primary opacity-75"></span>
            <span class="relative inline-flex rounded-full h-2 w-2 bg-primary"></span>
          </span>
        </button>
      </div>

      <div class="ml-auto flex items-center gap-2">
        <!-- 发现新版本时的突出提醒按钮 -->
        <Button
          v-if="updater.checkResult?.has_update"
          variant="outline"
          size="sm"
          class="border-primary/40 bg-primary/10 text-primary hover:bg-primary/20 gap-1.5"
          @click="updater.openDialog"
        >
          <Sparkles class="h-3.5 w-3.5 animate-pulse text-primary" />
          <span>发现新版本 {{ updater.checkResult.latest_version }}</span>
          <Badge variant="default" class="h-4 px-1 text-[10px]">Update</Badge>
        </Button>

        <Button variant="outline" size="sm" as-child>
          <RouterLink to="/dashboard">
            <LayoutGrid class="h-4 w-4" />
            大屏模式
          </RouterLink>
        </Button>
        <Button variant="ghost" size="sm" as-child>
          <a href="https://github.com/liuzhuogood/MyWebScrcpy" target="_blank" rel="noopener noreferrer" aria-label="在 GitHub 查看项目">
            <Github class="h-4 w-4" />
            GitHub
          </a>
        </Button>
        <Button variant="ghost" size="sm" as-child>
          <a href="/api-docs.html" target="_blank" rel="noopener">API 文档</a>
        </Button>
      </div>
    </div>
    <UpdateNoticeDialog />
  </header>
</template>
