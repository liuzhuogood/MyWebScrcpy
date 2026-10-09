<script setup lang="ts">
import { Download, Play, Trash2, X } from 'lucide-vue-next'
import { computed, ref } from 'vue'
import { Badge } from '@/components/ui/badge'
import { Button } from '@/components/ui/button'
import { Checkbox } from '@/components/ui/checkbox'
import { Progress } from '@/components/ui/progress'
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from '@/components/ui/select'
import { formatMs } from '@/api/replay'
import { formatRecordingDuration, formatRecordingTime, recordingDownloadUrl, type RecordingEntry } from '@/api/recordings'

const props = defineProps<{
  serial: string
  replaying: boolean
  currentId: string | null
  currentName: string
  elapsed: number | null
  total: number | null
  remain: number | null
  loopChecked: boolean
  paused: boolean
  segmentOn: boolean
  segmentLen: number
  msg: string
  list: RecordingEntry[]
  listError: string
}>()

const emit = defineEmits<{
  (e: 'close'): void
  (e: 'stop'): void
  (e: 'pause'): void
  (e: 'loop', v: boolean): void
  (e: 'seek', ms: number): void
  (e: 'seek-preview', ms: number | null): void
  (e: 'seg-toggle', lenMs: number): void
  (e: 'play', entry: RecordingEntry): void
  (e: 'delete', entry: RecordingEntry): void
}>()

const segLen = ref('2000')
const segCustom = ref('')
const segCustomShown = ref(false)

const pct = computed(() => {
  if (props.elapsed == null || !props.total)
    return 0
  return Math.min(100, Math.max(0, (props.elapsed / props.total) * 100))
})

function segLenMs(): number {
  if (segLen.value === '0') {
    const sec = Number.parseFloat(segCustom.value)
    return Number.isFinite(sec) && sec > 0 ? Math.round(sec * 1000) : 0
  }
  return Number.parseInt(segLen.value, 10) || 0
}

function onBarDown(e: PointerEvent) {
  if (!props.replaying || !props.total)
    return
  e.preventDefault()
  const bar = e.currentTarget as HTMLElement
  const frac = (clientX: number) => {
    const rect = bar.getBoundingClientRect()
    return rect.width > 0 ? Math.min(1, Math.max(0, (clientX - rect.left) / rect.width)) : 0
  }
  emit('seek-preview', frac(e.clientX) * (props.total ?? 0))
  try {
    bar.setPointerCapture?.(e.pointerId)
  }
  catch { /* 忽略 */ }
  const move = (ev: PointerEvent) => emit('seek-preview', frac(ev.clientX) * (props.total ?? 0))
  const up = (ev: PointerEvent) => {
    emit('seek', frac(ev.clientX) * (props.total ?? 0))
    emit('seek-preview', null)
    window.removeEventListener('pointermove', move)
    window.removeEventListener('pointerup', up)
    window.removeEventListener('pointercancel', cancel)
  }
  const cancel = () => {
    emit('seek-preview', null)
    window.removeEventListener('pointermove', move)
    window.removeEventListener('pointerup', up)
    window.removeEventListener('pointercancel', cancel)
  }
  window.addEventListener('pointermove', move)
  window.addEventListener('pointerup', up)
  window.addEventListener('pointercancel', cancel)
}
</script>

<template>
  <aside class="flex h-full min-h-0 w-full flex-col overflow-hidden rounded-xl border bg-card" aria-label="重放控制面板">
    <div class="flex items-center gap-2 border-b px-3 py-2">
      <strong class="text-sm">重放控制</strong>
      <Badge v-if="replaying && loopChecked">循环</Badge>
      <Button variant="ghost" size="icon" class="ml-auto h-7 w-7" aria-label="关闭重放面板" @click="emit('close')">
        <X class="h-4 w-4" />
      </Button>
    </div>

    <div class="min-h-0 flex-1 space-y-3 overflow-auto p-3">
      <p class="truncate font-mono text-xs text-muted-foreground" :title="currentName">{{ currentName || '--' }}</p>

      <div
        role="progressbar" aria-label="重放进度" aria-valuemin="0" aria-valuemax="100" :aria-valuenow="Math.round(pct)"
        class="cursor-pointer" @pointerdown="onBarDown"
      >
        <Progress :model-value="pct" class="h-2" />
      </div>
      <div class="flex justify-between font-mono text-xs text-muted-foreground">
        <span>已播 {{ formatMs(elapsed) }}</span>
        <span>总 {{ formatMs(total) }}</span>
        <span>剩余 {{ formatMs(remain) }}</span>
      </div>

      <div class="flex gap-1.5">
        <Button size="sm" variant="outline" :disabled="!replaying" @click="emit('pause')">
          {{ paused ? '继续' : '暂停' }}
        </Button>
        <Button size="sm" variant="outline" :disabled="!replaying" @click="emit('stop')">停止重放</Button>
      </div>

      <label class="flex cursor-pointer items-center gap-2 text-sm">
        <Checkbox :model-value="loopChecked" :disabled="replaying" aria-label="循环播放" @update:model-value="emit('loop', $event === true)" />
        循环播放
      </label>
      <p v-if="replaying" class="text-[11px] text-muted-foreground">重放进行中，循环设置下次重放生效</p>

      <div class="rounded-md border p-2">
        <div class="flex items-center gap-2">
          <span class="text-sm font-medium">小段循环</span>
          <Button size="sm" variant="outline" class="ml-auto h-7 text-xs" :disabled="!replaying" @click="emit('seg-toggle', segLenMs())">
            {{ segmentOn ? '停止' : '开启' }}
          </Button>
        </div>
        <div class="mt-1.5 flex gap-1.5">
          <Select
            :model-value="segLen" :disabled="segmentOn"
            @update:model-value="(v) => { segLen = String(v); segCustomShown = String(v) === '0' }"
          >
            <SelectTrigger class="h-8 flex-1" aria-label="小段时长"><SelectValue /></SelectTrigger>
            <SelectContent>
              <SelectItem value="1000">1 秒</SelectItem>
              <SelectItem value="2000">2 秒</SelectItem>
              <SelectItem value="3000">3 秒</SelectItem>
              <SelectItem value="5000">5 秒</SelectItem>
              <SelectItem value="10000">10 秒</SelectItem>
              <SelectItem value="15000">15 秒</SelectItem>
              <SelectItem value="30000">30 秒</SelectItem>
              <SelectItem value="0">自定义…</SelectItem>
            </SelectContent>
          </Select>
        </div>
        <input
          v-if="segCustomShown" v-model="segCustom" type="number" min="1" step="1" inputmode="numeric" placeholder="秒"
          aria-label="自定义小段秒数" :disabled="segmentOn"
          class="mt-1.5 h-8 w-full rounded-md border border-input bg-transparent px-2 text-sm"
        />
        <p v-if="msg" class="mt-1 text-[11px] text-muted-foreground" aria-live="polite">{{ msg }}</p>
      </div>

      <div>
        <p class="mb-1 text-sm font-medium">录制列表</p>
        <p v-if="listError" class="text-xs text-destructive">{{ listError }}</p>
        <p v-else-if="list.length === 0" class="text-xs text-muted-foreground">暂无录制记录</p>
        <ul v-else class="space-y-1.5">
          <li
            v-for="entry in list" :key="entry.recording_id"
            class="rounded-md border p-2" :class="{ 'border-primary': replaying && entry.recording_id === currentId }"
          >
            <p class="text-xs font-medium">{{ formatRecordingTime(entry) }}</p>
            <p class="text-[11px] text-muted-foreground">录制 {{ formatRecordingDuration(entry) }}</p>
            <div class="mt-1 flex gap-1">
              <Button
                v-if="entry.status === 'completed'" variant="ghost" size="sm" class="h-6 text-xs"
                :disabled="replaying && entry.recording_id === currentId" @click="emit('play', entry)"
              >
                <Play class="h-3 w-3" />重放
              </Button>
              <Button v-if="entry.status === 'completed'" variant="ghost" size="sm" class="h-6 text-xs" as-child>
                <a :href="recordingDownloadUrl(serial, entry.recording_id)"><Download class="h-3 w-3" />下载</a>
              </Button>
              <Button
                v-if="entry.status !== 'recording' && entry.status !== 'stopping'"
                variant="ghost" size="sm" class="h-6 text-xs text-destructive"
                :disabled="replaying && entry.recording_id === currentId" title="删除这条录制记录"
                @click="emit('delete', entry)"
              >
                <Trash2 class="h-3 w-3" />删除
              </Button>
            </div>
          </li>
        </ul>
      </div>
    </div>
  </aside>
</template>
