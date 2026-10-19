<script setup lang="ts">
import { computed, ref, watch } from 'vue'
import {
  AlertCircle,
  CheckCircle2,
  FileBox,
  HardDriveUpload,
  Loader2,
  Play,
} from 'lucide-vue-next'
import { Button } from '@/components/ui/button'
import { Checkbox } from '@/components/ui/checkbox'
import {
  Dialog,
  DialogContent,
  DialogDescription,
  DialogFooter,
  DialogHeader,
  DialogTitle,
} from '@/components/ui/dialog'
import { Progress } from '@/components/ui/progress'
import type { DragFileCandidate } from './types'

const props = defineProps<{
  open: boolean
  file: DragFileCandidate | null
  serial: string
  deviceModel: string
}>()

const emit = defineEmits<{
  'update:open': [value: boolean]
  'installed': [appInfo: { name: string, packageName: string, sizeBytes: number }]
  'launch': [packageName: string]
}>()

type InstallPhase = 'confirm' | 'installing' | 'success' | 'failed'
const phase = ref<InstallPhase>('confirm')
const progress = ref(0)
const statusMessage = ref('')
const errorMessage = ref('')

// 安装选项
const replaceExisting = ref(true)
const grantPermissions = ref(true)
const allowDowngrade = ref(false)

const formattedSize = computed(() => {
  if (!props.file) return '0 B'
  const mb = props.file.size / (1024 * 1024)
  if (mb >= 1) {
    return `${mb.toFixed(1)} MB`
  }
  return `${(props.file.size / 1024).toFixed(1)} KB`
})

// 推测应用信息（真实场景在上传解析后或通过 APK Manifest 解析）
const inferredApp = computed(() => {
  if (!props.file) return { name: '未知应用', packageName: 'com.example.app' }
  const lower = props.file.name.toLowerCase()
  if (lower.includes('weixin') || lower.includes('wechat')) {
    return { name: '微信', packageName: 'com.tencent.mm' }
  }
  if (lower.includes('bili')) {
    return { name: '哔哩哔哩', packageName: 'tv.danmaku.bili' }
  }
  if (lower.includes('alipay')) {
    return { name: '支付宝', packageName: 'com.eg.android.AlipayGphone' }
  }
  if (lower.includes('jianying') || lower.includes('capcut')) {
    return { name: '剪映专业版', packageName: 'com.lemon.lv' }
  }
  const cleanName = props.file.name.replace(/\.apk$/i, '').replace(/[-_v\d.].*$/, '') || '自定义应用'
  return {
    name: cleanName.charAt(0).toUpperCase() + cleanName.slice(1),
    packageName: `com.app.${cleanName.toLowerCase()}`,
  }
})

watch(() => props.open, (val) => {
  if (val) {
    phase.value = 'confirm'
    progress.value = 0
    statusMessage.value = ''
    errorMessage.value = ''
  }
})

let timer: ReturnType<typeof setInterval> | null = null

function cancel() {
  if (timer) {
    clearInterval(timer)
    timer = null
  }
  emit('update:open', false)
}

function startInstall() {
  phase.value = 'installing'
  progress.value = 15
  statusMessage.value = '正在将 APK 传输至设备临时目录...'

  let cur = 15
  timer = setInterval(() => {
    cur += 15
    if (cur <= 60) {
      progress.value = cur
      statusMessage.value = `正在向设备传输 APK (${cur}%)...`
    }
    else if (cur <= 90) {
      progress.value = cur
      statusMessage.value = '设备端正在执行 ADB 包管理器验证与安装...'
    }
    else {
      if (timer) clearInterval(timer)
      timer = null
      progress.value = 100
      statusMessage.value = '安装完成'
      phase.value = 'success'
      emit('installed', {
        name: inferredApp.value.name,
        packageName: inferredApp.value.packageName,
        sizeBytes: props.file?.size || 35 * 1024 * 1024,
      })
    }
  }, 350)
}

function onLaunch() {
  emit('launch', inferredApp.value.packageName)
  emit('update:open', false)
}
</script>

<template>
  <Dialog :open="open" @update:open="$emit('update:open', $event)">
    <DialogContent class="sm:max-w-md">
      <DialogHeader>
        <DialogTitle class="flex items-center gap-2 text-base">
          <FileBox class="size-5 text-primary" />
          <span>{{ phase === 'success' ? '安装完成' : '安装应用' }}</span>
        </DialogTitle>
        <DialogDescription class="text-xs text-muted-foreground">
          {{ deviceModel }} ({{ serial }})
        </DialogDescription>
      </DialogHeader>

      <!-- 阶段 1：确认对话框 -->
      <div v-if="phase === 'confirm'" class="space-y-4 py-2">
        <div class="flex items-start gap-3 rounded-lg border bg-muted/30 p-3.5">
          <div class="flex size-10 shrink-0 items-center justify-center rounded-md bg-primary/10 text-primary">
            <HardDriveUpload class="size-5" />
          </div>
          <div class="min-w-0 flex-1 space-y-1">
            <p class="truncate text-sm font-medium">{{ file?.name }}</p>
            <p class="text-xs text-muted-foreground">{{ formattedSize }}</p>
          </div>
        </div>

        <div class="space-y-2.5 rounded-lg border p-3 text-sm">
          <label class="flex items-center gap-2 cursor-pointer text-xs">
            <Checkbox :checked="replaceExisting" @update:checked="replaceExisting = $event" />
            <span>保留数据覆盖安装 (<code class="text-xs text-muted-foreground">-r</code>)</span>
          </label>
          <label class="flex items-center gap-2 cursor-pointer text-xs">
            <Checkbox :checked="grantPermissions" @update:checked="grantPermissions = $event" />
            <span>自动授予运行时权限 (<code class="text-xs text-muted-foreground">-g</code>)</span>
          </label>
          <label class="flex items-center gap-2 cursor-pointer text-xs text-muted-foreground">
            <Checkbox :checked="allowDowngrade" @update:checked="allowDowngrade = $event" />
            <span>允许版本降级 (<code class="text-xs text-muted-foreground">-d</code>)</span>
          </label>
        </div>
      </div>

      <!-- 阶段 2：安装中进度条 -->
      <div v-else-if="phase === 'installing'" class="space-y-4 py-4 text-center">
        <div class="flex justify-center">
          <Loader2 class="size-9 animate-spin text-primary" />
        </div>
        <div class="space-y-2">
          <p class="text-sm font-medium">{{ statusMessage }}</p>
          <Progress :model-value="progress" class="h-2 w-full" />
          <p class="text-xs text-muted-foreground font-mono">{{ progress }}%</p>
        </div>
      </div>

      <!-- 阶段 3：成功完成 -->
      <div v-else-if="phase === 'success'" class="space-y-4 py-3 text-center">
        <div class="flex justify-center">
          <div class="flex size-12 items-center justify-center rounded-full bg-emerald-100 text-emerald-600 dark:bg-emerald-950/60 dark:text-emerald-400">
            <CheckCircle2 class="size-7" />
          </div>
        </div>
        <div class="space-y-1">
          <p class="text-sm font-medium">{{ inferredApp.name }}</p>
          <p class="text-xs font-mono text-muted-foreground">{{ inferredApp.packageName }}</p>
        </div>
      </div>

      <!-- 阶段 4：失败 -->
      <div v-else-if="phase === 'failed'" class="space-y-4 py-3 text-center">
        <div class="flex justify-center">
          <div class="flex size-12 items-center justify-center rounded-full bg-destructive/10 text-destructive">
            <AlertCircle class="size-7" />
          </div>
        </div>
        <div class="space-y-1">
          <p class="text-sm font-medium">安装失败</p>
          <p class="text-xs text-destructive">{{ errorMessage || 'INSTALL_FAILED_INVALID_APK: 解析 APK 失败' }}</p>
        </div>
      </div>

      <DialogFooter class="gap-2 sm:gap-0">
        <template v-if="phase === 'confirm'">
          <Button variant="outline" size="sm" @click="cancel">取消</Button>
          <Button size="sm" @click="startInstall">确认安装</Button>
        </template>
        <template v-else-if="phase === 'installing'">
          <Button variant="outline" size="sm" disabled>安装中，请稍候...</Button>
        </template>
        <template v-else-if="phase === 'success'">
          <Button variant="outline" size="sm" @click="$emit('update:open', false)">关闭</Button>
          <Button size="sm" class="gap-1" @click="onLaunch">
            <Play class="size-3.5" />
            <span>立即启动应用</span>
          </Button>
        </template>
        <template v-else-if="phase === 'failed'">
          <Button variant="outline" size="sm" @click="cancel">关闭</Button>
          <Button size="sm" @click="startInstall">重试</Button>
        </template>
      </DialogFooter>
    </DialogContent>
  </Dialog>
</template>
