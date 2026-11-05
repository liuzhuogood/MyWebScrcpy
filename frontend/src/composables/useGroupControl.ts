import { ref } from 'vue'

export interface GroupControlChannel {
  serial: string
  send: (bytes: Uint8Array) => void
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

  const targetX = Math.round(ratioX * targetSize.width)
  const targetY = Math.round(ratioY * targetSize.height)

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

  const targetX = Math.round(ratioX * targetSize.width)
  const targetY = Math.round(ratioY * targetSize.height)

  view.setInt32(1, targetX, false)
  view.setInt32(5, targetY, false)
  view.setUint16(9, targetSize.width, false)
  view.setUint16(11, targetSize.height, false)

  return copy
}

// 单例状态保证多组件通信一致
const isActive = ref(false)
const targetSerials = ref<string[]>([])
const registeredChannels = new Map<string, Set<GroupControlChannel>>()
const backgroundSockets = new Map<string, { ws: WebSocket, size: GroupControlSize }>()
let isBroadcasting = false

function normalizeTargets(serials: string[]): string[] {
  return Array.from(new Set(serials.filter(Boolean)))
}

function ensureBackgroundSocket(serial: string) {
  if (backgroundSockets.has(serial))
    return
  if (typeof window === 'undefined' || !window.WebSocket)
    return

  try {
    const proto = location.protocol === 'https:' ? 'wss:' : 'ws:'
    const ws = new WebSocket(`${proto}//${location.host}/ws?serial=${encodeURIComponent(serial)}`)
    ws.binaryType = 'arraybuffer'
    const record = { ws, size: { width: 1080, height: 2400 } }
    backgroundSockets.set(serial, record)

    ws.onmessage = (event) => {
      try {
        if (typeof event.data === 'string') {
          const data = JSON.parse(event.data)
          if (data && typeof data.width === 'number' && typeof data.height === 'number') {
            record.size = { width: data.width, height: data.height }
          }
        }
      }
      catch {
        // 忽略非 JSON 元信息
      }
    }

    ws.onclose = () => {
      if (backgroundSockets.get(serial)?.ws === ws) {
        backgroundSockets.delete(serial)
      }
    }
  }
  catch (e) {
    console.warn(`[GroupControl] 为设备 ${serial} 建立后台控制连接失败:`, e)
  }
}

function cleanBackgroundSockets() {
  for (const [serial, item] of backgroundSockets.entries()) {
    try {
      item.ws.close()
    }
    catch {}
    backgroundSockets.delete(serial)
  }
}

function syncBackgroundConnections() {
  if (!isActive.value || targetSerials.value.length === 0) {
    cleanBackgroundSockets()
    return
  }

  const targetSet = new Set(targetSerials.value)
  // 清理不再选中的设备连接
  for (const [serial, item] of backgroundSockets.entries()) {
    if (!targetSet.has(serial)) {
      try {
        item.ws.close()
      }
      catch {}
      backgroundSockets.delete(serial)
    }
  }

  // 针对无活跃 UI 通道的已选目标设备建立后台 WS 通道
  for (const serial of targetSerials.value) {
    const uiChannels = registeredChannels.get(serial)
    if (!uiChannels || uiChannels.size === 0) {
      ensureBackgroundSocket(serial)
    }
    else {
      // 如果已有活跃 UI 渲染，则关闭多余的后台连接
      const bg = backgroundSockets.get(serial)
      if (bg) {
        try {
          bg.ws.close()
        }
        catch {}
        backgroundSockets.delete(serial)
      }
    }
  }
}

export function useGroupControl() {
  function setActive(active: boolean) {
    isActive.value = active
    syncBackgroundConnections()
  }

  function setTargets(serials: string[]) {
    targetSerials.value = normalizeTargets(serials)
    if (targetSerials.value.length === 0 && isActive.value) {
      isActive.value = false
    }
    syncBackgroundConnections()
  }

  function registerChannel(channel: GroupControlChannel): () => void {
    let set = registeredChannels.get(channel.serial)
    if (!set) {
      set = new Set()
      registeredChannels.set(channel.serial, set)
    }
    set.add(channel)

    // 若此前有后台兜底连接，可释放
    const bg = backgroundSockets.get(channel.serial)
    if (bg) {
      try {
        bg.ws.close()
      }
      catch {}
      backgroundSockets.delete(channel.serial)
    }

    return () => {
      unregisterChannel(channel)
    }
  }

  function unregisterChannel(channel: GroupControlChannel) {
    const set = registeredChannels.get(channel.serial)
    if (set) {
      set.delete(channel)
      if (set.size === 0) {
        registeredChannels.delete(channel.serial)
      }
    }
    syncBackgroundConnections()
  }

  function broadcastControl(
    sourceSerial: string,
    bytes: Uint8Array,
    sourceSize?: GroupControlSize,
  ) {
    if (!isActive.value || isBroadcasting)
      return
    const targets = targetSerials.value
    if (targets.length <= 1)
      return
    if (!targets.includes(sourceSerial))
      return

    isBroadcasting = true
    try {
      const srcW = sourceSize?.width || 1080
      const srcH = sourceSize?.height || 2400
      const sSize: GroupControlSize = { width: srcW, height: srcH }

      for (const targetSerial of targets) {
        if (targetSerial === sourceSerial)
          continue

        // 优先向已挂载的 UI 通道（DashTile 或小窗）同步
        const channels = registeredChannels.get(targetSerial)
        if (channels && channels.size > 0) {
          for (const ch of channels) {
            try {
              const chSize = ch.getDeviceSize?.() || sSize
              const tSize: GroupControlSize = {
                width: chSize.width > 0 ? chSize.width : srcW,
                height: chSize.height > 0 ? chSize.height : srcH,
              }
              let targetPacket = bytes
              if (bytes[0] === TYPE_INJECT_TOUCH_EVENT) {
                targetPacket = transformTouchPacket(bytes, sSize, tSize)
              }
              else if (bytes[0] === TYPE_INJECT_SCROLL_EVENT) {
                targetPacket = transformScrollPacket(bytes, sSize, tSize)
              }
              ch.send(targetPacket)
            }
            catch (err) {
              console.warn(`[GroupControl] 向设备 ${targetSerial} 同步控制指令异常:`, err)
            }
          }
          continue
        }

        // 兜底：向后台 WebSocket 通道同步
        const bg = backgroundSockets.get(targetSerial)
        if (bg && bg.ws.readyState === WebSocket.OPEN) {
          try {
            const tSize = bg.size.width > 0 ? bg.size : sSize
            let targetPacket = bytes
            if (bytes[0] === TYPE_INJECT_TOUCH_EVENT) {
              targetPacket = transformTouchPacket(bytes, sSize, tSize)
            }
            else if (bytes[0] === TYPE_INJECT_SCROLL_EVENT) {
              targetPacket = transformScrollPacket(bytes, sSize, tSize)
            }
            bg.ws.send(targetPacket as Uint8Array<ArrayBuffer>)
          }
          catch (err) {
            console.warn(`[GroupControl] 向后台连接 ${targetSerial} 同步失败:`, err)
          }
        }
      }
    }
    finally {
      isBroadcasting = false
    }
  }

  return {
    isActive,
    targetSerials,
    setActive,
    setTargets,
    registerChannel,
    unregisterChannel,
    broadcastControl,
  }
}
