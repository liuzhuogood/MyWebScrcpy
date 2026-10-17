<script setup lang="ts">
import { computed, onMounted, ref, watch } from 'vue'
import { RouterLink } from 'vue-router'
import {
  AlertCircle,
  AlertTriangle,
  Download,
  FileText,
  Folder,
  FolderOpen,
  ImageIcon,
  Loader2,
  RefreshCw,
  Settings2,
  Terminal,
  Video,
} from 'lucide-vue-next'
import { marked } from 'marked'
import DOMPurify from 'dompurify'
import { Button } from '@/components/ui/button'
import { Badge } from '@/components/ui/badge'
import { Input } from '@/components/ui/input'
import {
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from '@/components/ui/select'
import {
  Dialog,
  DialogContent,
  DialogFooter,
  DialogHeader,
  DialogTitle,
} from '@/components/ui/dialog'
import { Tabs, TabsContent, TabsList, TabsTrigger } from '@/components/ui/tabs'
import SystemConfigDialog from '@/components/system/SystemConfigDialog.vue'
import { fetchDevices } from '@/api/devices'
import {
  fetchTaskRunDetail,
  fetchTaskRuns,
  getTaskOutputUrl,
  scanTaskRuns,
  type OutputCheckResult,
  type TaskDetail,
  type TaskSummary,
} from '@/api/taskRuns'

// 列表与扫描状态
const scanning = ref(false)
const loading = ref(false)
const listError = ref('')
const items = ref<TaskSummary[]>([])
const total = ref(0)
const page = ref(1)
const limit = ref(20)
const scannedAt = ref<number>(0)
const rootDir = ref<string>('')

// 筛选字段
const deviceFilter = ref('all')
const statusFilter = ref('all')
const sourceFilter = ref('')
const devicesList = ref<string[]>([])

// 选中的任务与详情状态
const selectedTaskId = ref<string | null>(null)
const selectedDetail = ref<TaskDetail | null>(null)
const detailLoading = ref(false)
const detailError = ref('')
const detailTab = ref<'outputs' | 'report' | 'logs' | 'actions'>('outputs')
const imageErrors = ref<Record<string, boolean>>({})

// 页内大图预览状态
interface ImagePreviewState {
  url: string
  downloadUrl: string
  title: string
}
const previewImage = ref<ImagePreviewState | null>(null)
const previewImageError = ref(false)

// 系统与 AI 设置弹窗
const configOpen = ref(false)

// 竞态防御序号与防抖计时器
let currentListSeq = 0
let currentDetailSeq = 0
let filterDebounceTimer: ReturnType<typeof setTimeout> | null = null

// 可选设备列表（包含发现的设备与任务中的设备序列号）
const availableDevices = computed(() => {
  const set = new Set<string>(devicesList.value)
  for (const item of items.value) {
    if (item.device_serial) set.add(item.device_serial)
  }
  return Array.from(set).sort()
})

// 业务成功但产出不完整的任务统计
const incompleteSuccessCount = computed(() => {
  return items.value.filter(t => !t.outputs_complete && t.status === 'success').length
})

// 格式化时间戳
function formatTime(ts?: number | string): string {
  if (!ts) return '-'
  const d = new Date(typeof ts === 'string' && !isNaN(Number(ts)) ? Number(ts) : ts)
  if (isNaN(d.getTime())) return String(ts)
  const yyyy = d.getFullYear()
  const mm = String(d.getMonth() + 1).padStart(2, '0')
  const dd = String(d.getDate()).padStart(2, '0')
  const hh = String(d.getHours()).padStart(2, '0')
  const min = String(d.getMinutes()).padStart(2, '0')
  const ss = String(d.getSeconds()).padStart(2, '0')
  return `${yyyy}-${mm}-${dd} ${hh}:${min}:${ss}`
}

// 格式化字节大小
function formatBytes(bytes?: number): string {
  if (bytes === undefined || bytes === null || bytes === 0) return '0 B'
  const k = 1024
  const sizes = ['B', 'KB', 'MB', 'GB']
  const i = Math.floor(Math.log(bytes) / Math.log(k))
  return `${(bytes / Math.pow(k, i)).toFixed(1)} ${sizes[i]}`
}

// 渲染 Markdown 安全 HTML
const sanitizedReportHtml = computed(() => {
  const md = selectedDetail.value?.report_content || ''
  if (!md) return ''
  const rawHtml = marked.parse(md, { async: false }) as string
  return DOMPurify.sanitize(rawHtml, {
    USE_PROFILES: { html: true },
    FORBID_TAGS: ['img', 'video', 'audio', 'iframe', 'object', 'embed', 'style', 'script', 'source', 'track'],
    FORBID_ATTR: ['style', 'onerror', 'onload', 'onclick', 'srcset'],
  })
})

// 日志下载地址
const logDownloadUrl = computed(() => {
  if (!selectedDetail.value) return ''
  const logOut = selectedDetail.value.outputs?.find(o => o.category === 'log' && o.exists && o.valid)
  if (logOut) {
    return getTaskOutputUrl(selectedDetail.value.id, logOut.id, undefined, true)
  }
  return ''
})

// 判断文件名是否为图片
function isImageFile(filename: string): boolean {
  return /\.(png|jpe?g|webp|svg|gif|bmp|avif)$/i.test(filename)
}

// 提取产出中的图片子文件
function getImageFiles(out: OutputCheckResult): string[] {
  if (!out.files) return []
  return out.files.filter(f => isImageFile(f) || out.category === 'screenshot')
}

// 提取产出中的非图片子文件
function getNonImageFiles(out: OutputCheckResult): string[] {
  if (!out.files) return []
  return out.files.filter(f => !isImageFile(f) && out.category !== 'screenshot')
}

// 图片加载标识与错误处理
function getImageKey(taskId: string, outId: string, file?: string): string {
  return `${taskId}:${outId}:${file || ''}`
}

function handleImageError(key: string) {
  imageErrors.value[key] = true
}

// 页内大图预览控制
function openImagePreview(url: string, downloadUrl: string, title: string) {
  previewImageError.value = false
  previewImage.value = { url, downloadUrl, title }
}

function closeImagePreview() {
  previewImage.value = null
  previewImageError.value = false
}

// 加载最近一次扫描的任务列表
async function loadList() {
  const seq = ++currentListSeq
  loading.value = true
  listError.value = ''
  try {
    const res = await fetchTaskRuns({
      page: page.value,
      limit: limit.value,
      status: statusFilter.value === 'all' ? undefined : (statusFilter.value || undefined),
      device_serial: deviceFilter.value === 'all' ? undefined : (deviceFilter.value || undefined),
      source: sourceFilter.value.trim() || undefined,
    })
    if (seq !== currentListSeq) return
    items.value = res.items || []
    total.value = res.total || 0
    scannedAt.value = res.scanned_at || 0
    rootDir.value = res.root_dir || ''

    // 默认或维持选中项
    if (items.value.length > 0) {
      const exists = items.value.some(t => t.id === selectedTaskId.value)
      if (!selectedTaskId.value || !exists) {
        void selectTask(items.value[0].id)
      } else {
        void selectTask(selectedTaskId.value, true)
      }
    } else {
      selectedTaskId.value = null
      selectedDetail.value = null
    }
  } catch (err: unknown) {
    if (seq !== currentListSeq) return
    listError.value = err instanceof Error ? err.message : '加载任务列表失败'
  } finally {
    if (seq === currentListSeq) {
      loading.value = false
    }
  }
}

// 手动主动扫描
async function handleScan() {
  if (scanning.value) return
  scanning.value = true
  listError.value = ''
  try {
    const result = await scanTaskRuns()
    scannedAt.value = result.scanned_at
    rootDir.value = result.root_dir
    await loadList()
  } catch (err: unknown) {
    listError.value = err instanceof Error ? err.message : '扫描任务目录失败'
  } finally {
    scanning.value = false
  }
}

// 选择任务并加载详情（具有竞态防御）
async function selectTask(taskId: string, forceRefresh = false) {
  if (selectedTaskId.value === taskId && !forceRefresh && selectedDetail.value) {
    return
  }
  selectedTaskId.value = taskId
  const seq = ++currentDetailSeq
  detailLoading.value = true
  detailError.value = ''
  if (!forceRefresh) {
    selectedDetail.value = null
    detailTab.value = 'outputs'
  }

  try {
    const detail = await fetchTaskRunDetail(taskId)
    // 竞态防御：校验序列号和当前选中的任务ID
    if (seq !== currentDetailSeq || selectedTaskId.value !== taskId) return
    selectedDetail.value = detail
  } catch (err: unknown) {
    if (seq !== currentDetailSeq || selectedTaskId.value !== taskId) return
    detailError.value = err instanceof Error ? err.message : '加载任务详情失败'
  } finally {
    if (seq === currentDetailSeq && selectedTaskId.value === taskId) {
      detailLoading.value = false
    }
  }
}

// 刷新当前选中的任务详情
function refreshCurrentTask() {
  if (!selectedTaskId.value || detailLoading.value) return
  void selectTask(selectedTaskId.value, true)
}

// 筛选切换处理
function handleFilterChange() {
  page.value = 1
  void loadList()
}

// 分页切换
function changePage(p: number) {
  page.value = p
  void loadList()
}

// 获取产出图标
function getOutputIcon(category: string) {
  switch (category) {
    case 'report':
      return FileText
    case 'log':
      return Terminal
    case 'screenshot':
      return ImageIcon
    case 'video':
      return Video
    default:
      return FolderOpen
  }
}

// 获取产出图标色彩
function getOutputIconClass(category: string) {
  switch (category) {
    case 'report':
      return 'text-blue-500'
    case 'log':
      return 'text-amber-500'
    case 'screenshot':
      return 'text-emerald-500'
    case 'video':
      return 'text-purple-500'
    default:
      return 'text-muted-foreground'
  }
}

// 监听状态与设备筛选变更
watch([statusFilter, deviceFilter], () => {
  handleFilterChange()
})

// 监听来源筛选变更（防抖）
watch(sourceFilter, () => {
  if (filterDebounceTimer) clearTimeout(filterDebounceTimer)
  filterDebounceTimer = setTimeout(() => {
    handleFilterChange()
  }, 250)
})

// 监听系统设置弹窗关闭，关闭后重新读取（可能更改了任务根目录）
watch(configOpen, (isOpen, wasOpen) => {
  if (!isOpen && wasOpen) {
    void loadList()
  }
})

onMounted(async () => {
  try {
    const devs = await fetchDevices()
    devicesList.value = devs.map(d => d.serial).filter(Boolean)
  } catch {
    // 忽略设备列表异常
  }
  await loadList()
})
</script>

<template>
  <div class="flex flex-col gap-5 p-4 sm:p-6 max-w-7xl mx-auto min-h-screen">
    <!-- 顶栏与控制 -->
    <header class="flex flex-wrap items-center justify-between gap-4 border-b pb-4">
      <div class="flex items-center gap-3">
        <RouterLink
          to="/"
          class="flex size-10 items-center justify-center rounded-lg bg-primary/10 text-primary hover:bg-primary/20 transition-colors shrink-0"
          title="返回首页"
        >
          <Folder class="size-5" />
        </RouterLink>
        <div>
          <h1 class="text-xl font-bold tracking-tight">任务运行中心</h1>
          <p class="text-xs text-muted-foreground mt-0.5">
            工作目录：<span class="font-mono text-foreground">{{ rootDir || '~/.mywebscrcppy/tasks/' }}</span> ·
            上次扫描：{{ scannedAt ? formatTime(scannedAt) : '尚未扫描' }} · 共 {{ total }} 个任务
          </p>
        </div>
      </div>

      <div class="flex items-center gap-2">
        <Button
          variant="default"
          size="sm"
          :disabled="scanning"
          @click="handleScan"
        >
          <RefreshCw class="size-3.5 mr-1.5" :class="{ 'animate-spin': scanning }" />
          {{ scanning ? '扫描中...' : '扫描刷新' }}
        </Button>
        <Button variant="outline" size="sm" @click="configOpen = true">
          <Settings2 class="size-3.5 mr-1.5" />
          系统与 AI 设置
        </Button>
      </div>
    </header>

    <!-- 过滤条与提示 -->
    <div class="flex flex-wrap items-center justify-between gap-3 bg-muted/40 p-3 rounded-lg border">
      <div class="flex flex-wrap items-center gap-2">
        <span class="text-xs font-medium text-muted-foreground mr-1">过滤：</span>
        <Select v-model="deviceFilter">
          <SelectTrigger class="h-8 w-36 text-xs">
            <SelectValue placeholder="设备筛选" />
          </SelectTrigger>
          <SelectContent>
            <SelectItem value="all">全部设备</SelectItem>
            <SelectItem v-for="dev in availableDevices" :key="dev" :value="dev">{{ dev }}</SelectItem>
          </SelectContent>
        </Select>

        <Select v-model="statusFilter">
          <SelectTrigger class="h-8 w-32 text-xs">
            <SelectValue placeholder="状态筛选" />
          </SelectTrigger>
          <SelectContent>
            <SelectItem value="all">全部状态</SelectItem>
            <SelectItem value="running">运行中</SelectItem>
            <SelectItem value="success">成功</SelectItem>
            <SelectItem value="failed">失败</SelectItem>
            <SelectItem value="paused">已暂停</SelectItem>
            <SelectItem value="cancelled">已取消</SelectItem>
          </SelectContent>
        </Select>

        <Input
          v-model="sourceFilter"
          placeholder="来源筛选"
          class="h-8 w-32 text-xs bg-background"
          @keyup.enter="handleFilterChange"
        />
      </div>

      <div v-if="incompleteSuccessCount > 0" class="flex items-center gap-1 text-amber-600 dark:text-amber-400 text-xs font-medium">
        <AlertTriangle class="size-3.5" />
        {{ incompleteSuccessCount }} 个任务业务成功但产出不完整
      </div>
    </div>

    <!-- 主展示区：任务卡片列表 (5列) + 任务常驻详情 (7列) -->
    <div class="grid grid-cols-1 lg:grid-cols-12 gap-5 items-start">
      <!-- 任务列表 (5列) -->
      <div class="lg:col-span-5 space-y-2">
        <div class="text-xs font-medium text-muted-foreground px-1 mb-1">
          任务列表 ({{ total }})
        </div>

        <!-- 列表加载中 -->
        <div v-if="loading && items.length === 0" class="p-8 text-center text-muted-foreground border rounded-lg bg-card text-xs">
          <Loader2 class="size-5 animate-spin mx-auto mb-2 text-primary" />
          正在读取任务...
        </div>

        <!-- 列表错误提示 -->
        <div v-else-if="listError" class="p-4 rounded-lg border border-destructive/30 bg-destructive/5 text-destructive text-xs flex items-center gap-2">
          <AlertCircle class="size-4 shrink-0" />
          <span>{{ listError }}</span>
        </div>

        <!-- 列表空状态 -->
        <div v-else-if="items.length === 0" class="p-8 text-center text-muted-foreground border rounded-lg bg-card text-xs">
          当前未索引任何任务，请点击上方“扫描刷新”发现工作目录下的任务。
        </div>

        <!-- 任务卡片项 -->
        <div
          v-for="task in items"
          :key="task.id"
          class="p-3.5 rounded-lg border cursor-pointer transition-all hover:border-primary/50 text-left"
          :class="selectedTaskId === task.id ? 'border-primary bg-primary/5 shadow-xs' : 'bg-card'"
          @click="selectTask(task.id)"
        >
          <div class="flex items-start justify-between gap-2">
            <div class="font-medium text-sm line-clamp-1">{{ task.name || task.id }}</div>
            <div class="flex items-center gap-1.5 shrink-0">
              <Badge v-if="task.status === 'success' && task.outputs_complete" variant="default" class="bg-emerald-600 text-xs">
                全部完成
              </Badge>
              <Badge v-else-if="task.status === 'success' && !task.outputs_complete" variant="outline" class="border-amber-500 text-amber-600 text-xs">
                业务成功 · 产出不完整
              </Badge>
              <Badge v-else-if="task.status === 'running'" variant="secondary" class="text-xs animate-pulse">
                运行中
              </Badge>
              <Badge v-else-if="task.status === 'failed'" variant="destructive" class="text-xs">
                failed
              </Badge>
              <Badge v-else variant="outline" class="text-xs text-muted-foreground">
                {{ task.status }}
              </Badge>
            </div>
          </div>

          <p class="text-xs text-muted-foreground mt-1 line-clamp-1 font-mono">
            {{ task.id }} · {{ task.device_serial || '未绑定设备' }}
          </p>

          <div class="mt-2.5 flex items-center justify-between text-xs text-muted-foreground border-t pt-2">
            <span>更新：{{ formatTime(task.updated_at) }}</span>
            <span v-if="task.is_stale" class="text-amber-500 font-medium">信息陈旧</span>
            <span v-else-if="task.business_result" class="text-emerald-600 dark:text-emerald-400 line-clamp-1">{{ task.business_result }}</span>
            <span v-else class="text-muted-foreground">-</span>
          </div>
        </div>

        <!-- 分页控件 -->
        <div v-if="total > limit || page > 1" class="flex items-center justify-between text-xs text-muted-foreground pt-2 px-1">
          <span>共 {{ total }} 条 · 第 {{ page }} 页</span>
          <div class="flex items-center gap-1">
            <Button variant="outline" size="sm" class="h-7 px-2.5 text-xs" :disabled="page <= 1" @click="changePage(page - 1)">上一页</Button>
            <Button variant="outline" size="sm" class="h-7 px-2.5 text-xs" :disabled="page * limit >= total" @click="changePage(page + 1)">下一页</Button>
          </div>
        </div>
      </div>

      <!-- 任务详情右侧常驻面板 (7列) -->
      <div v-if="!selectedTaskId" class="lg:col-span-7 rounded-lg border bg-card p-12 text-center text-muted-foreground text-xs">
        <FolderOpen class="size-8 mx-auto mb-2 opacity-40" />
        请选择左侧任务查看详情
      </div>

      <div v-else-if="detailLoading && !selectedDetail" class="lg:col-span-7 rounded-lg border bg-card p-12 text-center text-muted-foreground text-xs">
        <Loader2 class="size-6 animate-spin mx-auto mb-2 text-primary" />
        正在读取任务详情与产出状态...
      </div>

      <div v-else-if="detailError" class="lg:col-span-7 rounded-lg border border-destructive/30 bg-destructive/5 p-8 text-center text-destructive text-xs">
        <AlertCircle class="size-6 mx-auto mb-2" />
        <p>{{ detailError }}</p>
        <Button variant="outline" size="sm" class="mt-3 text-xs" @click="refreshCurrentTask">重试</Button>
      </div>

      <div v-else-if="selectedDetail" class="lg:col-span-7 rounded-lg border bg-card p-4 space-y-4">
        <!-- 详情头部 -->
        <div class="flex items-start justify-between gap-3 border-b pb-3">
          <div>
            <div class="flex items-center gap-2 flex-wrap">
              <h2 class="text-base font-bold">{{ selectedDetail.task_def?.name || selectedDetail.id }}</h2>
              <Badge variant="outline" class="font-mono text-[11px]">{{ selectedDetail.id }}</Badge>
              <Badge v-if="selectedDetail.state?.status === 'success' && selectedDetail.outputs_complete" variant="default" class="bg-emerald-600 text-xs">
                全部完成
              </Badge>
              <Badge v-else-if="selectedDetail.state?.status === 'success' && !selectedDetail.outputs_complete" variant="outline" class="border-amber-500 text-amber-600 text-xs">
                业务成功 · 产出不完整
              </Badge>
              <Badge v-else-if="selectedDetail.state?.status === 'running'" variant="secondary" class="text-xs animate-pulse">
                运行中
              </Badge>
              <Badge v-else-if="selectedDetail.state?.status" variant="destructive" class="text-xs">
                {{ selectedDetail.state.status }}
              </Badge>
              <Badge v-if="selectedDetail.is_stale" variant="destructive" class="text-xs">已陈旧</Badge>
            </div>
            <p class="text-xs text-muted-foreground mt-0.5">
              设备：<span class="font-medium text-foreground">{{ selectedDetail.task_def?.device_serial || '未绑定设备' }}</span> ·
              技能：<span class="font-mono">{{ selectedDetail.task_def?.skill_dir || '-' }}</span>
            </p>
          </div>

          <Button
            variant="ghost"
            size="sm"
            class="h-8 text-xs shrink-0"
            :disabled="detailLoading"
            @click="refreshCurrentTask"
          >
            <RefreshCw class="size-3.5 mr-1" :class="{ 'animate-spin': detailLoading }" />
            刷新详情
          </Button>
        </div>

        <!-- 目标与结果 -->
        <div class="grid grid-cols-1 sm:grid-cols-2 gap-3 text-xs bg-muted/30 p-3 rounded-md border">
          <div>
            <span class="text-muted-foreground font-medium">任务目标：</span>
            <p class="mt-0.5 font-normal">{{ selectedDetail.task_def?.goal || '-' }}</p>
          </div>
          <div>
            <span class="text-muted-foreground font-medium">业务结果：</span>
            <p class="mt-0.5 font-normal text-emerald-600 dark:text-emerald-400">
              {{ selectedDetail.state?.last_result || selectedDetail.state?.status || '-' }}
            </p>
          </div>
        </div>

        <!-- 详情选项卡 -->
        <Tabs v-model="detailTab" class="w-full">
          <TabsList class="grid grid-cols-4 h-9">
            <TabsTrigger value="outputs" class="text-xs">
              产出清单 ({{ selectedDetail.outputs?.length || 0 }})
            </TabsTrigger>
            <TabsTrigger value="report" class="text-xs">报告预览</TabsTrigger>
            <TabsTrigger value="logs" class="text-xs">日志流</TabsTrigger>
            <TabsTrigger value="actions" class="text-xs">操作历史</TabsTrigger>
          </TabsList>

          <!-- 产出清单面板 -->
          <TabsContent value="outputs" class="space-y-3 pt-3">
            <div v-if="!selectedDetail.outputs || selectedDetail.outputs.length === 0" class="text-center py-8 text-xs text-muted-foreground">
              暂无产出内容
            </div>

            <div
              v-for="out in selectedDetail.outputs"
              :key="out.id"
              class="p-3 rounded-lg border text-xs space-y-2.5"
              :class="out.exists ? 'bg-background' : 'bg-destructive/5 border-destructive/30'"
            >
              <div class="flex items-center justify-between gap-2">
                <div class="flex items-center gap-2.5">
                  <component :is="getOutputIcon(out.category)" class="size-4 shrink-0" :class="getOutputIconClass(out.category)" />

                  <div>
                    <div class="font-mono font-medium">{{ out.path }}</div>
                    <div class="text-muted-foreground text-[11px]">
                      类别: {{ out.category }} · 格式: {{ out.format }} ·
                      {{ out.required ? '必需产出' : '可选产出' }}
                    </div>
                  </div>
                </div>

                <div class="flex items-center gap-2 shrink-0">
                  <span v-if="out.exists && out.size_bytes !== undefined" class="font-mono text-muted-foreground">{{ formatBytes(out.size_bytes) }}</span>
                  <span v-else-if="out.exists" class="font-mono text-muted-foreground">已生成</span>

                  <Badge v-if="out.exists && out.valid" variant="secondary" class="text-[11px] bg-emerald-50 text-emerald-700 dark:bg-emerald-950 dark:text-emerald-300">
                    有效
                  </Badge>
                  <Badge v-else-if="out.exists" variant="outline" class="text-[11px] border-amber-500 text-amber-600">
                    空内容
                  </Badge>
                  <Badge v-else variant="destructive" class="text-[11px]">
                    缺失
                  </Badge>

                  <a
                    v-if="out.exists && out.valid && (!out.files || out.files.length === 0)"
                    :href="getTaskOutputUrl(selectedDetail.id, out.id, undefined, true)"
                    download
                    class="inline-flex items-center gap-1 text-[11px] px-2 py-1 border rounded hover:bg-accent text-accent-foreground transition-colors"
                  >
                    <Download class="size-3" />
                    下载
                  </a>
                </div>
              </div>

              <!-- 异常提示 -->
              <div v-if="out.error" class="text-red-500 text-[11px]">
                异常: {{ out.error }}
              </div>

              <!-- 截图单文件缩略图预览（页内大图预览统一入口） -->
              <div v-if="out.category === 'screenshot' && out.exists && out.valid && (!out.files || out.files.length === 0)" class="pt-2 border-t">
                <div class="max-w-xs group flex flex-col rounded-md border bg-muted/20 overflow-hidden hover:border-primary/50 transition-colors">
                  <button
                    type="button"
                    class="block aspect-4/3 w-full bg-black/5 dark:bg-black/30 overflow-hidden relative flex items-center justify-center cursor-pointer"
                    title="点击预览大图"
                    @click="openImagePreview(getTaskOutputUrl(selectedDetail.id, out.id), getTaskOutputUrl(selectedDetail.id, out.id, undefined, true), out.path)"
                  >
                    <img
                      v-if="!imageErrors[getImageKey(selectedDetail.id, out.id)]"
                      :src="getTaskOutputUrl(selectedDetail.id, out.id)"
                      :alt="out.path"
                      class="w-full h-full object-contain p-1 transition-transform duration-200 group-hover:scale-105"
                      loading="lazy"
                      @error="handleImageError(getImageKey(selectedDetail.id, out.id))"
                    />
                    <div v-else class="text-center p-2 text-destructive text-[10px] flex flex-col items-center gap-1">
                      <AlertCircle class="size-4" />
                      <span>图片加载失败</span>
                    </div>
                  </button>
                  <div class="p-1.5 flex items-center justify-between border-t bg-card text-[11px]">
                    <button
                      type="button"
                      class="text-primary hover:underline cursor-pointer"
                      @click="openImagePreview(getTaskOutputUrl(selectedDetail.id, out.id), getTaskOutputUrl(selectedDetail.id, out.id, undefined, true), out.path)"
                    >
                      查看原图
                    </button>
                    <a
                      :href="getTaskOutputUrl(selectedDetail.id, out.id, undefined, true)"
                      download
                      class="text-muted-foreground hover:text-foreground inline-flex items-center gap-0.5"
                    >
                      <Download class="size-3" />
                      下载
                    </a>
                  </div>
                </div>
              </div>

              <!-- 视频播放器预览 -->
              <div v-if="out.category === 'video' && out.exists && out.valid" class="pt-2 border-t space-y-2">
                <video
                  controls
                  playsinline
                  preload="metadata"
                  class="max-h-56 rounded border bg-black w-full max-w-md"
                >
                  <source :src="getTaskOutputUrl(selectedDetail.id, out.id)" type="video/mp4" />
                  您的浏览器不支持 HTML5 视频播放。
                </video>
                <div class="flex items-center gap-2 text-xs">
                  <a
                    :href="getTaskOutputUrl(selectedDetail.id, out.id)"
                    target="_blank"
                    class="text-primary hover:underline"
                  >
                    在新标签页打开
                  </a>
                  <span class="text-muted-foreground">·</span>
                  <a
                    :href="getTaskOutputUrl(selectedDetail.id, out.id, undefined, true)"
                    download
                    class="text-primary hover:underline"
                  >
                    下载视频
                  </a>
                </div>
              </div>

              <!-- 目录类产出：图片子文件以缩略图网格平铺展示（页内大图预览统一入口） -->
              <div v-if="out.files && out.files.length > 0" class="pt-2 border-t space-y-3">
                <!-- 图片缩略图网格 -->
                <div v-if="getImageFiles(out).length > 0" class="space-y-1.5">
                  <div class="text-[11px] text-muted-foreground font-medium">
                    图片列表 ({{ getImageFiles(out).length }})：
                  </div>
                  <div class="grid grid-cols-2 sm:grid-cols-3 md:grid-cols-4 gap-2.5">
                    <div
                      v-for="f in getImageFiles(out)"
                      :key="f"
                      class="group flex flex-col rounded-md border bg-muted/20 overflow-hidden hover:border-primary/50 transition-colors"
                    >
                      <button
                        type="button"
                        class="block aspect-4/3 w-full bg-black/5 dark:bg-black/30 overflow-hidden relative flex items-center justify-center cursor-pointer"
                        title="点击预览大图"
                        @click="openImagePreview(getTaskOutputUrl(selectedDetail.id, out.id, f), getTaskOutputUrl(selectedDetail.id, out.id, f, true), f)"
                      >
                        <img
                          v-if="!imageErrors[getImageKey(selectedDetail.id, out.id, f)]"
                          :src="getTaskOutputUrl(selectedDetail.id, out.id, f)"
                          :alt="f"
                          class="w-full h-full object-contain p-1 transition-transform duration-200 group-hover:scale-105"
                          loading="lazy"
                          @error="handleImageError(getImageKey(selectedDetail.id, out.id, f))"
                        />
                        <div v-else class="text-center p-2 text-destructive text-[10px] flex flex-col items-center gap-1">
                          <AlertCircle class="size-4" />
                          <span>图片加载失败</span>
                        </div>
                      </button>

                      <div class="p-1.5 flex flex-col gap-1 border-t bg-card text-[11px]">
                        <span class="font-mono truncate text-foreground" :title="f">{{ f }}</span>
                        <div class="flex items-center justify-between text-[11px] pt-0.5">
                          <button
                            type="button"
                            class="text-primary hover:underline cursor-pointer"
                            @click="openImagePreview(getTaskOutputUrl(selectedDetail.id, out.id, f), getTaskOutputUrl(selectedDetail.id, out.id, f, true), f)"
                          >
                            查看原图
                          </button>
                          <a
                            :href="getTaskOutputUrl(selectedDetail.id, out.id, f, true)"
                            download
                            class="text-muted-foreground hover:text-foreground inline-flex items-center gap-0.5"
                          >
                            <Download class="size-3" />
                            下载
                          </a>
                        </div>
                      </div>
                    </div>
                  </div>
                </div>

                <!-- 非图片子文件列表 -->
                <div v-if="getNonImageFiles(out).length > 0" class="space-y-1.5">
                  <div class="text-[11px] text-muted-foreground font-medium">
                    文件列表 ({{ getNonImageFiles(out).length }})：
                  </div>
                  <div class="grid grid-cols-1 sm:grid-cols-2 gap-1.5 max-h-48 overflow-y-auto font-mono text-xs">
                    <div
                      v-for="f in getNonImageFiles(out)"
                      :key="f"
                      class="flex items-center justify-between p-1.5 bg-muted/30 rounded border"
                    >
                      <span class="truncate max-w-[200px]" :title="f">{{ f }}</span>
                      <div class="flex items-center gap-1 shrink-0">
                        <a
                          :href="getTaskOutputUrl(selectedDetail.id, out.id, f)"
                          target="_blank"
                          class="text-xs text-primary hover:underline px-1 py-0.5"
                        >
                          查看
                        </a>
                        <a
                          :href="getTaskOutputUrl(selectedDetail.id, out.id, f, true)"
                          download
                          class="text-xs text-muted-foreground hover:text-foreground px-1 py-0.5"
                        >
                          下载
                        </a>
                      </div>
                    </div>
                  </div>
                </div>
              </div>
            </div>
          </TabsContent>

          <!-- 报告预览面板 -->
          <TabsContent value="report" class="pt-3">
            <div
              v-if="selectedDetail.report_content"
              class="p-3 bg-muted/20 border rounded-lg text-xs leading-relaxed prose dark:prose-invert max-w-none"
              v-html="sanitizedReportHtml"
            />
            <div v-else class="text-center py-8 text-xs text-muted-foreground">
              暂无报告内容
            </div>
          </TabsContent>

          <!-- 日志流面板 -->
          <TabsContent value="logs" class="pt-3 space-y-2">
            <div class="flex justify-between items-center text-xs">
              <span class="text-muted-foreground font-mono truncate max-w-sm">
                工作目录: {{ selectedDetail.task_def?.skill_dir ? `${selectedDetail.task_def.skill_dir}/logs/task.log` : 'logs/task.log' }}
              </span>
              <a
                v-if="logDownloadUrl"
                :href="logDownloadUrl"
                download
                class="inline-flex items-center gap-1 border rounded px-2 py-1 text-xs hover:bg-accent text-accent-foreground"
              >
                <Download class="size-3 mr-1" />
                下载完整日志
              </a>
            </div>
            <div class="bg-zinc-950 text-zinc-100 p-3 rounded-lg font-mono text-[11px] leading-5 max-h-80 overflow-y-auto whitespace-pre-wrap">
              {{ selectedDetail.log_tail || '暂无日志输出或日志文件为空。' }}
            </div>
          </TabsContent>

          <!-- 操作历史面板 -->
          <TabsContent value="actions" class="pt-3 space-y-2">
            <div v-if="!selectedDetail.recent_actions || selectedDetail.recent_actions.length === 0" class="text-center py-8 text-xs text-muted-foreground">
              暂无操作记录。
            </div>
            <div v-else class="border rounded-md overflow-hidden text-xs">
              <table class="w-full text-left">
                <thead class="bg-muted text-muted-foreground border-b font-medium">
                  <tr>
                    <th class="p-2">时间</th>
                    <th class="p-2">动作</th>
                    <th class="p-2">结果</th>
                  </tr>
                </thead>
                <tbody>
                  <tr v-for="(act, i) in selectedDetail.recent_actions" :key="i" class="border-b last:border-none">
                    <td class="p-2 font-mono text-muted-foreground whitespace-nowrap">{{ formatTime(act.timestamp) }}</td>
                    <td class="p-2 font-medium">{{ act.action }}</td>
                    <td class="p-2">{{ act.result }}</td>
                  </tr>
                </tbody>
              </table>
            </div>
          </TabsContent>
        </Tabs>
      </div>
    </div>

    <!-- 页内大图预览 Dialog -->
    <Dialog :open="previewImage !== null" @update:open="(v) => { if (!v) closeImagePreview() }">
      <DialogContent class="sm:max-w-4xl max-h-[90vh] flex flex-col p-4 sm:p-6" :aria-describedby="undefined">
        <DialogHeader class="shrink-0 flex flex-row items-center justify-between gap-4 border-b pb-3 pr-8">
          <DialogTitle class="truncate font-mono text-sm">
            {{ previewImage?.title }}
          </DialogTitle>
        </DialogHeader>

        <!-- 大图展示区域：自适应视口尺寸 -->
        <div class="min-h-0 flex-1 overflow-auto flex items-center justify-center p-2 bg-black/5 dark:bg-black/40 rounded-md">
          <img
            v-if="previewImage && !previewImageError"
            :src="previewImage.url"
            :alt="previewImage.title"
            class="max-h-[70vh] max-w-full object-contain rounded"
            @error="previewImageError = true"
          />
          <div v-else-if="previewImageError" class="py-12 text-center text-destructive text-xs flex flex-col items-center gap-2">
            <AlertCircle class="size-6" />
            <span>图片加载失败</span>
          </div>
        </div>

        <DialogFooter class="shrink-0 flex items-center justify-between sm:justify-between pt-3 border-t">
          <a
            v-if="previewImage?.downloadUrl"
            :href="previewImage.downloadUrl"
            download
            class="inline-flex items-center gap-1.5 text-xs px-3 py-1.5 border rounded-md hover:bg-accent text-accent-foreground transition-colors"
          >
            <Download class="size-3.5" />
            下载原图
          </a>
          <div v-else />
          <Button variant="outline" size="sm" @click="closeImagePreview">关闭</Button>
        </DialogFooter>
      </DialogContent>
    </Dialog>

    <!-- 系统与 AI 设置 Dialog -->
    <SystemConfigDialog v-model:open="configOpen" initial-tab="service" />
  </div>
</template>
