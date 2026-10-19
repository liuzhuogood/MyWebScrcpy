import { onMounted, onUnmounted, ref } from 'vue'
import { invoke, isTauri } from '@tauri-apps/api/core'
import { listen } from '@tauri-apps/api/event'
import type { UnlistenFn } from '@tauri-apps/api/event'
import { toast } from 'vue-sonner'

export interface MainWindowState {
  alwaysOnTop: boolean
  opacity: number
  clickThrough: boolean
}

export function useMainWindowEffects() {
  const native = isTauri()
  const state = ref<MainWindowState>({
    alwaysOnTop: false,
    opacity: 100,
    clickThrough: false,
  })
  const shortcut = ref<string | null>(null)
  const busy = ref(false)
  let unlisten: UnlistenFn | undefined

  function applyDomEffects(s: MainWindowState) {
    if (typeof document === 'undefined') return
    if (s.opacity < 100) {
      document.documentElement.classList.add('desktop-translucent')
      document.documentElement.style.setProperty('--window-opacity', String(s.opacity / 100))
    }
    else {
      document.documentElement.classList.remove('desktop-translucent')
      document.documentElement.style.removeProperty('--window-opacity')
    }
  }

  async function fetchState() {
    if (!native) return
    try {
      const current = await invoke<MainWindowState>('get_main_window_state')
      state.value = current
      applyDomEffects(current)

      // 同步获取全局恢复快捷键
      const snapshot = await invoke<{ recoveryShortcut: string | null }>('list_player_windows')
      shortcut.value = snapshot.recoveryShortcut
    }
    catch (e) {
      console.warn('[Desktop] 获取主窗口状态失败', e)
    }
  }

  onMounted(async () => {
    if (!native) return
    await fetchState()
    try {
      unlisten = await listen<MainWindowState>('main-window-effects-changed', event => {
        state.value = event.payload
        applyDomEffects(event.payload)
      })
    }
    catch (e) {
      console.warn('[Desktop] 监听主窗口效果事件失败', e)
    }
  })

  onUnmounted(() => {
    if (unlisten) {
      unlisten()
      unlisten = undefined
    }
  })

  async function setOpacity(opacity: number) {
    if (!native || busy.value) return
    busy.value = true
    try {
      await invoke('set_main_window_opacity', { opacity })
      state.value.opacity = opacity
      applyDomEffects(state.value)
    }
    catch (e) {
      console.error('[Desktop] 设置主窗口透明度失败', e)
      toast.error(String(e))
    }
    finally {
      busy.value = false
    }
  }

  async function setClickThrough(enabled: boolean) {
    if (!native || busy.value) return
    busy.value = true
    try {
      await invoke('set_main_window_click_through', { enabled })
      state.value.clickThrough = enabled
    }
    catch (e) {
      console.error('[Desktop] 设置主窗口鼠标穿透失败', e)
      toast.error(String(e))
    }
    finally {
      busy.value = false
    }
  }

  async function toggleAlwaysOnTop() {
    if (!native || busy.value) return
    busy.value = true
    try {
      const next = !state.value.alwaysOnTop
      await invoke('set_main_window_always_on_top', { alwaysOnTop: next })
      state.value.alwaysOnTop = next
    }
    catch (e) {
      console.error('[Desktop] 切换主窗口置顶失败', e)
      toast.error(String(e))
    }
    finally {
      busy.value = false
    }
  }

  async function restore() {
    if (!native || busy.value) return
    busy.value = true
    try {
      await invoke('restore_main_window')
      state.value = { alwaysOnTop: false, opacity: 100, clickThrough: false }
      applyDomEffects(state.value)
    }
    catch (e) {
      console.error('[Desktop] 恢复主窗口失败', e)
      toast.error(String(e))
    }
    finally {
      busy.value = false
    }
  }

  return {
    native,
    state,
    shortcut,
    busy,
    setOpacity,
    setClickThrough,
    toggleAlwaysOnTop,
    restore,
  }
}
