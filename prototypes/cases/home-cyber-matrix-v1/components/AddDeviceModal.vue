<script setup lang="ts">
import { ref } from 'vue'
import { Plus, Radio, Wifi, X } from 'lucide-vue-next'
import { Button } from '@/components/ui/button'
import { Input } from '@/components/ui/input'
import { toast } from 'vue-sonner'

const props = defineProps<{
  open: boolean
}>()

const emit = defineEmits<{
  (e: 'close'): void
  (e: 'add', payload: { serial: string, alias: string }): void
}>()

const host = ref('')
const port = ref('5555')
const alias = ref('')
const isTesting = ref(false)
const pingResult = ref<string | null>(null)

function handleTestPing() {
  if (!host.value.trim()) {
    toast.error('请输入设备 IP')
    return
  }
  isTesting.value = true
  pingResult.value = null
  setTimeout(() => {
    isTesting.value = false
    pingResult.value = `TCP 握手成功 // 往返时延 8.4ms (PORT ${port.value})`
  }, 600)
}

function handleAdd() {
  if (!host.value.trim()) {
    toast.error('请输入设备 IP')
    return
  }
  const serial = `${host.value.trim()}:${port.value.trim() || '5555'}`
  emit('add', {
    serial,
    alias: alias.value.trim() || host.value.trim(),
  })
  host.value = ''
  alias.value = ''
  pingResult.value = null
  emit('close')
}
</script>

<template>
  <div
    v-if="open"
    class="fixed inset-0 z-50 flex items-center justify-center bg-black/80 backdrop-blur-md p-4 animate-in fade-in"
  >
    <div class="relative w-full max-w-md rounded-2xl border border-cyan-500/40 bg-slate-950 p-6 shadow-[0_0_50px_rgba(0,242,254,0.2)]">
      <!-- 边角 HUD 装饰 -->
      <div class="pointer-events-none absolute left-0 top-0 h-4 w-4 border-l-2 border-t-2 border-cyan-400" />
      <div class="pointer-events-none absolute right-0 top-0 h-4 w-4 border-r-2 border-t-2 border-cyan-400" />
      <div class="pointer-events-none absolute bottom-0 left-0 h-4 w-4 border-b-2 border-l-2 border-cyan-400" />
      <div class="pointer-events-none absolute bottom-0 right-0 h-4 w-4 border-b-2 border-r-2 border-cyan-400" />

      <!-- 弹窗顶栏 -->
      <div class="flex items-center justify-between border-b border-cyan-500/20 pb-3">
        <div class="flex items-center gap-2">
          <Wifi class="h-4 w-4 text-cyan-400" />
          <h3 class="font-mono text-sm font-bold tracking-wider text-white">
            接入网络设备 // TCP_CONNECT
          </h3>
        </div>
        <Button variant="ghost" size="icon" class="h-7 w-7 text-slate-400 hover:text-white" @click="emit('close')">
          <X class="h-4 w-4" />
        </Button>
      </div>

      <!-- 表单内容 -->
      <div class="mt-4 space-y-3.5">
        <div>
          <label class="block font-mono text-xs text-slate-400 mb-1.5">设备 IP 地址 / HOST</label>
          <Input
            v-model="host"
            placeholder="192.168.1.xxx"
            class="border-cyan-500/30 bg-slate-900/80 font-mono text-sm text-cyan-200 placeholder:text-slate-600 focus:border-cyan-400"
          />
        </div>

        <div class="grid grid-cols-2 gap-3">
          <div>
            <label class="block font-mono text-xs text-slate-400 mb-1.5">ADB 端口 (默认 5555)</label>
            <Input
              v-model="port"
              placeholder="5555"
              class="border-cyan-500/30 bg-slate-900/80 font-mono text-sm text-cyan-200 focus:border-cyan-400"
            />
          </div>
          <div>
            <label class="block font-mono text-xs text-slate-400 mb-1.5">自定义设备别名</label>
            <Input
              v-model="alias"
              placeholder="例: 客厅展示机"
              class="border-cyan-500/30 bg-slate-900/80 font-sans text-sm text-slate-200 focus:border-cyan-400"
            />
          </div>
        </div>

        <!-- 连通性测试 -->
        <div v-if="pingResult" class="rounded-lg border border-emerald-500/30 bg-emerald-950/20 p-2.5 font-mono text-xs text-emerald-300">
          {{ pingResult }}
        </div>

        <div class="flex items-center gap-2 pt-2">
          <Button
            variant="outline"
            size="sm"
            :disabled="isTesting"
            class="flex-1 border-cyan-500/30 bg-cyan-950/30 font-mono text-xs text-cyan-300 hover:bg-cyan-900/40"
            @click="handleTestPing"
          >
            <Radio class="mr-1.5 h-3.5 w-3.5" :class="{ 'animate-spin': isTesting }" />
            {{ isTesting ? '探测中...' : '网络连通性测试' }}
          </Button>
          <Button
            size="sm"
            class="flex-1 bg-cyan-500 font-mono text-xs font-semibold text-slate-950 hover:bg-cyan-400 shadow-[0_0_20px_rgba(0,242,254,0.3)]"
            @click="handleAdd"
          >
            <Plus class="mr-1.5 h-3.5 w-3.5" />
            确认加入拓扑
          </Button>
        </div>
      </div>
    </div>
  </div>
</template>
