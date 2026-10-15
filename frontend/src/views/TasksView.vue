<script setup lang="ts">
import { computed, onMounted, onUnmounted, ref, watch } from 'vue'
import { RouterLink } from 'vue-router'
import {
  Activity,
  AlertCircle,
  ArrowLeft,
  CheckCircle2,
  Download,
  Loader2,
  RefreshCw,
  Search,
  Terminal,
  XCircle,
} from 'lucide-vue-next'
import { marked } from 'marked'
import DOMPurify from 'dompurify'
import { Button } from '@/components/ui/button'
import { Badge } from '@/components/ui/badge'
import { Input } from '@/components/ui/input'
import {
  Dialog,
  DialogContent,
  DialogDescription,
  DialogHeader,
  DialogTitle,
} from '@/components/ui/dialog'
import { Tabs, TabsContent, TabsList, TabsTrigger } from '@/components/ui/tabs'
import {
  fetchTaskRunDetail,
  fetchTaskRunLogs,
  fetchTaskRuns,
  type TaskRunItem,
  type TaskRunLogEntry,
} from '@/api/taskRuns'

// 列表查询状态
const loading = ref(false)
const listError = ref('')
const detailError = ref('')
const items = ref<TaskRunItem[]>([])
const total = ref(0)
const page = ref(1)
const limit = ref(20)
const statusFilter = ref('')
const deviceFilter = ref('')
const autoRefresh = ref(true)
let refreshTimer: number | null = null

// 详情弹窗状态
const detailOpen = ref(false)
const selectedRun = ref<TaskRunItem | null>(null)
const logsLoading = ref(false)
const logs = ref<TaskRunLogEntry[]>([])
const logStreamFilter = ref<'all' | 'stdout' | 'stderr'>('all')
const autoScroll = ref(true)
const logContainerRef = ref<HTMLDivElement | null>(null)
let logPollingTimer: number | null = null

const filteredLogs = computed(() => {
  if (logStreamFilter.value === 'all') {
    return logs.value
  }
  return logs.value.filter(e => e.stream === logStreamFilter.value)
})

const renderedMarkdown = computed(() => {
  if (!selectedRun.value?.report_markdown) return ''
  const rawHtml = marked.parse(selectedRun.value.report_markdown) as string
  return DOMPurify.sanitize(rawHtml, { FORBID_TAGS: ['img', 'video', 'audio', 'iframe', 'style'], FORBID_ATTR: ['style', 'srcset'] })
})

async function loadTasks() {
  if (loading.value) return
  loading.value = true
  listError.value = ''
  try {
    const res = await fetchTaskRuns({
      page: page.value,
      limit: limit.value,
      status: statusFilter.value === 'lost' ? 'running' : statusFilter.value || undefined,
      connectivity: statusFilter.value === 'lost' ? 'disconnected' : statusFilter.value === 'running' ? 'connected' : undefined,
      device_serial: deviceFilter.value.trim() || undefined,
    })
    items.value = res.items
    total.value = res.total
  } catch (e) {
    listError.value = e instanceof Error ? e.message : '加载失败'
  } finally {
    loading.value = false
  }
}

async function openDetail(run: TaskRunItem) {
  selectedRun.value = run
  detailOpen.value = true
  logs.value = []
  await loadLogs(run.id, true)
}

async function loadLogs(runId: string, reset = false) {
  if (logsLoading.value) return
  logsLoading.value = true
  detailError.value = ''
  try {
    const lastSeq = reset || logs.value.length === 0 ? 0 : logs.value[logs.value.length - 1].seq
    const res = await fetchTaskRunLogs(runId, { after: lastSeq, limit: 500 })
    if (reset) {
      logs.value = res.items
    } else if (res.items.length > 0) {
      logs.value.push(...res.items)
    }
    // 同步更新详情状态
    const updated = await fetchTaskRunDetail(runId)
    selectedRun.value = updated

    if (autoScroll.value && logContainerRef.value) {
      setTimeout(() => {
        if (logContainerRef.value) {
          logContainerRef.value.scrollTop = logContainerRef.value.scrollHeight
        }
      }, 50)
    }
  } catch (e) {
    detailError.value = e instanceof Error ? e.message : '日志加载失败'
  } finally {
    logsLoading.value = false
  }
}

function startLogPolling() {
  stopLogPolling()
  if (selectedRun.value && selectedRun.value.status === 'running') {
    logPollingTimer = window.setInterval(() => {
      if (selectedRun.value) {
        loadLogs(selectedRun.value.id, false)
      }
    }, 2000)
  }
}

function stopLogPolling() {
  if (logPollingTimer !== null) {
    clearInterval(logPollingTimer)
    logPollingTimer = null
  }
}

watch(detailOpen, open => {
  if (open) {
    startLogPolling()
  } else {
    stopLogPolling()
  }
})

function formatTime(ts: number) {
  if (!ts) return '-'
  const d = new Date(ts)
  return `${d.toLocaleDateString()} ${d.toLocaleTimeString()}`
}

function formatDuration(ms: number) {
  if (!ms || ms < 0) return '-'
  if (ms < 1000) return `${ms}ms`
  const sec = Math.floor(ms / 1000)
  if (sec < 60) return `${sec}s`
  const min = Math.floor(sec / 60)
  return `${min}m ${sec % 60}s`
}

onMounted(() => {
  loadTasks()
  refreshTimer = window.setInterval(() => {
    if (autoRefresh.value && !detailOpen.value) {
      loadTasks()
    }
  }, 4000)
})

onUnmounted(() => {
  if (refreshTimer !== null) clearInterval(refreshTimer)
  stopLogPolling()
})
</script>

<template>
  <div class="flex min-h-screen flex-col bg-background">
    <!-- 顶部导航栏 -->
    <header class="flex flex-wrap items-center justify-between gap-3 border-b px-4 py-3">
      <div class="flex items-center gap-3">
        <Button variant="ghost" size="sm" as-child>
          <RouterLink to="/">
            <ArrowLeft class="size-4" />
            <span class="hidden sm:inline">返回设备</span>
          </RouterLink>
        </Button>
        <div class="h-4 w-px bg-border" />
        <Activity class="size-5 text-primary" />
        <div>
          <h1 class="text-base font-semibold leading-tight">任务运行</h1>
        </div>
      </div>

      <div class="flex items-center gap-2">
        <Button variant="ghost" size="sm" :disabled="loading" @click="loadTasks">
          <RefreshCw class="size-3.5" :class="{ 'animate-spin': loading }" />
          刷新
        </Button>
      </div>
    </header>

    <!-- 过滤控制栏 -->
    <div class="flex flex-wrap items-center justify-between gap-3 border-b bg-muted/20 px-4 py-2.5">
      <div class="flex flex-wrap items-center gap-2">
        <Tabs v-model="statusFilter" @update:model-value="() => { page = 1; loadTasks(); }">
          <TabsList>
            <TabsTrigger value="">全部</TabsTrigger>
            <TabsTrigger value="running">运行中</TabsTrigger>
            <TabsTrigger value="succeeded">已完成</TabsTrigger>
            <TabsTrigger value="failed">失败</TabsTrigger>
            <TabsTrigger value="lost">已失联</TabsTrigger>
          </TabsList>
        </Tabs>
      </div>

      <div class="flex items-center gap-2">
        <div class="relative w-48 sm:w-64">
          <Search class="absolute left-2.5 top-2.5 size-3.5 text-muted-foreground" />
          <Input
            v-model="deviceFilter"
            placeholder="按设备序列号筛选..."
            class="h-8 pl-8 text-xs"
            @keydown.enter="() => { page = 1; loadTasks(); }"
          />
        </div>
        <Button variant="ghost" size="sm" class="h-8 px-2 text-xs" @click="autoRefresh = !autoRefresh">
          <span class="size-2 rounded-full mr-1.5" :class="autoRefresh ? 'bg-emerald-500' : 'bg-muted-foreground'" />
          自动刷新
        </Button>
      </div>
    </div>

    <!-- 任务表格 -->
    <main class="flex-1 overflow-auto p-4">
      <p v-if="listError" role="alert" class="mb-3 text-sm text-destructive">{{ listError }}</p>
      <div class="rounded-md border bg-card">
        <div class="overflow-x-auto">
          <table class="w-full text-left text-sm">
            <thead class="border-b bg-muted/40 text-xs text-muted-foreground">
              <tr>
                <th class="px-4 py-2.5 font-medium">任务名称</th>
                <th class="px-4 py-2.5 font-medium">设备序列号</th>
                <th class="px-4 py-2.5 font-medium">执行端</th>
                <th class="px-4 py-2.5 font-medium">状态</th>
                <th class="px-4 py-2.5 font-medium">日志数</th>
                <th class="px-4 py-2.5 font-medium">最后活跃</th>
                <th class="px-4 py-2.5 font-medium">耗时</th>
                <th class="px-4 py-2.5 text-right font-medium">操作</th>
              </tr>
            </thead>
            <tbody class="divide-y divide-border">
              <tr v-if="loading && items.length === 0">
                <td colspan="8" class="h-32 text-center text-muted-foreground">
                  <div class="flex items-center justify-center gap-2">
                    <Loader2 class="size-4 animate-spin" />
                    <span>加载中...</span>
                  </div>
                </td>
              </tr>
              <tr v-else-if="!listError && items.length === 0">
                <td colspan="8" class="h-32 text-center text-muted-foreground">
                  暂无任务运行记录
                </td>
              </tr>
              <tr
                v-for="item in items"
                :key="item.id"
                class="hover:bg-muted/30 transition-colors cursor-pointer"
                @click="openDetail(item)"
              >
                <td class="px-4 py-3 font-medium">
                  <div class="flex flex-col">
                    <span>{{ item.name }}</span>
                    <span class="font-mono text-[11px] text-muted-foreground">{{ item.id }}</span>
                  </div>
                </td>
                <td class="px-4 py-3 font-mono text-xs">{{ item.device_serial }}</td>
                <td class="px-4 py-3 text-xs text-muted-foreground">{{ item.source }}</td>
                <td class="px-4 py-3">
                  <Badge v-if="item.status === 'succeeded'" variant="outline" class="border-emerald-500/40 bg-emerald-500/10 text-emerald-600 dark:text-emerald-400 gap-1">
                    <CheckCircle2 class="size-3" />
                    已完成
                  </Badge>
                  <Badge v-else-if="item.status === 'failed'" variant="outline" class="border-destructive/40 bg-destructive/10 text-destructive gap-1">
                    <XCircle class="size-3" />
                    失败
                  </Badge>
                  <Badge v-else-if="item.disconnected" variant="outline" class="border-amber-500/40 bg-amber-500/10 text-amber-600 dark:text-amber-400 gap-1">
                    <AlertCircle class="size-3" />
                    已失联
                  </Badge>
                  <Badge v-else-if="item.status === 'running'" variant="outline" class="border-blue-500/40 bg-blue-500/10 text-blue-600 dark:text-blue-400 gap-1">
                    <Loader2 class="size-3 animate-spin" />
                    运行中
                  </Badge>
                  <Badge v-else variant="outline" class="text-muted-foreground">
                    {{ item.status }}
                  </Badge>
                </td>
                <td class="px-4 py-3 text-xs">
                  <span>{{ item.log_count }}</span>
                  <span v-if="item.log_truncated" class="text-amber-500 text-[10px] ml-1">(截断)</span>
                </td>
                <td class="px-4 py-3 text-xs text-muted-foreground">{{ formatTime(item.last_heartbeat_at) }}</td>
                <td class="px-4 py-3 text-xs font-mono text-muted-foreground">{{ formatDuration(item.duration_ms) }}</td>
                <td class="px-4 py-3 text-right">
                  <Button variant="ghost" size="sm" class="h-7 px-2 text-xs" @click.stop="openDetail(item)">
                    详情
                  </Button>
                </td>
              </tr>
            </tbody>
          </table>
        </div>
      </div>
      <div v-if="total > limit" class="mt-3 flex items-center justify-end gap-3 text-sm">
        <Button variant="outline" size="sm" :disabled="page === 1 || loading" @click="page--; loadTasks()">上一页</Button>
        <span>{{ page }} / {{ Math.ceil(total / limit) }}</span>
        <Button variant="outline" size="sm" :disabled="page * limit >= total || loading" @click="page++; loadTasks()">下一页</Button>
      </div>
    </main>

    <!-- 任务详情对话框 -->
    <Dialog v-model:open="detailOpen">
      <DialogContent class="max-w-4xl max-h-[88vh] flex flex-col p-6">
        <DialogHeader class="border-b pb-3">
          <div class="flex items-center justify-between">
            <div class="flex items-center gap-2">
              <Terminal class="size-5 text-primary" />
              <DialogTitle class="text-base">{{ selectedRun?.name }}</DialogTitle>
            </div>
            <div class="flex items-center gap-2">
              <Badge v-if="selectedRun?.status === 'succeeded'" class="bg-emerald-500/15 text-emerald-600 border-emerald-500/30">已完成</Badge>
              <Badge v-else-if="selectedRun?.status === 'failed'" variant="destructive">执行失败</Badge>
              <Badge v-else-if="selectedRun?.disconnected" class="bg-amber-500/15 text-amber-600 border-amber-500/30">⚠️ 心跳失联</Badge>
              <Badge v-else-if="selectedRun?.status === 'running'" class="bg-blue-500/15 text-blue-600 border-blue-500/30">运行中</Badge>
            </div>
          </div>
          <DialogDescription class="font-mono text-xs">
            运行 ID: {{ selectedRun?.id }} | 设备: {{ selectedRun?.device_serial }} | 客户端 ID: {{ selectedRun?.client_run_id }}
          </DialogDescription>
        </DialogHeader>

        <p v-if="detailError" role="alert" class="text-sm text-destructive">{{ detailError }}</p>
        <Tabs default-value="logs" class="flex-1 flex flex-col min-h-0 pt-2">
          <div class="flex items-center justify-between border-b pb-2">
            <TabsList>
              <TabsTrigger value="logs">实时日志 ({{ logs.length }})</TabsTrigger>
              <TabsTrigger value="report">结果报告</TabsTrigger>
            </TabsList>
            <div class="flex items-center gap-2">
              <Button
                variant="outline"
                size="sm"
                class="h-7 text-xs"
                as-child
              >
                <a :href="`/api/task-runs/${selectedRun?.id}/logs?download=true`" target="_blank" download>
                  <Download class="size-3 mr-1" />
                  下载日志
                </a>
              </Button>
            </div>
          </div>

          <!-- 日志 Tab -->
          <TabsContent value="logs" class="flex-1 flex flex-col min-h-0 pt-2 m-0">
            <div class="flex items-center justify-between pb-2 text-xs text-muted-foreground">
              <div class="flex items-center gap-2">
                <span>流筛选:</span>
                <button
                  type="button"
                  class="px-2 py-0.5 rounded text-xs cursor-pointer"
                  :class="logStreamFilter === 'all' ? 'bg-primary text-primary-foreground font-medium' : 'bg-muted'"
                  @click="logStreamFilter = 'all'"
                >
                  全部
                </button>
                <button
                  type="button"
                  class="px-2 py-0.5 rounded text-xs cursor-pointer"
                  :class="logStreamFilter === 'stdout' ? 'bg-primary text-primary-foreground font-medium' : 'bg-muted'"
                  @click="logStreamFilter = 'stdout'"
                >
                  stdout
                </button>
                <button
                  type="button"
                  class="px-2 py-0.5 rounded text-xs cursor-pointer"
                  :class="logStreamFilter === 'stderr' ? 'bg-primary text-primary-foreground font-medium' : 'bg-muted'"
                  @click="logStreamFilter = 'stderr'"
                >
                  stderr
                </button>
              </div>
              <label class="flex items-center gap-1.5 cursor-pointer">
                <input v-model="autoScroll" type="checkbox" class="rounded border-border size-3.5" />
                <span>自动滚屏跟随</span>
              </label>
            </div>

            <div
              ref="logContainerRef"
              class="flex-1 min-h-[300px] overflow-auto rounded-md bg-zinc-950 p-3 font-mono text-xs text-zinc-100 border border-zinc-800"
            >
              <div v-if="filteredLogs.length === 0" class="h-full flex items-center justify-center text-zinc-500">
                暂无日志数据
              </div>
              <div
                v-for="entry in filteredLogs"
                :key="entry.seq"
                class="leading-relaxed hover:bg-zinc-900/60 px-1 rounded flex gap-2"
              >
                <span class="text-zinc-600 select-none w-10 text-right shrink-0">{{ entry.seq }}</span>
                <span class="text-zinc-500 shrink-0">{{ new Date(entry.timestamp).toLocaleTimeString() }}</span>
                <span
                  class="shrink-0 px-1 rounded text-[10px]"
                  :class="entry.stream === 'stderr' ? 'bg-rose-950 text-rose-400' : 'bg-zinc-800 text-zinc-400'"
                >
                  {{ entry.stream }}
                </span>
                <span class="break-all whitespace-pre-wrap flex-1" :class="{ 'text-rose-300': entry.stream === 'stderr' }">
                  {{ entry.message }}
                </span>
              </div>
            </div>
            <Button v-if="logs.length < (selectedRun?.log_count ?? 0)" variant="outline" size="sm" :disabled="logsLoading" @click="selectedRun && loadLogs(selectedRun.id)">加载更多日志</Button>
          </TabsContent>

          <!-- 结果报告 Tab -->
          <TabsContent value="report" class="flex-1 overflow-auto p-4 m-0">
            <div class="space-y-4">
              <div class="grid grid-cols-2 gap-4 rounded-lg border p-3 bg-muted/20">
                <div>
                  <span class="text-xs text-muted-foreground block">退出码</span>
                  <span class="font-mono text-sm font-semibold">
                    {{ selectedRun?.exit_code !== null ? selectedRun?.exit_code : '-' }}
                  </span>
                </div>
                <div>
                  <span class="text-xs text-muted-foreground block">总耗时</span>
                  <span class="font-mono text-sm font-semibold">
                    {{ formatDuration(selectedRun?.duration_ms || 0) }}
                  </span>
                </div>
              </div>

              <div v-if="selectedRun?.summary">
                <h4 class="text-xs font-semibold text-muted-foreground mb-1">执行摘要</h4>
                <div class="rounded border p-3 text-sm bg-card">{{ selectedRun.summary }}</div>
              </div>

              <div>
                <h4 class="text-xs font-semibold text-muted-foreground mb-1">报告</h4>
                <div
                  v-if="renderedMarkdown"
                  class="prose prose-sm dark:prose-invert max-w-none rounded border p-4 bg-card"
                  v-html="renderedMarkdown"
                />
                <div v-else class="text-xs text-muted-foreground italic rounded border p-4 text-center">
                  暂无报告
                </div>
              </div>
            </div>
          </TabsContent>
        </Tabs>
      </DialogContent>
    </Dialog>

  </div>
</template>
