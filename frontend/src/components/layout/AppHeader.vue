<script setup lang="ts">
import { onMounted, ref } from 'vue'
import { Bot, ChevronDown, Ellipsis, Github, LayoutGrid, Settings, Sparkles } from 'lucide-vue-next'
import { RouterLink } from 'vue-router'
import { Button } from '@/components/ui/button'
import { Badge } from '@/components/ui/badge'
import { DropdownMenu, DropdownMenuContent, DropdownMenuItem, DropdownMenuSeparator, DropdownMenuTrigger } from '@/components/ui/dropdown-menu'
import { useUpdaterStore } from '@/stores/updater'
import UpdateNoticeDialog from '@/components/updater/UpdateNoticeDialog.vue'
import SystemConfigDialog from '@/components/system/SystemConfigDialog.vue'
import MainWindowEffectsControl from '@/components/layout/MainWindowEffectsControl.vue'
const configOpen = ref(false)

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
    <div class="flex h-14 w-full items-center gap-2 px-4 sm:gap-3">
      <div class="flex min-w-0 items-center gap-2">
        <RouterLink to="/" class="flex min-w-0 items-center gap-2">
          <img :src="appIconUrl" alt="" class="h-8 w-8 shrink-0 object-contain" width="32" height="32" />
          <span class="truncate text-base font-semibold">MyWebScrcpy</span>
        </RouterLink>

        <!-- 版本号常显（点击可打开版本信息弹窗） -->
        <button
          v-if="updater.checkResult?.current_version"
          type="button"
          class="inline-flex shrink-0 items-center gap-1.5 rounded px-1.5 py-0.5 font-mono text-xs text-muted-foreground hover:bg-muted/60 hover:text-foreground transition-colors cursor-pointer"
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

      <div class="ml-auto hidden shrink-0 items-center gap-2 xl:flex">
        <!-- 发现新版本时的突出提醒按钮 -->
        <Button
          v-if="updater.checkResult?.has_update"
          variant="outline"
          size="sm"
          class="max-w-full border-primary/40 bg-primary/10 text-primary hover:bg-primary/20 gap-1.5"
          @click="updater.openDialog"
        >
          <Sparkles class="h-3.5 w-3.5 animate-pulse text-primary" />
          <span class="min-w-0 truncate">发现新版本 {{ updater.checkResult.latest_version }}</span>
          <Badge variant="default" class="h-4 px-1 text-[10px]">Update</Badge>
        </Button>

        <Button variant="outline" size="sm" as-child>
          <RouterLink to="/ai-integration">
            <Bot class="h-4 w-4" />
            AI 集成
          </RouterLink>
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
        <DropdownMenu>
          <DropdownMenuTrigger as-child>
            <Button variant="ghost" size="sm" class="gap-1">
              <span>API 文档</span>
              <ChevronDown class="h-3.5 w-3.5 opacity-60" />
            </Button>
          </DropdownMenuTrigger>
          <DropdownMenuContent align="end">
            <DropdownMenuItem as-child>
              <a href="/api-docs-automation.html" target="_blank" rel="noopener">
                🤖 自动化接口文档
              </a>
            </DropdownMenuItem>
            <DropdownMenuItem as-child>
              <a href="/api-docs.html" target="_blank" rel="noopener">
                📚 全量接口文档
              </a>
            </DropdownMenuItem>
          </DropdownMenuContent>
        </DropdownMenu>
      </div>
      <div class="ml-auto flex items-center gap-1.5 shrink-0 xl:ml-0">
        <MainWindowEffectsControl />
        <Button variant="ghost" size="sm" aria-label="系统设置" @click="configOpen = true"><Settings class="size-4" /><span class="hidden sm:inline">系统设置</span></Button>
      </div>
      <DropdownMenu>
        <DropdownMenuTrigger as-child>
          <Button class="xl:hidden" variant="ghost" size="icon" title="更多" aria-label="更多">
            <Ellipsis class="h-5 w-5" />
          </Button>
        </DropdownMenuTrigger>
        <DropdownMenuContent align="end">
          <DropdownMenuItem v-if="updater.checkResult?.has_update" @select="updater.openDialog">
            <Sparkles class="h-4 w-4" />发现新版本 {{ updater.checkResult.latest_version }}
          </DropdownMenuItem>
          <DropdownMenuSeparator v-if="updater.checkResult?.has_update" />
          <DropdownMenuItem as-child>
            <RouterLink to="/ai-integration"><Bot class="h-4 w-4" />AI 集成</RouterLink>
          </DropdownMenuItem>
          <DropdownMenuItem as-child>
            <RouterLink to="/dashboard"><LayoutGrid class="h-4 w-4" />大屏模式</RouterLink>
          </DropdownMenuItem>
          <DropdownMenuItem as-child>
            <a href="https://github.com/liuzhuogood/MyWebScrcpy" target="_blank" rel="noopener noreferrer"><Github class="h-4 w-4" />GitHub</a>
          </DropdownMenuItem>
          <DropdownMenuItem as-child>
            <a href="/api-docs-automation.html" target="_blank" rel="noopener">🤖 自动化接口文档</a>
          </DropdownMenuItem>
          <DropdownMenuItem as-child>
            <a href="/api-docs.html" target="_blank" rel="noopener">📚 全量接口文档</a>
          </DropdownMenuItem>
        </DropdownMenuContent>
      </DropdownMenu>
    </div>
    <UpdateNoticeDialog />
    <SystemConfigDialog v-model:open="configOpen" />
  </header>
</template>
