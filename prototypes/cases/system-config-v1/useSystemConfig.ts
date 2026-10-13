import { computed, onUnmounted, ref, watch } from 'vue'

export type Source = 'builtin' | 'system'
export type ToolId = 'pi-agent' | 'adb'
export type InstallState = 'missing' | 'downloading' | 'verifying' | 'ready' | 'failed'
type Selection = { source: Source; path: string }
export type Tool = {
  id: ToolId
  name: string
  purpose: string
  version: string
  size: string
  state: InstallState
  progress: number
  attempts: number
  error: string
  detectedPath: string
  selection: Selection
  pathChecked: boolean
  checking: boolean
  pathError: string
}

// All operations are local simulations. This module never calls the host or API.
export function useSystemConfig(installationTab = '依赖管理') {
  const open = ref(false)
  const tab = ref('theme')
  const theme = ref<'system' | 'light' | 'dark'>('system')
  const initialDark = document.documentElement.classList.contains('dark')
  const media = window.matchMedia('(prefers-color-scheme: dark)')
  const tools = ref<Tool[]>([
    { id: 'pi-agent', name: 'pi-agent', purpose: 'AI 助手', version: '0.86.1', size: '32 MiB', state: 'missing', progress: 0, attempts: 0, error: '', detectedPath: '', selection: { source: 'builtin', path: '' }, pathChecked: false, checking: false, pathError: '' },
    { id: 'adb', name: 'ADB', purpose: '设备连接与控制', version: '37.0.1', size: '以下载清单为准', state: 'missing', progress: 0, attempts: 0, error: '', detectedPath: '/opt/homebrew/bin/adb', selection: { source: 'system', path: '' }, pathChecked: true, checking: false, pathError: '' },
  ])
  const saved = ref<Record<ToolId, Selection>>({
    'pi-agent': { source: 'builtin', path: '' },
    adb: { source: 'system', path: '' },
  })
  const message = ref('')
  const formError = ref('')
  const timers = new Map<ToolId, ReturnType<typeof setInterval>>()
  const checkTimers = new Set<ReturnType<typeof setTimeout>>()
  const installing = computed(() => tools.value.some(tool => ['downloading', 'verifying'].includes(tool.state)))

  function applyTheme() {
    document.documentElement.classList.toggle('dark', theme.value === 'dark' || (theme.value === 'system' && media.matches))
  }
  watch(theme, applyTheme, { immediate: true })
  media.addEventListener('change', applyTheme)
  watch(open, value => {
    if (!value) return
    message.value = ''
    formError.value = ''
    for (const tool of tools.value) {
      tool.selection = { ...saved.value[tool.id] }
      tool.pathError = ''
      tool.pathChecked = !tool.selection.path && Boolean(tool.detectedPath)
    }
  })

  function stopTimer(id: ToolId) {
    const timer = timers.get(id)
    if (timer) clearInterval(timer)
    timers.delete(id)
  }
  function install(tool: Tool) {
    message.value = ''
    formError.value = ''
    stopTimer(tool.id)
    tool.state = 'downloading'
    tool.progress = 0
    tool.error = ''
    tool.attempts += 1
    timers.set(tool.id, setInterval(() => {
      tool.progress = Math.min(tool.progress + 12, 100)
      // The first Pi attempt exercises the recoverable network-failure path.
      if (tool.id === 'pi-agent' && tool.attempts === 1 && tool.progress >= 48) {
        stopTimer(tool.id)
        tool.state = 'failed'
        tool.error = '下载中断，请重试。'
      }
      else if (tool.progress === 100 && tool.state === 'downloading') tool.state = 'verifying'
      else if (tool.state === 'verifying') {
        stopTimer(tool.id)
        tool.state = 'ready'
      }
    }, 320))
  }
  function cancel(tool: Tool) {
    stopTimer(tool.id)
    tool.state = 'missing'
    tool.progress = 0
  }
  function chooseSource(tool: Tool, value: unknown) {
    if (value !== 'builtin' && value !== 'system') return
    tool.selection.source = value
    tool.pathError = ''
    message.value = ''
    formError.value = ''
  }
  function pathChanged(tool: Tool) {
    tool.pathChecked = false
    tool.pathError = ''
    message.value = ''
  }
  function checkPath(tool: Tool) {
    if (tool.checking) return
    tool.checking = true
    tool.pathChecked = false
    tool.pathError = ''
    const path = tool.selection.path.trim()
    const timer = setTimeout(() => {
      checkTimers.delete(timer)
      tool.checking = false
      if (!path) {
        tool.pathChecked = Boolean(tool.detectedPath)
        if (!tool.pathChecked) tool.pathError = '未在 PATH 中找到 pi，请填写可执行文件的绝对路径。'
      }
      else if (!path.startsWith('/') && !/^[a-zA-Z]:[\\/]/.test(path)) tool.pathError = '请输入可执行文件的绝对路径。'
      else if (!new RegExp(tool.id === 'adb' ? '[/\\\\]adb(?:\\.exe)?$' : '[/\\\\]pi(?:\\.exe)?$').test(path)) tool.pathError = `请选择 ${tool.id === 'adb' ? 'adb' : 'pi'} 可执行文件。`
      else tool.pathChecked = true
    }, 450)
    checkTimers.add(timer)
  }
  function save() {
    message.value = ''
    formError.value = ''
    const unavailable = tools.value.find(tool => tool.selection.source === 'builtin' ? tool.state !== 'ready' : !tool.pathChecked)
    if (unavailable) {
      formError.value = unavailable.selection.source === 'builtin'
        ? `请先在${installationTab}中安装 ${unavailable.name}。`
        : `请先检测 ${unavailable.name} 的系统路径。`
      return
    }
    for (const tool of tools.value) saved.value[tool.id] = { ...tool.selection, path: tool.selection.path.trim() }
    message.value = '已保存，新操作使用此配置。'
  }
  function stateLabel(state: InstallState) {
    return { missing: '未安装', downloading: '下载中', verifying: '校验中', ready: '已安装', failed: '安装失败' }[state]
  }
  onUnmounted(() => {
    for (const id of timers.keys()) stopTimer(id)
    for (const timer of checkTimers) clearTimeout(timer)
    media.removeEventListener('change', applyTheme)
    document.documentElement.classList.toggle('dark', initialDark)
  })
  return { open, tab, theme, tools, saved, message, formError, installing, install, cancel, chooseSource, pathChanged, checkPath, save, stateLabel }
}
