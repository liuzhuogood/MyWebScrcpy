export interface OutputDef {
  id: string
  category: string
  format: string
  path: string
  required: boolean
  allowed_types?: string[]
}

export interface TaskDef {
  name: string
  device_serial: string
  goal: string
  success_condition: string
  timeout_seconds: number
  source: string
  skill_dir: string
  outputs: OutputDef[]
}

export interface TaskState {
  status: string
  stage: string
  progress: number
  last_action: string
  last_result: string
  updated_at: number
  ended_reason?: string
}

export interface OutputCheckResult {
  id: string
  category: string
  format: string
  path: string
  required: boolean
  exists: boolean
  valid: boolean
  size_bytes?: number
  files?: string[]
  error?: string
}

export interface TaskSummary {
  id: string
  name: string
  device_serial: string
  source: string
  status: string
  business_result: string
  outputs_complete: boolean
  goal: string
  updated_at: number
  is_stale: boolean
  corrupted?: boolean
  error_message?: string
}

export interface ActionRecord {
  timestamp: number
  action: string
  result: string
}

export interface TaskDetail {
  id: string
  task_def: TaskDef
  state: TaskState
  outputs: OutputCheckResult[]
  outputs_complete: boolean
  is_stale: boolean
  report_content?: string
  recent_actions?: ActionRecord[]
  log_tail?: string
  corrupted?: boolean
  error_message?: string
}

export interface ScanResult {
  scanned_at: number
  count: number
  root_dir: string
  errors?: string[]
}

export interface TaskListResponse {
  items: TaskSummary[]
  total: number
  page: number
  limit: number
  scanned_at: number
  root_dir: string
}

async function fetchJSON<T>(url: string, init?: RequestInit): Promise<T> {
  const res = await fetch(url, init)
  if (!res.ok) {
    let msg = `HTTP ${res.status}`
    try {
      const data = await res.json()
      if (typeof data.error === 'string') msg = data.error
    } catch {
      // ignore
    }
    throw new Error(msg)
  }
  return res.json()
}

export async function scanTaskRuns(): Promise<ScanResult> {
  return fetchJSON<ScanResult>('/api/task-runs/scan', { method: 'POST' })
}

export async function fetchTaskRuns(params?: {
  page?: number
  limit?: number
  status?: string
  device_serial?: string
  source?: string
}): Promise<TaskListResponse> {
  const query = new URLSearchParams()
  if (params?.page) query.set('page', String(params.page))
  if (params?.limit) query.set('limit', String(params.limit))
  if (params?.status) query.set('status', params.status)
  if (params?.device_serial) query.set('device_serial', params.device_serial)
  if (params?.source) query.set('source', params.source)
  const qs = query.toString()
  return fetchJSON<TaskListResponse>(`/api/task-runs${qs ? `?${qs}` : ''}`)
}

export async function fetchTaskRunDetail(id: string): Promise<TaskDetail> {
  return fetchJSON<TaskDetail>(`/api/task-runs/${encodeURIComponent(id)}`)
}

export function getTaskOutputUrl(taskId: string, outputId: string, file?: string, download?: boolean): string {
  const base = `/api/task-runs/${encodeURIComponent(taskId)}/outputs/${encodeURIComponent(outputId)}`
  const params = new URLSearchParams()
  if (file) params.set('file', file)
  if (download) params.set('download', 'true')
  const qs = params.toString()
  return qs ? `${base}?${qs}` : base
}
