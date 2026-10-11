<script setup lang="ts">
import { computed } from 'vue'
import { Check, Code2, Copy, FileCode2, FileText, Image, Play, Square } from 'lucide-vue-next'
import { Button } from '@/components/ui/button'
import { Badge } from '@/components/ui/badge'
import { Tabs, TabsContent, TabsList, TabsTrigger } from '@/components/ui/tabs'
import { renderDocument, taskStatus } from '@/scripts/document'
import type { ScriptsWorkbench } from '@/scripts/useScriptsWorkbench'
const props = defineProps<{ w: ScriptsWorkbench }>()
const preview = computed(() => renderDocument(props.w.content, props.w.assets))
</script>
<template>
  <section aria-label="文档工作区" class="script-pane flex min-w-0 flex-col" :class="w.mobilePanel === 'editor' ? '' : 'hidden lg:flex'">
    <template v-if="w.current">
      <div class="space-y-2 border-b px-4 py-3">
        <div class="flex items-center gap-2"><h2 class="min-w-0 flex-1 truncate text-sm font-medium">{{ w.current.name }}</h2><span v-if="w.dirty" class="text-[10px] text-muted-foreground">未保存</span><Button size="sm" variant="outline" :disabled="!w.dirty || !!w.busy" @click="w.save"><Check class="size-3.5" />{{ w.busy === 'save' ? '保存中…' : '保存' }}</Button></div>
        <div class="flex flex-wrap gap-1"><Button size="sm" variant="ghost" class="text-xs" @click="w.templateOpen = true"><Image class="size-3.5" />模板库</Button><Button size="sm" variant="ghost" class="text-xs" :disabled="!!w.busy || w.dirty" :title="w.dirty ? '请先保存文档' : undefined" @click="w.compile"><Code2 class="size-3.5" />{{ w.busy === 'compile' ? '生成中…' : '生成 Python' }}</Button><Button size="sm" class="ml-auto text-xs" :disabled="!!w.busy || w.dirty || w.current.needs_device_configuration" :title="w.dirty ? '请先保存文档' : undefined" @click="w.run"><Play class="size-3.5" />{{ w.busy === 'run' ? '启动中…' : '运行' }}</Button></div>
      </div>
      <p v-if="w.notice" role="status" class="border-b bg-muted/30 px-4 py-2 text-xs">{{ w.notice }}</p>
      <div v-if="w.legacyTargets" class="flex flex-wrap items-center gap-2 border-b px-4 py-2 text-xs text-amber-700 dark:text-amber-400"><span class="flex-1">目标设备请在任务配置中选择，保存前请移除文档中的设备配置。</span><Button variant="outline" size="sm" class="text-xs" @click="w.removeLegacy">移除文档设备配置</Button></div>
      <Tabs v-model="w.tab" class="flex min-h-0 flex-1 flex-col gap-0">
        <TabsContent value="source" class="m-0 min-h-0 flex-1 data-[state=active]:flex"><textarea v-model="w.content" aria-label="Markdown 源码" spellcheck="false" class="h-full min-h-64 w-full resize-none border-0 bg-background p-5 font-mono text-xs leading-7 outline-none focus-visible:ring-2 focus-visible:ring-inset focus-visible:ring-ring" /></TabsContent>
        <TabsContent value="preview" class="m-0 min-h-0 flex-1 overflow-auto"><article class="script-markdown mx-auto max-w-3xl p-6" v-html="preview" /></TabsContent>
        <TabsContent value="python" class="m-0 min-h-0 flex-1 overflow-auto"><template v-if="w.current.compiled_python"><div class="sticky top-0 flex justify-end bg-background/90 px-4 py-2"><Button size="sm" variant="ghost" class="text-xs" @click="w.copyPython"><Copy class="size-3.5" />复制</Button></div><pre class="overflow-x-auto px-5 pb-5 font-mono text-xs leading-7"><code>{{ w.current.compiled_python }}</code></pre></template><div v-else class="flex h-full min-h-64 flex-col items-center justify-center gap-4 text-muted-foreground"><FileCode2 class="size-9 opacity-40" /><p class="text-sm">尚未生成 Python</p></div></TabsContent>
        <div class="flex flex-wrap items-center gap-2 border-t bg-muted/15 px-3 py-2"><TabsList class="h-8 bg-transparent p-0" aria-label="文档视图"><TabsTrigger value="source" class="gap-1.5 text-xs"><Code2 class="size-3.5" />源码</TabsTrigger><TabsTrigger value="preview" class="gap-1.5 text-xs"><FileText class="size-3.5" />预览</TabsTrigger><TabsTrigger value="python" class="gap-1.5 text-xs"><FileCode2 class="size-3.5" />Python</TabsTrigger></TabsList><div v-if="w.tab === 'source'" class="ml-auto flex gap-1"><Button v-for="tool in [{kind: 'wait', label: '等待'}, {kind: 'tap', label: '点击'}, {kind: 'swipe', label: '滑动'}] as const" :key="tool.kind" size="sm" variant="ghost" class="h-7 px-2 text-[11px]" @click="w.insertStep(tool.kind)">{{ tool.label }}</Button></div></div>
      </Tabs>
      <div v-if="w.currentRuns.length" aria-label="设备执行状态" class="max-h-40 space-y-2 overflow-auto border-t px-4 py-3"><div v-for="result in w.currentRuns" :key="result.device_serial" class="flex flex-wrap items-center gap-2 text-xs"><span class="min-w-0 break-all">{{ w.deviceName(result.device_serial) }}</span><Badge variant="outline" class="ml-auto text-[10px]">{{ taskStatus[result.status] }}</Badge><Button v-if="result.status === 'running' || result.status === 'stopping'" variant="ghost" size="icon-sm" :aria-label="`终止 ${w.deviceName(result.device_serial)}`" @click="w.stop(result.id)"><Square class="size-3" /></Button><p v-if="result.error_msg" class="w-full text-destructive">{{ result.error_msg }}</p></div></div>
    </template>
    <div v-else class="flex flex-1 flex-col items-center justify-center gap-4 text-muted-foreground"><FileText class="size-9 opacity-40" /><p class="text-sm">选择或新建任务</p><Button size="sm" variant="outline" @click="w.openForm(false)">新建任务</Button></div>
  </section>
</template>
<style scoped>
.script-markdown { font-size: 14px; line-height: 1.8; overflow-wrap: anywhere; }
.script-markdown :deep(h1) { font-size: 24px; font-weight: 600; margin: 0 0 24px; }
.script-markdown :deep(h2) { font-size: 18px; font-weight: 600; margin: 24px 0 12px; }
.script-markdown :deep(h3) { font-weight: 600; margin: 20px 0 8px; }
.script-markdown :deep(p) { margin: 12px 0; }
.script-markdown :deep(ul) { list-style: disc; padding-left: 24px; }
.script-markdown :deep(ol) { list-style: decimal; padding-left: 24px; }
.script-markdown :deep(li) { margin: 8px 0; }
.script-markdown :deep(img) { max-width: 100%; max-height: 240px; object-fit: contain; border: 1px solid var(--border); border-radius: 8px; margin: 12px 0; }
.script-markdown :deep(pre) { overflow-x: auto; background: var(--muted); padding: 16px; border-radius: 8px; }
.script-markdown :deep(code) { font-size: 12px; background: var(--muted); padding: 2px 4px; border-radius: 3px; }
.script-markdown :deep(table) { display: block; max-width: 100%; overflow-x: auto; border-collapse: collapse; margin: 16px 0; }
.script-markdown :deep(th), .script-markdown :deep(td) { border: 1px solid var(--border); padding: 6px 12px; }
.script-markdown :deep(blockquote) { border-left: 3px solid var(--border); padding-left: 16px; color: var(--muted-foreground); }
.script-markdown :deep(a) { color: var(--primary); text-decoration: underline; }
.script-markdown :deep(.template-unavailable) { color: var(--destructive); }
</style>
