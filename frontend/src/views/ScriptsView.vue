<script setup lang="ts">
import { reactive } from 'vue'
import { useMediaQuery } from '@vueuse/core'
import { SplitterGroup, SplitterPanel, SplitterResizeHandle } from 'reka-ui'
import { Activity, ArrowLeft, FileCode2 } from 'lucide-vue-next'
import { RouterLink } from 'vue-router'
import { Button } from '@/components/ui/button'
import { Tabs, TabsList, TabsTrigger } from '@/components/ui/tabs'
import ScriptTaskSidebar from '@/components/scripts/ScriptTaskSidebar.vue'
import ScriptEditor from '@/components/scripts/ScriptEditor.vue'
import ScriptAssistant from '@/components/scripts/ScriptAssistant.vue'
import ScriptDialogs from '@/components/scripts/ScriptDialogs.vue'
import { useScriptsWorkbench } from '@/scripts/useScriptsWorkbench'
const w = reactive(useScriptsWorkbench())
const desktop = useMediaQuery('(min-width: 1024px)')
</script>
<template>
  <div class="flex min-h-screen flex-col bg-background">
    <header class="flex flex-wrap items-center justify-between gap-2 border-b px-4 py-2.5"><div class="flex items-center gap-3"><Button variant="ghost" size="sm" as-child><RouterLink to="/"><ArrowLeft class="size-4" /><span class="hidden sm:inline">返回设备</span></RouterLink></Button><div class="h-4 w-px bg-border" /><FileCode2 class="size-4 text-primary" /><h1 class="text-sm font-semibold">脚本管理</h1></div><Button variant="outline" size="sm" class="text-xs" @click="w.taskManagerOpen = true"><Activity class="size-3.5" />运行任务<span v-if="w.activeTasks.length">{{ w.activeTasks.length }}</span></Button></header>
    <Tabs v-model="w.mobilePanel" class="border-b px-3 py-2 lg:hidden"><TabsList class="grid w-full grid-cols-3"><TabsTrigger value="tasks">任务</TabsTrigger><TabsTrigger value="editor">文档</TabsTrigger><TabsTrigger value="assistant">AI助手</TabsTrigger></TabsList></Tabs>
    <SplitterGroup v-if="desktop" id="scripts-workbench" direction="horizontal" auto-save-id="scripts-workbench-widths" :keyboard-resize-by="2" class="min-h-0 flex-1">
      <SplitterPanel id="script-tasks" :order="0" size-unit="px" :default-size="278" :min-size="220" :max-size="480"><ScriptTaskSidebar :w="w" /></SplitterPanel>
      <SplitterResizeHandle aria-label="调整任务侧栏宽度" class="script-resizer" />
      <SplitterPanel id="script-editor" :order="1" size-unit="px" :min-size="320"><ScriptEditor :w="w" /></SplitterPanel>
      <SplitterResizeHandle aria-label="调整 AI助手侧栏宽度" class="script-resizer" />
      <SplitterPanel id="script-assistant" :order="2" size-unit="px" :default-size="310" :min-size="260" :max-size="720"><ScriptAssistant :w="w" /></SplitterPanel>
    </SplitterGroup>
    <div v-else class="grid min-h-0 flex-1"><ScriptTaskSidebar :w="w" /><ScriptEditor :w="w" /><ScriptAssistant :w="w" /></div>
    <ScriptDialogs :w="w" />
  </div>
</template>
<style>
.script-pane { height: max(480px, calc(100dvh - 54px)); }
.script-resizer { position: relative; width: 1px; flex-shrink: 0; background: var(--border); }
.script-resizer::after { content: ""; position: absolute; inset: 0 -3px; transition: background .15s; }
.script-resizer:hover::after, .script-resizer[data-state="drag"]::after { background: var(--primary); }
.script-resizer:focus-visible { outline: 2px solid var(--ring); outline-offset: -2px; }
@media (max-width: 1023px) { .script-pane { height: max(480px, calc(100dvh - 108px)); } }
</style>
