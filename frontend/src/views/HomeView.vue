<script setup lang="ts">
import {
  CheckCircle2,
  Globe,
  Loader2,
  Plus,
  RefreshCw,
  Search,
  Smartphone,
  Usb,
  Wifi,
} from 'lucide-vue-next'
import { computed, onMounted, onUnmounted, ref } from 'vue'
import AppHeader from '@/components/layout/AppHeader.vue'
import DeviceCard from '@/components/device/DeviceCard.vue'
import FloatingPlayer from '@/components/player/FloatingPlayer.vue'
import { Button } from '@/components/ui/button'
import {
  Dialog,
  DialogContent,
  DialogDescription,
  DialogFooter,
  DialogHeader,
  DialogTitle,
} from '@/components/ui/dialog'
import { Empty, EmptyDescription, EmptyHeader, EmptyMedia, EmptyTitle } from '@/components/ui/empty'
import { Input } from '@/components/ui/input'
import { Skeleton } from '@/components/ui/skeleton'
import { Switch } from '@/components/ui/switch'
import { useDeviceStore } from '@/stores/devices'

const store = useDeviceStore()
const isDevelopment = import.meta.env.DEV
const floatingWindows = ref<string[]>([])
const focusedSerial = ref<string | null>(null)

function openFloating(serial: string) {
  if (!floatingWindows.value.includes(serial)) {
    floatingWindows.value.push(serial)
  }
  focusedSerial.value = serial
}

function closeFloating(serial: string) {
  floatingWindows.value = floatingWindows.value.filter(s => s !== serial)
  if (focusedSerial.value === serial) {
    focusedSerial.value = floatingWindows.value[floatingWindows.value.length - 1] || null
  }
}

function focusFloating(serial: string) {
  focusedSerial.value = serial
}

const countText = computed(() => {
  if (store.mockEnabled)
    return `${store.devices.length} 台设备（Mock）`
  if (store.loading && store.devices.length === 0)
    return '加载中...'
  if (store.error)
    return '加载失败'
  const online = store.devices.filter(d => d.state === 'device').length
  return `${online} 台设备在线`
})

// 添加设备对话框状态
const isAddDialogOpen = ref(false)
const addHost = ref('')
const addPort = ref('5555')
const addAlias = ref('')
const addError = ref('')
const isAdding = ref(false)

function openAddDialog() {
  addHost.value = ''
  addPort.value = '5555'
  addAlias.value = ''
  addError.value = ''
  isAdding.value = false
  isAddDialogOpen.value = true
}

async function handleAddSubmit() {
  const host = addHost.value.trim()
  const port = addPort.value.trim() || '5555'

  if (!host) {
    addError.value = '请输入设备 IP 或主机名'
    return
  }

  const serial = `${host}:${port}`
  addError.value = ''
  isAdding.value = true

  try {
    await store.addDevice(serial, addAlias.value.trim())
    isAddDialogOpen.value = false
  }
  catch (e) {
    addError.value = e instanceof Error ? e.message : '添加设备失败'
  }
  finally {
    isAdding.value = false
  }
}

onMounted(() => store.startPolling())
onUnmounted(() => store.stopPolling())
</script>

<template>
  <div class="min-h-screen">
    <AppHeader :device-count-text="countText" />
    <main class="w-full px-4 py-6">
      <div class="mb-4 flex flex-col gap-3 sm:flex-row sm:items-center sm:justify-between">
        <div class="flex flex-1 flex-wrap items-center gap-2">
          <div class="relative flex-1 max-w-sm">
            <Search class="absolute left-2.5 top-1/2 h-4 w-4 -translate-y-1/2 text-muted-foreground" />
            <Input v-model="store.keyword" placeholder="搜索名称 / serial / 型号" aria-label="搜索设备" class="pl-8" />
          </div>
          <Button variant="outline" size="icon" aria-label="刷新设备列表" @click="store.load(true)">
            <RefreshCw class="h-4 w-4" />
          </Button>

          <!-- 来源分类切换 -->
          <div class="flex items-center rounded-md border bg-muted/30 p-0.5 text-xs">
            <button
              class="rounded px-2.5 py-1 font-medium transition-colors"
              :class="store.sourceFilter === 'all' ? 'bg-background shadow-xs text-foreground' : 'text-muted-foreground hover:text-foreground'"
              @click="store.sourceFilter = 'all'"
            >
              全部
            </button>
            <button
              class="flex items-center gap-1 rounded px-2.5 py-1 font-medium transition-colors"
              :class="store.sourceFilter === 'auto' ? 'bg-background shadow-xs text-foreground' : 'text-muted-foreground hover:text-foreground'"
              @click="store.sourceFilter = 'auto'"
            >
              <Usb class="h-3 w-3" />
              自动发现
            </button>
            <button
              class="flex items-center gap-1 rounded px-2.5 py-1 font-medium transition-colors"
              :class="store.sourceFilter === 'manual' ? 'bg-background shadow-xs text-foreground' : 'text-muted-foreground hover:text-foreground'"
              @click="store.sourceFilter = 'manual'"
            >
              <Wifi class="h-3 w-3" />
              手动添加
            </button>
          </div>
        </div>

        <div class="flex items-center gap-3">
          <Button class="gap-1.5 shadow-sm" @click="openAddDialog">
            <Plus class="h-4 w-4" />
            添加网络设备
          </Button>
          <label v-if="isDevelopment" class="flex cursor-pointer items-center gap-2 text-sm text-muted-foreground">
            Mock
            <Switch :model-value="store.mockEnabled" aria-label="Mock 数据开关" @update:model-value="(v) => store.setMock(!!v)" />
          </label>
        </div>
      </div>

      <!-- 初次加载骨架 -->
      <div v-if="store.loading && store.devices.length === 0" class="grid grid-cols-1 gap-4 sm:grid-cols-2 lg:grid-cols-3">
        <Skeleton v-for="i in 6" :key="i" class="h-[92px] rounded-xl" />
      </div>

      <!-- 加载失败 -->
      <Empty v-else-if="store.error && store.devices.length === 0">
        <EmptyHeader>
          <EmptyMedia variant="icon"><Smartphone /></EmptyMedia>
          <EmptyTitle>设备列表加载失败</EmptyTitle>
          <EmptyDescription>{{ store.error }}</EmptyDescription>
        </EmptyHeader>
        <Button variant="outline" @click="store.load()">重新加载</Button>
      </Empty>

      <!-- 空设备 -->
      <Empty v-else-if="store.filtered.length === 0">
        <EmptyHeader>
          <EmptyMedia variant="icon"><Smartphone /></EmptyMedia>
          <EmptyTitle>{{ store.keyword ? '没有匹配的设备' : '未检测到设备' }}</EmptyTitle>
          <EmptyDescription>
            {{ store.keyword ? '换个关键词试试' : '请连接 USB 设备或点击上方「添加网络设备」手动录入' }}
          </EmptyDescription>
        </EmptyHeader>
      </Empty>

      <!-- 设备墙 -->
      <div v-else class="grid grid-cols-1 gap-4 sm:grid-cols-2 lg:grid-cols-3">
        <DeviceCard
          v-for="d in store.filtered"
          :key="d.serial"
          :device="d"
          @open-floating="openFloating"
        />
      </div>
    </main>

    <!-- 手动添加网络设备对话框 -->
    <Dialog v-model:open="isAddDialogOpen">
      <DialogContent class="sm:max-w-md">
        <DialogHeader>
          <DialogTitle class="flex items-center gap-2">
            <Globe class="h-5 w-5 text-primary" />
            手动添加网络设备
          </DialogTitle>
          <DialogDescription>
            输入 Android 设备的 IP 地址与端口，后台将持久化保存并自动执行 ADB 连接。
          </DialogDescription>
        </DialogHeader>

        <div class="space-y-4 py-2">
          <div class="grid grid-cols-3 gap-2">
            <div class="col-span-2 space-y-1.5">
              <label class="text-xs font-medium text-muted-foreground">设备 IP 地址</label>
              <Input
                v-model="addHost"
                placeholder="例如 192.168.1.100"
                :disabled="isAdding"
                @keydown.enter="handleAddSubmit"
              />
            </div>
            <div class="space-y-1.5">
              <label class="text-xs font-medium text-muted-foreground">端口</label>
              <Input
                v-model="addPort"
                placeholder="5555"
                :disabled="isAdding"
                @keydown.enter="handleAddSubmit"
              />
            </div>
          </div>

          <div class="space-y-1.5">
            <label class="text-xs font-medium text-muted-foreground">设备备注/别名 (可选)</label>
            <Input
              v-model="addAlias"
              placeholder="例如 客厅电视 / 测试机 A"
              :disabled="isAdding"
              @keydown.enter="handleAddSubmit"
            />
          </div>

          <div v-if="addError" class="text-xs text-destructive">
            {{ addError }}
          </div>

          <div class="rounded-lg bg-muted/50 p-3 text-xs text-muted-foreground leading-relaxed">
            提示：请确保目标 Android 设备已开启「开发者选项」中的「无线调试」或在 5555 端口开启 TCP/IP 调试。
          </div>
        </div>

        <DialogFooter class="gap-2 sm:gap-0">
          <Button variant="ghost" :disabled="isAdding" @click="isAddDialogOpen = false">
            取消
          </Button>
          <Button :disabled="isAdding" class="gap-1.5" @click="handleAddSubmit">
            <Loader2 v-if="isAdding" class="h-4 w-4 animate-spin" />
            <CheckCircle2 v-else class="h-4 w-4" />
            {{ isAdding ? '正在连接...' : '保存并连接' }}
          </Button>
        </DialogFooter>
      </DialogContent>
    </Dialog>

    <!-- 独立悬浮小窗（支持同时打开多个独立小窗，无遮罩，可拖拽缩放） -->
    <FloatingPlayer
      v-for="(s, idx) in floatingWindows"
      :key="s"
      :serial="s"
      :z-index="focusedSerial === s ? 60 : 50 + idx"
      :offset-index="idx"
      @close="closeFloating(s)"
      @focus="focusFloating(s)"
    />
  </div>
</template>
