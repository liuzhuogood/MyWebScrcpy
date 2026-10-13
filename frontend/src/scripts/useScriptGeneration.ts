import { computed, onUnmounted, reactive, type Ref } from 'vue'
import { streamPythonGeneration } from '@/api/generation'
import type { MarkdownScript } from '@/api/mdScripts'
import { updateActivity, type AssistantMessage } from './useScriptAssistant'

interface Generation { message: AssistantMessage, busy: boolean, status: string }

export function useScriptGeneration(currentId: Ref<number | null>) {
  const jobs = reactive<Record<number, Generation>>({})
  const controllers = new Map<number, AbortController>()
  const generation = computed(() => currentId.value === null ? undefined : jobs[currentId.value])
  async function generatePython(id: number): Promise<MarkdownScript> {
    if (controllers.has(id)) throw new Error('此任务正在生成 Python')
    const job = reactive<Generation>({ message: { role: 'assistant', content: '', activities: [] }, busy: true, status: '正在生成 Python…' })
    jobs[id] = job
    const controller = new AbortController()
    controllers.set(id, controller)
    try {
      const script = await streamPythonGeneration(id, event => {
        if (event.type === 'status') job.status = event.delta
        else if (event.type !== 'done' && event.type !== 'text_delta') updateActivity(job.message, event)
      }, controller.signal)
      job.status = 'Python 已生成'
      return script
    } catch (error) {
      job.status = controller.signal.aborted ? '已停止生成' : error instanceof Error ? error.message : '生成失败'
      throw new Error(job.status)
    } finally {
      job.busy = false
      for (const activity of job.message.activities!) {
        if (activity.kind === 'thinking') activity.running = false
        else if (activity.status === 'running') activity.status = 'interrupted'
      }
      controllers.delete(id)
    }
  }
  function stopGeneration() { if (currentId.value !== null) controllers.get(currentId.value)?.abort() }
  onUnmounted(() => { for (const controller of controllers.values()) controller.abort() })
  return { generation, generatePython, stopGeneration }
}
