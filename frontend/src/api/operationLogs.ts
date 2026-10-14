export interface OperationLog {
  id: string
  serial: string
  type: 'tap' | 'longpress' | 'swipe' | 'key' | 'text'
  description: string
  adb_command: string
  occurred_at: number
}

async function request<T>(init: RequestInit = {}): Promise<T> {
  const response = await fetch('/api/operation-logs' + (init.method ? '' : '?limit=500'), {
    ...init,
    signal: AbortSignal.timeout(10000),
  })
  if (!response.ok) {
    const error = new Error(`HTTP ${response.status}`) as Error & { status: number }
    error.status = response.status
    throw error
  }
  return response.json() as Promise<T>
}

export function appendOperationLogs(items: OperationLog[]) {
  return request<{ saved: number }>({ method: 'POST', headers: { 'Content-Type': 'application/json' }, body: JSON.stringify({ items }) })
}

export function listOperationLogs() {
  return request<{ items: OperationLog[], next_before: number }>()
}

export function deleteOperationLogs() {
  return request<{ deleted: number }>({ method: 'DELETE' })
}
