// 截图接口：整图为画布 PNG 上传；查看/下载走同一 viewUrl。

export interface ScreenshotEntry {
  screenshot_id: string
  created_at: string
  width?: number
  height?: number
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

export function listScreenshots(serial: string): Promise<{ screenshots: ScreenshotEntry[] }> {
  return request(`/api/screenshots${qs(serial)}`)
}

export function uploadScreenshot(serial: string, blob: Blob): Promise<ScreenshotEntry> {
  return request(`/api/screenshots${qs(serial)}`, { method: 'POST', body: blob })
}

export function deleteScreenshot(serial: string, id: string): Promise<unknown> {
  const res = fetch(`/api/screenshots/${encodeURIComponent(id)}${qs(serial)}`, { method: 'DELETE' })
  return res.then(async (r) => {
    if (!r.ok && r.status !== 204)
      throw new Error(`HTTP ${r.status}`)
  })
}

export function screenshotViewUrl(serial: string, id: string): string {
  return `/api/screenshots/${encodeURIComponent(id)}${qs(serial)}`
}

export function screenshotDownloadUrl(serial: string, id: string): string {
  return `${screenshotViewUrl(serial, id)}&download=1`
}

export function canvasToBlob(canvas: HTMLCanvasElement): Promise<Blob | null> {
  return new Promise(resolve => canvas.toBlob(resolve, 'image/png'))
}
