import { computed, nextTick, onUnmounted, ref, watch, type Ref } from 'vue'
import { getDispatch, getTaskDetail, getTaskLogs, getTaskReport, listDispatches, retryTaskReport, type Dispatch, type LogLine, type ReportResult, type TaskDetail, type TaskView } from '@/api/taskRuns'

const message = (error: unknown) => error instanceof Error ? error.message : '请求失败'
export function useExecutionRecords(scriptId: Ref<number | undefined>, active: Ref<boolean>, initialTask: Ref<string>) {
  const dispatches = ref<Dispatch[]>([]), tasks = ref<TaskView[]>([])
  const selectedDispatch = ref(''), selectedTask = ref('')
  const dispatch = ref<Dispatch>(), detail = ref<TaskDetail>(), report = ref<ReportResult>()
  const listLoading = ref(false), listError = ref(''), detailLoading = ref(false), detailError = ref('')
  const logLoading = ref(false), logError = ref(''), reportError = ref(''), retrying = ref(false)
  const lines = ref<LogLine[]>([]), stream = ref('all'), search = ref(''), follow = ref(true), older = ref(false)
  const tab = ref('logs'), mobileDetail = ref(false), highlighted = ref(0)
  let cursor = '', more = false, epoch = 0, after = 0, before = 0, queryEpoch = 0
  let controller = new AbortController(), logController = new AbortController(), listController = new AbortController()
  const hasMore = ref(false)
  const running = computed(() => !!detail.value && ['preparing', 'running', 'stopping'].includes(detail.value.task.status))
  const fresh = (generation: number) => generation === epoch && active.value
  async function loadList(append = false) {
    if (!scriptId.value || listLoading.value) return
    const id = scriptId.value, generation = epoch
    listLoading.value = true; listError.value = ''
    try {
      const response = await listDispatches(id, append ? cursor : '', listController.signal)
      if (!fresh(generation) || id !== scriptId.value) return
      const existing = append ? dispatches.value : []
      dispatches.value = [...existing, ...response.dispatches.filter(item => !existing.some(old => old.id === item.id))]
      cursor = response.next_cursor; more = response.has_more; hasMore.value = more
      if (!selectedDispatch.value && response.dispatches.length) selectDispatch(response.dispatches[0]!.id)
    } catch (error) { if (fresh(generation) && !listController.signal.aborted) listError.value = message(error) }
    finally { listLoading.value = false }
  }
  function selectDispatch(id: string) {
    resetRequest(); selectedDispatch.value = id; selectedTask.value = ''; tasks.value = []; detail.value = undefined; dispatch.value = undefined
    detailLoading.value = true; detailError.value = ''; void refreshDetail()
  }
  function resetRequest() { epoch++; controller.abort(); logController.abort(); controller = new AbortController(); logController = new AbortController(); queryEpoch++; logLoading.value = false; detailError.value = ''; logError.value = ''; reportError.value = '' }
  function selectTask(id: string, openMobile = true) {
    resetRequest(); selectedTask.value = id; lines.value = []; report.value = undefined; detail.value = undefined
    after = 0; before = 0; older.value = false; stream.value = 'all'; search.value = ''; follow.value = true; highlighted.value = 0
    if (openMobile) mobileDetail.value = true
    detailLoading.value = true; void refreshTask(); void loadLogs(); void loadReport()
  }
  async function refreshDetail() {
    const generation = epoch, id = scriptId.value, batch = selectedDispatch.value
    if (!id || !batch) return
    try {
      const response = await getDispatch(id, batch, controller.signal)
      if (!fresh(generation)) return
      dispatch.value = response.dispatch; tasks.value = response.tasks
      if (!selectedTask.value && response.tasks.length) selectTask(response.tasks[0]!.id, false)
    } catch (error) { if (fresh(generation) && !controller.signal.aborted) detailError.value = message(error) }
    finally { if (fresh(generation)) detailLoading.value = false }
  }
  async function refreshTask() {
    const generation = epoch, id = selectedTask.value
    if (!id) return
    try { const result = await getTaskDetail(id, controller.signal); if (fresh(generation) && id === selectedTask.value) { detail.value = result; detailError.value = '' } }
    catch (error) { if (fresh(generation) && !controller.signal.aborted) detailError.value = message(error) }
    finally { if (fresh(generation)) detailLoading.value = false }
  }
  async function loadLogs(mode: 'initial' | 'follow' | 'older' = 'initial') {
    const generation = epoch, queryGeneration = queryEpoch, id = selectedTask.value, signal = logController.signal
    if (!id || logLoading.value) return
    logLoading.value = true; logError.value = ''
    const query = new URLSearchParams({ limit: '200', stream: stream.value, q: search.value })
    if (mode === 'follow') query.set('after', String(after))
    if (mode === 'older') query.set('before', String(before))
    try {
      const result = await getTaskLogs(id, query, signal)
      if (!fresh(generation) || queryGeneration !== queryEpoch) return
      if (mode === 'initial') { lines.value = result.items; after = result.latest_seq; before = result.next_cursor; older.value = result.has_more }
      else {
        const merged = new Map((mode === 'follow' ? [...lines.value, ...result.items] : [...result.items, ...lines.value]).map(line => [line.seq, line]))
        lines.value = [...merged.values()].sort((a, b) => a.seq - b.seq)
        if (mode === 'follow') { after = result.next_cursor; if (lines.value.length > 1000) lines.value = lines.value.slice(-1000) }
        else { before = result.next_cursor; older.value = result.has_more }
      }
    } catch (error) { if (fresh(generation) && queryGeneration === queryEpoch && !signal.aborted) logError.value = message(error) }
    finally { if (queryGeneration === queryEpoch) logLoading.value = false }
  }
  async function loadReport() {
    const generation = epoch, id = selectedTask.value
    if (!id) return
    try { const result = await getTaskReport(id, controller.signal); if (fresh(generation)) { report.value = result; reportError.value = '' } }
    catch (error) { if (fresh(generation) && !controller.signal.aborted) reportError.value = message(error) }
  }
  async function retryReport() {
    if (retrying.value || !selectedTask.value) return
    const generation = epoch; retrying.value = true; reportError.value = ''
    try { await retryTaskReport(selectedTask.value, controller.signal); if (fresh(generation)) { await refreshTask(); await loadReport() } }
    catch (error) { if (fresh(generation) && !controller.signal.aborted) reportError.value = message(error) }
    finally { retrying.value = false }
  }
  async function jumpToLine(seq: number) {
    const generation = epoch; tab.value = 'logs'; follow.value = false; stream.value = 'all'; search.value = ''; highlighted.value = seq
    await nextTick()
    if (!fresh(generation)) return
    logController.abort(); logController = new AbortController(); const signal = logController.signal; const queryGeneration = ++queryEpoch; logLoading.value = true; logError.value = ''
    try {
      const result = await getTaskLogs(selectedTask.value, new URLSearchParams({ after: String(Math.max(0, seq - 5)), limit: '40' }), signal)
      if (fresh(generation) && queryGeneration === queryEpoch) { lines.value = result.items; after = result.next_cursor; before = result.oldest_seq; older.value = before > 1 }
    } catch (error) { if (fresh(generation) && queryGeneration === queryEpoch && !signal.aborted) logError.value = message(error) }
    finally { if (queryGeneration === queryEpoch) logLoading.value = false }
  }
  watch([stream, search], () => { logController.abort(); logController = new AbortController(); queryEpoch++; logLoading.value = false; lines.value = []; after = 0; before = 0; if (active.value) void loadLogs() })
  watch([scriptId, active, initialTask], async () => {
    resetRequest(); listController.abort(); listController = new AbortController(); listLoading.value = false; logLoading.value = false
    if (!active.value) return
    dispatches.value = []; selectedDispatch.value = ''; selectedTask.value = ''; detail.value = undefined; dispatch.value = undefined; mobileDetail.value = false
    if (initialTask.value) {
      const generation = epoch
      try { const target = await getTaskDetail(initialTask.value, controller.signal); if (fresh(generation)) { selectedDispatch.value = target.task.dispatch_id; await refreshDetail(); selectTask(target.task.id); } }
      catch (error) { if (fresh(generation)) detailError.value = message(error) }
    }
    void loadList()
  }, { immediate: true })
  let polling = false
  const timer = setInterval(async () => {
    if (!active.value || polling) return
    polling = true
    try { await Promise.allSettled([refreshDetail(), refreshTask(), loadReport(), follow.value && tab.value === 'logs' ? loadLogs('follow') : Promise.resolve()]) }
    finally { polling = false }
  }, 2000)
  onUnmounted(() => { clearInterval(timer); resetRequest(); listController.abort() })
  return { dispatches, tasks, selectedDispatch, selectedTask, dispatch, detail, report, listLoading, listError, detailLoading, detailError, logLoading, logError, reportError, retrying, lines, stream, search, follow, older, tab, mobileDetail, highlighted, hasMore, running, loadList, selectDispatch, selectTask, refreshTask, loadLogs, retryReport, jumpToLine }
}
