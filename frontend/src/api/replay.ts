// 回放接口：服务端切换视频源，前端本地计时画进度。

export interface ReplayView {
  recording_id: string
  started_at: string
  loop?: boolean
  loop_count?: number
  paused?: boolean
  segment_on?: boolean
  segment_start_ms?: number
  segment_len_ms?: number
}

async function request<T>(input: string, init: RequestInit = {}): Promise<T> {
  const res = await fetch(input, init)
  let data: { message: string, error: string } | null = null
  try {
    data = await res.json()
  }
  catch { /* 非 JSON */ }
  if (!res.ok) {
    const e = new Error(typeof data?.message === 'string' ? data.message : `HTTP ${res.status}`) as Error & { status?: number }
    e.status = res.status
    throw e
  }
  return data as T
}

const qs = (serial: string) => `?serial=${encodeURIComponent(serial)}`

export function getReplay(serial: string): Promise<{ replay: ReplayView | null }> {
  return request(`/api/replay${qs(serial)}`)
}

export function startReplay(serial: string, recordingId: string, loop: boolean): Promise<ReplayView> {
  return request(`/api/replay/start${qs(serial)}`, {
    method: 'POST',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify({ recording_id: recordingId, loop }),
  })
}

export function stopReplay(serial: string): Promise<unknown> {
  return request(`/api/replay/stop${qs(serial)}`, { method: 'POST' })
}

export function pauseReplay(serial: string, paused: boolean): Promise<unknown> {
  return request(`/api/replay/pause${qs(serial)}`, {
    method: 'POST',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify({ paused }),
  })
}

export function seekReplay(serial: string, positionMs: number): Promise<unknown> {
  return request(`/api/replay/seek${qs(serial)}`, {
    method: 'POST',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify({ position_ms: Math.round(positionMs) }),
  })
}

export function setReplaySegment(serial: string, opts: { enabled: boolean, start_ms?: number, len_ms?: number }): Promise<unknown> {
  return request(`/api/replay/segment${qs(serial)}`, {
    method: 'POST',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify(opts),
  })
}

export function formatMs(ms: number | null): string {
  if (ms == null || !Number.isFinite(ms) || ms < 0)
    return '--'
  const s = Math.floor(ms / 1000)
  return `${String(Math.floor(s / 60)).padStart(2, '0')}:${String(s % 60).padStart(2, '0')}`
}
