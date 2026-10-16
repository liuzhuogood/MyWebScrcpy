<script setup lang="ts">
import { AppWindow, Loader2, Pencil, RefreshCw, Smartphone, Trash2, Usb, Wifi, RotateCcw } from 'lucide-vue-next'
import { computed, nextTick, ref } from 'vue'
import type { ComponentPublicInstance } from 'vue'
import { useRouter } from 'vue-router'
import { invoke, isTauri } from '@tauri-apps/api/core'
import { Badge } from '@/components/ui/badge'
import { Button } from '@/components/ui/button'
import { Card, CardContent } from '@/components/ui/card'
import { Input } from '@/components/ui/input'
import type { Device } from '@/api/devices'
import { useDeviceStore } from '@/stores/devices'
import { usePlayerSettings } from '@/composables/usePlayerSettings'
import { useDesktopConfig } from '@/composables/useDesktopConfig'
import { usePlayerWindows } from '@/composables/usePlayerWindows'
import { toast } from 'vue-sonner'

const props = defineProps<{ device: Device }>()
const emit = defineEmits<{ openFloating: [serial: string] }>()
const store = useDeviceStore()
const router = useRouter()
const playerSettings = usePlayerSettings()
const desktopConfig = useDesktopConfig()
const windowEffects = usePlayerWindows(computed(() => props.device.serial))

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
  toast.info('正在尝试连接设备...')
  try {
    await store.reconnect(props.device.serial)
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
  <Card
    class="group transition-colors"
    :class="[
      isOnline ? 'cursor-pointer hover:border-primary/50' : 'opacity-85',
    ]"
    @click="open"
  >
    <CardContent class="flex items-start gap-3 p-4">
      <span
        class="flex h-10 w-10 shrink-0 items-center justify-center rounded-lg"
        :class="isOnline ? 'bg-primary/10 text-primary' : 'bg-muted text-muted-foreground'"
      >
        <Smartphone class="h-5 w-5" />
      </span>

      <div class="min-w-0 flex-1">
        <template v-if="!isEditing">
          <div class="flex items-center gap-2">
            <p class="truncate font-medium">{{ store.displayName(device) }}</p>

            <!-- 来源标识徽章 -->
            <Badge
              v-if="!isManual"
              variant="outline"
              class="flex shrink-0 items-center gap-1 text-[11px] font-normal text-muted-foreground"
            >
              <Usb class="h-2.5 w-2.5 text-blue-500" />
              自动发现
            </Badge>
            <Badge
              v-else
              variant="secondary"
              class="flex shrink-0 items-center gap-1 bg-amber-500/10 text-[11px] font-normal text-amber-600 dark:text-amber-400"
            >
              <Wifi class="h-2.5 w-2.5" />
              手动添加
            </Badge>
          </div>

          <p class="mt-1 truncate font-mono text-xs text-muted-foreground">
            {{ device.serial }}{{ device.product ? ` · ${device.product}` : '' }}
          </p>

          <div class="mt-2 flex items-center gap-2 text-xs">
            <span v-if="isOnline" class="flex items-center gap-1 text-emerald-600 dark:text-emerald-400 font-medium">
              <span class="h-1.5 w-1.5 rounded-full bg-emerald-500"></span>
              在线
            </span>
            <span v-else-if="isConnecting" class="flex items-center gap-1 text-blue-500 font-medium">
              <Loader2 class="h-3 w-3 animate-spin" />
              连接中...
            </span>
            <span v-else-if="isUnauthorized" class="flex items-center gap-1 text-amber-600 dark:text-amber-400 font-medium">
              <span class="h-1.5 w-1.5 rounded-full bg-amber-500"></span>
              未授权
            </span>
            <span v-else class="flex items-center gap-1 text-muted-foreground">
              <span class="h-1.5 w-1.5 rounded-full bg-muted-foreground/40"></span>
              离线
            </span>

            <span v-if="isManual && device.auto_reconnect" class="text-[11px] text-muted-foreground">
              · 自动重连
            </span>
          </div>
        </template>

        <template v-else>
          <div class="flex gap-1.5" @click.stop>
            <Input
              ref="inputRef"
              v-model="draft"
              maxlength="20"
              placeholder="输入设备名称"
              aria-label="设备名称"
              class="h-8"
              @keydown.enter.prevent="save"
              @keydown.esc.prevent="cancel"
            />
            <Button size="sm" @click="save">保存</Button>
            <Button size="sm" variant="ghost" @click="cancel">取消</Button>
          </div>
        </template>
      </div>

      <div v-if="!isEditing" class="flex items-center gap-0.5">
        <Button
          v-if="windowEffects.native && windowEffects.needsRecovery.value"
          variant="outline"
          size="sm"
          :disabled="windowEffects.busy.value"
          @click.stop="windowEffects.restore"
        >
          <RotateCcw class="h-4 w-4" />恢复小窗
        </Button>
        <Button
          v-if="isOnline"
          variant="ghost"
          size="icon"
          class="h-8 w-8 opacity-0 group-hover:opacity-100 focus-visible:opacity-100 text-muted-foreground hover:text-foreground"
          title="打开小窗"
          aria-label="打开小窗"
          @click.stop="openFloatingWindow"
        >
          <AppWindow class="h-4 w-4" />
        </Button>

        <Button
          variant="ghost"
          size="icon"
          class="h-8 w-8 opacity-0 group-hover:opacity-100 focus-visible:opacity-100 text-muted-foreground"
          :aria-label="`重命名 ${store.displayName(device)}`"
          @click="startEdit"
        >
          <Pencil class="h-4 w-4" />
        </Button>

        <!-- 手动设备离线时支持手动重连 -->
        <Button
          v-if="isManual && !isOnline"
          variant="ghost"
          size="icon"
          class="h-8 w-8 text-muted-foreground hover:text-foreground"
          :title="isConnecting ? '正在连接' : '重试连接'"
          :disabled="isConnecting"
          @click="handleReconnect"
        >
          <Loader2 v-if="isConnecting" class="h-4 w-4 animate-spin text-blue-500" />
          <RefreshCw v-else class="h-4 w-4" />
        </Button>

        <!-- 手动设备支持删除 -->
        <Button
          v-if="isManual"
          variant="ghost"
          size="icon"
          class="h-8 w-8 opacity-0 group-hover:opacity-100 focus-visible:opacity-100 text-muted-foreground hover:text-destructive"
          title="删除设备"
          :disabled="isDeleting"
          @click="handleDelete"
        >
          <Trash2 class="h-4 w-4" />
        </Button>
      </div>
    </CardContent>
  </Card>
</template>
