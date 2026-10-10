<script setup lang="ts">
import { RefreshCw, Search, Smartphone } from 'lucide-vue-next'
import { computed, onMounted, onUnmounted } from 'vue'
import AppHeader from '@/components/layout/AppHeader.vue'
import DeviceCard from '@/components/device/DeviceCard.vue'
import { Button } from '@/components/ui/button'
import { Empty, EmptyDescription, EmptyHeader, EmptyMedia, EmptyTitle } from '@/components/ui/empty'
import { Input } from '@/components/ui/input'
import { Skeleton } from '@/components/ui/skeleton'
import { Switch } from '@/components/ui/switch'
import { useDeviceStore } from '@/stores/devices'

const store = useDeviceStore()
const isDevelopment = import.meta.env.DEV

const countText = computed(() => {
  if (store.mockEnabled)
    return `${store.devices.length} 台设备（Mock）`
  if (store.loading && store.devices.length === 0)
    return '加载中...'
  if (store.error)
    return '加载失败'
  return store.devices.length > 0 ? `${store.devices.length} 台设备在线` : '无设备'
})

onMounted(() => store.startPolling())
onUnmounted(() => store.stopPolling())
</script>

<template>
  <div class="min-h-screen">
    <AppHeader :device-count-text="countText" />
    <main class="w-full px-4 py-6">
      <div class="mb-4 flex items-center justify-between gap-3">
        <div class="flex items-center gap-2 flex-1 max-w-sm">
          <div class="relative flex-1">
            <Search class="absolute left-2.5 top-1/2 h-4 w-4 -translate-y-1/2 text-muted-foreground" />
            <Input v-model="store.keyword" placeholder="搜索名称 / serial / 型号" aria-label="搜索设备" class="pl-8" />
          </div>
          <Button variant="outline" size="icon" aria-label="刷新设备列表" @click="store.load(true)">
            <RefreshCw class="h-4 w-4" />
          </Button>
        </div>

        <div class="flex items-center gap-3">
          <span class="text-xs text-muted-foreground">{{ countText }}</span>
          <label v-if="isDevelopment" class="flex cursor-pointer items-center gap-2 text-sm text-muted-foreground">
            Mock
            <Switch :model-value="store.mockEnabled" aria-label="Mock 数据开关" @update:model-value="(v) => store.setMock(!!v)" />
          </label>
        </div>
      </div>

      <!-- 初次加载骨架 -->
      <div v-if="store.loading && store.devices.length === 0" class="grid gap-4 sm:grid-cols-2 lg:grid-cols-3">
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
          <EmptyTitle>{{ store.keyword ? '没有匹配的设备' : '未检测到在线设备' }}</EmptyTitle>
          <EmptyDescription>
            {{ store.keyword ? '换个关键词试试' : '请连接设备并开启 USB 调试' }}
          </EmptyDescription>
        </EmptyHeader>
      </Empty>

      <!-- 设备墙 -->
      <div v-else class="grid gap-4 sm:grid-cols-2 lg:grid-cols-3">
        <DeviceCard v-for="d in store.filtered" :key="d.serial" :device="d" />
      </div>
    </main>
  </div>
</template>
