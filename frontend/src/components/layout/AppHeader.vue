<script setup lang="ts">
import { Github, LayoutGrid, MonitorSmartphone } from 'lucide-vue-next'
import { RouterLink } from 'vue-router'
import { Button } from '@/components/ui/button'

defineProps<{ deviceCountText: string }>()
</script>

<template>
  <header class="sticky top-0 z-10 border-b bg-background/80 backdrop-blur">
    <div class="flex h-14 w-full items-center gap-3 px-4">
      <RouterLink to="/" class="flex items-center gap-2">
        <span class="flex h-8 w-8 items-center justify-center rounded-md bg-primary text-primary-foreground">
          <MonitorSmartphone class="h-5 w-5" />
        </span>
        <span class="text-base font-semibold">MyWebScrcpy</span>
      </RouterLink>
      <span class="text-sm text-muted-foreground">{{ deviceCountText }}</span>
      <div class="ml-auto flex items-center gap-2">
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
  </header>
</template>
