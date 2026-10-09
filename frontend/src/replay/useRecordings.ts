import { computed, ref } from 'vue'
import {
  deleteRecording,
  getRecording,
  listRecordings,
  startRecording,
  stopRecording,
  type RecordingEntry,
} from '@/api/recordings'

/** 受控录屏：创建/停止轮询/记录列表/删除；与重放互斥由调用方协调 */
export function useRecordings(serial: string, isReplaying: () => boolean) {
  const history = ref<RecordingEntry[]>([])
  const historyError = ref('')
  const active = ref<RecordingEntry | null>(null)
  const busy = ref(false)
  const stateMsg = ref('')
  const minutes = ref(30)
  const recordAudio = ref(false)
  let timer: number | undefined

  const running = computed(() => !!active.value && ['recording', 'stopping'].includes(active.value.status))

  async function loadHistory() {
    try {
      const data = await listRecordings(serial)
      history.value = data.recordings || []
      historyError.value = ''
    }
    catch {
      historyError.value = '暂时无法获取录制记录'
    }
  }

  function stopPoll() {
    if (timer !== undefined) {
      clearTimeout(timer)
      timer = undefined
    }
  }

  function poll() {
    stopPoll()
    const current = active.value
    if (!current || !running.value)
      return
    timer = window.setTimeout(async () => {
      try {
        active.value = await getRecording(serial, current.recording_id)
      }
      catch { /* 保持状态继续轮询 */ }
      if (active.value && running.value) {
        if (!['recording', 'stopping'].includes(active.value.status))
          await loadHistory()
        poll()
      }
    }, 1000)
  }

  async function start(maxMinutes: number, withAudio: boolean): Promise<void> {
    if (isReplaying()) {
      stateMsg.value = '重放期间无法开始录制，请先停止重放。'
      return
    }
    busy.value = true
    stateMsg.value = '正在开始录制…'
    try {
      active.value = await startRecording(serial, maxMinutes * 60000, withAudio)
      stateMsg.value = ''
      await loadHistory()
    }
    catch (e) {
      stateMsg.value = e instanceof Error ? e.message : '暂时无法开始录制，请稍后重试。'
    }
    finally {
      busy.value = false
      poll()
    }
  }

  async function stop(): Promise<void> {
    if (!active.value)
      return
    busy.value = true
    try {
      active.value = await stopRecording(serial, active.value.recording_id)
      stateMsg.value = ''
      await loadHistory()
    }
    catch {
      stateMsg.value = '暂时无法停止录制，请重试。'
    }
    finally {
      busy.value = false
      poll()
    }
  }

  async function remove(id: string): Promise<void> {
    await deleteRecording(serial, id)
    await loadHistory()
  }

  function dispose() {
    stopPoll()
  }

  return {
    history, historyError, active, busy, stateMsg, minutes, recordAudio, running,
    loadHistory, start, stop, remove, dispose,
  }
}
