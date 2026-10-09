<script setup lang="ts">
import { Settings, X } from 'lucide-vue-next'
import { Switch } from '@/components/ui/switch'

defineProps<{ touchVisible: boolean, boxesVisible: boolean }>()
const emit = defineEmits<{
  'update:touchVisible': [value: boolean]
  'update:boxesVisible': [value: boolean]
  close: []
}>()
</script>

<template>
  <aside class="flex h-full min-h-0 flex-col overflow-hidden rounded-xl border bg-card" aria-label="投屏设置">
    <header class="flex h-11 shrink-0 items-center gap-2 border-b px-3">
      <Settings class="h-4 w-4" /><strong class="text-sm">设置</strong>
      <button type="button" class="ml-auto rounded-md p-1.5 text-muted-foreground hover:bg-accent hover:text-foreground" aria-label="关闭设置" @click="emit('close')"><X class="h-4 w-4" /></button>
    </header>
    <div class="space-y-4 overflow-auto p-4">
      <div class="flex items-center justify-between gap-4">
        <div><p class="text-sm font-medium">显示触摸反馈</p><p class="text-xs text-muted-foreground">在投屏画面中显示点击位置</p></div>
        <Switch :model-value="touchVisible" aria-label="显示触摸反馈" @update:model-value="emit('update:touchVisible', !!$event)" />
      </div>
      <div class="flex items-center justify-between gap-4">
        <div><p class="text-sm font-medium">显示匹配绿框</p><p class="text-xs text-muted-foreground">显示模板匹配和检查器定位框</p></div>
        <Switch :model-value="boxesVisible" aria-label="显示匹配绿框" @update:model-value="emit('update:boxesVisible', !!$event)" />
      </div>
    </div>
  </aside>
</template>
