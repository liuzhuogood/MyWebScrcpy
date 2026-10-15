import { computed, onMounted, onUnmounted, ref } from 'vue'
import type { Ref } from 'vue'
import { invoke, isTauri } from '@tauri-apps/api/core'
import { toast } from 'vue-sonner'

export interface PlayerWindowState {
  serial: string
  opacity: number
  clickThrough: boolean
  revision: number
}

interface Snapshot {
  windows: PlayerWindowState[]
  recoveryShortcut: string | null
}

const snapshot = ref<Snapshot>({ windows: [], recoveryShortcut: null })
let users = 0
let timer: ReturnType<typeof setInterval> | undefined
let revision = 0
let refreshing = false

async function refresh() {
  const request = ++revision
  const next = await invoke<Snapshot>('list_player_windows')
  if (request === revision)
    snapshot.value = next
}

async function poll() {
  if (refreshing) return
  refreshing = true
  try {
    await refresh()
  }
  catch (error) {
    console.warn('[Desktop] 读取小窗状态失败', error)
  }
  finally {
    refreshing = false
  }
}

export function usePlayerWindows(serial: Ref<string>) {
  const native = isTauri()
  const busy = ref(false)
  const player = computed(() => snapshot.value.windows.find(item => item.serial === serial.value))
  const needsRecovery = computed(() => Boolean(player.value && (player.value.opacity < 100 || player.value.clickThrough)))
  const shortcut = computed(() => snapshot.value.recoveryShortcut)
  let pendingOpacity: { opacity: number, expectedRevision: number } | undefined

  onMounted(() => {
    if (!native) return
    if (users++ === 0) {
      void poll()
      timer = setInterval(() => void poll(), 1000)
    }
  })

  onUnmounted(() => {
    if (!native) return
    if (--users === 0) {
      clearInterval(timer)
      timer = undefined
      ++revision
    }
  })

  let pendingClickThrough: { enabled: boolean, expectedRevision: number } | undefined
  let pendingRestore = false

  async function drain() {
    if (!native || busy.value) return
    busy.value = true
    ++revision
    try {
      while (pendingRestore || pendingOpacity !== undefined || pendingClickThrough !== undefined) {
        if (pendingRestore) {
          pendingRestore = false
          pendingOpacity = undefined
          pendingClickThrough = undefined
          await invoke('restore_player_window', { serial: serial.value })
          await refresh()
        }
        else if (pendingOpacity !== undefined) {
          const args = pendingOpacity
          pendingOpacity = undefined
          await invoke('set_player_opacity', args)
        }
        else if (pendingClickThrough !== undefined) {
          const args = pendingClickThrough
          pendingClickThrough = undefined
          await invoke('set_player_click_through', args)
        }
      }
      await refresh()
    }
    catch (error) {
      pendingOpacity = undefined
      pendingClickThrough = undefined
      console.error('[Desktop] 小窗操作失败', error)
      toast.error(String(error))
      await poll()
    }
    finally {
      busy.value = false
      // A recovery requested during an in-flight call must not be dropped.
      if (pendingRestore || pendingOpacity !== undefined || pendingClickThrough !== undefined)
        void drain()
    }
  }

  function setOpacity(opacity: number) {
    if (!player.value) return
    pendingOpacity = { opacity, expectedRevision: player.value.revision }
    return drain()
  }

  function setClickThrough(enabled: boolean) {
    if (!player.value) return
    pendingClickThrough = { enabled, expectedRevision: player.value.revision }
    return drain()
  }

  function restore() {
    pendingRestore = true
    return drain()
  }

  return {
    native, player, needsRecovery, shortcut, busy,
    setOpacity,
    setClickThrough, restore,
  }
}
