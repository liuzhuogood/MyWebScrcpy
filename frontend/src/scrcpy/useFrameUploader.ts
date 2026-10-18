import { onUnmounted, type Ref } from 'vue'

const UPLOAD_INTERVAL_MS = 1000
const JPEG_QUALITY = 0.8

/** 把投屏 canvas 的解码帧低频上传到后端，供 /api/device-state 等无解码能力的消费方取用 */
export function useFrameUploader(serial: string, canvasRef: Ref<HTMLCanvasElement | null>, isActive: () => boolean) {
  let timer: number | undefined
  let uploading = false

  async function uploadOnce() {
    if (uploading || document.visibilityState !== 'visible' || !isActive())
      return
    const canvas = canvasRef.value
    if (!canvas || !canvas.width || !canvas.height)
      return
    uploading = true
    try {
      const blob = await new Promise<Blob | null>(resolve => canvas.toBlob(resolve, 'image/jpeg', JPEG_QUALITY))
      if (!blob)
        return
      await fetch(`/api/vision/frame?serial=${encodeURIComponent(serial)}`, {
        method: 'POST',
        headers: { 'Content-Type': 'image/jpeg' },
        body: blob,
      })
    }
    catch {}
    finally {
      uploading = false
    }
  }

  function start() {
    stop()
    timer = window.setInterval(() => void uploadOnce(), UPLOAD_INTERVAL_MS)
  }

  function stop() {
    if (timer !== undefined) {
      window.clearInterval(timer)
      timer = undefined
    }
  }

  onUnmounted(stop)
  return { start, stop }
}
