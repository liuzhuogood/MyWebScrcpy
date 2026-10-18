<script setup lang="ts">
import { onMounted, ref } from 'vue'
import OcrTab from '@/inspector/OcrTab.vue'
import { provideInspector } from '@/inspector/useInspector'

const canvas = ref<HTMLCanvasElement | null>(null)
// 实际检查器与请求链；画面只来自本地 canvas，接口由专用预览服务模拟。
provideInspector('ocr-parallel-fixture', () => canvas.value)
onMounted(() => {
  const context = canvas.value?.getContext('2d')
  if (!context) throw new Error('无法创建 fixture 画面')
  context.fillStyle = '#fff'
  context.fillRect(0, 0, 360, 800)
  context.fillStyle = '#000'
  context.font = '24px sans-serif'
  context.fillText('设置', 36, 80)
})
</script>

<template>
  <main class="min-h-screen bg-background p-4 text-foreground">
    <section class="h-[720px] w-full max-w-[360px] overflow-hidden rounded-xl border">
      <h1 class="border-b px-3 py-2 text-sm font-semibold">OCR</h1>
      <div class="h-[670px]"><OcrTab /></div>
    </section>
    <canvas ref="canvas" width="360" height="800" hidden />
  </main>
</template>
