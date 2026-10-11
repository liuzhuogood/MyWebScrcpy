import { onMounted, onUnmounted } from 'vue'
import { useStorage } from '@vueuse/core'
import type { OpPayload, OpType } from '@/inspector/opLog'
import { COPY_KEY_COPY, COPY_KEY_CUT, type ControlPacker } from './protocol'

function fallbackCopyText(text: string): boolean {
  const active = document.activeElement as HTMLElement | null
  const textarea = document.createElement('textarea')
  textarea.value = text
  textarea.setAttribute('readonly', '')
  textarea.style.position = 'fixed'
  textarea.style.top = '-9999px'
  textarea.style.opacity = '0'
  document.body.appendChild(textarea)
  textarea.select()
  let copied = false
  try {
    copied = document.execCommand('copy')
  }
  catch {
    copied = false
  }
  finally {
    document.body.removeChild(textarea)
    active?.focus?.()
  }
  return copied
}

async function writeSystemClipboard(text: string): Promise<void> {
  if (window.isSecureContext && navigator.clipboard?.writeText) {
    try {
      await navigator.clipboard.writeText(text)
      return
    }
    catch {
      // 失焦或浏览器策略拒绝时回退 execCommand
    }
  }
  if (!fallbackCopyText(text))
    console.warn('[clipboard] 写入系统剪贴板失败')
}

export function useClipboardSync(callbacks: {
  send: (bytes: Uint8Array) => void
  packer: ControlPacker
  onOp: (type: OpType, data: OpPayload) => void
  isInputBlocked: (target: EventTarget | null) => boolean
}) {
  const clipboardSync = useStorage('clipboard-sync', true)
  let lastSyncedText = ''

  function onPaste(e: ClipboardEvent) {
    if (!clipboardSync.value || callbacks.isInputBlocked(e.target))
      return
    const text = e.clipboardData?.getData('text/plain')
    if (!text)
      return
    e.preventDefault()
    lastSyncedText = text
    callbacks.send(callbacks.packer.setClipboard(text, true))
    callbacks.onOp('text', { text })
  }

  function onKeyDown(e: KeyboardEvent) {
    if (!clipboardSync.value || callbacks.isInputBlocked(e.target))
      return
    if (e.altKey || e.shiftKey || !(e.ctrlKey || e.metaKey))
      return
    const key = e.key.toLowerCase()
    if (key !== 'c' && key !== 'x')
      return
    // 页面内已选中文本时保留浏览器原生复制行为
    if (window.getSelection()?.toString())
      return
    e.preventDefault()
    const copyKey = key === 'x' ? COPY_KEY_CUT : COPY_KEY_COPY
    callbacks.send(callbacks.packer.getClipboard(copyKey))
  }

  async function syncFromSystemOnFocus() {
    if (!clipboardSync.value || !window.isSecureContext || !navigator.clipboard?.readText)
      return
    try {
      if (navigator.permissions?.query) {
        const status = await navigator.permissions.query({ name: 'clipboard-read' as PermissionName })
        if (status.state !== 'granted')
          return
      }
      const text = await navigator.clipboard.readText()
      if (!text || text === lastSyncedText)
        return
      lastSyncedText = text
      callbacks.send(callbacks.packer.setClipboard(text, false))
    }
    catch {
      // 未授权或浏览器不支持无手势读取时静默忽略，由 paste 事件兜底
    }
  }

  function onWindowFocus() {
    void syncFromSystemOnFocus()
  }

  function onDeviceClipboard(text: string) {
    if (!clipboardSync.value || !text || text === lastSyncedText)
      return
    lastSyncedText = text
    void writeSystemClipboard(text)
  }

  onMounted(() => {
    window.addEventListener('paste', onPaste)
    window.addEventListener('keydown', onKeyDown)
    window.addEventListener('focus', onWindowFocus)
  })

  onUnmounted(() => {
    window.removeEventListener('paste', onPaste)
    window.removeEventListener('keydown', onKeyDown)
    window.removeEventListener('focus', onWindowFocus)
  })

  return {
    clipboardSync,
    onDeviceClipboard,
  }
}
