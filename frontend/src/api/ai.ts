export interface AIModelConfig {
  provider: 'deepseek' | 'openai' | 'ollama' | 'custom'
  base_url: string
  api_key: string
  model: string
  masked_key?: string
}

export interface ChatMessage {
  role: 'system' | 'user' | 'assistant'
  content: string
}

async function request<T>(input: string, init: RequestInit = {}): Promise<T> {
  const res = await fetch(input, init)
  let data: { message?: string, error?: string } | null = null
  try {
    data = await res.json()
  } catch {
    // 非 JSON
  }
  if (!res.ok) {
    throw new Error(data?.message || data?.error || `HTTP ${res.status}`)
  }
  return data as T
}

export function getAIConfig(): Promise<AIModelConfig> {
  return request('/api/ai/config')
}

export function saveAIConfig(config: AIModelConfig): Promise<{ ok: boolean, message: string }> {
  return request('/api/ai/config', {
    method: 'POST',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify(config),
  })
}

export function testAIConnection(config: AIModelConfig): Promise<{ ok: boolean, message: string }> {
  return request('/api/ai/test', {
    method: 'POST',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify(config),
  })
}

export function sendAIChat(messages: ChatMessage[]): Promise<{ ok: boolean, reply: string }> {
  return request('/api/ai/chat', {
    method: 'POST',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify({ messages }),
  })
}
