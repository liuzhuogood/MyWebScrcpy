import { computed, ref } from 'vue'
import {
  createFolder,
  deleteFiles,
  downloadFile,
  downloadUrl,
  listFiles,
  moveFile,
  renameFile,
  undoDelete,
  uploadFile,
  type Breadcrumb,
  type FileItem,
  type FileSort,
  type FileType,
  type SortOrder,
} from '@/api/files'

const PAGE_SIZE = 100

export interface UploadEntry {
  id: string
  file: File
  progress: number
  status: 'uploading' | 'done' | 'error' | 'cancelled'
  error?: string
  cancel: () => void
  retry: () => void
}

/** 文件管理状态：列表/选择/分页/上传队列；弹窗与 toast 由视图编排 */
export function useFiles(serial: string) {
  const path = ref('')
  const query = ref('')
  const type = ref<FileType>('all')
  const sort = ref<FileSort>('name')
  const order = ref<SortOrder>('asc')
  const items = ref<FileItem[]>([])
  const total = ref(0)
  const hasMore = ref(false)
  const page = ref(1)
  const breadcrumbs = ref<Breadcrumb[]>([])
  const storage = ref<{ used?: number, total?: number }>({})
  const loading = ref(true)
  const error = ref('')
  const selected = ref<Set<string>>(new Set())
  const uploads = ref<UploadEntry[]>([])
  let controller: AbortController | null = null

  const selectedCount = computed(() => selected.value.size)
  const allSelected = computed(() => items.value.length > 0 && items.value.every(i => selected.value.has(i.path)))

  async function load(append = false) {
    controller?.abort()
    controller = new AbortController()
    if (!append) {
      page.value = 1
      loading.value = true
      error.value = ''
    }
    try {
      const data = await listFiles(serial, {
        path: path.value,
        q: query.value,
        type: type.value,
        sort: sort.value,
        order: order.value,
        page: append ? page.value + 1 : 1,
        pageSize: PAGE_SIZE,
        signal: controller.signal,
      })
      page.value = data.page || page.value
      hasMore.value = Boolean(data.has_more)
      total.value = data.total || 0
      breadcrumbs.value = data.breadcrumbs || []
      storage.value = data.storage || {}
      items.value = append ? [...items.value, ...(data.items || [])] : (data.items || [])
      if (!append)
        selected.value = new Set()
    }
    catch (e) {
      if ((e as Error).name === 'AbortError')
        return
      if (!append)
        error.value = e instanceof Error ? e.message : '加载失败'
    }
    finally {
      loading.value = false
    }
  }

  function setPath(p: string) {
    path.value = p
    query.value = ''
    void load()
  }

  function search(q: string) {
    query.value = q.trim()
    path.value = ''
    void load()
  }

  function toggleSelect(p: string, on: boolean) {
    const next = new Set(selected.value)
    if (on)
      next.add(p)
    else next.delete(p)
    selected.value = next
  }

  function toggleSelectAll() {
    if (allSelected.value) {
      selected.value = new Set()
    }
    else {
      selected.value = new Set(items.value.map(i => i.path))
    }
  }

  function findItem(p: string): FileItem | undefined {
    return items.value.find(i => i.path === p)
  }

  async function doCreateFolder(name: string) {
    await createFolder(serial, path.value, name)
    await load()
  }

  async function doRename(item: FileItem, name: string, conflict?: 'rename') {
    await renameFile(serial, item.path, name, conflict)
    await load()
  }

  async function doMove(paths: string[], target: string, conflict?: 'fail' | 'rename') {
    for (const p of paths)
      await moveFile(serial, p, target, conflict)
    selected.value = new Set()
    await load()
  }

  async function doDelete(paths: string[]): Promise<string> {
    const res = await deleteFiles(serial, paths)
    selected.value = new Set()
    await load()
    return res.undo_token
  }

  async function doUndo(token: string) {
    await undoDelete(serial, token)
    await load()
  }

  async function doDownload(item: FileItem) {
    await downloadFile(serial, item)
  }

  function previewUrl(item: FileItem): string {
    return downloadUrl(serial, item.path, true)
  }

  function startUpload(file: File, conflict: 'fail' | 'rename' = 'fail') {
    const entry = ref<UploadEntry | null>(null)
    const id = `${Date.now()}-${Math.random().toString(36).slice(2)}`
    const cancelFn = uploadFile(
      serial,
      path.value,
      file,
      conflict,
      (progress) => {
        if (entry.value)
          entry.value.progress = progress
      },
      (result) => {
        const e = entry.value
        if (!e)
          return
        if (result.ok) {
          e.progress = 100
          e.status = 'done'
          void load()
        }
        else if (result.message === 'cancelled') {
          e.status = 'cancelled'
        }
        else {
          e.status = 'error'
          e.error = result.message
        }
      },
    )
    entry.value = {
      id,
      file,
      progress: 0,
      status: 'uploading',
      cancel: () => cancelFn(),
      retry: () => {
        const e = entry.value
        if (!e)
          return
        uploads.value = uploads.value.filter(u => u.id !== id)
        startUpload(file, 'fail')
      },
    }
    uploads.value = [...uploads.value, entry.value]
  }

  function retryUploadWithRename(id: string) {
    const target = uploads.value.find(u => u.id === id)
    if (!target)
      return
    uploads.value = uploads.value.filter(u => u.id !== id)
    startUpload(target.file, 'rename')
  }

  function clearFinishedUploads() {
    uploads.value = uploads.value.filter(u => u.status === 'uploading' || u.status === 'error')
  }

  return {
    path, query, type, sort, order,
    items, total, hasMore, breadcrumbs, storage,
    loading, error, selected, selectedCount, allSelected, uploads,
    load, setPath, search, toggleSelect, toggleSelectAll, findItem,
    doCreateFolder, doRename, doMove, doDelete, doUndo, doDownload,
    previewUrl, startUpload, retryUploadWithRename, clearFinishedUploads,
  }
}

export function formatBytes(value?: number): string {
  if (!value)
    return '—'
  const units = ['B', 'KB', 'MB', 'GB', 'TB']
  let size = value
  let index = 0
  while (size >= 1024 && index < units.length - 1) {
    size /= 1024
    index += 1
  }
  return `${size >= 10 || index === 0 ? Math.round(size) : size.toFixed(1)} ${units[index]}`
}

export function formatTime(value?: string): string {
  if (!value)
    return '—'
  const date = new Date(value)
  return Number.isNaN(date.valueOf()) ? value : date.toLocaleString('zh-CN', { dateStyle: 'short', timeStyle: 'short' })
}
