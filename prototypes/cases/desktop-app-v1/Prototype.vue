<script setup lang="ts">
import { ref } from 'vue'
import { Monitor, Smartphone, X } from 'lucide-vue-next'
import { Button } from '@/components/ui/button'
import { Card, CardContent } from '@/components/ui/card'
import { DropdownMenu, DropdownMenuContent, DropdownMenuItem, DropdownMenuSeparator, DropdownMenuTrigger } from '@/components/ui/dropdown-menu'
import DependencyPanel from '@/components/dependencies/DependencyPanel.vue'
import type { DependencyInfo, DependencyPaths } from '@/types/dependencies'

const visible = ref(true)
const page = ref('设备列表')
const saved = ref(false)
const paths = ref<DependencyPaths>({ adb: '', python: '' })
const tools = ref<DependencyInfo[]>([
  { id: 'adb', name: 'ADB', purpose: '设备连接与控制', installed: true, path: '/usr/local/bin/adb', version: 'Android Debug Bridge version 1.0.41', error: '', download_url: 'https://developer.android.com/tools/releases/platform-tools' },
  { id: 'python', name: 'Python', purpose: '自动化脚本', installed: false, path: '', version: '', error: '未找到 Python', download_url: 'https://www.python.org/downloads/' },
  { id: 'scrcpy-server', name: 'scrcpy-server', purpose: '设备投屏', installed: true, path: '内置', version: '与 App 配套', error: '', download_url: '' },
])
function navigate(value: string) { page.value = value; visible.value = true }
function save(value: DependencyPaths) { paths.value = value; saved.value = true }
</script>

<template>
  <div class="space-y-3">
    <div class="flex h-9 justify-end rounded-md border bg-muted/40 px-2">
      <DropdownMenu>
        <DropdownMenuTrigger as-child><Button variant="ghost" size="icon" aria-label="MyWebScrcpy 托盘菜单"><Monitor class="size-4" /></Button></DropdownMenuTrigger>
        <DropdownMenuContent align="end">
          <DropdownMenuItem @select="visible = true">打开主窗口</DropdownMenuItem>
          <DropdownMenuItem v-for="item in ['设备列表', '大屏监控', '脚本与任务', '依赖管理']" :key="item" @select="navigate(item)">{{ item }}</DropdownMenuItem>
          <DropdownMenuSeparator />
          <DropdownMenuItem @select="visible = false">退出 MyWebScrcpy</DropdownMenuItem>
        </DropdownMenuContent>
      </DropdownMenu>
    </div>
    <div v-if="visible" class="overflow-hidden rounded-lg border bg-background shadow-sm">
      <div class="flex h-11 items-center gap-1 border-b px-2">
        <DropdownMenu>
          <DropdownMenuTrigger as-child><Button size="sm" variant="ghost">MyWebScrcpy</Button></DropdownMenuTrigger>
          <DropdownMenuContent align="start">
            <DropdownMenuItem @select="navigate('依赖管理')">依赖管理</DropdownMenuItem>
            <DropdownMenuSeparator />
            <DropdownMenuItem @select="visible = false">退出 MyWebScrcpy</DropdownMenuItem>
          </DropdownMenuContent>
        </DropdownMenu>
        <DropdownMenu>
          <DropdownMenuTrigger as-child><Button size="sm" variant="ghost">视图</Button></DropdownMenuTrigger>
          <DropdownMenuContent>
            <DropdownMenuItem v-for="item in ['设备列表', '大屏监控', '脚本与任务']" :key="item" @select="navigate(item)">{{ item }}</DropdownMenuItem>
          </DropdownMenuContent>
        </DropdownMenu>
        <Button class="ml-auto" size="icon" variant="ghost" aria-label="关闭窗口" @click="visible = false"><X class="size-4" /></Button>
      </div>
      <DependencyPanel v-if="page === '依赖管理'" :tools="tools" :paths="paths" configurable :loading="false" :saving="false" error="" :saved="saved" @refresh="saved = false" @save="save" />
      <div v-else class="min-h-96 space-y-5 p-5">
        <h1 class="text-xl font-semibold">{{ page }}</h1>
        <Card v-if="page === '设备列表'" class="max-w-sm"><CardContent class="flex items-center gap-3 p-4"><Smartphone class="size-9 text-muted-foreground" /><div><p class="font-medium">测试设备</p><p class="text-sm text-muted-foreground">192.168.1.20:5555</p></div></CardContent></Card>
      </div>
    </div>
  </div>
</template>
