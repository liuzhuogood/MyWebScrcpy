import { computed, ref } from 'vue'
import { devices, initialContent, initialTasks, templates, type DemoTask } from './data'

interface Message { role: 'user' | 'assistant'; content: string }
interface RunResult { serial: string; status: '运行中' | '已终止' | '设备离线' | '模板不可用' }

// 原型状态仅驻留内存，不连接设备、模型、数据库或业务接口。
export function useWorkbench() {
  const tasks = ref<DemoTask[]>(structuredClone(initialTasks))
  const drafts = ref<Record<number, string>>(Object.fromEntries(initialTasks.map(task => [task.id, task.content])))
  const currentId = ref(1)
  const current = computed(() => {
    const task = tasks.value.find(item => item.id === currentId.value)
    if (!task) throw new Error('原型当前任务不存在')
    return task
  })
  const content = computed({ get: () => drafts.value[currentId.value], set: value => { drafts.value[currentId.value] = value } })
  const dirty = computed(() => content.value !== current.value.content)
  const tab = ref('source')
  const taskSearch = ref('')
  const filteredTasks = computed(() => tasks.value.filter(task => task.name.includes(taskSearch.value.trim())))
  const notice = ref('')
  const formOpen = ref(false)
  const editing = ref(false)
  const formName = ref('')
  const formDevices = ref<string[]>([])
  const formError = ref('')
  const formSearch = ref('')
  const filteredDevices = computed(() => devices.filter(item => `${item.alias} ${item.serial}`.toLowerCase().includes(formSearch.value.toLowerCase())))
  const templateOpen = ref(false)
  const templateSearch = ref('')
  const templateScope = ref('all')
  const availableTemplates = computed(() => templates.filter(asset => asset.serial === 'global' || current.value.target_devices.includes(asset.serial)))
  const visibleTemplates = computed(() => availableTemplates.value.filter(asset => (templateScope.value === 'all' || asset.serial === templateScope.value) && asset.name.includes(templateSearch.value.trim())))
  const chats = ref<Record<number, Message[]>>(Object.fromEntries(initialTasks.map(task => [task.id, [{ role: 'assistant', content: '想调整哪个步骤？' }]])))
  const messages = computed(() => chats.value[currentId.value])
  const prompt = ref('')
  const proposals = ref<Record<number, string>>({})
  const runs = ref<Record<number, RunResult[]>>({})
  const currentRuns = computed(() => runs.value[currentId.value] ?? [])
  const hasRunning = computed(() => currentRuns.value.some(item => item.status === '运行中'))
  const mobilePanel = ref('editor')

  function selectTask(id: number) {
    currentId.value = id
    templateScope.value = 'all'
    templateSearch.value = ''
    prompt.value = ''
    notice.value = ''
    mobilePanel.value = 'editor'
  }

  function openForm(isEditing: boolean) {
    editing.value = isEditing
    formName.value = isEditing ? current.value.name : ''
    formDevices.value = isEditing ? [...current.value.target_devices] : []
    formError.value = ''
    formSearch.value = ''
    formOpen.value = true
  }

  function toggleDevice(serial: string, checked: boolean) {
    formDevices.value = checked ? [...new Set([...formDevices.value, serial])] : formDevices.value.filter(item => item !== serial)
    formError.value = ''
  }

  function submitForm() {
    if (!formName.value.trim()) { formError.value = '请输入任务标题'; return }
    if (!formDevices.value.length) { formError.value = '请至少选择一台目标设备'; return }
    if (editing.value) {
      current.value.name = formName.value.trim()
      current.value.target_devices = [...formDevices.value]
    } else {
      const id = Math.max(...tasks.value.map(item => item.id)) + 1
      const name = formName.value.trim()
      const newContent = initialContent.replace('每日应用巡检', name).replace('enabled: true', 'enabled: false')
      tasks.value.unshift({ id, name, target_devices: [...formDevices.value], content: newContent, version: '1.0.0', schedule: '每天 09:00', timeout: 120, enabled: false, nextRun: '未排期', lastResult: '未运行', compiledPython: '', compiledContent: '' })
      drafts.value[id] = newContent
      chats.value[id] = [{ role: 'assistant', content: '想调整哪个步骤？' }]
      selectTask(id)
      tab.value = 'source'
    }
    templateScope.value = 'all'
    formOpen.value = false
  }

  function save() { current.value.content = content.value; notice.value = '已保存' }

  function compile() {
    if (dirty.value) { notice.value = '请先保存文档'; return }
    current.value.compiledPython = `import argparse\n\n\ndef run(device_serial: str):\n    # ${current.value.name}\n    print(f"开始执行：{device_serial}")\n\n\nif __name__ == "__main__":\n    parser = argparse.ArgumentParser()\n    parser.add_argument("--serial", required=True)\n    args = parser.parse_args()\n    run(args.serial)\n`
    current.value.compiledContent = current.value.content
    tab.value = 'python'
    notice.value = ''
  }

  function insertTemplate(id: string) {
    const asset = templates.find(item => item.id === id)
    if (!asset) throw new Error('原型模板不存在')
    content.value += `\n![${asset.name}](template://${asset.id}?threshold=${asset.threshold})\n`
    templateOpen.value = false
    tab.value = 'source'
  }

  function sendPrompt() {
    if (!prompt.value.trim()) return
    messages.value.push({ role: 'user', content: prompt.value.trim() })
    proposals.value[currentId.value] = content.value.includes('- 等待 3 秒') ? content.value.replace('- 等待 3 秒', '- 等待 5 秒\n- 若页面未加载完成，重试 1 次') : `${content.value}\n- 若页面未加载完成，重试 1 次\n`
    messages.value.push({ role: 'assistant', content: '建议将页面等待调整为 5 秒，并增加 1 次重试。' })
    prompt.value = ''
  }

  function applyProposal() {
    content.value = proposals.value[currentId.value]
    delete proposals.value[currentId.value]
    tab.value = 'source'
    mobilePanel.value = 'editor'
  }

  function run() {
    if (dirty.value || current.value.compiledContent !== current.value.content) { notice.value = '请先保存并生成 Python'; return }
    const refs = Array.from(current.value.content.matchAll(/template:\/\/([^?\s)]+)/g), match => match[1])
    runs.value[currentId.value] = current.value.target_devices.map(serial => {
      const device = devices.find(item => item.serial === serial)
      if (!device) throw new Error('原型设备不存在')
      if (device.state !== 'device') return { serial, status: '设备离线' }
      const invalid = refs.some(id => {
        const asset = templates.find(item => item.id === id)
        return !asset || (asset.serial !== 'global' && asset.serial !== serial)
      })
      return { serial, status: invalid ? '模板不可用' : '运行中' }
    })
    notice.value = ''
  }

  function stop(serial: string) {
    const result = currentRuns.value.find(item => item.serial === serial)
    if (result) result.status = '已终止'
  }

  return { tasks, current, content, dirty, tab, taskSearch, filteredTasks, notice, formOpen, editing, formName, formDevices, formError, formSearch, filteredDevices, templateOpen, templateSearch, templateScope, visibleTemplates, messages, prompt, proposals, currentRuns, hasRunning, mobilePanel, selectTask, openForm, toggleDevice, submitForm, save, compile, insertTemplate, sendPrompt, applyProposal, run, stop }
}
