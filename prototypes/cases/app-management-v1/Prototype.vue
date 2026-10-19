<script setup lang="ts">
import { computed, ref } from 'vue'
import {
  AlertTriangle,
  Download,
  HardDriveUpload,
  Layers,
  MoreHorizontal,
  Package,
  Play,
  RotateCcw,
  Search,
  Settings2,
  Smartphone,
  Square,
  Trash2,
  Upload,
  Wifi,
} from 'lucide-vue-next'
import { toast } from 'vue-sonner'
import { Toaster } from '@/components/ui/sonner'
import { Badge } from '@/components/ui/badge'
import { Button } from '@/components/ui/button'
import {
  DropdownMenu,
  DropdownMenuContent,
  DropdownMenuItem,
  DropdownMenuSeparator,
  DropdownMenuTrigger,
} from '@/components/ui/dropdown-menu'
import {
  AlertDialog,
  AlertDialogAction,
  AlertDialogCancel,
  AlertDialogContent,
  AlertDialogDescription,
  AlertDialogFooter,
  AlertDialogHeader,
  AlertDialogTitle,
} from '@/components/ui/alert-dialog'
import { Empty, EmptyDescription, EmptyHeader, EmptyMedia, EmptyTitle } from '@/components/ui/empty'
import { Input } from '@/components/ui/input'
import {
  Table,
  TableBody,
  TableCell,
  TableHead,
  TableHeader,
  TableRow,
} from '@/components/ui/table'
import { Tabs, TabsList, TabsTrigger } from '@/components/ui/tabs'
import AppDetailDialog from './AppDetailDialog.vue'
import AppInstallDialog from './AppInstallDialog.vue'
import { initialApps } from './mockData'
import type { AppItem, DragFileCandidate } from './types'

// 当前设备与模式
const serial = ref('192.168.1.20:5555')
const deviceModel = ref('Google Pixel 9')
const activeView = ref<'drag-install' | 'app-management' | 'split-view'>('drag-install')

// 应用数据状态
const apps = ref<AppItem[]>([...initialApps])
const searchQuery = ref('')
const selectedType = ref<'all' | 'user' | 'system'>('user')

// 拖拽与安装弹窗状态
const isDraggingOver = ref(false)
const installDialogOpen = ref(false)
const candidateFile = ref<DragFileCandidate | null>(null)

// 详情弹窗
const detailDialogOpen = ref(false)
const selectedApp = ref<AppItem | null>(null)

// 卸载确认弹窗
const uninstallTarget = ref<AppItem | null>(null)

// 计算过滤应用列表
const filteredApps = computed(() => {
  return apps.value.filter((app) => {
    if (selectedType.value !== 'all' && app.type !== selectedType.value) {
      return false
    }
    if (searchQuery.value.trim()) {
      const q = searchQuery.value.trim().toLowerCase()
      return app.name.toLowerCase().includes(q) || app.packageName.toLowerCase().includes(q)
    }
    return true
  })
})

const userAppsCount = computed(() => apps.value.filter(a => a.type === 'user').length)
const systemAppsCount = computed(() => apps.value.filter(a => a.type === 'system').length)

// 格式化大小
function formatBytes(bytes: number) {
  const mb = bytes / (1024 * 1024)
  if (mb >= 1) return `${mb.toFixed(1)} MB`
  return `${(bytes / 1024).toFixed(1)} KB`
}

// 拖拽投屏相关
function onDragOver(e: DragEvent) {
  e.preventDefault()
  if (e.dataTransfer) {
    e.dataTransfer.dropEffect = 'copy'
  }
  isDraggingOver.value = true
}

function onDragLeave(e: DragEvent) {
  e.preventDefault()
  isDraggingOver.value = false
}

function onDrop(e: DragEvent) {
  e.preventDefault()
  isDraggingOver.value = false
  const files = e.dataTransfer?.files
  if (!files || files.length === 0) return

  const file = files[0]
  if (!file.name.toLowerCase().endsWith('.apk')) {
    toast.error('仅支持拖拽 .apk 安装包文件')
    return
  }

  candidateFile.value = {
    name: file.name,
    size: file.size,
  }
  installDialogOpen.value = true
}

// 快捷模拟拖入
function triggerMockDrop(name: string, sizeMB: number) {
  candidateFile.value = {
    name,
    size: sizeMB * 1024 * 1024,
  }
  installDialogOpen.value = true
}

function triggerInvalidDrop() {
  toast.error('拦截非 APK 文件：仅支持拖拽 .apk 安装包')
}

// 处理安装成功写回
function handleInstalled(appInfo: { name: string, packageName: string, sizeBytes: number }) {
  const exists = apps.value.find(a => a.packageName === appInfo.packageName)
  if (exists) {
    exists.updateTime = '刚刚'
    exists.status = 'enabled'
    toast.success(`应用已覆盖安装成功：${appInfo.name}`)
  }
  else {
    apps.value.unshift({
      packageName: appInfo.packageName,
      name: appInfo.name,
      iconColor: 'bg-emerald-600',
      type: 'user',
      status: 'enabled',
      versionName: '1.0.0',
      versionCode: 1,
      sizeBytes: appInfo.sizeBytes,
      apkPath: `/data/app/~~newApp==/${appInfo.packageName}/base.apk`,
      installTime: '刚刚',
      updateTime: '刚刚',
      minSdk: 26,
      targetSdk: 34,
      permissions: ['android.permission.INTERNET', 'android.permission.ACCESS_NETWORK_STATE'],
    })
    toast.success(`应用安装成功：${appInfo.name}`)
  }
}

// 应用操作
function launchApp(pkg: string) {
  const item = apps.value.find(a => a.packageName === pkg)
  toast.success(`已向设备发送启动指令：${item ? item.name : pkg}`)
}

function stopApp(pkg: string) {
  const item = apps.value.find(a => a.packageName === pkg)
  toast.info(`已强制停止应用：${item ? item.name : pkg}`)
}

function clearData(pkg: string) {
  const item = apps.value.find(a => a.packageName === pkg)
  toast.success(`已清除 ${item?.name || pkg} 的应用缓存与用户数据`)
}

function exportApk(pkg: string) {
  const item = apps.value.find(a => a.packageName === pkg)
  toast.info(`正在导出 ${item?.name || pkg} 的 APK 安装包...`)
}

function toggleStatus(pkg: string) {
  const item = apps.value.find(a => a.packageName === pkg)
  if (!item) return
  if (item.status === 'enabled') {
    item.status = 'disabled'
    toast.info(`已停用应用：${item.name}`)
  }
  else {
    item.status = 'enabled'
    toast.success(`已重新启用应用：${item.name}`)
  }
}

function openDetail(app: AppItem) {
  selectedApp.value = app
  detailDialogOpen.value = true
}

function requestUninstall(app: AppItem) {
  uninstallTarget.value = app
}

function confirmUninstall() {
  if (!uninstallTarget.value) return
  const target = uninstallTarget.value
  apps.value = apps.value.filter(a => a.packageName !== target.packageName)
  toast.success(`应用 ${target.name} 已成功卸载`)
  uninstallTarget.value = null
  if (selectedApp.value?.packageName === target.packageName) {
    detailDialogOpen.value = false
  }
}
</script>

<template>
  <div class="space-y-6">
    <Toaster position="bottom-center" />

    <!-- 顶部状态栏与视角切换 -->
    <header class="flex flex-wrap items-center justify-between gap-3 rounded-xl border bg-card p-4 shadow-sm">
      <div class="flex items-center gap-3">
        <div class="flex size-10 items-center justify-center rounded-lg bg-primary/10 text-primary">
          <Smartphone class="size-5" />
        </div>
        <div>
          <div class="flex items-center gap-2">
            <h1 class="text-base font-semibold">{{ deviceModel }}</h1>
            <Badge variant="secondary" class="gap-1 text-xs">
              <Wifi class="size-3" />在线
            </Badge>
          </div>
          <p class="font-mono text-xs text-muted-foreground">{{ serial }}</p>
        </div>
      </div>

      <!-- 视图模式切换 -->
      <div class="flex items-center gap-1 rounded-lg border bg-muted/40 p-1">
        <Button
          size="sm"
          :variant="activeView === 'drag-install' ? 'default' : 'ghost'"
          class="h-8 gap-1.5 text-xs"
          @click="activeView = 'drag-install'"
        >
          <HardDriveUpload class="size-3.5" />
          <span>投屏操控</span>
        </Button>
        <Button
          size="sm"
          :variant="activeView === 'app-management' ? 'default' : 'ghost'"
          class="h-8 gap-1.5 text-xs"
          @click="activeView = 'app-management'"
        >
          <Package class="size-3.5" />
          <span>应用管理</span>
        </Button>
        <Button
          size="sm"
          :variant="activeView === 'split-view' ? 'default' : 'ghost'"
          class="h-8 gap-1.5 text-xs"
          @click="activeView = 'split-view'"
        >
          <Layers class="size-3.5" />
          <span>抽屉内嵌</span>
        </Button>
      </div>
    </header>

    <!-- 视图 1：投屏操控（支持直接拖入 APK） -->
    <div v-if="activeView === 'drag-install'" class="flex flex-col items-center justify-center py-2">
      <div class="w-full max-w-sm space-y-3">
        <!-- 模拟手机外壳与投屏画面 -->
        <div
          class="relative aspect-[9/19.5] w-full overflow-hidden rounded-[2.5rem] border-4 border-neutral-800 bg-neutral-950 p-3 shadow-2xl transition-all"
          :class="{ 'ring-4 ring-primary ring-offset-2': isDraggingOver }"
          @dragover="onDragOver"
          @dragleave="onDragLeave"
          @drop="onDrop"
        >
          <!-- 手机顶部刘海 / 药丸挖孔 -->
          <div class="absolute left-1/2 top-4 h-4 w-20 -translate-x-1/2 rounded-full bg-neutral-800" />

          <!-- 手机顶栏状态 -->
          <div class="flex items-center justify-between px-3 pt-4 text-[11px] font-medium text-neutral-400">
            <span>12:00</span>
            <div class="flex items-center gap-1.5">
              <Wifi class="size-3" />
              <span>88%</span>
            </div>
          </div>

          <!-- 手机主屏幕内容 -->
          <div class="mt-8 flex flex-col items-center justify-between h-[82%] px-2">
            <!-- 桌面时钟小部件 -->
            <div class="text-center space-y-1">
              <p class="text-3xl font-light tracking-tight text-neutral-100">12:00</p>
              <p class="text-xs text-neutral-400">10月20日 星期二 · 晴 22℃</p>
            </div>

            <!-- 桌面图标网格 -->
            <div class="grid w-full grid-cols-4 gap-4 px-2">
              <div
                v-for="app in apps.filter(a => a.type === 'user').slice(0, 8)"
                :key="app.packageName"
                class="flex flex-col items-center gap-1 cursor-pointer transition-transform hover:scale-105"
                @click="launchApp(app.packageName)"
              >
                <div
                  class="flex size-11 items-center justify-center rounded-2xl text-white font-semibold text-sm shadow-md"
                  :class="app.iconColor"
                >
                  {{ app.name.slice(0, 1) }}
                </div>
                <span class="truncate w-12 text-center text-[10px] text-neutral-300">{{ app.name }}</span>
              </div>
            </div>

            <!-- 底部 Dock 栏 -->
            <div class="flex w-full justify-around rounded-3xl bg-white/10 p-2.5 backdrop-blur-md">
              <div class="flex size-10 items-center justify-center rounded-xl bg-emerald-500 text-white shadow">
                <span class="text-xs font-bold">电</span>
              </div>
              <div class="flex size-10 items-center justify-center rounded-xl bg-blue-500 text-white shadow">
                <span class="text-xs font-bold">信</span>
              </div>
              <div class="flex size-10 items-center justify-center rounded-xl bg-amber-500 text-white shadow">
                <span class="text-xs font-bold">览</span>
              </div>
              <div class="flex size-10 items-center justify-center rounded-xl bg-cyan-600 text-white shadow">
                <span class="text-xs font-bold">相</span>
              </div>
            </div>
          </div>

          <!-- 拖拽高亮覆盖层 -->
          <div
            v-if="isDraggingOver"
            class="absolute inset-0 z-50 flex flex-col items-center justify-center bg-primary/20 backdrop-blur-sm p-6 text-center animate-in fade-in"
          >
            <div class="flex size-14 items-center justify-center rounded-2xl bg-primary text-primary-foreground shadow-lg animate-bounce">
              <Upload class="size-7" />
            </div>
            <p class="mt-3 text-sm font-semibold text-white">松开以安装应用</p>
          </div>
        </div>

        <!-- 极简辅助选项 -->
        <div class="flex items-center justify-center gap-2 pt-1">
          <Button
            variant="ghost"
            size="sm"
            class="h-7 px-2 text-[11px] text-muted-foreground hover:text-foreground"
            @click="triggerMockDrop('weixin_v8.0.48.apk', 256)"
          >
            测试拖入微信
          </Button>
          <Button
            variant="ghost"
            size="sm"
            class="h-7 px-2 text-[11px] text-muted-foreground hover:text-foreground"
            @click="triggerMockDrop('bilibili_v7.72.apk', 118)"
          >
            测试拖入哔哩哔哩
          </Button>
          <Button
            variant="ghost"
            size="sm"
            class="h-7 px-2 text-[11px] text-muted-foreground hover:text-foreground"
            @click="triggerInvalidDrop"
          >
            测试非 APK
          </Button>
        </div>
      </div>
    </div>

    <!-- 视图 2：应用管理全屏页 -->
    <div v-else-if="activeView === 'app-management'" class="space-y-4">
      <!-- 汇总卡片 -->
      <div class="grid grid-cols-2 gap-3 sm:grid-cols-4">
        <div class="rounded-xl border bg-card p-3.5 shadow-sm space-y-1">
          <span class="text-xs text-muted-foreground">全部应用总数</span>
          <p class="text-2xl font-bold font-mono">{{ apps.length }}</p>
        </div>
        <div class="rounded-xl border bg-card p-3.5 shadow-sm space-y-1">
          <div class="flex items-center justify-between">
            <span class="text-xs text-muted-foreground">用户安装应用</span>
            <Badge variant="default" class="text-[10px] h-4 px-1.5">核心管理</Badge>
          </div>
          <p class="text-2xl font-bold font-mono text-primary">{{ userAppsCount }}</p>
        </div>
        <div class="rounded-xl border bg-card p-3.5 shadow-sm space-y-1">
          <div class="flex items-center justify-between">
            <span class="text-xs text-muted-foreground">系统内置应用</span>
            <Badge variant="secondary" class="text-[10px] h-4 px-1.5">安全保护</Badge>
          </div>
          <p class="text-2xl font-bold font-mono text-muted-foreground">{{ systemAppsCount }}</p>
        </div>
        <div class="rounded-xl border bg-card p-3.5 shadow-sm space-y-1">
          <span class="text-xs text-muted-foreground">已占用存储空间</span>
          <p class="text-2xl font-bold font-mono">1.18 GB</p>
        </div>
      </div>

      <!-- 搜索与分类 Tab -->
      <div class="flex flex-col gap-3 rounded-xl border bg-card p-4 shadow-sm sm:flex-row sm:items-center sm:justify-between">
        <Tabs :model-value="selectedType" class="w-full sm:w-auto" @update:model-value="selectedType = $event as any">
          <TabsList class="grid w-full grid-cols-3 sm:w-auto">
            <TabsTrigger value="user" class="gap-1.5 text-xs">
              <span>用户应用</span>
              <Badge variant="secondary" class="text-[10px] px-1 h-4 font-mono">{{ userAppsCount }}</Badge>
            </TabsTrigger>
            <TabsTrigger value="system" class="gap-1.5 text-xs">
              <span>系统应用</span>
              <Badge variant="secondary" class="text-[10px] px-1 h-4 font-mono">{{ systemAppsCount }}</Badge>
            </TabsTrigger>
            <TabsTrigger value="all" class="gap-1.5 text-xs">
              <span>全部</span>
              <Badge variant="secondary" class="text-[10px] px-1 h-4 font-mono">{{ apps.length }}</Badge>
            </TabsTrigger>
          </TabsList>
        </Tabs>

        <div class="flex items-center gap-2">
          <div class="relative w-full sm:w-72">
            <Search class="absolute left-2.5 top-1/2 size-4 -translate-y-1/2 text-muted-foreground" />
            <Input
              v-model="searchQuery"
              class="h-9 pl-8 text-xs"
              placeholder="搜索应用名称或包名..."
            />
          </div>
          <Button
            size="sm"
            variant="outline"
            class="h-9 gap-1 text-xs shrink-0"
            @click="triggerMockDrop('new_upload.apk', 45)"
          >
            <Upload class="size-3.5" />
            <span>安装 APK</span>
          </Button>
        </div>
      </div>

      <!-- 应用表格列表 -->
      <div class="rounded-xl border bg-card shadow-sm overflow-hidden">
        <Table>
          <TableHeader>
            <TableRow class="bg-muted/30">
              <TableHead class="w-[320px] text-xs">应用与包名</TableHead>
              <TableHead class="text-xs">类型</TableHead>
              <TableHead class="text-xs">状态</TableHead>
              <TableHead class="text-xs">版本 / 体积</TableHead>
              <TableHead class="text-xs">更新时间</TableHead>
              <TableHead class="text-right text-xs">操作</TableHead>
            </TableRow>
          </TableHeader>
          <TableBody>
            <TableRow v-for="app in filteredApps" :key="app.packageName" class="hover:bg-muted/40">
              <!-- 应用名与包名 -->
              <TableCell>
                <div class="flex items-center gap-3">
                  <div
                    class="flex size-9 shrink-0 items-center justify-center rounded-xl text-white font-semibold text-xs shadow-sm"
                    :class="app.iconColor"
                  >
                    {{ app.name.slice(0, 1) }}
                  </div>
                  <div class="min-w-0 space-y-0.5">
                    <p class="text-xs font-medium truncate text-foreground">{{ app.name }}</p>
                    <p class="text-[11px] font-mono text-muted-foreground truncate">{{ app.packageName }}</p>
                  </div>
                </div>
              </TableCell>

              <!-- 应用类型区分 -->
              <TableCell>
                <Badge
                  :variant="app.type === 'user' ? 'default' : 'secondary'"
                  class="text-[11px] font-normal"
                >
                  {{ app.type === 'user' ? '用户应用' : '系统内置' }}
                </Badge>
              </TableCell>

              <!-- 状态 -->
              <TableCell>
                <span class="inline-flex items-center gap-1.5 text-xs">
                  <span
                    class="size-1.5 rounded-full"
                    :class="app.status === 'enabled' ? 'bg-emerald-500' : 'bg-muted-foreground'"
                  />
                  <span :class="app.status === 'enabled' ? 'text-foreground' : 'text-muted-foreground'">
                    {{ app.status === 'enabled' ? '正常启用' : '已停用' }}
                  </span>
                </span>
              </TableCell>

              <!-- 版本与体积 -->
              <TableCell class="font-mono text-xs">
                <div>v{{ app.versionName }}</div>
                <div class="text-[11px] text-muted-foreground">{{ formatBytes(app.sizeBytes) }}</div>
              </TableCell>

              <!-- 更新时间 -->
              <TableCell class="text-xs text-muted-foreground">
                {{ app.updateTime }}
              </TableCell>

              <!-- 操作列 -->
              <TableCell class="text-right">
                <div class="flex items-center justify-end gap-1">
                  <Button
                    variant="ghost"
                    size="sm"
                    class="h-7 px-2 text-xs gap-1 text-emerald-600 hover:text-emerald-700"
                    title="启动"
                    @click="launchApp(app.packageName)"
                  >
                    <Play class="size-3.5 fill-current" />
                    <span>启动</span>
                  </Button>
                  <Button
                    variant="ghost"
                    size="sm"
                    class="h-7 px-2 text-xs"
                    @click="openDetail(app)"
                  >
                    <span>详情</span>
                  </Button>

                  <!-- 更多菜单 -->
                  <DropdownMenu>
                    <DropdownMenuTrigger as-child>
                      <Button variant="ghost" size="icon" class="size-7">
                        <MoreHorizontal class="size-3.5" />
                      </Button>
                    </DropdownMenuTrigger>
                    <DropdownMenuContent align="end" class="w-36 text-xs">
                      <DropdownMenuItem @click="stopApp(app.packageName)">
                        <Square class="size-3.5 mr-2" />
                        <span>强行停止</span>
                      </DropdownMenuItem>
                      <DropdownMenuItem @click="clearData(app.packageName)">
                        <RotateCcw class="size-3.5 mr-2" />
                        <span>清除数据</span>
                      </DropdownMenuItem>
                      <DropdownMenuItem @click="exportApk(app.packageName)">
                        <Download class="size-3.5 mr-2" />
                        <span>导出 APK</span>
                      </DropdownMenuItem>
                      <DropdownMenuSeparator />
                      <DropdownMenuItem @click="toggleStatus(app.packageName)">
                        <span>{{ app.status === 'enabled' ? '停用应用' : '启用应用' }}</span>
                      </DropdownMenuItem>
                      <DropdownMenuItem
                        class="text-destructive focus:text-destructive"
                        @click="requestUninstall(app)"
                      >
                        <Trash2 class="size-3.5 mr-2" />
                        <span>卸载应用</span>
                      </DropdownMenuItem>
                    </DropdownMenuContent>
                  </DropdownMenu>
                </div>
              </TableCell>
            </TableRow>
          </TableBody>
        </Table>

        <div v-if="filteredApps.length === 0" class="py-12 text-center">
          <Empty>
            <EmptyHeader>
              <EmptyMedia><Package class="size-10 text-muted-foreground/50" /></EmptyMedia>
              <EmptyTitle>没有找到匹配的应用</EmptyTitle>
              <EmptyDescription>请尝试更换搜索关键字或切换分类标签页。</EmptyDescription>
            </EmptyHeader>
          </Empty>
        </div>
      </div>
    </div>

    <!-- 视图 3：投屏 + 抽屉联动模式 (对齐现有 DevicePlayer 模式) -->
    <div v-else-if="activeView === 'split-view'" class="rounded-xl border bg-card shadow-sm overflow-hidden">
      <div class="flex items-center justify-between border-b px-4 py-2.5 bg-muted/30">
        <div class="flex items-center gap-2 text-xs font-medium">
          <Smartphone class="size-4 text-primary" />
          <span>投屏操控与右侧抽屉内嵌应用管理面板 (Embedded Mode)</span>
        </div>
        <span class="text-xs text-muted-foreground">与文件管理器 (FilesView) 架构对称</span>
      </div>

      <div class="grid lg:grid-cols-12 min-h-[580px]">
        <!-- 左侧：投屏画面 -->
        <div class="lg:col-span-7 flex flex-col items-center justify-center p-6 bg-neutral-950 border-r border-border/40">
          <div
            class="relative aspect-[9/18] w-64 overflow-hidden rounded-3xl border-2 border-neutral-800 bg-neutral-900 shadow-xl"
            @dragover="onDragOver"
            @dragleave="onDragLeave"
            @drop="onDrop"
          >
            <div class="flex items-center justify-between px-3 pt-3 text-[10px] text-neutral-400">
              <span>12:00</span>
              <Wifi class="size-3" />
            </div>
            <div class="flex flex-col items-center justify-center h-[80%] text-center p-4 space-y-3">
              <Package class="size-12 text-primary/80" />
              <p class="text-xs text-neutral-200">投屏流已连接</p>
              <p class="text-[11px] text-neutral-400">支持拖入 APK 安装</p>
              <Button size="sm" variant="secondary" class="h-7 text-xs" @click="triggerMockDrop('demo.apk', 30)">
                模拟拖入
              </Button>
            </div>
          </div>
        </div>

        <!-- 右侧：内嵌抽屉应用列表 -->
        <div class="lg:col-span-5 flex flex-col bg-background p-4 space-y-3 overflow-hidden">
          <div class="flex items-center justify-between border-b pb-2">
            <div class="flex items-center gap-1.5 font-medium text-xs">
              <Package class="size-4 text-primary" />
              <span>应用管理</span>
            </div>
            <div class="flex items-center gap-1">
              <Button
                size="sm"
                variant="ghost"
                class="h-7 text-xs"
                :class="{ 'bg-muted': selectedType === 'user' }"
                @click="selectedType = 'user'"
              >
                用户
              </Button>
              <Button
                size="sm"
                variant="ghost"
                class="h-7 text-xs"
                :class="{ 'bg-muted': selectedType === 'system' }"
                @click="selectedType = 'system'"
              >
                系统
              </Button>
            </div>
          </div>

          <Input v-model="searchQuery" class="h-8 text-xs" placeholder="快速过滤..." />

          <div class="flex-1 overflow-y-auto space-y-2 pr-1 max-h-[460px]">
            <div
              v-for="app in filteredApps"
              :key="app.packageName"
              class="flex items-center justify-between rounded-lg border p-2 text-xs hover:bg-muted/40 transition-colors"
            >
              <div class="flex items-center gap-2 min-w-0">
                <div
                  class="flex size-7 shrink-0 items-center justify-center rounded-lg text-white font-semibold text-[10px]"
                  :class="app.iconColor"
                >
                  {{ app.name.slice(0, 1) }}
                </div>
                <div class="min-w-0">
                  <p class="font-medium truncate">{{ app.name }}</p>
                  <p class="text-[10px] text-muted-foreground truncate font-mono">{{ app.packageName }}</p>
                </div>
              </div>
              <div class="flex items-center gap-1 shrink-0">
                <Button variant="ghost" size="icon" class="size-6" title="启动" @click="launchApp(app.packageName)">
                  <Play class="size-3 text-emerald-500 fill-current" />
                </Button>
                <Button variant="ghost" size="icon" class="size-6" title="详情" @click="openDetail(app)">
                  <Settings2 class="size-3" />
                </Button>
              </div>
            </div>
          </div>
        </div>
      </div>
    </div>

    <!-- 弹窗组件：安装确认与进度 -->
    <AppInstallDialog
      v-model:open="installDialogOpen"
      :file="candidateFile"
      :serial="serial"
      :device-model="deviceModel"
      @installed="handleInstalled"
      @launch="launchApp"
    />

    <!-- 弹窗组件：应用详情 -->
    <AppDetailDialog
      v-model:open="detailDialogOpen"
      :app="selectedApp"
      @launch="launchApp"
      @stop="stopApp"
      @clear="clearData"
      @export="exportApk"
      @toggle-status="toggleStatus"
      @uninstall="requestUninstall"
    />

    <!-- 卸载二次确认弹窗 -->
    <AlertDialog :open="uninstallTarget !== null" @update:open="(val) => { if (!val) uninstallTarget = null }">
      <AlertDialogContent>
        <AlertDialogHeader>
          <AlertDialogTitle class="flex items-center gap-2 text-base">
            <AlertTriangle v-if="uninstallTarget?.type === 'system'" class="size-5 text-amber-500" />
            <span>确认卸载 {{ uninstallTarget?.name }}？</span>
          </AlertDialogTitle>
          <AlertDialogDescription class="text-xs space-y-2">
            <template v-if="uninstallTarget?.type === 'system'">
              <span class="block text-destructive font-medium">
                ⚠️ 注意：该应用为系统内置应用（{{ uninstallTarget.packageName }}），卸载可能导致系统功能异常或设备不稳定！
              </span>
              <span class="block text-muted-foreground">
                建议优先使用【停用】功能。若继续卸载，将执行系统级用户分区卸载。
              </span>
            </template>
            <template v-else>
              <span>将从设备完全移除该应用程序及相关运行缓存（{{ uninstallTarget?.packageName }}）。</span>
            </template>
          </AlertDialogDescription>
        </AlertDialogHeader>
        <AlertDialogFooter>
          <AlertDialogCancel>取消</AlertDialogCancel>
          <AlertDialogAction
            class="bg-destructive text-destructive-foreground hover:bg-destructive/90"
            @click="confirmUninstall"
          >
            确认卸载
          </AlertDialogAction>
        </AlertDialogFooter>
      </AlertDialogContent>
    </AlertDialog>
  </div>
</template>
