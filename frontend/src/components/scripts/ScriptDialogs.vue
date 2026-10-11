<script setup lang="ts">
import { computed } from 'vue'
import { Search, Square } from 'lucide-vue-next'
import { Button } from '@/components/ui/button'
import { Input } from '@/components/ui/input'
import { Badge } from '@/components/ui/badge'
import { Checkbox } from '@/components/ui/checkbox'
import { Dialog, DialogContent, DialogDescription, DialogFooter, DialogHeader, DialogTitle } from '@/components/ui/dialog'
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from '@/components/ui/select'
import ModelConfigDialog from '@/components/ai/ModelConfigDialog.vue'
import { templateImageUrl } from '@/api/templates'
import { hasDocumentTargets, taskStatus } from '@/scripts/document'
import type { ScriptsWorkbench } from '@/scripts/useScriptsWorkbench'
const props = defineProps<{ w: ScriptsWorkbench }>()
const formHasLegacy = computed(() => hasDocumentTargets(props.w.formContent))
</script>
<template>
  <Dialog v-model:open="w.formOpen"><DialogContent class="max-h-[90dvh] overflow-auto sm:max-w-md" @interact-outside="event => { if (w.formBusy) event.preventDefault() }"><DialogHeader><DialogTitle>{{ w.editingId === null ? '新建任务' : '任务配置' }}</DialogTitle><DialogDescription>填写标题并选择目标设备。</DialogDescription></DialogHeader>
    <form class="space-y-4" @submit.prevent="w.submitForm">
      <div class="space-y-2"><label for="script-title" class="text-xs font-medium">任务标题</label><Input id="script-title" v-model="w.formName" :disabled="w.formBusy" placeholder="填写任务标题" /></div>
      <fieldset class="space-y-2" :disabled="w.formBusy"><legend class="mb-2 text-xs font-medium">目标设备 · 已选 {{ w.formDevices.length }} 台</legend><div class="relative"><Search class="absolute top-3 left-3 size-3.5 text-muted-foreground" /><Input v-model="w.formSearch" aria-label="搜索设备" placeholder="搜索设备" class="pl-9 text-xs" /></div>
        <div v-if="w.deviceError" role="alert" class="space-y-2 text-xs text-destructive"><p>{{ w.deviceError }}</p><Button variant="outline" size="sm" @click="w.loadDevices">重试</Button></div>
        <div v-else class="max-h-56 space-y-2 overflow-auto rounded-lg border p-2"><label v-for="device in w.filteredDevices" :key="device.serial" class="flex cursor-pointer items-center gap-3 rounded-md p-2 hover:bg-muted"><Checkbox :model-value="w.formDevices.includes(device.serial)" :disabled="w.formBusy || device.state === 'missing'" :aria-label="`选择设备 ${w.deviceName(device.serial)}`" @update:model-value="w.toggleDevice(device.serial)" /><span class="min-w-0 flex-1 break-all text-xs"><span class="block font-medium">{{ w.deviceName(device.serial) }}</span><span v-if="w.deviceName(device.serial) !== device.serial" class="mt-0.5 block text-[10px] text-muted-foreground">{{ device.serial }}</span></span><Badge variant="outline" class="text-[10px]">{{ device.state === 'device' ? '在线' : device.state === 'missing' ? '已移除' : '离线' }}</Badge><Button v-if="device.state === 'missing'" variant="ghost" size="sm" class="text-xs" @click.prevent="w.toggleDevice(device.serial)">移除</Button></label><p v-if="!w.filteredDevices.length" class="py-5 text-center text-xs text-muted-foreground">{{ w.devices.length ? '没有匹配的设备' : '请先添加目标设备' }}</p></div>
      </fieldset>
      <label v-if="formHasLegacy" class="flex items-start gap-2 text-xs"><Checkbox v-model="w.formRemoveLegacy" :disabled="w.formBusy" class="mt-0.5" />同时移除文档中的旧设备配置</label>
      <p v-if="w.formError" role="alert" class="text-xs text-destructive">{{ w.formError }}</p><DialogFooter><Button type="button" variant="ghost" :disabled="w.formBusy" @click="w.formOpen = false">取消</Button><Button type="submit" :disabled="w.formBusy">{{ w.formBusy ? '保存中…' : w.editingId === null ? '创建任务' : '保存配置' }}</Button></DialogFooter>
    </form>
  </DialogContent></Dialog>
  <Dialog v-model:open="w.templateOpen"><DialogContent class="max-h-[85dvh] overflow-auto sm:max-w-2xl"><DialogHeader><DialogTitle>模板库</DialogTitle><DialogDescription>已选设备与公共模板。</DialogDescription></DialogHeader>
    <div class="flex gap-2"><div class="relative flex-1"><Search class="absolute top-3 left-3 size-3.5 text-muted-foreground" /><Input v-model="w.templateSearch" aria-label="搜索模板" placeholder="搜索模板" class="pl-9 text-xs" /></div><Select v-model="w.templateScope"><SelectTrigger class="w-32" aria-label="模板归属"><SelectValue /></SelectTrigger><SelectContent><SelectItem value="all">全部归属</SelectItem><SelectItem value="global">公共</SelectItem><SelectItem v-for="serial in w.current?.target_devices" :key="serial" :value="serial">{{ w.deviceName(serial) }}</SelectItem></SelectContent></Select></div>
    <p v-if="w.templateLoading" role="status" class="py-8 text-center text-sm text-muted-foreground">正在加载模板…</p><div v-else-if="w.templateError" role="alert" class="space-y-3 py-8 text-center text-sm"><p class="text-destructive">{{ w.templateError }}</p><Button variant="outline" size="sm" @click="w.loadTemplates">重试</Button></div>
    <template v-else><div class="grid grid-cols-2 gap-3 sm:grid-cols-3"><Button v-for="asset in w.visibleTemplates" :key="asset.id" variant="outline" :disabled="!w.current" :aria-label="`插入模板 ${asset.name}`" class="h-auto flex-col items-stretch gap-0 overflow-hidden p-0" @click="w.insertTemplate(asset)"><div class="flex h-24 items-center justify-center bg-muted/40 p-3"><img :src="templateImageUrl(asset.id, asset.updated_at)" :alt="asset.name" class="max-h-full max-w-full object-contain" loading="lazy" /></div><div class="space-y-1.5 p-3 text-left"><p class="text-xs font-medium break-words whitespace-normal">{{ asset.name }}</p><div class="flex items-center justify-between gap-1"><Badge variant="secondary" class="min-w-0 truncate text-[10px]">{{ asset.serial === 'global' ? '公共' : w.deviceName(asset.serial) }}</Badge><span class="text-[10px] text-muted-foreground">{{ asset.threshold }}</span></div></div></Button></div><p v-if="!w.visibleTemplates.length" role="status" class="py-8 text-center text-sm text-muted-foreground">没有匹配的模板</p></template>
  </DialogContent></Dialog>
  <Dialog v-model:open="w.taskManagerOpen"><DialogContent class="max-h-[85dvh] overflow-auto sm:max-w-xl"><DialogHeader><DialogTitle>运行任务</DialogTitle><DialogDescription>各设备正在运行的任务。</DialogDescription></DialogHeader><div v-for="task in w.activeTasks" :key="task.id" class="flex items-center gap-3 rounded-lg border p-3"><div class="min-w-0 flex-1 text-xs"><p class="font-medium">{{ task.script_name }}</p><p class="mt-1 break-all text-muted-foreground">{{ w.deviceName(task.device_serial) }}</p></div><Badge variant="outline">{{ taskStatus[task.status] }}</Badge><Button variant="ghost" size="icon-sm" :aria-label="`终止任务 ${task.script_name} ${w.deviceName(task.device_serial)}`" @click="w.stop(task.id)"><Square class="size-3.5" /></Button></div><p v-if="!w.activeTasks.length" class="py-8 text-center text-xs text-muted-foreground">没有运行中的任务</p></DialogContent></Dialog>
  <Dialog v-model:open="w.deleteOpen"><DialogContent class="sm:max-w-md"><DialogHeader><DialogTitle>删除任务</DialogTitle><DialogDescription>确定删除「{{ w.current?.name }}」？正在运行的实例仍可从运行任务中终止。</DialogDescription></DialogHeader><DialogFooter><Button variant="ghost" @click="w.deleteOpen = false">取消</Button><Button variant="destructive" :disabled="!!w.busy" @click="w.deleteCurrent">删除</Button></DialogFooter></DialogContent></Dialog>
  <ModelConfigDialog v-model:open="w.modelConfigOpen" />
</template>
