<script setup lang="ts">
import { computed } from 'vue'
import {
  Activity,
  Cpu,
  Layers,
  Radio,
  RefreshCw,
  Server,
  ShieldCheck,
} from 'lucide-vue-next'
import { Button } from '@/components/ui/button'
import type { CyberDevice } from '../mockDevices'

const props = defineProps<{
  devices: CyberDevice[]
  activeFilter: string
}>()

const emit = defineEmits<{
  (e: 'refresh'): void
  (e: 'openAdd'): void
  (e: 'setFilter', filter: string): void
}>()

const onlineCount = computed(
  () => props.devices.filter(d => d.state === 'device').length
)
const totalThroughput = computed(() => {
  const online = props.devices.filter(d => d.state === 'device')
  const totalFps = online.reduce((acc, cur) => acc + cur.telemetry.fps, 0)
  return (totalFps * 0.42).toFixed(1)
})
const avgLatency = computed(() => {
  const online = props.devices.filter(d => d.state === 'device' && d.telemetry.latencyMs > 0)
  if (!online.length) return 0
  const sum = online.reduce((acc, cur) => acc + cur.telemetry.latencyMs, 0)
  return Math.round(sum / online.length)
})
</script>

<template>
  <div class="relative z-10 w-full rounded-2xl border border-cyan-500/20 bg-slate-950/80 p-4 shadow-[0_0_30px_rgba(0,242,254,0.06)] backdrop-blur-xl">
    <!-- 顶部极客状态微标 -->
    <div class="flex flex-wrap items-center justify-between gap-3 border-b border-cyan-500/10 pb-3">
      <div class="flex items-center gap-2.5">
        <div class="relative flex h-3 w-3 items-center justify-center">
          <span class="absolute inline-flex h-full w-full animate-ping rounded-full bg-emerald-400 opacity-75" />
          <span class="relative inline-flex h-2 w-2 rounded-full bg-emerald-500" />
        </div>
        <div class="flex items-center gap-1.5 font-mono text-xs font-semibold tracking-wider text-cyan-400">
          <Server class="h-3.5 w-3.5" />
          <span>ADB SERVER // NODE_01 (PORT 5037)</span>
        </div>
        <span class="rounded bg-cyan-950/60 px-1.5 py-0.5 font-mono text-[10px] text-cyan-300/80 border border-cyan-800/40">
          HEALTHY
        </span>
      </div>

      <div class="flex items-center gap-4 text-xs font-mono text-slate-400">
        <div class="flex items-center gap-1.5">
          <ShieldCheck class="h-3.5 w-3.5 text-emerald-400" />
          <span>门控调度: <strong class="text-slate-200">串行排队协调中</strong></span>
        </div>
        <div class="flex items-center gap-1.5">
          <Radio class="h-3.5 w-3.5 text-cyan-400 animate-pulse" />
          <span>平均时延: <strong class="text-cyan-300">{{ avgLatency }} ms</strong></span>
        </div>
      </div>
    </div>

    <!-- 遥测指标网格 -->
    <div class="mt-3.5 grid grid-cols-2 gap-3 sm:grid-cols-4">
      <!-- 在线节点数 -->
      <div class="relative overflow-hidden rounded-xl border border-cyan-500/15 bg-cyan-950/20 p-3">
        <div class="flex items-center justify-between">
          <span class="text-xs font-medium text-slate-400">在线设备阵列</span>
          <Layers class="h-4 w-4 text-cyan-400" />
        </div>
        <div class="mt-2 flex items-baseline gap-2">
          <span class="font-mono text-2xl font-bold tracking-tight text-white">{{ onlineCount }}</span>
          <span class="font-mono text-xs text-slate-500">/ {{ devices.length }} 节点</span>
        </div>
        <div class="mt-2 h-1 w-full overflow-hidden rounded-full bg-slate-800">
          <div
            class="h-full bg-gradient-to-r from-cyan-500 to-emerald-400 transition-all duration-500"
            :style="{ width: `${(onlineCount / (devices.length || 1)) * 100}%` }"
          />
        </div>
      </div>

      <!-- 实时聚合视频流吞吐 -->
      <div class="relative overflow-hidden rounded-xl border border-emerald-500/15 bg-emerald-950/20 p-3">
        <div class="flex items-center justify-between">
          <span class="text-xs font-medium text-slate-400">集群视频流吞吐</span>
          <Activity class="h-4 w-4 text-emerald-400" />
        </div>
        <div class="mt-2 flex items-baseline gap-1.5">
          <span class="font-mono text-2xl font-bold tracking-tight text-emerald-300">{{ totalThroughput }}</span>
          <span class="font-mono text-xs text-slate-400">MB/s</span>
        </div>
        <!-- 迷你脉冲波形 -->
        <div class="mt-2 flex h-2 items-end gap-1">
          <div class="h-1.5 w-1 rounded-full bg-emerald-400/80 animate-pulse" />
          <div class="h-2 w-1 rounded-full bg-emerald-400 animate-pulse delay-75" />
          <div class="h-1 w-1 rounded-full bg-emerald-400/60 animate-pulse delay-150" />
          <div class="h-2 w-1 rounded-full bg-emerald-400 animate-pulse delay-100" />
          <div class="h-1.5 w-1 rounded-full bg-emerald-400/70" />
          <span class="ml-1 text-[10px] font-mono text-emerald-400/80">60-120 FPS 流畅</span>
        </div>
      </div>

      <!-- 高速连接通道占比 -->
      <div class="relative overflow-hidden rounded-xl border border-blue-500/15 bg-blue-950/20 p-3">
        <div class="flex items-center justify-between">
          <span class="text-xs font-medium text-slate-400">通道介质分布</span>
          <Cpu class="h-4 w-4 text-blue-400" />
        </div>
        <div class="mt-2 flex items-center gap-3 font-mono text-xs">
          <div class="flex items-center gap-1 text-slate-200">
            <span class="h-2 w-2 rounded-full bg-cyan-400" />
            <span>USB 极速: <strong>{{ devices.filter(d => d.source === 'auto').length }}</strong></span>
          </div>
          <div class="flex items-center gap-1 text-slate-200">
            <span class="h-2 w-2 rounded-full bg-emerald-400" />
            <span>Wi-Fi 集群: <strong>{{ devices.filter(d => d.source === 'manual').length }}</strong></span>
          </div>
        </div>
        <div class="mt-2 text-[10px] font-mono text-slate-400">
          支持 TCP 5555 低抖动投屏
        </div>
      </div>

      <!-- 快速运维操作 -->
      <div class="flex flex-col justify-between rounded-xl border border-slate-800 bg-slate-900/60 p-3">
        <span class="text-xs font-medium text-slate-400">控制台运维</span>
        <div class="mt-2 flex items-center gap-2">
          <Button
            size="sm"
            variant="outline"
            class="h-8 flex-1 border-cyan-500/30 bg-cyan-950/40 text-xs font-mono text-cyan-300 hover:bg-cyan-900/50 hover:text-white"
            @click="emit('refresh')"
          >
            <RefreshCw class="mr-1.5 h-3.5 w-3.5" />
            扫描拓扑
          </Button>
          <Button
            size="sm"
            class="h-8 flex-1 bg-gradient-to-r from-cyan-500 to-blue-600 text-xs font-mono font-medium text-slate-950 hover:from-cyan-400 hover:to-blue-500 shadow-[0_0_15px_rgba(0,242,254,0.3)]"
            @click="emit('openAdd')"
          >
            + 接入网络设备
          </Button>
        </div>
      </div>
    </div>
  </div>
</template>
