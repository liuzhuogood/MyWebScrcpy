import { createParser } from 'eventsource-parser'
import type { ChatStreamEvent } from './aiStream'
import type { MarkdownScript } from './mdScripts'

export type GenerationEvent = Exclude<ChatStreamEvent, { type: 'done' }> | { type: 'status', delta: string } | { type: 'done', script: MarkdownScript }

export async function streamPythonGeneration(id: number, onEvent: (event: GenerationEvent) => void, signal: AbortSignal): Promise<MarkdownScript> {
  const response = await fetch(`/api/md-scripts/${id}/compile`, { method: 'POST', headers: { Accept: 'text/event-stream' }, signal })
  if (!response.ok) {
    const error = await response.json() as { message: string }
    throw new Error(error.message)
  }
  if (!response.body || !response.headers.get('content-type')?.startsWith('text/event-stream')) throw new Error('生成服务未返回实时消息流')
  const reader = response.body.getReader()
  const decoder = new TextDecoder()
  let script: MarkdownScript | undefined
  const parser = createParser({ maxBufferSize: 2 * 1024 * 1024, onEvent: ({ data }) => {
    const event = JSON.parse(data) as GenerationEvent
    if (event.type === 'error') throw new Error(event.error)
    if (event.type === 'done') {
      if (!event.script) throw new Error('生成完成但缺少脚本')
      script = event.script
    }
    onEvent(event)
  } })
  try {
    while (!script) {
      const chunk = await reader.read()
      if (chunk.done) {
        parser.feed(decoder.decode())
        if (!script) throw new Error('生成连接中断')
        break
      }
      parser.feed(decoder.decode(chunk.value, { stream: true }))
    }
    return script!
  } finally {
    await reader.cancel().catch(() => {})
    reader.releaseLock()
  }
}
