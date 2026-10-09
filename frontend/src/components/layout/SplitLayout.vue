<script setup lang="ts">
import { computed, nextTick, onUnmounted, ref, watch } from 'vue'

const props = withDefaults(defineProps<{ open: boolean, initialWidth?: number, minWidth?: number, maxWidth?: number }>(), {
  initialWidth: 360,
  minWidth: 320,
  maxWidth: Number.MAX_SAFE_INTEGER,
})
const width = ref(props.initialWidth)
const dragging = ref(false)
const resized = ref(false)
const style = computed(() => ({ '--split-width': `${width.value}px` }))
let startX = 0
let startWidth = 0

function move(e: PointerEvent) {
  if (!dragging.value) return
  width.value = Math.max(props.minWidth, Math.min(props.maxWidth, startWidth + startX - e.clientX))
  window.dispatchEvent(new Event('resize'))
}
function stop() {
  dragging.value = false
  window.removeEventListener('pointermove', move)
  window.removeEventListener('pointerup', stop)
}
function start(e: PointerEvent) {
  if (window.innerWidth <= 820) return
  startX = e.clientX
  const side = (e.currentTarget as HTMLElement).parentElement?.querySelector<HTMLElement>('.split-layout-side')
  startWidth = side?.getBoundingClientRect().width || width.value
  width.value = startWidth
  resized.value = true
  dragging.value = true
  window.addEventListener('pointermove', move)
  window.addEventListener('pointerup', stop)
  e.preventDefault()
}
onUnmounted(stop)
watch(() => props.open, async () => {
  await nextTick()
  window.dispatchEvent(new Event('resize'))
})
</script>

<template>
  <div class="split-layout" :class="{ 'split-layout-open': open, 'split-layout-resized': resized, 'split-layout-dragging': dragging }" :style="style">
    <div class="split-layout-main"><slot name="main" /></div>
    <template v-if="open">
      <div class="split-layout-resizer" role="separator" aria-label="调整右侧面板宽度" @pointerdown="start" />
      <aside class="split-layout-side"><slot name="side" /></aside>
    </template>
  </div>
</template>

<style scoped>
.split-layout { height: 100%; max-height: 100%; min-height: 0; min-width: 0; flex: 1; overflow: hidden; }
.split-layout:not(.split-layout-open) { display: flex; }
.split-layout-open { display: grid; grid-template-columns: minmax(0, 1fr) 8px minmax(420px, 1fr); grid-template-rows: minmax(0, 1fr); }
.split-layout-open.split-layout-resized { grid-template-columns: minmax(0, 1fr) 8px var(--split-width); }
.split-layout-main { min-width: 0; min-height: 0; overflow: hidden; }
.split-layout:not(.split-layout-open) .split-layout-main { width: 100%; height: 100%; }
.split-layout-open .split-layout-main { grid-column: 1; grid-row: 1; width: 100%; }
.split-layout-side { height: 100%; max-height: 100%; min-width: 0; min-height: 0; overflow: hidden; grid-column: 3; grid-row: 1; }
.split-layout-resizer { width: 8px; cursor: col-resize; touch-action: none; position: relative; grid-column: 2; grid-row: 1; }
.split-layout-resizer::after { content: ''; position: absolute; inset: 0 3px; background: transparent; transition: background .15s; }
.split-layout-resizer:hover::after, .split-layout-dragging .split-layout-resizer::after { background: var(--primary); }
@media (max-width: 820px) {
  .split-layout { display: flex; flex-direction: column; overflow: auto; }
  .split-layout-side { width: 100%; flex: 0 0 min(52vh, 520px); }
  .split-layout-resizer { display: none; }
}
</style>
