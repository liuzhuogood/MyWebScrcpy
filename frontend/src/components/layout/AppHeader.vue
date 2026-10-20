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
import { useMainWindowEffects } from '@/composables/useMainWindowEffects'

const configOpen = ref(false)
const effects = useMainWindowEffects()
const isHeaderHovered = ref(false)

defineProps<{ deviceCountText: string }>()

const updater = useUpdaterStore()
const appIconUrl = `${import.meta.env.BASE_URL}app-icon.png`

onMounted(() => {
  // 进入页面自动检查一次更新
  updater.check()
})
</script>

<template>
  <!-- 无边框模式下的顶部感应区：鼠标悬停自动呼出导航栏 -->
  <div
    v-if="effects.state.value.borderless"
    class="fixed top-0 inset-x-0 h-3 z-50 pointer-events-auto"
    @mouseenter="isHeaderHovered = true"
  />

  <header
    class="z-40 border-b border-slate-200/80 bg-white/80 dark:border-white/10 dark:bg-slate-950/60 backdrop-blur-2xl shadow-xs transition-all duration-200"
    :class="effects.state.value.borderless
      ? [
          'fixed top-0 inset-x-0 shadow-md',
          isHeaderHovered ? 'translate-y-0 opacity-100 pointer-events-auto' : '-translate-y-full opacity-0 pointer-events-none',
        ]
      : 'sticky top-0 z-10'"
    @mouseleave="isHeaderHovered = false"
  >
    <div class="flex h-14 w-full items-center gap-2 px-4 sm:gap-3">
      <div class="flex min-w-0 items-center gap-2.5">
        <RouterLink to="/" class="flex min-w-0 items-center gap-2 group">
          <img :src="appIconUrl" alt="" class="h-8 w-8 shrink-0 object-contain transition-transform group-hover:scale-105" width="32" height="32" />
          <span class="truncate text-base font-bold tracking-tight text-slate-900 dark:text-white">MyWebScrcpy</span>
        </RouterLink>

        <!-- 版本号常显（点击可打开版本信息弹窗） -->
        <button
          v-if="updater.checkResult?.current_version"
          type="button"
          class="inline-flex shrink-0 items-center gap-1.5 rounded-lg border border-slate-200/80 bg-slate-100/80 px-2 py-0.5 font-mono text-xs text-slate-700 hover:bg-slate-200 hover:text-slate-900 dark:border-white/10 dark:bg-white/10 dark:text-slate-200 dark:hover:bg-white/20 dark:hover:text-white transition-colors cursor-pointer"
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
          class="max-w-full border-primary/40 bg-primary/10 text-primary hover:bg-primary/20 gap-1.5 rounded-xl"
          @click="updater.openDialog"
        >
          <Sparkles class="h-3.5 w-3.5 animate-pulse text-primary" />
          <span class="min-w-0 truncate">发现新版本 {{ updater.checkResult.latest_version }}</span>
          <Badge variant="default" class="h-4 px-1 text-[10px]">Update</Badge>
        </Button>

        <Button
          variant="ghost"
          size="sm"
          class="h-8 rounded-xl border border-slate-200/80 bg-slate-100/70 text-slate-800 hover:bg-slate-200/80 hover:text-slate-950 dark:border-white/15 dark:bg-white/10 dark:text-slate-100 dark:hover:bg-white/20 dark:hover:text-white text-xs font-medium px-3 gap-1.5 shadow-2xs backdrop-blur-md transition-all cursor-pointer"
          as-child
        >
          <RouterLink to="/ai-integration">
            <Bot class="h-4 w-4" />
            AI 集成
          </RouterLink>
        </Button>

        <Button
          variant="ghost"
          size="sm"
          class="h-8 rounded-xl border border-slate-200/80 bg-slate-100/70 text-slate-800 hover:bg-slate-200/80 hover:text-slate-950 dark:border-white/15 dark:bg-white/10 dark:text-slate-100 dark:hover:bg-white/20 dark:hover:text-white text-xs font-medium px-3 gap-1.5 shadow-2xs backdrop-blur-md transition-all cursor-pointer"
          as-child
        >
          <RouterLink to="/dashboard">
            <LayoutGrid class="h-4 w-4" />
            大屏模式
          </RouterLink>
        </Button>

        <Button
          variant="ghost"
          size="sm"
          class="h-8 rounded-xl text-slate-700 hover:text-slate-950 hover:bg-slate-100 dark:text-slate-300 dark:hover:text-white dark:hover:bg-white/10 text-xs font-medium px-2.5 gap-1.5 transition-colors cursor-pointer"
          as-child
        >
          <a href="https://github.com/liuzhuogood/MyWebScrcpy" target="_blank" rel="noopener noreferrer" aria-label="在 GitHub 查看项目">
            <Github class="h-4 w-4" />
            GitHub
          </a>
        </Button>

        <DropdownMenu>
          <DropdownMenuTrigger as-child>
            <Button
              variant="ghost"
              size="sm"
              class="h-8 rounded-xl text-slate-700 hover:text-slate-950 hover:bg-slate-100 dark:text-slate-300 dark:hover:text-white dark:hover:bg-white/10 text-xs font-medium px-2.5 gap-1 transition-colors cursor-pointer"
            >
              <span>API 文档</span>
              <ChevronDown class="h-3.5 w-3.5 opacity-60" />
            </Button>
          </DropdownMenuTrigger>
          <DropdownMenuContent align="end" class="rounded-2xl border-slate-200 bg-white/95 dark:border-white/10 dark:bg-slate-900/95 backdrop-blur-xl">
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
        <Button
          variant="ghost"
          size="sm"
          class="h-8 rounded-xl border border-slate-200/80 bg-slate-100/70 text-slate-800 hover:bg-slate-200/80 hover:text-slate-950 dark:border-white/15 dark:bg-white/10 dark:text-slate-100 dark:hover:bg-white/20 dark:hover:text-white text-xs font-medium px-3 gap-1.5 shadow-2xs backdrop-blur-md transition-all cursor-pointer"
          aria-label="系统设置"
          @click="configOpen = true"
        >
          <Settings class="size-4" />
          <span class="hidden sm:inline">系统设置</span>
        </Button>
      </div>

      <DropdownMenu>
        <DropdownMenuTrigger as-child>
          <Button class="xl:hidden h-8 w-8 rounded-xl text-slate-700 dark:text-slate-300 hover:bg-slate-100 dark:hover:bg-white/10" variant="ghost" size="icon" title="更多" aria-label="更多">
            <Ellipsis class="h-5 w-5" />
          </Button>
        </DropdownMenuTrigger>
        <DropdownMenuContent align="end" class="rounded-2xl border-slate-200 bg-white/95 dark:border-white/10 dark:bg-slate-900/95 backdrop-blur-xl">
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
