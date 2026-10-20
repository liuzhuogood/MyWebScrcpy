<script setup lang="ts">
import { onBeforeUnmount, onMounted, ref } from 'vue'
import {
  Cast,
  Clock,
  Compass,
  LayoutGrid,
  Moon,
  Plus,
  Radio,
  RefreshCw,
} from 'lucide-vue-next'
import { Button } from '@/components/ui/button'

defineProps<{
  totalCount: number
  onlineCount: number
  selectedCount: number
}>()

const emit = defineEmits<{
  (e: 'refresh'): void
  (e: 'tileAll'): void
  (e: 'syncBroadcast'): void
  (e: 'screenOff'): void
  (e: 'openAdd'): void
}>()

const missionTime = ref('')
let timer: number | null = null

function updateClock() {
  const now = new Date()
  missionTime.value = now.toTimeString().split(' ')[0]
}

onMounted(() => {
  updateClock()
  timer = window.setInterval(updateClock, 1000)
})

onBeforeUnmount(() => {
  if (timer) clearInterval(timer)
})
</script>

<template>
  <div class="relative z-20 flex flex-col gap-3 rounded-2xl border border-blue-500/30 bg-[#070b14]/90 p-4 shadow-[0_0_40px_rgba(30,58,138,0.3)] backdrop-blur-xl">
    <!-- 顶层指挥站状态与时钟 -->
    <div class="flex flex-wrap items-center justify-between gap-3 border-b border-blue-500/20 pb-3">
      <div class="flex items-center gap-3">
        <div class="flex h-9 w-9 items-center justify-center rounded-xl bg-blue-950/80 border border-blue-400/40 text-blue-400 shadow-[0_0_15px_rgba(59,130,246,0.3)]">
          <Compass class="h-5 w-5 animate-spin" style="animation-duration: 20s" />
        </div>
        <div>
          <div class="flex items-center gap-2">
            <h2 class="font-mono text-sm font-bold tracking-widest text-white">
              MYWEBSCRCPY // FLEET COMMAND CENTER
            </h2>
            <span class="rounded bg-blue-900/60 px-1.5 py-0.5 font-mono text-[10px] text-blue-200 border border-blue-700/50">
              TACTICAL_OS v2.4
            </span>
          </div>
          <p class="font-mono text-[11px] text-blue-400/80">星舰多机阵列总控平台 · 毫秒级全域协同</p>
        </div>
      </div>

      <!-- 战备遥测时钟与门控指示 -->
      <div class="flex items-center gap-5 font-mono text-xs text-slate-300">
        <div class="flex items-center gap-2">
          <Clock class="h-3.5 w-3.5 text-blue-400" />
          <span>战术时钟: <strong class="text-white">{{ missionTime }}</strong></span>
        </div>
        <div class="hidden sm:flex items-center gap-2">
          <span class="h-2 w-2 rounded-full bg-emerald-400 animate-pulse" />
          <span>阵列在线率: <strong class="text-emerald-300">{{ Math.round((onlineCount / (totalCount || 1)) * 100) }}%</strong></span>
        </div>
        <div class="hidden md:flex items-center gap-2">
          <Radio class="h-3.5 w-3.5 text-cyan-400 animate-pulse" />
          <span>DeviceGate: <strong class="text-cyan-300">零拥堵调度</strong></span>
        </div>
      </div>
    </div>

    <!-- 群控作战协同工具条 -->
    <div class="flex flex-wrap items-center justify-between gap-2 pt-1 text-xs font-mono">
      <div class="flex flex-wrap items-center gap-2">
        <span class="text-slate-400 font-medium">战备指令群:</span>

        <!-- 批量小窗平铺 -->
        <Button
          size="sm"
          variant="outline"
          class="h-8 border-blue-500/30 bg-blue-950/40 text-blue-300 hover:bg-blue-900/50 hover:text-white"
          @click="emit('tileAll')"
        >
          <LayoutGrid class="mr-1.5 h-3.5 w-3.5" />
          全量平铺投屏
        </Button>

        <!-- 一键群控同步广播 -->
        <Button
          size="sm"
          variant="outline"
          class="h-8 border-cyan-500/30 bg-cyan-950/40 text-cyan-300 hover:bg-cyan-900/50 hover:text-white"
          @click="emit('syncBroadcast')"
        >
          <Cast class="mr-1.5 h-3.5 w-3.5" />
          群控事件广播 {{ selectedCount > 0 ? `(${selectedCount} 台)` : '(全选)' }}
        </Button>

        <!-- 一键息屏 -->
        <Button
          size="sm"
          variant="outline"
          class="h-8 border-slate-700 bg-slate-900/50 text-slate-300 hover:bg-slate-800"
          @click="emit('screenOff')"
        >
          <Moon class="mr-1.5 h-3.5 w-3.5" />
          一键熄屏节能
        </Button>
      </div>

      <div class="flex items-center gap-2">
        <Button
          size="sm"
          variant="ghost"
          class="h-8 border border-blue-500/20 text-blue-300 hover:bg-blue-950/40"
          @click="emit('refresh')"
        >
          <RefreshCw class="mr-1 h-3.5 w-3.5" />
          扫描拓扑
        </Button>

        <Button
          size="sm"
          class="h-8 bg-blue-600 font-semibold text-white hover:bg-blue-500 shadow-[0_0_15px_rgba(37,99,235,0.4)]"
          @click="emit('openAdd')"
        >
          <Plus class="mr-1 h-3.5 w-3.5" />
          接入新战位
        </Button>
      </div>
    </div>
  </div>
</template>
