<script setup lang="ts">
import { computed } from 'vue'
import { Bot, Check, ChevronRight, Code2, FileCode2, FileText, Image, ListTodo, Play, Plus, Search, Send, Settings2, Smartphone, Square } from 'lucide-vue-next'
import { Badge } from '@/components/ui/badge'
import { Button } from '@/components/ui/button'
import { Checkbox } from '@/components/ui/checkbox'
import { Dialog, DialogContent, DialogDescription, DialogFooter, DialogHeader, DialogTitle } from '@/components/ui/dialog'
import { Input } from '@/components/ui/input'
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from '@/components/ui/select'
import { Tabs, TabsContent, TabsList, TabsTrigger } from '@/components/ui/tabs'
import { deviceName, templateOwner, templates } from './data'
import { useWorkbench } from './useWorkbench'

const { current, content, dirty, tab, taskSearch, filteredTasks, notice, formOpen, editing, formName, formDevices, formError, formSearch, filteredDevices, templateOpen, templateSearch, templateScope, visibleTemplates, messages, prompt, proposals, currentRuns, hasRunning, mobilePanel, selectTask, openForm, toggleDevice, submitForm, save, compile, insertTemplate, sendPrompt, applyProposal, run, stop } = useWorkbench()

// 此处仅展示本原型示例的标题、段落、步骤与模板引用；完整 Markdown 渲染留给正式实现。
const previewLines = computed(() => content.value.replace(/^---\n[\s\S]*?\n---\n?/, '').split('\n').filter(line => line.trim()).map(line => {
  const image = line.match(/^!\[([^\]]*)\]\(template:\/\/([^?\s)]+)/)
  if (image) return { kind: 'image', text: image[1], asset: templates.find(item => item.id === image[2]) }
  if (line.startsWith('## ')) return { kind: 'h2', text: line.slice(3) }
  if (line.startsWith('# ')) return { kind: 'h1', text: line.slice(2) }
  if (line.startsWith('- ')) return { kind: 'step', text: line.slice(2) }
  return { kind: 'text', text: line }
}))
</script>

<template>
  <div class="workbench overflow-hidden rounded-xl border bg-background shadow-sm">
    <div class="flex items-center justify-between gap-3 border-b px-4 py-3">
      <div class="flex items-center gap-2"><FileCode2 class="size-4 text-primary" /><h1 class="text-sm font-semibold">脚本管理</h1></div>
    </div>
    <Tabs v-model="mobilePanel" class="border-b px-3 py-2 lg:hidden">
      <TabsList class="grid w-full grid-cols-3"><TabsTrigger value="tasks">任务</TabsTrigger><TabsTrigger value="editor">文档</TabsTrigger><TabsTrigger value="assistant">助手</TabsTrigger></TabsList>
    </Tabs>
    <div class="grid lg:grid-cols-[258px_minmax(0,1fr)_282px]">
      <aside aria-label="任务列表" class="pane flex flex-col bg-muted/20 lg:border-r" :class="mobilePanel === 'tasks' ? '' : 'hidden lg:flex'">
        <div class="flex items-center justify-between px-4 pt-4 pb-3"><h2 class="flex items-center gap-2 text-sm font-semibold"><ListTodo class="size-4" />任务</h2><Button size="sm" variant="outline" @click="openForm(false)"><Plus class="size-3.5" />新建</Button></div>
        <div class="relative mx-3 mb-3"><Search class="absolute top-2.5 left-3 size-3.5 text-muted-foreground" /><Input v-model="taskSearch" aria-label="搜索任务" placeholder="搜索任务" class="h-9 pl-9 text-xs" /></div>
        <div class="min-h-0 flex-1 space-y-2 overflow-auto px-3 pb-3">
          <p v-if="!filteredTasks.length" role="status" class="py-8 text-center text-xs text-muted-foreground">没有匹配的任务</p>
          <Button v-for="task in filteredTasks" :key="task.id" variant="ghost" :aria-label="`选择任务 ${task.name}`" :aria-pressed="task.id === current.id" class="h-auto w-full items-start justify-start rounded-lg border p-3 text-left whitespace-normal" :class="task.id === current.id ? 'border-primary/30 bg-primary/5 hover:bg-primary/10' : 'border-transparent hover:bg-muted'" @click="selectTask(task.id)">
            <div class="w-full space-y-2.5">
              <div class="flex items-start justify-between gap-2"><span class="text-sm font-medium">{{ task.name }}</span><ChevronRight v-if="task.id === current.id" class="mt-0.5 size-3.5 shrink-0 text-primary" /></div>
              <div class="space-y-1"><div v-for="serial in task.target_devices" :key="serial" class="flex items-center gap-1.5 text-xs text-muted-foreground"><Smartphone class="size-3" /><span>{{ deviceName(serial) }}</span><span class="ml-auto text-[10px]">{{ serial }}</span></div></div>
              <dl class="grid grid-cols-2 gap-x-3 gap-y-1 text-[11px] font-normal text-muted-foreground">
                <div class="flex justify-between"><dt>版本</dt><dd>{{ task.version }}</dd></div><div class="flex justify-between"><dt>超时</dt><dd>{{ task.timeout }} 秒</dd></div>
                <div class="col-span-2 flex justify-between"><dt>定时</dt><dd>{{ task.schedule }}</dd></div>
                <div class="col-span-2 flex justify-between"><dt>下次执行</dt><dd>{{ task.nextRun }}</dd></div>
                <div class="col-span-2 flex justify-between"><dt>最近结果</dt><dd>{{ task.lastResult }}</dd></div>
              </dl>
              <div class="flex items-center justify-between"><Badge :variant="task.enabled ? 'secondary' : 'outline'" class="text-[10px]">{{ task.enabled ? '已启用' : '已停用' }}</Badge><span class="text-[10px] font-normal text-muted-foreground">{{ task.compiledPython ? 'Python 已生成' : 'Python 未生成' }}</span></div>
            </div>
          </Button>
        </div>
        <div class="border-t px-3 py-2"><Button variant="ghost" class="w-full justify-start text-xs" @click="openForm(true)"><Settings2 class="size-3.5" />编辑任务配置</Button></div>
      </aside>

      <section aria-label="文档工作区" class="pane flex min-w-0 flex-col" :class="mobilePanel === 'editor' ? '' : 'hidden lg:flex'">
        <div class="flex flex-wrap items-center justify-between gap-2 border-b px-4 py-3">
          <div class="flex min-w-0 items-center gap-2"><h2 class="truncate text-sm font-medium">{{ current.name }}</h2><span v-if="dirty" class="shrink-0 text-[10px] text-muted-foreground">未保存</span></div>
          <Button size="sm" variant="outline" :disabled="!dirty" @click="save"><Check class="size-3.5" />保存</Button>
          <div class="flex w-full gap-1.5"><Button size="sm" variant="ghost" class="text-xs" @click="templateOpen = true"><Image class="size-3.5" />模板库</Button><Button size="sm" variant="ghost" class="text-xs" @click="compile"><Code2 class="size-3.5" />生成 Python</Button><Button size="sm" class="ml-auto text-xs" :disabled="hasRunning" @click="run"><Play class="size-3.5" />运行</Button></div>
        </div>
        <p v-if="notice" role="status" class="border-b bg-muted/30 px-4 py-2 text-xs">{{ notice }}</p>
        <Tabs v-model="tab" class="flex min-h-0 flex-1 flex-col gap-0">
          <TabsContent value="source" class="m-0 min-h-0 flex-1 data-[state=active]:flex"><textarea v-model="content" aria-label="Markdown 源码" spellcheck="false" class="source-editor h-full min-h-96 w-full resize-none border-0 bg-background p-5 font-mono text-xs leading-7 outline-none focus-visible:ring-2 focus-visible:ring-inset focus-visible:ring-ring" /></TabsContent>
          <TabsContent value="preview" class="m-0 min-h-0 flex-1 overflow-auto">
            <article class="mx-auto max-w-xl space-y-4 p-6">
              <template v-for="(line, index) in previewLines" :key="index">
                <h1 v-if="line.kind === 'h1'" class="mb-6 text-xl font-semibold tracking-tight">{{ line.text }}</h1>
                <h2 v-else-if="line.kind === 'h2'" class="pt-4 text-sm font-semibold">{{ line.text }}</h2>
                <div v-else-if="line.kind === 'step'" class="flex items-start gap-3 text-sm"><span class="mt-2 size-1.5 shrink-0 rounded-full bg-primary/70" />{{ line.text }}</div>
                <div v-else-if="line.kind === 'image'" class="flex items-center gap-3 rounded-lg border bg-muted/20 p-4"><div class="flex size-12 items-center justify-center rounded-lg border bg-background"><Check class="size-6 text-primary" /></div><div><p class="text-sm font-medium">{{ line.text }}</p><p class="mt-1 text-xs text-muted-foreground">{{ line.asset ? templateOwner(line.asset) : '模板不存在' }}</p></div></div>
                <p v-else class="text-sm leading-7">{{ line.text }}</p>
              </template>
            </article>
          </TabsContent>
          <TabsContent value="python" class="m-0 min-h-0 flex-1 overflow-auto">
            <pre v-if="current.compiledPython" class="p-5 font-mono text-xs leading-7"><code>{{ current.compiledPython }}</code></pre>
            <div v-else class="flex h-full min-h-80 flex-col items-center justify-center gap-4 text-muted-foreground"><FileCode2 class="size-9 opacity-40" /><p class="text-sm">尚未生成 Python</p><Button size="sm" variant="outline" @click="compile">生成 Python</Button></div>
          </TabsContent>
          <div class="border-t bg-muted/15 px-3 py-2"><TabsList class="h-8 bg-transparent p-0" aria-label="文档视图"><TabsTrigger value="source" class="gap-1.5 text-xs"><Code2 class="size-3.5" />源码</TabsTrigger><TabsTrigger value="preview" class="gap-1.5 text-xs"><FileText class="size-3.5" />预览</TabsTrigger><TabsTrigger value="python" class="gap-1.5 text-xs"><FileCode2 class="size-3.5" />Python</TabsTrigger></TabsList></div>
        </Tabs>
        <div v-if="currentRuns.length" aria-label="设备执行状态" class="space-y-2 border-t px-4 py-3"><div v-for="result in currentRuns" :key="result.serial" class="flex items-center justify-between gap-2 text-xs"><span>{{ deviceName(result.serial) }}</span><Badge variant="outline" class="ml-auto text-[10px]">{{ result.status }}</Badge><Button v-if="result.status === '运行中'" variant="ghost" size="icon-sm" :aria-label="`终止 ${deviceName(result.serial)}`" @click="stop(result.serial)"><Square class="size-3" /></Button></div></div>
      </section>

      <aside aria-label="文档助手" class="pane flex flex-col bg-muted/10 lg:border-l" :class="mobilePanel === 'assistant' ? '' : 'hidden lg:flex'">
        <div class="flex items-center gap-2 border-b px-4 py-4"><div class="flex size-7 items-center justify-center rounded-lg bg-primary/10"><Bot class="size-4 text-primary" /></div><h2 class="text-sm font-semibold">文档助手</h2></div>
        <div class="min-h-0 flex-1 space-y-5 overflow-auto p-4"><div v-for="(message, index) in messages" :key="index" :class="message.role === 'user' ? 'ml-6 rounded-lg bg-primary/10 p-3' : 'mr-2'" class="text-sm leading-6"><span v-if="message.role === 'assistant'" class="mb-1 block text-[10px] font-medium text-muted-foreground">助手</span>{{ message.content }}</div><Button v-if="proposals[current.id]" size="sm" variant="outline" class="w-full" @click="applyProposal">应用到文档</Button></div>
        <form class="space-y-2 border-t p-3" @submit.prevent="sendPrompt"><label for="assistant-prompt" class="sr-only">文档修改要求</label><Input id="assistant-prompt" v-model="prompt" placeholder="描述要调整的步骤…" class="text-xs" /><Button type="submit" size="sm" class="w-full text-xs" :disabled="!prompt.trim()"><Send class="size-3.5" />发送</Button></form>
      </aside>
    </div>

    <Dialog v-model:open="formOpen"><DialogContent class="max-h-[90dvh] overflow-auto sm:max-w-md"><DialogHeader><DialogTitle>{{ editing ? '任务配置' : '新建任务' }}</DialogTitle><DialogDescription>填写标题并选择目标设备。</DialogDescription></DialogHeader><form class="space-y-5" @submit.prevent="submitForm"><div class="space-y-2"><label for="task-title" class="text-sm font-medium">任务标题</label><Input id="task-title" v-model="formName" placeholder="输入任务标题" /></div><div class="space-y-3"><div class="flex items-center justify-between"><h3 class="text-sm font-medium">目标设备</h3><span class="text-xs text-muted-foreground">已选 {{ formDevices.length }} 台</span></div><Input v-model="formSearch" aria-label="搜索设备" placeholder="搜索设备" /><div class="space-y-2"><label v-for="device in filteredDevices" :key="device.serial" :for="`pick-${device.serial}`" class="flex cursor-pointer items-center gap-3 rounded-lg border p-3" :class="formDevices.includes(device.serial) ? 'border-primary/40 bg-primary/5' : ''"><Checkbox :id="`pick-${device.serial}`" :model-value="formDevices.includes(device.serial)" @update:model-value="checked => toggleDevice(device.serial, checked === true)" /><div class="min-w-0 flex-1"><div class="text-sm font-medium">{{ device.alias }}</div><div class="mt-0.5 text-[11px] text-muted-foreground">{{ device.serial }}</div></div><Badge variant="outline" class="text-[10px]">{{ device.state === 'device' ? '在线' : '离线' }}</Badge></label><p v-if="!filteredDevices.length" class="py-4 text-center text-xs text-muted-foreground">没有匹配的设备</p></div></div><p v-if="formError" role="alert" class="text-sm text-destructive">{{ formError }}</p><DialogFooter><Button type="button" variant="outline" @click="formOpen = false">取消</Button><Button type="submit">{{ editing ? '保存配置' : '创建任务' }}</Button></DialogFooter></form></DialogContent></Dialog>

    <Dialog v-model:open="templateOpen"><DialogContent class="max-h-[90dvh] overflow-auto sm:max-w-xl"><DialogHeader><DialogTitle>模板库</DialogTitle><DialogDescription>{{ current.target_devices.map(deviceName).join('、') }}及公共模板</DialogDescription></DialogHeader><div class="flex gap-2"><Input v-model="templateSearch" aria-label="搜索模板" placeholder="搜索模板" class="flex-1" /><Select v-model="templateScope"><SelectTrigger class="w-36" aria-label="模板归属"><SelectValue /></SelectTrigger><SelectContent><SelectItem value="all">全部</SelectItem><SelectItem value="global">公共</SelectItem><SelectItem v-for="serial in current.target_devices" :key="serial" :value="serial">{{ deviceName(serial) }}</SelectItem></SelectContent></Select></div><div class="grid grid-cols-2 gap-3"><Button v-for="asset in visibleTemplates" :key="asset.id" variant="outline" class="h-auto flex-col items-stretch gap-0 overflow-hidden p-0" :aria-label="`插入 ${templateOwner(asset)} ${asset.name}`" @click="insertTemplate(asset.id)"><div class="flex h-24 items-center justify-center bg-muted/40"><div class="flex items-center gap-2 rounded-lg border bg-background px-4 py-2 text-primary"><Check v-if="asset.id === 'common-confirm'" class="size-4" /><ChevronRight v-else-if="asset.id === 'common-back'" class="size-4 rotate-180" /><Smartphone v-else class="size-4" /><span aria-hidden="true" class="h-1.5 w-8 rounded-full bg-primary/20" /></div></div><div class="space-y-1.5 p-3 text-left"><p class="text-xs font-medium">{{ asset.name }}</p><div class="flex items-center justify-between"><Badge variant="secondary" class="text-[10px]">{{ templateOwner(asset) }}</Badge><span class="text-[10px] text-muted-foreground">{{ asset.threshold }}</span></div></div></Button></div><p v-if="!visibleTemplates.length" role="status" class="py-8 text-center text-sm text-muted-foreground">没有匹配的模板</p></DialogContent></Dialog>
  </div>
</template>

<style scoped>
.workbench { width: calc(100vw - 48px); max-width: 1520px; position: relative; left: 50%; transform: translateX(-50%); }
.pane { height: max(460px, calc(100dvh - 174px)); }
@media (max-width: 1023px) {
  .workbench { width: 100%; }
  .pane { height: max(600px, calc(100dvh - 240px)); }
}
</style>
