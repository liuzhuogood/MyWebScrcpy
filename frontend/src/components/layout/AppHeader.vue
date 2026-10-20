<script setup lang="ts">
import { computed, onMounted, ref } from 'vue'
import { ArrowLeft, Bot, ChevronDown, Ellipsis, Github, LayoutGrid, Settings, Sparkles, X } from 'lucide-vue-next'
import { RouterLink } from 'vue-router'
import { invoke, isTauri } from '@tauri-apps/api/core'
import { getCurrentWindow } from '@tauri-apps/api/window'
import { Button } from '@/components/ui/button'
import { Badge } from '@/components/ui/badge'
import { DropdownMenu, DropdownMenuContent, DropdownMenuItem, DropdownMenuSeparator, DropdownMenuTrigger } from '@/components/ui/dropdown-menu'
import { useUpdaterStore } from '@/stores/updater'
import UpdateNoticeDialog from '@/components/updater/UpdateNoticeDialog.vue'
import SystemConfigDialog from '@/components/system/SystemConfigDialog.vue'
import MainWindowEffectsControl from '@/components/layout/MainWindowEffectsControl.vue'
import { useMainWindowEffects } from '@/composables/useMainWindowEffects'

interface Props {
  deviceCountText?: string
  title?: string
  subtitle?: string
  icon?: any
  back?: boolean
  backTo?: string
  closeable?: boolean
  embedded?: boolean
  compactToolbar?: boolean
  showLogo?: boolean
  showVersion?: boolean
  showNav?: boolean
  showEffects?: boolean
  showConfig?: boolean
}

const props = withDefaults(defineProps<Props>(), {
  deviceCountText: '',
  title: undefined,
  subtitle: undefined,
  icon: undefined,
  back: false,
  backTo: '/',
  closeable: false,
  embedded: false,
  compactToolbar: false,
  showLogo: undefined,
  showVersion: undefined,
  showNav: undefined,
  showEffects: undefined,
  showConfig: undefined,
})

const emit = defineEmits<{
  (e: 'close'): void
  (e: 'back'): void
}>()

const configOpen = ref(false)
const effects = useMainWindowEffects()
const isHeaderHovered = ref(false)

const isMac = typeof navigator !== 'undefined' && /Mac|iPhone|iPod|iPad/i.test(navigator.userAgent || navigator.platform)
const isMacDesktop = computed(() => (effects.native || Boolean((window as any).__TAURI_INTERNALS__)) && isMac)

const updater = useUpdaterStore()
const appIconUrl = `${import.meta.env.BASE_URL}app-icon.png`

const shouldShowLogo = computed(() => {
  if (props.showLogo !== undefined) return props.showLogo
  return !props.back && !props.closeable && !props.title
})

const shouldShowVersion = computed(() => {
  if (props.showVersion !== undefined) return props.showVersion
  return shouldShowLogo.value
})

const shouldShowNav = computed(() => {
  if (props.showNav !== undefined) return props.showNav
  return !props.embedded && (!props.back || props.title === '大屏监控')
})

const shouldShowEffects = computed(() => {
  if (props.showEffects !== undefined) return props.showEffects
  return !props.embedded
})

const shouldShowConfig = computed(() => {
  if (props.showConfig !== undefined) return props.showConfig
  return !props.embedded
})

let lastToggleTime = 0

async function handleHeaderDblClick(e: MouseEvent) {
  if (!isTauri() || props.embedded) return
  // 排除按钮、链接、输入框、下拉菜单、滑动条等交互元素上的点击
  if ((e.target as HTMLElement).closest('button, a, input, select, textarea, [role="button"], [role="menuitem"], [role="slider"]')) {
    return
  }
  const now = Date.now()
  if (now - lastToggleTime < 400) {
    return
  }
  lastToggleTime = now
  try {
    await invoke('toggle_window_maximize')
    return
  }
  catch (e1) {
    console.warn('[Desktop] invoke toggle_window_maximize 失败', e1)
  }
  try {
    const win = getCurrentWindow()
    await win.toggleMaximize()
  }
  catch (err) {
    console.warn('[Desktop] 切换窗口最大化失败', err)
  }
}

function handleHeaderMouseDown(e: MouseEvent) {
  // WebKit 拖拽区域下，连续双击时 mousedown 会带有 detail === 2
  if (e.detail === 2) {
    void handleHeaderDblClick(e)
  }
}

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
    v-bind="$attrs"
    data-tauri-drag-region
    class="z-40 border-b border-slate-200/80 bg-white/80 dark:border-white/10 dark:bg-slate-950/60 backdrop-blur-2xl shadow-xs transition-all duration-200 select-none"
    :class="[
      effects.state.value.borderless
        ? [
            'fixed top-0 inset-x-0 shadow-md',
            isHeaderHovered ? 'translate-y-0 opacity-100 pointer-events-auto' : '-translate-y-full opacity-0 pointer-events-none',
          ]
        : 'sticky top-0 z-10',
      compactToolbar ? 'toolbar-compact' : '',
    ]"
    @mouseleave="isHeaderHovered = false"
    @dblclick="handleHeaderDblClick"
    @mousedown="handleHeaderMouseDown"
  >
    <div
      data-tauri-drag-region
      class="flex w-full items-center gap-2 transition-all sm:gap-3"
      :class="[
        embedded
          ? 'h-11 px-2.5 cursor-move'
          : isMacDesktop
            ? 'h-11 pl-[82px] pr-4'
            : 'h-14 px-4',
      ]"
      @dblclick="handleHeaderDblClick"
      @mousedown="handleHeaderMouseDown"
    >
      <!-- 左侧内容区：支持 slot 覆盖，或默认支持返回键/关闭键/标题/Logo/版本号 -->
      <slot name="left">
        <div class="flex min-w-0 items-center gap-2.5">
          <!-- 小窗模式关闭按键 -->
          <Button
            v-if="closeable"
            variant="ghost"
            size="icon"
            aria-label="关闭窗口"
            title="关闭窗口"
            class="h-8 w-8 rounded-xl text-slate-600 hover:text-red-600 hover:bg-red-500/10 dark:text-slate-300 dark:hover:text-red-400 dark:hover:bg-red-500/15 cursor-pointer transition-colors"
            @click="emit('close')"
          >
            <X class="h-4 w-4" />
          </Button>

          <!-- 页面返回键 -->
          <Button
            v-else-if="back"
            variant="ghost"
            size="icon"
            aria-label="返回"
            class="h-8 w-8 rounded-xl border border-slate-200/70 bg-white/70 text-slate-700 hover:bg-slate-200/80 hover:text-slate-950 dark:border-white/10 dark:bg-white/5 dark:text-slate-200 dark:hover:bg-white/15 dark:hover:text-white cursor-pointer transition-colors"
            as-child
            @click="emit('back')"
          >
            <RouterLink :to="backTo">
              <ArrowLeft class="h-4 w-4" />
            </RouterLink>
          </Button>

          <!-- 首页默认品牌 Logo + MyWebScrcpy -->
          <template v-if="shouldShowLogo">
            <RouterLink to="/" class="flex min-w-0 items-center gap-2 group">
              <img
                :src="appIconUrl"
                alt=""
                class="shrink-0 object-contain transition-transform group-hover:scale-105"
                :class="isMacDesktop ? 'h-6 w-6' : 'h-8 w-8'"
                width="32"
                height="32"
              />
              <span
                class="truncate font-bold tracking-tight text-slate-900 dark:text-white"
                :class="isMacDesktop ? 'text-sm' : 'text-base'"
              >
                MyWebScrcpy
              </span>
            </RouterLink>

            <!-- 版本号常显（点击可打开版本信息弹窗） -->
            <button
              v-if="shouldShowVersion && updater.checkResult?.current_version"
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
          </template>

          <!-- 自定义图标 -->
          <component
            :is="icon"
            v-if="icon"
            class="h-5 w-5 shrink-0 text-slate-800 dark:text-slate-200"
          />

          <!-- 页面标题 -->
          <span
            v-if="title"
            class="truncate font-bold tracking-tight text-slate-900 dark:text-white"
            :class="isMacDesktop ? 'text-sm' : 'text-base'"
          >
            {{ title }}
          </span>

          <!-- 副标题/状态徽标 -->
          <span
            v-if="subtitle || deviceCountText"
            class="inline-flex items-center gap-1.5 rounded-full px-2.5 py-0.5 text-xs font-semibold backdrop-blur-md bg-slate-200/70 text-slate-600 border border-slate-300/60 dark:bg-white/5 dark:text-slate-300 dark:border-white/10"
          >
            {{ subtitle || deviceCountText }}
          </span>
        </div>
      </slot>

      <!-- 中间自定义区域 -->
      <slot name="center" />

      <!-- 右侧操作与工具区 -->
      <div class="ml-auto flex shrink-0 items-center gap-1.5 sm:gap-2">
        <!-- 页面自定义操作插槽 -->
        <slot name="actions" />

        <!-- 桌面端导航项（大屏监控、AI 集成、GitHub、API 文档） -->
        <div v-if="shouldShowNav" class="hidden shrink-0 items-center gap-2 xl:flex">
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
              大屏监控
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

        <!-- 窗口效果调节 (透明度/无边框/置顶) -->
        <MainWindowEffectsControl v-if="shouldShowEffects" />

        <!-- 系统设置 -->
        <Button
          v-if="shouldShowConfig"
          variant="ghost"
          size="sm"
          class="h-8 rounded-xl border border-slate-200/80 bg-slate-100/70 text-slate-800 hover:bg-slate-200/80 hover:text-slate-950 dark:border-white/15 dark:bg-white/10 dark:text-slate-100 dark:hover:bg-white/20 dark:hover:text-white text-xs font-medium px-3 gap-1.5 shadow-2xs backdrop-blur-md transition-all cursor-pointer"
          aria-label="系统设置"
          @click="configOpen = true"
        >
          <Settings class="size-4" />
          <span class="hidden sm:inline">系统设置</span>
        </Button>

        <!-- 移动端更多折叠菜单 -->
        <DropdownMenu v-if="shouldShowNav">
          <DropdownMenuTrigger as-child>
            <Button class="xl:hidden h-8 w-8 rounded-xl text-slate-700 dark:text-slate-300 hover:bg-slate-100 dark:hover:bg-white/10" variant="ghost" size="icon" title="更多" aria-label="更多">
              <Ellipsis class="h-5 w-5" />
            </Button>
          </DropdownMenuTrigger>
          <DropdownMenuContent align="end" class="rounded-2xl border-slate-200 bg-white/95 dark:border-white/10 dark:bg-slate-900/95 backdrop-blur-xl">
            <slot name="more" />
            <DropdownMenuItem v-if="updater.checkResult?.has_update" @select="updater.openDialog">
              <Sparkles class="h-4 w-4" />发现新版本 {{ updater.checkResult.latest_version }}
            </DropdownMenuItem>
            <DropdownMenuSeparator v-if="updater.checkResult?.has_update" />
            <DropdownMenuItem as-child>
              <RouterLink to="/ai-integration"><Bot class="h-4 w-4" />AI 集成</RouterLink>
            </DropdownMenuItem>
            <DropdownMenuItem as-child>
              <RouterLink to="/dashboard"><LayoutGrid class="h-4 w-4" />大屏监控</RouterLink>
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
    </div>
    <UpdateNoticeDialog />
    <SystemConfigDialog v-model:open="configOpen" />
  </header>
</template>
