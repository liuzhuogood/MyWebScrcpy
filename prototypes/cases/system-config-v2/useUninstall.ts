import { computed, onUnmounted, ref, type Ref } from 'vue'
import type { Tool, ToolId } from '../system-config-v1/useSystemConfig'

// Simulate removal of the application's dependency copy without touching files.
export function useUninstall(tools: Ref<Tool[]>, clearFeedback: () => void) {
  const pendingId = ref<ToolId | null>(null)
  const removingId = ref<ToolId | null>(null)
  const pendingTool = computed(() => tools.value.find(tool => tool.id === pendingId.value))
  const confirmationOpen = computed({
    get: () => pendingId.value !== null,
    set: (value: boolean) => { if (!value) pendingId.value = null },
  })
  let timer: ReturnType<typeof setTimeout> | undefined

  function request(tool: Tool) {
    if (tool.state !== 'ready' || removingId.value !== null) return
    pendingId.value = tool.id
  }
  function confirm() {
    const tool = pendingTool.value
    if (!tool || tool.state !== 'ready' || removingId.value !== null) return
    pendingId.value = null
    removingId.value = tool.id
    clearFeedback()
    timer = setTimeout(() => {
      tool.state = 'missing'
      tool.progress = 0
      tool.error = ''
      removingId.value = null
      timer = undefined
    }, 600)
  }
  onUnmounted(() => { if (timer) clearTimeout(timer) })
  return { confirmationOpen, pendingTool, removingId, request, confirm }
}
