<script setup lang="ts">
import { Download, Play, Trash2 } from 'lucide-vue-next'
import { computed } from 'vue'
import { Button } from '@/components/ui/button'
import { Checkbox } from '@/components/ui/checkbox'
import { Empty, EmptyDescription, EmptyHeader, EmptyTitle } from '@/components/ui/empty'
import { Slider } from '@/components/ui/slider'
import { formatMinutes, formatRecordingDuration, formatRecordingTime, recordingDownloadUrl, type RecordingEntry } from '@/api/recordings'

const props = defineProps<{
  serial: string
  minutes: number
  recordAudio: boolean
  busy: boolean
  running: boolean
  stateMsg: string
  history: RecordingEntry[]
  historyError: string
}>()

const emit = defineEmits<{
  (e: 'update:minutes', v: number): void
  (e: 'update:record-audio', v: boolean): void
  (e: 'start'): void
  (e: 'stop'): void
  (e: 'replay', entry: RecordingEntry): void
  (e: 'delete', entry: RecordingEntry): void
}>()

const minutesModel = computed({
  get: () => [props.minutes],
  set: (v: number[]) => emit('update:minutes', v[0]),
})
</script>

<template>
  <div class="w-80 max-w-full space-y-3 p-1">
    <div>
      <div class="mb-1 flex items-center justify-between text-sm">
        <span class="font-medium">录制时长</span>
        <output class="text-muted-foreground">{{ formatMinutes(minutes) }}</output>
      </div>
      <Slider v-model="minutesModel" :min="1" :max="480" :step="1" aria-label="录制时长（分钟）" />
      <label class="mt-2 flex cursor-pointer items-center gap-2 text-sm">
        <Checkbox :model-value="recordAudio" @update:model-value="emit('update:record-audio', $event === true)" />
        同时录制设备声音
      </label>
      <div class="mt-2 flex gap-2">
        <Button v-if="running" class="flex-1" :disabled="busy" @click="emit('stop')">停止录制</Button>
        <Button v-else class="flex-1" :disabled="busy" @click="emit('start')">开始录制</Button>
      </div>
      <p v-if="stateMsg" class="mt-1 text-xs text-muted-foreground" aria-live="polite">{{ stateMsg }}</p>
    </div>

    <div>
      <p class="mb-1 text-sm font-medium">录制记录</p>
      <p v-if="historyError" class="text-xs text-destructive">{{ historyError }}</p>
      <Empty v-else-if="history.length === 0">
        <EmptyHeader>
          <EmptyTitle class="text-sm">暂无录制记录</EmptyTitle>
          <EmptyDescription>开始一次录制后会显示在这里</EmptyDescription>
        </EmptyHeader>
      </Empty>
      <ul v-else class="max-h-64 space-y-2 overflow-auto">
        <li v-for="entry in history" :key="entry.recording_id" class="rounded-md border p-2">
          <p class="text-xs font-medium">{{ formatRecordingTime(entry) }}</p>
          <p class="text-[11px] text-muted-foreground">
            录制 {{ formatRecordingDuration(entry) }}{{ entry.record_audio ? ' · 含声音' : ' · 无声音' }} · {{ entry.status }}
          </p>
          <div class="mt-1 flex gap-1">
            <Button
              v-if="entry.status === 'completed'" variant="ghost" size="sm" class="h-6 text-xs"
              @click="emit('replay', entry)"
            >
              <Play class="h-3 w-3" />重放
            </Button>
            <Button v-if="entry.status === 'completed'" variant="ghost" size="sm" class="h-6 text-xs" as-child>
              <a :href="recordingDownloadUrl(serial, entry.recording_id)"><Download class="h-3 w-3" />下载</a>
            </Button>
            <Button
              v-if="entry.status !== 'recording' && entry.status !== 'stopping'"
              variant="ghost" size="sm" class="h-6 text-xs text-destructive" @click="emit('delete', entry)"
            >
              <Trash2 class="h-3 w-3" />删除
            </Button>
          </div>
        </li>
      </ul>
    </div>
  </div>
</template>
