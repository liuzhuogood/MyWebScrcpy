<script setup lang="ts">
import { Loader2, Pencil, RefreshCw, Smartphone, Trash2, Usb, Wifi } from 'lucide-vue-next'
import { computed, nextTick, ref } from 'vue'
import type { ComponentPublicInstance } from 'vue'
import { useRouter } from 'vue-router'
import { Badge } from '@/components/ui/badge'
import { Button } from '@/components/ui/button'
import { Card, CardContent } from '@/components/ui/card'
import { Input } from '@/components/ui/input'
import type { Device } from '@/api/devices'
import { useDeviceStore } from '@/stores/devices'

const props = defineProps<{ device: Device }>()
const store = useDeviceStore()
const router = useRouter()

const isEditing = computed(() => store.editingSerial === props.device.serial)
const draft = ref('')
const inputRef = ref<ComponentPublicInstance | null>(null)
const isReconnecting = ref(false)
const isDeleting = ref(false)

const isManual = computed(() => props.device.source === 'manual')
const isOnline = computed(() => props.device.state === 'device')
const isConnecting = computed(() => props.device.state === 'connecting' || isReconnecting.value)

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
  try {
    await store.reconnect(props.device.serial)
  }
  catch {
    // 忽略或等待轮询
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
  }
  catch {
    isDeleting.value = false
  }
}

function open() {
  if (isEditing.value || !isOnline.value)
    return
  router.push({ name: 'player', query: { serial: props.device.serial } })
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
