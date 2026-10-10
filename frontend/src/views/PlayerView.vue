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
} from 'lucide-vue-next'
import { computed, onMounted, ref } from 'vue'
import { useRoute, RouterLink } from 'vue-router'
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
  DropdownMenuSeparator,
  DropdownMenuTrigger,
} from '@/components/ui/dropdown-menu'
import { Dialog, DialogContent, DialogHeader, DialogTitle } from '@/components/ui/dialog'
import { Empty, EmptyDescription, EmptyHeader, EmptyMedia, EmptyTitle } from '@/components/ui/empty'
import { Popover, PopoverContent, PopoverTrigger } from '@/components/ui/popover'
import { Slider } from '@/components/ui/slider'
import { setInspector } from '@/inspector/guard'
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
import FilesView from './FilesView.vue'
import PlayerSettingsPanel from '@/components/player/PlayerSettingsPanel.vue'
import TemplatePanel from '@/template/TemplatePanel.vue'
import { useTemplateMatches } from '@/template/useTemplateMatches'
import type { RecordingEntry } from '@/api/recordings'

const route = useRoute()
const serial = computed(() => (route.query.serial as string) || '')

const p = usePlayer(serial.value, {
  onOp: (type, data) => {
    recordOp(serial.value, type, data)
  },
  onTouch: event => touchVisualizerRef.value?.handle(event),
})
const ins = provideInspector(serial.value, () => p.canvasRef.value)
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
const touchVisible = ref(localStorage.getItem('show-touch-overlay') !== 'false')
const touchVisualizerRef = ref<InstanceType<typeof TouchVisualizer> | null>(null)
const deleteRecordingTarget = ref<RecordingEntry | null>(null)

onMounted(() => {
  if (serial.value) {
    tm.start()
    void rec.loadHistory()
  }
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

  <div v-else :ref="p.stageRef" class="flex h-screen flex-col bg-background">
    <!-- 工具栏 -->
    <header class="flex h-14 shrink-0 items-center gap-1 overflow-x-auto border-b px-3">
      <Button variant="ghost" size="icon" aria-label="返回设备列表" as-child>
        <RouterLink to="/"><ArrowLeft class="h-5 w-5" /></RouterLink>
      </Button>
      <span class="ml-1 max-w-48 truncate font-mono text-sm font-medium" :title="serial">{{ serial }}</span>
      <Badge :variant="p.live.value ? 'default' : 'secondary'">{{ p.statusText.value }}</Badge>
      <Badge v-if="p.showFps.value" variant="outline">{{ p.fps.value }} fps</Badge>

      <div class="ml-auto flex items-center gap-1">
        <Button variant="ghost" size="icon" title="返回" aria-label="返回" @click="p.pressBack()">
          <Undo2 class="h-5 w-5" />
        </Button>
        <Button variant="ghost" size="icon" title="主页" aria-label="主页" @click="p.pressHome()">
          <Home class="h-5 w-5" />
        </Button>
        <Button variant="ghost" size="icon" title="切换任务" aria-label="切换任务" @click="p.pressRecents()">
          <GalleryHorizontal class="h-5 w-5" />
        </Button>
        <Button variant="ghost" size="icon" title="电源键" aria-label="电源键" :disabled="p.powerBusy.value" @click="p.pressPower()">
          <Power class="h-5 w-5" />
        </Button>
        <Button variant="ghost" size="icon" title="横竖屏" aria-label="横竖屏" @click="p.rotate()">
          <RotateCw class="h-5 w-5" />
        </Button>
        <Button
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
        <Popover v-else class="toolbar-optional">
          <PopoverTrigger as-child>
            <Button variant="ghost" size="icon" title="录制" aria-label="录制">
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
        <Popover class="toolbar-optional">
          <PopoverTrigger as-child>
            <Button variant="ghost" size="icon" title="截图" aria-label="截图">
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
          variant="ghost" size="icon"
          title="检查画布与界面" aria-label="检查"
          :class="{ 'bg-accent': inspectorVisible }"
          @click="toggleInspector()"
        >
          <ScanSearch class="h-5 w-5" />
        </Button>
        <Button
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
    <SplitLayout :class="{ 'capture-layout': captureOpen }" :open="captureOpen || filesOpen || shellOpen || inspectorVisible || replayOpen || templateOpen || settingsOpen">
      <template #main><div class="flex h-full min-h-0 flex-col gap-3 overflow-auto bg-muted/40 p-0">
      <main class="relative flex min-h-80 min-w-0 flex-1 items-center justify-center">
        <div class="relative">
          <canvas :ref="p.canvasRef" class="touch-none rounded-sm bg-black select-none" aria-label="设备投屏画面" />
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
            actions-target="#capture-actions-host"
            @close="captureOpen = false"
          />
        </div>

        <!-- 加载中 -->
        <div v-if="p.loading.value" class="absolute inset-0 flex flex-col items-center justify-center gap-3 bg-black/60">
          <Loader2 class="h-8 w-8 animate-spin text-white" />
          <p class="text-sm text-white/80">{{ p.statusText.value }}</p>
        </div>

        <!-- 连接失败 -->
        <div v-if="p.errorMsg.value" class="absolute inset-0 flex items-center justify-center bg-black/70 p-4">
          <div class="flex max-w-sm flex-col items-center gap-2 rounded-xl bg-card p-6 text-center">
            <TriangleAlert class="h-8 w-8 text-destructive" />
            <p class="text-base font-semibold">连接失败</p>
            <p class="text-sm text-muted-foreground">{{ p.errorMsg.value }}</p>
            <Button class="mt-2" @click="p.retry()">重新连接</Button>
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
      <div v-if="captureOpen" id="capture-actions-host" class="h-full overflow-auto bg-card" />
      <div v-show="!captureOpen" class="flex h-full min-h-0 flex-col">
      <FilesView v-if="filesOpen" embedded @close="filesOpen = false" />
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
        @update:touch-visible="setTouchVisible"
        @update:boxes-visible="tm.setShow"
        @update:auto-unlock-on-wake="p.autoUnlockOnWake.value = $event"
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
@media (max-width: 900px) {
  .toolbar-optional { display: none !important; }
}
</style>
