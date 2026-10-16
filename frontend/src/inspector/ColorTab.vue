<script setup lang="ts">
import { useInspector } from './useInspector'

const ins = useInspector()
</script>

<template>
  <div class="space-y-2 p-3">
    <dl class="grid grid-cols-[auto_1fr] gap-x-3 gap-y-1 text-sm">
      <dt class="text-muted-foreground">坐标</dt>
      <dd class="font-mono">{{ ins.colorText.value.pos }}</dd>
      <dt class="text-muted-foreground">归一化</dt>
      <dd class="font-mono">{{ ins.colorText.value.norm }}</dd>
      <dt class="text-muted-foreground">颜色</dt>
      <dd class="font-mono">{{ ins.colorText.value.color }}</dd>
    </dl>
    <p v-if="ins.colorPinned.value" class="text-xs text-muted-foreground">已固定，再次进入检查模式可重新取色</p>
    <canvas :ref="ins.magnifierRef" width="240" height="240" class="h-60 w-60 rounded-md border" aria-label="像素放大镜" />
  </div>
</template>
