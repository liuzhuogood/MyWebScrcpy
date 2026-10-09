// 文件管理接口：作用域均为 ?serial=；错误体 {error:{message,code}}。

export interface FileItem {
  path: string
  name: string
  kind: string
  mime?: string
  size?: number
  modified?: string
}

export interface Breadcrumb {
  name: string
  path: string
}

export interface FileListResponse {
  items: FileItem[]
  page: number
  has_more: boolean
  total: number
  breadcrumbs: Breadcrumb[]
  storage: { used?: number, total?: number }
}

export type FileType = 'all' | 'folder' | 'image' | 'video' | 'document' | 'audio' | 'other'
export type FileSort = 'name' | 'modified' | 'size'
export type SortOrder = 'asc' | 'desc'

export class FilesApiError extends Error {
  code?: string
  status?: number
  constructor(message: string, code?: string, status?: number) {
    super(message)
    this.code = code
    this.status = status
  }
}

function withSerial(endpoint: string, serial: string): string {
  const sep = endpoint.includes('?') ? '&' : '?'
  return `${endpoint}${sep}serial=${encodeURIComponent(serial)}`
}

async function request<T>(endpoint: string, serial: string, options: RequestInit = {}): Promise<T> {
  const headers = new Headers(options.headers || {})
  if (options.body && !(options.body instanceof FormData))
    headers.set('Content-Type', 'application/json')
  const res = await fetch(withSerial(endpoint, serial), { ...options, headers })
  let data: { error?: { message?: string, code?: string } } | null = null
  try {
    data = await res.json()
  }
  catch { /* 下载等非 JSON 响应 */ }
  if (!res.ok)
    throw new FilesApiError(data?.error?.message || '操作失败，请重试', data?.error?.code, res.status)
  return data as T
}

export interface ListParams {
  path: string
  q: string
  type: FileType
  sort: FileSort
  order: SortOrder
  page: number
  pageSize: number
  signal?: AbortSignal
}

export function listFiles(serial: string, p: ListParams): Promise<FileListResponse> {
  const params = new URLSearchParams({
    path: p.path,
    q: p.q,
    type: p.type === 'all' ? '' : p.type,
    sort: p.sort,
    order: p.order,
    page: String(p.page),
    page_size: String(p.pageSize),
  })
  return request<FileListResponse>(`/api/files?${params}`, serial, { signal: p.signal })
}

export function createFolder(serial: string, path: string, name: string): Promise<unknown> {
  return request('/api/files/folders', serial, { method: 'POST', body: JSON.stringify({ path, name }) })
}

export function renameFile(serial: string, path: string, name: string, conflict?: 'rename'): Promise<unknown> {
  return request('/api/files/rename', serial, {
    method: 'POST',
    body: JSON.stringify(conflict ? { path, name, conflict } : { path, name }),
  })
}

export function moveFile(serial: string, path: string, target: string, conflict?: 'fail' | 'rename'): Promise<unknown> {
  return request('/api/files/move', serial, {
    method: 'POST',
    body: JSON.stringify({ path, target, conflict: conflict ?? 'fail' }),
  })
}

export function deleteFiles(serial: string, paths: string[]): Promise<{ undo_token: string }> {
  return request('/api/files/delete', serial, { method: 'POST', body: JSON.stringify({ paths }) })
}

export function undoDelete(serial: string, token: string): Promise<unknown> {
  return request('/api/files/undo', serial, { method: 'POST', body: JSON.stringify({ token }) })
}

export function downloadUrl(serial: string, path: string, preview = false): string {
  return withSerial(`/api/files/download?path=${encodeURIComponent(path)}${preview ? '&preview=1' : ''}`, serial)
}

/** 下载文件到本地（a[download] 触发保存） */
export async function downloadFile(serial: string, item: FileItem): Promise<void> {
  const res = await fetch(downloadUrl(serial, item.path))
  if (!res.ok)
    throw new Error('下载失败，请重试')
  const blob = await res.blob()
  const url = URL.createObjectURL(blob)
  const a = document.createElement('a')
  a.href = url
  a.download = item.name
  document.body.append(a)
  a.click()
  a.remove()
  URL.revokeObjectURL(url)
}

export interface UploadTask {
  id: string
  file: File
  progress: number
  status: 'uploading' | 'done' | 'error' | 'cancelled'
  error?: string
  cancel: () => void
  retry: () => void
}

/** XHR 上传（fetch 无上传进度）；409 冲突时抛 FilesApiError(code=conflict)。 */
export function uploadFile(
  serial: string,
  dirPath: string,
  file: File,
  conflict: 'fail' | 'rename',
  onProgress: (percent: number) => void,
  onDone: (task: { ok: boolean, message?: string }) => void,
): () => void {
  const form = new FormData()
  form.append('path', dirPath)
  form.append('conflict', conflict)
  form.append('file', file, file.name)
  const xhr = new XMLHttpRequest()
  xhr.open('POST', withSerial('/api/files/upload', serial))
  xhr.upload.onprogress = (e) => {
    if (e.lengthComputable)
      onProgress(Math.round((e.loaded / e.total) * 100))
  }
  xhr.onload = () => {
    let data: { error?: { message?: string, code?: string } } = {}
    try {
      data = JSON.parse(xhr.responseText)
    }
    catch { /* 通用错误 */ }
    if (xhr.status === 409 && conflict === 'fail') {
      onDone({ ok: false, message: data?.error?.code === 'conflict' ? 'conflict' : (data?.error?.message || '文件已存在') })
    }
    else if (xhr.status >= 200 && xhr.status < 300) {
      onDone({ ok: true })
    }
    else {
      onDone({ ok: false, message: data?.error?.message || '上传失败，可重试' })
    }
  }
  xhr.onerror = () => onDone({ ok: false, message: '网络错误，可重试' })
  xhr.onabort = () => onDone({ ok: false, message: 'cancelled' })
  xhr.send(form)
  return () => xhr.abort()
}
