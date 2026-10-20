export interface Device {
  serial: string
  state: string
  source?: 'auto' | 'manual'
  alias?: string
  product?: string
  model?: string
  device?: string
  transport_id?: string
  auto_reconnect?: boolean
  brand?: string
  manufacturer?: string
  android_release?: string
  sdk_version?: number
  cpu_abi?: string
  build_display_id?: string
  battery_level?: number
  battery_temp?: number
  battery_charging?: boolean
  cpu_temp?: number
  cpu_temp_supported?: boolean
  telemetry_updated_at?: number
}

export async function fetchDevices(): Promise<Device[]> {
  const res = await fetch('/api/devices')
  if (!res.ok)
    throw new Error((await res.text()).trim() || `获取设备列表失败（${res.status}）`)
  return res.json()
}

export async function addManualDevice(serial: string, alias?: string): Promise<Device> {
  const res = await fetch('/api/devices', {
    method: 'POST',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify({ serial, alias: alias || '' }),
  })
  if (!res.ok) {
    const txt = await res.text()
    throw new Error(txt || `添加设备失败（${res.status}）`)
  }
  return res.json()
}

export async function removeManualDevice(serial: string): Promise<void> {
  const res = await fetch(`/api/devices/${encodeURIComponent(serial)}`, {
    method: 'DELETE',
  })
  if (!res.ok) {
    const txt = await res.text()
    throw new Error(txt || `删除设备失败（${res.status}）`)
  }
}

export async function updateDeviceAlias(serial: string, alias: string): Promise<void> {
  const res = await fetch(`/api/devices/${encodeURIComponent(serial)}`, {
    method: 'PATCH',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify({ alias }),
  })
  if (!res.ok) {
    const txt = await res.text()
    throw new Error(txt || `更新别名失败（${res.status}）`)
  }
}

export async function reconnectDevice(serial: string): Promise<void> {
  const res = await fetch(`/api/devices/${encodeURIComponent(serial)}/connect`, {
    method: 'POST',
  })
  if (!res.ok) {
    const txt = await res.text()
    throw new Error(txt || `连接设备失败（${res.status}）`)
  }
}
