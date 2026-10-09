import { onUnmounted, ref } from 'vue'

const MAX_RECONNECT = 3

export function useAdbShell(serial: string) {
  const status = ref<'disconnected' | 'connecting' | 'connected' | 'error'>('disconnected')
  const errorMessage = ref('')
  let ws: WebSocket | null = null
  let reconnectTimer: number | undefined
  let attempts = 0
  let onOutput: ((data: string | Uint8Array) => void) | undefined
  let onResize: (() => void) | undefined

  function connect() {
    if (!serial) return
    if (ws) ws.close()
    status.value = 'connecting'
    errorMessage.value = ''
    const protocol = location.protocol === 'https:' ? 'wss:' : 'ws:'
    const socket = new WebSocket(`${protocol}//${location.host}/ws/adb-shell?serial=${encodeURIComponent(serial)}`)
    socket.binaryType = 'arraybuffer'
    ws = socket
    socket.onopen = () => {
      if (ws !== socket) return
      attempts = 0
      status.value = 'connected'
      onResize?.()
    }
    socket.onmessage = event => {
      if (typeof event.data === 'string') onOutput?.(event.data)
      else if (event.data instanceof ArrayBuffer) onOutput?.(new Uint8Array(event.data))
    }
    socket.onerror = () => {
      if (ws === socket) status.value = 'error'
    }
    socket.onclose = () => {
      if (ws !== socket) return
      ws = null
      status.value = 'error'
      if (attempts < MAX_RECONNECT) {
        const delay = 1500 * 2 ** attempts
        attempts++
        reconnectTimer = window.setTimeout(connect, delay)
      } else {
        errorMessage.value = `连接已断开，已重试 ${MAX_RECONNECT} 次`
      }
    }
  }

  function send(message: Record<string, unknown>) {
    if (ws?.readyState === WebSocket.OPEN) ws.send(JSON.stringify(message))
  }

  function input(data: string) { send({ type: 'input', data }) }
  function resize(cols: number, rows: number) { send({ type: 'resize', cols, rows }) }
  function retry() { attempts = 0; errorMessage.value = ''; connect() }

  function close() {
    if (reconnectTimer !== undefined) window.clearTimeout(reconnectTimer)
    reconnectTimer = undefined
    if (ws) { ws.onclose = null; ws.close(); ws = null }
    status.value = 'disconnected'
  }

  function configure(options: { output: (data: string | Uint8Array) => void, resize: () => void }) {
    onOutput = options.output
    onResize = options.resize
  }

  onUnmounted(close)
  return { status, errorMessage, connect, close, retry, input, resize, configure }
}
