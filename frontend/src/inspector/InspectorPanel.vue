<script setup lang="ts">
import { onMounted, onUnmounted, watch } from 'vue'
import SidePanelHeader from '@/components/layout/SidePanelHeader.vue'
import { Tabs, TabsContent, TabsList, TabsTrigger } from '@/components/ui/tabs'
import ColorTab from './ColorTab.vue'
import { inspectorTab, setInspector } from './guard'
import OcrTab from './OcrTab.vue'
import OpLogTab from './OpLogTab.vue'
import PageInfoTab from './PageInfoTab.vue'
import { useInspector } from './useInspector'
import XmlTab from './XmlTab.vue'

const emit = defineEmits<{ (e: 'close'): void }>()

const ins = useInspector()

function onTabChange(tab: string | number) {
  inspectorTab.value = tab as typeof inspectorTab.value
  ins.redraw()
}

// 面板自动化点击只派发 click（Reka TabsTrigger 靠 mousedown 选中），显式同步一份
function onTabClick(tab: typeof inspectorTab.value) {
  if (inspectorTab.value !== tab) {
    inspectorTab.value = tab
    ins.redraw()
  }
}

function close() {
  ins.closeInspector()
  setInspector(false)
  emit('close')
}

function onCanvasMove(e: MouseEvent) {
  ins.hoverColor(e)
}

function onCanvasLeave() {
  ins.leaveColor()
}

function onCanvasClick(e: MouseEvent) {
  ins.clickCanvas(e)
}

onMounted(() => {
  const canvas = ins.getCanvas()
  canvas?.addEventListener('mousemove', onCanvasMove)
  canvas?.addEventListener('mouseleave', onCanvasLeave)
  canvas?.addEventListener('click', onCanvasClick)
  // 打开即预取页面信息；XML 等切到对应 Tab 再取（与老行为一致）
  void ins.loadPageInfo()
})

// XML Tab 首次进入时自动加载快照（已加载成功或正在加载则跳过；加载失败不自动重试，由用户手动刷新）
watch(() => inspectorTab.value, (tab) => {
  if (tab === 'xml' && !ins.xmlDoc.value && !ins.xmlLoading.value && !ins.xmlError.value)
    void ins.loadXml()
})

onUnmounted(() => {
  const canvas = ins.getCanvas()
  canvas?.removeEventListener('mousemove', onCanvasMove)
  canvas?.removeEventListener('mouseleave', onCanvasLeave)
  canvas?.removeEventListener('click', onCanvasClick)
})
</script>

<template>
  <aside class="flex h-full min-h-0 w-full flex-col overflow-hidden bg-card" aria-label="检查面板">
    <SidePanelHeader title="检查" @close="close()" />
    <Tabs :model-value="inspectorTab" class="flex min-h-0 flex-1 flex-col overflow-hidden" @update:model-value="onTabChange">
      <TabsList class="mx-3 mt-2 grid shrink-0 grid-cols-5">
        <TabsTrigger value="color" @click="onTabClick('color')">取色</TabsTrigger>
        <TabsTrigger value="xml" @click="onTabClick('xml')">XML</TabsTrigger>
        <TabsTrigger value="ocr" @click="onTabClick('ocr')">OCR</TabsTrigger>
        <TabsTrigger value="page" @click="onTabClick('page')">页面</TabsTrigger>
        <TabsTrigger value="log" @click="onTabClick('log')">日志</TabsTrigger>
      </TabsList>
      <TabsContent value="color" class="min-h-0 flex-1 overflow-auto"><ColorTab /></TabsContent>
      <TabsContent value="xml" class="min-h-0 flex-1 overflow-hidden"><XmlTab /></TabsContent>
      <TabsContent value="ocr" class="min-h-0 flex-1 overflow-hidden"><OcrTab /></TabsContent>
      <TabsContent value="page" class="min-h-0 flex-1 overflow-auto"><PageInfoTab /></TabsContent>
      <TabsContent value="log" class="min-h-0 flex-1 overflow-hidden"><OpLogTab /></TabsContent>
    </Tabs>
  </aside>
</template>
