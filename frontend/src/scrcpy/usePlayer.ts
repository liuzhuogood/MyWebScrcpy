import { onMounted, onUnmounted, ref } from 'vue'
import { inspectorConsumesInput } from '@/inspector/guard'
import { type OpPayload, type OpType } from '@/inspector/opLog'
import { DeviceAudio, type AudioStatus } from './audio'
import { ScrcpyDecoder } from './decoder'
import { PictureInPicture } from './pip'
import { useClipboardSync } from './useClipboardSync'
import { useScreenPower } from './useScreenPower'
import { usePlayerSettings } from '@/composables/usePlayerSettings'
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
  keyboardEventToKeyCode,
} from './protocol'

const MAX_RECONNECT = 20
// adb 启动 scrcpy server 在冷启动、无线设备或设备负载较高时可能超过 3 秒。
const CONNECT_TIMEOUT_MS = 10000
const NO_FRAME_TIMEOUT_MS = 5000
const SCREEN_STATE_POLL_MS = 2000
const DEFAULT_VIDEO_WIDTH = 472
const DEFAULT_VIDEO_HEIGHT = 1024

function loadCachedVideoSize(serial: string): { w: number, h: number } {
  try {
    const raw = localStorage.getItem(`scrcpy:video-size:${serial}`)
    if (raw) {
      const parsed = JSON.parse(raw) as { w?: number, h?: number }
      if (parsed.w && parsed.w > 0 && parsed.h && parsed.h > 0)
        return { w: parsed.w, h: parsed.h }
    }
  }
  catch {}
  return { w: DEFAULT_VIDEO_WIDTH, h: DEFAULT_VIDEO_HEIGHT }
}

function saveCachedVideoSize(serial: string, w: number, h: number) {
  if (w <= 0 || h <= 0)
    return
  try {
    localStorage.setItem(`scrcpy:video-size:${serial}`, JSON.stringify({ w, h }))
  }
  catch {}
}

interface WsMeta {
  type: string
  session_id?: string
  codec?: string
  width?: number
  height?: number
  audio_available?: boolean
  audio_reason?: string
  status?: string
  reason?: string
  message?: string
  text?: string
}
export interface TouchVisualEvent { kind: 'down' | 'move' | 'up' | 'tap' | 'swipe', x: number, y: number, x2?: number, y2?: number }

/** 投屏连接：WS 生命周期 + 解码 + 输入 + 工具栏动作，视图只做渲染 */
export function usePlayer(serial: string, opts: { onOp?: (type: OpType, data: OpPayload) => void, onTouch?: (event: TouchVisualEvent) => void, onDisconnect?: () => void, onFrame?: () => void } = {}) {
  const playerSettings = usePlayerSettings()
  const op = (type: OpType, data: OpPayload) => opts.onOp?.(type, data)
  const canvasRef = ref<HTMLCanvasElement | null>(null)
  const stageRef = ref<HTMLElement | null>(null)

  const loading = ref(true)
  const errorMsg = ref('')
  const statusText = ref('连接中...')
  const live = ref(false)
  const fps = ref('')
  const showFps = ref(false)
  const videoSize = ref({ w: 0, h: 0 })
  let decodedFrameVersion = 0
  const screenOff = ref(false)
  const displayScale = ref<number | null>(null)
  const displayLabel = ref('适应窗口')

  const audioReady = ref(false)
  const audioMuted = ref(true)
  const audioTitle = ref('设备声音不可用')
  const pipSupported = ref(false)
  const pipActive = ref(false)
  const fullscreen = ref(false)
  const rebooting = ref(false)
  const { autoUnlockOnWake, powerBusy, pressPower, observeScreenState } = useScreenPower(serial, {
    sendPowerKey: () => {
      keyPress(AKEYCODE.POWER)
      op('key', { label: '电源键', keycode: 'KEYCODE_POWER' })
    },
    onWake: () => {
      screenOffDetected = false
      screenOff.value = false
      op('key', { label: '唤醒', keycode: 'KEYCODE_WAKEUP' })
    },
    checkScreenState,
  })

  const packer = new ControlPacker()
  const { clipboardSync, onDeviceClipboard } = useClipboardSync({
    send: b => send(b),
    packer,
    onOp: op,
    isInputBlocked: target => inspectorConsumesInput() || isInteractiveTarget(target),
  })
  let decoder: ScrcpyDecoder | null = null
  let audio: DeviceAudio | null = null
  let pip: PictureInPicture | null = null

  let ws: WebSocket | null = null
  let connectionId = 0
  let connected = false
  let reconnectTimer: number | undefined
  let reconnectAttempts = 0
  let connectTimer: number | undefined
  let watchdogTimer: number | undefined
  let screenTimer: number | undefined
  let lastActivityAt = 0
  let serverErrorAttempts = 0
  let lastServerError = ''
  let errorShown = false
  let screenOffDetected = false
  let screenChecking = false
  let screenAbort: AbortController | undefined
  let disposed = false
  let pseudoFs = false
  let resizeObserver: ResizeObserver | undefined
  let mouseDown = false

  // ---------- 状态 ----------
  function setStatus(text: string, isLive: boolean) {
    statusText.value = text
    live.value = isLive
  }

  function showError(msg: string) {
    clearConnectTimer()
    errorShown = true
    loading.value = false
    errorMsg.value = msg
  }

  // ---------- WS ----------
  function wsUrl(): string {
    const proto = location.protocol === 'https:' ? 'wss:' : 'ws:'
    const params = new URLSearchParams({ serial })
    if (playerSettings.bitrate.value) {
      params.set('bitrate', String(playerSettings.bitrate.value))
    }
    if (playerSettings.maxFps.value) {
      params.set('max_fps', String(playerSettings.maxFps.value))
    }
    return `${proto}//${location.host}/ws?${params.toString()}`
  }

  function closeWebSocket() {
    if (!ws)
      return
    const old = ws
    ws = null
    old.onopen = null
    old.onmessage = null
    old.onerror = null
    old.onclose = null
    if (old.readyState === WebSocket.OPEN || old.readyState === WebSocket.CONNECTING)
      old.close()
  }

  function clearConnectTimer() {
    if (connectTimer) {
      clearTimeout(connectTimer)
      connectTimer = undefined
    }
  }

  function startConnectTimer() {
    clearConnectTimer()
    connectTimer = window.setTimeout(() => {
      showError(lastServerError || '连接超时，请检查设备连接')
    }, CONNECT_TIMEOUT_MS)
  }

  function clearWatchdog() {
    if (watchdogTimer) {
      clearInterval(watchdogTimer)
      watchdogTimer = undefined
    }
  }

  function startWatchdog() {
    clearWatchdog()
    lastActivityAt = Date.now()
    watchdogTimer = window.setInterval(() => {
      if (!connected || !lastActivityAt)
        return
      if (Date.now() - lastActivityAt > NO_FRAME_TIMEOUT_MS)
        handleStreamLoss('视频流已中断，正在重连')
    }, 1000)
  }

  function reconnect() {
    if (reconnectTimer !== undefined)
      return
    reconnectAttempts++
    const delay = Math.min(1000 * reconnectAttempts, 5000)
    setStatus(`重连中...(${reconnectAttempts})`, false)
    console.log('[ws]', delay, 'ms 后重连')
    reconnectTimer = window.setTimeout(() => {
      reconnectTimer = undefined
      screenOffDetected = false
      screenOff.value = false
      if (!errorShown) {
        errorMsg.value = ''
        loading.value = true
      }
      connect()
    }, delay)
  }

  function handleStreamLoss(message: string) {
    if (!connected && reconnectTimer !== undefined)
      return
    if (connected) opts.onDisconnect?.()
    connected = false
    decodedFrameVersion = 0
    clearWatchdog()
    closeWebSocket()
    setStatus(message || '视频流已中断，正在重连', false)
    reconnect()
  }

  function connect() {
    if (ws && (ws.readyState === WebSocket.OPEN || ws.readyState === WebSocket.CONNECTING))
      closeWebSocket()
    if (!errorShown)
      errorMsg.value = ''
    startConnectTimer()

    const socket = new WebSocket(wsUrl())
    ws = socket
    const currentId = ++connectionId
    socket.binaryType = 'arraybuffer'

    socket.onopen = () => {
      if (ws !== socket || currentId !== connectionId)
        return
      reconnectAttempts = 0
      setStatus('已连接，等待画面...', true)
    }
    socket.onmessage = ev => handleMessage(socket, currentId, ev)
    socket.onerror = () => {
      if (ws !== socket || currentId !== connectionId)
        return
      console.error('[ws] error')
    }
    socket.onclose = () => {
      if (ws !== socket || currentId !== connectionId)
        return
      console.log('[ws] closed')
      ws = null
      clearConnectTimer()
      if (connected) opts.onDisconnect?.()
      connected = false
      decodedFrameVersion = 0
      clearWatchdog()
      if (reconnectAttempts < MAX_RECONNECT) {
        reconnect()
      }
      else {
        showError(lastServerError || '连接失败，请刷新')
        setStatus('连接失败，请刷新', false)
      }
    }
  }

  function handleMessage(socket: WebSocket, currentId: number, ev: MessageEvent) {
    if (ws !== socket || currentId !== connectionId)
      return
    try {
      if (typeof ev.data === 'string') {
        const msg = JSON.parse(ev.data) as WsMeta
        if (msg.type === 'audio.status' && msg.status) {
          audio?.serverStatus({ status: msg.status as AudioStatus, reason: msg.reason })
          return
        }
        if (msg.type === 'clipboard' && typeof msg.text === 'string') {
          onDeviceClipboard(msg.text)
          return
        }
        if (msg.type === 'meta') {
          onMeta(msg)
          return
        }
        if (msg.type === 'error') {
          // 服务端启动失败常见，静默重试；连续 3 次才亮红
          lastServerError = msg.message || '设备连接错误'
          serverErrorAttempts++
          console.warn('[ws] 服务端错误（第', serverErrorAttempts, '次），静默重试:', lastServerError)
          if (serverErrorAttempts >= 3)
            showError(lastServerError)
          return
        }
        if (msg.type === 'disconnected') {
          console.log('[ws] server 通知断开，准备重连')
          handleStreamLoss('视频流已中断，正在重连')
        }
        return
      }
      lastActivityAt = Date.now()
      const data = new Uint8Array(ev.data as ArrayBuffer)
      if (data[0] === 0x80)
        audio?.handlePacket(ev.data as ArrayBuffer)
      else decoder?.handleFrame(data)
    }
    catch (e) {
      console.error('[ws] message error:', e)
    }
  }

  function onMeta(msg: WsMeta) {
    clearConnectTimer()
    serverErrorAttempts = 0
    lastServerError = ''
    errorShown = false
    errorMsg.value = ''
    const w = msg.width ?? 0
    const h = msg.height ?? 0
    saveCachedVideoSize(serial, w, h)
    if (msg.audio_available === false)
      audio?.serverStatus({ status: 'unavailable', reason: msg.audio_reason || '设备音频不可用' })
    packer.setDeviceSize(w, h)
    videoSize.value = { w, h }
    try {
      decoder?.configure(msg.codec || 'h264', w, h)
    }
    catch (e) {
      console.error('[decoder] 初始化失败:', e)
      handleStreamLoss('视频解码初始化失败，正在重连')
      return
    }
    loading.value = false
    setStatus(`${w}x${h}`, true)
    showFps.value = true
    connected = true
    startWatchdog()
    requestAnimationFrame(() => requestAnimationFrame(fitCanvas))
  }

  function retry() {
    errorShown = false
    errorMsg.value = ''
    reconnectAttempts = 0
    serverErrorAttempts = 0
    lastServerError = ''
    loading.value = true
    connect()
  }

  /** 供重放切换视频源：关闭当前 /ws 并重连（新订阅拿到重放源或回到实时流） */
  function switchStream(message: string) {
    handleStreamLoss(message || '正在切换视频源…')
  }

  // ---------- 发送 ----------
  function send(bytes: Uint8Array) {
    if (ws && ws.readyState === WebSocket.OPEN)
      ws.send(bytes as Uint8Array<ArrayBuffer>)
  }

  function toDevice(clientX: number, clientY: number): { x: number, y: number } {
    const canvas = canvasRef.value
    const rect = canvas?.getBoundingClientRect()
    const dw = packer.deviceWidth
    const dh = packer.deviceHeight
    if (!rect || !dw || !dh)
      return { x: 0, y: 0 }
    const x = Math.round(((clientX - rect.left) * dw) / rect.width)
    const y = Math.round(((clientY - rect.top) * dh) / rect.height)
    return {
      x: Math.max(0, Math.min(dw - 1, x)),
      y: Math.max(0, Math.min(dh - 1, y)),
    }
  }

  // ---------- 输入事件 ----------
  let mouseDownPos: { x: number, y: number } | null = null
  let mouseDownTime = 0
  let lastMousePoint: { x: number, y: number } | null = null
  // 滑动轨迹取样参数：时间下限 ~60Hz，距离下限滤掉抖动/亚像素重复点
  const MIN_SAMPLE_INTERVAL = 16
  const MIN_SAMPLE_STEP = 12
  let dragPath: { x: number, y: number }[] = []
  let lastSampleTime = 0

  function beginDrag(x: number, y: number) {
    dragPath = [{ x, y }]
    lastSampleTime = Date.now()
  }

  function sampleDrag(x: number, y: number) {
    const last = dragPath[dragPath.length - 1]
    const dx = x - last.x
    const dy = y - last.y
    const now = Date.now()
    if (Math.sqrt(dx * dx + dy * dy) >= MIN_SAMPLE_STEP && now - lastSampleTime >= MIN_SAMPLE_INTERVAL) {
      dragPath.push({ x, y })
      lastSampleTime = now
    }
  }

  function endDrag(x: number, y: number) {
    const last = dragPath[dragPath.length - 1]
    if (last.x !== x || last.y !== y)
      dragPath.push({ x, y })
  }

  function logTapSwipe(start: { x: number, y: number } | null, since: number, x: number, y: number) {
    if (!start)
      return
    const dx = x - start.x
    const dy = y - start.y
    const duration = Date.now() - since
    if (Math.sqrt(dx * dx + dy * dy) < 15) {
      if (duration >= 500) {
        op('longpress', { x, y, duration })
        opts.onTouch?.({ kind: 'tap', x: x / packer.deviceWidth, y: y / packer.deviceHeight })
      } else {
        op('tap', { x, y })
        opts.onTouch?.({ kind: 'tap', x: x / packer.deviceWidth, y: y / packer.deviceHeight })
      }
    }
    else {
      endDrag(x, y)
      op('swipe', { x1: start.x, y1: start.y, x2: x, y2: y, duration, path: dragPath })
      // down/move/up 已反显实际轨迹，松开时不再补播首尾直线动画。
    }
  }

  function onMouseDown(e: MouseEvent) {
    e.preventDefault()
    if (inspectorConsumesInput())
      return
    if (e.button === 2) {
      send(packer.back(KEY_UP)) // 右键 = 返回
      op('key', { label: '返回', keycode: 'KEYCODE_BACK' })
      return
    }
    mouseDown = true
    const { x, y } = toDevice(e.clientX, e.clientY)
    mouseDownPos = { x, y }
    lastMousePoint = { x, y }
    mouseDownTime = Date.now()
    beginDrag(x, y)
    send(packer.touch(ACTION_DOWN, POINTER_MOUSE, x, y, 1.0, 0x1, 0x1))
    opts.onTouch?.({ kind: 'down', x: x / packer.deviceWidth, y: y / packer.deviceHeight })
  }

  function onMouseMove(e: MouseEvent) {
    if (!mouseDown)
      return
    e.preventDefault()
    const { x, y } = toDevice(e.clientX, e.clientY)
    sampleDrag(x, y)
    sendMouseMove(x, y)
    opts.onTouch?.({ kind: 'move', x: x / packer.deviceWidth, y: y / packer.deviceHeight })
  }

  // Fast drags can reach mouseup before the browser delivers mousemove. Send the
  // release position as a move first so Android receives an actual swipe.
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
    opts.onTouch?.({ kind: 'up', x: x / packer.deviceWidth, y: y / packer.deviceHeight })
    logTapSwipe(mouseDownPos, mouseDownTime, x, y)
    mouseDownPos = null
    lastMousePoint = null
    dragPath = []
  }
  function onWheel(e: WheelEvent) {
    e.preventDefault()
    if (inspectorConsumesInput()) return
    const { x, y } = toDevice(e.clientX, e.clientY)
    send(packer.scroll(x, y, e.deltaX, e.deltaY))
  }
  function onContextMenu(e: Event) {
    e.preventDefault()
  }

  let touchActive = false
  let touchStartPos: { x: number, y: number } | null = null
  let touchStartTime = 0
  let lastTouchPoint: { x: number, y: number } | null = null

  function onTouchStart(e: TouchEvent) {
    e.preventDefault()
    const t = e.touches[0]
    if (inspectorConsumesInput())
      return
    touchActive = true
    const { x, y } = toDevice(t.clientX, t.clientY)
    touchStartPos = { x, y }
    lastTouchPoint = { x, y }
    touchStartTime = Date.now()
    beginDrag(x, y)
    send(packer.touch(ACTION_DOWN, POINTER_FINGER_BASE, x, y, t.force || 1.0))
    opts.onTouch?.({ kind: 'down', x: x / packer.deviceWidth, y: y / packer.deviceHeight })
  }

  function onTouchMove(e: TouchEvent) {
    if (!touchActive)
      return
    e.preventDefault()
    const t = e.touches[0]
    const { x, y } = toDevice(t.clientX, t.clientY)
    sampleDrag(x, y)
    sendTouchMove(x, y, t.force || 1.0)
    opts.onTouch?.({ kind: 'move', x: x / packer.deviceWidth, y: y / packer.deviceHeight })
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
    opts.onTouch?.({ kind: 'up', x: x / packer.deviceWidth, y: y / packer.deviceHeight })
    logTapSwipe(touchStartPos, touchStartTime, x, y)
    touchStartPos = null
    lastTouchPoint = null
    dragPath = []
  }

  function isInteractiveField(el: Element | null): boolean {
    if (!el)
      return false
    const tag = el.tagName
    if (tag === 'INPUT' || tag === 'TEXTAREA' || tag === 'SELECT')
      return true
    if ((el as HTMLElement).isContentEditable)
      return true
    // Reka Select/菜单把内容渲染到 body 的 portal：选项焦点是 role=option/listbox，需一并放行
    const interactivePanel = el.closest?.([
      '[data-capture-layer]',
      '[role="dialog"]',
      '[role="listbox"]',
      '[role="option"]',
      '[role="menuitem"]',
      '[data-radix-select-viewport]',
      '[data-radix-select-content]',
      '[data-radix-popper-content-wrapper]',
      'dialog',
      '.xterm',
      '.ui-inspector-panel',
      '.capture-actions',
    ].join(', '))
    if (interactivePanel) {
      // 手机弹窗的外壳允许画布接收按键，内部工具弹窗仍按原规则拦截。
      const playerDialog = stageRef.value?.closest('[data-player-dialog]')
      return interactivePanel !== playerDialog || el !== canvasRef.value
    }
    return false
  }
  function isInteractiveTarget(target: EventTarget | null): boolean {
    return isInteractiveField(target instanceof Element ? target : null) || isInteractiveField(document.activeElement)
  }
  function shouldIgnoreKey(e: KeyboardEvent): boolean {
    if (inspectorConsumesInput()) return true
    if (e.key === 'Escape' && stageRef.value?.closest('[data-player-dialog]'))
      return true
    if (e.key === 'Shift' || e.key === 'Control' || e.key === 'Alt' || e.key === 'Meta')
      return true
    // Command / Alt 组合一律放行给浏览器
    if (e.metaKey || (e.altKey && !e.ctrlKey))
      return true
    // 事件目标或当前焦点处于输入控件或可交互浮层/面板内时，不注入设备（与旧版 MyWebScrcpy 对齐）
    return isInteractiveTarget(e.target)
  }

  const KEY_LOG_MAP: Record<number, { label: string, keycode: string }> = {
    [AKEYCODE.DEL]: { label: '退格', keycode: 'KEYCODE_DEL' },
    [AKEYCODE.ENTER]: { label: '回车', keycode: 'KEYCODE_ENTER' },
    [AKEYCODE.TAB]: { label: 'Tab', keycode: 'KEYCODE_TAB' },
    [AKEYCODE.ESCAPE]: { label: 'ESC', keycode: 'KEYCODE_ESCAPE' },
    [AKEYCODE.SPACE]: { label: '空格', keycode: 'KEYCODE_SPACE' },
    [AKEYCODE.DPAD_UP]: { label: '↑', keycode: 'KEYCODE_DPAD_UP' },
    [AKEYCODE.DPAD_DOWN]: { label: '↓', keycode: 'KEYCODE_DPAD_DOWN' },
    [AKEYCODE.DPAD_LEFT]: { label: '←', keycode: 'KEYCODE_DPAD_LEFT' },
    [AKEYCODE.DPAD_RIGHT]: { label: '→', keycode: 'KEYCODE_DPAD_RIGHT' },
  }

  function onKeyDown(e: KeyboardEvent) {
    if (shouldIgnoreKey(e))
      return
    const result = keyboardEventToKeyCode(e)
    if (result) {
      e.preventDefault()
      send(packer.keyCode(KEY_DOWN, result.keycode, result.meta))
      const info = KEY_LOG_MAP[result.keycode]
      if (info)
        op('key', info)
      return
    }
    // 可打印字符 → 文本注入（带修饰键的组合键不拦截）
    if (e.key.length === 1 && !e.ctrlKey && !e.altKey && !e.metaKey) {
      e.preventDefault()
      send(packer.text(e.key))
      op('text', { text: e.key })
    }
  }

  function onKeyUp(e: KeyboardEvent) {
    if (shouldIgnoreKey(e))
      return
    const result = keyboardEventToKeyCode(e)
    if (result) {
      e.preventDefault()
      send(packer.keyCode(KEY_UP, result.keycode, result.meta))
    }
  }

  // ---------- 工具栏动作 ----------
  function keyPress(keycode: number) {
    send(packer.keyCode(KEY_DOWN, keycode, 0))
    send(packer.keyCode(KEY_UP, keycode, 0))
  }

  function pressBack() {
    send(new Uint8Array([4, KEY_DOWN]))
    send(new Uint8Array([4, KEY_UP]))
    op('key', { label: '返回', keycode: 'KEYCODE_BACK' })
  }

  function pressHome() {
    keyPress(AKEYCODE.HOME)
    op('key', { label: 'HOME', keycode: 'KEYCODE_HOME' })
  }

  function pressRecents() {
    keyPress(AKEYCODE.APP_SWITCH)
    op('key', { label: '最近应用', keycode: 'KEYCODE_APP_SWITCH' })
  }

  function rotate() {
    const before = `${videoSize.value.w}x${videoSize.value.h}`
    send(packer.rotate())
    setStatus('正在请求旋转…', false)
    setTimeout(() => {
      const after = `${videoSize.value.w}x${videoSize.value.h}`
      setStatus(before === after ? '应用可能锁定方向' : '已切换方向', true)
    }, 1200)
  }

  async function reboot(): Promise<void> {
    rebooting.value = true
    try {
      const res = await fetch(`/api/reboot?serial=${encodeURIComponent(serial)}`, { method: 'POST' })
      const data = await res.json()
      if (!res.ok || !data.ok)
        throw new Error(data.error || '重启请求失败')
      setStatus('设备重启中...', false)
    }
    finally {
      rebooting.value = false
    }
  }

  async function toggleAudio() {
    await audio?.toggle()
  }

  async function togglePip() {
    try {
      await pip?.toggle()
    }
    catch (e) {
      setStatus(e instanceof Error ? e.message : '画中画不可用', false)
      setTimeout(() => connected && setStatus(`${videoSize.value.w}x${videoSize.value.h}`, true), 2000)
    }
  }

  // ---------- 全屏（原生优先，iOS 回退伪全屏） ----------
  function isFullscreen(): boolean {
    return !!(document.fullscreenElement || (document as Document & { webkitFullscreenElement?: Element }).webkitFullscreenElement) || pseudoFs
  }

  function enterFullscreen() {
    const el = stageRef.value
    if (!el)
      return
    const req = el.requestFullscreen || (el as HTMLElement & { webkitRequestFullscreen?: () => Promise<void> }).webkitRequestFullscreen
    if (req) {
      const p = req.call(el) as Promise<void> | undefined
      p?.catch?.(() => enablePseudo())
    }
    else {
      enablePseudo()
    }
  }

  function exitFullscreen() {
    if (pseudoFs) {
      disablePseudo()
    }
    else {
      const doc = document as Document & { webkitExitFullscreen?: () => void }
      const ex = document.exitFullscreen || doc.webkitExitFullscreen
      ex?.call(document)
    }
  }

  function enablePseudo() {
    pseudoFs = true
    stageRef.value?.classList.add('pseudo-fullscreen')
    onFsChange()
  }

  function disablePseudo() {
    pseudoFs = false
    stageRef.value?.classList.remove('pseudo-fullscreen')
    onFsChange()
  }

  function onFsChange() {
    fullscreen.value = isFullscreen()
    setTimeout(fitCanvas, 100)
  }

  function toggleFullscreen() {
    if (isFullscreen())
      exitFullscreen()
    else enterFullscreen()
  }

  // ---------- 显示尺寸 ----------
  function setDisplayScale(scale: number) {
    displayScale.value = Math.abs(scale - 100) <= 4 ? 100 : scale
    const s = displayScale.value
    if (videoSize.value.w)
      displayLabel.value = s === 100 ? `原始尺寸 ${videoSize.value.w} × ${videoSize.value.h}` : `${Math.round(videoSize.value.w * s / 100)} × ${Math.round(videoSize.value.h * s / 100)}`
    else displayLabel.value = `${s}%`
    fitCanvas()
  }

  function fitWindow() {
    displayScale.value = null
    displayLabel.value = '适应窗口'
    fitCanvas()
  }

  function fitCanvas() {
    const canvas = canvasRef.value
    const stage = stageRef.value
    if (!canvas || !stage || !canvas.width || !canvas.height)
      return
    // 右侧面板打开时，画布应按 SplitLayout 左栏的实际空间适配，
    // 不能继续使用包含右栏的整页宽度，否则会产生空白或被右栏挤压。
    const layoutMain = canvas.closest<HTMLElement>('.split-layout-main')
    const viewport = layoutMain || stage
    const padding = layoutMain ? 0 : 48
    const availW = viewport.clientWidth - padding
    const availH = viewport.clientHeight - padding
    if (availW <= 0 || availH <= 0)
      return
    const cw = canvas.width
    const ch = canvas.height
    let dw: number
    let dh: number
    if (displayScale.value !== null) {
      dw = (cw * displayScale.value) / 100
      dh = (ch * displayScale.value) / 100
    }
    else {
      const ratio = cw / ch
      dw = availW
      dh = dw / ratio
      if (dh > availH) {
        dh = availH
        dw = dh * ratio
      }
    }
    canvas.style.width = `${Math.round(dw)}px`
    canvas.style.height = `${Math.round(dh)}px`
  }

  // ---------- 灭屏检测 ----------
  async function checkScreenState() {
    if (!serial || screenChecking || disposed)
      return
    screenChecking = true
    const controller = new AbortController()
    screenAbort = controller
    const timeout = window.setTimeout(() => controller.abort(), 10000)
    try {
      const res = await fetch(`/api/screen-state?serial=${encodeURIComponent(serial)}`, { signal: controller.signal })
      if (!res.ok)
        throw new Error(`读取屏幕状态失败（${res.status}）`)
      const data = await res.json()
      if (disposed)
        return
      if (data.error) {
        console.warn('[screen-state] 检测失败:', data.error)
        return
      }
      if (data.screenOn === false && !screenOffDetected) {
        screenOffDetected = true
        screenOff.value = true
      }
      else if (data.screenOn === true && screenOffDetected) {
        screenOffDetected = false
        screenOff.value = false
      }
      if (typeof data.screenOn === 'boolean')
        await observeScreenState(data.screenOn)
    }
    catch (e) {
      if (!disposed)
        console.warn('[screen-state] 请求失败:', e)
    }
    finally {
      window.clearTimeout(timeout)
      screenAbort = undefined
      screenChecking = false
    }
  }

  // ---------- 生命周期 ----------
  onMounted(() => {
    const canvas = canvasRef.value
    if (!canvas)
      return
    const initialSize = loadCachedVideoSize(serial)
    canvas.width = initialSize.w
    canvas.height = initialSize.h
    fitCanvas()

    decoder = new ScrcpyDecoder(canvas)
    decoder.onDecodedFrame = () => {
      decodedFrameVersion++
      opts.onFrame?.()
      lastActivityAt = Date.now()
    }
    decoder.onFps = v => (fps.value = v)
    decoder.onDeviceResize = (w, h) => {
      saveCachedVideoSize(serial, w, h)
      packer.setDeviceSize(w, h)
      videoSize.value = { w, h }
      fitCanvas()
    }
    decoder.onError = (info) => {
      handleStreamLoss(info.message || '视频解码失败，正在重连')
    }

    audio = new DeviceAudio()
    audio.onStatus = (status, reason) => {
      audioReady.value = status === 'ready'
      audioTitle.value
        = status === 'ready'
          ? (audio?.muted ? '点击播放设备声音' : '点击静音设备声音')
          : (reason || '设备声音不可用')
      audioMuted.value = audio?.muted ?? true
      if (status === 'ready' && playerSettings.autoAudio.value && audioMuted.value) {
        audio?.toggle().catch(() => {})
      }
    }

    pip = new PictureInPicture(canvas)
    pipSupported.value = pip.supported
    pip.onChange = v => (pipActive.value = v)

    canvas.addEventListener('mousedown', onMouseDown)
    window.addEventListener('mousemove', onMouseMove)
    window.addEventListener('mouseup', onMouseUp)
    canvas.addEventListener('wheel', onWheel, { passive: false })
    canvas.addEventListener('contextmenu', onContextMenu)
    canvas.addEventListener('touchstart', onTouchStart, { passive: false })
    canvas.addEventListener('touchmove', onTouchMove, { passive: false })
    canvas.addEventListener('touchend', onTouchEnd)
    window.addEventListener('keydown', onKeyDown)
    window.addEventListener('keyup', onKeyUp)
    document.addEventListener('fullscreenchange', onFsChange)
    document.addEventListener('webkitfullscreenchange', onFsChange)

    resizeObserver = new ResizeObserver(() => fitCanvas())
    if (stageRef.value)
      resizeObserver.observe(stageRef.value)
    window.addEventListener('resize', fitCanvas)

    screenTimer = window.setInterval(checkScreenState, SCREEN_STATE_POLL_MS)
    void checkScreenState()
    connect()
  })

  onUnmounted(() => {
    disposed = true
    screenAbort?.abort()
    if (reconnectTimer !== undefined)
      window.clearTimeout(reconnectTimer)
    if (screenTimer !== undefined)
      window.clearInterval(screenTimer)
    clearConnectTimer()
    clearWatchdog()
    closeWebSocket()
    decoder?.close()
    audio?.close()
    void pip?.close().catch(() => {})
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
    window.removeEventListener('keydown', onKeyDown)
    window.removeEventListener('keyup', onKeyUp)
    document.removeEventListener('fullscreenchange', onFsChange)
    document.removeEventListener('webkitfullscreenchange', onFsChange)
    window.removeEventListener('resize', fitCanvas)
  })

  return {
    frameVersion: () => decodedFrameVersion,
    canvasRef,
    stageRef,
    loading,
    errorMsg,
    statusText,
    live,
    fps,
    showFps,
    videoSize,
    screenOff,
    displayScale,
    displayLabel,
    audioReady,
    audioMuted,
    audioTitle,
    pipSupported,
    pipActive,
    fullscreen,
    rebooting,
    autoUnlockOnWake,
    clipboardSync,
    powerBusy,
    retry,
    switchStream,
    pressBack,
    pressHome,
    pressRecents,
    pressPower,
    rotate,
    reboot,
    toggleAudio,
    togglePip,
    toggleFullscreen,
    setDisplayScale,
    fitWindow,
  }
}
