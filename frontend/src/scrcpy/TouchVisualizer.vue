<script setup lang="ts">
import { onMounted, onUnmounted, ref, watch } from 'vue'

export interface TouchVisualEvent { kind: 'down' | 'move' | 'up' | 'tap' | 'swipe', x: number, y: number, x2?: number, y2?: number }
const props = defineProps<{ visible: boolean, width: number, height: number }>()
const canvasRef = ref<HTMLCanvasElement | null>(null)
type Point = { x: number, y: number, t: number }
type Effect = { type: 'ripple' | 'swipe', x: number, y: number, x2: number, y2: number, born: number }
const effects: Effect[] = []
const trail: Point[] = []
let raf = 0

function resize() {
  const canvas = canvasRef.value
  if (!canvas) return
  canvas.width = Math.max(1, props.width)
  canvas.height = Math.max(1, props.height)
  if (!props.visible) clear()
}
function clear() { canvasRef.value?.getContext('2d')?.clearRect(0, 0, props.width, props.height) }
function point(e: TouchVisualEvent) { return { x: e.x * props.width, y: e.y * props.height } }
function finger(ctx: CanvasRenderingContext2D, x: number, y: number, alpha: number, scale = 1) {
  const r = Math.max(10, props.width / 60) * scale
  ctx.beginPath(); ctx.arc(x, y, r, 0, Math.PI * 2)
  ctx.fillStyle = `rgba(255,255,255,${0.45 * alpha})`; ctx.fill()
  ctx.lineWidth = Math.max(2, props.width / 400)
  ctx.strokeStyle = `rgba(0,150,255,${0.9 * alpha})`; ctx.stroke()
}
function render() {
  raf = 0
  const canvas = canvasRef.value
  const ctx = canvas?.getContext('2d')
  if (!canvas || !ctx || !props.visible) { clear(); return }
  const now = performance.now()
  ctx.clearRect(0, 0, canvas.width, canvas.height)
  while (trail.length && now - trail[0].t > 700) trail.shift()
  if (trail.length) {
    ctx.lineWidth = Math.max(3, props.width / 300); ctx.lineCap = 'round'; ctx.lineJoin = 'round'
    for (let i = 1; i < trail.length; i++) {
      const alpha = 1 - (now - trail[i].t) / 700
      ctx.strokeStyle = `rgba(0,150,255,${0.7 * alpha})`; ctx.beginPath(); ctx.moveTo(trail[i - 1].x, trail[i - 1].y); ctx.lineTo(trail[i].x, trail[i].y); ctx.stroke()
    }
    const last = trail[trail.length - 1]; finger(ctx, last.x, last.y, 1)
  }
  for (let i = effects.length - 1; i >= 0; i--) {
    const e = effects[i]
    const duration = e.type === 'ripple' ? 600 : 480
    const t = (now - e.born) / duration
    if (t >= 1) { effects.splice(i, 1); continue }
    const fade = 1 - t
    if (e.type === 'ripple') {
      finger(ctx, e.x, e.y, fade)
      ctx.beginPath(); ctx.arc(e.x, e.y, Math.max(10, props.width / 60) + t * props.width / 25, 0, Math.PI * 2)
      ctx.strokeStyle = `rgba(0,150,255,${0.8 * fade})`; ctx.lineWidth = Math.max(2, props.width / 400); ctx.stroke()
    } else {
      const head = Math.min(1, t * 2.2)
      ctx.strokeStyle = `rgba(0,150,255,${0.85 * (t < 0.7 ? 1 : (1 - t) / 0.3)})`
      ctx.lineWidth = Math.max(4, props.width / 250); ctx.lineCap = 'round'; ctx.beginPath(); ctx.moveTo(e.x, e.y); ctx.lineTo(e.x + (e.x2 - e.x) * head, e.y + (e.y2 - e.y) * head); ctx.stroke()
    }
  }
  if (trail.length || effects.length) raf = requestAnimationFrame(render)
}
function start() { if (!raf) raf = requestAnimationFrame(render) }
function handle(event: TouchVisualEvent) {
  if (!props.visible) return
  const p = point(event); const now = performance.now()
  if (event.kind === 'down' || event.kind === 'move') { trail.push({ ...p, t: now }); if (trail.length > 200) trail.splice(0, trail.length - 200) }
  if (event.kind === 'up' || event.kind === 'tap') effects.push({ type: 'ripple', ...p, x2: p.x, y2: p.y, born: now })
  if (event.kind === 'swipe' && event.x2 !== undefined && event.y2 !== undefined)
    effects.push({ type: 'swipe', x: p.x, y: p.y, x2: event.x2 * props.width, y2: event.y2 * props.height, born: now })
  start()
}
watch(() => [props.width, props.height], resize)
watch(() => props.visible, value => { if (!value) { effects.length = 0; trail.length = 0; if (raf) cancelAnimationFrame(raf); raf = 0; clear() } else start() })
onMounted(resize)
onUnmounted(() => { if (raf) cancelAnimationFrame(raf) })
defineExpose({ handle })
</script>

<template><canvas ref="canvasRef" class="pointer-events-none absolute inset-0 h-full w-full" aria-hidden="true" /></template>
