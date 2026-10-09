import { computed, onUnmounted, ref } from 'vue'
import {
  getReplay,
  pauseReplay,
  seekReplay,
  setReplaySegment,
  startReplay,
  stopReplay,
  type ReplayView,
} from '@/api/replay'
import { listRecordings, type RecordingEntry, deleteRecording } from '@/api/recordings'

export interface ReplayListEntry extends RecordingEntry {}

/** 重放状态机：本地墙钟计时为主，后端 loop_count 对齐；短片绕回靠取模 */
export function useReplay(serial: string, onSwitchStream: (msg: string) => void) {
  const replaying = ref(false)
  const panelOpen = ref(false)
  const list = ref<ReplayListEntry[]>([])
  const listError = ref('')
  const currentId = ref<string | null>(null)
  const currentEntry = ref<RecordingEntry | null>(null)
  const totalMs = ref<number | null>(null)
  const loop = ref(false)
  const paused = ref(false)
  const segmentOn = ref(false)
  const segmentLenMs = ref(0)
  const msg = ref('')

  let sessionSeq = 0
  let baseStartMs = 0
  let lastLoopCount: number | null = null
  let pausedTotalMs = 0
  let pausedElapsedMs: number | null = null
  let pausedAtMs: number | null = null
  let totalFixTried = false
  let tickTimer: number | undefined
  let pollTimer: number | undefined
  let dragPosMs: number | null = null
  let dragging = false
  // 墙钟节拍：驱动进度 computed 重算（页签切后台节流，回来按 Date.now 差值追齐）
  const now = ref(Date.now())

  const elapsedMs = computed<number | null>(() => {
    if (!replaying.value || !baseStartMs)
      return null
    // 订阅节拍
    void now.value
    if (dragging && dragPosMs != null)
      return dragPosMs
    if (paused.value && pausedElapsedMs != null)
      return pausedElapsedMs
    let ms = Date.now() - baseStartMs - pausedTotalMs
    if (ms < 0)
      ms = 0
    if (totalMs.value) {
      if (loop.value)
        ms = ms % totalMs.value
      else if (ms > totalMs.value)
        ms = totalMs.value
    }
    if (segmentOn.value && segmentLenMs.value > 0) {
      const start = 0
      const windowLen = totalMs.value ? Math.min(segmentLenMs.value, totalMs.value) : segmentLenMs.value
      ms = start + (Math.max(0, ms - start) % windowLen)
      if (totalMs.value)
        ms = Math.min(ms, totalMs.value)
    }
    return ms
  })

  // 老逻辑小段循环起点为开启时刻；简化为从头循环（段起点固定 0），与后端 segment 参数对齐由 start 决定
  const remainMs = computed<number | null>(() => {
    const e = elapsedMs.value
    const t = totalMs.value
    return e != null && t != null ? Math.max(0, t - e) : null
  })

  function entryTotalMs(entry: RecordingEntry | null): number | null {
    try {
      if (!entry?.started_at || !entry.ended_at)
        return null
      const ms = new Date(entry.ended_at).getTime() - new Date(entry.started_at).getTime()
      return Number.isFinite(ms) && ms > 0 ? ms : null
    }
    catch {
      return null
    }
  }

  function stopTick() {
    if (tickTimer !== undefined) {
      clearInterval(tickTimer)
      tickTimer = undefined
    }
  }

  function startTick() {
    stopTick()
    tickTimer = window.setInterval(() => {
      now.value = Date.now()
    }, 500)
  }

  function stopPoll() {
    if (pollTimer !== undefined) {
      clearTimeout(pollTimer)
      pollTimer = undefined
    }
  }

  function poll() {
    stopPoll()
    if (!replaying.value)
      return
    const mySeq = sessionSeq
    pollTimer = window.setTimeout(async () => {
      if (mySeq !== sessionSeq || !replaying.value)
        return
      try {
        const { replay } = await getReplay(serial)
        if (mySeq !== sessionSeq || !replaying.value)
          return
        if (!replay) {
          await finish('重放已播完，已回到实时流', mySeq)
          return
        }
        applyView(replay)
        if (totalMs.value == null && currentId.value && !totalFixTried) {
          totalFixTried = true
          const found = await lookupEntry(currentId.value).catch(() => null)
          if (found && replaying.value && totalMs.value == null) {
            if (!currentEntry.value)
              currentEntry.value = found
            totalMs.value = entryTotalMs(found)
          }
        }
      }
      catch { /* 抖动保持，继续轮询 */ }
      if (mySeq !== sessionSeq || !replaying.value)
        return
      poll()
    }, 2000)
  }

  function applyView(view: ReplayView) {
    if (typeof view.loop === 'boolean')
      loop.value = view.loop
    if (typeof view.segment_on === 'boolean')
      segmentOn.value = view.segment_on
    if (typeof view.segment_len_ms === 'number')
      segmentLenMs.value = Math.max(0, view.segment_len_ms)
    const n = view.loop_count
    if (typeof n === 'number') {
      if (lastLoopCount == null) {
        lastLoopCount = n
      }
      else if (n !== lastLoopCount) {
        lastLoopCount = n
        baseStartMs = Date.now()
      }
    }
    else {
      lastLoopCount = null
    }
  }

  async function lookupEntry(id: string): Promise<RecordingEntry | null> {
    try {
      const { recordings } = await listRecordings(serial)
      return recordings.find(e => e.recording_id === id) || null
    }
    catch {
      return null
    }
  }

  async function finish(message: string | undefined, seq: number) {
    if (seq !== sessionSeq)
      return
    stopPoll()
    stopTick()
    replaying.value = false
    sessionSeq++
    currentId.value = null
    currentEntry.value = null
    totalMs.value = null
    baseStartMs = 0
    lastLoopCount = null
    loop.value = false
    paused.value = false
    pausedTotalMs = 0
    pausedElapsedMs = null
    pausedAtMs = null
    dragging = false
    dragPosMs = null
    segmentOn.value = false
    segmentLenMs.value = 0
    onSwitchStream('正在回到实时流…')
    if (message)
      msg.value = message
  }

  async function start(id: string, entry?: RecordingEntry | null, loopNext?: boolean) {
    if (replaying.value) {
      msg.value = '已在重放中，请先停止当前重放'
      return
    }
    const useLoop = loopNext ?? loop.value
    try {
      const view = await startReplay(serial, id, useLoop)
      replaying.value = true
      sessionSeq++
      currentId.value = id
      currentEntry.value = entry || await lookupEntry(id)
      totalMs.value = entryTotalMs(currentEntry.value)
      totalFixTried = false
      applyView(view)
      segmentOn.value = false
      segmentLenMs.value = 0
      let t = Date.parse(view.started_at)
      baseStartMs = Number.isFinite(t) ? t : Date.now()
      if (baseStartMs > Date.now() || Math.abs(Date.now() - baseStartMs) > 3600 * 1000)
        baseStartMs = Date.now()
      msg.value = loop.value ? '已开始重放（循环播放）' : '已开始重放'
      onSwitchStream('正在切换到重放…')
      startTick()
      poll()
    }
    catch (e) {
      stopTick()
      stopPoll()
      const status = (e as Error & { status?: number }).status
      if (status === 404 || status === 501)
        msg.value = '重放接口暂未就绪，请稍后重试'
      else if (status === 409)
        msg.value = '该设备正在录制或重放中，请稍后重试'
      else msg.value = '开始重放失败，请重试'
    }
  }

  async function stop(manual = true) {
    if (!replaying.value && manual)
      return
    const mySeq = sessionSeq
    try {
      await stopReplay(serial)
    }
    catch (e) {
      if ((e as Error & { status?: number }).status === 404) {
        await finish(manual ? '重放已结束，回到实时流' : undefined, mySeq)
        return
      }
      msg.value = '停止重放失败，请重试'
      return
    }
    await finish(manual ? '已停止重放，回到实时流' : undefined, mySeq)
  }

  async function togglePause() {
    if (!replaying.value)
      return
    const target = !paused.value
    try {
      await pauseReplay(serial, target)
      if (target) {
        pausedElapsedMs = elapsedMs.value
        pausedAtMs = Date.now()
      }
      else {
        if (pausedAtMs != null)
          pausedTotalMs += Date.now() - pausedAtMs
        pausedAtMs = null
        pausedElapsedMs = null
      }
      paused.value = target
    }
    catch {
      msg.value = '暂停操作失败，请重试'
    }
  }

  async function seek(positionMs: number) {
    if (!replaying.value || !totalMs.value)
      return
    try {
      await seekReplay(serial, positionMs)
      paused.value = false
      pausedElapsedMs = null
      pausedAtMs = null
      pausedTotalMs = 0
      let pos = positionMs
      if (loop.value && totalMs.value)
        pos = positionMs % totalMs.value
      baseStartMs = Date.now() - pos
    }
    catch {
      msg.value = '跳转失败，请重试'
    }
  }

  function previewSeek(positionMs: number | null) {
    dragPosMs = positionMs
  }

  function setDragging(on: boolean) {
    dragging = on
    if (!on)
      dragPosMs = null
  }

  async function toggleSegment(lenMs: number) {
    if (!replaying.value)
      return
    if (segmentOn.value) {
      try {
        await setReplaySegment(serial, { enabled: false })
        segmentOn.value = false
        segmentLenMs.value = 0
        msg.value = '已停止小段循环，回到整体重放'
      }
      catch {
        msg.value = '停止小段循环失败，请重试'
      }
      return
    }
    const start = elapsedMs.value
    if (!lenMs || start == null) {
      msg.value = '请选择有效的小段时长'
      return
    }
    try {
      await setReplaySegment(serial, { enabled: true, start_ms: Math.round(start), len_ms: lenMs })
      segmentOn.value = true
      segmentLenMs.value = lenMs
      msg.value = `已开启小段循环（${Math.round(lenMs / 1000)} 秒）`
    }
    catch {
      msg.value = '开启小段循环失败，请重试'
    }
  }

  async function loadList() {
    try {
      const { recordings } = await listRecordings(serial)
      list.value = recordings
      listError.value = ''
    }
    catch {
      listError.value = '录制列表加载失败，请重试'
    }
  }

  async function removeRecording(id: string) {
    await deleteRecording(serial, id)
    await loadList()
  }

  /** 打开面板时恢复后端仍活动的重放，避免孤儿态无法停止 */
  async function restore() {
    if (replaying.value)
      return
    msg.value = ''
    try {
      const { replay } = await getReplay(serial)
      if (!replay?.recording_id)
        return
      replaying.value = true
      sessionSeq++
      currentId.value = replay.recording_id
      applyView(replay)
      paused.value = !!replay.paused
      segmentOn.value = !!replay.segment_on
      segmentLenMs.value = replay.segment_len_ms ?? 0
      let t = Date.parse(replay.started_at)
      baseStartMs = Number.isFinite(t) ? t : Date.now()
      if (baseStartMs > Date.now() || Math.abs(Date.now() - baseStartMs) > 3600 * 1000)
        baseStartMs = Date.now()
      if (paused.value) {
        pausedElapsedMs = elapsedMs.value
        pausedAtMs = Date.now()
      }
      currentEntry.value = await lookupEntry(currentId.value)
      totalMs.value = entryTotalMs(currentEntry.value)
      startTick()
      poll()
    }
    catch { /* 保持现状 */ }
  }

  onUnmounted(() => {
    stopPoll()
    stopTick()
  })

  return {
    replaying, panelOpen, list, listError, currentId, currentEntry,
    totalMs, elapsedMs, remainMs, loop, paused, segmentOn, segmentLenMs, msg,
    start, stop, togglePause, seek, previewSeek, setDragging,
    toggleSegment, loadList, removeRecording, restore,
  }
}
