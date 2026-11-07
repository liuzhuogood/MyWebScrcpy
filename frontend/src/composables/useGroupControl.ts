import { ref } from 'vue'

export type GroupControlPacket = Uint8Array | string

export interface GroupControlChannel {
  serial: string
  send: (bytes: GroupControlPacket) => void
  isReady: () => boolean
  getDeviceSize?: () => { width: number, height: number }
}

export interface GroupControlSize {
  width: number
  height: number
}

// 触摸消息类型定义与 scrcpy-server 对齐
const TYPE_INJECT_TOUCH_EVENT = 2
const TYPE_INJECT_SCROLL_EVENT = 3

/**
 * 转换 scrcpy 32 字节触摸事件坐标
 * 针对不同目标设备分辨率自动做等比缩放映射
 */
export function transformTouchPacket(
  bytes: Uint8Array,
  sourceSize: GroupControlSize,
  targetSize: GroupControlSize,
): Uint8Array {
  if (bytes.length !== 32 || bytes[0] !== TYPE_INJECT_TOUCH_EVENT) {
    return bytes
  }
  if (!sourceSize.width || !sourceSize.height || !targetSize.width || !targetSize.height) {
    return bytes
  }
  // 尺寸完全相同时直接复用原始包
  if (sourceSize.width === targetSize.width && sourceSize.height === targetSize.height) {
    return bytes
  }

  const copy = new Uint8Array(bytes.length)
  copy.set(bytes)
  const view = new DataView(copy.buffer, copy.byteOffset, copy.byteLength)

  const srcX = view.getInt32(10, false)
  const srcY = view.getInt32(14, false)
  const ratioX = Math.max(0, Math.min(1, srcX / sourceSize.width))
  const ratioY = Math.max(0, Math.min(1, srcY / sourceSize.height))

  const targetX = Math.min(targetSize.width - 1, Math.round(ratioX * targetSize.width))
  const targetY = Math.min(targetSize.height - 1, Math.round(ratioY * targetSize.height))

  view.setInt32(10, targetX, false)
  view.setInt32(14, targetY, false)
  view.setUint16(18, targetSize.width, false)
  view.setUint16(20, targetSize.height, false)

  return copy
}

/**
 * 转换 scrcpy 21 字节滚动事件坐标
 */
export function transformScrollPacket(
  bytes: Uint8Array,
  sourceSize: GroupControlSize,
  targetSize: GroupControlSize,
): Uint8Array {
  if (bytes.length !== 21 || bytes[0] !== TYPE_INJECT_SCROLL_EVENT) {
    return bytes
  }
  if (!sourceSize.width || !sourceSize.height || !targetSize.width || !targetSize.height) {
    return bytes
  }
  if (sourceSize.width === targetSize.width && sourceSize.height === targetSize.height) {
    return bytes
  }

  const copy = new Uint8Array(bytes.length)
  copy.set(bytes)
  const view = new DataView(copy.buffer, copy.byteOffset, copy.byteLength)

  const srcX = view.getInt32(1, false)
  const srcY = view.getInt32(5, false)
  const ratioX = Math.max(0, Math.min(1, srcX / sourceSize.width))
  const ratioY = Math.max(0, Math.min(1, srcY / sourceSize.height))

  const targetX = Math.min(targetSize.width - 1, Math.round(ratioX * targetSize.width))
  const targetY = Math.min(targetSize.height - 1, Math.round(ratioY * targetSize.height))

  view.setInt32(1, targetX, false)
  view.setInt32(5, targetY, false)
  view.setUint16(9, targetSize.width, false)
  view.setUint16(11, targetSize.height, false)

  return copy
}

// 首页协调群控；独立小窗通过 BroadcastChannel 传入原始输入，只有协调页派发。
// 同一设备只选一个通道，WebSocket.send 不等待目标执行，各设备服务端队列并行。
const isActive = ref(false)
const targetSerials = ref<string[]>([])
const lastError = ref('')
const registeredChannels = new Map<string, Set<GroupControlChannel>>()
const instanceId = `${Date.now()}-${Math.random().toString(36).slice(2)}`
const MAX_TARGETS = 1024
const MAX_CONNECTING = 4
const MAX_BUFFER_BYTES = 128 * 1024
const INPUT_TTL_MS = 500
let isCoordinator = false
let owner: string | null = null
let bus: BroadcastChannel | undefined
let heartbeat: ReturnType<typeof setInterval> | undefined
let lease: ReturnType<typeof setTimeout> | undefined
const warnings = new Map<string, number>()
const heldInputs = new Map<string, Map<string, { packet: Uint8Array, source: string }>>()
interface BackgroundConnection {
  ws: WebSocket
  size: GroupControlSize | null
  deadline: ReturnType<typeof setTimeout>
}
const backgroundSockets = new Map<string, BackgroundConnection>()
const retries = new Map<string, number>()
const retryTimers = new Map<string, ReturnType<typeof setTimeout>>()

function report(serial: string, message: string) {
  if (Date.now() - (warnings.get(serial) ?? 0) < 5000) return
  warnings.set(serial, Date.now())
  console.warn('[GroupControl]', { serial, message })
  lastError.value = `${serial}：${message}`
}

function closeBackground(serial: string) {
  const record = backgroundSockets.get(serial)
  if (!record) return
  backgroundSockets.delete(serial)
  clearTimeout(record.deadline)
  record.ws.close()
}

function rememberInput(serial: string, packet: GroupControlPacket, source: string) {
  if (typeof packet === 'string') return
  const view = new DataView(packet.buffer, packet.byteOffset, packet.byteLength)
  const key = packet[0] === 2 && packet.length === 32 ? `touch:${view.getBigUint64(2)}`
    : packet[0] === 0 && packet.length === 14 ? `key:${view.getUint32(2)}` : null
  if (!key) return
  let held = heldInputs.get(serial)
  if (!held) { held = new Map(); heldInputs.set(serial, held) }
  if (packet[1] === 1 || packet[1] === 3) held.delete(key)
  else if (packet[1] === 0 || held.has(key)) held.set(key, { packet: packet.slice(), source })
  if (held.size === 0) heldInputs.delete(serial)
}

function releaseTarget(serial: string, sources?: Set<string>) {
  const held = heldInputs.get(serial)
  if (!held) return
  const channel = readyChannel(serial)
  const background = backgroundSockets.get(serial)
  for (const [key, input] of held) {
    if (sources && !sources.has(input.source)) continue
    held.delete(key)
    const { packet } = input
    packet[1] = 1 // UP: 避免关闭群控或取消目标后残留按住状态。
    if (packet[0] === 2) packet.fill(0, 22)
    try {
      if (channel) channel.send(packet)
      else if (background?.ws.readyState === WebSocket.OPEN) background.ws.send(packet as Uint8Array<ArrayBuffer>)
    }
    catch (error) { report(serial, `释放输入失败：${String(error)}`) }
  }
  if (held.size === 0) heldInputs.delete(serial)
}

function releaseSources(serials: string[]) {
  if (isCoordinator) {
    for (const serial of heldInputs.keys()) releaseTarget(serial, new Set(serials))
  }
  else bus?.postMessage({ type: 'source-closed', owner, serials })
}

function cleanBackgroundSockets() {
  for (const serial of heldInputs.keys()) releaseTarget(serial)
  for (const serial of backgroundSockets.keys()) closeBackground(serial)
  for (const timer of retryTimers.values()) clearTimeout(timer)
  retryTimers.clear()
  retries.clear()
}

function readyChannel(serial: string) {
  return [...(registeredChannels.get(serial) ?? [])].find(channel => channel.isReady())
}

function syncBackgroundConnections() {
  if (!isCoordinator || !isActive.value) {
    cleanBackgroundSockets()
    return
  }
  const targets = new Set(targetSerials.value)
  for (const serial of backgroundSockets.keys()) {
    if (!targets.has(serial) || readyChannel(serial)) closeBackground(serial)
  }
  for (const [serial, timer] of retryTimers) {
    if (!targets.has(serial)) { clearTimeout(timer); retryTimers.delete(serial); retries.delete(serial) }
  }
  if (typeof window === 'undefined') return
  let connecting = [...backgroundSockets.values()].filter(record => record.size === null).length
  for (const serial of targets) {
    if (connecting >= MAX_CONNECTING) break
    if (readyChannel(serial) || backgroundSockets.has(serial) || retryTimers.has(serial) || (retries.get(serial) ?? 0) >= 3) continue
    connecting++
    const proto = location.protocol === 'https:' ? 'wss:' : 'ws:'
    const ws = new WebSocket(`${proto}//${location.host}/ws?serial=${encodeURIComponent(serial)}&control_only=1`)
    const record: BackgroundConnection = {
      ws, size: null,
      deadline: setTimeout(() => { report(serial, '群控连接超时'); ws.close() }, 10000),
    }
    backgroundSockets.set(serial, record)
    ws.onmessage = (event) => {
      if (backgroundSockets.get(serial) !== record || typeof event.data !== 'string') return
      try {
        const meta = JSON.parse(event.data)
        if (meta.type === 'error') { report(serial, meta.message); return }
        if ((meta.type === 'meta' || meta.type === 'control.meta') && validSize(meta)) {
          record.size = { width: meta.width, height: meta.height }
          retries.delete(serial) // 已成功恢复，下一次独立断线有完整的重连预算。
          clearTimeout(record.deadline)
          record.deadline = setTimeout(() => { report(serial, '群控连接心跳超时'); ws.close() }, 10000)
          syncBackgroundConnections()
        }
      }
      catch (error) { report(serial, `群控元数据无效：${String(error)}`) }
    }
    ws.onclose = () => {
      if (backgroundSockets.get(serial) !== record) return
      backgroundSockets.delete(serial)
      clearTimeout(record.deadline)
      if (!isCoordinator || !isActive.value || !targetSerials.value.includes(serial)) return
      retries.set(serial, (retries.get(serial) ?? 0) + 1)
      report(serial, '群控连接断开，未就绪期间的操作不会重放')
      retryTimers.set(serial, setTimeout(() => {
        retryTimers.delete(serial)
        syncBackgroundConnections()
      }, 1000))
      syncBackgroundConnections()
    }
    ws.onerror = () => report(serial, '群控连接失败')
  }
}

function validSize(size: GroupControlSize | undefined | null): size is GroupControlSize {
  return !!size && Number.isInteger(size.width) && Number.isInteger(size.height)
    && size.width > 0 && size.height > 0 && size.width <= 65535 && size.height <= 65535
}

function transformPacket(bytes: GroupControlPacket, sourceSize: GroupControlSize | undefined, targetSize: GroupControlSize | undefined | null) {
  if (typeof bytes === 'string') return bytes // input.hid / input.pointer 已使用归一化坐标。
  if (bytes[0] !== TYPE_INJECT_TOUCH_EVENT && bytes[0] !== TYPE_INJECT_SCROLL_EVENT) return bytes
  if (!validSize(sourceSize) || !validSize(targetSize)) throw new Error('尚未取得真实屏幕尺寸')
  return bytes[0] === TYPE_INJECT_TOUCH_EVENT
    ? transformTouchPacket(bytes, sourceSize, targetSize)
    : transformScrollPacket(bytes, sourceSize, targetSize)
}

function dispatch(sourceSerial: string, bytes: GroupControlPacket, sourceSize?: GroupControlSize) {
  if (!isCoordinator || !isActive.value) return
  syncBackgroundConnections()
  for (const serial of targetSerials.value) {
    if (serial === sourceSerial) continue
    try {
      const channel = readyChannel(serial)
      if (channel) {
        const packet = transformPacket(bytes, sourceSize, channel.getDeviceSize?.())
        channel.send(packet)
        rememberInput(serial, packet, sourceSerial)
        continue
      }
      const background = backgroundSockets.get(serial)
      if (!background || background.ws.readyState !== WebSocket.OPEN || !background.size) throw new Error('群控连接尚未就绪')
      if (background.ws.bufferedAmount > MAX_BUFFER_BYTES) {
        releaseTarget(serial)
        closeBackground(serial)
        throw new Error('群控通道拥塞，已关闭连接以限制积压')
      }
      const packet = transformPacket(bytes, sourceSize, background.size)
      background.ws.send(packet as string | Uint8Array<ArrayBuffer>)
      rememberInput(serial, packet, sourceSerial)
    }
    catch (error) { report(serial, error instanceof Error ? error.message : String(error)) }
  }
}

function publishState() {
  bus?.postMessage({ type: 'state', owner: instanceId, active: isActive.value, serials: [...targetSerials.value] })
}

function ensureBus() {
  if (bus || typeof window === 'undefined' || typeof BroadcastChannel === 'undefined') return
  bus = new BroadcastChannel('mywebscrcpy.group-control.v1')
  bus.onmessage = ({ data }) => {
    if (!data || typeof data !== 'object') return
    if (data.type === 'request-state') {
      if (isCoordinator) publishState()
    }
    else if (data.type === 'source-closed' && isCoordinator && data.owner === instanceId
      && Array.isArray(data.serials) && data.serials.length <= MAX_TARGETS && data.serials.every((s: unknown) => typeof s === 'string')) {
      releaseSources(data.serials)
    }
    else if (data.type === 'state' && typeof data.owner === 'string' && typeof data.active === 'boolean'
      && Array.isArray(data.serials) && data.serials.length <= MAX_TARGETS && data.serials.every((s: unknown) => typeof s === 'string')) {
      isCoordinator = false
      clearInterval(heartbeat)
      cleanBackgroundSockets()
      owner = data.owner
      targetSerials.value = data.serials
      isActive.value = data.active
      clearTimeout(lease)
      lease = setTimeout(() => { isActive.value = false; owner = null }, 6000)
    }
    else if (data.type === 'input' && isCoordinator && data.owner === instanceId
      && typeof data.sourceSerial === 'string' && typeof data.sentAt === 'number'
      && Date.now() - data.sentAt >= 0 && Date.now() - data.sentAt <= INPUT_TTL_MS
      && (data.bytes instanceof Uint8Array || typeof data.bytes === 'string') && data.bytes.length <= MAX_BUFFER_BYTES) {
      dispatch(data.sourceSerial, data.bytes, data.sourceSize)
    }
  }
  bus.postMessage({ type: 'request-state' })
  window.addEventListener('pagehide', () => {
    if (isCoordinator) { isActive.value = false; publishState() }
    else releaseSources([...registeredChannels.keys()])
    clearInterval(heartbeat)
    clearTimeout(lease)
    cleanBackgroundSockets()
    bus?.close()
    bus = undefined
  }, { once: true })
}

export function useGroupControl() {
  ensureBus()
  function setActive(active: boolean) {
    if (active) {
      if (targetSerials.value.length > 0) lastError.value = ''
      isCoordinator = true
      owner = instanceId
      clearTimeout(lease)
      clearInterval(heartbeat)
      heartbeat = setInterval(publishState, 2000)
    }
    isActive.value = active && targetSerials.value.length > 0
    syncBackgroundConnections()
    if (isCoordinator) publishState()
    if (!isActive.value) clearInterval(heartbeat)
  }

  function setTargets(serials: string[]) {
    const next = [...new Set(serials)]
    for (const serial of heldInputs.keys()) if (!next.includes(serial)) releaseTarget(serial)
    if (next.length > MAX_TARGETS) {
      report('群控', `设备数量超过上限 ${MAX_TARGETS}，未启用`)
      targetSerials.value = []
    }
    else targetSerials.value = next
    if (targetSerials.value.length === 0) isActive.value = false
    syncBackgroundConnections()
    if (isCoordinator) publishState()
    if (!isActive.value) clearInterval(heartbeat)
    return next.length <= MAX_TARGETS
  }

  function registerChannel(channel: GroupControlChannel): () => void {
    let channels = registeredChannels.get(channel.serial)
    if (!channels) { channels = new Set(); registeredChannels.set(channel.serial, channels) }
    channels.add(channel)
    syncBackgroundConnections()
    return () => unregisterChannel(channel)
  }

  function unregisterChannel(channel: GroupControlChannel) {
    const channels = registeredChannels.get(channel.serial)
    channels?.delete(channel)
    if (channels?.size === 0) {
      registeredChannels.delete(channel.serial)
      releaseSources([channel.serial])
    }
    syncBackgroundConnections()
  }

  function broadcastControl(sourceSerial: string, bytes: GroupControlPacket, sourceSize?: GroupControlSize) {
    if (!isActive.value) return
    if (isCoordinator) dispatch(sourceSerial, bytes, sourceSize)
    else bus?.postMessage({ type: 'input', owner, sourceSerial, bytes, sourceSize, sentAt: Date.now() })
  }

  return { isActive, targetSerials, lastError, setActive, setTargets, registerChannel, unregisterChannel, broadcastControl }
}
