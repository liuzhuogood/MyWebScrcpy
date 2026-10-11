import { computed, reactive, ref, type WritableComputedRef, type Ref } from 'vue'
import { sendAIChat, type ChatMessage } from '@/api/ai'
import { hasDocumentTargets, initialDocument } from './document'

interface Message { role: 'user' | 'assistant', content: string }
interface Proposal { content: string, base: string }
interface Conversation { messages: Message[], prompt: string, busy: boolean, proposal?: Proposal }

// Retain the existing local rule suggestions, but require explicit application instead of overwriting drafts.
function localSuggestion(prompt: string, content: string): string {
  if (['生成', '新建', '小红书', '抖音'].some(word => prompt.includes(word))) {
    return initialDocument('社交平台内容巡检与点赞').replace('# 步骤流程\n- 唤醒设备\n- 等待 2 秒', '# 自动巡检流程\n- 唤醒手机屏幕\n- 向上滑动屏幕解锁\n- 启动应用包名 `com.xingin.xhs`\n- 等待 3 秒\n- 向下滑动浏览两条推荐帖子')
  }
  return `${content}\n- ${prompt}\n`
}

export function useScriptAssistant(currentId: Ref<number | null>, content: WritableComputedRef<string>, notify: (message: string) => void) {
  const conversations = reactive<Record<number, Conversation>>({})
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
    const history: ChatMessage[] = chat.messages.map(message => ({ ...message }))
    try {
      const res = await sendAIChat([
        { role: 'system', content: `你是移动端自动化文档助手。根据需求修改 Markdown 脚本。给出完整文档时使用 markdown 代码块。保留 version、schedule、timeout_sec、enabled 配置。目标设备在任务配置中单独选择，禁止生成 target_device 或 target_devices 字段。不要臆造模板 ID。当前文档：\n${base}` },
        ...history,
      ])
      const block = res.reply.match(/```markdown\s*\n([\s\S]*?)\n```/)
      const candidate = block ? block[1]! : res.reply.startsWith('---') ? res.reply : undefined
      if (candidate) {
        const explanation = block ? res.reply.replace(block[0], '').trim() : ''
        if (explanation) chat.messages.push({ role: 'assistant', content: explanation })
        if (hasDocumentTargets(candidate)) chat.messages.push({ role: 'assistant', content: `${candidate}\n\n建议含有文档设备配置，请让助手移除后再应用。` })
        else chat.proposal = { content: candidate, base }
      } else chat.messages.push({ role: 'assistant', content: res.reply })
    } catch (error) {
      console.warn('[ScriptAssistant] 请求失败', error)
      chat.messages.push({ role: 'assistant', content: `助手暂不可用：${error instanceof Error ? error.message : '请求失败'}。以下为内置步骤建议。` })
      chat.proposal = { content: localSuggestion(prompt, base), base }
    } finally { chat.busy = false }
  }
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
  return { conversation, sendPrompt, applyProposal }
}
