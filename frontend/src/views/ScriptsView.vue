<script setup lang="ts">
import { reactive } from 'vue'
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
</script>
<template>
  <div class="flex min-h-screen flex-col bg-background">
    <header class="flex flex-wrap items-center justify-between gap-2 border-b px-4 py-2.5"><div class="flex items-center gap-3"><Button variant="ghost" size="sm" as-child><RouterLink to="/"><ArrowLeft class="size-4" /><span class="hidden sm:inline">返回设备</span></RouterLink></Button><div class="h-4 w-px bg-border" /><FileCode2 class="size-4 text-primary" /><h1 class="text-sm font-semibold">脚本管理</h1></div><Button variant="outline" size="sm" class="text-xs" @click="w.taskManagerOpen = true"><Activity class="size-3.5" />运行任务<span v-if="w.activeTasks.length">{{ w.activeTasks.length }}</span></Button></header>
    <Tabs v-model="w.mobilePanel" class="border-b px-3 py-2 lg:hidden"><TabsList class="grid w-full grid-cols-3"><TabsTrigger value="tasks">任务</TabsTrigger><TabsTrigger value="editor">文档</TabsTrigger><TabsTrigger value="assistant">助手</TabsTrigger></TabsList></Tabs>
    <div class="grid min-h-0 flex-1 lg:grid-cols-[258px_minmax(0,1fr)_282px] xl:grid-cols-[278px_minmax(0,1fr)_310px]"><ScriptTaskSidebar :w="w" /><ScriptEditor :w="w" /><ScriptAssistant :w="w" /></div>
    <ScriptDialogs :w="w" />
  </div>
</template>
<style>
.script-pane { height: max(480px, calc(100dvh - 54px)); }
@media (max-width: 1023px) { .script-pane { height: max(480px, calc(100dvh - 108px)); } }
</style>
