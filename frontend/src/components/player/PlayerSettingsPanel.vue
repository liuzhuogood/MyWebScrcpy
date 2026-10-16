<script setup lang="ts">
import { Settings } from 'lucide-vue-next'
import SidePanelHeader from '@/components/layout/SidePanelHeader.vue'
import { Switch } from '@/components/ui/switch'

defineProps<{
  touchVisible: boolean
  boxesVisible: boolean
  autoUnlockOnWake: boolean
  clipboardSync: boolean
  borderless?: boolean
  windowBusy?: boolean
}>()
const emit = defineEmits<{
  'update:touchVisible': [value: boolean]
  'update:boxesVisible': [value: boolean]
  'update:autoUnlockOnWake': [value: boolean]
  'update:clipboardSync': [value: boolean]
  'update:borderless': [value: boolean]
  close: []
}>()
</script>

<template>
  <aside class="flex h-full min-h-0 flex-col overflow-hidden bg-card" aria-label="投屏设置">
    <SidePanelHeader title="设置" @close="emit('close')">
      <template #title>
        <div class="flex min-w-0 items-center gap-2">
          <Settings class="h-4 w-4 shrink-0 text-muted-foreground" />
          <strong class="truncate text-sm font-semibold">设置</strong>
        </div>
      </template>
    </SidePanelHeader>
    <div class="space-y-4 overflow-auto p-4">
      <div v-if="borderless !== undefined" class="flex items-center justify-between gap-4">
        <p class="text-sm font-medium">无边框</p>
        <Switch :model-value="borderless" :disabled="windowBusy" aria-label="无边框" @update:model-value="emit('update:borderless', !!$event)" />
      </div>
      <div class="flex items-center justify-between gap-4">
        <div><p class="text-sm font-medium">剪贴板共享</p><p class="text-xs text-muted-foreground">在电脑与设备间同步剪贴板文本</p></div>
        <Switch :model-value="clipboardSync" aria-label="剪贴板共享" @update:model-value="emit('update:clipboardSync', !!$event)" />
      </div>
      <div class="flex items-center justify-between gap-4">
        <div><p class="text-sm font-medium">显示触摸反馈</p><p class="text-xs text-muted-foreground">在投屏画面中显示点击位置</p></div>
        <Switch :model-value="touchVisible" aria-label="显示触摸反馈" @update:model-value="emit('update:touchVisible', !!$event)" />
      </div>
      <div class="flex items-center justify-between gap-4">
        <div><p class="text-sm font-medium">显示匹配绿框</p><p class="text-xs text-muted-foreground">显示模板匹配和检查器定位框</p></div>
        <Switch :model-value="boxesVisible" aria-label="显示匹配绿框" @update:model-value="emit('update:boxesVisible', !!$event)" />
      </div>
      <div class="flex items-center justify-between gap-4">
        <div><p class="text-sm font-medium">唤醒后尝试解锁</p><p class="text-xs text-muted-foreground">有密码时仍需手动验证</p></div>
        <Switch :model-value="autoUnlockOnWake" aria-label="唤醒后尝试解锁" @update:model-value="emit('update:autoUnlockOnWake', !!$event)" />
      </div>
    </div>
  </aside>
</template>
