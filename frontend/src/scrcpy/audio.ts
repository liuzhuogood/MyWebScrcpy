// 设备音频播放：Web Audio + AudioDecoder 解 AAC 包，默认静音、用户主动开启。
//
// 服务端音频包格式：[1B 0x80][1B kind][8B pts BE][payload…]，kind 1=config(AAC ASC) 0=media。

export type AudioStatus = 'unavailable' | 'connecting' | 'ready' | 'decoder_error'

const SAMPLE_RATES = [96000, 88200, 64000, 48000, 44100, 32000, 24000, 22050, 16000, 12000, 11025, 8000, 7350]

export class DeviceAudio {
  onStatus: ((status: AudioStatus, reason?: string) => void) | null = null

  status: AudioStatus = 'unavailable'
  reason = '设备声音不可用'
  muted = true

  private decoder: AudioDecoder | null = null
  private context: AudioContext | null = null
  private gain: GainNode | null = null
  private anchorPTS: number | null = null
  private anchorTime: number | null = null
  private readonly supported = typeof AudioDecoder !== 'undefined'

  private setStatus(next: AudioStatus, reason?: string) {
    this.status = next
    if (reason !== undefined)
      this.reason = reason
    this.onStatus?.(next, this.reason)
  }

  /** 服务端 audio.status 消息 */
  serverStatus(msg: { status: AudioStatus, reason?: string }) {
    if (msg.status === 'connecting')
      return
    if (!this.supported) {
      this.setStatus('decoder_error', '当前浏览器不支持设备音频播放')
      return
    }
    this.setStatus(msg.status, msg.reason)
  }

  /** 用户点击：静音 ↔ 播放（需 AudioContext 用户手势） */
  async toggle(): Promise<void> {
    if (this.status !== 'ready')
      return
    this.muted = !this.muted
    try {
      await this.ensureContext()
      this.setStatus('ready')
    }
    catch {
      this.setStatus('decoder_error', '浏览器未允许播放设备声音')
    }
  }

  /** 服务端 0x80 音频包 */
  handlePacket(data: ArrayBuffer) {
    if (!this.supported)
      return
    const bytes = new Uint8Array(data)
    if (bytes.length < 10)
      return
    const isConfig = bytes[1] === 1
    const pts = Number(new DataView(bytes.buffer, bytes.byteOffset + 2, 8).getBigUint64(0))
    const payload = bytes.slice(10)
    try {
      if (isConfig) {
        this.resetDecoder(payload)
      }
      else if (this.decoder && this.status === 'ready') {
        this.decoder.decode(new EncodedAudioChunk({ type: 'key', timestamp: pts, data: payload }))
      }
    }
    catch (e) {
      this.setStatus('decoder_error', `音频不可播放：${e instanceof Error ? e.message : e}`)
    }
  }

  private parseASC(bytes: Uint8Array): AudioDecoderConfig {
    if (bytes.length < 2)
      throw new Error('AAC 配置不完整')
    let objectType = bytes[0] >> 3
    const frequencyIndex = ((bytes[0] & 7) << 1) | (bytes[1] >> 7)
    if (objectType === 31)
      objectType = 32 + ((bytes[1] & 0x7F) << 1) + (bytes[2] >> 7)
    const channels = (bytes[1] >> 3) & 15
    const sampleRate
      = frequencyIndex === 15
        ? ((bytes[1] & 7) << 21) | (bytes[2] << 13) | (bytes[3] << 5) | (bytes[4] >> 3)
        : SAMPLE_RATES[frequencyIndex]
    if (!sampleRate || !channels || !objectType)
      throw new Error('不支持的 AAC 配置')
    return {
      codec: `mp4a.40.${objectType}`,
      sampleRate,
      numberOfChannels: channels,
      description: bytes,
    }
  }

  private resetDecoder(description: Uint8Array) {
    if (!this.supported)
      throw new Error('当前浏览器不支持设备音频播放')
    try {
      this.decoder?.close()
    }
    catch { /* 忽略 */ }
    const config = this.parseASC(description)
    this.decoder = new AudioDecoder({
      output: audio => this.outputAudio(audio),
      error: e => this.setStatus('decoder_error', `音频解码失败：${e.message || e}`),
    })
    this.decoder.configure(config)
    this.anchorPTS = null
    this.anchorTime = null
    this.setStatus('ready')
  }

  private async ensureContext() {
    if (!this.context) {
      this.context = new AudioContext()
      this.gain = this.context.createGain()
      this.gain.connect(this.context.destination)
    }
    if (this.context.state !== 'running')
      await this.context.resume()
    if (this.gain)
      this.gain.gain.value = this.muted ? 0 : 1
  }

  private outputAudio(audio: AudioData) {
    try {
      if (!this.context || this.muted)
        return
      const now = this.context.currentTime
      const pts = Number(audio.timestamp) / 1e6
      if (this.anchorPTS === null || Math.abs(pts - this.anchorPTS - (now - (this.anchorTime ?? now))) > 0.35) {
        this.anchorPTS = pts
        this.anchorTime = now + 0.08
      }
      const start = Math.max(now + 0.01, (this.anchorTime ?? now) + (pts - (this.anchorPTS ?? pts)))
      const buffer = this.context.createBuffer(audio.numberOfChannels, audio.numberOfFrames, audio.sampleRate)
      for (let ch = 0; ch < audio.numberOfChannels; ch++)
        audio.copyTo(buffer.getChannelData(ch), { planeIndex: ch, format: 'f32-planar' })
      const source = this.context.createBufferSource()
      source.buffer = buffer
      if (this.gain)
        source.connect(this.gain)
      source.start(start)
    }
    finally {
      audio.close()
    }
  }

  close() {
    try {
      this.decoder?.close()
    }
    catch { /* 忽略 */ }
    this.decoder = null
    void this.context?.close().catch(() => {})
    this.context = null
    this.gain = null
  }
}
