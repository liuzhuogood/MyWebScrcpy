<script setup lang="ts">
import { onMounted, ref } from 'vue'
import { ArrowLeft, Settings } from 'lucide-vue-next'
import { RouterLink } from 'vue-router'
import { Button } from '@/components/ui/button'
import DependencyPanel from '@/components/dependencies/DependencyPanel.vue'
import SystemConfigDialog from '@/components/system/SystemConfigDialog.vue'
const configOpen = ref(false)
import type { DependencyPaths, DependencyStatus } from '@/types/dependencies'

const status = ref<DependencyStatus>({ tools: [], paths: { adb: '', python: '' }, configurable: false })
const loading = ref(false)
const saving = ref(false)
const error = ref('')
const saved = ref(false)
const token = (window as Window & { __MYWEBSCRCPY_DESKTOP_TOKEN__?: string }).__MYWEBSCRCPY_DESKTOP_TOKEN__
async function refresh() {
  loading.value = true
  error.value = ''
  try {
    const response = await fetch('/api/dependencies')
    if (!response.ok) throw new Error('依赖检测失败')
    status.value = await response.json() as DependencyStatus
  }
  catch (cause) { error.value = cause instanceof Error ? cause.message : '依赖检测失败' }
  finally { loading.value = false }
}
async function save(paths: DependencyPaths) {
  saving.value = true
  saved.value = false
  error.value = ''
  try {
    if (!token) throw new Error('请在桌面 App 中配置依赖')
    const response = await fetch('/api/dependencies/paths', {
      method: 'PUT',
      headers: { 'Content-Type': 'application/json', Authorization: `Bearer ${token}` },
      body: JSON.stringify(paths),
    })
    if (!response.ok) throw new Error((await response.text()).trim())
    await refresh()
    saved.value = true
  }
  catch (cause) { error.value = cause instanceof Error ? cause.message : '保存失败' }
  finally { saving.value = false }
}
onMounted(refresh)
</script>

<template>
  <div class="min-h-screen bg-background">
    <header class="flex h-14 items-center gap-3 border-b px-4">
      <Button as-child variant="ghost" size="icon"><RouterLink to="/" aria-label="返回设备列表"><ArrowLeft class="size-4" /></RouterLink></Button>
      <span class="font-semibold">MyWebScrcpy</span>
      <Button class="ml-auto" variant="ghost" size="sm" @click="configOpen = true"><Settings class="size-4" />系统配置</Button>
    </header>
    <SystemConfigDialog v-model:open="configOpen" />
    <DependencyPanel :tools="status.tools" :paths="status.paths" :configurable="status.configurable && Boolean(token)" :loading="loading" :saving="saving" :error="error" :saved="saved" @refresh="refresh" @save="save" />
  </div>
</template>
