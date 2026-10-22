<script setup lang="ts">
import {
  CheckCircle2,
  CheckSquare,
  EyeOff,
  Globe,
  Loader2,
  Smartphone,
  Square,
} from 'lucide-vue-next'
import { computed, onMounted, onUnmounted, ref, watch } from 'vue'
import { useFullscreen } from '@vueuse/core'
import AppHeader from '@/components/layout/AppHeader.vue'
import DeviceCard from '@/components/device/DeviceCard.vue'
import FloatingPlayer from '@/components/player/FloatingPlayer.vue'
import HomeAuroraBackground from '@/components/home/HomeAuroraBackground.vue'
import HomeControlCapsule from '@/components/home/HomeControlCapsule.vue'
import HomePhoneView from '@/components/home/HomePhoneView.vue'
import BatchOperationPanel from '@/components/home/BatchOperationPanel.vue'
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
import { useHomePhoneView } from '@/composables/useHomePhoneView'
import { useDeviceSelection } from '@/composables/useDeviceSelection'
import { useDeviceBatch } from '@/composables/useDeviceBatch'
import { useHomeFilter } from '@/composables/useHomeFilter'
import { toast } from 'vue-sonner'

type ViewMode = 'deck' | 'compact' | 'phone'

const store = useDeviceStore()
const isDevelopment = import.meta.env.DEV
const floatingWindows = ref<string[]>([])
const focusedSerial = ref<string | null>(null)

// 页面全屏控制
const { isFullscreen: isPageFs, toggle: togglePageFs } = useFullscreen()

// 视图布局：支持 deck | compact | phone
const VIEW_MODE_KEY = 'mywebscrcpy.view_mode'
const rawViewMode = localStorage.getItem(VIEW_MODE_KEY)
const viewMode = ref<ViewMode>(
  (rawViewMode === 'deck' || rawViewMode === 'compact' || rawViewMode === 'phone')
    ? rawViewMode
    : 'deck',
)

function setViewMode(mode: ViewMode) {
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

// 首页专属过滤：默认仅显示在线设备，开启后包含离线/未授权设备
const { showOffline, filterOnlineDevices } = useHomeFilter()
const homeFilteredDevices = computed(() => {
  return filterOnlineDevices(store.filtered, showOffline.value)
})

// 手机视图及三视图显隐状态管理
const {
  dashSize,
  hiddenSerials,
  serialToNumberMap,
  visibleDevices,
  toggleDeviceVisibility,
  showAllDevices,
  showOnlySelected,
  setDashSize,
} = useHomePhoneView(() => homeFilteredDevices.value)

// 批量面板折叠状态
const isBatchPanelOpen = ref(false)

// 当前选择范围（三视图严格统一仅允许操作可见设备，隐藏设备不接受新动作）
const currentScopeDevices = computed(() => {
  return visibleDevices.value
})

// 多选管理：传入 mockEnabled 响应式引用，Mock 打开时禁止选中
const isMockEnabledRef = computed(() => !!store.mockEnabled)
const {
  selectedSerials,
  selectableDevices,
  isAllSelected,
  toggleSelect,
  selectAll,
  clearSelection,
  getValidatedTargets,
} = useDeviceSelection(currentScopeDevices, isMockEnabledRef)

// 切换筛选、切换离线显示或切换视图时，清空选择，避免操作隐藏设备
watch(
  [() => store.keyword, () => store.sourceFilter, showOffline, viewMode],
  () => {
    clearSelection()
  },
)

// 批量调度管理：传入派发前实时在线复查回调
function checkTargetOperable(serial: string): boolean {
  if (store.mockEnabled)
    return false
  const target = currentScopeDevices.value.find(d => d.serial === serial)
  return target?.state === 'device'
}

const {
  isRunning: batchRunning,
  currentAction,
  results: batchResults,
  summary: batchSummary,
  runBatch,
  cancelBatch,
  clearResults,
  destroy: destroyBatch,
} = useDeviceBatch(checkTargetOperable)

async function handleBatchAction(action: 'back' | 'home' | 'app_switch') {
  if (store.mockEnabled) {
    toast.error('当前处于 Mock 模拟状态，无法执行真机批量操作')
    return
  }

  // 发起前重校验目标，排除隐藏/已掉线设备
  const validTargets = getValidatedTargets()
  if (validTargets.length === 0 || batchRunning.value)
    return

  const deviceMap: Record<string, string> = {}
  for (const d of store.devices) {
    deviceMap[d.serial] = store.displayName(d)
  }

  await runBatch(validTargets, action, deviceMap)
}

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
onUnmounted(() => {
  store.stopPolling()
  destroyBatch()
})
</script>

<template>
  <div class="relative min-h-screen text-slate-900 dark:text-slate-100 select-none pb-20 overflow-x-hidden flex flex-col">
    <!-- 极光背景 -->
    <HomeAuroraBackground />

    <!-- 顶部应用导航栏 -->
    <AppHeader />

    <!-- 主工作区：单列铺满主视口，批量面板为独立浮动图层不占用宽度 -->
    <div class="relative z-10 flex-1 w-full min-w-0">
      <main class="w-full min-w-0 px-4 sm:px-6 py-5 space-y-5">
        <!-- 顶部搜索/筛选控制中枢：占满整行，不受右侧面板影响 -->
        <HomeControlCapsule
          :keyword="store.keyword"
          :source-filter="store.sourceFilter"
          :show-offline="showOffline"
          :view-mode="viewMode"
          :total-count="store.devices.length"
          :online-count="onlineCount"
          :is-development="isDevelopment"
          :mock-enabled="store.mockEnabled"
          :is-panel-open="isBatchPanelOpen"
          :selected-count="selectedSerials.length"
          @update:keyword="store.keyword = $event"
          @update:source-filter="store.sourceFilter = $event"
          @update:show-offline="showOffline = $event"
          @update:view-mode="setViewMode"
          @toggle-panel="isBatchPanelOpen = !isBatchPanelOpen"
          @refresh="store.load(true)"
          @open-add="openAddDialog"
          @toggle-mock="store.setMock($event)"
        />

        <!-- 主内容区域：左侧设备卡片区域 + 右侧预留批量面板位置（不遮挡卡片，桌面端并排预留；窄屏就近置顶） -->
        <div class="flex flex-col lg:flex-row items-start gap-5 w-full min-w-0">
          <!-- 批量操作面板（窄屏通过 order-first 就近置于上方避免长滚动，桌面端 lg:order-last 置于右侧预留空间且顶端对齐） -->
          <BatchOperationPanel
            v-if="isBatchPanelOpen"
            class="w-full lg:w-80 xl:w-88 shrink-0 order-first lg:order-last"
            :is-open="isBatchPanelOpen"
            :view-mode="viewMode"
            :devices="homeFilteredDevices"
            :hidden-serials="hiddenSerials"
            :selected-serials="selectedSerials"
            :display-name="store.displayName"
            :selected-count="selectedSerials.length"
            :is-all-selected="isAllSelected"
            :has-selectable="selectableDevices.length > 0"
            :batch-running="batchRunning"
            :current-action="currentAction"
            :batch-results="batchResults"
            :batch-summary="batchSummary"
            :dash-size="dashSize"
            :is-page-fullscreen="isPageFs"
            @update:is-open="isBatchPanelOpen = $event"
            @toggle-hide="toggleDeviceVisibility"
            @show-all="showAllDevices"
            @show-only-selected="showOnlySelected(selectedSerials)"
            @select-all="selectAll"
            @clear-selection="clearSelection"
            @trigger-action="handleBatchAction"
            @cancel-batch="cancelBatch"
            @clear-results="clearResults"
            @update:dash-size="setDashSize"
            @toggle-page-fullscreen="togglePageFs"
          />

          <!-- 设备卡片展示主区 -->
          <div class="flex-1 min-w-0 w-full space-y-5">
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
              v-else-if="homeFilteredDevices.length === 0"
              class="flex flex-col items-center justify-center rounded-3xl border border-slate-200/80 bg-white/70 p-16 text-center backdrop-blur-2xl shadow-sm dark:border-white/10 dark:bg-white/[0.03]"
            >
              <div class="flex h-16 w-16 items-center justify-center rounded-2xl bg-gradient-to-tr from-violet-600 to-cyan-500 text-white border border-white/20 mb-4 shadow-lg shadow-violet-500/20">
                <Smartphone class="h-8 w-8 text-white" />
              </div>
              <h3 class="text-lg font-bold text-slate-900 dark:text-white">
                {{ store.keyword ? '没有找到匹配的设备' : '未检测到在线设备' }}
              </h3>
              <p class="mt-1.5 text-xs text-slate-500 dark:text-slate-400 max-w-md">
                {{ store.keyword ? '请尝试更换搜索关键字，或重置分类筛选' : (showOffline ? '请通过 USB 数据线连接 Android 设备，或点击上方「连接设备」输入 IP 端口' : '当前无在线可用设备。可开启上方「显示离线」查看所有设备，或连接 USB 手机') }}
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

            <!-- 所有设备已隐藏状态（当有匹配设备但均被编号面板隐藏时，甲板与紧凑视图友好呈现恢复入口） -->
            <div
              v-else-if="visibleDevices.length === 0 && viewMode !== 'phone'"
              class="flex flex-col items-center justify-center rounded-3xl border border-dashed border-slate-200 dark:border-white/10 bg-white/40 dark:bg-white/[0.01] p-12 text-center"
            >
              <EyeOff class="h-8 w-8 text-slate-400 mb-2" />
              <h3 class="text-sm font-bold text-slate-900 dark:text-white">所有设备已隐藏</h3>
              <p class="mt-1 text-xs text-slate-500 dark:text-slate-400">可通过右侧面板点击「全显」或单机编号方块重新显示</p>
              <Button
                variant="outline"
                size="sm"
                class="mt-4 rounded-xl cursor-pointer"
                @click="showAllDevices"
              >
                全显
              </Button>
            </div>

            <!-- 视图模式三：手机视图 (Phone View) -->
            <HomePhoneView
              v-else-if="viewMode === 'phone'"
              :visible-devices="visibleDevices"
              :dash-size="dashSize"
              :serial-to-number-map="serialToNumberMap"
              :selected-serials="selectedSerials"
              :mock-enabled="store.mockEnabled"
              @toggle-select="toggleSelect"
              @show-all="showAllDevices"
            />

            <!-- 视图模式一：3D 全息设备甲板视图 (Deck View) 铺满全屏 -->
            <div
              v-else-if="viewMode === 'deck'"
              class="grid grid-cols-1 gap-5 md:grid-cols-2 xl:grid-cols-3 2xl:grid-cols-4"
            >
              <!-- 宿主层独立多选控件：三视图统一渲染可见设备 -->
              <div
                v-for="d in visibleDevices"
                :key="d.serial"
                class="group relative transition-all duration-150"
              >
                <!-- 宿主独立勾选按钮（左上角悬浮，点击不穿透，独立多选） -->
                <button
                  v-if="d.state === 'device' && !store.mockEnabled"
                  type="button"
                  class="absolute left-3 top-3 z-20 flex h-6 w-6 items-center justify-center rounded-lg border border-slate-200/80 bg-white/90 shadow-sm backdrop-blur-md transition-all cursor-pointer dark:border-white/15 dark:bg-slate-900/90 text-slate-400 hover:text-slate-900 dark:hover:text-white"
                  :class="{ '!border-violet-500 !bg-violet-600 !text-white': selectedSerials.includes(d.serial) }"
                  :title="selectedSerials.includes(d.serial) ? '取消勾选' : '勾选此设备进行批量操作'"
                  @click.stop="toggleSelect(d.serial)"
                >
                  <component :is="selectedSerials.includes(d.serial) ? CheckSquare : Square" class="h-3.5 w-3.5" />
                </button>

                <div :class="{ 'ring-2 ring-violet-500 rounded-3xl': selectedSerials.includes(d.serial) }">
                  <DeviceCard
                    :device="d"
                    view-mode="deck"
                    @open-floating="openFloating"
                  />
                </div>
              </div>
            </div>

            <!-- 视图模式二：紧凑型设备阵列视图 (Compact View) 铺满全屏 -->
            <div
              v-else
              class="grid grid-cols-1 gap-4 md:grid-cols-2 xl:grid-cols-3"
            >
              <!-- 宿主层独立多选控件：三视图统一渲染可见设备 -->
              <div
                v-for="d in visibleDevices"
                :key="d.serial"
                class="group relative transition-all duration-150"
              >
                <!-- 宿主独立勾选按钮（左上角悬浮，点击不穿透，独立多选） -->
                <button
                  v-if="d.state === 'device' && !store.mockEnabled"
                  type="button"
                  class="absolute left-3 top-3 z-20 flex h-6 w-6 items-center justify-center rounded-lg border border-slate-200/80 bg-white/90 shadow-sm backdrop-blur-md transition-all cursor-pointer dark:border-white/15 dark:bg-slate-900/90 text-slate-400 hover:text-slate-900 dark:hover:text-white"
                  :class="{ '!border-violet-500 !bg-violet-600 !text-white': selectedSerials.includes(d.serial) }"
                  :title="selectedSerials.includes(d.serial) ? '取消勾选' : '勾选此设备进行批量操作'"
                  @click.stop="toggleSelect(d.serial)"
                >
                  <component :is="selectedSerials.includes(d.serial) ? CheckSquare : Square" class="h-3.5 w-3.5" />
                </button>

                <div :class="{ 'ring-2 ring-violet-500 rounded-2xl': selectedSerials.includes(d.serial) }">
                  <DeviceCard
                    :device="d"
                    view-mode="compact"
                    @open-floating="openFloating"
                  />
                </div>
              </div>
            </div>
          </div>
        </div>
      </main>
    </div>

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

    <!-- 独立悬浮小窗 -->
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
