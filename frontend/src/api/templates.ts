// 模板资产接口。Go 错误响应为 {error: string, message: string}。

export interface TemplateItem {
  id: string
  name: string
  serial: string
  threshold: number
  method?: string
  grayscale?: boolean
  scales?: number[]
  enabled: boolean
  width?: number
  height?: number
  image_count?: number
  created_at?: string
  updated_at?: string
}

export interface TemplateMatch {
  template_id?: string
  name?: string
  /** 命中框：归一化 0~1（vision 同款） */
  x: number
  y: number
  w: number
  h: number
  confidence?: number
  timestamp?: string | number
}

export type TemplateScope = 'all' | 'device' | 'global'

async function request<T>(input: string, init: RequestInit = {}): Promise<T> {
  const res = await fetch(input, init)
  let data: { message: string, error: string } | null = null
  try {
    data = await res.json()
  }
  catch { /* 非 JSON */ }
  if (!res.ok)
    throw new Error(typeof data?.message === 'string' ? data.message : `HTTP ${res.status}`)
  return data as T
}

export function listTemplates(serial: string): Promise<{ templates: TemplateItem[], total: number }> {
  return request(`/api/templates?serial=${encodeURIComponent(serial)}`)
}

export function updateTemplate(id: string, patch: Partial<Pick<TemplateItem, 'serial' | 'enabled' | 'threshold' | 'name'>>): Promise<TemplateItem> {
  return request(`/api/templates/${encodeURIComponent(id)}`, {
    method: 'PUT',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify(patch),
  })
}

export function deleteTemplate(id: string): Promise<unknown> {
  return request(`/api/templates/${encodeURIComponent(id)}`, { method: 'DELETE' })
}

export function createTemplate(form: FormData): Promise<TemplateItem> {
  return request('/api/templates', { method: 'POST', body: form })
}

export function addTemplateImage(id: string, image: File): Promise<unknown> {
  const form = new FormData()
  form.append('image', image)
  return request(`/api/templates/${encodeURIComponent(id)}/images`, { method: 'POST', body: form })
}

export function importTemplates(file: File, serial: string): Promise<{ imported?: number }> {
  const form = new FormData()
  form.append('file', file)
  form.append('serial', serial)
  return request('/api/templates/import', { method: 'POST', body: form })
}

export function exportUrl(serial: string, scope: TemplateScope): string {
  return `/api/templates/export?serial=${encodeURIComponent(serial)}&scope=${encodeURIComponent(scope)}`
}

export function templateImageUrl(id: string, updatedAt?: string): string {
  return `/api/templates/${encodeURIComponent(id)}/image?_t=${updatedAt || Date.now()}`
}

export function templateImageDownloadUrl(id: string): string {
  return `/api/templates/${encodeURIComponent(id)}/image?download=1`
}

export function getMatches(serial: string): Promise<{ count: number, matches: TemplateMatch[] }> {
  return request(`/api/templates/matches?serial=${encodeURIComponent(serial)}`)
}

/** 定位并点击：后端定位模板后按 mode 注入（sdk/adb）；body 契约以服务端为准 */
export function clickTemplate(body: { serial: string, template_id: string, mode?: 'sdk' | 'adb' }): Promise<unknown> {
  return request('/api/templates/click', {
    method: 'POST',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify(body),
  })
}
