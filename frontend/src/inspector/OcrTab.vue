<script setup lang="ts">
import { onUnmounted } from 'vue'
import { Copy, Crop, Loader2, ScanText, Square } from 'lucide-vue-next'
import { toast } from 'vue-sonner'
import { Button } from '@/components/ui/button'
import { Input } from '@/components/ui/input'
import { ScrollArea } from '@/components/ui/scroll-area'
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from '@/components/ui/select'
import { Switch } from '@/components/ui/switch'
import { useInspector } from './useInspector'

const ins = useInspector()
const ocr = ins.ocr
const engineNames: Record<string, string> = { indraocr: 'Indra OCR', macocr: 'macOCR', paddleocr: 'PaddleOCR' }

function copyText(text: string) {
  navigator.clipboard.writeText(text).then(
    () => toast.success('已复制文本'),
    () => toast.error('复制失败'),
  )
}
onUnmounted(ocr.panelHidden)
</script>

<template>
  <div class="flex h-full min-h-0 flex-col gap-3 overflow-auto p-3">
    <div class="shrink-0 space-y-3">
      <Select v-model="ocr.engine.value">
        <SelectTrigger class="w-full text-xs" aria-label="识别引擎"><SelectValue /></SelectTrigger>
        <SelectContent>
          <SelectItem value="auto">自动</SelectItem>
          <SelectItem value="indraocr">Indra OCR</SelectItem>
          <SelectItem value="macocr">macOCR</SelectItem>
          <SelectItem value="paddleocr">PaddleOCR</SelectItem>
        </SelectContent>
      </Select>
      <div class="grid gap-1 rounded-lg bg-muted p-1" :class="ins.selectedNode.value ? 'grid-cols-3' : 'grid-cols-2'">
        <Button size="sm" :variant="ocr.scope.value === 'screen' ? 'default' : 'ghost'" :aria-pressed="ocr.scope.value === 'screen'" class="h-7 text-xs" @click="ocr.scope.value = 'screen'"><ScanText class="size-3.5" />全屏</Button>
        <Button size="sm" :variant="ocr.scope.value === 'region' ? 'default' : 'ghost'" :aria-pressed="ocr.scope.value === 'region'" class="h-7 text-xs" @click="ocr.beginRegionSelection"><Crop class="size-3.5" />框选区域</Button>
        <Button v-if="ins.selectedNode.value" size="sm" :variant="ocr.scope.value === 'element' ? 'default' : 'ghost'" :aria-pressed="ocr.scope.value === 'element'" class="h-7 text-xs" @click="ocr.scope.value = 'element'">选中元素</Button>
      </div>
      <p v-if="ocr.selecting.value" class="text-xs text-muted-foreground">在画面上拖动框选识别区域</p>
      <div class="flex items-center justify-between text-xs"><label for="inspector-continuous-ocr">连续识别</label><Switch id="inspector-continuous-ocr" v-model="ocr.continuous.value" /></div>
      <Button v-if="ocr.running.value" size="sm" variant="destructive" class="w-full" @click="ocr.stop"><Square class="size-3.5" />停止识别</Button>
      <Button v-else size="sm" class="w-full" :disabled="ocr.loading.value || ocr.selecting.value" @click="ocr.run"><Loader2 v-if="ocr.loading.value" class="size-4 animate-spin" /><ScanText v-else class="size-4" />{{ ocr.loading.value ? '识别中…' : ocr.result.value ? '重新识别' : '开始识别' }}</Button>
    </div>
    <p v-if="ocr.error.value" role="alert" class="shrink-0 text-xs text-destructive [overflow-wrap:anywhere]">{{ ocr.error.value }}</p>
    <template v-if="ocr.result.value">
      <div class="shrink-0 space-y-3 border-t pt-3">
        <div class="flex flex-wrap items-center justify-between gap-1 text-xs">
          <span class="text-muted-foreground">{{ engineNames[ocr.result.value.engine] }} · {{ ocr.result.value.elapsed_ms }} ms · {{ ocr.result.value.boxes.length }} 处</span>
          <Button size="sm" variant="ghost" class="h-6 px-1.5 text-xs" :disabled="!ocr.result.value.text" @click="copyText(ocr.result.value.text)"><Copy class="size-3" />复制全部</Button>
        </div>
        <div class="flex items-center justify-between text-xs"><label for="inspector-ocr-boxes">显示文字框</label><Switch id="inspector-ocr-boxes" v-model="ocr.showBoxes.value" /></div>
        <Input v-if="ocr.result.value.boxes.length" v-model="ocr.keyword.value" aria-label="筛选识别文字" placeholder="筛选文字" class="h-8 text-xs" />
      </div>
      <ScrollArea class="min-h-24 flex-1">
        <div v-if="ocr.filtered.value.length" class="space-y-1 pr-2">
          <div v-for="({ box, index }) in ocr.filtered.value" :key="index" class="flex items-center gap-1 rounded-lg border p-2.5 transition-colors" :class="ocr.selected.value === index ? 'border-primary bg-primary/5' : 'border-transparent hover:bg-muted/60'">
            <Button variant="ghost" class="h-auto min-w-0 flex-1 flex-col items-start gap-0 whitespace-normal p-0 text-left hover:bg-transparent" :aria-label="`选中 ${box.text}`" :aria-pressed="ocr.selected.value === index" @click="ocr.select(index)">
              <span class="w-full break-words text-sm font-normal">{{ box.text }}</span>
              <span class="text-[10px] font-normal text-muted-foreground">{{ (box.confidence * 100).toFixed(1) }}%</span>
              <span class="break-words font-mono text-[10px] font-normal leading-4 text-muted-foreground">{{ ocr.coordinates(box) }}</span>
            </Button>
            <Button size="icon" variant="ghost" class="size-7 shrink-0" :aria-label="`复制 ${box.text}`" @click="copyText(box.text)"><Copy class="size-3.5" /></Button>
          </div>
        </div>
        <div v-else class="p-6 text-center text-xs text-muted-foreground">{{ ocr.result.value.boxes.length ? '没有匹配的文字' : '未识别到文字' }}</div>
      </ScrollArea>
      <div class="flex shrink-0 justify-end"><Button size="sm" variant="ghost" class="text-xs text-muted-foreground" @click="ocr.clear">清空结果</Button></div>
    </template>
    <div v-else class="flex min-h-24 flex-1 flex-col items-center justify-center gap-2 py-6 text-muted-foreground"><ScanText class="size-8 opacity-40" /><p class="text-xs">{{ ocr.loading.value ? '正在识别画面文字' : '识别结果将显示在这里' }}</p></div>
  </div>
</template>
