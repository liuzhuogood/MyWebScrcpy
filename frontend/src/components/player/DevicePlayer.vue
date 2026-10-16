<script setup lang="ts">
import {
  GalleryHorizontal,
  ArrowLeft,
  Camera,
  Circle,
  Crop,
  Ellipsis,
  FolderOpen,
  Home,
  LayoutTemplate,
  Loader2,
  Maximize,
  Minimize,
  MonitorOff,
  PictureInPicture2,
  Play,
  Power,
  RotateCw,
  Scaling,
  ScanSearch,
  Settings,
  Square,
  TriangleAlert,
  Terminal,
  Undo2,
  Volume2,
  VolumeX,
  X,
  ChevronDown,
  Pin,
  PinOff,
} from 'lucide-vue-next'
import { computed, nextTick, onMounted, onUnmounted, ref, useId, watch } from 'vue'
import { invoke, isTauri } from '@tauri-apps/api/core'
import { getCurrentWindow } from '@tauri-apps/api/window'
import { toast } from 'vue-sonner'
import { useElementSize } from '@vueuse/core'
import { RouterLink } from 'vue-router'
import {
  AlertDialog,
  AlertDialogAction,
  AlertDialogCancel,
  AlertDialogContent,
  AlertDialogDescription,
  AlertDialogFooter,
  AlertDialogHeader,
  AlertDialogTitle,
  AlertDialogTrigger,
} from '@/components/ui/alert-dialog'
import { Badge } from '@/components/ui/badge'
import { Button } from '@/components/ui/button'
import {
  DropdownMenu,
  DropdownMenuContent,
  DropdownMenuItem,
  DropdownMenuLabel,
  DropdownMenuSeparator,
  DropdownMenuTrigger,
} from '@/components/ui/dropdown-menu'
import { Dialog, DialogContent, DialogHeader, DialogTitle } from '@/components/ui/dialog'
import { Empty, EmptyDescription, EmptyHeader, EmptyMedia, EmptyTitle } from '@/components/ui/empty'
import { Popover, PopoverContent, PopoverTrigger } from '@/components/ui/popover'
import { Slider } from '@/components/ui/slider'
import { inspectionMode, setInspector } from '@/inspector/guard'
import CaptureLayer from '@/capture/CaptureLayer.vue'
import InspectorPanel from '@/inspector/InspectorPanel.vue'
import { provideInspector } from '@/inspector/useInspector'
import { recordOp } from '@/inspector/opLog'
import RecordingPanel from '@/replay/RecordingPanel.vue'
import ReplayPanel from '@/replay/ReplayPanel.vue'
import ScreenshotsPanel from '@/replay/ScreenshotsPanel.vue'
import { useRecordings } from '@/replay/useRecordings'
import { useReplay } from '@/replay/useReplay'
import { usePlayer } from '@/scrcpy/usePlayer'
import TouchVisualizer from '@/scrcpy/TouchVisualizer.vue'
import AdbShellPanel from '@/scrcpy/AdbShellPanel.vue'
import SplitLayout from '@/components/layout/SplitLayout.vue'
import FilesView from '@/views/FilesView.vue'
import PlayerSettingsPanel from '@/components/player/PlayerSettingsPanel.vue'
import WindowEffectsControl from '@/components/player/WindowEffectsControl.vue'
import { usePlayerWindows } from '@/composables/usePlayerWindows'
import TemplatePanel from '@/template/TemplatePanel.vue'
import { useTemplateMatches } from '@/template/useTemplateMatches'
import { useDesktopConfig } from '@/composables/useDesktopConfig'
import type { RecordingEntry } from '@/api/recordings'

const props = withDefaults(defineProps<{ serial: string, embedded?: boolean }>(), { embedded: false })
const emit = defineEmits<{
  close: []
  layout: [layout: { width: number, height: number, side: 'capture' | 'tools' | null }]
}>()
const serial = computed(() => props.serial)
const windowEffects = usePlayerWindows(serial)
const nativePlayerWindow = isTauri() && getCurrentWindow().label.startsWith('player-')
const desktopConfig = useDesktopConfig()
const isGui = computed(() => {
  return desktopConfig.isDesktop.value || Boolean(
    (window as any).__MYWEBSCRCPY_DESKTOP_TOKEN__ ||
    (window as any).__TAURI_INTERNALS__ ||
    (window as any).__TAURI__
  )
})
const captureHostId = `capture-actions-${useId()}`

const p = usePlayer(serial.value, {
  onOp: (type, data) => {
    recordOp(serial.value, type, data)
  },
  onTouch: event => touchVisualizerRef.value?.handle(event),
  onFrame: () => ins.ocr.frameChanged(),
  onDisconnect: () => {
    ins.ocr.invalidate()
    if (props.embedded && nativePlayerWindow)
      void windowEffects.restore()
  },
})
const showPip = computed(() => !isGui.value && p.pipSupported.value)
const ins = provideInspector(serial.value, () => p.canvasRef.value, p.frameVersion, () => p.live.value && p.frameVersion() > 0)
const inspectorVisible = ref(false)
const templateOpen = ref(false)
const tm = useTemplateMatches(serial.value, () => p.canvasRef.value)

// 录制 / 重放共享控制器
const replay = useReplay(serial.value, msg => p.switchStream(msg))
const rec = useRecordings(serial.value, () => replay.replaying.value)
const replayOpen = ref(false)
const captureOpen = ref(false)
const shellOpen = ref(false)
const filesOpen = ref(false)
const settingsOpen = ref(false)
const sizeOpen = ref(false)
const mobileTool = ref<'recording' | 'screenshots' | null>(null)
const touchVisible = ref(localStorage.getItem('show-touch-overlay') !== 'false')
const touchVisualizerRef = ref<InstanceType<typeof TouchVisualizer> | null>(null)
const deleteRecordingTarget = ref<RecordingEntry | null>(null)
const { width: stageWidth } = useElementSize(p.stageRef)
const compactToolbar = computed(() => stageWidth.value < 1280)
const isVeryNarrow = computed(() => stageWidth.value < 270)
const sidePanel = computed(() => captureOpen.value ? 'capture' : filesOpen.value || shellOpen.value || inspectorVisible.value || replayOpen.value || templateOpen.value || settingsOpen.value ? 'tools' : null)

const localToolbarVisible = ref(true)
const toolbarVisible = computed(() => nativePlayerWindow ? (windowEffects.player.value?.toolbarVisible ?? true) : localToolbarVisible.value)
const isAlwaysOnTop = ref(false)

async function dragBorderlessWindow(event: MouseEvent) {
  if (!nativePlayerWindow || !windowEffects.player.value?.borderless || event.button !== 0) return
  if ((event.target as HTMLElement).closest('button, a, input, [role="slider"]')) return
  try {
    await getCurrentWindow().startDragging()
  }
  catch (error) {
    console.warn('[Desktop] 拖动无边框小窗失败', error)
  }
}

async function setToolbarVisible(visible: boolean) {
  if (nativePlayerWindow)
    await windowEffects.setToolbarVisible(visible)
  else
    localToolbarVisible.value = visible
}

async function initAlwaysOnTop() {
  if (isGui.value && isTauri()) {
    try {
      isAlwaysOnTop.value = await invoke<boolean>('is_window_always_on_top')
    }
    catch (e) {
      console.warn('获取置顶状态失败', e)
    }
  }
}

async function toggleAlwaysOnTop() {
  if (isGui.value && isTauri()) {
    try {
      const next = !isAlwaysOnTop.value
      await invoke('set_window_always_on_top', { alwaysOnTop: next })
      isAlwaysOnTop.value = next
    }
    catch (e) {
      console.warn('切换置顶状态失败', e)
    }
  }
}

watch(toolbarVisible, async visible => {
  if (!visible) {
    const shortcut = windowEffects.shortcut.value
    toast.info('顶栏已隐藏', {
      description: nativePlayerWindow
        ? `${shortcut ? `按 ${shortcut}，或` : ''}在菜单“视图 → 恢复所有小窗”中恢复；也可点击窗口顶部把手。`
        : '点击窗口顶部把手可重新显示。',
      duration: 8000,
    })
  }
  await nextTick()
  window.dispatchEvent(new Event('resize'))
})

watch([p.videoSize, sidePanel], ([size, side]) => {
  emit('layout', { width: size.w, height: size.h, side })
}, { immediate: true })

onMounted(() => {
  if (serial.value) {
    tm.start()
    void rec.loadHistory()
  }
  if (props.embedded && isGui.value) {
    void initAlwaysOnTop()
  }
})

onUnmounted(() => {
  if (inspectorVisible.value)
    setInspector(false)
})

function closeSidePanels() {
  filesOpen.value = false
  shellOpen.value = false
  if (inspectorVisible.value) {
    inspectorVisible.value = false
    setInspector(false)
    ins.closeInspector()
  }
  if (replayOpen.value) {
    replayOpen.value = false
    replay.panelOpen.value = false
  }
  templateOpen.value = false
  settingsOpen.value = false
}

function openReplayPanel() {
  closeSidePanels()
  replayOpen.value = true
  replay.panelOpen.value = true
  void replay.restore()
  void replay.loadList()
}

function toggleReplayPanel() {
  if (replayOpen.value) {
    replayOpen.value = false
    replay.panelOpen.value = false
  }
  else {
    openReplayPanel()
  }
}

async function startReplayFromRecording(entry: RecordingEntry) {
  openReplayPanel()
  await replay.start(entry.recording_id, entry)
}

async function confirmDeleteRecording() {
  if (!deleteRecordingTarget.value)
    return
  try {
    await rec.remove(deleteRecordingTarget.value.recording_id)
    await replay.loadList()
  }
  catch {
    // 错误由面板消息呈现
  }
  finally {
    deleteRecordingTarget.value = null
  }
}

function toggleInspector() {
  if (inspectorVisible.value) {
    inspectorVisible.value = false
    setInspector(false)
    ins.closeInspector()
    return
  }
  closeSidePanels()
  inspectorVisible.value = true
  setInspector(true)
}

function openShell() {
  if (shellOpen.value) {
    shellOpen.value = false
    return
  }
  closeSidePanels()
  shellOpen.value = true
}

function openFiles() {
  if (filesOpen.value) {
    filesOpen.value = false
    return
  }
  closeSidePanels()
  filesOpen.value = true
}

function openTemplate() {
  if (templateOpen.value) {
    templateOpen.value = false
    return
  }
  closeSidePanels()
  templateOpen.value = true
}

function setTouchVisible(on: boolean) {
  touchVisible.value = on
  localStorage.setItem('show-touch-overlay', String(on))
}

function openSettings() {
  if (settingsOpen.value) {
    settingsOpen.value = false
    return
  }
  closeSidePanels()
  settingsOpen.value = true
}

const scaleModel = computed({
  get: () => [p.displayScale.value ?? 100],
  set: (v: number[]) => p.setDisplayScale(v[0]),
})

async function onReboot() {
  try {
    await p.reboot()
  }
  catch (e) {
    p.statusText.value = e instanceof Error ? e.message : '重启请求失败'
  }
}

function handleEscape(event: KeyboardEvent) {
  if (captureOpen.value) {
    event.preventDefault()
    captureOpen.value = false
  }
}

defineExpose({ handleEscape })
</script>

<template>
  <div v-if="!serial" class="flex min-h-screen items-center justify-center">
    <Empty>
      <EmptyHeader>
        <EmptyMedia variant="icon"><TriangleAlert /></EmptyMedia>
        <EmptyTitle>缺少设备 serial</EmptyTitle>
        <EmptyDescription>请从首页设备列表进入投屏。</EmptyDescription>
      </EmptyHeader>
      <Button as-child><RouterLink to="/">返回首页</RouterLink></Button>
    </Empty>
  </div>

  <div v-else :ref="p.stageRef" class="relative flex min-h-0 min-w-0 flex-col bg-background" :class="embedded ? 'h-full overflow-hidden' : 'h-dvh'">
    <!-- 工具栏隐藏时的顶部唤出把手 -->
    <div
      v-if="!toolbarVisible"
      class="group absolute top-0 inset-x-0 z-50 flex h-6 items-start justify-center"
    >
      <button
        type="button"
        class="flex h-4 items-center justify-center rounded-b bg-muted/80 px-4 text-muted-foreground shadow-sm backdrop-blur transition-all hover:h-5 hover:bg-muted hover:text-foreground cursor-pointer opacity-40 hover:opacity-100"
        aria-label="展开工具条"
        title="展开工具条"
        @click="setToolbarVisible(true)"
      >
        <ChevronDown class="h-3 w-3" />
      </button>
    </div>

    <!-- 工具栏 -->
    <header v-if="toolbarVisible" class="flex h-14 shrink-0 items-center gap-1 border-b px-3" :class="{ 'toolbar-compact': compactToolbar, 'cursor-move select-none': embedded }" @mousedown="dragBorderlessWindow">
      <Button
        v-if="embedded"
        variant="ghost"
        size="icon"
        :aria-label="isGui ? '隐藏工具条' : '关闭小窗'"
        :title="isGui ? '隐藏工具条' : '关闭小窗'"
        class="cursor-pointer"
        @click="isGui ? setToolbarVisible(false) : emit('close')"
      >
        <X class="h-4 w-4" />
      </Button>
      <Button v-else variant="ghost" size="icon" aria-label="返回设备列表" as-child>
        <RouterLink to="/"><ArrowLeft class="h-5 w-5" /></RouterLink>
      </Button>
      <span class="ml-1 min-w-0 max-w-44 flex-1 truncate font-mono text-sm font-medium" :class="{ 'cursor-move select-none': embedded }" :title="serial">{{ serial }}</span>
      <Badge v-if="!embedded" :variant="p.live.value ? 'default' : 'secondary'">{{ p.statusText.value }}</Badge>
      <Badge v-if="!embedded && p.showFps.value" class="toolbar-secondary" variant="outline">{{ p.fps.value }} fps</Badge>

      <!-- 小窗模式专属精简工具栏：默认只显示 置顶(GUI)、返回、主页、切换 + 更多；过窄时全部收入更多 -->
      <div v-if="embedded" class="ml-auto flex shrink-0 items-center gap-0.5">
        <WindowEffectsControl
          v-if="nativePlayerWindow"
          :opacity="windowEffects.player.value?.opacity ?? 100"
          :click-through="windowEffects.player.value?.clickThrough ?? false"
          :borderless="windowEffects.player.value?.borderless ?? false"
          :shortcut="windowEffects.shortcut.value"
          :busy="windowEffects.busy.value"
          @opacity="windowEffects.setOpacity"
          @click-through="windowEffects.setClickThrough"
          @borderless="windowEffects.setBorderless"
          @restore="windowEffects.restore"
        />
        <Button
          v-if="isGui"
          variant="ghost"
          size="icon"
          :title="isAlwaysOnTop ? '取消置顶' : '保持窗口置顶'"
          :aria-label="isAlwaysOnTop ? '取消置顶' : '保持窗口置顶'"
          :class="{ 'text-primary bg-primary/10': isAlwaysOnTop }"
          @click="toggleAlwaysOnTop()"
        >
          <PinOff v-if="isAlwaysOnTop" class="h-4 w-4" />
          <Pin v-else class="h-4 w-4" />
        </Button>
        <template v-if="!isVeryNarrow">
          <Button variant="ghost" size="icon" title="返回" aria-label="返回" @click="p.pressBack()">
            <Undo2 class="h-4 w-4" />
          </Button>
          <Button variant="ghost" size="icon" title="主页" aria-label="主页" @click="p.pressHome()">
            <Home class="h-4 w-4" />
          </Button>
          <Button variant="ghost" size="icon" title="切换任务" aria-label="切换任务" @click="p.pressRecents()">
            <GalleryHorizontal class="h-4 w-4" />
          </Button>
        </template>
        <DropdownMenu :modal="false">
          <DropdownMenuTrigger as-child>
            <Button variant="ghost" size="icon" title="更多" aria-label="更多">
              <Ellipsis class="h-4 w-4" />
            </Button>
          </DropdownMenuTrigger>
          <DropdownMenuContent align="end" class="z-[9999]">
            <template v-if="isVeryNarrow">
              <DropdownMenuItem v-if="isGui" @select="toggleAlwaysOnTop()">
                <PinOff v-if="isAlwaysOnTop" class="h-4 w-4" />
                <Pin v-else class="h-4 w-4" />
                {{ isAlwaysOnTop ? '取消置顶' : '保持窗口置顶' }}
              </DropdownMenuItem>
              <DropdownMenuItem @select="p.pressBack()"><Undo2 class="h-4 w-4" />返回</DropdownMenuItem>
              <DropdownMenuItem @select="p.pressHome()"><Home class="h-4 w-4" />主页</DropdownMenuItem>
              <DropdownMenuItem @select="p.pressRecents()"><GalleryHorizontal class="h-4 w-4" />切换任务</DropdownMenuItem>
              <DropdownMenuSeparator />
            </template>
            <DropdownMenuItem :disabled="p.powerBusy.value" @select="p.pressPower()">
              <Power class="h-4 w-4" />电源键
            </DropdownMenuItem>
            <DropdownMenuItem @select="p.rotate()">
              <RotateCw class="h-4 w-4" />横竖屏
            </DropdownMenuItem>
            <DropdownMenuItem @select="p.toggleFullscreen()">
              <Minimize v-if="p.fullscreen.value" class="h-4 w-4" />
              <Maximize v-else class="h-4 w-4" />{{ p.fullscreen.value ? '退出全屏' : '全屏' }}
            </DropdownMenuItem>
            <DropdownMenuItem v-if="showPip" :disabled="!p.pipSupported.value" @select="p.togglePip()">
              <PictureInPicture2 class="h-4 w-4" />{{ p.pipActive.value ? '退出画中画' : '画中画' }}
            </DropdownMenuItem>
            <DropdownMenuItem @select="openSettings()"><Settings class="h-4 w-4" />设置</DropdownMenuItem>
          </DropdownMenuContent>
        </DropdownMenu>
      </div>

      <!-- 完整页面模式工具栏 -->
      <div v-else class="ml-auto flex shrink-0 items-center gap-1">
        <Button class="toolbar-secondary" variant="ghost" size="icon" title="返回" aria-label="返回" @click="p.pressBack()">
          <Undo2 class="h-5 w-5" />
        </Button>
        <Button variant="ghost" size="icon" title="主页" aria-label="主页" @click="p.pressHome()">
          <Home class="h-5 w-5" />
        </Button>
        <Button class="toolbar-secondary" variant="ghost" size="icon" title="切换任务" aria-label="切换任务" @click="p.pressRecents()">
          <GalleryHorizontal class="h-5 w-5" />
        </Button>
        <Button class="toolbar-secondary" variant="ghost" size="icon" title="电源键" aria-label="电源键" :disabled="p.powerBusy.value" @click="p.pressPower()">
          <Power class="h-5 w-5" />
        </Button>
        <Button class="toolbar-secondary" variant="ghost" size="icon" title="横竖屏" aria-label="横竖屏" @click="p.rotate()">
          <RotateCw class="h-5 w-5" />
        </Button>
        <Button
          class="toolbar-secondary"
          variant="ghost" size="icon"
          :title="p.audioTitle.value" :aria-label="p.audioTitle.value"
          :disabled="!p.audioReady.value" @click="p.toggleAudio()"
        >
          <VolumeX v-if="p.audioMuted.value" class="h-5 w-5" />
          <Volume2 v-else class="h-5 w-5" />
        </Button>
        <Button class="toolbar-optional" variant="ghost" size="icon" title="文件管理" aria-label="文件管理" @click="openFiles()">
          <FolderOpen class="h-5 w-5" />
        </Button>
        <Button class="toolbar-optional" variant="ghost" size="icon" title="ADB Shell" aria-label="ADB Shell" @click="openShell()">
          <Terminal class="h-5 w-5" />
        </Button>
        <Button
          v-if="rec.running.value"
          variant="ghost" size="icon" title="停止录制" aria-label="停止录制"
          class="toolbar-optional text-destructive" :disabled="rec.busy.value" @click="rec.stop()"
        >
          <Square class="h-5 w-5 fill-current" />
        </Button>
        <Popover v-else>
          <PopoverTrigger as-child>
            <Button class="toolbar-optional" variant="ghost" size="icon" title="录制" aria-label="录制">
              <Circle class="h-5 w-5 fill-current" />
            </Button>
          </PopoverTrigger>
          <PopoverContent align="end" class="w-auto p-3">
            <RecordingPanel
              :serial="serial"
              :minutes="rec.minutes.value" :record-audio="rec.recordAudio.value"
              :busy="rec.busy.value" :running="rec.running.value"
              :state-msg="rec.stateMsg.value" :history="rec.history.value" :history-error="rec.historyError.value"
              @update:minutes="rec.minutes.value = $event"
              @update:record-audio="rec.recordAudio.value = $event"
              @start="rec.start(rec.minutes.value, rec.recordAudio.value)"
              @stop="rec.stop()"
              @replay="startReplayFromRecording"
              @delete="deleteRecordingTarget = $event"
            />
          </PopoverContent>
        </Popover>
        <Button class="toolbar-optional"
          variant="ghost" size="icon" title="重放" aria-label="重放"
          :class="{ 'bg-accent': replayOpen || replay.replaying.value }"
          @click="toggleReplayPanel()"
        >
          <Play class="h-5 w-5" />
        </Button>
        <Popover>
          <PopoverTrigger as-child>
            <Button class="toolbar-optional" variant="ghost" size="icon" title="截图" aria-label="截图">
              <Camera class="h-5 w-5" />
            </Button>
          </PopoverTrigger>
          <PopoverContent align="end" class="w-auto p-3">
            <ScreenshotsPanel :serial="serial" :get-canvas="() => p.canvasRef.value" />
          </PopoverContent>
        </Popover>
        <Button class="toolbar-optional"
          variant="ghost" size="icon" title="区域截图" aria-label="区域截图"
          :disabled="!p.videoSize.value.w" @click="captureOpen = true"
        >
          <Crop class="h-5 w-5" />
        </Button>
        <Button class="toolbar-optional"
          variant="ghost" size="icon"
          :title="p.fullscreen.value ? '退出全屏' : '全屏'" :aria-label="p.fullscreen.value ? '退出全屏' : '全屏'"
          @click="p.toggleFullscreen()"
        >
          <Minimize v-if="p.fullscreen.value" class="h-5 w-5" />
          <Maximize v-else class="h-5 w-5" />
        </Button>

        <Button
          class="toolbar-secondary"
          variant="ghost" size="icon"
          title="检查画布与界面" aria-label="检查"
          :class="{ 'bg-accent': inspectorVisible }"
          @click="toggleInspector()"
        >
          <ScanSearch class="h-5 w-5" />
        </Button>
        <Button
          class="toolbar-secondary"
          variant="ghost" size="icon"
          title="模板管理" aria-label="模板管理"
          :class="{ 'bg-accent': templateOpen }"
          @click="openTemplate()"
        >
          <LayoutTemplate class="h-5 w-5" />
        </Button>
        <Button class="toolbar-optional" variant="ghost" size="icon" title="设置" aria-label="设置" :class="{ 'bg-accent': settingsOpen }" @click="openSettings()">
          <Settings class="h-5 w-5" />
        </Button>
        <DropdownMenu>
          <DropdownMenuTrigger as-child>
            <Button variant="ghost" size="icon" title="更多" aria-label="更多">
              <Ellipsis class="h-5 w-5" />
            </Button>
          </DropdownMenuTrigger>
          <DropdownMenuContent align="end">
            <DropdownMenuLabel v-if="compactToolbar && p.showFps.value" class="font-mono text-xs text-muted-foreground">{{ p.fps.value }} fps</DropdownMenuLabel>
            <DropdownMenuItem v-if="compactToolbar" @select="p.pressBack()"><Undo2 class="h-4 w-4" />返回</DropdownMenuItem>
            <DropdownMenuItem v-if="compactToolbar" @select="p.pressRecents()"><GalleryHorizontal class="h-4 w-4" />切换任务</DropdownMenuItem>
            <DropdownMenuItem v-if="compactToolbar" :disabled="p.powerBusy.value" @select="p.pressPower()"><Power class="h-4 w-4" />电源键</DropdownMenuItem>
            <DropdownMenuItem v-if="compactToolbar" @select="p.rotate()"><RotateCw class="h-4 w-4" />横竖屏</DropdownMenuItem>
            <DropdownMenuItem v-if="compactToolbar" :disabled="!p.audioReady.value" @select="p.toggleAudio()">
              <VolumeX v-if="p.audioMuted.value" class="h-4 w-4" /><Volume2 v-else class="h-4 w-4" />{{ p.audioTitle.value }}
            </DropdownMenuItem>
            <DropdownMenuSeparator v-if="compactToolbar" />
            <DropdownMenuItem v-if="compactToolbar && rec.running.value" :disabled="rec.busy.value" @select="rec.stop()"><Square class="h-4 w-4" />停止录制</DropdownMenuItem>
            <DropdownMenuItem v-else-if="compactToolbar" @select="mobileTool = 'recording'"><Circle class="h-4 w-4" />录制</DropdownMenuItem>
            <DropdownMenuItem v-if="compactToolbar" @select="toggleReplayPanel()"><Play class="h-4 w-4" />重放</DropdownMenuItem>
            <DropdownMenuItem v-if="compactToolbar" @select="mobileTool = 'screenshots'"><Camera class="h-4 w-4" />截图</DropdownMenuItem>
            <DropdownMenuItem v-if="compactToolbar" :disabled="!p.videoSize.value.w" @select="captureOpen = true"><Crop class="h-4 w-4" />区域截图</DropdownMenuItem>
            <DropdownMenuItem v-if="compactToolbar" @select="p.toggleFullscreen()">
              <Minimize v-if="p.fullscreen.value" class="h-4 w-4" /><Maximize v-else class="h-4 w-4" />{{ p.fullscreen.value ? '退出全屏' : '全屏' }}
            </DropdownMenuItem>
            <DropdownMenuItem v-if="compactToolbar" @select="toggleInspector()"><ScanSearch class="h-4 w-4" />检查</DropdownMenuItem>
            <DropdownMenuSeparator v-if="compactToolbar" />
            <DropdownMenuItem @select="openFiles()"><FolderOpen class="h-4 w-4" />文件管理</DropdownMenuItem>
            <DropdownMenuItem @select="openTemplate()"><LayoutTemplate class="h-4 w-4" />模板管理</DropdownMenuItem>
            <DropdownMenuItem @select="openShell()"><Terminal class="h-4 w-4" />ADB Shell</DropdownMenuItem>
            <DropdownMenuItem :disabled="!p.pipSupported.value" @select="p.togglePip()">
              <PictureInPicture2 class="h-4 w-4" />{{ p.pipActive.value ? '退出画中画' : '画中画' }}
            </DropdownMenuItem>
            <DropdownMenuItem @select="sizeOpen = true">
              <Scaling class="h-4 w-4" />显示尺寸
            </DropdownMenuItem>
            <DropdownMenuItem @select="openSettings()"><Settings class="h-4 w-4" />设置</DropdownMenuItem>
            <DropdownMenuSeparator />
            <AlertDialog>
              <AlertDialogTrigger as-child>
                <div
                  class="relative flex cursor-pointer select-none items-center gap-2 rounded-sm px-2 py-1.5 text-sm text-destructive outline-none hover:bg-accent focus:bg-accent"
                  role="menuitem"
                >
                  <Power class="h-4 w-4" />重启设备
                </div>
              </AlertDialogTrigger>
              <AlertDialogContent>
                <AlertDialogHeader>
                  <AlertDialogTitle>重启这台设备？</AlertDialogTitle>
                  <AlertDialogDescription>
                    {{ serial }} 将立即重启，当前投屏连接会中断并自动重连。
                  </AlertDialogDescription>
                </AlertDialogHeader>
                <AlertDialogFooter>
                  <AlertDialogCancel>取消</AlertDialogCancel>
                  <AlertDialogAction :disabled="p.rebooting.value" @click="onReboot">
                    {{ p.rebooting.value ? '正在重启…' : '确认重启' }}
                  </AlertDialogAction>
                </AlertDialogFooter>
              </AlertDialogContent>
            </AlertDialog>
          </DropdownMenuContent>
        </DropdownMenu>
      </div>
    </header>
    <Dialog :open="mobileTool !== null" @update:open="(open) => { if (!open) mobileTool = null }">
      <DialogContent class="max-h-[calc(100dvh-2rem)] grid-cols-1 overflow-y-auto" :aria-describedby="undefined">
        <DialogHeader><DialogTitle>{{ mobileTool === 'recording' ? '录制' : '截图' }}</DialogTitle></DialogHeader>
        <RecordingPanel
          v-if="mobileTool === 'recording'"
          :serial="serial"
          :minutes="rec.minutes.value" :record-audio="rec.recordAudio.value"
          :busy="rec.busy.value" :running="rec.running.value"
          :state-msg="rec.stateMsg.value" :history="rec.history.value" :history-error="rec.historyError.value"
          @update:minutes="rec.minutes.value = $event"
          @update:record-audio="rec.recordAudio.value = $event"
          @start="rec.start(rec.minutes.value, rec.recordAudio.value)"
          @stop="rec.stop()"
          @replay="mobileTool = null; startReplayFromRecording($event)"
          @delete="deleteRecordingTarget = $event"
        />
        <ScreenshotsPanel v-else-if="mobileTool === 'screenshots'" :serial="serial" :get-canvas="() => p.canvasRef.value" />
      </DialogContent>
    </Dialog>
    <Dialog v-model:open="sizeOpen">
      <DialogContent class="w-72">
        <DialogHeader><DialogTitle>显示尺寸</DialogTitle></DialogHeader>
        <Slider v-model="scaleModel" :min="25" :max="175" :step="1" aria-label="显示尺寸百分比" />
        <div class="flex items-center justify-between text-xs text-muted-foreground">
          <span>{{ p.displayLabel.value }}</span><Button variant="ghost" size="sm" @click="p.fitWindow()">适应窗口</Button>
        </div>
      </DialogContent>
    </Dialog>
    <!-- 舞台 + 侧栏 -->
    <SplitLayout :class="{ 'capture-layout': captureOpen }" :open="sidePanel !== null">
      <template #main><div class="flex h-full min-h-0 flex-col overflow-hidden p-0" :class="embedded ? 'bg-black' : 'bg-muted/40 gap-3 overflow-auto'">
      <main class="relative flex min-w-0 flex-1 items-center justify-center" :class="embedded ? 'min-h-0 bg-black' : 'min-h-80'">
        <div class="relative">
          <canvas :ref="p.canvasRef" tabindex="0" class="block touch-none bg-black select-none" :class="{ 'rounded-sm': !embedded, 'cursor-crosshair': inspectorVisible && inspectionMode }" aria-label="设备投屏画面" @pointerdown="p.canvasRef.value?.focus({ preventScroll: true })" />
          <canvas
            :ref="ins.overlayRef"
            class="pointer-events-none absolute inset-0 h-full w-full"
            aria-hidden="true"
          />
          <canvas
            :ref="tm.overlayRef"
            class="pointer-events-none absolute inset-0 h-full w-full"
            aria-hidden="true"
          />
          <TouchVisualizer ref="touchVisualizerRef" :visible="touchVisible" :width="p.videoSize.value.w" :height="p.videoSize.value.h" />
          <CaptureLayer
            v-if="captureOpen" :serial="serial" :get-canvas="() => p.canvasRef.value"
            :actions-target="`#${captureHostId}`"
            @close="captureOpen = false"
          />

          <!-- 加载中 -->
          <div v-if="p.loading.value" class="absolute inset-0 flex flex-col items-center justify-center gap-3 rounded-sm bg-black/60">
            <Loader2 class="h-8 w-8 animate-spin text-white" />
            <p class="text-sm text-white/80">{{ p.statusText.value }}</p>
          </div>

          <!-- 连接失败 -->
          <div v-if="p.errorMsg.value" class="absolute inset-0 flex items-center justify-center rounded-sm bg-black/70 p-4">
            <div class="flex max-w-sm flex-col items-center gap-2 rounded-xl bg-card p-6 text-center">
              <TriangleAlert class="h-8 w-8 text-destructive" />
              <p class="text-base font-semibold">连接失败</p>
              <p class="text-sm text-muted-foreground">{{ p.errorMsg.value }}</p>
              <Button class="mt-2" @click="p.retry()">重新连接</Button>
            </div>
          </div>
        </div>

        <!-- 灭屏提示 -->
        <div v-if="p.screenOff.value" class="absolute inset-x-0 top-0 flex justify-center p-3">
          <Badge variant="secondary" class="gap-1.5 px-3 py-1.5">
            <MonitorOff class="h-4 w-4" />设备屏幕已熄灭（投屏继续，点击电源键可点亮）
          </Badge>
        </div>

        <!-- 重放中徽标 -->
        <div v-if="replay.replaying.value" class="absolute inset-x-0 top-0 flex justify-center p-3">
          <Badge class="gap-1.5 px-3 py-1.5">正在重放{{ replay.loop.value ? '（循环）' : '' }}</Badge>
        </div>
      </main></div></template>
      <template #side>
      <div v-if="captureOpen" :id="captureHostId" class="h-full overflow-auto bg-card" />
      <div v-show="!captureOpen" class="flex h-full min-h-0 flex-col">
      <FilesView v-if="filesOpen" :serial="serial" embedded @close="filesOpen = false" />
      <InspectorPanel v-else-if="inspectorVisible" @close="inspectorVisible = false; setInspector(false)" />
      <AdbShellPanel v-else-if="shellOpen" :serial="serial" @close="shellOpen = false" />
      <ReplayPanel v-else-if="replayOpen"
        :serial="serial"
        :replaying="replay.replaying.value"
        :current-id="replay.currentId.value"
        :current-name="replay.currentEntry.value ? replay.currentEntry.value.recording_id : ''"
        :elapsed="replay.elapsedMs.value" :total="replay.totalMs.value" :remain="replay.remainMs.value"
        :loop-checked="replay.loop.value" :paused="replay.paused.value"
        :segment-on="replay.segmentOn.value" :segment-len="replay.segmentLenMs.value"
        :msg="replay.msg.value" :list="replay.list.value" :list-error="replay.listError.value"
        @close="toggleReplayPanel()"
        @stop="replay.stop()"
        @pause="replay.togglePause()"
        @loop="replay.loop.value = $event"
        @seek="replay.seek($event)"
        @seek-preview="replay.previewSeek($event); replay.setDragging($event !== null)"
        @seg-toggle="replay.toggleSegment($event)"
        @play="replay.start($event.recording_id, $event)"
        @delete="deleteRecordingTarget = $event"
      />
      <TemplatePanel v-else-if="templateOpen" :serial="serial" @close="templateOpen = false" />
      <PlayerSettingsPanel
        v-else-if="settingsOpen"
        :touch-visible="touchVisible" :boxes-visible="tm.showBoxes.value"
        :auto-unlock-on-wake="p.autoUnlockOnWake.value"
        :clipboard-sync="p.clipboardSync.value"
        :borderless="nativePlayerWindow ? windowEffects.player.value?.borderless : undefined"
        :window-busy="windowEffects.busy.value"
        @update:touch-visible="setTouchVisible"
        @update:boxes-visible="tm.setShow"
        @update:auto-unlock-on-wake="p.autoUnlockOnWake.value = $event"
        @update:clipboard-sync="p.clipboardSync.value = $event"
        @update:borderless="windowEffects.setBorderless"
        @close="settingsOpen = false"
      />
      </div>
      </template>
    </SplitLayout>


    <AlertDialog :open="deleteRecordingTarget !== null" @update:open="(v) => { if (!v) deleteRecordingTarget = null }">
      <AlertDialogContent>
        <AlertDialogHeader>
          <AlertDialogTitle>删除这条录制记录？</AlertDialogTitle>
          <AlertDialogDescription>含录像文件，不可撤销。</AlertDialogDescription>
        </AlertDialogHeader>
        <AlertDialogFooter>
          <AlertDialogCancel>取消</AlertDialogCancel>
          <AlertDialogAction class="bg-destructive text-destructive-foreground hover:bg-destructive/90" @click="confirmDeleteRecording">
            删除
          </AlertDialogAction>
        </AlertDialogFooter>
      </AlertDialogContent>
    </AlertDialog>
  </div>
</template>

<style scoped>
.pseudo-fullscreen {
  position: fixed;
  inset: 0;
  z-index: 50;
}
@media (min-width: 821px) {
  .capture-layout.split-layout-open:not(.split-layout-resized) {
    grid-template-columns: minmax(0, 1fr) 8px 280px;
  }
}
.toolbar-compact .toolbar-optional, .toolbar-compact .toolbar-secondary { display: none !important; }
</style>
