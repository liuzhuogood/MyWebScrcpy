export interface MarkdownScript {
  id: number
  name: string
  target_devices: string[]
  needs_device_configuration: boolean
  content: string
  extracted_version: string
  schedule_cron: string
  is_enabled: number
  timeout_sec: number
  compiled_version: string
  compiled_python: string
  compiled_at: number
  last_run_at: number
  last_run_status: string
  created_at: number
  updated_at: number
}

export interface ParsedTemplateRef {
  alt: string
  id: string
  threshold: number
}

export interface ParsedMeta {
  name: string
  extracted_version: string
  is_fallback_ver: boolean
  schedule_cron: string
  timeout_sec: number
  is_enabled: boolean
  templates: ParsedTemplateRef[]
  content_hash: string
}

export interface RunningTask {
  id: string
  script_id: number
  script_name: string
  device_serial: string
  pid: number
  status: 'running' | 'stopping' | 'completed' | 'failed' | 'killed' | 'skipped'
  exit_code: number
  error_msg: string
  started_at: number
  ended_at: number
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

// 脚本 CRUD
export function listMarkdownScripts(): Promise<{ scripts: MarkdownScript[], total: number }> {
  return request('/api/md-scripts')
}

export function getMarkdownScript(id: number): Promise<{ script: MarkdownScript, meta: ParsedMeta, should_recompile: boolean }> {
  return request(`/api/md-scripts/${id}`)
}

export function createMarkdownScript(name: string, target_devices: string[], content: string): Promise<{ ok: boolean, script: MarkdownScript, meta: ParsedMeta }> {
  return request('/api/md-scripts', {
    method: 'POST',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify({ name, target_devices, content }),
  })
}

export function updateMarkdownScript(id: number, name: string, target_devices: string[], content: string): Promise<{ ok: boolean, script: MarkdownScript, meta: ParsedMeta }> {
  return request(`/api/md-scripts/${id}`, {
    method: 'PUT',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify({ name, target_devices, content }),
  })
}

export function deleteMarkdownScript(id: number): Promise<{ ok: boolean }> {
  return request(`/api/md-scripts/${id}`, { method: 'DELETE' })
}

export function compileMarkdownScript(id: number): Promise<{ ok: boolean, script: MarkdownScript, compiled_version: string, compiled_python: string }> {
  return request(`/api/md-scripts/${id}/compile`, { method: 'POST' })
}

export interface DeviceRunResult {
  device_serial: string
  status: 'started' | 'skipped' | 'failed'
  task?: RunningTask
  error: string
}

export function runMarkdownScript(id: number): Promise<{ ok: boolean, results: DeviceRunResult[], total: number }> {
  return request(`/api/md-scripts/${id}/run`, { method: 'POST' })
}

export function listScriptRuns(id: number): Promise<{ tasks: RunningTask[], total: number }> {
  return request(`/api/md-scripts/${id}/runs`)
}

// 任务管理器
export function listActiveTasks(): Promise<{ tasks: RunningTask[], total: number }> {
  return request('/api/tasks/active')
}

export function killTask(id: string): Promise<{ ok: boolean, message: string }> {
  return request(`/api/tasks/${encodeURIComponent(id)}/kill`, { method: 'POST' })
}
