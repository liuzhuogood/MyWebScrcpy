<script setup lang="ts">
import {
  Check,
  Layers,
  Loader2,
  MonitorPlay,
  Pencil,
  RefreshCw,
  RotateCcw,
  Trash2,
  Tv,
  Usb,
  Wifi,
  X,
} from 'lucide-vue-next'
import { computed, nextTick, ref } from 'vue'
import type { ComponentPublicInstance } from 'vue'
import { useRouter } from 'vue-router'
import { invoke, isTauri } from '@tauri-apps/api/core'
import { Button } from '@/components/ui/button'
import { Input } from '@/components/ui/input'
import type { Device } from '@/api/devices'
import { useDeviceStore } from '@/stores/devices'
import { usePlayerSettings } from '@/composables/usePlayerSettings'
import { useDesktopConfig } from '@/composables/useDesktopConfig'
import { usePlayerWindows } from '@/composables/usePlayerWindows'
import { toast } from 'vue-sonner'

const props = withDefaults(
  defineProps<{
    device: Device
    viewMode?: 'deck' | 'compact'
  }>(),
  {
    viewMode: 'deck',
  },
)

const emit = defineEmits<{
  (e: 'openFloating', serial: string): void
}>()

const store = useDeviceStore()
const router = useRouter()
const playerSettings = usePlayerSettings()
const desktopConfig = useDesktopConfig()
const windowEffects = usePlayerWindows(computed(() => props.device.serial))

const cardRef = ref<HTMLDivElement | null>(null)
const rotateX = ref(0)
const rotateY = ref(0)
const glowX = ref(50)
const glowY = ref(50)
const isHovering = ref(false)

const isGui = computed(() => {
  return desktopConfig.isDesktop.value || Boolean(
    isTauri() ||
    (window as any).__MYWEBSCRCPY_DESKTOP_TOKEN__ ||
    (window as any).__TAURI_INTERNALS__ ||
    (window as any).__TAURI__
  )
})

const isEditing = computed(() => store.editingSerial === props.device.serial)
const draft = ref('')
const inputRef = ref<ComponentPublicInstance | null>(null)
const isReconnecting = ref(false)
const isDeleting = ref(false)

const isManual = computed(() => props.device.source === 'manual')
const isOnline = computed(() => props.device.state === 'device')
const isConnecting = computed(() => props.device.state === 'connecting' || isReconnecting.value)
const isUnauthorized = computed(() => props.device.state === 'unauthorized')

// Mock 前台活跃应用
interface ForegroundApp {
  name: string
  pkg: string
}

const foregroundApp = computed<ForegroundApp>(() => {
  if (!isOnline.value) {
    return { name: '无前台活跃应用', pkg: '设备处于离线休眠状态' }
  }
  const s = props.device.serial
  const charCode = s.charCodeAt(s.length - 1) || 0
  const apps: ForegroundApp[] = [
    { name: '控制台主页', pkg: 'com.mywebscrcpy.desktop' },
    { name: '微信', pkg: 'com.tencent.mm' },
    { name: '系统设置', pkg: 'com.android.settings' },
    { name: '系统桌面', pkg: 'com.android.launcher3' },
    { name: '哔哩哔哩', pkg: 'tv.danmaku.bili' },
  ]
  return apps[charCode % apps.length]
})

function handleMouseMove(e: MouseEvent) {
  if (!cardRef.value || props.viewMode !== 'deck') return
  const rect = cardRef.value.getBoundingClientRect()
  const x = e.clientX - rect.left
  const y = e.clientY - rect.top
  const centerX = rect.width / 2
  const centerY = rect.height / 2

  // 限制最大旋转 8 度，呈现轻盈立体的 3D 浮雕感
  rotateX.value = ((y - centerY) / centerY) * -8
  rotateY.value = ((x - centerX) / centerX) * 8

  glowX.value = (x / rect.width) * 100
  glowY.value = (y / rect.height) * 100
  isHovering.value = true
}

function handleMouseLeave() {
  rotateX.value = 0
  rotateY.value = 0
  isHovering.value = false
}

async function startEdit(e: Event) {
  e.stopPropagation()
  store.editingSerial = props.device.serial
  draft.value = props.device.alias || store.aliases[props.device.serial] || props.device.model || ''
  await nextTick()
  const el = inputRef.value?.$el as HTMLInputElement | undefined
  el?.focus()
  el?.select()
}

function save() {
  store.saveAlias(props.device.serial, draft.value, props.device.model || '未知设备')
  store.editingSerial = null
}

function cancel() {
  store.editingSerial = null
}

async function handleReconnect(e: Event) {
  e.stopPropagation()
  if (isConnecting.value)
    return
  isReconnecting.value = true
  toast.info('正在尝试重新握手连接...')
  try {
    await store.reconnect(props.device.serial)
    toast.success('设备连接指令已发送')
  }
  catch (err) {
    toast.error(err instanceof Error ? err.message : '连接设备失败')
  }
  finally {
    setTimeout(() => {
      isReconnecting.value = false
    }, 1500)
  }
}

async function handleDelete(e: Event) {
  e.stopPropagation()
  if (isDeleting.value)
    return
  isDeleting.value = true
  try {
    await store.removeDevice(props.device.serial)
    toast.success('已删除设备')
  }
  catch (err) {
    isDeleting.value = false
    toast.error(err instanceof Error ? err.message : '删除设备失败')
  }
}

function open() {
  if (isEditing.value || !isOnline.value)
    return
  router.push({ name: 'player', query: { serial: props.device.serial } })
}

async function openFloatingWindow(e?: Event) {
  e?.stopPropagation()
  if (isEditing.value || !isOnline.value)
    return
  if (isGui.value && isTauri()) {
    try {
      const HEADER_HEIGHT = 56
      const targetW = playerSettings.floatingSize.value === 'compact' ? 320 : playerSettings.floatingSize.value === 'large' ? 440 : 380
      const baseH = playerSettings.floatingSize.value === 'compact' ? 680 : playerSettings.floatingSize.value === 'large' ? 880 : 780
      const targetH = baseH + HEADER_HEIGHT
      await invoke('open_device_window', {
        serial: props.device.serial,
        title: store.displayName(props.device),
        width: targetW,
        height: targetH,
        borderless: playerSettings.borderless.value,
      })
      return
    }
    catch (e) {
      console.warn('打开桌面小窗失败，降级为网页小窗', e)
    }
  }
  emit('openFloating', props.device.serial)
}
</script>

<template>
  <!-- 模式一：3D 全息甲板视图 (Deck View) -->
  <div
    v-if="viewMode === 'deck'"
    class="perspective-1000"
    style="perspective: 1000px;"
  >
    <div
      ref="cardRef"
      class="group relative flex flex-col justify-between overflow-hidden rounded-3xl border p-6 transition-all duration-200 ease-out will-change-transform cursor-pointer select-none"
      :class="[
        isOnline
          ? 'border-slate-200/90 bg-white/85 shadow-[0_10px_30px_rgba(0,0,0,0.06)] backdrop-blur-xl hover:border-blue-500/50 hover:bg-white/95 hover:shadow-[0_20px_40px_rgba(64,107,219,0.14)] text-slate-900 dark:border-white/10 dark:bg-slate-900/60 dark:shadow-[0_20px_50px_rgba(0,0,0,0.5)] dark:backdrop-blur-2xl dark:hover:border-blue-400/40 dark:hover:bg-slate-900/80 dark:text-white'
          : isConnecting
            ? 'border-blue-500/30 bg-blue-50/70 text-slate-900 dark:border-blue-500/25 dark:bg-blue-950/30 dark:text-white backdrop-blur-xl'
            : isUnauthorized
              ? 'border-amber-500/30 bg-amber-50/70 text-slate-900 dark:border-amber-500/25 dark:bg-amber-950/30 dark:text-white backdrop-blur-xl'
              : 'border-slate-200/70 bg-white/60 text-slate-600 opacity-75 hover:opacity-100 dark:border-white/5 dark:bg-slate-900/30 dark:text-slate-300 backdrop-blur-xl',
      ]"
      :style="{
        transform: `rotateX(${rotateX}deg) rotateY(${rotateY}deg) ${isHovering ? 'scale3d(1.02, 1.02, 1.02)' : 'scale3d(1, 1, 1)'}`,
      }"
      @mousemove="handleMouseMove"
      @mouseleave="handleMouseLeave"
      @click="open"
    >
      <!-- 3D 玻璃动态反光高光层 (Specular Highlight) -->
      <div
        class="pointer-events-none absolute inset-0 opacity-0 transition-opacity duration-300 group-hover:opacity-100"
        :style="{
          background: `radial-gradient(circle at ${glowX}% ${glowY}%, rgba(64, 107, 219, 0.12) 0%, rgba(59, 130, 246, 0.06) 35%, transparent 70%)`,
        }"
      />

      <!-- 顶栏状态徽章与来源 -->
      <div class="relative z-10 flex items-center justify-between">
        <div class="flex items-center gap-2">
          <!-- 状态指示徽章 -->
          <span
            class="inline-flex items-center gap-1.5 rounded-full px-2.5 py-1 text-[11px] font-semibold backdrop-blur-md"
            :class="[
              isOnline
                ? 'bg-emerald-500/15 text-emerald-700 dark:text-emerald-300 border border-emerald-500/30'
                : isConnecting
                  ? 'bg-blue-500/15 text-blue-700 dark:text-blue-300 border border-blue-500/30'
                  : isUnauthorized
                    ? 'bg-amber-500/15 text-amber-700 dark:text-amber-300 border border-amber-500/30'
                    : 'bg-slate-200/70 text-slate-600 border border-slate-300/60 dark:bg-white/5 dark:text-slate-400 dark:border-white/10',
            ]"
          >
            <span
              v-if="!isConnecting"
              class="h-1.5 w-1.5 rounded-full"
              :class="isOnline ? 'bg-emerald-500 dark:bg-emerald-400 animate-pulse' : isUnauthorized ? 'bg-amber-500' : 'bg-slate-400'"
            />
            <Loader2 v-else class="h-3 w-3 animate-spin text-blue-500" />
            {{ isOnline ? '设备在线' : isConnecting ? '正在连接...' : isUnauthorized ? '等待授权' : '离线待机' }}
          </span>

          <!-- 硬件型号标识 -->
          <span class="rounded-full bg-slate-100/90 text-slate-700 border border-slate-200/80 px-2.5 py-1 text-[10px] font-mono truncate max-w-[120px] dark:bg-white/5 dark:text-slate-300 dark:border-white/10">
            {{ device.product || device.device || 'Android' }}
          </span>
        </div>

        <!-- 来源类型：直连 or 无线 -->
        <div class="flex items-center gap-1.5 text-xs text-slate-500 dark:text-slate-400">
          <Usb v-if="!isManual" class="h-3.5 w-3.5 text-cyan-600 dark:text-cyan-400" />
          <Wifi v-else class="h-3.5 w-3.5 text-violet-600 dark:text-violet-400" />
          <span class="text-[11px] font-medium">{{ !isManual ? 'USB 直连' : '无线连接' }}</span>
        </div>
      </div>

      <!-- 设备核心信息区 -->
      <div class="relative z-10 mt-5">
        <div v-if="!isEditing" class="flex items-center justify-between gap-2">
          <div class="min-w-0 flex-1">
            <h3 class="truncate text-lg font-bold tracking-tight text-slate-900 dark:text-white group-hover:text-blue-600 dark:group-hover:text-blue-300 transition-colors">
              {{ store.displayName(device) }}
            </h3>
            <p class="mt-0.5 truncate text-xs text-slate-500 dark:text-slate-400 font-mono">
              {{ device.model || 'Android' }} · {{ device.serial }}
            </p>
          </div>
          <Button
            variant="ghost"
            size="icon"
            class="h-7 w-7 rounded-lg text-slate-400 hover:text-slate-900 dark:hover:text-white opacity-0 group-hover:opacity-100 transition-opacity cursor-pointer shrink-0"
            title="重命名设备"
            aria-label="重命名设备"
            @click.stop="startEdit"
          >
            <Pencil class="h-3.5 w-3.5" />
          </Button>
        </div>

        <!-- 编辑别名状态 -->
        <div v-else class="flex items-center gap-1.5 py-1" @click.stop>
          <Input
            ref="inputRef"
            v-model="draft"
            maxlength="20"
            placeholder="输入设备名称"
            aria-label="设备名称"
            class="h-8 rounded-xl border-slate-300 bg-white text-xs text-slate-900 dark:border-white/20 dark:bg-black/40 dark:text-white"
            @keydown.enter.prevent="save"
            @keydown.esc.prevent="cancel"
          />
          <Button size="sm" class="h-8 rounded-xl bg-[#406BDB] hover:bg-[#3258be] text-white px-2.5" @click="save">
            <Check class="h-3.5 w-3.5" />
          </Button>
          <Button size="sm" variant="ghost" class="h-8 rounded-xl text-slate-500 hover:text-slate-900 dark:text-slate-400 dark:hover:text-white px-2.5" @click="cancel">
            <X class="h-3.5 w-3.5" />
          </Button>
        </div>

        <!-- 前台活跃应用小卡片（替换原有的通道/传输/状态统计） -->
        <div class="relative mt-4 overflow-hidden rounded-2xl border border-slate-200/90 bg-slate-50/80 p-3.5 backdrop-blur-md shadow-inner dark:border-white/10 dark:bg-black/40">
          <div class="flex items-center justify-between text-xs">
            <span class="text-slate-500 dark:text-slate-400 flex items-center gap-1.5 font-medium">
              <Layers class="h-3.5 w-3.5 text-blue-600 dark:text-blue-400" />
              前台应用
            </span>
            <span
              v-if="isOnline"
              class="font-medium text-emerald-600 dark:text-emerald-400 flex items-center gap-1 text-[11px]"
            >
              <span class="h-1.5 w-1.5 rounded-full bg-emerald-500 animate-pulse" />
              运行中
            </span>
            <span
              v-else
              class="text-slate-400 text-[11px]"
            >
              待命
            </span>
          </div>

          <div class="mt-2.5 rounded-xl bg-white/80 p-2.5 border border-slate-200/60 dark:bg-white/[0.04] dark:border-white/5">
            <div class="flex items-center justify-between gap-2">
              <div class="min-w-0 flex-1">
                <span class="text-xs font-semibold text-slate-900 dark:text-white truncate block">
                  {{ foregroundApp.name }}
                </span>
                <span class="text-[10px] font-mono text-slate-500 dark:text-slate-400 truncate block mt-0.5">
                  {{ foregroundApp.pkg }}
                </span>
              </div>
              <span class="rounded-lg bg-blue-500/10 px-2 py-0.5 text-[10px] font-medium text-blue-600 dark:text-blue-400 shrink-0 border border-blue-500/20">
                前台
              </span>
            </div>
          </div>
        </div>
      </div>

      <!-- 底部操作条 -->
      <div class="relative z-10 mt-5 border-t border-slate-200/80 dark:border-white/10 pt-3.5" @click.stop>
        <div class="flex items-center gap-2">
          <!-- 恢复小窗 (当存在桌面小窗需要恢复时) -->
          <Button
            v-if="windowEffects.native && windowEffects.needsRecovery.value"
            variant="outline"
            size="sm"
            class="rounded-xl border-amber-500/50 bg-amber-500/10 text-amber-700 dark:text-amber-300 hover:bg-amber-500/20 text-xs px-2.5"
            :disabled="windowEffects.busy.value"
            @click.stop="windowEffects.restore"
          >
            <RotateCcw class="mr-1 h-3.5 w-3.5" />
            恢复
          </Button>

          <!-- 设备投屏主按钮（纯蓝色 #406BDB） -->
          <Button
            size="sm"
            :disabled="!isOnline"
            class="flex-1 rounded-xl bg-[#406BDB] hover:bg-[#3258be] font-medium text-white shadow-sm transition-colors cursor-pointer text-xs"
            @click.stop="open"
          >
            <MonitorPlay class="mr-1.5 h-3.5 w-3.5" />
            设备投屏
          </Button>

          <!-- 设备画中画微窗 -->
          <Button
            size="sm"
            variant="ghost"
            :disabled="!isOnline"
            class="h-9 w-9 rounded-xl border border-slate-200/80 bg-white/80 text-slate-700 hover:bg-slate-100 hover:text-slate-950 dark:border-white/10 dark:bg-white/5 dark:text-slate-200 dark:hover:bg-white/10 dark:hover:text-white p-0 cursor-pointer shadow-2xs"
            title="画中画微窗"
            aria-label="画中画微窗"
            @click.stop="openFloatingWindow"
          >
            <Tv class="h-3.5 w-3.5" />
          </Button>

          <!-- 手动网络设备：离线重试连接 -->
          <Button
            v-if="isManual && !isOnline"
            size="sm"
            variant="ghost"
            class="h-9 w-9 rounded-xl border border-slate-200/80 bg-white/80 text-slate-700 hover:bg-slate-100 hover:text-slate-950 dark:border-white/10 dark:bg-white/5 dark:text-slate-200 dark:hover:bg-white/10 dark:hover:text-white p-0 cursor-pointer shadow-2xs"
            :title="isConnecting ? '正在重连' : '重新连接'"
            :disabled="isConnecting"
            @click.stop="handleReconnect"
          >
            <Loader2 v-if="isConnecting" class="h-3.5 w-3.5 animate-spin text-blue-500" />
            <RefreshCw v-else class="h-3.5 w-3.5" />
          </Button>

          <!-- 手动网络设备：删除设备 -->
          <Button
            v-if="isManual"
            size="sm"
            variant="ghost"
            class="h-9 w-9 rounded-xl border border-slate-200/80 bg-white/80 text-slate-600 hover:bg-red-50 hover:text-red-600 dark:border-white/10 dark:bg-white/5 dark:text-slate-300 dark:hover:bg-red-500/20 dark:hover:text-red-400 p-0 cursor-pointer shadow-2xs"
            title="删除设备"
            aria-label="删除设备"
            :disabled="isDeleting"
            @click.stop="handleDelete"
          >
            <Trash2 class="h-3.5 w-3.5" />
          </Button>
        </div>
      </div>
    </div>
  </div>

  <!-- 模式二：紧凑型设备阵列 (Compact View) -->
  <div
    v-else
    class="group relative flex items-center justify-between rounded-2xl border border-slate-200/80 bg-white/85 p-4 text-slate-900 backdrop-blur-xl transition-all hover:border-blue-500/50 hover:bg-white/95 shadow-sm dark:border-white/10 dark:bg-slate-900/60 dark:text-white dark:hover:border-blue-400/40 dark:hover:bg-slate-900/80 select-none cursor-pointer"
    @click="open"
  >
    <div class="flex items-center gap-3 min-w-0">
      <div
        class="flex h-10 w-10 shrink-0 items-center justify-center rounded-xl"
        :class="isOnline ? 'bg-emerald-500/15 text-emerald-600 border border-emerald-500/30' : 'bg-slate-100 text-slate-500 border border-slate-200 dark:bg-white/5 dark:text-slate-400 dark:border-white/10'"
      >
        <Smartphone class="h-5 w-5" />
      </div>

      <div class="min-w-0">
        <div class="flex items-center gap-2">
          <p class="truncate font-semibold text-sm text-slate-900 group-hover:text-blue-600 dark:text-white dark:group-hover:text-blue-300 transition-colors">
            {{ store.displayName(device) }}
          </p>
          <span
            class="h-1.5 w-1.5 rounded-full shrink-0"
            :class="isOnline ? 'bg-emerald-500' : isConnecting ? 'bg-blue-500 animate-pulse' : 'bg-slate-400'"
          />
        </div>
        <p class="truncate font-mono text-[11px] text-slate-500 dark:text-slate-400">
          {{ device.serial }} · <span class="text-blue-600 dark:text-blue-400">{{ foregroundApp.name }}</span>
        </p>
      </div>
    </div>

    <!-- 紧凑右侧操作（纯蓝色 #406BDB） -->
    <div class="flex items-center gap-1.5 shrink-0" @click.stop>
      <Button
        v-if="isOnline"
        size="sm"
        class="h-8 rounded-xl bg-[#406BDB] hover:bg-[#3258be] text-white text-xs px-3 shadow-xs cursor-pointer"
        @click.stop="open"
      >
        <MonitorPlay class="mr-1 h-3.5 w-3.5" />
        投屏
      </Button>
      <Button
        v-if="isOnline"
        size="icon"
        variant="ghost"
        class="h-8 w-8 rounded-xl border border-slate-200/80 bg-white/80 text-slate-700 hover:text-slate-950 dark:border-white/10 dark:bg-white/5 dark:text-slate-300 dark:hover:text-white cursor-pointer"
        title="画中画"
        @click.stop="openFloatingWindow"
      >
        <Tv class="h-3.5 w-3.5" />
      </Button>
      <Button
        size="icon"
        variant="ghost"
        class="h-8 w-8 rounded-xl text-slate-500 hover:text-slate-950 dark:text-slate-400 dark:hover:text-white cursor-pointer"
        title="重命名"
        @click.stop="startEdit"
      >
        <Pencil class="h-3.5 w-3.5" />
      </Button>
      <Button
        v-if="isManual"
        size="icon"
        variant="ghost"
        class="h-8 w-8 rounded-xl text-slate-500 hover:text-red-600 dark:text-slate-400 dark:hover:text-red-400 cursor-pointer"
        title="删除"
        @click.stop="handleDelete"
      >
        <Trash2 class="h-3.5 w-3.5" />
      </Button>
    </div>
  </div>
</template>
