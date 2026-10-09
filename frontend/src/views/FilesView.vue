<script setup lang="ts">
import {
  ArrowLeft,
  ArrowDownWideNarrow,
  Download,
  Eye,
  FileUp,
  FolderInput,
  FolderPlus,
  Folder as FolderIcon,
  Pencil,
  Search,
  Trash2,
  TriangleAlert,
} from 'lucide-vue-next'
import { computed, onMounted, ref } from 'vue'
import { useRoute, RouterLink } from 'vue-router'
import { toast } from 'vue-sonner'
import {
  AlertDialog,
  AlertDialogAction,
  AlertDialogCancel,
  AlertDialogContent,
  AlertDialogDescription,
  AlertDialogFooter,
  AlertDialogHeader,
  AlertDialogTitle,
} from '@/components/ui/alert-dialog'
import { Badge } from '@/components/ui/badge'
import {
  Breadcrumb,
  BreadcrumbItem,
  BreadcrumbLink,
  BreadcrumbList,
  BreadcrumbPage,
  BreadcrumbSeparator,
} from '@/components/ui/breadcrumb'
import { Button } from '@/components/ui/button'
import { Checkbox } from '@/components/ui/checkbox'
import {
  Dialog,
  DialogContent,
  DialogDescription,
  DialogFooter,
  DialogHeader,
  DialogTitle,
} from '@/components/ui/dialog'
import { Empty, EmptyDescription, EmptyHeader, EmptyMedia, EmptyTitle } from '@/components/ui/empty'
import { Input } from '@/components/ui/input'
import { Progress } from '@/components/ui/progress'
import {
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from '@/components/ui/select'
import { Skeleton } from '@/components/ui/skeleton'
import {
  Table,
  TableBody,
  TableCell,
  TableHead,
  TableHeader,
  TableRow,
} from '@/components/ui/table'
import { FilesApiError, type FileItem, type FileSort } from '@/api/files'
import { badgeText, getPreviewType, kindLabel } from '@/files/fileKind'
import { formatBytes, formatTime, useFiles } from '@/files/useFiles'

const route = useRoute()
const props = withDefaults(defineProps<{ embedded?: boolean }>(), { embedded: false })
const emit = defineEmits<{ close: [] }>()
const serial = computed(() => (route.query.serial as string) || '')
const backTo = computed(() => (serial.value ? { name: 'player', query: { serial: serial.value } } : '/'))
const f = useFiles(serial.value)

type ActionMode = 'folder' | 'rename' | 'move'
const actionOpen = ref(false)
const actionMode = ref<ActionMode>('folder')
const actionItem = ref<FileItem | null>(null)
const actionValue = ref('')
const actionBusy = ref(false)

const deleteOpen = ref(false)
const deletePaths = ref<string[]>([])

const conflictOpen = ref(false)
const conflictBusy = ref(false)

const previewOpen = ref(false)
const previewItem = ref<FileItem | null>(null)
const previewText = ref('')
const searchText = ref('')

const actionTitle = computed(() => ({ folder: '新建文件夹', rename: '重命名', move: '移动到' })[actionMode.value])
const actionLabel = computed(() => ({ folder: '文件夹名称', rename: '新名称', move: '目标文件夹路径' })[actionMode.value])
const storageText = computed(() => {
  const s = f.storage.value
  return s.total ? `已使用 ${formatBytes(s.used)} / ${formatBytes(s.total)}` : '容量信息不可用'
})
const emptyText = computed(() => (f.query.value || f.type.value !== 'all' ? '没有找到匹配的文件' : '这个文件夹还是空的'))
const uploadingCount = computed(() => f.uploads.value.filter(u => u.status === 'uploading').length)

function openAction(mode: ActionMode, item: FileItem | null = null) {
  actionMode.value = mode
  actionItem.value = item
  actionValue.value = mode === 'rename' && item ? item.name : mode === 'move' ? f.path.value : ''
  actionOpen.value = true
}

async function submitAction() {
  const value = actionValue.value.trim()
  if (!value)
    return
  actionBusy.value = true
  try {
    if (actionMode.value === 'folder') {
      await f.doCreateFolder(value)
      toast.success('文件夹已创建')
    }
    else if (actionMode.value === 'rename' && actionItem.value) {
      await f.doRename(actionItem.value, value)
      toast.success('重命名成功')
    }
    else if (actionMode.value === 'move') {
      const paths = actionItem.value ? [actionItem.value.path] : [...f.selected.value]
      await f.doMove(paths, value, 'fail')
      toast.success('移动成功')
    }
    actionOpen.value = false
  }
  catch (e) {
    if (e instanceof FilesApiError && e.code === 'conflict') {
      actionOpen.value = false
      conflictOpen.value = true
    }
    else {
      toast.error(e instanceof Error ? e.message : '操作失败')
    }
  }
  finally {
    actionBusy.value = false
  }
}

async function submitConflictKeepCopy() {
  const value = actionValue.value.trim()
  conflictBusy.value = true
  try {
    if (actionMode.value === 'rename' && actionItem.value) {
      await f.doRename(actionItem.value, value, 'rename')
    }
    else if (actionMode.value === 'move') {
      const paths = actionItem.value ? [actionItem.value.path] : [...f.selected.value]
      await f.doMove(paths, value, 'rename')
    }
    conflictOpen.value = false
    toast.success('已保留副本')
  }
  catch (e) {
    toast.error(e instanceof Error ? e.message : '操作失败')
  }
  finally {
    conflictBusy.value = false
  }
}

async function submitDelete() {
  deleteOpen.value = false
  try {
    const token = await f.doDelete(deletePaths.value)
    toast.success(`已删除 ${deletePaths.value.length} 项`, {
      action: {
        label: '撤销',
        onClick: async () => {
          try {
            await f.doUndo(token)
            toast.success('已撤销删除')
          }
          catch (e) {
            toast.error(e instanceof Error ? e.message : '撤销失败')
          }
        },
      },
      duration: 8000,
    })
  }
  catch (e) {
    toast.error(e instanceof Error ? e.message : '删除失败')
  }
  finally {
    deletePaths.value = []
  }
}

async function download(item: FileItem) {
  try {
    await f.doDownload(item)
    toast.success('开始下载')
  }
  catch (e) {
    toast.error(e instanceof Error ? e.message : '下载失败')
  }
}

function openPreview(item: FileItem) {
  if (!getPreviewType(item))
    return
  previewItem.value = item
  previewText.value = ''
  previewOpen.value = true
  if (getPreviewType(item) === 'text') {
    previewText.value = '加载中…'
    fetch(f.previewUrl(item))
      .then(async (res) => {
        if (!res.ok)
          throw new Error()
        return res.text()
      })
      .then((content) => {
        if (previewItem.value === item)
          previewText.value = content
      })
      .catch(() => {
        if (previewItem.value === item)
          previewText.value = '加载文件内容失败'
      })
  }
}

function onFilePicked(e: Event) {
  const input = e.target as HTMLInputElement
  ;[...(input.files || [])].forEach(file => f.startUpload(file))
  input.value = ''
}

function onUploadConflict(entryId: string, fileName: string) {
  toast.warning(`「${fileName}」已存在`, {
    action: { label: '保留副本', onClick: () => f.retryUploadWithRename(entryId) },
  })
}

function onSearch() {
  f.search(searchText.value)
}

function goPath(p: string) {
  searchText.value = ''
  f.setPath(p)
}

function orderLabel(): string {
  const sortName = { name: '名称', modified: '修改时间', size: '大小' }[f.sort.value]
  return `${sortName}${f.order.value === 'asc' ? '升序' : '降序'}`
}

onMounted(() => {
  if (serial.value)
    void f.load()
  else
    f.error.value = '未指定手机，请从投屏页面打开文件管理'
})
</script>

<template>
  <div class="flex flex-col" :class="props.embedded ? 'h-full min-h-0' : 'min-h-screen'">
    <header v-if="props.embedded" class="flex h-11 shrink-0 items-center gap-2 border-b px-3">
      <FolderIcon class="h-4 w-4" /><strong class="text-sm">文件管理</strong>
    </header>
    <header v-if="!props.embedded" class="sticky top-0 z-10 border-b bg-background/80 backdrop-blur">
      <div class="mx-auto flex h-14 max-w-6xl items-center gap-3 px-4">
        <Button variant="ghost" size="icon" aria-label="返回" as-child>
          <RouterLink :to="backTo"><ArrowLeft class="h-5 w-5" /></RouterLink>
        </Button>
        <span class="text-base font-semibold">文件管理</span>
        <span v-if="serial" class="max-w-48 truncate font-mono text-xs text-muted-foreground" :title="serial">{{ serial }}</span>
        <span class="ml-auto text-xs text-muted-foreground" aria-live="polite">{{ storageText }}</span>
      </div>
    </header>

    <main class="w-full max-w-6xl flex-1 overflow-auto px-4 py-4" :class="props.embedded ? 'max-w-none' : 'mx-auto'">
      <!-- 路径 + 主要操作 -->
      <div class="mb-3 flex flex-wrap items-center gap-2">
        <Breadcrumb class="min-w-0 flex-1">
          <BreadcrumbList>
            <template v-for="(crumb, i) in f.breadcrumbs.value" :key="crumb.path">
              <BreadcrumbItem>
                <BreadcrumbLink v-if="i < f.breadcrumbs.value.length - 1" as-child>
                  <button type="button" @click="goPath(crumb.path)">{{ crumb.name }}</button>
                </BreadcrumbLink>
                <BreadcrumbPage v-else>{{ crumb.name }}</BreadcrumbPage>
              </BreadcrumbItem>
              <BreadcrumbSeparator v-if="i < f.breadcrumbs.value.length - 1" />
            </template>
          </BreadcrumbList>
        </Breadcrumb>
        <Button as-child>
          <label for="file-input" class="cursor-pointer"><FileUp class="h-4 w-4" />上传</label>
        </Button>
        <input id="file-input" type="file" multiple class="hidden" @change="onFilePicked" />
        <Button variant="secondary" @click="openAction('folder')"><FolderPlus class="h-4 w-4" />新建文件夹</Button>
      </div>

      <!-- 搜索筛选 -->
      <div class="mb-3 flex flex-wrap items-center gap-2">
        <form class="relative max-w-xs flex-1" role="search" @submit.prevent="onSearch">
          <Search class="absolute left-2.5 top-1/2 h-4 w-4 -translate-y-1/2 text-muted-foreground" />
          <Input v-model="searchText" type="search" placeholder="搜索文件和文件夹" aria-label="搜索文件和文件夹" class="pl-8" autocomplete="off" />
        </form>
        <Select v-model="f.type.value" @update:model-value="f.load()">
          <SelectTrigger class="w-28" aria-label="类型筛选"><SelectValue /></SelectTrigger>
          <SelectContent>
            <SelectItem value="all">全部</SelectItem>
            <SelectItem value="folder">文件夹</SelectItem>
            <SelectItem value="image">图片</SelectItem>
            <SelectItem value="video">视频</SelectItem>
            <SelectItem value="document">文档</SelectItem>
            <SelectItem value="audio">音频</SelectItem>
            <SelectItem value="other">其他</SelectItem>
          </SelectContent>
        </Select>
        <Select v-model="f.sort.value as FileSort" @update:model-value="f.load()">
          <SelectTrigger class="w-28" aria-label="排序字段"><SelectValue /></SelectTrigger>
          <SelectContent>
            <SelectItem value="name">名称</SelectItem>
            <SelectItem value="modified">修改时间</SelectItem>
            <SelectItem value="size">大小</SelectItem>
          </SelectContent>
        </Select>
        <Button
          variant="outline" size="sm" :title="`当前${orderLabel()}，点击切换`"
          @click="f.order.value = f.order.value === 'asc' ? 'desc' : 'asc'; f.load()"
        >
          <ArrowDownWideNarrow class="h-4 w-4" />{{ orderLabel() }}
        </Button>
        <span v-if="f.total.value" class="text-xs text-muted-foreground">{{ f.total.value }} 项</span>
      </div>

      <!-- 批量条 -->
      <div v-if="f.selectedCount.value > 0" class="mb-3 flex items-center gap-2 rounded-lg border bg-muted/50 px-3 py-2">
        <span class="text-sm">已选择 {{ f.selectedCount.value }} 项</span>
        <Button variant="ghost" size="sm" @click="f.toggleSelectAll()">{{ f.allSelected.value ? '取消全选' : '选择全部' }}</Button>
        <Button variant="ghost" size="sm" @click="openAction('move')"><FolderInput class="h-4 w-4" />移动到</Button>
        <Button
          variant="ghost" size="sm" class="text-destructive"
          @click="deletePaths = [...f.selected.value]; deleteOpen = true"
        >
          <Trash2 class="h-4 w-4" />删除
        </Button>
      </div>

      <!-- 内容区 -->
      <div v-if="f.loading.value" class="space-y-2" aria-label="正在加载文件">
        <Skeleton v-for="i in 8" :key="i" class="h-12 w-full rounded-md" />
      </div>

      <Empty v-else-if="f.error.value">
        <EmptyHeader>
          <EmptyMedia variant="icon"><TriangleAlert /></EmptyMedia>
          <EmptyTitle>文件加载失败</EmptyTitle>
          <EmptyDescription>{{ f.error.value }}</EmptyDescription>
        </EmptyHeader>
        <Button variant="outline" @click="f.load()">重试</Button>
      </Empty>

      <Empty v-else-if="f.items.value.length === 0">
        <EmptyHeader>
          <EmptyMedia variant="icon"><FolderIcon /></EmptyMedia>
          <EmptyTitle>{{ emptyText }}</EmptyTitle>
        </EmptyHeader>
        <Button as-child><label for="file-input" class="cursor-pointer">上传文件</label></Button>
      </Empty>

      <div v-else class="overflow-x-auto rounded-lg border">
        <Table>
          <TableHeader>
            <TableRow>
              <TableHead class="w-10"><span class="sr-only">选择</span></TableHead>
              <TableHead>名称</TableHead>
              <TableHead>类型</TableHead>
              <TableHead class="text-right">大小</TableHead>
              <TableHead>修改时间</TableHead>
              <TableHead class="text-right">操作</TableHead>
            </TableRow>
          </TableHeader>
          <TableBody>
            <TableRow v-for="item in f.items.value" :key="item.path">
              <TableCell>
                <Checkbox
                  :model-value="f.selected.value.has(item.path)"
                  :aria-label="`选择 ${item.name}`"
                  @update:model-value="f.toggleSelect(item.path, $event === true)"
                />
              </TableCell>
              <TableCell>
                <div class="flex items-center gap-2">
                  <FolderIcon v-if="item.kind === 'folder'" class="h-5 w-5 shrink-0 text-primary" />
                  <Badge v-else variant="secondary" class="shrink-0 font-mono text-[10px]">{{ badgeText(item) }}</Badge>
                  <button
                    type="button" class="truncate text-left font-medium hover:underline"
                    @click="item.kind === 'folder' ? f.setPath(item.path) : openPreview(item)"
                  >
                    {{ item.name }}
                  </button>
                </div>
                <p v-if="f.query.value" class="truncate font-mono text-[11px] text-muted-foreground">{{ item.path }}</p>
              </TableCell>
              <TableCell class="whitespace-nowrap text-muted-foreground">{{ kindLabel(item) }}</TableCell>
              <TableCell class="whitespace-nowrap text-right tabular-nums">{{ item.kind === 'folder' ? '—' : formatBytes(item.size) }}</TableCell>
              <TableCell class="whitespace-nowrap text-muted-foreground">{{ formatTime(item.modified) }}</TableCell>
              <TableCell>
                <div class="flex justify-end gap-0.5">
                  <Button v-if="getPreviewType(item)" variant="ghost" size="icon" class="h-8 w-8" title="预览" :aria-label="`预览 ${item.name}`" @click="openPreview(item)">
                    <Eye class="h-4 w-4" />
                  </Button>
                  <Button v-if="item.kind !== 'folder'" variant="ghost" size="icon" class="h-8 w-8" title="下载" :aria-label="`下载 ${item.name}`" @click="download(item)">
                    <Download class="h-4 w-4" />
                  </Button>
                  <Button variant="ghost" size="icon" class="h-8 w-8" title="重命名" :aria-label="`重命名 ${item.name}`" @click="openAction('rename', item)">
                    <Pencil class="h-4 w-4" />
                  </Button>
                  <Button variant="ghost" size="icon" class="h-8 w-8" title="移动到" :aria-label="`移动 ${item.name}`" @click="openAction('move', item)">
                    <FolderInput class="h-4 w-4" />
                  </Button>
                  <Button variant="ghost" size="icon" class="h-8 w-8 text-destructive" title="删除" :aria-label="`删除 ${item.name}`" @click="deletePaths = [item.path]; deleteOpen = true">
                    <Trash2 class="h-4 w-4" />
                  </Button>
                </div>
              </TableCell>
            </TableRow>
          </TableBody>
        </Table>
      </div>
      <div v-if="f.hasMore.value" class="mt-3 flex items-center gap-3">
        <Button variant="outline" @click="f.load(true)">加载更多</Button>
        <span class="text-xs text-muted-foreground">已显示 {{ f.items.value.length }} / {{ f.total.value }} 项</span>
      </div>
    </main>

    <!-- 上传队列 -->
    <div v-if="f.uploads.value.length > 0" class="fixed bottom-4 right-4 w-80 rounded-xl border bg-card p-3 shadow-lg" role="status" aria-label="上传队列">
      <div class="mb-2 flex items-center gap-2">
        <strong class="text-sm">上传队列{{ uploadingCount ? `（${uploadingCount} 项上传中）` : '' }}</strong>
        <Button variant="ghost" size="sm" class="ml-auto" @click="f.clearFinishedUploads()">清除已完成</Button>
      </div>
      <ul class="max-h-64 space-y-2 overflow-auto">
        <li v-for="u in f.uploads.value" :key="u.id" class="rounded-md border p-2">
          <p class="truncate text-xs font-medium">{{ u.file.name }}</p>
          <Progress :model-value="u.status === 'done' ? 100 : u.progress" class="my-1.5 h-1.5" />
          <div class="flex items-center gap-2">
            <span class="text-[11px] text-muted-foreground">
              {{ u.status === 'done' ? '上传完成' : u.status === 'cancelled' ? '已取消' : u.status === 'error' ? (u.error || '上传失败') : `${u.progress}%` }}
            </span>
            <Button
              v-if="u.status === 'error' && u.error !== 'conflict'" variant="ghost" size="sm" class="ml-auto h-6 text-xs"
              @click="u.retry()"
            >
              重试
            </Button>
            <Button
              v-if="u.status === 'error' && u.error === 'conflict'" variant="ghost" size="sm" class="ml-auto h-6 text-xs"
              @click="onUploadConflict(u.id, u.file.name)"
            >
              已存在？
            </Button>
            <Button v-if="u.status === 'uploading'" variant="ghost" size="sm" class="ml-auto h-6 text-xs" @click="u.cancel()">取消</Button>
          </div>
        </li>
      </ul>
    </div>

    <!-- 新建/重命名/移动 -->
    <Dialog v-model:open="actionOpen">
      <DialogContent>
        <DialogHeader>
          <DialogTitle>{{ actionTitle }}</DialogTitle>
          <DialogDescription>{{ actionItem ? `当前对象：${actionItem.name}` : `将在「${f.path.value || '根目录'}」中创建` }}</DialogDescription>
        </DialogHeader>
        <div class="space-y-2">
          <label class="text-sm font-medium" for="action-input">{{ actionLabel }}</label>
          <Input
            id="action-input" v-model="actionValue" autocomplete="off"
            :placeholder="actionMode === 'move' ? '例如：图片/2026（留空表示根目录）' : ''"
            @keydown.enter.prevent="submitAction"
          />
        </div>
        <DialogFooter>
          <Button variant="ghost" @click="actionOpen = false">取消</Button>
          <Button :disabled="actionBusy || !actionValue.trim()" @click="submitAction">
            {{ actionBusy ? '处理中…' : '确定' }}
          </Button>
        </DialogFooter>
      </DialogContent>
    </Dialog>

    <!-- 删除确认 -->
    <AlertDialog v-model:open="deleteOpen">
      <AlertDialogContent>
        <AlertDialogHeader>
          <AlertDialogTitle>删除 {{ deletePaths.length }} 项？</AlertDialogTitle>
          <AlertDialogDescription>删除后可在本次会话中撤销（8 秒内）。</AlertDialogDescription>
        </AlertDialogHeader>
        <AlertDialogFooter>
          <AlertDialogCancel>取消</AlertDialogCancel>
          <AlertDialogAction class="bg-destructive text-destructive-foreground hover:bg-destructive/90" @click="submitDelete">
            删除
          </AlertDialogAction>
        </AlertDialogFooter>
      </AlertDialogContent>
    </AlertDialog>

    <!-- 冲突：保留副本 -->
    <AlertDialog v-model:open="conflictOpen">
      <AlertDialogContent>
        <AlertDialogHeader>
          <AlertDialogTitle>目标位置已有同名对象</AlertDialogTitle>
          <AlertDialogDescription>是否自动保留副本继续操作？</AlertDialogDescription>
        </AlertDialogHeader>
        <AlertDialogFooter>
          <AlertDialogCancel>取消</AlertDialogCancel>
          <AlertDialogAction :disabled="conflictBusy" @click="submitConflictKeepCopy">
            {{ conflictBusy ? '处理中…' : '保留副本' }}
          </AlertDialogAction>
        </AlertDialogFooter>
      </AlertDialogContent>
    </AlertDialog>

    <!-- 预览 -->
    <Dialog v-model:open="previewOpen">
      <DialogContent class="max-w-3xl">
        <DialogHeader>
          <DialogTitle class="truncate">{{ previewItem?.name }}</DialogTitle>
          <DialogDescription class="sr-only">文件内容预览</DialogDescription>
        </DialogHeader>
        <div class="max-h-[70vh] overflow-auto">
          <img v-if="previewItem && getPreviewType(previewItem) === 'image'" :src="f.previewUrl(previewItem)" :alt="previewItem.name" class="mx-auto max-w-full" />
          <video v-else-if="previewItem && getPreviewType(previewItem) === 'video'" :src="f.previewUrl(previewItem)" controls autoplay class="mx-auto max-w-full" />
          <pre v-else class="max-h-[60vh] overflow-auto rounded-md bg-muted p-3 text-xs">{{ previewText }}</pre>
        </div>
        <DialogFooter>
          <Button variant="outline" @click="previewItem && download(previewItem)">下载</Button>
        </DialogFooter>
      </DialogContent>
    </Dialog>
  </div>
</template>
