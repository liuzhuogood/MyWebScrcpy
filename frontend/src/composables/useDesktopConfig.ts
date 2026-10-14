import { ref } from 'vue'

export interface DesktopConfig {
  port: number
  effective: number
  env_port: number
  is_overridden: boolean
}

export function useDesktopConfig() {
  const config = ref<DesktopConfig>()
  const draftPort = ref<number | string>(1989)
  const loading = ref(false)
  const saving = ref(false)
  const error = ref('')
  const message = ref('')
  const isDesktop = ref(false)

  function headers(): Record<string, string> {
    const desktopToken = (window as Window & { __MYWEBSCRCPY_DESKTOP_TOKEN__?: string }).__MYWEBSCRCPY_DESKTOP_TOKEN__
    return { 'Content-Type': 'application/json', ...(desktopToken ? { Authorization: `Bearer ${desktopToken}` } : {}) }
  }

  async function load() {
    const desktopToken = (window as Window & { __MYWEBSCRCPY_DESKTOP_TOKEN__?: string }).__MYWEBSCRCPY_DESKTOP_TOKEN__
    isDesktop.value = Boolean(desktopToken)
    if (!desktopToken) return

    loading.value = true
    error.value = ''
    try {
      const res = await fetch('/api/desktop/config', { headers: headers() })
      if (!res.ok) throw new Error((await res.text()).trim() || '加载配置失败')
      const data: DesktopConfig = await res.json()
      config.value = data
      draftPort.value = data.port || 1989
    } catch (e) {
      error.value = e instanceof Error ? e.message : '加载配置失败'
    } finally {
      loading.value = false
    }
  }

  async function save() {
    error.value = ''
    message.value = ''
    const port = Number(draftPort.value)
    if (!Number.isInteger(port) || port < 1 || port > 65535) {
      error.value = '端口必须在 1 到 65535 之间'
      return
    }

    saving.value = true
    try {
      const res = await fetch('/api/desktop/config', {
        method: 'PUT',
        headers: headers(),
        body: JSON.stringify({ port }),
      })
      if (!res.ok) throw new Error((await res.text()).trim() || '保存失败')
      message.value = '已保存，重启应用后生效'
      await load()
    } catch (e) {
      error.value = e instanceof Error ? e.message : '保存失败'
    } finally {
      saving.value = false
    }
  }

  return {
    config,
    draftPort,
    loading,
    saving,
    error,
    message,
    isDesktop,
    load,
    save,
  }
}
