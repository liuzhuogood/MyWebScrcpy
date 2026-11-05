import { computed, onUnmounted, ref, watch, type Ref } from 'vue'
import QRCode from 'qrcode'
import { pollDiscovery, startPairing, startScan, stopDiscovery, type DiscoveryJob, type ScanRequest } from '@/api/discovery'

// One independent leased session per kind. Changing tabs does not cancel work.
export function useDiscoverySession(open: Ref<boolean>, kind: 'pair' | 'scan', onComplete: () => Promise<void>) {
  const job = ref<DiscoveryJob | null>(null)
  const busy = ref(false)
  const error = ref('')
  const qrImage = ref('')
  const running = computed(() => job.value?.state === 'running' || job.value?.state === 'waiting')
  let generation = 0
  let controller: AbortController | undefined
  let timer: ReturnType<typeof setTimeout> | undefined
  let stopping = Promise.resolve()

  function report(cause: unknown) {
    error.value = cause instanceof Error ? cause.message : '设备发现失败'
    console.warn(`[device-discovery] ${kind}`, error.value)
  }
  function stop() {
    const previous = job.value
    generation++
    controller?.abort()
    controller = undefined
    clearTimeout(timer)
    job.value = null
    qrImage.value = ''
    busy.value = false
    if (previous && (previous.state === 'running' || previous.state === 'waiting')) {
      const current = generation
      stopping = stopping.then(() => stopDiscovery(previous.id)).then(() => {}, cause => {
        if (current === generation && open.value) report(cause)
        else console.warn(`[device-discovery] ${kind} stop failed; lease will expire`, cause)
      })
    }
    return stopping
  }
  async function poll(id: string, current: number, signal: AbortSignal) {
    try {
      const next = await pollDiscovery(id, signal)
      if (current !== generation || !open.value) return
      job.value = next
      if (next.state === 'running' || next.state === 'waiting') {
        timer = setTimeout(() => void poll(id, current, signal), 1000)
      }
      else {
        qrImage.value = ''
        if (next.state === 'failed') error.value = next.error ?? '设备发现失败'
        void onComplete().catch(report)
      }
    }
    catch (cause) {
      if (current !== generation || signal.aborted) return
      report(cause)
      void stop()
    }
  }
  async function start(body?: ScanRequest) {
    if (!open.value || busy.value || running.value) return
    const current = ++generation
    busy.value = true
    error.value = ''
    const request = new AbortController()
    controller = request
    try {
      await stopping
      if (current !== generation || !open.value) return
      const next = kind === 'pair' ? await startPairing(request.signal) : await startScan(body!, request.signal)
      if (current !== generation || !open.value) { await stopDiscovery(next.id); return }
      job.value = next
      if (kind === 'pair') {
        if (!next.qr) throw new Error('配对二维码未生成')
        const image = await QRCode.toDataURL(next.qr, { width: 240, margin: 2, errorCorrectionLevel: 'M' })
        if (current !== generation || !open.value) return
        qrImage.value = image
      }
      timer = setTimeout(() => void poll(next.id, current, request.signal), 500)
    }
    catch (cause) {
      if (current !== generation || request.signal.aborted) return
      report(cause)
      void stop()
    }
    finally { if (current === generation) busy.value = false }
  }
  watch(open, value => {
    if (!value) void stop()
    else if (kind === 'pair') void start()
  }, { immediate: true })
  onUnmounted(() => { void stop() })
  return { job, busy, error, qrImage, running, start, stop }
}
