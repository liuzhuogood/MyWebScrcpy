<script setup lang="ts">
import { ref } from 'vue'
import { Button } from '@/components/ui/button'
import { Card, CardContent, CardHeader, CardTitle } from '@/components/ui/card'
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from '@/components/ui/select'

const engine = ref('auto')
const defaultEngine = ref('auto')
const resultEngine = ref('')
const engineNames: Record<string, string> = {
  auto: '自动', parallel: '并行', indraocr: 'Indra OCR', macocr: 'macOCR', paddleocr: 'PaddleOCR',
}
// 本地选择与结果演示，不调用真实 OCR、配置接口或存储。
function recognize() {
  resultEngine.value = engine.value === 'auto' ? 'indraocr' : engine.value
}
</script>

<template>
  <div class="grid items-start gap-4 md:grid-cols-[320px_minmax(0,1fr)]">
    <Card class="min-w-0">
      <CardHeader class="pb-3"><CardTitle class="text-sm font-semibold">OCR</CardTitle></CardHeader>
      <CardContent class="space-y-3 pt-0">
        <Select v-model="engine" @update:model-value="resultEngine = ''">
          <SelectTrigger class="w-full text-xs" aria-label="识别引擎"><SelectValue /></SelectTrigger>
          <SelectContent>
            <SelectItem v-for="(name, value) in engineNames" :key="value" :value="value">{{ name }}</SelectItem>
          </SelectContent>
        </Select>
        <Button size="sm" class="w-full" @click="recognize">{{ resultEngine ? '重新识别' : '开始识别' }}</Button>
        <div v-if="resultEngine" class="space-y-3 border-t pt-3">
          <p class="text-xs text-muted-foreground">{{ engineNames[resultEngine] }} · 48 ms · 1 处</p>
          <p class="text-sm">设置</p>
        </div>
        <p v-else class="py-6 text-center text-xs text-muted-foreground">识别结果将显示在这里</p>
      </CardContent>
    </Card>
    <Card class="min-w-0">
      <CardHeader class="pb-3"><CardTitle class="text-sm font-semibold">OCR 引擎与远程服务配置</CardTitle></CardHeader>
      <CardContent class="space-y-1.5 pt-0">
        <p id="default-ocr-engine-label" class="text-xs font-medium">默认 OCR 引擎模式</p>
        <div role="group" aria-labelledby="default-ocr-engine-label" class="grid grid-cols-2 gap-2 sm:grid-cols-4">
          <Button v-for="(name, value) in engineNames" :key="value" size="sm" :variant="defaultEngine === value ? 'default' : 'outline'" :aria-pressed="defaultEngine === value" class="text-xs" @click="defaultEngine = value">
            {{ value === 'auto' ? '自动降级 (auto)' : name }}
          </Button>
        </div>
        <p class="text-[11px] text-muted-foreground">自动模式：Indra OCR → macOCR → PaddleOCR，不可用时依次切换。</p>
      </CardContent>
    </Card>
  </div>
</template>
