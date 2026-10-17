import { ref } from 'vue'

export interface ServiceConfig {
  task_root_dir: string
  ai_chat_endpoint: string
  ai_model: string
  ai_api_key?: string
  ai_api_key_masked?: string
  ai_has_key?: boolean
  ai_timeout_seconds: number
}

export function useServiceConfig() {
  const config = ref<ServiceConfig>({
    task_root_dir: '',
    ai_chat_endpoint: '',
    ai_model: '',
    ai_timeout_seconds: 30,
  })
  const apiKeyDraft = ref('')
  const loading = ref(false)
  const saving = ref(false)
  const error = ref('')
  const message = ref('')

  let currentLoadSeq = 0
  let currentSaveSeq = 0

  async function load() {
    const seq = ++currentLoadSeq
    loading.value = true
    error.value = ''
    try {
      const res = await fetch('/api/system/service-config')
      if (!res.ok) throw new Error((await res.text()).trim() || '加载服务配置失败')
      const data: ServiceConfig = await res.json()
      if (seq !== currentLoadSeq) return
      config.value = data
      apiKeyDraft.value = ''
    } catch (e) {
      if (seq !== currentLoadSeq) return
      error.value = e instanceof Error ? e.message : '加载服务配置失败'
    } finally {
      if (seq === currentLoadSeq) {
        loading.value = false
      }
    }
  }

  async function save() {
    const seq = ++currentSaveSeq
    error.value = ''
    message.value = ''
    saving.value = true
    try {
      const payload: Partial<ServiceConfig> = {
        task_root_dir: config.value.task_root_dir.trim(),
        ai_chat_endpoint: config.value.ai_chat_endpoint.trim(),
        ai_model: config.value.ai_model.trim(),
        ai_timeout_seconds: Number(config.value.ai_timeout_seconds) || 30,
      }
      if (apiKeyDraft.value.trim()) {
        payload.ai_api_key = apiKeyDraft.value.trim()
      }

      const res = await fetch('/api/system/service-config', {
        method: 'PUT',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify(payload),
      })
      if (!res.ok) throw new Error((await res.text()).trim() || '保存服务配置失败')
      if (seq !== currentSaveSeq) return
      message.value = '服务配置已保存'
      await load()
    } catch (e) {
      if (seq !== currentSaveSeq) return
      error.value = e instanceof Error ? e.message : '保存服务配置失败'
    } finally {
      if (seq === currentSaveSeq) {
        saving.value = false
      }
    }
  }

  return {
    config,
    apiKeyDraft,
    loading,
    saving,
    error,
    message,
    load,
    save,
  }
}
