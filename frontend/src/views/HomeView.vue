<script setup lang="ts">
import {
  CheckCircle2,
  Globe,
  Loader2,
  Smartphone,
} from 'lucide-vue-next'
import { computed, onMounted, onUnmounted, ref } from 'vue'
import AppHeader from '@/components/layout/AppHeader.vue'
import DeviceCard from '@/components/device/DeviceCard.vue'
import FloatingPlayer from '@/components/player/FloatingPlayer.vue'
import HomeAuroraBackground from '@/components/home/HomeAuroraBackground.vue'
import HomeControlCapsule from '@/components/home/HomeControlCapsule.vue'
import { Button } from '@/components/ui/button'
import {
  Dialog,
  DialogContent,
  DialogDescription,
  DialogFooter,
  DialogHeader,
  DialogTitle,
} from '@/components/ui/dialog'
import { Input } from '@/components/ui/input'
import { useDeviceStore } from '@/stores/devices'
import { toast } from 'vue-sonner'

const store = useDeviceStore()
const isDevelopment = import.meta.env.DEV
const floatingWindows = ref<string[]>([])
const focusedSerial = ref<string | null>(null)

// 视图布局：默认 3D 全息甲板视图 (deck)
const VIEW_MODE_KEY = 'mywebscrcpy.view_mode'
const viewMode = ref<'deck' | 'compact'>(
  (localStorage.getItem(VIEW_MODE_KEY) as 'deck' | 'compact') || 'deck',
)

function setViewMode(mode: 'deck' | 'compact') {
  viewMode.value = mode
  localStorage.setItem(VIEW_MODE_KEY, mode)
}

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

const onlineCount = computed(() => {
  return store.devices.filter(d => d.state === 'device').length
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
    toast.success('已添加网络设备')
  }
  catch (e) {
    const msg = e instanceof Error ? e.message : '添加设备失败'
    addError.value = msg
    toast.error(msg)
  }
  finally {
    isAdding.value = false
  }
}

onMounted(() => store.startPolling())
onUnmounted(() => store.stopPolling())
</script>

<template>
  <div class="relative min-h-screen text-slate-900 dark:text-slate-100 select-none pb-20 overflow-x-hidden">
    <!-- 极光背景（桌面端全透，浏览器自适应黑白主题） -->
    <HomeAuroraBackground />

    <!-- 顶部应用导航栏（高对比晶体按键） -->
    <AppHeader />

    <!-- 满屏宽度布局，完全填满窗口宽度，无居中留白约束 -->
    <main class="relative z-10 w-full px-4 sm:px-6 py-5 space-y-5">
      <!-- 悬浮设备控制中枢胶囊 -->
      <HomeControlCapsule
        :keyword="store.keyword"
        :source-filter="store.sourceFilter"
        :view-mode="viewMode"
        :total-count="store.devices.length"
        :online-count="onlineCount"
        :is-development="isDevelopment"
        :mock-enabled="store.mockEnabled"
        @update:keyword="store.keyword = $event"
        @update:source-filter="store.sourceFilter = $event"
        @update:view-mode="setViewMode"
        @refresh="store.load(true)"
        @open-add="openAddDialog"
        @toggle-mock="store.setMock($event)"
      />

      <!-- 初次加载骨架屏 -->
      <div
        v-if="store.loading && store.devices.length === 0"
        class="grid grid-cols-1 gap-5 md:grid-cols-2 xl:grid-cols-3 2xl:grid-cols-4"
      >
        <div
          v-for="i in 8"
          :key="i"
          class="h-[230px] rounded-3xl border border-slate-200/80 bg-white/60 p-6 backdrop-blur-xl animate-pulse space-y-4 dark:border-white/10 dark:bg-white/[0.02]"
        >
          <div class="flex items-center justify-between">
            <div class="h-6 w-24 rounded-full bg-slate-200 dark:bg-white/10" />
            <div class="h-5 w-16 rounded-full bg-slate-200 dark:bg-white/10" />
          </div>
          <div class="h-6 w-36 rounded-lg bg-slate-200 dark:bg-white/10" />
          <div class="h-4 w-48 rounded bg-slate-100 dark:bg-white/5" />
          <div class="h-16 rounded-2xl bg-slate-100 dark:bg-white/5" />
        </div>
      </div>

      <!-- 加载失败错误状态 -->
      <div
        v-else-if="store.error && store.devices.length === 0"
        class="flex flex-col items-center justify-center rounded-3xl border border-red-500/20 bg-red-50/80 p-12 text-center backdrop-blur-2xl dark:bg-red-950/20"
      >
        <div class="flex h-14 w-14 items-center justify-center rounded-2xl bg-red-500/15 text-red-600 dark:text-red-400 border border-red-500/30 mb-4">
          <Smartphone class="h-7 w-7" />
        </div>
        <h3 class="text-lg font-bold text-slate-900 dark:text-white">设备列表加载失败</h3>
        <p class="mt-1 text-xs text-red-600 dark:text-red-300 max-w-sm">{{ store.error }}</p>
        <Button
          variant="outline"
          class="mt-5 rounded-2xl border-slate-300 bg-white text-slate-900 hover:bg-slate-100 dark:border-white/20 dark:bg-white/10 dark:text-white"
          @click="store.load()"
        >
          重新尝试连接
        </Button>
      </div>

      <!-- 空设备检测状态 -->
      <div
        v-else-if="store.filtered.length === 0"
        class="flex flex-col items-center justify-center rounded-3xl border border-slate-200/80 bg-white/70 p-16 text-center backdrop-blur-2xl shadow-sm dark:border-white/10 dark:bg-white/[0.03]"
      >
        <div class="flex h-16 w-16 items-center justify-center rounded-2xl bg-gradient-to-tr from-violet-600 to-cyan-500 text-white border border-white/20 mb-4 shadow-lg shadow-violet-500/20">
          <Smartphone class="h-8 w-8 text-white" />
        </div>
        <h3 class="text-lg font-bold text-slate-900 dark:text-white">
          {{ store.keyword ? '没有找到匹配的设备' : '未检测到在线设备' }}
        </h3>
        <p class="mt-1.5 text-xs text-slate-500 dark:text-slate-400 max-w-md">
          {{ store.keyword ? '请尝试更换搜索关键字，或重置分类筛选' : '请通过 USB 数据线连接 Android 设备（并开启 USB 调试），或点击上方「连接设备」输入 IP 端口' }}
        </p>
        <div class="mt-6 flex items-center gap-3">
          <Button
            size="sm"
            class="rounded-2xl bg-gradient-to-r from-violet-600 to-indigo-600 text-white font-medium hover:opacity-95 px-4 text-xs shadow-md"
            @click="openAddDialog"
          >
            连接网络设备
          </Button>
          <Button
            variant="ghost"
            size="sm"
            class="rounded-2xl border border-slate-200/80 bg-white/80 text-slate-700 hover:bg-slate-100 hover:text-slate-900 dark:border-white/10 dark:bg-white/5 dark:text-slate-300 dark:hover:bg-white/10 dark:hover:text-white px-4 text-xs"
            @click="store.load(true)"
          >
            重新扫描
          </Button>
        </div>
      </div>

      <!-- 视图模式一：3D 全息设备甲板视图 (Deck View) 铺满全屏 -->
      <div
        v-else-if="viewMode === 'deck'"
        class="grid grid-cols-1 gap-5 md:grid-cols-2 xl:grid-cols-3 2xl:grid-cols-4"
      >
        <DeviceCard
          v-for="d in store.filtered"
          :key="d.serial"
          :device="d"
          view-mode="deck"
          @open-floating="openFloating"
        />
      </div>

      <!-- 视图模式二：紧凑型设备阵列视图 (Compact View) 铺满全屏 -->
      <div
        v-else
        class="grid grid-cols-1 gap-4 md:grid-cols-2 xl:grid-cols-3"
      >
        <DeviceCard
          v-for="d in store.filtered"
          :key="d.serial"
          :device="d"
          view-mode="compact"
          @open-floating="openFloating"
        />
      </div>
    </main>

    <!-- 手动添加网络设备对话框 -->
    <Dialog v-model:open="isAddDialogOpen">
      <DialogContent class="sm:max-w-md border-slate-200 bg-white/95 text-slate-900 dark:border-white/15 dark:bg-[#0e121d]/95 dark:text-white backdrop-blur-3xl shadow-2xl rounded-3xl">
        <DialogHeader>
          <DialogTitle class="flex items-center gap-2 text-slate-900 dark:text-white font-bold">
            <Globe class="h-5 w-5 text-cyan-600 dark:text-cyan-400" />
            连接网络 Android 设备
          </DialogTitle>
          <DialogDescription class="text-slate-500 dark:text-slate-400 text-xs">
            输入局域网 Android 设备的 IP 地址与端口，后台将持久化保存并自动建立 ADB 调试握手。
          </DialogDescription>
        </DialogHeader>

        <div class="space-y-4 py-2">
          <div class="grid grid-cols-3 gap-2">
            <div class="col-span-2 space-y-1.5">
              <label class="text-xs font-medium text-slate-700 dark:text-slate-300">设备 IP 地址</label>
              <Input
                v-model="addHost"
                placeholder="例如 192.168.1.100"
                class="rounded-xl border-slate-200 bg-slate-100/80 text-xs text-slate-900 placeholder:text-slate-400 focus:border-cyan-500 dark:border-white/15 dark:bg-black/40 dark:text-white dark:placeholder:text-slate-500 dark:focus:border-cyan-400"
                :disabled="isAdding"
                @keydown.enter="handleAddSubmit"
              />
            </div>
            <div class="space-y-1.5">
              <label class="text-xs font-medium text-slate-700 dark:text-slate-300">端口</label>
              <Input
                v-model="addPort"
                placeholder="5555"
                class="rounded-xl border-slate-200 bg-slate-100/80 text-xs text-slate-900 placeholder:text-slate-400 focus:border-cyan-500 dark:border-white/15 dark:bg-black/40 dark:text-white dark:placeholder:text-slate-500 dark:focus:border-cyan-400 font-mono"
                :disabled="isAdding"
                @keydown.enter="handleAddSubmit"
              />
            </div>
          </div>

          <div class="space-y-1.5">
            <label class="text-xs font-medium text-slate-700 dark:text-slate-300">设备别名 / 备注 (可选)</label>
            <Input
              v-model="addAlias"
              placeholder="例如 客厅大屏 / 测试平板"
              class="rounded-xl border-slate-200 bg-slate-100/80 text-xs text-slate-900 placeholder:text-slate-400 focus:border-cyan-500 dark:border-white/15 dark:bg-black/40 dark:text-white dark:placeholder:text-slate-500 dark:focus:border-cyan-400"
              :disabled="isAdding"
              @keydown.enter="handleAddSubmit"
            />
          </div>

          <div v-if="addError" class="text-xs text-red-600 bg-red-50 p-2.5 rounded-xl border border-red-200 dark:text-red-400 dark:bg-red-950/30 dark:border-red-500/20">
            {{ addError }}
          </div>

          <div class="rounded-2xl border border-slate-200/80 bg-slate-50/80 p-3 text-xs text-slate-500 leading-relaxed dark:border-white/10 dark:bg-white/[0.03] dark:text-slate-400">
            提示：请确保目标 Android 设备已开启「开发者选项」中的「无线调试」或在 5555 端口开启 TCP/IP 调试网络。
          </div>
        </div>

        <DialogFooter class="gap-2 sm:gap-0">
          <Button
            variant="ghost"
            class="rounded-xl border border-slate-200/80 bg-white text-slate-700 hover:bg-slate-100 dark:border-white/10 dark:bg-white/5 dark:text-slate-300 dark:hover:bg-white/10 dark:hover:text-white"
            :disabled="isAdding"
            @click="isAddDialogOpen = false"
          >
            取消
          </Button>
          <Button
            :disabled="isAdding"
            class="rounded-xl bg-gradient-to-r from-violet-600 to-cyan-600 text-white font-medium hover:opacity-95 shadow-md"
            @click="handleAddSubmit"
          >
            <Loader2 v-if="isAdding" class="mr-1.5 h-4 w-4 animate-spin" />
            <CheckCircle2 v-else class="mr-1.5 h-4 w-4" />
            {{ isAdding ? '正在连接...' : '保存并握手' }}
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
