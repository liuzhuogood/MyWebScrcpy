import { computed, onUnmounted, reactive, ref, type WritableComputedRef, type Ref } from 'vue'
import { type ChatMessage } from '@/api/ai'
import { streamAIChat, type ChatStreamEvent } from '@/api/aiStream'
import { hasDocumentTargets } from './document'

export type AssistantActivity =
  | { kind: 'thinking', content: string, running: boolean }
  | { kind: 'tool', id: string, name: string, args: string, output: string, status: 'running' | 'completed' | 'failed' | 'interrupted' }
export interface AssistantMessage {
  role: 'user' | 'assistant'
  content: string
  activities?: AssistantActivity[]
  reply?: string
  failed?: boolean
}
interface Proposal { content: string, base: string }
interface Conversation { messages: AssistantMessage[], prompt: string, busy: boolean, proposal?: Proposal }

export function updateActivity(message: AssistantMessage, event: ChatStreamEvent) {
  const activities = message.activities!
  const last = activities.at(-1)
  switch (event.type) {
    case 'thinking_start':
      activities.push({ kind: 'thinking', content: '', running: true })
      break
    case 'thinking_delta':
      if (last?.kind === 'thinking' && last.running) last.content += event.delta ?? ''
      else activities.push({ kind: 'thinking', content: event.delta ?? '', running: true })
      break
    case 'thinking_end':
      if (last?.kind === 'thinking') last.running = false
      break
    case 'text_delta':
      message.content += event.delta ?? ''
      break
    case 'tool_start':
      activities.push({ kind: 'tool', id: event.tool_call_id, name: event.tool_name, args: event.args ? JSON.stringify(event.args, null, 2) : '', output: '', status: 'running' })
      break
    case 'tool_update':
    case 'tool_end': {
      const tool = activities.find(item => item.kind === 'tool' && item.id === event.tool_call_id)
      if (tool?.kind !== 'tool') throw new Error('工具进度缺少对应的调用记录')
      tool.output = event.output ?? ''
      if (event.type === 'tool_end') tool.status = event.is_error ? 'failed' : 'completed'
      break
    }
  }
}

export function useScriptAssistant(currentId: Ref<number | null>, content: WritableComputedRef<string>, notify: (message: string) => void) {
  const conversations = reactive<Record<number, Conversation>>({})
  const controllers = new Map<number, AbortController>()
  const empty = ref<Conversation>({ messages: [], prompt: '', busy: false })
  const conversation = computed(() => {
    const id = currentId.value
    if (id === null) return empty.value
    if (!conversations[id]) conversations[id] = { messages: [], prompt: '', busy: false }
    return conversations[id]!
  })
  async function sendPrompt() {
    const id = currentId.value
    const chat = conversation.value
    const prompt = chat.prompt.trim()
    if (id === null || !prompt || chat.busy) return
    const base = content.value
    chat.messages.push({ role: 'user', content: prompt })
    chat.prompt = ''
    chat.busy = true
    delete chat.proposal
    const history: ChatMessage[] = chat.messages.filter(message => !message.failed).map(message => ({ role: message.role, content: message.reply ?? message.content }))
    const response = reactive<AssistantMessage>({ role: 'assistant', content: '', activities: [] })
    chat.messages.push(response)
    const controller = new AbortController()
    controllers.set(id, controller)
    try {
      const reply = await streamAIChat([
        { role: 'system', content: '你是移动端自动化文档助手。根据需求修改 Markdown 脚本。给出完整文档时使用 markdown 代码块。保留 version、schedule、timeout_sec、enabled 配置。目标设备在任务配置中单独选择，禁止生成 target_device 或 target_devices 字段。不要臆造模板 ID。当前文档见 current.md，可使用只读工具查阅。' },
        ...history,
      ], base, event => updateActivity(response, event), controller.signal)
      response.reply = reply
      const block = reply.match(/```markdown\s*\n([\s\S]*?)\n```/)
      const candidate = block ? block[1]! : reply.startsWith('---') ? reply : undefined
      if (candidate) {
        if (hasDocumentTargets(candidate)) response.content = `${reply}\n\n建议含有文档设备配置，请让助手移除后再应用。`
        else {
          response.content = block ? reply.replace(block[0], '').trim() : ''
          chat.proposal = { content: candidate, base }
        }
      } else response.content = reply
    } catch (error) {
      response.failed = true
      if (controller.signal.aborted) response.content += '\n已停止。'
      else {
        console.warn('[ScriptAssistant] 请求失败', error)
        response.content += `\n助手暂不可用：${error instanceof Error ? error.message : '请求失败'}`
      }
    } finally {
      for (const activity of response.activities!) {
        if (activity.kind === 'thinking') activity.running = false
        else if (activity.status === 'running') activity.status = 'interrupted'
      }
      controllers.delete(id)
      chat.busy = false
    }
  }
  function stopAssistant() {
    if (currentId.value !== null) controllers.get(currentId.value)?.abort()
  }
  onUnmounted(() => { for (const controller of controllers.values()) controller.abort() })
  function applyProposal() {
    const proposal = conversation.value.proposal
    if (!proposal) return
    if (content.value !== proposal.base) {
      notify('文档已有新的修改，请重新生成建议。')
      return
    }
    content.value = proposal.content
    delete conversation.value.proposal
  }
  return { conversation, sendPrompt, stopAssistant, applyProposal }
}
