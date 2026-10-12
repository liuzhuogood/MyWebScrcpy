<script setup lang="ts">
import { ref, watch } from 'vue'
import { Download, RefreshCw } from 'lucide-vue-next'
import { Badge } from '@/components/ui/badge'
import { Button } from '@/components/ui/button'
import { Input } from '@/components/ui/input'
import { Card, CardContent } from '@/components/ui/card'
import type { DependencyInfo, DependencyPaths } from '@/types/dependencies'

const props = defineProps<{
  tools: DependencyInfo[]
  paths: DependencyPaths
  configurable: boolean
  loading: boolean
  saving: boolean
  error: string
  saved: boolean
}>()
const emit = defineEmits<{ refresh: []; save: [paths: DependencyPaths] }>()
const adb = ref('')
const python = ref('')
watch(() => props.paths, value => { adb.value = value.adb; python.value = value.python }, { immediate: true })
</script>

<template>
  <section class="mx-auto w-full max-w-4xl space-y-5 p-4 sm:p-6">
    <div class="flex items-center justify-between gap-3">
      <h1 class="text-xl font-semibold">依赖管理</h1>
      <Button variant="outline" size="sm" :disabled="loading" @click="emit('refresh')">
        <RefreshCw class="size-4" :class="{ 'animate-spin': loading }" />重新检测
      </Button>
    </div>
    <p v-if="error" role="alert" class="text-sm text-destructive">{{ error }}</p>
    <p v-if="loading && tools.length === 0" role="status" class="text-sm text-muted-foreground">正在检测…</p>
    <Card v-for="tool in tools" :key="tool.id">
      <CardContent class="flex flex-wrap items-start gap-3 p-4 sm:flex-nowrap">
        <div class="min-w-0 flex-1 space-y-1">
          <div class="flex items-center gap-2">
            <h2 class="font-medium">{{ tool.name }}</h2>
            <Badge :variant="tool.installed ? 'secondary' : 'outline'">{{ tool.installed ? '可用' : '不可用' }}</Badge>
          </div>
          <p class="text-sm text-muted-foreground">{{ tool.purpose }}</p>
          <p v-if="tool.path" class="break-all text-xs text-muted-foreground">{{ tool.path }}</p>
          <p v-if="tool.version" class="whitespace-pre-line break-all text-xs text-muted-foreground">{{ tool.version }}</p>
          <p v-if="tool.error" class="break-all text-xs text-destructive">{{ tool.error }}</p>
        </div>
        <Button v-if="tool.download_url" as-child variant="outline" size="sm">
          <a :href="tool.download_url" rel="noopener noreferrer"><Download class="size-4" />官方安装</a>
        </Button>
      </CardContent>
    </Card>
    <Card v-if="configurable">
      <CardContent class="space-y-4 p-4">
        <form class="space-y-4" @submit.prevent="emit('save', { adb: adb.trim(), python: python.trim() })">
          <div class="space-y-2">
            <label for="adb-path" class="text-sm font-medium">ADB 路径</label>
            <Input id="adb-path" v-model="adb" placeholder="留空使用系统环境" autocomplete="off" />
          </div>
          <div class="space-y-2">
            <label for="python-path" class="text-sm font-medium">Python 路径</label>
            <Input id="python-path" v-model="python" placeholder="留空使用系统环境" autocomplete="off" />
          </div>
          <div class="flex flex-wrap items-center gap-3">
            <Button type="submit" :disabled="saving">{{ saving ? '保存中…' : '保存' }}</Button>
            <p v-if="saved" role="status" class="text-sm text-muted-foreground">已保存，重启 App 后生效。</p>
          </div>
        </form>
      </CardContent>
    </Card>
  </section>
</template>
