export interface OCRBox {
  text: string
  confidence: number
  x: number
  y: number
  w: number
  h: number
  pixel_x?: number
  pixel_y?: number
  pixel_w?: number
  pixel_h?: number
}

export interface OCRDetectResponse {
  status: 'matched' | 'not_found'
  matched: boolean
  text: string
  boxes: OCRBox[]
  engine: string
  elapsed_ms: number
  fallback_reason?: string[]
}

export interface OCRStatusResponse {
  indra_installed: boolean
  indra_running: boolean
  indra_ready: boolean
  indra_pid: number
  indra_port: number
  indraocr_url: string
  indra_status?: 'uninstalled' | 'installed' | 'installing' | 'starting' | 'running' | 'failed'
  indra_error?: string
  paddle_installed: boolean
  paddle_running: boolean
  paddle_ready: boolean
  paddle_pid: number
  paddle_port: number
  paddleocr_url: string
  macocr_url: string
  macocr_auth_set: boolean
  engine: string
  paddle_status?: 'uninstalled' | 'installed' | 'installing' | 'starting' | 'running' | 'failed'
  paddle_error?: string
}

async function request<T>(input: string, init: RequestInit = {}): Promise<T> {
  const res = await fetch(input, init)
  let data: any = null
  try {
    data = await res.json()
  }
  catch { /* 非 JSON */ }
  if (!res.ok) {
    const msg = data?.error || data?.message || `HTTP ${res.status}`
    throw new Error(msg)
  }
  return data as T
}

export function detectOCR(formData: FormData, signal?: AbortSignal): Promise<OCRDetectResponse> {
  return request('/api/vision/ocr', {
    method: 'POST',
    body: formData,
    signal,
  })
}

export function getOCRStatus(): Promise<OCRStatusResponse> {
  return request('/api/ocr/status')
}

export function startPaddleOCR(): Promise<OCRStatusResponse> {
  return request('/api/ocr/start', { method: 'POST', signal: AbortSignal.timeout(17 * 60 * 1000) })
}

export function startIndraOCR(): Promise<OCRStatusResponse> {
  return request('/api/ocr/indra/start', { method: 'POST', signal: AbortSignal.timeout(17 * 60 * 1000) })
}

export function stopIndraOCR(): Promise<OCRStatusResponse> {
  return request('/api/ocr/indra/stop', { method: 'POST' })
}

export function stopPaddleOCR(): Promise<OCRStatusResponse> {
  return request('/api/ocr/stop', { method: 'POST' })
}

export function saveOCRConfig(payload: {
  macocr_url?: string
  macocr_auth?: string
  paddleocr_url?: string
  indraocr_url?: string
  engine?: string
}): Promise<OCRStatusResponse> {
  return request('/api/ocr/config', {
    method: 'POST',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify(payload),
  })
}

export function testOCRConnection(url: string, auth?: string): Promise<{ status: string, elapsed_ms: number }> {
  return request('/api/ocr/test', {
    method: 'POST',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify({ url, auth }),
  })
}
