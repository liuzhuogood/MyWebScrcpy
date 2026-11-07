import { ACTION_DOWN, ACTION_MOVE, ACTION_UP, TYPE_BACK_OR_SCREEN_ON, TYPE_INJECT_KEYCODE, TYPE_INJECT_TOUCH_EVENT } from './protocol'

interface ControlMessage {
  bytes: Uint8Array | string
  fromGroupSync: boolean
}
interface SenderOptions<T> {
  write: (message: T) => void
  bufferedAmount: () => number
  requestFrame: (callback: () => void) => number
  cancelFrame: (id: number) => void
  setTimer: (callback: () => void, delay: number) => number
  clearTimer: (id: number) => void
  now: () => number
  onCongested: (reason: string) => void
}

const MAX_BUFFER_BYTES = 4 * 1024
const MAX_PENDING_POINTERS = 16
const MAX_HELD_INPUTS = 128
const CONGESTION_TIMEOUT_MS = 500
const RETRY_INTERVAL_MS = 16

function inputIdentity(bytes: Uint8Array | string): string | undefined {
  if (typeof bytes === 'string') return
  const view = new DataView(bytes.buffer, bytes.byteOffset, bytes.byteLength)
  if (bytes.length === 32 && bytes[0] === TYPE_INJECT_TOUCH_EVENT)
    return `touch:${view.getBigUint64(2)}`
  if (bytes.length === 14 && bytes[0] === TYPE_INJECT_KEYCODE)
    return `key:${view.getUint32(2)}`
  if (bytes.length === 2 && bytes[0] === TYPE_BACK_OR_SCREEN_ON)
    return 'back'
}

/** Keep one unsent MOVE per pointer; all other commands are ordering barriers. */
export class ControlSender<T extends ControlMessage> {
  private options: SenderOptions<T>
  private moves = new Map<string, T>()
  private held = new Map<string, T>()
  private frame: number | undefined
  private timer: number | undefined
  private congestionSince: number | undefined
  private blocked = false
  private epoch = 0
  private connectionEpoch = 0

  constructor(options: SenderOptions<T>) {
    this.options = options
  }

  send(message: T) {
    if (this.blocked) return
    const connectionEpoch = this.connectionEpoch
    const { bytes } = message
    // A group source already coalesces motion. A target must not introduce a
    // second animation-frame delay or broadcast the packet back to the group.
    if (!message.fromGroupSync && typeof bytes !== 'string' && bytes.length === 32
      && bytes[0] === TYPE_INJECT_TOUCH_EVENT && bytes[1] === ACTION_MOVE) {
      const key = inputIdentity(bytes)!
      this.moves.set(key, message)
      if (this.moves.size > MAX_PENDING_POINTERS) {
        this.fail('待发送触摸指针超过上限')
        return
      }
      if (this.frame === undefined && this.timer === undefined) {
        const epoch = this.epoch
        this.frame = this.options.requestFrame(() => {
          if (epoch !== this.epoch) return
          this.frame = undefined
          this.flush()
        })
      }
      return
    }
    if (!this.flush(true) || connectionEpoch !== this.connectionEpoch) return
    if (this.isBackedUp()) {
      this.fail('控制通道发送积压超过上限')
      return
    }
    this.write(message)
  }

  private flush(barrier = false): boolean {
    if (this.blocked) return false
    if (this.moves.size === 0) return true
    this.cancelScheduled()
    if (this.isBackedUp()) {
      if (barrier || this.congestionExpired()) {
        this.fail('控制通道拥塞，正在重新连接')
        return false
      }
      this.monitorBuffer()
      return false
    }
    const moves = [...this.moves.values()]
    this.moves.clear()
    for (const message of moves) {
      if (!this.write(message)) return false
    }
    return true
  }

  private write(message: T): boolean {
    const connectionEpoch = this.connectionEpoch
    try {
      this.options.write(message)
      const key = inputIdentity(message.bytes)
      if (key && typeof message.bytes !== 'string') {
        const action = message.bytes[1]
        if (action === ACTION_UP || action === 3) this.held.delete(key)
        else if (action === ACTION_DOWN || this.held.has(key)) this.held.set(key, message)
        if (this.held.size > MAX_HELD_INPUTS) {
          this.fail('按住的输入数量超过上限')
          return false
        }
      }
      this.monitorBuffer()
      return !this.blocked && connectionEpoch === this.connectionEpoch
    }
    catch {
      this.fail('控制通道发送失败，正在重新连接')
      return false
    }
  }

  private congestionExpired() {
    return this.congestionSince !== undefined && this.options.now() - this.congestionSince >= CONGESTION_TIMEOUT_MS
  }

  private isBackedUp() {
    const buffered = this.options.bufferedAmount()
    if (buffered === 0) {
      this.congestionSince = undefined
      return false
    }
    this.congestionSince ??= this.options.now()
    return buffered > MAX_BUFFER_BYTES || this.congestionExpired()
  }

  private monitorBuffer() {
    if (this.blocked) return
    const buffered = this.options.bufferedAmount()
    if (buffered === 0) {
      this.congestionSince = undefined
      return
    }
    this.congestionSince ??= this.options.now()
    if (this.congestionExpired()) {
      this.fail('控制通道持续积压，正在重新连接')
      return
    }
    if (this.timer === undefined) {
      const epoch = this.epoch
      this.timer = this.options.setTimer(() => {
        if (epoch !== this.epoch) return
        this.timer = undefined
        if (this.moves.size > 0) this.flush()
        else this.monitorBuffer()
      }, RETRY_INTERVAL_MS)
    }
  }

  private cancelScheduled() {
    this.epoch++
    if (this.frame !== undefined) this.options.cancelFrame(this.frame)
    if (this.timer !== undefined) this.options.clearTimer(this.timer)
    this.frame = this.timer = undefined
  }

  private clearPending() {
    this.cancelScheduled()
    this.congestionSince = undefined
    this.moves.clear()
  }

  private releaseHeld(localOnly = false) {
    let failed = false
    for (const [key, message] of this.held) {
      if (localOnly && message.fromGroupSync) continue
      this.held.delete(key)
      const bytes = (message.bytes as Uint8Array).slice()
      bytes[1] = ACTION_UP
      if (bytes[0] === TYPE_INJECT_TOUCH_EVENT) bytes.fill(0, 22)
      try { this.options.write({ ...message, bytes }) }
      catch { failed = true }
    }
    return failed
  }

  private fail(reason: string) {
    if (this.blocked) return
    this.blocked = true
    this.clearPending()
    // Best effort before closing; the server also releases this connection's
    // held inputs on disconnect, so a stalled release cannot leave a finger down.
    this.releaseHeld()
    this.options.onCongested(reason)
  }

  /** Losing focus cancels local gestures without releasing another group source. */
  releaseLocal() {
    this.clearPending()
    if (this.releaseHeld(true)) this.fail('释放控制输入失败，正在重新连接')
  }

  /** Cancel the old connection epoch, including late scheduled callbacks. */
  reset() {
    this.connectionEpoch++
    this.clearPending()
    const failed = this.releaseHeld()
    this.blocked = false
    if (failed) this.fail('释放控制输入失败，正在重新连接')
  }
}
