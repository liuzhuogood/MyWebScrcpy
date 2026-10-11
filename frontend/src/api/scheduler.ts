export interface ScheduleEntry {
  script_id: number
  script_name: string
  cron_expr: string
  target_devices: string[]
  needs_device_configuration: boolean
  is_enabled: boolean
  next_run_at: number
  last_run_at: number
  last_status: 'completed' | 'killed' | 'success' | 'started' | 'partial' | 'failed' | 'skipped' | ''
}

async function request<T>(input: string, init: RequestInit = {}): Promise<T> {
  const res = await fetch(input, init)
  let data: { message?: string, error?: string } | null = null
  try {
    data = await res.json()
  } catch {
    // 非 JSON
  }
  if (!res.ok) {
    throw new Error(data?.message || data?.error || `HTTP ${res.status}`)
  }
  return data as T
}

export function listScheduleEntries(): Promise<{ entries: ScheduleEntry[], total: number }> {
  return request('/api/scheduler/entries')
}

export function toggleSchedule(scriptId: number, enabled: boolean): Promise<{ ok: boolean, script_id: number, is_enabled: boolean }> {
  return request(`/api/md-scripts/${scriptId}/toggle-schedule`, {
    method: 'POST',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify({ enabled }),
  })
}
