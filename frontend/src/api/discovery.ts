export interface DiscoveryJob {
  id: string
  kind: 'scan' | 'pair'
  state: 'running' | 'waiting' | 'completed' | 'cancelled' | 'failed'
  tried: number
  total: number
  found: string[]
  error?: string
  qr?: string
}

async function request(path: string, init: RequestInit, signal?: AbortSignal): Promise<DiscoveryJob> {
  const response = await fetch(`/api/device-discovery/${path}`, { ...init, signal })
  if (!response.ok)
    throw new Error((await response.text()).trim() || `设备发现失败（${response.status}）`)
  return response.json()
}
export const startPairing = (signal: AbortSignal) => request('pair', { method: 'POST' }, signal)
export const startScan = (body: { range: string, ports: number[], continuous: boolean, interval_seconds: number }, signal: AbortSignal) =>
  request('scan', { method: 'POST', headers: { 'Content-Type': 'application/json' }, body: JSON.stringify(body) }, signal)
export const pollDiscovery = (id: string, signal: AbortSignal) => request(`jobs/${encodeURIComponent(id)}`, {}, signal)
export const stopDiscovery = (id: string) => request(`jobs/${encodeURIComponent(id)}`, { method: 'DELETE', keepalive: true })
