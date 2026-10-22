import { computed, ref, type Ref } from 'vue'
import type { Device } from '../api/devices.ts'
import { sendDeviceEvent } from '../api/deviceActions.ts'

export type BatchItemStatus =
  | 'waiting'
  | 'running'
  | 'success'
  | 'failed'
  | 'timeout'
  | 'cancelled'
  | 'unknown'

export interface BatchItemResult {
  serial: string
  name?: string
  status: BatchItemStatus
  error?: string
  elapsedMs?: number
  requestId?: string
}

export interface BatchSummary {
  batchId: string
  action: string
  total: number
  success: number
  failed: number
  cancelled: number
  unknown: number
  inProgress: boolean
}

export interface BatchExecutionOptions {
  batchId: string
  generation: number
  serials: string[]
  action: 'back' | 'home' | 'app_switch'
  concurrency?: number
  perDeviceTimeoutMs?: number
  overallTimeoutMs?: number
  signal?: AbortSignal
  targetResults?: BatchItemResult[]
  isGenerationActive?: (gen: number) => boolean
  isTargetStillOperable?: (serial: string) => boolean
  runner?: (serial: string, signal: AbortSignal) => Promise<unknown>
}

export const MAX_BATCH_TARGETS = 64
export const MAX_BATCH_CONCURRENCY = 4
export const DEFAULT_BATCH_CONCURRENCY = 4
export const DEFAULT_PER_DEVICE_TIMEOUT_MS = 15000
export const DEFAULT_OVERALL_TIMEOUT_MS = 120000

export function dedupeSerials(serials: string[] | null | undefined): string[] {
  if (!serials || !Array.isArray(serials))
    return []
  const seen = new Set<string>()
  const result: string[] = []
  for (const s of serials) {
    if (s && typeof s === 'string' && !seen.has(s)) {
      seen.add(s)
      result.push(s)
    }
  }
  return result.slice(0, MAX_BATCH_TARGETS)
}

export function filterSelectableDevices(devices: Device[] | null | undefined): Device[] {
  if (!devices || !Array.isArray(devices))
    return []
  return devices.filter(d => d && d.state === 'device')
}

export async function executeBatchAction(options: BatchExecutionOptions): Promise<{
  results: BatchItemResult[]
  summary: BatchSummary
}> {
  const {
    batchId,
    generation,
    serials,
    action,
    concurrency = DEFAULT_BATCH_CONCURRENCY,
    perDeviceTimeoutMs = DEFAULT_PER_DEVICE_TIMEOUT_MS,
    overallTimeoutMs = DEFAULT_OVERALL_TIMEOUT_MS,
    signal,
    targetResults,
    isGenerationActive = () => true,
    isTargetStillOperable = () => true,
    runner,
  } = options

  const targets = dedupeSerials(serials)
  const results: BatchItemResult[] = targetResults || targets.map(s => ({
    serial: s,
    status: 'waiting',
  }))

  // 严格限制并发数不能超过 4
  const boundedConcurrency = Math.min(Math.max(1, concurrency), MAX_BATCH_CONCURRENCY)
  const effectiveConcurrency = Math.min(boundedConcurrency, targets.length || 1)
  let cursor = 0

  const overallController = new AbortController()
  const overallTimeoutTimer = setTimeout(() => {
    overallController.abort(new Error(`整批执行超过截止时间 (${overallTimeoutMs}ms)`))
  }, overallTimeoutMs)

  function onParentAbort() {
    overallController.abort(signal?.reason || new Error('操作已取消'))
  }

  if (signal) {
    if (signal.aborted) {
      overallController.abort(signal.reason || new Error('操作已取消'))
    }
    else {
      signal.addEventListener('abort', onParentAbort)
    }
  }

  async function worker() {
    while (cursor < targets.length) {
      if (overallController.signal.aborted || !isGenerationActive(generation)) {
        break
      }

      const idx = cursor++
      const targetSerial = targets[idx]
      const item = results.find(r => r.serial === targetSerial)
      if (!item)
        continue

      if (!isGenerationActive(generation))
        break

      // 派发前动态复核：设备是否依然可操作（避免掉线/隐藏/Mock 切换后继续发指令）
      if (!isTargetStillOperable(targetSerial)) {
        item.status = 'cancelled'
        item.error = '目标设备已不可操作或已隐藏'
        continue
      }

      item.status = 'running'
      const start = Date.now()

      const itemController = new AbortController()
      let dispatched = false

      const onCombinedAbort = () => {
        itemController.abort(overallController.signal.reason || new Error('批次取消'))
      }
      overallController.signal.addEventListener('abort', onCombinedAbort)

      const perItemTimer = setTimeout(() => {
        const timeoutErr = new Error(`设备执行超时 (${perDeviceTimeoutMs}ms)`)
        timeoutErr.name = 'TimeoutError'
        itemController.abort(timeoutErr)
      }, perDeviceTimeoutMs)

      // 提取外层强制守卫定时器和监听器引用，确保 100% 被清理
      let guardTimer: ReturnType<typeof setTimeout> | undefined
      let guardAbortHandler: (() => void) | undefined

      try {
        dispatched = true
        const runPromise = runner
          ? runner(item.serial, itemController.signal)
          : sendDeviceEvent(item.serial, action, itemController.signal, perDeviceTimeoutMs)

        const guardPromise = new Promise((_, reject) => {
          guardTimer = setTimeout(() => {
            reject(new Error(`强制守卫超时 (${perDeviceTimeoutMs + 500}ms)`))
          }, perDeviceTimeoutMs + 500)

          guardAbortHandler = () => {
            reject(itemController.signal.reason || new Error('操作已取消'))
          }
          itemController.signal.addEventListener('abort', guardAbortHandler, { once: true })
        })

        const res = await Promise.race([runPromise, guardPromise])
        if (!isGenerationActive(generation)) {
          return
        }

        item.elapsedMs = Date.now() - start
        item.status = 'success'
        if (res && typeof res === 'object' && 'request_id' in res) {
          item.requestId = (res as { request_id: string }).request_id
        }
      }
      catch (err: unknown) {
        if (!isGenerationActive(generation)) {
          return
        }

        item.elapsedMs = Date.now() - start
        const errorObj = err instanceof Error ? err : new Error(String(err))

        // 语义规范：一旦派发，因超时或取消导致未确认时，一律为 unknown，未派发才为 cancelled
        if (dispatched) {
          if (errorObj.name === 'TimeoutError' || errorObj.message.includes('超时')) {
            item.status = 'unknown'
            item.error = errorObj.message || '超时未确认'
          }
          else if (overallController.signal.aborted || itemController.signal.aborted) {
            item.status = 'unknown'
            item.error = '执行被中断，实际状态未知'
          }
          else {
            item.status = 'failed'
            item.error = errorObj.message || '执行失败'
          }
        }
        else {
          item.status = 'cancelled'
          item.error = '操作已取消'
        }
      }
      finally {
        // 彻底清理所有计时器与监听器，杜绝任何资源泄漏
        clearTimeout(perItemTimer)
        if (guardTimer !== undefined) {
          clearTimeout(guardTimer)
          guardTimer = undefined
        }
        if (guardAbortHandler) {
          itemController.signal.removeEventListener('abort', guardAbortHandler)
          guardAbortHandler = undefined
        }
        overallController.signal.removeEventListener('abort', onCombinedAbort)
      }
    }
  }

  try {
    const workers = Array.from({ length: effectiveConcurrency }, () => worker())
    await Promise.all(workers)
  }
  finally {
    clearTimeout(overallTimeoutTimer)
    if (signal) {
      signal.removeEventListener('abort', onParentAbort)
    }
  }

  // 代次仍然有效时清理未派发项
  if (isGenerationActive(generation)) {
    for (const item of results) {
      if (item.status === 'waiting') {
        item.status = 'cancelled'
        item.error = '未派发'
      }
    }
  }

  const summary: BatchSummary = {
    batchId,
    action,
    total: results.length,
    success: results.filter(r => r.status === 'success').length,
    failed: results.filter(r => r.status === 'failed').length,
    cancelled: results.filter(r => r.status === 'cancelled').length,
    unknown: results.filter(r => r.status === 'unknown').length,
    inProgress: false,
  }

  return { results, summary }
}

export function useDeviceBatch(isOperableCheck?: (serial: string) => boolean) {
  const isRunning = ref(false)
  const currentBatchId = ref<string>('')
  const currentAction = ref<string>('')
  const currentGeneration = ref<number>(0)
  const results: Ref<BatchItemResult[]> = ref([])
  let activeAbortController: AbortController | null = null

  const summary = computed<BatchSummary>(() => {
    return {
      batchId: currentBatchId.value,
      action: currentAction.value,
      total: results.value.length,
      success: results.value.filter(r => r.status === 'success').length,
      failed: results.value.filter(r => r.status === 'failed').length,
      cancelled: results.value.filter(r => r.status === 'cancelled').length,
      unknown: results.value.filter(r => r.status === 'unknown').length,
      inProgress: isRunning.value,
    }
  })

  function isGenerationActive(gen: number): boolean {
    return gen === currentGeneration.value
  }

  async function runBatch(
    serials: string[],
    action: 'back' | 'home' | 'app_switch',
    deviceMap?: Record<string, string>,
    customRunner?: (serial: string, signal: AbortSignal) => Promise<unknown>,
  ) {
    if (isRunning.value)
      return

    const targets = dedupeSerials(serials)
    if (targets.length === 0)
      return

    currentGeneration.value++
    const myGen = currentGeneration.value
    const batchId = `batch-${Date.now()}-${Math.random().toString(36).slice(2, 7)}`
    currentBatchId.value = batchId
    currentAction.value = action
    isRunning.value = true

    results.value = targets.map(s => ({
      serial: s,
      name: deviceMap?.[s] || s,
      status: 'waiting',
    }))

    activeAbortController = new AbortController()
    const { signal } = activeAbortController

    try {
      await executeBatchAction({
        batchId,
        generation: myGen,
        serials: targets,
        action,
        concurrency: DEFAULT_BATCH_CONCURRENCY,
        perDeviceTimeoutMs: DEFAULT_PER_DEVICE_TIMEOUT_MS,
        overallTimeoutMs: DEFAULT_OVERALL_TIMEOUT_MS,
        signal,
        targetResults: results.value,
        isGenerationActive,
        isTargetStillOperable: isOperableCheck || (() => true),
        runner: customRunner,
      })
    }
    catch (err) {
      console.error(`[useDeviceBatch] 批次 ${batchId} 执行发生异常:`, err)
    }
    finally {
      if (isGenerationActive(myGen)) {
        isRunning.value = false
        activeAbortController = null
      }
    }
  }

  function cancelBatch() {
    if (activeAbortController) {
      activeAbortController.abort(new Error('用户取消批次'))
    }
    currentGeneration.value++
    isRunning.value = false
    activeAbortController = null

    for (const r of results.value) {
      if (r.status === 'waiting') {
        r.status = 'cancelled'
        r.error = '操作已取消'
      }
      else if (r.status === 'running') {
        r.status = 'unknown'
        r.error = '执行被取消，实际状态未知'
      }
    }
  }

  function clearResults() {
    if (!isRunning.value) {
      results.value = []
      currentBatchId.value = ''
      currentAction.value = ''
    }
  }

  function destroy() {
    cancelBatch()
    clearResults()
  }

  return {
    isRunning,
    currentBatchId,
    currentAction,
    currentGeneration,
    results,
    summary,
    runBatch,
    cancelBatch,
    clearResults,
    destroy,
  }
}
