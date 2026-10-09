<script setup lang="ts">
import { RouterLink } from 'vue-router'
import { Button } from '@/components/ui/button'

defineProps<{ title: string; serial?: string }>()
</script>

<template>
  <main class="mx-auto flex min-h-screen max-w-xl flex-col items-center justify-center gap-3 px-4 text-center">
    <h1 class="text-xl font-semibold">{{ title }}</h1>
    <p class="text-sm text-muted-foreground">
      {{ serial ? `设备 ${serial} 的新版页面将在后续迭代实现。` : '该页面将在后续迭代实现，当前仅完成首页。' }}
    </p>
    <Button as-child><RouterLink to="/">返回首页</RouterLink></Button>
  </main>
</template>
