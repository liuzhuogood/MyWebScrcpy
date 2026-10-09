<script setup lang="ts">
import { Camera, Download, Eye, Trash2 } from 'lucide-vue-next'
import { ref } from 'vue'
import { toast } from 'vue-sonner'
import { Button } from '@/components/ui/button'
import {
  Dialog,
  DialogContent,
  DialogHeader,
  DialogTitle,
} from '@/components/ui/dialog'
import { Empty, EmptyDescription, EmptyHeader, EmptyTitle } from '@/components/ui/empty'
import { Skeleton } from '@/components/ui/skeleton'
import {
  canvasToBlob,
  deleteScreenshot,
  listScreenshots,
  screenshotDownloadUrl,
  screenshotViewUrl,
  uploadScreenshot,
  type ScreenshotEntry,
} from '@/api/screenshots'

const props = defineProps<{
  serial: string
  getCanvas: () => HTMLCanvasElement | null
}>()

const list = ref<ScreenshotEntry[]>([])
const loading = ref(true)
const loadError = ref('')
const busy = ref(false)
const viewItem = ref<ScreenshotEntry | null>(null)

async function load() {
  loading.value = true
  try {
    const data = await listScreenshots(props.serial)
    list.value = data.screenshots || []
    loadError.value = ''
  }
  catch {
    loadError.value = '暂时无法获取截图记录'
  }
  finally {
    loading.value = false
  }
}

function fmtTime(s: string): string {
  try {
    return new Intl.DateTimeFormat('zh-CN', { dateStyle: 'short', timeStyle: 'medium' }).format(new Date(s))
  }
  catch {
    return s || ''
  }
}

/** 点击即截取当前画面并上传 */
async function capture() {
  const canvas = props.getCanvas()
  if (!canvas?.width || !canvas.height) {
    toast.error('画面尚未准备好，暂时无法截图')
    return
  }
  busy.value = true
  try {
    const blob = await canvasToBlob(canvas)
    if (!blob) {
      toast.error('截图生成失败，请重试')
      return
    }
    await uploadScreenshot(props.serial, blob)
    toast.success('截图已保存')
    await load()
  }
  catch (e) {
    const status = (e as Error & { status?: number }).status
    if (status === 404 || status === 501)
      toast.error('截图接口暂未就绪，请稍后重试')
    else toast.error('截图上传失败，请重试')
  }
  finally {
    busy.value = false
  }
}

async function remove(item: ScreenshotEntry) {
  try {
    await deleteScreenshot(props.serial, item.screenshot_id)
    toast.success('已删除截图')
    await load()
  }
  catch {
    toast.error('删除截图失败，请重试')
  }
}

void load()

defineExpose({ capture, load })
</script>

<template>
  <div class="w-80 space-y-2 p-1">
    <Button class="w-full" :disabled="busy" @click="capture()">
      <Camera class="h-4 w-4" />{{ busy ? '截图上传中…' : '截取当前画面' }}
    </Button>
    <div v-if="loading" class="space-y-2">
      <Skeleton v-for="i in 3" :key="i" class="h-14 w-full rounded-md" />
    </div>
    <p v-else-if="loadError" class="text-xs text-destructive">{{ loadError }}</p>
    <Empty v-else-if="list.length === 0">
      <EmptyHeader>
        <EmptyTitle class="text-sm">暂无截图</EmptyTitle>
        <EmptyDescription>点击上方按钮保存当前画面</EmptyDescription>
      </EmptyHeader>
    </Empty>
    <ul v-else class="max-h-80 space-y-2 overflow-auto">
      <li v-for="s in list" :key="s.screenshot_id" class="flex items-center gap-2 rounded-md border p-2">
        <img
          :src="screenshotViewUrl(serial, s.screenshot_id)" alt="截图缩略图" loading="lazy"
          class="h-10 w-10 shrink-0 rounded object-cover"
        />
        <div class="min-w-0 flex-1">
          <p class="text-xs font-medium">{{ fmtTime(s.created_at) }}</p>
          <p class="font-mono text-[11px] text-muted-foreground">{{ s.width || '?' }} × {{ s.height || '?' }}</p>
        </div>
        <Button variant="ghost" size="icon" class="h-7 w-7" title="查看大图" :aria-label="`查看截图大图`" @click="viewItem = s">
          <Eye class="h-3.5 w-3.5" />
        </Button>
        <Button variant="ghost" size="icon" class="h-7 w-7" title="下载" :aria-label="`下载截图`" as-child>
          <a :href="screenshotDownloadUrl(serial, s.screenshot_id)"><Download class="h-3.5 w-3.5" /></a>
        </Button>
        <Button variant="ghost" size="icon" class="h-7 w-7 text-destructive" title="删除截图" aria-label="删除截图" @click="remove(s)">
          <Trash2 class="h-3.5 w-3.5" />
        </Button>
      </li>
    </ul>

    <Dialog :open="viewItem !== null" @update:open="(v) => { if (!v) viewItem = null }">
      <DialogContent class="max-w-3xl">
        <DialogHeader>
          <DialogTitle>{{ viewItem ? fmtTime(viewItem.created_at) : '' }}</DialogTitle>
        </DialogHeader>
        <img
          v-if="viewItem" :src="screenshotViewUrl(serial, viewItem.screenshot_id)" alt="截图大图"
          class="mx-auto max-h-[70vh] max-w-full rounded-md"
        />
      </DialogContent>
    </Dialog>
  </div>
</template>
