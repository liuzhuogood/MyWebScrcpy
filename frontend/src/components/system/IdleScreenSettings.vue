<script setup lang="ts">
import { onMounted, onUnmounted, ref } from 'vue'
import { Button } from '@/components/ui/button'
import { Switch } from '@/components/ui/switch'

const enabled = ref(false)
const loading = ref(true)
const saving = ref(false)
const error = ref('')
let request: AbortController | undefined
let disposed = false

async function update(value?: boolean) {
  if (saving.value || disposed) return
  request?.abort()
  const controller = new AbortController()
  request = controller
  const timeout = window.setTimeout(() => controller.abort(), 10000)
  const isSave = value !== undefined
  saving.value = isSave
  loading.value = !isSave
  error.value = ''
  try {
    const response = await fetch('/api/system/idle-screen', {
      method: isSave ? 'PUT' : 'GET',
      headers: isSave ? { 'Content-Type': 'application/json' } : undefined,
      body: isSave ? JSON.stringify({ enabled: value }) : undefined,
      signal: controller.signal,
    })
    if (!response.ok) throw new Error((await response.text()).trim() || '熄屏设置请求失败')
    const config: { enabled: boolean } = await response.json()
    if (typeof config.enabled !== 'boolean') throw new Error('熄屏设置数据无效')
    if (!disposed && request === controller) enabled.value = config.enabled
  }
  catch (cause) {
    if (!disposed && request === controller) error.value = cause instanceof Error ? cause.message : '熄屏设置请求失败'
  }
  finally {
    window.clearTimeout(timeout)
    if (!disposed && request === controller) {
      loading.value = false
      saving.value = false
      request = undefined
    }
  }
}

onMounted(() => { void update() })
onUnmounted(() => { disposed = true; request?.abort() })
</script>

<template>
  <div class="space-y-3 rounded-lg border p-4">
    <div class="flex items-center justify-between gap-4">
      <div>
        <p id="idle-screen-label" class="text-sm font-medium">空闲自动熄屏</p>
        <p id="idle-screen-description" class="text-xs text-muted-foreground">设备无投屏、ADB 等操作满 10 分钟后自动熄屏</p>
      </div>
      <Switch
        :model-value="enabled"
        :disabled="loading || saving"
        aria-labelledby="idle-screen-label"
        aria-describedby="idle-screen-description"
        @update:model-value="value => update(!!value)"
      />
    </div>
    <div v-if="error" class="flex items-center justify-between gap-3" role="alert">
      <p class="text-xs text-destructive">{{ error }}</p>
      <Button variant="outline" size="sm" @click="update()">重新加载</Button>
    </div>
  </div>
</template>
