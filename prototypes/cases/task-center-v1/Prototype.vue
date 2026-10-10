<script setup lang="ts">
import { computed, ref } from 'vue'
import { Check, Plus, Search, Smartphone } from 'lucide-vue-next'
import { Badge } from '@/components/ui/badge'
import { Button } from '@/components/ui/button'
import { Card, CardContent } from '@/components/ui/card'
import { Dialog, DialogContent, DialogDescription, DialogFooter, DialogHeader, DialogTitle } from '@/components/ui/dialog'
import { Input } from '@/components/ui/input'
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from '@/components/ui/select'

type TaskStatus = 'pending' | 'completed'
interface Task {
  id: number
  name: string
  device: string
  status: TaskStatus
}

const tasks = ref<Task[]>([
  { id: 1, name: '每日截图', device: '测试设备 A', status: 'pending' },
  { id: 2, name: '清理临时文件', device: '测试设备 B', status: 'pending' },
  { id: 3, name: '设备状态巡检', device: '测试设备 A', status: 'completed' },
])
const keyword = ref('')
const status = ref('all')
const dialogOpen = ref(false)
const name = ref('')
const device = ref('测试设备 A')
const formError = ref('')
const filteredTasks = computed(() => tasks.value.filter(task =>
  (status.value === 'all' || task.status === status.value)
  && `${task.name} ${task.device}`.toLowerCase().includes(keyword.value.trim().toLowerCase()),
))

function openCreate() {
  name.value = ''
  device.value = '测试设备 A'
  formError.value = ''
  dialogOpen.value = true
}

function createTask() {
  if (!name.value.trim()) {
    formError.value = '请输入任务名称。'
    return
  }
  tasks.value.unshift({
    id: Math.max(0, ...tasks.value.map(task => task.id)) + 1,
    name: name.value.trim(),
    device: device.value,
    status: 'pending',
  })
  keyword.value = ''
  status.value = 'all'
  dialogOpen.value = false
}

function clearFilters() {
  keyword.value = ''
  status.value = 'all'
}
</script>

<template>
  <section class="space-y-6">
    <div class="flex items-center justify-between gap-4">
      <h1 class="text-2xl font-semibold tracking-tight">任务管理</h1>
      <Button @click="openCreate"><Plus class="h-4 w-4" />新建任务</Button>
    </div>

    <div class="flex flex-col gap-3 sm:flex-row">
      <div class="relative flex-1">
        <Search class="pointer-events-none absolute top-2.5 left-3 h-4 w-4 text-muted-foreground" />
        <Input v-model="keyword" class="pl-9" placeholder="搜索任务或设备" aria-label="搜索任务或设备" />
      </div>
      <Select v-model="status">
        <SelectTrigger class="w-full sm:w-40" aria-label="任务状态"><SelectValue /></SelectTrigger>
        <SelectContent>
          <SelectItem value="all">全部状态</SelectItem>
          <SelectItem value="pending">待执行</SelectItem>
          <SelectItem value="completed">已完成</SelectItem>
        </SelectContent>
      </Select>
    </div>

    <div class="space-y-3">
      <Card v-for="task in filteredTasks" :key="task.id">
        <CardContent class="flex flex-wrap items-center gap-4 p-4 sm:flex-nowrap sm:p-5">
          <span class="flex h-10 w-10 shrink-0 items-center justify-center rounded-lg bg-muted">
            <Smartphone class="h-5 w-5 text-muted-foreground" />
          </span>
          <div class="min-w-0 flex-1">
            <h2 class="truncate font-medium">{{ task.name }}</h2>
            <p class="mt-1 text-sm text-muted-foreground">{{ task.device }}</p>
          </div>
          <Badge :variant="task.status === 'completed' ? 'secondary' : 'outline'">{{ task.status === 'completed' ? '已完成' : '待执行' }}</Badge>
          <Button v-if="task.status === 'pending'" variant="outline" size="sm" class="ml-auto" :aria-label="`完成 ${task.name}`" @click="task.status = 'completed'">
            <Check class="h-4 w-4" />完成
          </Button>
        </CardContent>
      </Card>
      <div v-if="!filteredTasks.length" role="status" class="rounded-xl border border-dashed px-4 py-12 text-center">
        <p class="text-sm text-muted-foreground">没有匹配的任务</p>
        <Button variant="ghost" class="mt-3" @click="clearFilters">清除筛选</Button>
      </div>
    </div>

    <Dialog v-model:open="dialogOpen">
      <DialogContent>
        <DialogHeader>
          <DialogTitle>新建任务</DialogTitle>
          <DialogDescription class="sr-only">填写任务名称并选择设备。</DialogDescription>
        </DialogHeader>
        <form id="create-task" class="space-y-4" @submit.prevent="createTask">
          <div class="space-y-2">
            <label for="task-name" class="text-sm font-medium">任务名称</label>
            <Input id="task-name" v-model="name" maxlength="40" :aria-invalid="Boolean(formError)" :aria-describedby="formError ? 'task-name-error' : undefined" @update:model-value="formError = ''" />
            <p v-if="formError" id="task-name-error" role="alert" class="text-sm text-destructive">{{ formError }}</p>
          </div>
          <div class="space-y-2">
            <label id="task-device-label" class="text-sm font-medium">设备</label>
            <Select v-model="device">
              <SelectTrigger class="w-full" aria-labelledby="task-device-label"><SelectValue /></SelectTrigger>
              <SelectContent>
                <SelectItem value="测试设备 A">测试设备 A</SelectItem>
                <SelectItem value="测试设备 B">测试设备 B</SelectItem>
              </SelectContent>
            </Select>
          </div>
        </form>
        <DialogFooter>
          <Button variant="outline" @click="dialogOpen = false">取消</Button>
          <Button type="submit" form="create-task">创建</Button>
        </DialogFooter>
      </DialogContent>
    </Dialog>
  </section>
</template>
