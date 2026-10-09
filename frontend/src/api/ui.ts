// UI 检查接口：XML 快照与页面信息。

export interface XmlSnapshot {
  xml: string
  captured_at: string
  display_size?: { width: number, height: number }
}

export interface PageInfo {
  package_name?: string
  activity?: string
  component?: string
  window_name?: string
  display?: { width: number, height: number, density_dpi?: number, rotation?: number }
  fetched_at?: string
}

async function getJson<T>(url: string): Promise<T> {
  const res = await fetch(url)
  let data: { message?: string } | null = null
  try {
    data = await res.json()
  }
  catch { /* 非 JSON */ }
  if (!res.ok)
    throw new Error(data?.message || '获取失败')
  return data as T
}

export function fetchXml(serial: string): Promise<XmlSnapshot> {
  return getJson<XmlSnapshot>(`/api/ui/xml?serial=${encodeURIComponent(serial)}`)
}

export function fetchPageInfo(serial: string): Promise<PageInfo> {
  return getJson<PageInfo>(`/api/ui/page-info?serial=${encodeURIComponent(serial)}`)
}
