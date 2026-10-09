<script setup lang="ts">
import { AppWindow, ExternalLink, Home, Loader2, Maximize, Minimize, Power, TriangleAlert } from 'lucide-vue-next'
import { onMounted, onUnmounted, ref } from 'vue'
import { RouterLink } from 'vue-router'
import { Badge } from '@/components/ui/badge'
import { Button } from '@/components/ui/button'
import type { Device } from '@/api/devices'
import { ScrcpyDecoder } from '@/scrcpy/decoder'
import {
  ACTION_DOWN,
  ACTION_MOVE,
  ACTION_UP,
  AKEYCODE,
  ControlPacker,
  KEY_DOWN,
  KEY_UP,
  POINTER_FINGER_BASE,
  POINTER_MOUSE,
} from '@/scrcpy/protocol'
import { useDeviceStore } from '@/stores/devices'

const props = defineProps<{ device: Device }>()
const deviceStore = useDeviceStore()

const canvasRef = ref<HTMLCanvasElement | null>(null)
const wrapRef = ref<HTMLElement | null>(null)
const phoneRef = ref<HTMLElement | null>(null)

type TileState = 'loading' | 'live' | 'reconnecting' | 'error'
const state = ref<TileState>('loading')
const statusText = ref('连接中...')
const errorMsg = ref('')
const isLandscape = ref(false)
const fullscreen = ref(false)

const packer = new ControlPacker()
let decoder: ScrcpyDecoder | null = null
let ws: WebSocket | null = null
let reconnectTimer: number | undefined
let reconnectAttempts = 0
let serverErrorAttempts = 0
let lastServerError = ''
let errorShown = false
let destroyed = false
let mouseDown = false
let touchActive = false
let lastMousePoint: { x: number, y: number } | null = null
let lastTouchPoint: { x: number, y: number } | null = null
let resizeObserver: ResizeObserver | undefined
let pseudoFs = false

const serial = props.device.serial
const displayName = deviceStore.displayName(props.device)

function setPhoneRef(el: unknown) {
  phoneRef.value = el as HTMLElement | null
}

function setLive() {
  state.value = 'live'
}

function showLoading(text = '连接中...') {
  if (!errorShown) {
    state.value = 'loading'
    statusText.value = text
  }
}

function showError(msg: string) {
  errorShown = true
  state.value = 'error'
  errorMsg.value = msg
}

function connect() {
  if (destroyed)
    return
  const proto = location.protocol === 'https:' ? 'wss:' : 'ws:'
  ws = new WebSocket(`${proto}//${location.host}/ws?serial=${encodeURIComponent(serial)}`)
  ws.binaryType = 'arraybuffer'
  showLoading()
  const socket = ws

  socket.onopen = () => {
    if (ws !== socket)
      return
    reconnectAttempts = 0
    state.value = 'loading'
    statusText.value = '等待画面'
  }
  socket.onmessage = (ev) => {
    if (ws !== socket)
      return
    handleMessage(ev)
  }
  socket.onerror = () => {}
  socket.onclose = () => {
    if (ws !== socket || destroyed)
      return
    ws = null
    if (reconnectAttempts < 10) {
      reconnect()
    }
    else {
      statusText.value = '连接失败'
      showError(lastServerError || '连接失败')
    }
  }
}

function handleMessage(ev: MessageEvent) {
  try {
    if (typeof ev.data === 'string') {
      const msg = JSON.parse(ev.data) as { type: string, session_id?: string, codec?: string, width?: number, height?: number, message?: string }
      if (msg.type === 'meta') {
        packer.setDeviceSize(msg.width ?? 0, msg.height ?? 0)
        decoder?.configure(msg.codec || 'h264', msg.width ?? 0, msg.height ?? 0)
        serverErrorAttempts = 0
        lastServerError = ''
        errorShown = false
        errorMsg.value = ''
        setLive()
        isLandscape.value = (msg.width ?? 0) > (msg.height ?? 0)
        fitCanvas()
        return
      }
      if (msg.type === 'error') {
        // 服务端启动失败常见，静默重试；连续 3 次才亮红
        lastServerError = msg.message || '设备连接错误'
        serverErrorAttempts++
        console.warn('[dash] 服务端错误（第', serverErrorAttempts, '次），静默重试:', lastServerError)
        if (serverErrorAttempts >= 3) {
          statusText.value = '错误'
          showError(lastServerError)
        }
        return
      }
      if (msg.type === 'disconnected') {
        reconnect()
      }
      return
    }
    decoder?.handleFrame(new Uint8Array(ev.data as ArrayBuffer))
  }
  catch (e) {
    console.error('[dash] message error:', e)
  }
}

function reconnect() {
  if (destroyed || reconnectTimer !== undefined)
    return
  reconnectAttempts++
  state.value = 'reconnecting'
  statusText.value = '重连中'
  closeWs()
  reconnectTimer = window.setTimeout(() => {
    reconnectTimer = undefined
    connect()
  }, Math.min(1000 * reconnectAttempts, 5000))
}

function closeWs() {
  if (!ws)
    return
  const old = ws
  ws = null
  old.onclose = null
  old.onmessage = null
  old.onerror = null
  old.onopen = null
  if (old.readyState === WebSocket.OPEN || old.readyState === WebSocket.CONNECTING)
    old.close()
}

function send(bytes: Uint8Array) {
  if (ws && ws.readyState === WebSocket.OPEN)
    ws.send(bytes as Uint8Array<ArrayBuffer>)
}

function toDevice(clientX: number, clientY: number) {
  const rect = canvasRef.value?.getBoundingClientRect()
  const dw = packer.deviceWidth
  const dh = packer.deviceHeight
  if (!rect || !dw || !dh || rect.width === 0 || rect.height === 0)
    return { x: 0, y: 0 }
  const x = Math.round(((clientX - rect.left) * dw) / rect.width)
  const y = Math.round(((clientY - rect.top) * dh) / rect.height)
  return { x: Math.max(0, Math.min(dw - 1, x)), y: Math.max(0, Math.min(dh - 1, y)) }
}

function keyPress(keycode: number) {
  send(packer.keyCode(KEY_DOWN, keycode, 0))
  send(packer.keyCode(KEY_UP, keycode, 0))
}

// ---------- 全屏（作用于整卡） ----------
function isFs(): boolean {
  return !!(document.fullscreenElement || (document as Document & { webkitFullscreenElement?: Element }).webkitFullscreenElement)
    || pseudoFs
    || (phoneRef.value?.classList.contains('pseudo-fs') ?? false)
}

function onFsChange() {
  fullscreen.value = isFs()
  setTimeout(fitCanvas, 100)
}

function toggleFullscreen() {
  const el = phoneRef.value
  if (!el)
    return
  if (isFs()) {
    if (pseudoFs || el.classList.contains('pseudo-fs')) {
      pseudoFs = false
      el.classList.remove('pseudo-fs')
      onFsChange()
    }
    else {
      const doc = document as Document & { webkitExitFullscreen?: () => void }
      ;(document.exitFullscreen || doc.webkitExitFullscreen)?.call(document)
    }
    return
  }
  const req = el.requestFullscreen || (el as HTMLElement & { webkitRequestFullscreen?: () => Promise<void> }).webkitRequestFullscreen
  if (req) {
    const p = req.call(el) as Promise<void> | undefined
    p?.catch?.(() => {
      el.classList.add('pseudo-fs')
      onFsChange()
    })
  }
  else {
    el.classList.add('pseudo-fs')
    onFsChange()
  }
}

function fitCanvas() {
  const canvas = canvasRef.value
  const wrap = wrapRef.value
  if (!canvas || !wrap || !canvas.width || !canvas.height)
    return
  const availW = wrap.clientWidth
  const availH = wrap.clientHeight
  if (availW <= 0 || availH <= 0)
    return
  const ratio = canvas.width / canvas.height
  let dw = availW
  let dh = dw / ratio
  if (dh > availH) {
    dh = availH
    dw = dh * ratio
  }
  canvas.style.width = `${Math.round(dw)}px`
  canvas.style.height = `${Math.round(dh)}px`
}

// ---------- 输入事件（setup 作用域，卸载时可移除） ----------
function onMouseDown(e: MouseEvent) {
  e.preventDefault()
  if (e.button === 2) {
    send(packer.back(KEY_UP))
    return
  }
  mouseDown = true
  const { x, y } = toDevice(e.clientX, e.clientY)
  lastMousePoint = { x, y }
  send(packer.touch(ACTION_DOWN, POINTER_MOUSE, x, y, 1.0, 0x1, 0x1))
}

function onMouseMove(e: MouseEvent) {
  if (!mouseDown)
    return
  e.preventDefault()
  const { x, y } = toDevice(e.clientX, e.clientY)
  sendMouseMove(x, y)
}

function sendMouseMove(x: number, y: number) {
  if (lastMousePoint?.x === x && lastMousePoint.y === y)
    return
  lastMousePoint = { x, y }
  send(packer.touch(ACTION_MOVE, POINTER_MOUSE, x, y, 1.0, 0x1, 0x1))
}

function onMouseUp(e: MouseEvent) {
  if (!mouseDown)
    return
  mouseDown = false
  const { x, y } = toDevice(e.clientX, e.clientY)
  sendMouseMove(x, y)
  send(packer.touch(ACTION_UP, POINTER_MOUSE, x, y, 0.0, 0, 0))
  lastMousePoint = null
}

function onWheel(e: WheelEvent) {
  e.preventDefault()
  const { x, y } = toDevice(e.clientX, e.clientY)
  send(packer.scroll(x, y, e.deltaX, e.deltaY))
}

function onContextMenu(e: Event) {
  e.preventDefault()
}

function onTouchStart(e: TouchEvent) {
  e.preventDefault()
  const t = e.touches[0]
  touchActive = true
  const { x, y } = toDevice(t.clientX, t.clientY)
  lastTouchPoint = { x, y }
  send(packer.touch(ACTION_DOWN, POINTER_FINGER_BASE, x, y, t.force || 1.0))
}

function onTouchMove(e: TouchEvent) {
  if (!touchActive)
    return
  e.preventDefault()
  const t = e.touches[0]
  const { x, y } = toDevice(t.clientX, t.clientY)
  sendTouchMove(x, y, t.force || 1.0)
}

function sendTouchMove(x: number, y: number, pressure: number) {
  if (lastTouchPoint?.x === x && lastTouchPoint.y === y)
    return
  lastTouchPoint = { x, y }
  send(packer.touch(ACTION_MOVE, POINTER_FINGER_BASE, x, y, pressure))
}

function onTouchEnd(e: TouchEvent) {
  if (!touchActive)
    return
  touchActive = false
  const t = e.changedTouches[0]
  const { x, y } = toDevice(t.clientX, t.clientY)
  sendTouchMove(x, y, t.force || 1.0)
  send(packer.touch(ACTION_UP, POINTER_FINGER_BASE, x, y, 0))
  lastTouchPoint = null
}

onMounted(() => {
  const canvas = canvasRef.value
  if (!canvas)
    return
  decoder = new ScrcpyDecoder(canvas)
  decoder.onDeviceResize = (w, h) => {
    packer.setDeviceSize(w, h)
    isLandscape.value = w > h
    fitCanvas()
  }

  canvas.addEventListener('mousedown', onMouseDown)
  window.addEventListener('mousemove', onMouseMove)
  window.addEventListener('mouseup', onMouseUp)
  canvas.addEventListener('wheel', onWheel, { passive: false })
  canvas.addEventListener('contextmenu', onContextMenu)
  canvas.addEventListener('touchstart', onTouchStart, { passive: false })
  canvas.addEventListener('touchmove', onTouchMove, { passive: false })
  canvas.addEventListener('touchend', onTouchEnd)

  document.addEventListener('fullscreenchange', onFsChange)
  document.addEventListener('webkitfullscreenchange', onFsChange)
  resizeObserver = new ResizeObserver(() => fitCanvas())
  if (wrapRef.value)
    resizeObserver.observe(wrapRef.value)
  connect()
})

onUnmounted(() => {
  destroyed = true
  if (reconnectTimer !== undefined)
    window.clearTimeout(reconnectTimer)
  document.removeEventListener('fullscreenchange', onFsChange)
  document.removeEventListener('webkitfullscreenchange', onFsChange)
  resizeObserver?.disconnect()
  const canvas = canvasRef.value
  canvas?.removeEventListener('mousedown', onMouseDown)
  window.removeEventListener('mousemove', onMouseMove)
  window.removeEventListener('mouseup', onMouseUp)
  canvas?.removeEventListener('wheel', onWheel)
  canvas?.removeEventListener('contextmenu', onContextMenu)
  canvas?.removeEventListener('touchstart', onTouchStart)
  canvas?.removeEventListener('touchmove', onTouchMove)
  canvas?.removeEventListener('touchend', onTouchEnd)
  decoder?.close()
  closeWs()
})
</script>

<template>
  <div
    :ref="setPhoneRef"
    class="dash-phone relative flex flex-col overflow-hidden rounded-xl border bg-card"
    :class="{ 'landscape': isLandscape }"
  >
    <!-- 画面区 -->
      <div ref="wrapRef" class="relative flex min-h-40 items-center justify-center bg-muted/30">
      <canvas ref="canvasRef" class="touch-none select-none" :aria-label="`${displayName} 投屏画面`" />

      <div v-if="state !== 'live'" class="absolute inset-0 flex flex-col items-center justify-center gap-2 bg-background/80 p-3 text-center backdrop-blur-sm">
        <Loader2 v-if="state !== 'error'" class="h-6 w-6 animate-spin text-white" />
        <TriangleAlert v-else class="h-6 w-6 text-destructive" />
        <p class="text-xs text-foreground/80">{{ state === 'error' ? errorMsg : statusText }}</p>
        <Button v-if="state === 'error'" variant="secondary" size="sm" as-child>
          <RouterLink :to="{ name: 'player', query: { serial } }">在播放器中打开<ExternalLink class="h-3.5 w-3.5" /></RouterLink>
        </Button>
      </div>
    </div>

    <!-- 信息 + 工具栏 -->
    <div class="flex items-center gap-1 px-2 py-1.5">
      <span class="min-w-0 flex-1 truncate text-xs font-medium" :title="serial">{{ displayName }}</span>
      <Badge v-if="state !== 'live'" variant="secondary" class="text-[10px">{{ statusText }}</Badge>
      <Button variant="ghost" size="icon" class="h-7 w-7" title="返回主页" :aria-label="`${displayName} 返回主页`" @click="keyPress(AKEYCODE.HOME)">
        <Home class="h-4 w-4" />
      </Button>
      <Button variant="ghost" size="icon" class="h-7 w-7" title="切换任务" :aria-label="`${displayName} 切换任务`" @click="keyPress(AKEYCODE.APP_SWITCH)">
        <AppWindow class="h-4 w-4" />
      </Button>
      <Button variant="ghost" size="icon" class="h-7 w-7" title="点亮/熄屏" :aria-label="`${displayName} 点亮熄屏`" @click="keyPress(AKEYCODE.POWER)">
        <Power class="h-4 w-4" />
      </Button>
      <Button variant="ghost" size="icon" class="h-7 w-7" :title="fullscreen ? '退出全屏' : '全屏'" :aria-label="`${displayName} 全屏`" @click="toggleFullscreen()">
        <Minimize v-if="fullscreen" class="h-4 w-4" />
        <Maximize v-else class="h-4 w-4" />
      </Button>
    </div>
  </div>
</template>

<style scoped>
.dash-phone.landscape {
  flex-direction: row;
}
.dash-phone.landscape > div:first-child {
  flex: 1;
  min-width: 0;
}
.pseudo-fs {
  position: fixed;
  inset: 0;
  z-index: 50;
  border-radius: 0;
}
</style>
