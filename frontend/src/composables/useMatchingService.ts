import { computed, onMounted, onUnmounted, ref } from 'vue'
export interface MatchingStatus {
  state: 'stopped' | 'checking' | 'installing' | 'starting' | 'running' | 'stopping' | 'failed'
  pid: number; error: string
  environment: { path: string; version: string; ready: boolean; error: string; warning: string; dependencies: { name: string; available: boolean; error: string }[] }
  devices: { serial: string; state: 'waiting' | 'matching' | 'failed'; error: string }[]
}
export function useMatchingService() {
  const status = ref<MatchingStatus>()
  const error = ref('')
  const acting = ref(false)
  const busy = computed(() => acting.value || ['checking', 'installing', 'starting', 'stopping'].includes(status.value?.state ?? ''))
  let timer: ReturnType<typeof setTimeout> | undefined
  let disposed = false
  let requestRevision = 0
  async function request(operation: 'status' | 'check' | 'start' | 'stop') {
    const revision = ++requestRevision
    const desktopToken = (window as Window & { __MYWEBSCRCPY_DESKTOP_TOKEN__?: string }).__MYWEBSCRCPY_DESKTOP_TOKEN__
    const response = await fetch(`/api/matching-service/${operation}`, { method: operation === 'status' ? 'GET' : 'POST', headers: desktopToken ? { Authorization: `Bearer ${desktopToken}` } : {}, signal: AbortSignal.timeout(operation === 'start' ? 17 * 60 * 1000 : 40000) })
    if (!response.ok) {
      if (response.headers.get('content-type')?.includes('application/json')) {
        const failure = await response.json() as { error: string; status: MatchingStatus }
        if (!disposed && revision === requestRevision) status.value = failure.status
        throw new Error(failure.error)
      }
      const text = (await response.text()).trim()
      throw new Error(text || `请求失败（${response.status}）`)
    }
    const current = await response.json() as MatchingStatus
    if (!disposed && revision === requestRevision) status.value = current
  }
  async function refresh() {
    try { await request('status'); if (!acting.value) error.value = '' }
    catch (cause) { error.value = cause instanceof Error ? cause.message : '状态读取失败' }
  }
  async function action(operation: 'check' | 'start' | 'stop') {
    if (busy.value) return
    acting.value = true; error.value = ''
    try { await request(operation) }
    catch (cause) { error.value = cause instanceof Error ? cause.message : '操作失败' }
    finally { acting.value = false }
  }
  async function poll() { await refresh(); if (!disposed) timer = setTimeout(poll, 2500) }
  onMounted(poll)
  onUnmounted(() => { disposed = true; if (timer) clearTimeout(timer) })
  return { status, error, busy, action }
}
