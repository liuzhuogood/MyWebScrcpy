import { computed, onUnmounted, ref } from 'vue'
import type { DependencyInfo, DependencyStatus } from '@/types/dependencies'

export type DependencyId = 'adb' | 'python'
export interface Selection { source: 'builtin' | 'system'; path: string }
export interface Config { schema: 2; adb: Selection; python: Selection }
export interface InstallTask { stage: string; received: number; total: number; error: string }
export interface ManagedTool {
  id: DependencyId
  name: string
  selection: Selection
  installed: boolean
  residual: boolean
  version: string
  builtin_path?: string
  supported: boolean
  effective: DependencyInfo
  task: InstallTask
  modules_task?: InstallTask
}
export interface ManagedStatus extends DependencyStatus { managed: ManagedTool[]; config: Config }
const emptySelection = (): Selection => ({ source: 'system', path: '' })
export function useDependencies() {
  const status = ref<ManagedStatus>()
  const draft = ref<Config>({ schema: 2, adb: emptySelection(), python: emptySelection() })
  const error = ref('')
  const message = ref('')
  const loading = ref(false)
  const saving = ref(false)
  const acting = ref<DependencyId>()
  const checks = ref<Partial<Record<DependencyId, DependencyInfo>>>({})
  const checking = ref<DependencyId>()
  let timer: ReturnType<typeof setTimeout> | undefined
  let stopped = false
  let refreshing: Promise<ManagedStatus> | undefined
  function headers(): Record<string, string> {
    const desktopToken = (window as Window & { __MYWEBSCRCPY_DESKTOP_TOKEN__?: string }).__MYWEBSCRCPY_DESKTOP_TOKEN__
    return { 'Content-Type': 'application/json', ...(desktopToken ? { Authorization: `Bearer ${desktopToken}` } : {}) }
  }
  async function request<T>(url: string, init?: RequestInit): Promise<T> {
    const response = await fetch(url, { ...init, headers: headers() })
    if (!response.ok) throw new Error((await response.text()).trim() || '请求失败')
    return response.status === 204 || response.status === 202 ? undefined as T : response.json() as Promise<T>
  }
  async function refresh(reset = false) {
    try {
      const work = refreshing ?? request<ManagedStatus>('/api/dependencies')
      refreshing = work
      const result = await work
      if (refreshing === work) refreshing = undefined
      const previousPython = status.value?.managed.find(tool => tool.id === 'python')
      const python = result.managed.find(tool => tool.id === 'python')
      // pip holds the runtime lock. Keep its last verified display until the
      // installation finishes, without probing an environment being modified.
      if (python?.modules_task?.stage === 'installing' && previousPython?.effective.installed && JSON.stringify(python.selection) === JSON.stringify(previousPython.selection)) {
        python.effective = previousPython.effective
      }
      status.value = result
      if (reset) { draft.value = structuredClone(result.config); checks.value = {}; error.value = ''; message.value = '' }
      else {
        const modulesFinished = python?.modules_task && ['ready', 'failed', 'cancelled'].includes(python.modules_task.stage) && python.modules_task.stage !== previousPython?.modules_task?.stage
        if ((python?.task.stage === 'ready' && previousPython?.task.stage !== 'ready') || modulesFinished) await detect('python')
      }
    }
    catch (cause) { refreshing = undefined; error.value = cause instanceof Error ? cause.message : '依赖检测失败' }
  }
  async function load() { loading.value = true; await refresh(true); loading.value = false }
  function selection(id: DependencyId) { return draft.value[id] }
  function runtimeBusy(tool: ManagedTool) { return ['downloading', 'verifying', 'publishing', 'removing'].includes(tool.task.stage) }
  function busy(tool: ManagedTool) { return runtimeBusy(tool) || tool.modules_task?.stage === 'installing' }
  const pending = computed(() => status.value?.managed.some(busy) || Boolean(acting.value) || Boolean(checking.value))
  async function action(id: DependencyId, url: string, method: string, body?: unknown) {
    acting.value = id; error.value = ''; message.value = ''
    try { await refreshing?.catch(() => undefined); await request(url, { method, ...(body ? { body: JSON.stringify(body) } : {}) }); await refresh() }
    catch (cause) { error.value = cause instanceof Error ? cause.message : '操作失败' }
    finally { acting.value = undefined }
  }
  async function detect(id: DependencyId) {
    checking.value = id; error.value = ''; delete checks.value[id]
    try { checks.value[id] = await request<DependencyInfo>('/api/dependencies/detect', { method: 'POST', body: JSON.stringify({ id, selection: selection(id) }) }) }
    catch (cause) { error.value = cause instanceof Error ? cause.message : '检测失败' }
    finally { checking.value = undefined }
  }
  async function save() {
    saving.value = true; error.value = ''; message.value = ''
    try { await refreshing?.catch(() => undefined); await request('/api/dependencies/config', { method: 'PUT', body: JSON.stringify(draft.value) }); await refresh(); message.value = '已保存' }
    catch (cause) { error.value = cause instanceof Error ? cause.message : '保存失败' }
    finally { saving.value = false }
  }
  function startPolling() {
    stopped = false
    async function poll() { if (!acting.value && !saving.value && !checking.value) await refresh(); if (!stopped) timer = setTimeout(poll, 2500) }
    timer = setTimeout(poll, 2500)
  }
  function stopPolling() { stopped = true; if (timer) clearTimeout(timer) }
  onUnmounted(stopPolling)
  return { status, draft, error, message, loading, saving, acting, checking, checks, pending, selection, busy, runtimeBusy, refresh, load, action, detect, save, startPolling, stopPolling }
}
