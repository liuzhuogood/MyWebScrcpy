// 画中画：优先原生 Video PiP（canvas.captureStream），兜底文档画中画。

export class PictureInPicture {
  onChange: ((active: boolean) => void) | null = null

  supported = false
  active = false

  private canvas: HTMLCanvasElement
  private video: HTMLVideoElement | null = null
  private stream: MediaStream | null = null
  private pipWindow: Window | null = null
  private pictureCanvas: HTMLCanvasElement | null = null
  private drawFrame = 0
  private webkitPiPActive = false
  private readonly supportsStandard: boolean
  private readonly supportsWebkit: boolean
  private readonly supportsDocument: boolean

  constructor(canvas: HTMLCanvasElement) {
    this.canvas = canvas
    const videoProto = window.HTMLVideoElement?.prototype as (HTMLVideoElement & { requestPictureInPicture?: unknown }) | undefined
    const canCapture = typeof canvas.captureStream === 'function'
    this.supportsStandard = canCapture && typeof videoProto?.requestPictureInPicture === 'function'
    this.supportsWebkit
      = canCapture
        && typeof (videoProto as unknown as Record<string, unknown>)?.webkitSetPresentationMode === 'function'
    const docPiP = window as unknown as { documentPictureInPicture?: { requestWindow: (opts: { width: number, height: number }) => Promise<Window> } }
    this.supportsDocument = typeof docPiP.documentPictureInPicture?.requestWindow === 'function'
    this.supported = this.supportsStandard || this.supportsWebkit || this.supportsDocument
  }

  isActive(): boolean {
    if (this.video && document.pictureInPictureElement === this.video)
      return true
    const w = this.video as unknown as { webkitPresentationMode?: string } | null
    if (w?.webkitPresentationMode === 'picture-in-picture')
      return true
    return !!this.pipWindow && !this.pipWindow.closed
  }

  async toggle(): Promise<void> {
    if (this.isActive()) {
      await this.close()
      return
    }
    this.cleanup()
    await this.open()
  }

  private async open(): Promise<void> {
    if (!this.canvas.width || !this.canvas.height)
      throw new Error('投屏画面尚未准备好')
    try {
      if (this.supportsStandard || this.supportsWebkit)
        await this.openVideoPiP()
      else await this.openDocumentPiP()
    }
    catch (videoError) {
      this.cleanup()
      try {
        await this.openDocumentPiP()
      }
      catch (documentError) {
        console.warn('[pip] 打开失败:', videoError, documentError)
        this.cleanup()
        throw new Error('当前浏览器或页面设置不支持该功能')
      }
    }
    this.sync()
  }

  private async openVideoPiP(): Promise<void> {
    this.stream = this.canvas.captureStream(30)
    const video = document.createElement('video')
    video.muted = true
    video.playsInline = true
    video.style.display = 'none'
    video.srcObject = this.stream
    document.body.append(video)
    this.video = video
    video.addEventListener('webkitpresentationmodechanged', () => {
      const w = video as unknown as { webkitPresentationMode?: string }
      if (this.webkitPiPActive && w.webkitPresentationMode !== 'picture-in-picture')
        this.cleanup()
    })
    await video.play()
    await this.waitForVideoFrame(video)
    if (this.supportsStandard) {
      await (video as HTMLVideoElement & { requestPictureInPicture: () => Promise<void> }).requestPictureInPicture()
    }
    else {
      const w = video as unknown as { webkitSetPresentationMode: (m: string) => void }
      w.webkitSetPresentationMode('picture-in-picture')
      this.webkitPiPActive = true
    }
  }

  private waitForVideoFrame(video: HTMLVideoElement): Promise<void> {
    return new Promise((resolve, reject) => {
      const finish = () => {
        clearTimeout(timeout)
        video.removeEventListener('loadedmetadata', check)
        video.removeEventListener('canplay', check)
      }
      const check = () => {
        if (!video.videoWidth || !video.videoHeight)
          return
        finish()
        resolve()
      }
      const timeout = setTimeout(() => {
        finish()
        reject(new Error('画中画画面未准备完成'))
      }, 2500)
      video.addEventListener('loadedmetadata', check)
      video.addEventListener('canplay', check)
      check()
    })
  }

  private async openDocumentPiP(): Promise<void> {
    const docPiP = (window as unknown as { documentPictureInPicture: { requestWindow: (opts: { width: number, height: number }) => Promise<Window> } }).documentPictureInPicture
    if (!docPiP)
      throw new Error('当前浏览器不支持文档画中画')
    const ratio = this.canvas.width / this.canvas.height
    let width = Math.min(480, Math.max(240, this.canvas.width))
    let height = Math.round(width / ratio)
    if (height > 720) {
      height = 720
      width = Math.round(height * ratio)
    }
    const win = await docPiP.requestWindow({ width, height })
    this.pipWindow = win
    win.document.body.innerHTML = '<canvas aria-label="设备投屏画面"></canvas>'
    win.document.head.insertAdjacentHTML(
      'beforeend',
      '<style>html,body{width:100%;height:100%;margin:0;background:#000;overflow:hidden}canvas{display:block;width:100%;height:100%;object-fit:contain}</style>',
    )
    this.pictureCanvas = win.document.querySelector('canvas')
    win.addEventListener('pagehide', () => this.cleanup(), { once: true })
    this.drawDocumentPiP()
  }

  private drawDocumentPiP() {
    if (!this.pipWindow || this.pipWindow.closed || !this.pictureCanvas) {
      this.cleanup()
      return
    }
    if (this.pictureCanvas.width !== this.canvas.width || this.pictureCanvas.height !== this.canvas.height) {
      this.pictureCanvas.width = this.canvas.width
      this.pictureCanvas.height = this.canvas.height
    }
    this.pictureCanvas.getContext('2d')?.drawImage(this.canvas, 0, 0)
    this.drawFrame = requestAnimationFrame(() => this.drawDocumentPiP())
  }

  async close(): Promise<void> {
    const activeVideo = this.video
    if (this.pipWindow && !this.pipWindow.closed)
      this.pipWindow.close()
    if (activeVideo && document.pictureInPictureElement === activeVideo) {
      try {
        await document.exitPictureInPicture()
      }
      catch (e) {
        if ((e as Error).name !== 'InvalidStateError')
          console.warn('[pip] 关闭失败:', e)
      }
    }
    else if ((activeVideo as unknown as { webkitPresentationMode?: string } | null)?.webkitPresentationMode === 'picture-in-picture') {
      ;(activeVideo as unknown as { webkitSetPresentationMode: (m: string) => void }).webkitSetPresentationMode('inline')
    }
    this.cleanup()
  }

  private cleanup() {
    cancelAnimationFrame(this.drawFrame)
    this.drawFrame = 0
    this.pictureCanvas = null
    this.pipWindow = null
    this.webkitPiPActive = false
    if (this.video) {
      this.video.srcObject = null
      this.video.remove()
      this.video = null
    }
    this.stream?.getTracks().forEach(t => t.stop())
    this.stream = null
    this.sync()
  }

  private sync() {
    this.active = this.isActive()
    this.onChange?.(this.active)
  }
}
