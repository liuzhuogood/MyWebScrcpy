<script setup lang="ts">
import { ref, watch } from 'vue'
import { Download, Loader2, Monitor, Moon, RefreshCw, Sun, Trash2 } from 'lucide-vue-next'
import { Badge } from '@/components/ui/badge'
import { Button } from '@/components/ui/button'
import { Input } from '@/components/ui/input'
import { Checkbox } from '@/components/ui/checkbox'
import { Dialog, DialogContent, DialogDescription, DialogFooter, DialogHeader, DialogTitle } from '@/components/ui/dialog'
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from '@/components/ui/select'
import { Tabs, TabsContent, TabsList, TabsTrigger } from '@/components/ui/tabs'
import { theme } from '@/composables/useTheme'
import { useDependencies, type DependencyId, type ManagedTool, type Selection } from '@/composables/useDependencies'
import { useDesktopConfig } from '@/composables/useDesktopConfig'
import PythonModulesPanel from './PythonModulesPanel.vue'

const open = defineModel<boolean>('open', { default: false })
const tab = ref('theme')
const w = useDependencies()
const desktopConfig = useDesktopConfig()
const { status, error, message, loading, saving, pending, checking, checks } = w
const uninstallTarget = ref<ManagedTool>()
const licenseOpen = ref(false)
const licenseAccepted = ref(false)
const themes = [{ id: 'auto' as const, name: '系统', icon: Monitor }, { id: 'light' as const, name: '明亮', icon: Sun }, { id: 'dark' as const, name: '黑暗', icon: Moon }]
watch(open, async (value) => {
  if (value) {
    void desktopConfig.load()
    await w.load()
    if (open.value) w.startPolling()
  } else {
    w.stopPolling()
  }
})
function change(id: DependencyId, key: keyof Selection, value: unknown) {
  if (typeof value !== 'string') return
  if (key === 'source' && value !== 'system' && value !== 'builtin') return
  Object.assign(w.selection(id), { [key]: value })
  delete checks.value[id]; message.value = ''
  if (key === 'source') void w.detect(id)
}
function install(tool: ManagedTool) {
  if (tool.id === 'adb') { licenseAccepted.value = false; licenseOpen.value = true }
  else void w.action(tool.id, '/api/dependencies/install', 'POST', { id: tool.id, accept_license: false })
}
async function uninstall() {
  const tool = uninstallTarget.value
  uninstallTarget.value = undefined
  if (tool) await w.action(tool.id, `/api/dependencies/installed/${tool.id}`, 'DELETE')
}
function result(tool: ManagedTool) {
  if (checks.value[tool.id]) return checks.value[tool.id]
  return JSON.stringify(w.selection(tool.id)) === JSON.stringify(tool.selection) ? tool.effective : undefined
}
function label(tool: ManagedTool) {
  if (w.runtimeBusy(tool)) return ({ downloading: '正在下载', verifying: '正在校验', publishing: '正在安装', removing: '正在卸载' } as Record<string, string>)[tool.task.stage]
  if (tool.installed) return '已安装'
  if (tool.residual) return '安装文件需要清理'
  return tool.task.stage === 'failed' ? '安装失败' : '未安装'
}
</script>

<template>
  <Dialog v-model:open="open">
    <DialogContent class="flex max-h-[90dvh] flex-col gap-0 overflow-hidden p-0 sm:max-w-2xl" :aria-describedby="undefined">
      <DialogHeader class="shrink-0 border-b px-5 py-4"><DialogTitle>系统配置</DialogTitle></DialogHeader>
      <Tabs v-model="tab" class="flex min-h-0 flex-1 flex-col gap-0">
        <div class="shrink-0 px-5 pt-4"><TabsList class="grid w-full grid-cols-3"><TabsTrigger value="theme">主题</TabsTrigger><TabsTrigger value="service">服务</TabsTrigger><TabsTrigger value="dependencies">依赖</TabsTrigger></TabsList></div>
        <div class="min-h-0 overflow-y-auto px-5 py-5">
          <TabsContent value="theme" class="m-0 min-h-56 space-y-4">
            <p class="text-sm font-medium">外观</p>
            <div class="grid grid-cols-3 gap-3" role="group" aria-label="主题">
              <Button v-for="item in themes" :key="item.id" variant="outline" class="h-24 flex-col gap-3" :class="theme === item.id ? 'border-primary bg-primary/5 text-primary' : ''" :aria-pressed="theme === item.id" @click="theme = item.id"><component :is="item.icon" class="size-6" />{{ item.name }}</Button>
            </div>
          </TabsContent>
          <TabsContent value="service" class="m-0 space-y-4">
            <div class="space-y-3 rounded-lg border p-4">
              <div class="flex items-center justify-between gap-3">
                <div>
                  <label for="desktop-port" class="text-sm font-medium">服务端口</label>
                  <p class="mt-1 text-xs text-muted-foreground">桌面内部服务与外部调用的本地回环端口（默认 1989）</p>
                </div>
                <Badge variant="outline">{{ desktopConfig.config.value?.effective ? `运行中 · ${desktopConfig.config.value.effective}` : '默认 1989' }}</Badge>
              </div>
              <div class="space-y-2">
                <Input
                  id="desktop-port"
                  v-model="desktopConfig.draftPort.value"
                  type="number"
                  min="1"
                  max="65535"
                  autocomplete="off"
                  :disabled="desktopConfig.saving.value || desktopConfig.config.value?.is_overridden"
                  class="w-40"
                />
                <p v-if="desktopConfig.config.value?.is_overridden" class="text-xs text-muted-foreground">
                  受环境变量覆盖 ({{ desktopConfig.config.value?.env_port }})
                </p>
                <p class="text-xs text-muted-foreground">重启生效</p>
              </div>
            </div>
          </TabsContent>
          <TabsContent value="dependencies" class="m-0 space-y-3">
            <DialogDescription class="text-xs">内置依赖按需下载，保存在应用数据目录。</DialogDescription>
            <p v-if="loading" role="status" class="text-sm text-muted-foreground">正在检测依赖…</p>
            <template v-if="status">
              <div class="flex items-center justify-between gap-3 rounded-lg border p-4"><div><p class="text-sm font-medium">scrcpy-server.jar</p><p class="mt-1 text-xs text-muted-foreground">设备投屏 · 4.0</p></div><Badge variant="secondary">随应用内置</Badge></div>
              <div v-for="tool in status.managed" :key="tool.id" class="space-y-3 rounded-lg border p-4">
                <div class="flex items-center justify-between gap-3">
                  <div><label :for="`${tool.id}-source`" class="text-sm font-medium">{{ tool.name }}</label><p class="mt-1 text-xs text-muted-foreground">{{ tool.id === 'adb' ? '设备连接与控制' : tool.id === 'python' ? '自动化脚本' : 'AI 助手' }}</p></div>
                  <Select :model-value="w.selection(tool.id).source" :disabled="w.busy(tool) || Boolean(w.acting.value) || Boolean(checking) || saving" @update:model-value="value => change(tool.id, 'source', value)"><SelectTrigger :id="`${tool.id}-source`" class="w-28" :aria-label="`${tool.name} 来源`"><SelectValue /></SelectTrigger><SelectContent><SelectItem value="builtin">内置</SelectItem><SelectItem value="system">系统</SelectItem></SelectContent></Select>
                </div>
                <template v-if="w.selection(tool.id).source === 'system'">
                  <div class="flex items-center justify-between gap-2">
                    <p class="min-w-0 break-all text-xs" :class="result(tool)?.installed ? 'text-muted-foreground' : 'text-destructive'">{{ result(tool)?.path || '填写路径或点击检测自动查找' }}</p>
                    <Button variant="ghost" size="sm" :disabled="pending || saving" :aria-label="`检测 ${tool.name}`" @click="w.detect(tool.id)"><RefreshCw class="size-3.5" :class="{ 'animate-spin': checking === tool.id }" />检测</Button>
                  </div>
                  <div class="space-y-2"><label :for="`${tool.id}-path`" class="text-xs text-muted-foreground">自定义执行路径（留空自动查找）</label><Input :id="`${tool.id}-path`" :model-value="w.selection(tool.id).path" autocomplete="off" :disabled="pending || saving" @update:model-value="value => change(tool.id, 'path', value)" /></div>
                  <p v-if="result(tool)?.installed" role="status" class="text-xs text-muted-foreground">可用 · {{ result(tool)?.version }}</p>
                  <p v-else-if="result(tool)?.error" class="text-xs text-destructive">{{ result(tool)?.error }}</p>
                </template>
                <div v-if="w.selection(tool.id).source === 'builtin'" class="flex items-center justify-between gap-3">
                  <div class="flex flex-wrap items-center gap-2"><Badge variant="outline">{{ label(tool) }}</Badge><span class="text-xs text-muted-foreground">{{ tool.version }}</span></div>
                  <Button v-if="w.runtimeBusy(tool) && tool.task.stage === 'downloading'" variant="ghost" size="sm" @click="w.action(tool.id, `/api/dependencies/install/${tool.id}`, 'DELETE')">取消</Button>
                  <Loader2 v-else-if="w.runtimeBusy(tool)" class="size-4 animate-spin" />
                  <Button v-else-if="tool.installed || tool.residual" variant="ghost" size="sm" class="text-destructive hover:text-destructive" :aria-label="`卸载内置 ${tool.name}`" :disabled="pending || saving" @click="uninstallTarget = tool"><Trash2 class="size-3.5" />卸载</Button>
                  <Button v-else-if="tool.supported" variant="outline" size="sm" :disabled="pending || saving" @click="install(tool)"><Download class="size-3.5" />{{ tool.task.stage === 'failed' ? '重试' : '安装' }}</Button>
                  <span v-else class="text-xs text-muted-foreground">请使用系统依赖</span>
                </div>
                <div v-if="w.selection(tool.id).source === 'builtin' && w.runtimeBusy(tool)" class="space-y-2"><div class="flex justify-between text-xs text-muted-foreground"><span v-if="tool.task.received > 0">{{ (tool.task.received / 1024 / 1024).toFixed(1) }} MiB</span><span v-if="tool.task.total > 0">{{ Math.min(100, Math.floor(tool.task.received / tool.task.total * 100)) }}%</span></div><div role="progressbar" :aria-label="`${tool.name} 安装进度`" :aria-valuenow="tool.task.total > 0 ? Math.floor(tool.task.received / tool.task.total * 100) : undefined" aria-valuemin="0" aria-valuemax="100" class="h-1.5 overflow-hidden rounded-full bg-muted"><div class="h-full bg-primary" :class="{ 'w-1/3 animate-pulse': tool.task.total <= 0 }" :style="tool.task.total > 0 ? { width: `${Math.min(100, tool.task.received / tool.task.total * 100)}%` } : undefined" /></div></div>
                <p v-if="w.selection(tool.id).source === 'builtin' && tool.task.error" role="alert" class="text-xs text-destructive">{{ tool.task.error }}</p>
                <PythonModulesPanel v-if="tool.id === 'python'" :runtime="result(tool)" :task="tool.modules_task" :disabled="Boolean(pending) || saving" @install="w.action('python', '/api/dependencies/python/modules', 'POST', { selection: w.selection('python') })" @cancel="w.action('python', '/api/dependencies/python/modules', 'DELETE')" />
              </div>
            </template>
          </TabsContent>
        </div>
      </Tabs>
      <footer class="flex shrink-0 items-center gap-3 border-t px-5 py-3">
        <p v-if="tab === 'dependencies' && error" role="alert" class="flex-1 text-xs text-destructive">{{ error }}</p>
        <p v-else-if="tab === 'dependencies' && message" role="status" class="flex-1 text-xs text-muted-foreground">{{ message }}</p>
        <p v-else-if="tab === 'service' && desktopConfig.error.value" role="alert" class="flex-1 text-xs text-destructive">{{ desktopConfig.error.value }}</p>
        <p v-else-if="tab === 'service' && desktopConfig.message.value" role="status" class="flex-1 text-xs text-muted-foreground">{{ desktopConfig.message.value }}</p>
        <div class="ml-auto flex items-center gap-2">
          <Button variant="ghost" size="sm" @click="open = false">关闭</Button>
          <Button v-if="tab === 'dependencies'" size="sm" :disabled="!status || loading || pending || saving" @click="w.save">
            <Loader2 v-if="saving" class="size-3.5 animate-spin" />保存
          </Button>
          <Button v-else-if="tab === 'service'" size="sm" :disabled="desktopConfig.saving.value || desktopConfig.config.value?.is_overridden" @click="desktopConfig.save">
            <Loader2 v-if="desktopConfig.saving.value" class="size-3.5 animate-spin" />保存
          </Button>
        </div>
      </footer>
    </DialogContent>
  </Dialog>
  <Dialog :open="Boolean(uninstallTarget)" @update:open="value => { if (!value) uninstallTarget = undefined }"><DialogContent class="sm:max-w-sm"><DialogHeader><DialogTitle>卸载内置 {{ uninstallTarget?.name }}</DialogTitle><DialogDescription>将删除应用安装的依赖文件。{{ uninstallTarget?.selection.source === 'builtin' ? '重新安装后才能使用相关功能。' : '不会删除系统安装的依赖。' }}</DialogDescription></DialogHeader><DialogFooter class="gap-2"><Button variant="outline" @click="uninstallTarget = undefined">取消</Button><Button variant="destructive" @click="uninstall">卸载</Button></DialogFooter></DialogContent></Dialog>
  <Dialog v-model:open="licenseOpen"><DialogContent class="sm:max-w-md"><DialogHeader><DialogTitle>安装 ADB</DialogTitle><DialogDescription>下载并安装 Android SDK Platform-Tools 36.0.2。</DialogDescription></DialogHeader><a href="https://developer.android.com/studio/terms" target="_blank" rel="noopener" class="text-sm underline">阅读 Android SDK 许可协议</a><label class="flex items-center gap-2 text-sm"><Checkbox :model-value="licenseAccepted" @update:model-value="value => licenseAccepted = value === true" />我已阅读并同意许可协议</label><DialogFooter><Button variant="outline" @click="licenseOpen = false">取消</Button><Button :disabled="!licenseAccepted" @click="licenseOpen = false; w.action('adb', '/api/dependencies/install', 'POST', { id: 'adb', accept_license: true })">安装</Button></DialogFooter></DialogContent></Dialog>
</template>
