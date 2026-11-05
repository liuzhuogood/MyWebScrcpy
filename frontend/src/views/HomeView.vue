<script setup lang="ts">
import {
  Smartphone,
} from 'lucide-vue-next'
import { computed, defineAsyncComponent, onMounted, onUnmounted, ref, watch } from 'vue'
import { useRouter } from 'vue-router'
import { invoke, isTauri } from '@tauri-apps/api/core'
import AppHeader from '@/components/layout/AppHeader.vue'
import HomeAuroraBackground from '@/components/home/HomeAuroraBackground.vue'
import HomePhoneView from '@/components/home/HomePhoneView.vue'
import HomeIosDockBar from '@/components/home/HomeIosDockBar.vue'
import SystemConfigDialog from '@/components/system/SystemConfigDialog.vue'
import { Button } from '@/components/ui/button'
import AddDeviceDialog from '@/components/home/AddDeviceDialog.vue'
import { useDeviceStore } from '@/stores/devices'
import { useHomePhoneView } from '@/composables/useHomePhoneView'
import { useDeviceSelection } from '@/composables/useDeviceSelection'
import { useDeviceBatch } from '@/composables/useDeviceBatch'
import {
  type HomeViewMode,
  normalizeHomeViewMode,
  usePlayerSettings,
} from '@/composables/usePlayerSettings'
import { useGroupControl } from '@/composables/useGroupControl'
import { toast } from 'vue-sonner'

type ViewMode = HomeViewMode

const router = useRouter()
const store = useDeviceStore()
const playerSettings = usePlayerSettings()
const isConfigOpen = ref(false)

// 投屏小窗按需加载，避免首页首屏加载完整投屏器代码
const FloatingPlayer = defineAsyncComponent(() => import('@/components/player/FloatingPlayer.vue'))

const VIEW_MODE_KEY = 'mywebscrcpy.view_mode'
const viewMode = ref<ViewMode>(normalizeHomeViewMode(localStorage.getItem(VIEW_MODE_KEY)))

function setViewMode(mode: ViewMode) {
  viewMode.value = mode
  localStorage.setItem(VIEW_MODE_KEY, mode)
}

const onlineCount = computed(() => {
  return store.devices.filter(d => d.state === 'device').length
})

const homeFilteredDevices = computed(() => {
  return store.filtered.filter(d => d.state === 'device')
})

// 手机视图及三视图状态管理
const {
  dashSize,
  serialToNumberMap,
  visibleDevices,
  showAllDevices,
  showOnlySelected,
  hideDevice,
  hiddenSerials,
  restoreHiddenDevices,
} = useHomePhoneView(() => homeFilteredDevices.value, () => store.numberMap)

// 当前选择范围（三视图严格统一仅允许操作可见设备，隐藏设备不接受新动作）
const currentScopeDevices = computed(() => visibleDevices.value)

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
  [() => store.keyword, viewMode],
  () => {
    clearSelection()
  },
)

function handleUpdateSelectedSerials(serials: string[]) {
  const selectableSet = new Set(selectableDevices.value.map(d => d.serial))
  selectedSerials.value = serials.filter(s => selectableSet.has(s))
}

// 点击设备卡片：优先打开原生小窗（桌面端），Web 端使用内嵌浮动小窗
const floatingSerials = ref<string[]>([])

async function handleOpenFloating(serial: string) {
  if (store.mockEnabled) {
    toast.info('模拟设备仅用于预览')
    return
  }
  const device = store.devices.find(d => d.serial === serial)
  if (!device || device.state !== 'device') {
    toast.error('设备当前不在线，无法打开投屏小窗')
    return
  }

  if (isTauri()) {
    try {
      const size = playerSettings.floatingSize.value
      const targetW = size === 'compact' ? 320 : size === 'large' ? 440 : 380
      const baseH = size === 'compact' ? 680 : size === 'large' ? 880 : 780
      await invoke('open_device_window', {
        serial,
        title: store.displayName(device),
        width: targetW,
        height: baseH,
      })
      return
    }
    catch (e) {
      console.error('[Desktop] 打开原生投屏小窗失败', { serial, error: e })
      toast.error('打开投屏小窗失败，请稍后重试')
      return
    }
  }

  if (!floatingSerials.value.includes(serial))
    floatingSerials.value = [...floatingSerials.value, serial]
}

function closeFloatingWindow(serial: string) {
  floatingSerials.value = floatingSerials.value.filter(item => item !== serial)
}

function handleOpenAi() {
  router.push('/ai-integration')
}

function handleOpenShortcuts() {
  toast.info('快捷键指南：双击投屏全屏，Esc 退出；Option/Alt + P 恢复小窗')
}

function handleOpenDocs() {
  window.open('/api-docs.html', '_blank')
}

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

async function handleBatchAction(action: 'back' | 'home' | 'app_switch' | 'lock') {
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
  if (action === 'lock') void store.load(true)
}

// 群控同步操作状态
const groupControl = useGroupControl()
const isSyncActive = groupControl.isActive

function handleToggleSync() {
  if (store.mockEnabled) {
    toast.error('当前处于 Mock 模拟状态，无法启用群控')
    return
  }
  if (selectedSerials.value.length === 0) {
    toast.error('请先勾选需要参与群控的设备')
    return
  }
  const next = !groupControl.isActive.value
  groupControl.setActive(next)
  groupControl.setTargets(selectedSerials.value)
  if (next) {
    toast.success(`已开启群控：触控操作任意已选设备将实时同步至其余 ${Math.max(0, selectedSerials.value.length - 1)} 台设备`)
  }
  else {
    toast.info('已关闭群控')
  }
}

watch(selectedSerials, (serials) => {
  groupControl.setTargets(serials)
  if (serials.length === 0 && groupControl.isActive.value) {
    groupControl.setActive(false)
    toast.info('已选择设备已清空，群控已自动关闭')
  }
})

function handleReorder(serials: string[]) {
  try { store.reorder(serials) } catch (cause) { toast.error(cause instanceof Error ? cause.message : '排序失败') }
}
async function handleDeviceAction(action: string, serial: string) {
  const device = store.devices.find(d => d.serial === serial)
  if (!device) return
  if (action === 'hide') { hideDevice(serial); return }
  if (action === 'open') { await handleOpenFloating(serial); return }
  if (action === 'detail') { if (!store.mockEnabled) router.push({path:'/player',query:{serial}}); return }
  if (action === 'delete' && device.source === 'manual' && !store.mockEnabled) {
    try { await store.removeDevice(serial) }
    catch (cause) { console.error('[devices] Remove failed', {serial,cause}); toast.error(cause instanceof Error ? cause.message : '删除失败') }
  }
}
const isAddDialogOpen = ref(false)
function openAddDialog() { isAddDialogOpen.value = true }

onMounted(() => store.startPolling())
onUnmounted(() => {
  store.stopPolling()
  destroyBatch()
})
</script>

<template>
  <div class="home-shell relative flex h-dvh min-h-0 flex-col overflow-hidden text-slate-900 dark:text-slate-100 select-none">
    <!-- 极光背景 -->
    <HomeAuroraBackground />

    <!-- 顶部应用导航栏 -->
    <AppHeader glass class="shrink-0" />

    <!-- 主工作区：单列铺满主视口，底部预留悬浮底栏安全边距 -->
    <div class="home-content relative z-10 min-h-0 w-full min-w-0 flex-1 overflow-y-auto overscroll-contain">
      <main class="flex min-h-full w-full min-w-0 flex-col px-4 pt-4 pb-24 sm:px-6">
        <!-- 设备卡片展示主区：100% 视口铺展 -->
        <div class="flex min-h-0 w-full flex-1 flex-col">
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
            <div class="flex h-16 w-16 items-center justify-center rounded-2xl bg-gradient-to-tr from-blue-600 to-cyan-500 text-white border border-white/20 mb-4 shadow-lg shadow-blue-500/20">
              <Smartphone class="h-8 w-8 text-white" />
            </div>
            <h3 class="text-lg font-bold text-slate-900 dark:text-white">
              {{ store.keyword ? '没有找到匹配的设备' : '未检测到在线设备' }}
            </h3>
            <p class="mt-1.5 text-xs text-slate-500 dark:text-slate-400 max-w-md">
              {{ store.keyword ? '请尝试更换搜索关键字' : '请通过 USB 数据线连接 Android 设备，或点击「连接设备」输入 IP 端口' }}
            </p>
            <div class="mt-6 flex items-center gap-3">
              <Button
                size="sm"
                class="rounded-2xl bg-blue-600 text-white font-medium hover:bg-blue-700 px-4 text-xs shadow-md"
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

          <HomePhoneView
            v-else-if="viewMode === 'deck' || viewMode === 'phone'"
            :view-mode="viewMode"
            :visible-devices="visibleDevices"
            :dash-size="dashSize"
            :serial-to-number-map="serialToNumberMap"
            :selected-serials="selectedSerials"
            :mock-enabled="store.mockEnabled"
            @toggle-select="toggleSelect"
            @update:selected-serials="handleUpdateSelectedSerials"
            @open-floating="handleOpenFloating"
            @show-all="restoreHiddenDevices(); showAllDevices()"
            @reorder="handleReorder"
            @device-action="handleDeviceAction"
          />
        </div>
      </main>
    </div>

    <!-- Web 端内嵌投屏小窗 -->
    <FloatingPlayer
      v-for="(floatingSerial, index) in floatingSerials"
      :key="floatingSerial"
      :serial="floatingSerial"
      :offset-index="index"
      :z-index="60 + index"
      @close="closeFloatingWindow(floatingSerial)"
    />

    <!-- iOS 风格贴底居中悬浮控制栏 -->
    <HomeIosDockBar
      :keyword="store.keyword"
      :view-mode="viewMode"
      :devices="store.filtered"
      :number-map="store.numberMap"
      :hidden-count="hiddenSerials.length"
      :mock-enabled="store.mockEnabled"
      :selected-serials="selectedSerials"
      :online-count="onlineCount"
      :is-all-selected="isAllSelected"
      :batch-running="batchRunning"
      :current-action="currentAction"
      :batch-results="batchResults"
      :batch-summary="batchSummary"
      :is-sync-active="isSyncActive"
      :display-name="store.displayName"
      @update:keyword="store.keyword = $event"
      @update:view-mode="setViewMode"
      @update:selected-serials="handleUpdateSelectedSerials"
      @toggle-select="toggleSelect"
      @select-all="selectAll"
      @clear-selection="clearSelection"
      @trigger-action="handleBatchAction"
      @toggle-sync="handleToggleSync"
      @cancel-batch="cancelBatch"
      @clear-results="clearResults"
      @open-add="openAddDialog"
      @show-only-selected="showOnlySelected(selectedSerials)"
      @restore-hidden="restoreHiddenDevices"
      @show-all="showAllDevices"
      @device-action="handleDeviceAction"
      @open-config="isConfigOpen = true"
      @open-ai="handleOpenAi"
      @open-shortcuts="handleOpenShortcuts"
      @open-docs="handleOpenDocs"
    />

    <!-- 系统配置弹窗 -->
    <SystemConfigDialog v-model:open="isConfigOpen" />

    <AddDeviceDialog v-model:open="isAddDialogOpen" />

  </div>
</template>
