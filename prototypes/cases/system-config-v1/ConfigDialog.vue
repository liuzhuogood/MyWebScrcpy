<script setup lang="ts">
import { Download, Loader2, Monitor, Moon, RefreshCw, Sun } from 'lucide-vue-next'
import { Button } from '@/components/ui/button'
import { Badge } from '@/components/ui/badge'
import { Input } from '@/components/ui/input'
import { Dialog, DialogContent, DialogDescription, DialogHeader, DialogTitle } from '@/components/ui/dialog'
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from '@/components/ui/select'
import { Tabs, TabsContent, TabsList, TabsTrigger } from '@/components/ui/tabs'
import { useSystemConfig } from './useSystemConfig'

const w = useSystemConfig()
const { open, tab, theme, tools, saved, message, formError } = w
defineExpose({ show: () => { open.value = true } })
</script>

<template>
  <Dialog v-model:open="open">
    <DialogContent class="flex max-h-[90dvh] flex-col gap-0 overflow-hidden p-0 sm:max-w-2xl" :aria-describedby="undefined">
      <DialogHeader class="shrink-0 border-b px-5 py-4">
        <DialogTitle class="text-base">系统配置</DialogTitle>
      </DialogHeader>
      <Tabs v-model="tab" class="flex min-h-0 flex-1 flex-col gap-0">
        <div class="shrink-0 px-5 pt-4">
          <TabsList class="grid w-full grid-cols-3">
            <TabsTrigger value="theme">主题</TabsTrigger>
            <TabsTrigger value="manage">依赖管理</TabsTrigger>
            <TabsTrigger value="config">依赖配置</TabsTrigger>
          </TabsList>
        </div>
        <div class="min-h-0 overflow-y-auto px-5 py-5">
          <TabsContent value="theme" class="m-0 min-h-56 space-y-4">
            <p class="text-sm font-medium">外观</p>
            <div class="grid grid-cols-3 gap-3" role="group" aria-label="主题">
              <Button v-for="item in [{ id: 'system' as const, name: '系统', icon: Monitor }, { id: 'light' as const, name: '明亮', icon: Sun }, { id: 'dark' as const, name: '黑暗', icon: Moon }]" :key="item.id" variant="outline" class="h-24 flex-col gap-3" :class="theme === item.id ? 'border-primary bg-primary/5 text-primary' : ''" :aria-pressed="theme === item.id" @click="theme = item.id">
                <component :is="item.icon" class="size-6" />{{ item.name }}
              </Button>
            </div>
          </TabsContent>
          <TabsContent value="manage" class="m-0 space-y-3">
            <DialogDescription class="mb-4 text-xs">内置依赖按需下载，保存在应用数据目录。</DialogDescription>
            <div class="flex items-center gap-3 rounded-lg border p-4">
              <div class="min-w-0 flex-1"><p class="text-sm font-medium">scrcpy-server.jar</p><p class="mt-1 text-xs text-muted-foreground">设备投屏 · 4.0</p></div>
              <Badge variant="secondary">随应用内置</Badge>
            </div>
            <div v-for="tool in tools" :key="tool.id" class="space-y-3 rounded-lg border p-4">
              <div class="flex items-center gap-3">
                <div class="min-w-0 flex-1">
                  <div class="flex flex-wrap items-center gap-2"><p class="text-sm font-medium">{{ tool.name }}</p><Badge :variant="tool.state === 'ready' ? 'secondary' : 'outline'">{{ w.stateLabel(tool.state) }}</Badge></div>
                  <p class="mt-1 text-xs text-muted-foreground">{{ tool.purpose }} · {{ tool.version }}</p>
                </div>
                <Button v-if="tool.state === 'missing' || tool.state === 'failed'" size="sm" variant="outline" @click="w.install(tool)"><Download v-if="tool.state === 'missing'" class="size-3.5" /><RefreshCw v-else class="size-3.5" />{{ tool.state === 'failed' ? '重试' : '安装' }}</Button>
                <Button v-else-if="tool.state === 'downloading'" size="sm" variant="ghost" @click="w.cancel(tool)">取消</Button>
                <Loader2 v-else-if="tool.state === 'verifying'" class="size-4 animate-spin text-muted-foreground" />
              </div>
              <div v-if="tool.state === 'downloading' || tool.state === 'verifying'" class="space-y-2">
                <div class="flex justify-between text-xs text-muted-foreground"><span>{{ tool.state === 'verifying' ? '正在校验文件…' : '正在下载…' }}</span><span>{{ tool.progress }}%</span></div>
                <div role="progressbar" :aria-label="`${tool.name} 安装进度`" :aria-valuenow="tool.progress" aria-valuemin="0" aria-valuemax="100" class="h-1.5 overflow-hidden rounded-full bg-muted"><div class="h-full bg-primary transition-all" :style="{ width: `${tool.progress}%` }" /></div>
              </div>
              <p v-if="tool.error" role="alert" class="text-xs text-destructive">{{ tool.error }}</p>
              <p v-if="tool.state === 'missing'" class="text-xs text-muted-foreground">{{ tool.id === 'pi-agent' ? '包含 Pi 与搜索工具 · ' + tool.size : '安装到应用目录' }}</p>
              <p v-if="tool.state === 'ready' && saved[tool.id].source === 'system'" class="text-xs text-muted-foreground">当前使用系统依赖</p>
            </div>
          </TabsContent>
          <TabsContent value="config" class="m-0 space-y-3">
            <div class="flex items-center gap-3 rounded-lg border p-4"><p class="min-w-0 flex-1 text-sm font-medium">scrcpy-server.jar</p><span class="text-xs text-muted-foreground">内置 · 4.0</span></div>
            <div v-for="tool in tools" :key="tool.id" class="space-y-3 rounded-lg border p-4">
              <div class="flex items-center justify-between gap-3">
                <label :for="`${tool.id}-source`" class="text-sm font-medium">{{ tool.name }}</label>
                <Select :model-value="tool.selection.source" @update:model-value="value => w.chooseSource(tool, value)">
                  <SelectTrigger :id="`${tool.id}-source`" class="w-28" :aria-label="`${tool.name} 来源`"><SelectValue /></SelectTrigger>
                  <SelectContent><SelectItem value="builtin">内置</SelectItem><SelectItem value="system">系统</SelectItem></SelectContent>
                </Select>
              </div>
              <template v-if="tool.selection.source === 'builtin'">
                <p v-if="tool.state === 'ready'" class="text-xs text-muted-foreground">{{ tool.version }} · 已安装</p>
                <div v-else class="flex items-center justify-between gap-2"><p class="text-xs text-muted-foreground">内置依赖未安装</p><Button variant="ghost" size="sm" @click="tab = 'manage'">前往安装</Button></div>
              </template>
              <template v-else>
                <div class="flex items-center justify-between gap-2">
                  <p class="min-w-0 break-all text-xs" :class="tool.detectedPath ? 'text-muted-foreground' : 'text-destructive'">{{ tool.detectedPath ? tool.detectedPath : `未在 PATH 中找到 ${tool.id === 'adb' ? 'adb' : 'pi'}` }}</p>
                  <Button variant="ghost" size="sm" :disabled="tool.checking" :aria-label="`重新检测 ${tool.name}`" @click="w.checkPath(tool)"><RefreshCw class="size-3.5" :class="{ 'animate-spin': tool.checking }" />检测</Button>
                </div>
                <div class="space-y-2">
                  <label :for="`${tool.id}-path`" class="text-xs text-muted-foreground">{{ tool.detectedPath ? '自定义路径（可选）' : '执行路径' }}</label>
                  <Input :id="`${tool.id}-path`" v-model="tool.selection.path" :placeholder="tool.id === 'adb' ? '/path/to/adb' : '/path/to/pi'" autocomplete="off" :disabled="tool.checking" @update:model-value="w.pathChanged(tool)" />
                </div>
                <p v-if="tool.pathError" role="alert" class="text-xs text-destructive">{{ tool.pathError }}</p>
                <p v-else-if="tool.pathChecked && tool.selection.path" role="status" class="text-xs text-muted-foreground">路径可用</p>
                <details v-if="tool.id === 'pi-agent'" class="text-xs text-muted-foreground">
                  <summary class="cursor-pointer py-1">搜索工具</summary>
                  <div class="mt-2 space-y-2 pl-3"><p class="break-all">fd · /opt/homebrew/bin/fd</p><p class="break-all">rg · /opt/homebrew/bin/rg</p></div>
                </details>
              </template>
            </div>
          </TabsContent>
        </div>
      </Tabs>
      <footer class="flex shrink-0 items-center gap-3 border-t px-5 py-3">
        <p v-if="tab === 'config' && formError" role="alert" class="flex-1 text-xs text-destructive">{{ formError }}</p>
        <p v-else-if="tab === 'config' && message" role="status" class="flex-1 text-xs text-muted-foreground">{{ message }}</p>
        <div class="ml-auto flex items-center gap-2">
          <Button variant="ghost" size="sm" @click="open = false">关闭</Button>
          <Button v-if="tab === 'config'" size="sm" :disabled="tools.some(tool => tool.checking)" @click="w.save">保存</Button>
        </div>
      </footer>
    </DialogContent>
  </Dialog>
</template>
