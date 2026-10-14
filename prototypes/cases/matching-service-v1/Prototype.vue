<script setup lang="ts">
import { computed, ref } from 'vue'
import { Button } from '@/components/ui/button'
import { Badge } from '@/components/ui/badge'
import { Card, CardContent } from '@/components/ui/card'
import { Switch } from '@/components/ui/switch'
import MatchingServiceContent from '@/components/system/MatchingServiceContent.vue'
import type { MatchingStatus } from '@/composables/useMatchingService'
const checked = ref(false)
const running = ref(false)
const missing = ref(false)
const settings = ref<HTMLElement>()
const status = computed<MatchingStatus>(() => ({
  state: running.value ? 'running' : 'stopped', pid: running.value ? 3210 : 0, error: '',
  environment: { path: '/usr/local/bin/python3', version: checked.value ? '3.14.3' : '', ready: checked.value && !missing.value, error: '', warning: '', dependencies: checked.value ? ['httpx', 'websocket', 'av', 'cv2', 'numpy'].map(name => ({ name, available: !missing.value || !['av', 'cv2'].includes(name), error: '' })) : [] },
  devices: running.value ? [{ serial: '10.0.0.30:5555', state: 'matching', error: '' }, { serial: '测试设备', state: 'waiting', error: '' }] : [],
}))
function action(operation: 'check' | 'start' | 'stop') { if (operation === 'check') checked.value = true; else running.value = operation === 'start' }
</script>
<template>
  <main class="mx-auto max-w-3xl space-y-6 p-5">
    <h1 class="text-lg font-semibold">系统配置</h1>
    <section ref="settings" tabindex="-1"><Card><CardContent class="pt-6"><MatchingServiceContent :status="status" :busy="false" error="" @action="action" /></CardContent></Card></section>
    <Card><CardContent class="space-y-3 pt-6"><h2 class="text-sm font-medium">模板管理</h2><div v-if="!running" class="flex flex-wrap items-center justify-between gap-2 text-xs text-muted-foreground"><span>自动模板匹配未启动，启动后显示匹配框。</span><Button variant="link" size="sm" @click="settings?.focus()">前往系统设置</Button></div><div class="flex justify-between text-sm"><span>桌面图标</span><Badge variant="secondary">已启用</Badge></div></CardContent></Card>
    <label class="flex items-center gap-3 text-xs text-muted-foreground"><Switch :model-value="missing" @update:model-value="v => { missing = !!v; checked = false; running = false }" />缺少依赖</label>
  </main>
</template>
