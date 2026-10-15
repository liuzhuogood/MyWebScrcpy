<script setup lang="ts">
import { computed } from 'vue'
import { Copy, Loader2, ScanText } from 'lucide-vue-next'
import { toast } from 'vue-sonner'
import { Badge } from '@/components/ui/badge'
import { Button } from '@/components/ui/button'
import { ScrollArea } from '@/components/ui/scroll-area'
import { useInspector } from './useInspector'

const ins = useInspector()

const scopeText = computed(() => {
  if (ins.selectedNode.value) {
    const tag = ins.selectedNode.value.tagName || '选中元素'
    return `${tag} (局部区域)`
  }
  return '整个屏幕 (全屏)'
})

function copyText(text: string) {
  navigator.clipboard.writeText(text).then(
    () => toast.success('已复制文本'),
    () => toast.error('复制失败'),
  )
}
</script>

<template>
  <div class="flex h-full flex-col gap-2 p-3">
    <!-- 检测范围与执行操作 -->
    <div class="space-y-2 rounded-lg border bg-muted/30 p-2.5">
      <div class="flex items-center justify-between text-xs">
        <span class="text-muted-foreground">检测范围:</span>
        <span class="font-medium truncate max-w-[180px]">{{ scopeText }}</span>
      </div>

      <div class="flex gap-2">
        <Button
          size="sm"
          class="flex-1 text-xs"
          :disabled="ins.ocrLoading.value"
          @click="ins.runOcr()"
        >
          <Loader2 v-if="ins.ocrLoading.value" class="mr-1.5 h-3.5 w-3.5 animate-spin" />
          <ScanText v-else class="mr-1.5 h-3.5 w-3.5" />
          {{ ins.selectedNode.value ? '识别选中元素' : '全屏 OCR 检测' }}
        </Button>
        <Button
          v-if="ins.selectedNode.value"
          size="sm"
          variant="outline"
          class="text-xs"
          @click="ins.selectNode(null)"
        >
          取消选择
        </Button>
      </div>
    </div>

    <!-- 错误提示 -->
    <p v-if="ins.ocrError.value" class="text-xs text-destructive">
      {{ ins.ocrError.value }}
    </p>

    <!-- 识别结果展示 -->
    <template v-if="ins.ocrResult.value">
      <div class="flex items-center justify-between">
        <span class="text-xs text-muted-foreground">
          识别文字 ({{ ins.ocrResult.value.boxes?.length || 0 }} 处 · {{ ins.ocrResult.value.elapsed_ms }}ms)
        </span>
        <Button
          v-if="ins.ocrResult.value.text"
          size="sm"
          variant="ghost"
          class="h-6 px-1.5 text-xs"
          @click="copyText(ins.ocrResult.value.text)"
        >
          <Copy class="mr-1 h-3 w-3" />复制全部
        </Button>
      </div>

      <ScrollArea class="min-h-0 flex-1 rounded-md border">
        <div v-if="ins.ocrResult.value.boxes?.length" class="space-y-1.5 p-2">
          <div
            v-for="(box, i) in ins.ocrResult.value.boxes"
            :key="i"
            class="rounded border bg-card p-2 text-xs transition-colors hover:bg-muted/50"
          >
            <div class="flex items-center justify-between gap-1">
              <span class="font-medium text-foreground select-text">{{ box.text }}</span>
              <Badge variant="outline" class="font-mono text-[10px] text-emerald-500 border-emerald-500/30">
                {{ Math.round(box.confidence * 100) }}%
              </Badge>
            </div>
            <div class="mt-1 flex items-center justify-between text-[11px] text-muted-foreground font-mono">
              <span>x: {{ Math.round(box.x * 100) }}% y: {{ Math.round(box.y * 100) }}%</span>
              <Button
                size="sm"
                variant="ghost"
                class="h-5 px-1 text-[11px]"
                @click="copyText(box.text)"
              >
                复制
              </Button>
            </div>
          </div>
        </div>
        <div v-else class="p-6 text-center text-xs text-muted-foreground">
          范围内未识别到文字
        </div>
      </ScrollArea>
    </template>

    <!-- 空状态 -->
    <div v-else-if="!ins.ocrLoading.value && !ins.ocrError.value" class="flex flex-1 flex-col items-center justify-center p-6 text-center text-xs text-muted-foreground border rounded-md border-dashed">
      <ScanText class="h-8 w-8 text-muted-foreground/40 mb-2" />
      <p>点击「{{ ins.selectedNode.value ? '识别选中元素' : '全屏 OCR 检测' }}」</p>
      <p class="text-[11px] text-muted-foreground/70 mt-1">识别文本将在屏幕上自动渲染绿色外框</p>
    </div>
  </div>
</template>
