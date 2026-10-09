// WebCodecs VideoDecoder 封装，解码 scrcpy 的 H.264 裸流。
//
// 服务端二进制帧格式：[1B kind][8B pts BE][payload…]
//   kind 0=config(SPS/PPS)  1=key(IDR)  2=delta(P)  3=session(8B: [4B width][4B height])

export type VideoCodec = 'h264' | 'h265' | 'av1'

export interface DecodeErrorInfo {
  error: unknown
  fatal: boolean
  count: number
  message: string
}

const FrameKind = {
  Config: 0,
  Key: 1,
  Delta: 2,
  Session: 3,
} as const

export class ScrcpyDecoder {
  onFps: ((fps: string) => void) | null = null
  onError: ((info: DecodeErrorInfo) => void) | null = null
  onDecodedFrame: (() => void) | null = null
  onDeviceResize: ((w: number, h: number) => void) | null = null

  private canvas: HTMLCanvasElement
  private ctx: CanvasRenderingContext2D
  private decoder: VideoDecoder | null = null
  private codec: VideoCodec = 'h264'
  private configured = false
  private frameCount = 0
  private lastFrameTime = 0
  private decodeErrorCount = 0
  private decodeErrorNotified = false

  constructor(canvas: HTMLCanvasElement) {
    this.canvas = canvas
    // 取色需要频繁 getImageData，声明偏好避免浏览器警告
    const ctx = canvas.getContext('2d', { willReadFrequently: true })
    if (!ctx)
      throw new Error('Canvas 2D 上下文不可用')
    this.ctx = ctx
  }

  private codecString(): string {
    // H.264 Baseline，具体 profile 由 config 包 SPS 决定，这里给通用值
    if (this.codec === 'h264')
      return 'avc1.42E01F'
    if (this.codec === 'h265')
      return 'hvc1.1.6.L93.B0'
    if (this.codec === 'av1')
      return 'av01.0.04M.08'
    return 'avc1.42E01F'
  }

  configure(codec: string, width: number, height: number) {
    this.codec = (codec === 'h265' || codec === 'av1' ? codec : 'h264')
    this.canvas.width = width
    this.canvas.height = height

    if (typeof VideoDecoder === 'undefined')
      throw new Error('此浏览器不支持 WebCodecs，请用 Chrome 94+')

    this.close()
    this.decoder = new VideoDecoder({
      output: frame => this.onFrame(frame),
      error: (e) => {
        console.error('[decoder] error', e)
        this.notifyDecodeError(e, true)
      },
    })
    this.configured = false
    this.decodeErrorCount = 0
    this.decodeErrorNotified = false
    console.log(`[decoder] canvas ${width}x${height}, codec ${codec}，等待 config 包…`)
  }

  handleFrame(data: Uint8Array) {
    const view = new DataView(data.buffer, data.byteOffset, data.byteLength)
    const kind = view.getUint8(0)
    const pts = Number(view.getBigUint64(1, false))
    const payload = data.subarray(9)

    try {
      switch (kind) {
        case FrameKind.Config: this.onConfig(payload); break
        case FrameKind.Key: this.decodeChunk(payload, true, pts); break
        case FrameKind.Delta: this.decodeChunk(payload, false, pts); break
        case FrameKind.Session: this.onResize(payload); break
        default: break
      }
    }
    catch (e) {
      console.error('[decoder] handleFrame error:', e)
    }
  }

  private onConfig(payload: Uint8Array) {
    if (!this.decoder)
      return
    // 每次 config 都重新 configure：旋转时服务端重建编码器，旧 decoder 解不了新输出
    const description = this.toAVCC(payload)
    if (this.decoder.state === 'configured')
      this.decoder.reset()
    this.decoder.configure({
      codec: this.codecString(),
      description,
      optimizeForLatency: true,
    })
    this.configured = true
    console.log('[decoder] decoder 已 configure, description size=', description.length)
  }

  /** Annex-B 起始码流 → AVCC description；已是长度前缀则直通 */
  private toAVCC(payload: Uint8Array): Uint8Array<ArrayBuffer> {
    if (
      payload.length >= 4
      && payload[0] === 0 && payload[1] === 0
      && payload[2] === 0 && payload[3] === 1
    ) {
      return this.annexBToAvcC(payload)
    }
    return payload as Uint8Array<ArrayBuffer>
  }

  private annexBToAvcC(data: Uint8Array): Uint8Array<ArrayBuffer> {
    const nals = this.splitNAL(data)
    let sps: Uint8Array | null = null
    let pps: Uint8Array | null = null
    for (const nal of nals) {
      if (nal.length === 0)
        continue
      const nalType = nal[0] & 0x1F
      if (nalType === 7)
        sps = nal
      else if (nalType === 8)
        pps = nal
    }
    if (!sps || !pps) {
      console.warn('[decoder] Annex-B 中未找到 SPS/PPS, nals=', nals.length)
      return data as Uint8Array<ArrayBuffer>
    }
    // avcC: 01 <profile> <compat> <level> FF E1 <spsLen 2B> <sps> 01 <ppsLen 2B> <pps>
    const buf = new Uint8Array(11 + sps.length + pps.length)
    let i = 0
    buf[i++] = 1
    buf[i++] = sps[1]
    buf[i++] = sps[2]
    buf[i++] = sps[3]
    buf[i++] = 0xFF
    buf[i++] = 0xE1
    buf[i++] = (sps.length >> 8) & 0xFF
    buf[i++] = sps.length & 0xFF
    buf.set(sps, i); i += sps.length
    buf[i++] = 1
    buf[i++] = (pps.length >> 8) & 0xFF
    buf[i++] = pps.length & 0xFF
    buf.set(pps, i)
    console.log('[decoder] Annex-B → avcC 转换完成, profile=', sps[1], 'level=', sps[3])
    return buf as Uint8Array<ArrayBuffer>
  }

  /** 按起始码切分 NAL（去掉起始码） */
  private splitNAL(data: Uint8Array): Uint8Array[] {
    const nals: Uint8Array[] = []
    let i = 0
    while (i < data.length) {
      const start = this.findStartCode(data, i)
      if (start === -1)
        break
      let end = this.findStartCode(data, start + 3)
      if (end === -1)
        end = data.length
      let nalStart = start
      if (data[nalStart + 2] === 1)
        nalStart += 3
      else if (data[nalStart + 3] === 1)
        nalStart += 4
      nals.push(data.subarray(nalStart, end))
      i = end
    }
    return nals
  }

  private findStartCode(data: Uint8Array, from: number): number {
    for (let i = from; i < data.length - 3; i++) {
      if (data[i] === 0 && data[i + 1] === 0) {
        if (data[i + 2] === 1)
          return i
        if (data[i + 2] === 0 && data[i + 3] === 1)
          return i
      }
    }
    return -1
  }

  private decodeChunk(payload: Uint8Array, isKey: boolean, pts: number) {
    if (!this.configured || !this.decoder || this.decoder.state !== 'configured')
      return
    // media 包可能是 AVCC 或 Annex-B；Annex-B 转长度前缀后喂给 WebCodecs
    let chunkData: Uint8Array<ArrayBuffer> = payload as Uint8Array<ArrayBuffer>
    if (
      payload.length >= 4
      && payload[0] === 0 && payload[1] === 0
      && payload[2] === 0 && payload[3] === 1
    ) {
      chunkData = this.annexBToLengthPrefixed(payload)
    }
    const chunk = new EncodedVideoChunk({
      type: isKey ? 'key' : 'delta',
      timestamp: pts,
      data: chunkData,
    })
    try {
      this.decoder.decode(chunk)
    }
    catch (e) {
      this.notifyDecodeError(e, false)
    }
  }

  private notifyDecodeError(error: unknown, fatal: boolean) {
    this.decodeErrorCount++
    // 单个坏帧不断流；连续失败或致命错误才上报
    if (!fatal && this.decodeErrorCount < 3)
      return
    if (this.decodeErrorNotified)
      return
    this.decodeErrorNotified = true
    this.onError?.({
      error,
      fatal,
      count: this.decodeErrorCount,
      message: fatal ? '视频解码失败' : '视频连续解码失败',
    })
  }

  private annexBToLengthPrefixed(data: Uint8Array): Uint8Array<ArrayBuffer> {
    const nals = this.splitNAL(data)
    let totalLen = 0
    for (const nal of nals) totalLen += 4 + nal.length
    const buf = new Uint8Array(totalLen)
    let off = 0
    for (const nal of nals) {
      new DataView(buf.buffer, off, 4).setUint32(0, nal.length, false)
      buf.set(nal, off + 4)
      off += 4 + nal.length
    }
    return buf as Uint8Array<ArrayBuffer>
  }

  private onResize(payload: Uint8Array) {
    const view = new DataView(payload.buffer, payload.byteOffset, payload.byteLength)
    const w = view.getUint32(0, false)
    const h = view.getUint32(4, false)
    console.log(`[decoder] 尺寸变化: ${w}x${h}`)
    this.canvas.width = w
    this.canvas.height = h
    this.onDeviceResize?.(w, h)
  }

  private onFrame(frame: VideoFrame) {
    this.ctx.drawImage(frame, 0, 0, this.canvas.width, this.canvas.height)
    frame.close()
    this.frameCount++
    this.decodeErrorCount = 0
    this.decodeErrorNotified = false
    this.onDecodedFrame?.()
    const now = performance.now()
    if (now - this.lastFrameTime > 1000) {
      const fps = ((this.frameCount * 1000) / (now - this.lastFrameTime)).toFixed(1)
      this.onFps?.(fps)
      this.frameCount = 0
      this.lastFrameTime = now
    }
  }

  close() {
    if (this.decoder) {
      if (this.decoder.state !== 'closed')
        this.decoder.close()
      this.decoder = null
    }
    this.configured = false
  }
}
