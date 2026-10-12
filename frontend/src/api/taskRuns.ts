import type { RunningTask } from './mdScripts'
export interface RecordingOptions { enabled: boolean, record_audio: boolean }
export interface ReportOptions { enabled: boolean, prompt: string }
export interface ExecutionSnapshot { availability: 'available' | 'legacy_unavailable', script_name?: string, document_version?: string, document_hash?: string, target_devices?: string[], recording?: RecordingOptions, report?: ReportOptions }
export interface Dispatch { id: string, script_id: number, started_at: number, ended_at: number, trigger: 'manual' | 'schedule' | 'legacy_unknown', dispatch_state: 'dispatching' | 'finished', status: string, error_msg: string, snapshot: ExecutionSnapshot }
export interface TaskView extends RunningTask { dispatch_id: string, preparation_state: 'preparing' | 'ready' | 'failed' | 'finished' }
export interface LogState { availability: 'available' | 'unavailable' | 'expired', capture_state: 'waiting' | 'running' | 'completed' | 'degraded', truncated: boolean, last_seq: number, bytes: number, expires_at: number | null, error_msg: string }
export interface RecordingState { state: 'unavailable' | 'disabled' | 'not_started' | 'preparing' | 'recording' | 'stopping' | 'completed' | 'failed' | 'expired', recording_id: string | null, record_audio: boolean | null, coverage_complete: boolean | null, error_msg: string }
export interface ReportState { state: 'unavailable' | 'disabled' | 'waiting' | 'pending' | 'running' | 'completed' | 'failed', generation_revision: number, available_revision: number, can_retry: boolean, error_msg: string }
export interface TaskDetail { task: TaskView, artifacts: { log: LogState, recording: RecordingState, report: ReportState } }
export interface LogLine { task_id: string, seq: number, at_ms: number, stream: 'stdout' | 'stderr' | 'system', message: string, continued: boolean }
export interface LogPage { items: LogLine[], next_cursor: number, oldest_seq: number, latest_seq: number, has_more: boolean, complete: boolean, truncated: boolean, availability: LogState['availability'] }
export interface ReportResult { generation: ReportState, report: { markdown: string, citations: { seq: number }[] } | null, revision: number, input_hash: string, created_at: number, log_availability: LogState['availability'] }
async function request<T>(path: string, signal?: AbortSignal, method = 'GET'): Promise<T> {
  const response = await fetch(path, { signal, method })
  let data: { message: string }
  try { data = await response.json() }
  catch { throw new Error(response.ok ? '服务器响应无效，请重试' : `请求失败（${response.status}）`) }
  if (!response.ok) throw new Error(data.message)
  return data as T
}
const taskPath = (id: string) => `/api/tasks/${encodeURIComponent(id)}`
export const listDispatches = (id: number, cursor: string, signal?: AbortSignal) => request<{ dispatches: Dispatch[], next_cursor: string, has_more: boolean }>(`/api/md-scripts/${id}/dispatches${cursor ? `?cursor=${encodeURIComponent(cursor)}` : ''}`, signal)
export const getDispatch = (id: number, dispatch: string, signal?: AbortSignal) => request<{ dispatch: Dispatch, tasks: TaskView[] }>(`/api/md-scripts/${id}/dispatches/${encodeURIComponent(dispatch)}`, signal)
export const getTaskDetail = (id: string, signal?: AbortSignal) => request<TaskDetail>(taskPath(id), signal)
export const getTaskLogs = (id: string, query: URLSearchParams, signal?: AbortSignal) => request<LogPage>(`${taskPath(id)}/logs?${query}`, signal)
export const getTaskReport = (id: string, signal?: AbortSignal) => request<ReportResult>(`${taskPath(id)}/report`, signal)
export const retryTaskReport = (id: string, signal?: AbortSignal) => request<{ ok: boolean }>(`${taskPath(id)}/report/retry`, signal, 'POST')
export const recordingMedia = (id: string) => `${taskPath(id)}/recording/media`
export async function downloadArtifact(id: string, kind: 'logs' | 'recording' | 'report', revision?: number): Promise<void> {
  const path = kind === 'recording' ? recordingMedia(id) : `${taskPath(id)}/${kind}/download${revision ? `?revision=${revision}` : ''}`
  const response = await fetch(path)
  if (!response.ok) { const data = await response.json(); throw new Error(data.message) }
  const blob = await response.blob()
  const url = URL.createObjectURL(blob)
  const a = document.createElement('a'); a.href = url; a.download = `${id}-${kind}${revision ? `-${revision}` : ''}.${kind === 'logs' ? 'jsonl' : kind === 'recording' ? 'mp4' : 'md'}`; a.click()
  setTimeout(() => URL.revokeObjectURL(url), 1000)
}
