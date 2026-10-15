<script setup lang="ts">
import { computed, ref } from 'vue'
import { Ellipsis, GalleryHorizontal, Home, Pin, Undo2, X } from 'lucide-vue-next'
import { Button } from '@/components/ui/button'
import { Card, CardContent } from '@/components/ui/card'
import WindowEffectsControl from '@/components/player/WindowEffectsControl.vue'

const opacity = ref(100)
const clickThrough = ref(false)
const needsRecovery = computed(() => opacity.value < 100 || clickThrough.value)
function restore() {
  opacity.value = 100
  clickThrough.value = false
}
</script>

<template>
  <div class="mx-auto grid w-full max-w-4xl gap-8 p-4 sm:grid-cols-[minmax(0,1fr)_320px] sm:p-8">
    <Card class="h-fit">
      <CardContent class="flex flex-wrap items-center justify-between gap-3 p-4">
        <div class="min-w-0"><p class="font-medium">Pixel 8</p><p class="mt-1 font-mono text-xs text-muted-foreground">emulator-5554</p></div>
        <Button v-if="needsRecovery" variant="outline" size="sm" @click="restore">恢复小窗</Button>
      </CardContent>
    </Card>
    <div class="relative h-[560px] overflow-hidden rounded-xl border bg-muted shadow-sm">
      <div class="absolute inset-0 grid place-content-center gap-4 p-6">
        <div class="h-10 w-48 rounded bg-background" /><div class="h-32 w-48 rounded bg-background" /><div class="h-24 w-48 rounded bg-background" />
      </div>
      <div class="relative flex h-full flex-col bg-background" :class="{ 'pointer-events-none': clickThrough }" :style="{ opacity: opacity / 100 }">
        <header class="flex h-14 shrink-0 items-center gap-0.5 border-b px-2">
          <Button variant="ghost" size="icon" aria-label="隐藏工具条"><X class="h-4 w-4" /></Button>
          <span class="min-w-0 flex-1 truncate text-sm">Pixel 8</span>
          <WindowEffectsControl :opacity="opacity" :click-through="clickThrough" shortcut="Option + P" :busy="false" @opacity="opacity = $event" @click-through="clickThrough = $event" @restore="restore" />
          <Button variant="ghost" size="icon" aria-label="保持窗口置顶"><Pin class="h-4 w-4" /></Button>
          <Button variant="ghost" size="icon" aria-label="返回"><Undo2 class="h-4 w-4" /></Button>
          <Button variant="ghost" size="icon" aria-label="主页"><Home class="h-4 w-4" /></Button>
          <Button variant="ghost" size="icon" aria-label="切换任务"><GalleryHorizontal class="h-4 w-4" /></Button>
          <Button variant="ghost" size="icon" aria-label="更多"><Ellipsis class="h-4 w-4" /></Button>
        </header>
        <div class="flex flex-1 flex-col items-center justify-center gap-5 bg-gradient-to-br from-blue-950 to-indigo-700 text-white">
          <span class="text-5xl font-light tabular-nums">09:41</span>
          <span class="text-sm text-white/70">10月15日 星期四</span>
          <div class="mt-12 grid grid-cols-3 gap-5" aria-hidden="true"><div v-for="index in 6" :key="index" class="h-12 w-12 rounded-xl bg-white/20" /></div>
        </div>
      </div>
    </div>
  </div>
</template>
