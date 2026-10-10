<script setup lang="ts">
import { computed, defineAsyncComponent, onErrorCaptured, onUnmounted, ref, watch } from 'vue'
import { Moon, Sun } from 'lucide-vue-next'
import { Button } from '@/components/ui/button'
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from '@/components/ui/select'
import { prototypes } from './catalog'

function readSelection() {
  return location.hash.slice(1) || prototypes[0]?.id || ''
}

const selectedId = ref(readSelection())
const dark = ref(window.matchMedia('(prefers-color-scheme: dark)').matches)
const loadError = ref(false)
const selected = computed(() => prototypes.find(item => item.id === selectedId.value))
const page = computed(() => selected.value ? defineAsyncComponent(selected.value.loader) : null)

function selectCase(value: unknown) {
  if (typeof value === 'string')
    location.hash = value
}

function syncSelection() {
  selectedId.value = readSelection()
}

watch(selectedId, () => { loadError.value = false })
watch(selected, (item) => {
  document.title = item ? `${item.title} · 原型` : 'MyWebScrcpy · 原型'
}, { immediate: true })
watch(dark, value => document.documentElement.classList.toggle('dark', value), { immediate: true })
window.addEventListener('hashchange', syncSelection)
onUnmounted(() => window.removeEventListener('hashchange', syncSelection))
onErrorCaptured((error) => {
  console.error('[prototype] 页面加载失败:', error)
  loadError.value = true
  return false
})
</script>

<template>
  <div class="min-h-screen bg-background text-foreground">
    <header class="border-b bg-background">
      <div class="mx-auto flex max-w-6xl items-center gap-3 px-4 py-3 sm:px-6">
        <span class="shrink-0 text-sm font-medium">原型</span>
        <Select :model-value="selectedId" @update:model-value="selectCase">
          <SelectTrigger class="w-full max-w-64" aria-label="选择原型">
            <SelectValue placeholder="选择原型" />
          </SelectTrigger>
          <SelectContent>
            <SelectItem v-for="item in prototypes" :key="item.id" :value="item.id">{{ item.title }}</SelectItem>
          </SelectContent>
        </Select>
        <Button variant="ghost" size="icon" class="ml-auto shrink-0" :aria-label="dark ? '切换为浅色' : '切换为深色'" @click="dark = !dark">
          <Sun v-if="dark" class="h-4 w-4" />
          <Moon v-else class="h-4 w-4" />
        </Button>
      </div>
    </header>
    <main class="mx-auto max-w-6xl px-4 py-8 sm:px-6">
      <p v-if="loadError" role="alert" class="text-destructive">页面加载失败，请刷新后重试。</p>
      <Suspense v-else-if="page">
        <component :is="page" :key="selectedId" />
        <template #fallback><p role="status" class="text-muted-foreground">加载中…</p></template>
      </Suspense>
      <p v-else role="status" class="text-muted-foreground">{{ prototypes.length ? '未找到该原型。' : '暂无原型。' }}</p>
    </main>
  </div>
</template>
