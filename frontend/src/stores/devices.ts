import { defineStore } from 'pinia'
import { computed, ref } from 'vue'
import {
  type Device,
  addManualDevice,
  fetchDevices,
  reconnectDevice,
  removeManualDevice,
  updateDeviceAlias,
} from '@/api/devices'
import { MOCK_DEVICES } from '@/mocks/devices'

const ALIAS_KEY = 'device_aliases'
const MOCK_KEY = 'mywebscrcpy.mock'
const POLL_MS = 3000

function readAliases(): Record<string, string> {
  try {
    return JSON.parse(localStorage.getItem(ALIAS_KEY) || '{}')
  }
  catch {
    return {}
  }
}

export const useDeviceStore = defineStore('devices', () => {
  const devices = ref<Device[]>([])
  const loading = ref(false)
  const error = ref('')
  const keyword = ref('')
  const sourceFilter = ref<'all' | 'auto' | 'manual'>('all')
  const editingSerial = ref<string | null>(null)
  const aliases = ref<Record<string, string>>(readAliases())
  const mockEnabled = ref(import.meta.env.DEV && localStorage.getItem(MOCK_KEY) === '1')
  let timer: number | undefined

  const displayName = (d: Device) => d.alias?.trim() || aliases.value[d.serial]?.trim() || d.model || '未知设备'

  const filtered = computed(() => {
    let list = devices.value
    if (sourceFilter.value !== 'all') {
      list = list.filter(d => (d.source || 'auto') === sourceFilter.value)
    }

    const k = keyword.value.trim().toLowerCase()
    if (!k)
      return list
    return list.filter(d =>
      [displayName(d), d.serial, d.model, d.product].filter(Boolean)
        .join(' ').toLowerCase().includes(k),
    )
  })

  async function load(silent = false) {
    if (mockEnabled.value) {
      // Mock 模式：本地假数据，不请求后端
      await new Promise(r => setTimeout(r, silent ? 0 : 300))
      devices.value = MOCK_DEVICES
      error.value = ''
      loading.value = false
      return
    }
    if (!silent)
      loading.value = true
    try {
      devices.value = await fetchDevices()
      error.value = ''
    }
    catch (e) {
      error.value = e instanceof Error ? e.message : '加载失败'
    }
    finally {
      loading.value = false
    }
  }

  function setMock(on: boolean) {
    mockEnabled.value = on
    localStorage.setItem(MOCK_KEY, on ? '1' : '0')
    load()
  }

  function startPolling() {
    stopPolling()
    load()
    timer = window.setInterval(() => {
      if (!editingSerial.value)
        load(true)
    }, POLL_MS)
  }

  function stopPolling() {
    if (timer)
      window.clearInterval(timer)
    timer = undefined
  }

  async function saveAlias(serial: string, name: string, originalModel: string) {
    const next = { ...aliases.value }
    const v = name.trim()
    if (!v || v === originalModel)
      delete next[serial]
    else
      next[serial] = v.slice(0, 20)
    aliases.value = next
    localStorage.setItem(ALIAS_KEY, JSON.stringify(next))

    try {
      await updateDeviceAlias(serial, v)
      await load(true)
    }
    catch {
      // 容错处理
    }
  }

  async function addDevice(serial: string, alias?: string) {
    await addManualDevice(serial, alias)
    await load(true)
  }

  async function removeDevice(serial: string) {
    await removeManualDevice(serial)
    await load(true)
  }

  async function reconnect(serial: string) {
    await reconnectDevice(serial)
    await load(true)
  }

  return {
    devices, loading, error, keyword, sourceFilter, editingSerial, aliases, mockEnabled,
    filtered, displayName, load, startPolling, stopPolling, saveAlias, setMock,
    addDevice, removeDevice, reconnect,
  }
})
