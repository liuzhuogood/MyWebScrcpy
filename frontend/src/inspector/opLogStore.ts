import { ref } from 'vue'
import { appendOperationLogs, deleteOperationLogs, listOperationLogs, type OperationLog } from '@/api/operationLogs'

export interface OpEntry {
  id: string
  time: string
  desc: string
  adb: string
  occurredAt: number
}

export const opEntries = ref<OpEntry[]>([])
export const opLoading = ref(false)
export const opClearing = ref(false)
export const opError = ref('')
export const opLoadFailed = ref(false)

const MAX_ENTRIES = 500
const MAX_PENDING = 1000
const sessionID = Array.from(crypto.getRandomValues(new Uint8Array(16)), v => v.toString(16).padStart(2, '0')).join('')
let sequence = 0
let revision = 0
let running = false
let timer: ReturnType<typeof setTimeout> | undefined

type Pending = { item: OperationLog } | { clear: true, ids: Set<string>, resolve: () => void }
const pending: Pending[] = []

function toEntry(item: OperationLog): OpEntry {
  return {
    id: item.id,
    time: new Date(item.occurred_at).toLocaleString('zh-CN', { hour12: false }),
    desc: item.description,
    adb: item.adb_command,
    occurredAt: item.occurred_at,
  }
}

export function appendOp(serial: string, type: OperationLog['type'], desc: string, adb: string) {
  const item: OperationLog = {
    id: `${sessionID}-${++sequence}`,
    serial,
    type,
    description: desc,
    adb_command: adb,
    occurred_at: Date.now(),
  }
  opEntries.value = [...opEntries.value, toEntry(item)].slice(-MAX_ENTRIES)
  // Only enqueue in the input callback; all HTTP, encoding and retries happen later.
  if (pending.length >= MAX_PENDING) {
    opError.value = '部分操作记录未保存：待保存记录过多'
    console.warn('[OperationLog] 待保存记录已达上限')
    return
  }
  pending.push({ item })
  schedule()
}

function schedule() {
  if (!running && timer === undefined) {
    timer = setTimeout(() => {
      timer = undefined
      void flush()
    }, 250)
  }
}

async function retry(action: () => Promise<unknown>) {
  for (let attempt = 0; ; attempt++) {
    try {
      await action()
      return
    }
    catch (error) {
      const status = (error as Error & { status?: number }).status
      if (attempt >= 2 || (status !== undefined && status < 500 && status !== 429))
        throw error
      await new Promise(resolve => setTimeout(resolve, 1000 * (attempt + 1)))
    }
  }
}

async function flush() {
  if (running)
    return
  running = true
  try {
    while (pending.length) {
      const first = pending[0]
      if ('clear' in first) {
        pending.shift()
        try {
          await retry(deleteOperationLogs)
          opEntries.value = opEntries.value.filter(entry => !first.ids.has(entry.id))
        }
        catch (error) {
          opError.value = '清理操作记录失败，请稍后重试'
          console.warn('[OperationLog] 清理失败', error)
        }
        finally {
          opClearing.value = false
          first.resolve()
        }
        continue
      }
      const items: OperationLog[] = []
      while (pending.length && items.length < 50) {
        const next = pending[0]
        if (!('item' in next))
          break
        // Keep large text/swipe batches below the server's body limit.
        if (items.length && new TextEncoder().encode(JSON.stringify({ items: [...items, next.item] })).length > 240 * 1024)
          break
        items.push(next.item)
        pending.shift()
      }
      try {
        await retry(() => appendOperationLogs(items))
      }
      catch (error) {
        opError.value = '部分操作记录未保存，请检查服务连接'
        console.warn('[OperationLog] 后台保存失败', error)
      }
    }
  }
  finally {
    running = false
    if (pending.length)
      schedule()
  }
}

export async function loadOps() {
  if (opLoading.value || opClearing.value)
    return
  const version = revision
  opLoading.value = true
  if (opLoadFailed.value)
    opError.value = ''
  opLoadFailed.value = false
  try {
    const { items } = await listOperationLogs()
    if (version !== revision)
      return
    const merged = new Map(items.map(item => [item.id, toEntry(item)]))
    const cutoff = Date.now() - 7 * 24 * 60 * 60 * 1000
    for (const entry of opEntries.value) {
      if (entry.occurredAt > cutoff)
        merged.set(entry.id, entry)
    }
    opEntries.value = [...merged.values()].sort((a, b) => a.occurredAt - b.occurredAt).slice(-MAX_ENTRIES)
  }
  catch (error) {
    if (version === revision) {
      opError.value = '读取操作记录失败，请稍后重试'
      opLoadFailed.value = true
      console.warn('[OperationLog] 读取失败', error)
    }
  }
  finally {
    opLoading.value = false
  }
}

export function clearOps(): Promise<void> {
  if (opClearing.value)
    return Promise.resolve()
  revision++
  opError.value = ''
  opLoadFailed.value = false
  opClearing.value = true
  // The marker orders deletion after earlier uploads and before subsequent actions.
  return new Promise(resolve => {
    pending.push({ clear: true, ids: new Set(opEntries.value.map(entry => entry.id)), resolve })
    schedule()
  })
}
