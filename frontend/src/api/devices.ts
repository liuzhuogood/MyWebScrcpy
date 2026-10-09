export interface Device {
  serial: string
  state: string
  product?: string
  model?: string
  device?: string
  transport_id?: string
}

export async function fetchDevices(): Promise<Device[]> {
  const res = await fetch('/api/devices')
  if (!res.ok)
    throw new Error(`获取设备列表失败（${res.status}）`)
  return res.json()
}
