<script setup lang="ts">
import {
  CheckCircle2,
  Globe,
  Loader2,
  Pencil,
  Plus,
  RefreshCw,
  Search,
  Smartphone,
  Trash2,
  Usb,
  Wifi,
} from 'lucide-vue-next'
import { computed, nextTick, ref } from 'vue'
import type { ComponentPublicInstance } from 'vue'
import { Badge } from '@/components/ui/badge'
import { Button } from '@/components/ui/button'
import { Card, CardContent } from '@/components/ui/card'
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

export interface ProtoDevice {
  id: number
  serial: string
  source: 'auto' | 'manual'
  alias: string
  state: 'device' | 'offline' | 'connecting' | 'unauthorized'
  product: string
  model: string
  deviceName: string
  transportId: string
  autoReconnect: boolean
  lastConnectedAt?: number
}

// 模拟初始设备列表（包含自动发现与手动添加）
const mockDevices = ref<ProtoDevice[]>([
  {
    id: 1,
    serial: '10.0.0.104:5555',
    source: 'auto',
    alias: '',
    state: 'device',
    product: 'PD2072',
    model: 'V2072A',
    deviceName: 'PD2072',
    transportId: '1',
    autoReconnect: false,
    lastConnectedAt: Date.now() - 1000 * 60 * 5,
  },
  {
    id: 2,
    serial: 'RFCW10JXYZ',
    source: 'auto',
    alias: '测试备用机',
    state: 'device',
    product: 'Pixel 7',
    model: 'Pixel 7',
    deviceName: 'panther',
    transportId: '2',
    autoReconnect: false,
    lastConnectedAt: Date.now() - 1000 * 60 * 12,
  },
  {
    id: 3,
    serial: '192.168.1.188:5555',
    source: 'manual',
    alias: '客厅电视盒子',
    state: 'device',
    product: 'MiBOX4',
    model: 'MDZ-22-AB',
    deviceName: 'aquaman',
    transportId: '3',
    autoReconnect: true,
    lastConnectedAt: Date.now() - 1000 * 60 * 20,
  },
  {
    id: 4,
    serial: '192.168.1.199:5555',
    source: 'manual',
    alias: '二楼平板',
    state: 'offline',
    product: '',
    model: '',
    deviceName: '',
    transportId: '',
    autoReconnect: true,
    lastConnectedAt: 0,
  },
])

// 过滤与筛选
const keyword = ref('')
const sourceFilter = ref<'all' | 'auto' | 'manual'>('all')

const filteredDevices = computed(() => {
  return mockDevices.value.filter((d) => {
    if (sourceFilter.value !== 'all' && d.source !== sourceFilter.value)
      return false
    if (!keyword.value.trim())
      return true
    const kw = keyword.value.trim().toLowerCase()
    return (
      d.serial.toLowerCase().includes(kw)
      || d.alias.toLowerCase().includes(kw)
      || d.model.toLowerCase().includes(kw)
      || d.product.toLowerCase().includes(kw)
    )
  })
})

const countSummary = computed(() => {
  const total = mockDevices.value.length
  const online = mockDevices.value.filter(d => d.state === 'device').length
  const manual = mockDevices.value.filter(d => d.source === 'manual').length
  const auto = total - manual
  return `${online}/${total} 在线 (自动发现: ${auto}, 手动添加: ${manual})`
})

// 添加设备对话框状态
const isAddDialogOpen = ref(false)
const addHost = ref('')
const addPort = ref('5555')
const addAlias = ref('')
const addError = ref('')
const isConnecting = ref(false)

function openAddDialog() {
  addHost.value = ''
  addPort.value = '5555'
  addAlias.value = ''
  addError.value = ''
  isConnecting.value = false
  isAddDialogOpen.value = true
}

function handleAddSubmit() {
  const host = addHost.value.trim()
  const port = addPort.value.trim() || '5555'

  if (!host) {
    addError.value = '请输入设备 IP 或主机名'
    return
  }

  const serial = `${host}:${port}`
  if (mockDevices.value.some(d => d.serial === serial)) {
    addError.value = '该设备地址已存在'
    return
  }

  addError.value = ''
  isConnecting.value = true

  // 模拟后台自动 ADB 连接流程
  setTimeout(() => {
    isConnecting.value = false
    const newDev: ProtoDevice = {
      id: Date.now(),
      serial,
      source: 'manual',
      alias: addAlias.value.trim(),
      state: 'device', // 模拟连接成功
      product: 'WLAN-DEV',
      model: 'Android Device',
      deviceName: host,
      transportId: 'auto-' + Math.floor(Math.random() * 100),
      autoReconnect: true,
      lastConnectedAt: Date.now(),
    }
    mockDevices.value.unshift(newDev)
    isAddDialogOpen.value = false
  }, 1200)
}

// 模拟手动触发重连
function retryConnect(dev: ProtoDevice) {
  dev.state = 'connecting'
  setTimeout(() => {
    dev.state = 'device'
    dev.lastConnectedAt = Date.now()
  }, 1000)
}

// 删除手动设备
function removeDevice(dev: ProtoDevice) {
  const idx = mockDevices.value.findIndex(d => d.id === dev.id)
  if (idx !== -1) {
    mockDevices.value.splice(idx, 1)
  }
}

// 行内重命名编辑
const editingSerial = ref<string | null>(null)
const draftAlias = ref('')
const inputRef = ref<ComponentPublicInstance | null>(null)

async function startEdit(dev: ProtoDevice) {
  editingSerial.value = dev.serial
  draftAlias.value = dev.alias || dev.model || dev.serial
  await nextTick()
  const el = inputRef.value?.$el as HTMLInputElement | undefined
  el?.focus()
  el?.select()
}

function saveEdit(dev: ProtoDevice) {
  dev.alias = draftAlias.value.trim()
  editingSerial.value = null
}

function cancelEdit() {
  editingSerial.value = null
}

function displayName(dev: ProtoDevice): string {
  if (dev.alias)
    return dev.alias
  if (dev.model)
    return dev.model
  return dev.serial
}
</script>

<template>
  <div class="min-h-screen bg-background text-foreground">
    <!-- 顶部模拟导航栏 -->
    <header class="sticky top-0 z-10 border-b bg-background/80 backdrop-blur">
      <div class="flex h-14 w-full items-center justify-between px-4 sm:px-6">
        <div class="flex items-center gap-3">
          <div class="flex h-8 w-8 items-center justify-center rounded-lg bg-primary text-primary-foreground font-semibold">
            WS
          </div>
          <span class="font-semibold tracking-tight">MyWebScrcpy</span>
          <span class="rounded bg-muted px-2 py-0.5 text-xs text-muted-foreground">原型预览</span>
        </div>
        <div class="text-xs text-muted-foreground">
          {{ countSummary }}
        </div>
      </div>
    </header>

    <main class="mx-auto max-w-7xl px-4 py-6 sm:px-6">
      <!-- 操作与过滤栏 -->
      <div class="mb-6 flex flex-col gap-3 sm:flex-row sm:items-center sm:justify-between">
        <!-- 搜索与来源分类过滤 -->
        <div class="flex flex-1 flex-wrap items-center gap-2">
          <div class="relative w-full max-w-xs">
            <Search class="absolute left-2.5 top-1/2 h-4 w-4 -translate-y-1/2 text-muted-foreground" />
            <Input v-model="keyword" placeholder="搜索名称 / 序列号 / IP" class="pl-8" />
          </div>

          <div class="flex items-center rounded-md border bg-muted/30 p-0.5 text-xs">
            <button
              class="rounded px-2.5 py-1 font-medium transition-colors"
              :class="sourceFilter === 'all' ? 'bg-background shadow-xs text-foreground' : 'text-muted-foreground hover:text-foreground'"
              @click="sourceFilter = 'all'"
            >
              全部
            </button>
            <button
              class="flex items-center gap-1 rounded px-2.5 py-1 font-medium transition-colors"
              :class="sourceFilter === 'auto' ? 'bg-background shadow-xs text-foreground' : 'text-muted-foreground hover:text-foreground'"
              @click="sourceFilter = 'auto'"
            >
              <Usb class="h-3 w-3" />
              自动发现
            </button>
            <button
              class="flex items-center gap-1 rounded px-2.5 py-1 font-medium transition-colors"
              :class="sourceFilter === 'manual' ? 'bg-background shadow-xs text-foreground' : 'text-muted-foreground hover:text-foreground'"
              @click="sourceFilter = 'manual'"
            >
              <Wifi class="h-3 w-3" />
              手动添加
            </button>
          </div>
        </div>

        <!-- 手动添加设备按钮 -->
        <div class="flex items-center gap-2">
          <Button class="gap-1.5 shadow-sm" @click="openAddDialog">
            <Plus class="h-4 w-4" />
            添加网络设备
          </Button>
        </div>
      </div>

      <!-- 设备卡片网格 -->
      <div v-if="filteredDevices.length > 0" class="grid gap-4 sm:grid-cols-2 lg:grid-cols-3">
        <Card
          v-for="d in filteredDevices"
          :key="d.id"
          class="group relative transition-all hover:border-primary/50 hover:shadow-sm"
        >
          <CardContent class="p-4">
            <div class="flex items-start gap-3">
              <!-- 设备图标 -->
              <span
                class="flex h-10 w-10 shrink-0 items-center justify-center rounded-lg"
                :class="d.state === 'device' ? 'bg-primary/10 text-primary' : 'bg-muted text-muted-foreground'"
              >
                <Smartphone class="h-5 w-5" />
              </span>

              <!-- 设备主要信息 -->
              <div class="min-w-0 flex-1">
                <!-- 标题与编辑状态 -->
                <template v-if="editingSerial !== d.serial">
                  <div class="flex items-center gap-2">
                    <p class="truncate font-medium text-foreground">
                      {{ displayName(d) }}
                    </p>

                    <!-- 来源标签：自动发现 vs 手动添加 -->
                    <Badge
                      v-if="d.source === 'auto'"
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
                    {{ d.serial }}{{ d.product ? ` · ${d.product}` : '' }}
                  </p>

                  <!-- 状态与附加信息 -->
                  <div class="mt-2.5 flex items-center gap-2 text-xs">
                    <span v-if="d.state === 'device'" class="flex items-center gap-1 text-emerald-600 dark:text-emerald-400 font-medium">
                      <span class="h-1.5 w-1.5 rounded-full bg-emerald-500"></span>
                      在线
                    </span>
                    <span v-else-if="d.state === 'connecting'" class="flex items-center gap-1 text-blue-500">
                      <Loader2 class="h-3 w-3 animate-spin" />
                      连接中...
                    </span>
                    <span v-else class="flex items-center gap-1 text-muted-foreground">
                      <span class="h-1.5 w-1.5 rounded-full bg-muted-foreground/40"></span>
                      离线
                    </span>

                    <span v-if="d.source === 'manual' && d.autoReconnect" class="text-[11px] text-muted-foreground">
                      · 自动重连开启
                    </span>
                  </div>
                </template>

                <!-- 编辑别名 -->
                <template v-else>
                  <div class="flex gap-1.5">
                    <Input
                      ref="inputRef"
                      v-model="draftAlias"
                      maxlength="20"
                      placeholder="输入设备名称"
                      class="h-8"
                      @keydown.enter.prevent="saveEdit(d)"
                      @keydown.esc.prevent="cancelEdit"
                    />
                    <Button size="sm" @click="saveEdit(d)">保存</Button>
                    <Button size="sm" variant="ghost" @click="cancelEdit">取消</Button>
                  </div>
                </template>
              </div>

              <!-- 右侧快捷按钮 -->
              <div v-if="editingSerial !== d.serial" class="flex items-center gap-0.5">
                <Button
                  variant="ghost"
                  size="icon"
                  class="h-8 w-8 text-muted-foreground hover:text-foreground"
                  title="重命名设备"
                  @click.stop="startEdit(d)"
                >
                  <Pencil class="h-3.5 w-3.5" />
                </Button>

                <!-- 手动设备离线时支持重试连接 -->
                <Button
                  v-if="d.source === 'manual' && d.state === 'offline'"
                  variant="ghost"
                  size="icon"
                  class="h-8 w-8 text-muted-foreground hover:text-foreground"
                  title="重试连接"
                  @click.stop="retryConnect(d)"
                >
                  <RefreshCw class="h-3.5 w-3.5" />
                </Button>

                <!-- 手动设备支持从数据库移除 -->
                <Button
                  v-if="d.source === 'manual'"
                  variant="ghost"
                  size="icon"
                  class="h-8 w-8 text-muted-foreground hover:text-destructive"
                  title="删除此设备"
                  @click.stop="removeDevice(d)"
                >
                  <Trash2 class="h-3.5 w-3.5" />
                </Button>
              </div>
            </div>
          </CardContent>
        </Card>
      </div>

      <!-- 空状态 -->
      <Empty v-else class="my-12">
        <EmptyHeader>
          <EmptyMedia variant="icon"><Smartphone /></EmptyMedia>
          <EmptyTitle>{{ keyword ? '没有匹配的设备' : '暂无设备' }}</EmptyTitle>
          <EmptyDescription>
            {{ keyword ? '尝试更换搜索条件' : '可以通过 USB 连接设备，或点击上方「添加网络设备」手动录入' }}
          </EmptyDescription>
        </EmptyHeader>
      </Empty>
    </main>

    <!-- 添加设备模态框 Dialog -->
    <Dialog v-model:open="isAddDialogOpen">
      <DialogContent class="sm:max-w-md">
        <DialogHeader>
          <DialogTitle class="flex items-center gap-2">
            <Globe class="h-5 w-5 text-primary" />
            手动添加网络设备
          </DialogTitle>
          <DialogDescription>
            输入局域网 Android 设备的 IP 地址与端口，后台将持久化保存并自动执行 ADB 连结。
          </DialogDescription>
        </DialogHeader>

        <div class="space-y-4 py-2">
          <div class="grid grid-cols-3 gap-2">
            <div class="col-span-2 space-y-1.5">
              <label class="text-xs font-medium text-muted-foreground">设备 IP 地址</label>
              <Input
                v-model="addHost"
                placeholder="例如 192.168.1.100"
                :disabled="isConnecting"
                @keydown.enter="handleAddSubmit"
              />
            </div>
            <div class="space-y-1.5">
              <label class="text-xs font-medium text-muted-foreground">端口</label>
              <Input
                v-model="addPort"
                placeholder="5555"
                :disabled="isConnecting"
                @keydown.enter="handleAddSubmit"
              />
            </div>
          </div>

          <div class="space-y-1.5">
            <label class="text-xs font-medium text-muted-foreground">设备备注/别名 (可选)</label>
            <Input
              v-model="addAlias"
              placeholder="例如 客厅电视 / 自动化测试机 A"
              :disabled="isConnecting"
              @keydown.enter="handleAddSubmit"
            />
          </div>

          <div v-if="addError" class="text-xs text-destructive">
            {{ addError }}
          </div>

          <div class="rounded-lg bg-muted/50 p-3 text-xs text-muted-foreground leading-relaxed">
            提示：请确保目标 Android 设备已开启「开发者选项」中的「无线调试」或已在 5555 端口开启 TCP/IP 调试。
          </div>
        </div>

        <DialogFooter class="gap-2 sm:gap-0">
          <Button variant="ghost" :disabled="isConnecting" @click="isAddDialogOpen = false">
            取消
          </Button>
          <Button :disabled="isConnecting" class="gap-1.5" @click="handleAddSubmit">
            <Loader2 v-if="isConnecting" class="h-4 w-4 animate-spin" />
            <CheckCircle2 v-else class="h-4 w-4" />
            {{ isConnecting ? '正在连接...' : '保存并连接' }}
          </Button>
        </DialogFooter>
      </DialogContent>
    </Dialog>
  </div>
</template>
