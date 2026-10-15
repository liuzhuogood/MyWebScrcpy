export interface TaskRunItem {
  id: string
  client_run_id: string
  name: string
  source: string
  device_serial: string
  status: 'running' | 'succeeded' | 'failed' | 'cancelled'
  exit_code: number | null
  summary: string
  report_markdown: string
  log_count: number
  log_truncated: boolean
  created_at: number
  updated_at: number
  last_heartbeat_at: number
  ended_at: number
  connectivity: 'connected' | 'disconnected' | 'closed'
  disconnected: boolean
  duration_ms: number
}

export interface TaskRunLogEntry {
  seq: number
  timestamp: number
  stream: 'stdout' | 'stderr'
  message: string
}

export interface TaskRunListResponse {
  items: TaskRunItem[]
  total: number
  page: number
  limit: number
}

export interface TaskRunLogsResponse {
  items: TaskRunLogEntry[]
  acknowledged_seq: number
  truncated: boolean
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

export async function fetchTaskRuns(params?: {
  page?: number
  limit?: number
  status?: string
  connectivity?: string
  device_serial?: string
  source?: string
  q?: string
}): Promise<TaskRunListResponse> {
  const query = new URLSearchParams()
  if (params?.page) query.set('page', String(params.page))
  if (params?.limit) query.set('limit', String(params.limit))
  if (params?.status) query.set('status', params.status)
  if (params?.connectivity) query.set('connectivity', params.connectivity)
  if (params?.device_serial) query.set('device_serial', params.device_serial)
  if (params?.source) query.set('source', params.source)
  if (params?.q) query.set('q', params.q)
  const qs = query.toString()
  return fetchJSON<TaskRunListResponse>(`/api/task-runs${qs ? `?${qs}` : ''}`)
}

export async function fetchTaskRunDetail(id: string): Promise<TaskRunItem> {
  return fetchJSON<TaskRunItem>(`/api/task-runs/${encodeURIComponent(id)}`)
}

export async function fetchTaskRunLogs(
  id: string,
  params?: { after?: number; limit?: number; stream?: string }
): Promise<TaskRunLogsResponse> {
  const query = new URLSearchParams()
  if (params?.after !== undefined) query.set('after', String(params.after))
  if (params?.limit !== undefined) query.set('limit', String(params.limit))
  if (params?.stream) query.set('stream', params.stream)
  const qs = query.toString()
  return fetchJSON<TaskRunLogsResponse>(`/api/task-runs/${encodeURIComponent(id)}/logs${qs ? `?${qs}` : ''}`)
}
