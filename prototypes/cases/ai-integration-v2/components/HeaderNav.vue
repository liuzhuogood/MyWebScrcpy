<script setup lang="ts">
import { ListTodo, Minus, Monitor, Settings, Smartphone, Sparkles, Square, X } from 'lucide-vue-next'
</script>

<template>
  <header class="w-full border-b bg-background/95 backdrop-blur supports-[backdrop-filter]:bg-background/60">
    <div class="mx-auto flex h-14 max-w-7xl items-center justify-between px-3 sm:px-6">
      <!-- 品牌标识 -->
      <div class="flex items-center gap-2 sm:gap-3">
        <div class="flex h-8 w-8 shrink-0 items-center justify-center rounded-lg bg-primary text-sm font-bold text-primary-foreground shadow-sm">
          M
        </div>
        <span class="text-sm font-semibold tracking-tight text-foreground sm:text-base">MyWebScrcpy</span>
        <span class="hidden text-xs text-muted-foreground sm:inline">v1.26.0</span>
      </div>

      <!-- 右侧导航视觉与窗口装饰 -->
      <div class="flex items-center gap-1 sm:gap-2">
        <nav class="flex items-center gap-1 sm:gap-2" aria-label="应用导航概览">
          <span class="hidden items-center gap-1.5 px-2 py-1 text-sm text-muted-foreground md:inline-flex">
            <Smartphone class="h-4 w-4" aria-hidden="true" />
            <span>设备</span>
          </span>

          <span class="hidden items-center gap-1.5 px-2 py-1 text-sm text-muted-foreground md:inline-flex">
            <ListTodo class="h-4 w-4" aria-hidden="true" />
            <span>任务</span>
          </span>

          <!-- 当前高亮入口：AI 集成 -->
          <span class="inline-flex items-center gap-1 rounded-full bg-primary/10 px-2.5 py-1 text-xs font-medium text-primary sm:gap-1.5 sm:px-3 sm:text-sm">
            <Sparkles class="h-3.5 w-3.5 sm:h-4 sm:w-4" aria-hidden="true" />
            <span>AI 集成</span>
          </span>

          <span class="hidden items-center gap-1.5 px-2 py-1 text-sm text-muted-foreground lg:inline-flex">
            <Monitor class="h-4 w-4" aria-hidden="true" />
            <span>大屏模式</span>
          </span>

          <span class="hidden items-center gap-1.5 px-2 py-1 text-sm text-muted-foreground lg:inline-flex">
            <Settings class="h-4 w-4" aria-hidden="true" />
            <span>系统设置</span>
          </span>
        </nav>

        <div class="ml-2 hidden items-center gap-1 border-l pl-3 text-muted-foreground/60 sm:flex" aria-hidden="true">
          <span class="p-1"><Minus class="h-3.5 w-3.5" /></span>
          <span class="p-1"><Square class="h-3.5 w-3.5" /></span>
          <span class="p-1"><X class="h-3.5 w-3.5" /></span>
        </div>
      </div>
    </div>
  </header>
</template>
