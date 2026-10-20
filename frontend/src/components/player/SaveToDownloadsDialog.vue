<script setup lang="ts">
import { computed, ref, watch } from 'vue'
import {
  AlertCircle,
  Download,
  FolderDown,
  Loader2,
} from 'lucide-vue-next'
import { toast } from 'vue-sonner'
import { Button } from '@/components/ui/button'
import {
  Dialog,
  DialogContent,
  DialogDescription,
  DialogFooter,
  DialogHeader,
  DialogTitle,
} from '@/components/ui/dialog'
import { Progress } from '@/components/ui/progress'
import { uploadFile } from '@/api/files'
import { formatFileSize } from '@/composables/useDeviceFileDrop'

const props = defineProps<{
  open: boolean
  file: File | null
  serial: string
}>()

const emit = defineEmits<{
  'update:open': [value: boolean]
  'saved': [filename: string]
}>()

type Phase = 'confirm' | 'progress' | 'error'
const phase = ref<Phase>('confirm')
const progress = ref(0)
const statusMessage = ref('')
const errorMessage = ref('')

let cancelUpload: (() => void) | null = null

const formattedSize = computed(() => {
  if (!props.file) return '0 B'
  return formatFileSize(props.file.size)
})

watch(() => props.open, (val) => {
  if (val) {
    phase.value = 'confirm'
    progress.value = 0
    statusMessage.value = ''
    errorMessage.value = ''
  }
  else {
    if (cancelUpload) {
      cancelUpload()
      cancelUpload = null
    }
  }
})

function close() {
  if (cancelUpload) {
    cancelUpload()
    cancelUpload = null
  }
  emit('update:open', false)
}

function handleSave() {
  if (!props.file || !props.serial) return

  phase.value = 'progress'
  progress.value = 0
  statusMessage.value = '正在保存到 Downloads...'
  errorMessage.value = ''

  cancelUpload = uploadFile(
    props.serial,
    'Download',
    props.file,
    'rename',
    (pct) => {
      progress.value = pct
      statusMessage.value = `正在保存到 Downloads (${pct}%)...`
    },
    (res) => {
      cancelUpload = null
      if (res.ok) {
        toast.success(`文件已保存到 Downloads: ${props.file?.name}`)
        emit('saved', props.file?.name || '')
        emit('update:open', false)
      }
      else {
        phase.value = 'error'
        errorMessage.value = res.message || '保存失败'
      }
    },
  )
}
</script>

<template>
  <Dialog :open="open" @update:open="close">
    <DialogContent class="sm:max-w-md">
      <DialogHeader>
        <DialogTitle class="flex items-center gap-2 text-base">
          <FolderDown class="h-5 w-5 text-primary" />
          <span>保存文件到设备</span>
        </DialogTitle>
        <DialogDescription class="text-xs text-muted-foreground font-mono">
          {{ serial }}
        </DialogDescription>
      </DialogHeader>

      <!-- 阶段 1：确认对话框 -->
      <div v-if="phase === 'confirm'" class="space-y-4 py-2">
        <div class="flex items-start gap-3 rounded-lg border bg-muted/30 p-3.5">
          <div class="flex h-10 w-10 shrink-0 items-center justify-center rounded-md bg-primary/10 text-primary">
            <Download class="h-5 w-5" />
          </div>
          <div class="min-w-0 flex-1 space-y-1">
            <p class="truncate text-sm font-medium" :title="file?.name">{{ file?.name }}</p>
            <p class="text-xs text-muted-foreground">{{ formattedSize }}</p>
          </div>
        </div>

        <p class="text-xs text-muted-foreground">
          是否将此文件保存到设备的 Downloads 目录（<code class="text-xs text-foreground">/storage/emulated/0/Download</code>）？
        </p>
      </div>

      <!-- 阶段 2：上传保存进度条 -->
      <div v-else-if="phase === 'progress'" class="space-y-4 py-4 text-center">
        <div class="flex justify-center">
          <Loader2 class="h-8 w-8 animate-spin text-primary" />
        </div>
        <div class="space-y-2">
          <p class="text-sm font-medium">{{ statusMessage }}</p>
          <Progress :model-value="progress" class="h-2 w-full" />
          <p class="text-xs text-muted-foreground font-mono">{{ progress }}%</p>
        </div>
      </div>

      <!-- 阶段 3：失败 -->
      <div v-else-if="phase === 'error'" class="space-y-3 py-3 text-center">
        <div class="flex justify-center">
          <div class="flex h-12 w-12 items-center justify-center rounded-full bg-destructive/10 text-destructive">
            <AlertCircle class="h-7 w-7" />
          </div>
        </div>
        <div class="space-y-1">
          <p class="text-sm font-medium text-destructive">保存失败</p>
          <p class="text-xs text-muted-foreground px-2">{{ errorMessage }}</p>
        </div>
      </div>

      <DialogFooter class="gap-2 sm:gap-0">
        <template v-if="phase === 'confirm'">
          <Button variant="outline" size="sm" @click="close">取消</Button>
          <Button size="sm" @click="handleSave">确认保存</Button>
        </template>
        <template v-else-if="phase === 'progress'">
          <Button variant="outline" size="sm" @click="close">取消并中断</Button>
        </template>
        <template v-else-if="phase === 'error'">
          <Button variant="outline" size="sm" @click="close">关闭</Button>
          <Button size="sm" @click="handleSave">重试</Button>
        </template>
      </DialogFooter>
    </DialogContent>
  </Dialog>
</template>
