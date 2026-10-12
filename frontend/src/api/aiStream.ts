import { createParser } from 'eventsource-parser'
import type { ChatMessage } from './ai'

export type ChatStreamEvent =
  | { type: 'thinking_start' | 'thinking_end' | 'text_start' | 'text_end' }
  | { type: 'thinking_delta' | 'text_delta', delta?: string }
  | { type: 'tool_start', tool_call_id: string, tool_name: string, args?: Record<string, unknown> }
  | { type: 'tool_update', tool_call_id: string, tool_name: string, output?: string }
  | { type: 'tool_end', tool_call_id: string, tool_name: string, output?: string, is_error?: boolean }
  | { type: 'done', reply: string }
  | { type: 'error', error: string }

export async function streamAIChat(messages: ChatMessage[], document: string, onEvent: (event: ChatStreamEvent) => void, signal: AbortSignal): Promise<string> {
  const res = await fetch('/api/ai/chat', {
    method: 'POST',
    headers: { 'Content-Type': 'application/json', Accept: 'text/event-stream' },
    body: JSON.stringify({ messages, document }),
    signal,
  })
  if (!res.ok) {
    const data = await res.json() as { message: string }
    throw new Error(data.message)
  }
  if (!res.headers.get('content-type')?.startsWith('text/event-stream') || !res.body) {
    throw new Error('助手未返回实时消息流')
  }
  const reader = res.body.getReader()
  const decoder = new TextDecoder()
  let reply: string | undefined
  const parser = createParser({
    maxBufferSize: 16 * 1024 * 1024,
    onEvent: ({ data }) => {
      const event = JSON.parse(data) as ChatStreamEvent
      if (event.type === 'error') throw new Error(event.error)
      if (event.type === 'done') reply = event.reply
      onEvent(event)
    },
  })
  try {
    while (reply === undefined) {
      const chunk = await reader.read()
      if (chunk.done) {
        parser.feed(decoder.decode())
        if (reply === undefined) throw new Error('助手连接中断，回复尚未完成')
        break
      }
      parser.feed(decoder.decode(chunk.value, { stream: true }))
    }
    return reply!
  } finally {
    await reader.cancel().catch(() => {})
    reader.releaseLock()
  }
}
