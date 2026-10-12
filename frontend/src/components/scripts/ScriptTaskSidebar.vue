<script setup lang="ts">
import { ChevronRight, ListTodo, Plus, Search, Settings2, Smartphone, Trash2 } from 'lucide-vue-next'
import { Button } from '@/components/ui/button'
import { Badge } from '@/components/ui/badge'
import { Input } from '@/components/ui/input'
import { dateTime, taskStatus } from '@/scripts/document'
import type { ScriptsWorkbench } from '@/scripts/useScriptsWorkbench'
defineProps<{ w: ScriptsWorkbench }>()
</script>
<template>
  <aside aria-label="任务列表" class="script-pane flex min-w-0 flex-col bg-muted/20 lg:border-r" :class="w.mobilePanel === 'tasks' ? '' : 'hidden lg:flex'">
    <div class="flex items-center justify-between px-4 pt-4 pb-3"><h2 class="flex items-center gap-2 text-sm font-semibold"><ListTodo class="size-4" />任务</h2><Button size="sm" variant="outline" @click="w.openForm(false)"><Plus class="size-3.5" />新建</Button></div>
    <div class="relative mx-3 mb-3"><Search class="absolute top-2.5 left-3 size-3.5 text-muted-foreground" /><Input v-model="w.taskSearch" aria-label="搜索任务" placeholder="搜索任务" class="h-9 pl-9 text-xs" /></div>
    <div class="min-h-0 flex-1 space-y-2 overflow-auto px-3 pb-3">
      <p v-if="w.loading" role="status" class="py-8 text-center text-xs text-muted-foreground">正在加载任务…</p>
      <div v-else-if="w.loadError" role="alert" class="space-y-3 py-6 text-center text-xs"><p class="text-destructive">{{ w.loadError }}</p><Button size="sm" variant="outline" @click="w.loadScripts">重试</Button></div>
      <p v-else-if="!w.filteredScripts.length" role="status" class="py-8 text-center text-xs text-muted-foreground">{{ w.scripts.length ? '没有匹配的任务' : '尚未创建任务' }}</p>
      <Button v-for="task in w.filteredScripts" :key="task.id" variant="ghost" :aria-label="`选择任务 ${task.name}`" :aria-pressed="task.id === w.currentId" class="h-auto w-full items-start justify-start rounded-lg border p-3 text-left whitespace-normal" :class="task.id === w.currentId ? 'border-primary/30 bg-primary/5 hover:bg-primary/10' : 'border-transparent hover:bg-muted'" @click="w.selectTask(task.id)">
        <div class="w-full space-y-2.5">
          <div class="flex items-start justify-between gap-2"><span class="break-words text-sm font-medium">{{ task.name }}</span><ChevronRight v-if="task.id === w.currentId" class="mt-0.5 size-3.5 shrink-0 text-primary" /></div>
          <div class="space-y-1"><p v-if="task.needs_device_configuration" class="text-xs text-amber-600 dark:text-amber-400">待配置设备</p><div v-for="serial in task.target_devices" :key="serial" class="flex min-w-0 items-start gap-1.5 text-xs font-normal text-muted-foreground"><Smartphone class="mt-0.5 size-3 shrink-0" /><span class="min-w-0 break-all">{{ w.deviceName(serial) }}<small v-if="w.deviceName(serial) !== serial" class="mt-0.5 block text-[10px]">{{ serial }}</small></span></div></div>
          <dl class="grid grid-cols-2 gap-x-3 gap-y-1 text-[11px] font-normal text-muted-foreground">
            <div class="flex justify-between gap-1"><dt>版本</dt><dd class="break-all">{{ task.extracted_version }}</dd></div><div class="flex justify-between"><dt>超时</dt><dd>{{ task.timeout_sec }} 秒</dd></div>
            <div class="col-span-2 flex justify-between gap-2"><dt class="shrink-0">定时</dt><dd class="break-all text-right">{{ task.schedule_cron || '手动' }}</dd></div>
            <div class="col-span-2 flex justify-between gap-2"><dt class="shrink-0">下次执行</dt><dd>{{ dateTime(w.scheduleFor(task.id)?.next_run_at ?? 0) }}</dd></div>
            <div class="col-span-2 flex justify-between"><dt>最近结果</dt><dd>{{ taskStatus[w.scheduleFor(task.id)?.last_status ?? task.last_run_status] ?? '—' }}</dd></div>
            <div class="col-span-2 flex justify-between gap-2"><dt>Python 版本</dt><dd class="break-all">{{ task.compiled_version || '未生成' }}</dd></div>
          </dl>
          <Badge :variant="task.is_enabled === 1 ? 'secondary' : 'outline'" class="text-[10px]">{{ task.is_enabled === 1 ? '已启用' : '已停用' }}</Badge>
        </div>
      </Button>
    </div>
    <div class="flex gap-1 border-t px-3 py-2"><Button variant="ghost" class="flex-1 justify-start text-xs" :disabled="!w.current || !!w.busy" @click="w.openForm(true)"><Settings2 class="size-3.5" />任务配置</Button><Button variant="ghost" size="sm" class="text-xs" :disabled="!w.current || !!w.busy || w.dirty || (w.current.needs_device_configuration && w.current.is_enabled !== 1)" @click="w.toggleEnabled">{{ w.current?.is_enabled === 1 ? '停用' : '启用' }}</Button><Button variant="ghost" size="icon-sm" aria-label="删除当前任务" :disabled="!w.current || !!w.busy" @click="w.deleteOpen = true"><Trash2 class="size-3.5" /></Button></div>
  </aside>
</template>
