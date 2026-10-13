import { computed, onMounted, onUnmounted, reactive, ref, watch } from 'vue'
import { fetchDevices, type Device } from '@/api/devices'
import { createMarkdownScript, deleteMarkdownScript, killTask, listActiveTasks, listMarkdownScripts, listScriptRuns, runMarkdownScript, updateMarkdownScript, type MarkdownScript, type RunningTask } from '@/api/mdScripts'
import { listScheduleEntries, toggleSchedule, type ScheduleEntry } from '@/api/scheduler'
import { listTemplates, type TemplateItem } from '@/api/templates'
import { hasDocumentTargets, initialDocument, removeDocumentTargets } from './document'
import { useScriptAssistant } from './useScriptAssistant'
import { useScriptGeneration } from './useScriptGeneration'

export function useScriptsWorkbench() {
  const scripts = ref<MarkdownScript[]>([])
  const devices = ref<Device[]>([])
  const schedules = ref<ScheduleEntry[]>([])
  const activeTasks = ref<RunningTask[]>([])
  const history = reactive<Record<number, RunningTask[]>>({})
  const drafts = reactive<Record<number, string>>({})
  const currentId = ref<number | null>(null)
  const current = computed(() => scripts.value.find(item => item.id === currentId.value))
  const content = computed({ get: () => currentId.value !== null && drafts[currentId.value] !== undefined ? drafts[currentId.value]! : current.value?.content ?? '', set: value => { if (currentId.value !== null) drafts[currentId.value] = value } })
  const dirty = computed(() => !!current.value && content.value !== current.value.content)
  const legacyTargets = computed(() => hasDocumentTargets(content.value))
  const tab = ref('source')
 const executionsOpen = ref(false)
 const executionTask = ref('')
 function openExecutions(taskId = '') { executionTask.value = taskId; executionsOpen.value = true }
  const mobilePanel = ref('editor')
  const pythonGeneration = useScriptGeneration(currentId)
  const taskSearch = ref('')
  const filteredScripts = computed(() => scripts.value.filter(item => `${item.name} ${item.target_devices.map(serial => `${deviceName(serial)} ${serial}`).join(' ')}`.toLowerCase().includes(taskSearch.value.toLowerCase())))
  const loading = ref(true)
  const loadError = ref('')
  const deviceError = ref('')
  const notice = ref('')
  const busy = ref('')
  const notify = (message: string) => { notice.value = message }
  const deviceName = (serial: string) => { const device = devices.value.find(item => item.serial === serial); return device?.alias ? device.alias : serial }
  const scheduleFor = (id: number) => schedules.value.find(item => item.script_id === id)
  const currentRuns = computed(() => {
    if (!current.value) return []
    const latest = new Map<string, RunningTask>()
    for (const task of history[current.value.id] ?? []) if (!latest.has(task.device_serial)) latest.set(task.device_serial, task)
    // Active records take priority even if an overlapping attempt was skipped afterwards.
    for (const task of activeTasks.value.filter(item => item.script_id === current.value!.id)) {
      const attempt = latest.get(task.device_serial)
      latest.set(task.device_serial, { ...task, error_msg: attempt && attempt.id !== task.id && attempt.started_at >= task.started_at ? attempt.error_msg : task.error_msg })
    }
    return [...latest.values()]
  })
  const taskManagerOpen = ref(false)
  const modelConfigOpen = ref(false)

  async function loadDevices() {
    deviceError.value = ''
    try { devices.value = await fetchDevices() }
    catch (error) { deviceError.value = errorText(error); console.error('[Scripts] 设备列表失败', error) }
  }
  async function loadScripts() {
    loading.value = true
    loadError.value = ''
    try {
      scripts.value = (await listMarkdownScripts()).scripts
      if (!scripts.value.some(item => item.id === currentId.value)) currentId.value = scripts.value[0]?.id ?? null
    } catch (error) { loadError.value = errorText(error); console.error('[Scripts] 任务列表失败', error) }
    finally { loading.value = false }
  }
  let polling = false
  let disposed = false
  async function refreshStatus() {
    if (polling || disposed) return
    polling = true
    const id = currentId.value
    try {
      const requests = await Promise.allSettled([listActiveTasks(), listScheduleEntries(), id === null ? Promise.resolve(null) : listScriptRuns(id)])
      const [tasks, entries, runs] = requests
      if (disposed) return
      if (tasks.status === 'fulfilled') activeTasks.value = tasks.value.tasks
      if (entries.status === 'fulfilled') schedules.value = entries.value.entries
      if (id !== null && runs.status === 'fulfilled' && runs.value) history[id] = runs.value.tasks
      const failure = requests.find(item => item.status === 'rejected')
      if (failure?.status === 'rejected') { notify(`状态更新失败：${errorText(failure.reason)}`); console.warn('[Scripts] 状态更新失败', failure.reason) }
    } finally { polling = false; if (!disposed && currentId.value !== id) void refreshStatus() }
  }
  function selectTask(id: number) { currentId.value = id; mobilePanel.value = 'editor'; notice.value = '' }
  function replaceScript(script: MarkdownScript) {
    const index = scripts.value.findIndex(item => item.id === script.id)
    if (index < 0) scripts.value.unshift(script)
    else scripts.value[index] = script
  }
  async function save() {
    const script = current.value
    if (!script || busy.value) return
    const snapshot = content.value
    busy.value = 'save'
    try {
      const res = await updateMarkdownScript(script.id, script.name, [...script.target_devices], snapshot)
      replaceScript(res.script)
      if (drafts[script.id] === snapshot) delete drafts[script.id]
      notify('已保存')
      await refreshStatus()
    } catch (error) { notify(errorText(error)); console.error('[Scripts] 保存失败', error) }
    finally { busy.value = '' }
  }
  const formOpen = ref(false)
  const editingId = ref<number | null>(null)
  const formName = ref('')
  const formDevices = ref<string[]>([])
  const formSearch = ref('')
  const formError = ref('')
  const formContent = ref('')
  const formBusy = ref(false)
  const formRemoveLegacy = ref(false)
  const filteredDevices = computed(() => {
    const selectable = [...devices.value]
    for (const serial of formDevices.value) if (!selectable.some(item => item.serial === serial)) selectable.push({ serial, state: 'missing' })
    return selectable.filter(item => `${item.alias ?? ''} ${item.serial}`.toLowerCase().includes(formSearch.value.toLowerCase()))
  })
  function openForm(editing: boolean) {
    const script = editing ? current.value : undefined
    if (editing && !script) return
    editingId.value = script?.id ?? null
    formName.value = script?.name ?? ''
    formDevices.value = script ? [...script.target_devices] : []
    formContent.value = script ? content.value : ''
    formRemoveLegacy.value = false
    formSearch.value = ''
    formError.value = ''
    formOpen.value = true
    void loadDevices()
  }
  function toggleDevice(serial: string) {
    formDevices.value = formDevices.value.includes(serial) ? formDevices.value.filter(item => item !== serial) : [...formDevices.value, serial]
  }
  async function submitForm() {
    if (formBusy.value) return
    const name = formName.value.trim()
    if (!name) { formError.value = '请填写任务标题'; return }
    if (!formDevices.value.length) { formError.value = '请选择至少一台目标设备'; return }
    const id = editingId.value
    const original = formContent.value
    let snapshot = id === null ? initialDocument(name) : original
    if (formRemoveLegacy.value) snapshot = removeDocumentTargets(snapshot)
    formBusy.value = true
    formError.value = ''
    try {
      const res = id === null ? await createMarkdownScript(name, [...formDevices.value], snapshot) : await updateMarkdownScript(id, name, [...formDevices.value], snapshot)
      replaceScript(res.script)
      if (id !== null && drafts[id] === original) delete drafts[id]
      currentId.value = res.script.id
      formOpen.value = false
      mobilePanel.value = 'editor'
      notify(id === null ? '任务已创建' : '任务配置已保存')
      await refreshStatus()
    } catch (error) { formError.value = errorText(error); console.error('[Scripts] 任务配置保存失败', error) }
    finally { formBusy.value = false }
  }

  const assets = ref<TemplateItem[]>([])
  const templateLoading = ref(false)
  const templateError = ref('')
  const templateOpen = ref(false)
  const templateSearch = ref('')
  const templateScope = ref('all')
  let templateGeneration = 0
  const targetKey = computed(() => current.value?.target_devices.join('\u0000') ?? '')
  async function loadTemplates() {
    const generation = ++templateGeneration
    const targets = current.value ? [...current.value.target_devices] : []
    assets.value = []
    templateError.value = ''
    templateLoading.value = true
    try {
      const responses = await Promise.all((targets.length ? targets : ['global']).map(serial => listTemplates(serial)))
      if (generation !== templateGeneration || disposed) return
      const unique = new Map<string, TemplateItem>()
      for (const response of responses) for (const asset of response.templates) if (asset.serial === 'global' || targets.includes(asset.serial)) unique.set(asset.id, asset)
      assets.value = [...unique.values()]
    } catch (error) {
      if (generation === templateGeneration && !disposed) templateError.value = errorText(error)
      console.error('[Scripts] 模板列表失败', error)
    } finally { if (generation === templateGeneration) templateLoading.value = false }
  }
  const visibleTemplates = computed(() => assets.value.filter(asset => asset.enabled && (templateScope.value === 'all' || templateScope.value === asset.serial) && asset.name.toLowerCase().includes(templateSearch.value.toLowerCase())))
  function insertTemplate(asset: TemplateItem) {
    if (!current.value || !asset.enabled || (asset.serial !== 'global' && !current.value.target_devices.includes(asset.serial))) return
    const name = asset.name.replace(/[\[\]\r\n]/g, '')
    content.value += `\n- 匹配并点击目标：\n  ![${name}](template://${asset.id}?threshold=${asset.threshold.toFixed(2)})\n`
    templateOpen.value = false
    tab.value = 'source'
    mobilePanel.value = 'editor'
  }
  function insertStep(kind: 'wait' | 'tap' | 'swipe') {
    const steps = { wait: '等待 2 秒', tap: '点击屏幕坐标 (540, 960)', swipe: '从屏幕底部 (500, 1600) 向上滑动至 (500, 600)' }
    content.value += `\n- ${steps[kind]}\n`
  }
  function removeLegacy() { content.value = removeDocumentTargets(content.value) }
  async function compile() {
    const script = current.value
    if (!script || busy.value || dirty.value || pythonGeneration.generation.value?.busy) return
    mobilePanel.value = 'assistant'
    try {
      const generated = await pythonGeneration.generatePython(script.id)
      replaceScript(generated)
      if (currentId.value === script.id) {
        if (content.value !== script.content) notify('文档已有新的修改，请保存后重新生成')
        else tab.value = 'python'
      }
    } catch (error) {
      console.error('[Scripts] 生成失败', error)
    }
  }
  async function run() {
    const script = current.value
    if (!script || busy.value || dirty.value || script.needs_device_configuration || script.needs_generation || pythonGeneration.generation.value?.busy) return
    busy.value = 'run'
    try {
      const res = await runMarkdownScript(script.id)
      const started = res.results.filter(item => item.status === 'started').length
      notify(`已启动 ${started}/${res.total} 台设备`)
      await loadScripts()
      await refreshStatus()
    } catch (error) { notify(errorText(error)); console.error('[Scripts] 运行失败', error) }
    finally { busy.value = '' }
  }
  async function stop(id: string) {
    try { await killTask(id); await refreshStatus() }
    catch (error) { notify(errorText(error)); console.error('[Scripts] 终止失败', error) }
  }
  async function toggleEnabled() {
    const script = current.value
    if (!script || busy.value) return
    busy.value = 'toggle'
    try { await toggleSchedule(script.id, script.is_enabled !== 1); await loadScripts(); await refreshStatus() }
    catch (error) { notify(errorText(error)); console.error('[Scripts] 调度切换失败', error) }
    finally { busy.value = '' }
  }
  const deleteOpen = ref(false)
  async function deleteCurrent() {
    const script = current.value
    if (!script || busy.value) return
    busy.value = 'delete'
    try { await deleteMarkdownScript(script.id); delete drafts[script.id]; deleteOpen.value = false; await loadScripts(); await refreshStatus() }
    catch (error) { notify(errorText(error)); console.error('[Scripts] 删除失败', error) }
    finally { busy.value = '' }
  }
  async function copyPython() {
    if (!current.value?.compiled_python) return
    try { await navigator.clipboard.writeText(current.value.compiled_python); notify('已复制 Python') }
    catch (error) { notify(`复制失败：${errorText(error)}`) }
  }
  const assistant = useScriptAssistant(currentId, content, notify)
  watch(targetKey, () => { templateScope.value = 'all'; void loadTemplates() })
  watch(currentId, () => { executionsOpen.value=false; void refreshStatus() })
  let timer: ReturnType<typeof setInterval> | undefined
  onMounted(async () => {
    await Promise.allSettled([loadScripts(), loadDevices(), refreshStatus()])
    if (!disposed) { await refreshStatus(); timer = setInterval(() => { void refreshStatus() }, 3000) }
  })
  onUnmounted(() => { disposed = true; templateGeneration++; if (timer) clearInterval(timer) })
  return { ...pythonGeneration, executionsOpen,executionTask,openExecutions,scripts, devices, current, currentId, content, dirty, legacyTargets, tab, mobilePanel, taskSearch, filteredScripts, loading, loadError, deviceError, notice, busy, deviceName, scheduleFor, currentRuns, activeTasks, taskManagerOpen, modelConfigOpen, loadScripts, loadDevices, selectTask, save, formOpen, editingId, formName, formDevices, formSearch, formError, formContent, formRemoveLegacy, formBusy, filteredDevices, openForm, toggleDevice, submitForm, assets, templateLoading, templateError, templateOpen, templateSearch, templateScope, visibleTemplates, loadTemplates, insertTemplate, insertStep, removeLegacy, compile, run, stop, toggleEnabled, deleteOpen, deleteCurrent, copyPython, ...assistant }
}

function errorText(error: unknown): string { return error instanceof Error ? error.message : '请求失败，请重试' }

export type ScriptsWorkbench = import('vue').UnwrapNestedRefs<ReturnType<typeof useScriptsWorkbench>>
