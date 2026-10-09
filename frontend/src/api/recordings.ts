// 受控录屏接口：复用 H.264 共享流，不中断投屏。

export interface RecordingEntry {
  recording_id: string
  status: 'recording' | 'stopping' | 'completed' | 'failed'
  started_at: string
  ended_at?: string
  record_audio?: boolean
}

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

const qs = (serial: string) => `?serial=${encodeURIComponent(serial)}`

export function listRecordings(serial: string): Promise<{ recordings: RecordingEntry[] }> {
  return request(`/api/recordings${qs(serial)}`)
}

export function getRecording(serial: string, id: string): Promise<RecordingEntry> {
  return request(`/api/recordings/${encodeURIComponent(id)}${qs(serial)}`)
}

export function startRecording(serial: string, maxDurationMs: number, recordAudio: boolean): Promise<RecordingEntry> {
  return request(`/api/recordings${qs(serial)}`, {
    method: 'POST',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify({ max_duration_ms: maxDurationMs, record_audio: recordAudio }),
  })
}

export function stopRecording(serial: string, id: string): Promise<RecordingEntry> {
  return request(`/api/recordings/${encodeURIComponent(id)}/stop${qs(serial)}`, { method: 'POST' })
}

export function deleteRecording(serial: string, id: string): Promise<unknown> {
  return request(`/api/recordings/${encodeURIComponent(id)}${qs(serial)}`, { method: 'DELETE' })
}

export function recordingDownloadUrl(serial: string, id: string): string {
  return `/api/recordings/download${qs(serial)}&recording_id=${encodeURIComponent(id)}`
}

export function formatRecordingDuration(entry: RecordingEntry): string {
  const end = entry.ended_at ? new Date(entry.ended_at) : new Date()
  const seconds = Math.max(0, Math.round((end.getTime() - new Date(entry.started_at).getTime()) / 1000))
  const minutes = Math.floor(seconds / 60)
  return minutes ? `${minutes} 分${String(seconds % 60).padStart(2, '0')} 秒` : `${seconds} 秒`
}

export function formatRecordingTime(entry: RecordingEntry): string {
  try {
    return new Intl.DateTimeFormat('zh-CN', { dateStyle: 'short', timeStyle: 'medium' }).format(new Date(entry.started_at))
  }
  catch {
    return entry.recording_id
  }
}

export function formatMinutes(minutes: number): string {
  return minutes >= 60
    ? `${Math.floor(minutes / 60)} 小时${minutes % 60 ? ` ${minutes % 60} 分钟` : ''}`
    : `${minutes} 分钟`
}
