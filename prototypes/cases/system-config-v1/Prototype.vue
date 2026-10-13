<script setup lang="ts">
import { ref } from 'vue'
import { FileCode2, LayoutGrid, Plus, RefreshCw, Search, Settings2, Smartphone, Usb, Wifi } from 'lucide-vue-next'
import { Button } from '@/components/ui/button'
import { Input } from '@/components/ui/input'
import { Badge } from '@/components/ui/badge'
import ConfigDialog from './ConfigDialog.vue'

const config = ref<InstanceType<typeof ConfigDialog>>()
const keyword = ref('')
const devices = [
  { name: 'Pixel 9', serial: '192.168.1.20:5555', model: 'Google Pixel 9', wireless: true },
  { name: '测试手机', serial: 'R58M42D3ABC', model: 'Samsung Galaxy S23', wireless: false },
]
</script>

<template>
  <div class="min-h-[65vh] overflow-hidden rounded-xl border bg-background shadow-sm">
    <header class="flex h-14 items-center gap-3 border-b px-4">
      <Smartphone class="size-6 shrink-0 text-primary" />
      <span class="min-w-0 truncate text-base font-semibold">MyWebScrcpy</span>
      <span class="hidden text-xs text-muted-foreground sm:inline">v1.19.5</span>
      <div class="ml-auto flex shrink-0 items-center gap-1">
        <Button variant="ghost" size="sm" class="hidden sm:inline-flex" disabled><FileCode2 class="size-4" />脚本与任务</Button>
        <Button variant="ghost" size="sm" class="hidden md:inline-flex" disabled><LayoutGrid class="size-4" />大屏模式</Button>
        <Button variant="outline" size="sm" aria-label="系统配置" @click="config?.show()"><Settings2 class="size-4" /><span class="hidden sm:inline">系统配置</span></Button>
      </div>
    </header>
    <main class="space-y-5 p-4 sm:p-6">
      <div class="flex flex-wrap items-center gap-2">
        <div class="relative min-w-36 flex-1 sm:max-w-xs"><Search class="absolute left-3 top-1/2 size-4 -translate-y-1/2 text-muted-foreground" /><Input v-model="keyword" class="pl-9" placeholder="搜索设备" aria-label="搜索设备" /></div>
        <Button variant="outline" size="icon" aria-label="刷新设备" disabled><RefreshCw class="size-4" /></Button>
        <Button size="sm" class="ml-auto" disabled><Plus class="size-4" />添加设备</Button>
      </div>
      <div class="grid gap-4 sm:grid-cols-2 lg:grid-cols-3">
        <div v-for="device in devices.filter(item => `${item.name} ${item.serial}`.includes(keyword))" :key="device.serial" class="flex items-start gap-3 rounded-lg border p-4">
          <Smartphone class="mt-1 size-9 shrink-0 text-muted-foreground" />
          <div class="min-w-0 space-y-1"><div class="flex items-center gap-2"><p class="font-medium">{{ device.name }}</p><Badge variant="secondary">在线</Badge></div><p class="truncate text-xs text-muted-foreground">{{ device.model }}</p><p class="flex items-center gap-1 text-xs text-muted-foreground"><Wifi v-if="device.wireless" class="size-3" /><Usb v-else class="size-3" />{{ device.serial }}</p></div>
        </div>
      </div>
    </main>
    <ConfigDialog ref="config" />
  </div>
</template>
