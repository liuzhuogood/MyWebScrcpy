<script setup lang="ts">
import { onBeforeUnmount, onMounted, ref } from 'vue'
import {
  Camera,
  Copy,
  Cpu,
  Layers,
  Monitor,
  Power,
  RotateCw,
} from 'lucide-vue-next'
import { Button } from '@/components/ui/button'
import { toast } from 'vue-sonner'
import type { FleetDevice } from '../mockDevices'

const props = defineProps<{
  device: FleetDevice | null
}>()

const emit = defineEmits<{
  (e: 'play', serial: string): void
}>()

const canvasRef = ref<HTMLCanvasElement | null>(null)
let animId: number | null = null

// 3D 线框全息投影渲染器
function initHologram() {
  const canvas = canvasRef.value
  if (!canvas) return
  const ctx = canvas.getContext('2d')
  if (!ctx) return

  let angle = 0

  // 手机几何体顶点 (x, y, z)
  const w = 1.0
  const h = 2.1
  const d = 0.14

  const vertices = [
    [-w, -h, -d], [w, -h, -d], [w, h, -d], [-w, h, -d],
    [-w, -h, d], [w, -h, d], [w, h, d], [-w, h, d],
  ]

  const edges = [
    [0, 1], [1, 2], [2, 3], [3, 0],
    [4, 5], [5, 6], [6, 7], [7, 4],
    [0, 4], [1, 5], [2, 6], [3, 7],
  ]

  const render = () => {
    ctx.clearRect(0, 0, canvas.width, canvas.height)
    const cx = canvas.width / 2
    const cy = canvas.height / 2
    const scale = 36

    angle += 0.015

    const cosA = Math.cos(angle)
    const sinA = Math.sin(angle)
    const cosB = Math.cos(0.2)
    const sinB = Math.sin(0.2)

    // 绘制全息底座光圈
    ctx.beginPath()
    ctx.ellipse(cx, cy + 95, 60, 18, 0, 0, Math.PI * 2)
    ctx.strokeStyle = 'rgba(0, 242, 254, 0.25)'
    ctx.lineWidth = 1.5
    ctx.stroke()

    // 旋转并投影顶点
    const projected = vertices.map(([x, y, z]) => {
      // 绕 Y 轴旋转
      const x1 = x * cosA - z * sinA
      const z1 = x * sinA + z * cosA
      // 绕 X 轴倾斜
      const y2 = y * cosB - z1 * sinB
      const z2 = y * sinB + z1 * cosB

      // 透视投影
      const distance = 4.5
      const fov = 160 / (distance + z2)
      return {
        x: cx + x1 * scale * fov * 0.02,
        y: cy + y2 * scale * fov * 0.02,
      }
    })

    // 绘制边缘线框
    ctx.lineWidth = 1.5
    ctx.strokeStyle = 'rgba(0, 242, 254, 0.8)'
    ctx.shadowBlur = 8
    ctx.shadowColor = '#00f2fe'

    for (const [i, j] of edges) {
      ctx.beginPath()
      ctx.moveTo(projected[i].x, projected[i].y)
      ctx.lineTo(projected[j].x, projected[j].y)
      ctx.stroke()
    }

    ctx.shadowBlur = 0
    animId = requestAnimationFrame(render)
  }

  render()
}

onMounted(() => {
  initHologram()
})

onBeforeUnmount(() => {
  if (animId) cancelAnimationFrame(animId)
})

function sendCommand(cmd: string) {
  toast.success(`[COMMAND_SENT] 向战位注入: ${cmd}`)
}

function copyText(txt: string) {
  navigator.clipboard?.writeText(txt)
  toast.info('已复制到剪贴板')
}
</script>

<template>
  <div class="relative flex flex-col justify-between rounded-2xl border border-blue-500/30 bg-[#070b14]/90 p-5 shadow-[0_0_35px_rgba(30,58,138,0.25)] backdrop-blur-xl">
    <!-- 顶栏状态 -->
    <div class="flex items-center justify-between border-b border-blue-500/20 pb-3">
      <div class="flex items-center gap-2">
        <Cpu class="h-4 w-4 text-cyan-400" />
        <h3 class="font-mono text-xs font-bold tracking-wider text-white">
          TACTICAL INSPECTOR // 战术深研舱
        </h3>
      </div>
      <span class="rounded bg-cyan-950/60 px-2 py-0.5 font-mono text-[10px] text-cyan-300 border border-cyan-800/40">
        {{ device ? device.serial : 'NO TARGET' }}
      </span>
    </div>

    <!-- 聚焦设备详情 -->
    <div v-if="device" class="mt-4 flex-1 space-y-4 font-mono text-xs">
      <!-- 3D 全息线框投影展示窗 -->
      <div class="relative flex flex-col items-center justify-center overflow-hidden rounded-xl border border-cyan-500/20 bg-black/60 p-2">
        <canvas
          ref="canvasRef"
          width="280"
          height="180"
          class="pointer-events-none"
        />
        <div class="absolute bottom-2 left-3 right-3 flex items-center justify-between text-[10px] text-cyan-400/80">
          <span>HOLO_ROTATION: ACTIVE</span>
          <span>FPS: {{ device.fps }}</span>
        </div>
      </div>

      <!-- 设备标识 -->
      <div>
        <h4 class="text-sm font-bold text-white tracking-wide font-sans">{{ device.name }}</h4>
        <p class="text-[11px] text-blue-300/80 mt-0.5">{{ device.model }} · {{ device.androidVersion }}</p>
      </div>

      <!-- 实时前台 Activity 解析 -->
      <div class="rounded-xl border border-blue-500/20 bg-blue-950/20 p-3 space-y-1.5">
        <div class="flex items-center justify-between text-[11px]">
          <span class="text-slate-400">前台焦点 Activity:</span>
          <button
            class="text-cyan-400 hover:text-cyan-200"
            @click="copyText(device.currentActivity)"
          >
            <Copy class="h-3 w-3" />
          </button>
        </div>
        <p class="text-[11px] text-emerald-300 font-semibold truncate">{{ device.currentApp }}</p>
        <p class="text-[10px] text-slate-400 truncate">{{ device.currentPackage }}</p>
        <p class="text-[10px] text-slate-500 truncate">{{ device.currentActivity }}</p>
      </div>

      <!-- 遥测规格指标 -->
      <div class="grid grid-cols-2 gap-2 text-[11px]">
        <div class="rounded-lg border border-slate-800 bg-slate-900/50 p-2">
          <span class="text-slate-400 text-[10px] block">显示规格</span>
          <span class="text-white font-semibold">{{ device.resolution }}</span>
          <span class="text-[10px] text-slate-500 block">DPI {{ device.dpi }}</span>
        </div>
        <div class="rounded-lg border border-slate-800 bg-slate-900/50 p-2">
          <span class="text-slate-400 text-[10px] block">动力与温度</span>
          <span class="text-emerald-300 font-semibold">{{ device.batteryPercent }}% {{ device.batteryCharging ? '(充)' : '' }}</span>
          <span class="text-[10px] text-slate-500 block">{{ device.tempCelsius }}°C</span>
        </div>
      </div>

      <!-- 战术物理按键注入 -->
      <div class="space-y-2 pt-1">
        <span class="text-[11px] text-slate-400 block font-medium">战术按键注入:</span>
        <div class="grid grid-cols-3 gap-2">
          <Button
            size="sm"
            variant="outline"
            class="h-8 border-slate-700 bg-slate-900/60 text-xs font-mono text-slate-200 hover:bg-slate-800"
            @click="sendCommand('KEYCODE_BACK')"
          >
            返回
          </Button>
          <Button
            size="sm"
            variant="outline"
            class="h-8 border-slate-700 bg-slate-900/60 text-xs font-mono text-slate-200 hover:bg-slate-800"
            @click="sendCommand('KEYCODE_HOME')"
          >
            主屏
          </Button>
          <Button
            size="sm"
            variant="outline"
            class="h-8 border-slate-700 bg-slate-900/60 text-xs font-mono text-slate-200 hover:bg-slate-800"
            @click="sendCommand('KEYCODE_APP_SWITCH')"
          >
            多任务
          </Button>
        </div>

        <div class="grid grid-cols-3 gap-2">
          <Button
            size="sm"
            variant="outline"
            class="h-8 border-slate-700 bg-slate-900/60 text-xs font-mono text-slate-300 hover:bg-slate-800"
            @click="sendCommand('SCREENSHOT_CAPTURE')"
          >
            <Camera class="mr-1 h-3.5 w-3.5" /> 截屏
          </Button>
          <Button
            size="sm"
            variant="outline"
            class="h-8 border-slate-700 bg-slate-900/60 text-xs font-mono text-slate-300 hover:bg-slate-800"
            @click="sendCommand('SCREEN_ROTATE')"
          >
            <RotateCw class="mr-1 h-3.5 w-3.5" /> 旋转
          </Button>
          <Button
            size="sm"
            variant="outline"
            class="h-8 border-rose-500/30 bg-rose-950/20 text-xs font-mono text-rose-300 hover:bg-rose-900/30"
            @click="sendCommand('KEYCODE_POWER')"
          >
            <Power class="mr-1 h-3.5 w-3.5" /> 电源
          </Button>
        </div>
      </div>
    </div>

    <!-- 未锁定目标时的空占位 -->
    <div v-else class="flex flex-1 flex-col items-center justify-center p-8 text-center font-mono text-xs text-slate-500">
      <Layers class="h-8 w-8 text-slate-600 mb-2" />
      <span>请在左侧矩阵中选中目标战位</span>
    </div>

    <!-- 底部主控按钮 -->
    <div v-if="device" class="mt-4 border-t border-blue-500/20 pt-4">
      <Button
        :disabled="device.state !== 'device'"
        class="w-full h-10 bg-gradient-to-r from-blue-600 to-cyan-500 font-mono text-xs font-bold text-white shadow-[0_0_20px_rgba(59,130,246,0.4)] hover:from-blue-500 hover:to-cyan-400"
        @click="emit('play', device.serial)"
      >
        <Monitor class="mr-2 h-4 w-4" />
        拉起战位超清投屏
      </Button>
    </div>
  </div>
</template>
