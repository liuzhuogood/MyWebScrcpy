<script setup lang="ts">
import { ArrowLeft } from 'lucide-vue-next'
import { RouterLink } from 'vue-router'
import { Button } from '@/components/ui/button'
import DependencyPanel from '@/components/dependencies/DependencyPanel.vue'
</script>

<template>
  <div class="flex h-dvh flex-col bg-background">
    <header class="flex h-14 shrink-0 items-center gap-3 border-b px-4">
      <Button as-child variant="ghost" size="icon"><RouterLink to="/" aria-label="返回设备列表"><ArrowLeft class="size-4" /></RouterLink></Button>
      <span class="font-semibold">MyWebScrcpy</span>
    </header>
    <main class="mx-auto flex min-h-0 w-full max-w-2xl flex-1 flex-col">
      <h1 class="shrink-0 px-5 pt-5 text-xl font-semibold">依赖管理</h1>
      <DependencyPanel class="flex-1" />
    </main>
  </div>
</template>
