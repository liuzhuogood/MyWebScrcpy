<script setup lang="ts">
import { computed, onUnmounted, ref } from 'vue'
import { useWindowSize } from '@vueuse/core'
import { usePlayerSettings } from '@/composables/usePlayerSettings'
import DevicePlayer from './DevicePlayer.vue'

const props = withDefaults(defineProps<{
  serial: string
  zIndex?: number
  offsetIndex?: number
}>(), {
  zIndex: 50,
  offsetIndex: 0,
})

const emit = defineEmits<{
  close: []
  focus: []
}>()

const settings = usePlayerSettings()
const { width: winW, height: winH } = useWindowSize()

// 视频原始比例（默认常用手机 9:19.5 ≈ 0.4615）
const videoAspect = ref<number>(9 / 19.5)

function onLayout(layout: { width: number, height: number }) {
  if (layout.width > 0 && layout.height > 0) {
    videoAspect.value = layout.width / layout.height
  }
}

// 依据用户投屏设置配置默认初始宽度
const initialWidth = computed(() => {
  if (settings.floatingSize.value === 'compact') return 300
  if (settings.floatingSize.value === 'large') return 440
  return 360
})

const userWidth = ref<number | null>(null)
const currentWidth = computed({
  get: () => userWidth.value ?? initialWidth.value,
  set: (val: number) => {
    userWidth.value = Math.max(220, Math.min(val, winW.value - 32))
  },
})

// 顶栏高度为 56px (h-14)
const HEADER_HEIGHT = 56

// 窗口尺寸以投屏的宽度与视频比例严格对齐，彻底消除黑边与白边
const floatingHeight = computed(() => {
  const aspect = videoAspect.value > 0 ? videoAspect.value : (9 / 19.5)
  const contentH = Math.round(currentWidth.value / aspect)
  const maxAvailableH = Math.max(240, winH.value - 40)
  return Math.min(HEADER_HEIGHT + contentH, maxAvailableH)
})

// 多小窗错开级联初始位置，并在屏幕中间居中
const stagger = (props.offsetIndex % 6) * 28
const initialCenterX = Math.max(16, Math.floor((window.innerWidth - currentWidth.value) / 2) + stagger)
const initialCenterY = Math.max(16, Math.floor((window.innerHeight - floatingHeight.value) / 2) + stagger)

const currentX = ref(initialCenterX)
const currentY = ref(initialCenterY)

// 限制小窗在屏幕可视区域内
const clampedX = computed(() => {
  const maxX = Math.max(0, winW.value - currentWidth.value)
  return Math.min(Math.max(0, currentX.value), maxX)
})

const clampedY = computed(() => {
  const maxY = Math.max(0, winH.value - floatingHeight.value)
  return Math.min(Math.max(0, currentY.value), maxY)
})

// ---------------- 拖拽移动窗口 ----------------
const isDragging = ref(false)
let cleanupDrag: (() => void) | null = null

function onHeaderPointerDown(e: PointerEvent) {
  if (e.button !== 0 && e.pointerType === 'mouse')
    return

  const target = e.target as HTMLElement | null
  const header = target?.closest('header')
  if (!header)
    return

  // 点击按钮、下拉菜单等交互控件时不触发窗口移动
  const interactive = (e.target as Element | null)?.closest(
    'button, a, input, select, [role="button"], [role="menuitem"], [data-slot="dropdown-menu-trigger"], [data-radix-collection-item], [data-state]'
  )
  if (interactive)
    return

  emit('focus')
  e.preventDefault()
  isDragging.value = true
  const startX = e.clientX
  const startY = e.clientY
  const startLeft = clampedX.value
  const startTop = clampedY.value

  const onPointerMove = (ev: PointerEvent) => {
    const dx = ev.clientX - startX
    const dy = ev.clientY - startY
    const maxX = Math.max(0, winW.value - currentWidth.value)
    const maxY = Math.max(0, winH.value - floatingHeight.value)
    currentX.value = Math.min(Math.max(0, startLeft + dx), maxX)
    currentY.value = Math.min(Math.max(0, startTop + dy), maxY)
  }

  const onPointerUp = () => {
    isDragging.value = false
    if (cleanupDrag) {
      cleanupDrag()
      cleanupDrag = null
    }
  }

  window.addEventListener('pointermove', onPointerMove)
  window.addEventListener('pointerup', onPointerUp)
  window.addEventListener('pointercancel', onPointerUp)

  cleanupDrag = () => {
    window.removeEventListener('pointermove', onPointerMove)
    window.removeEventListener('pointerup', onPointerUp)
    window.removeEventListener('pointercancel', onPointerUp)
  }
}

// ---------------- 拖动边缘调整大小 ----------------
const isResizing = ref(false)
let cleanupResize: (() => void) | null = null

function onResizePointerDown(e: PointerEvent) {
  if (e.button !== 0 && e.pointerType === 'mouse')
    return

  emit('focus')
  e.preventDefault()
  e.stopPropagation()
  isResizing.value = true

  const startX = e.clientX
  const startW = currentWidth.value

  const onPointerMove = (ev: PointerEvent) => {
    const dx = ev.clientX - startX
    const aspect = videoAspect.value > 0 ? videoAspect.value : (9 / 19.5)
    const maxW = Math.min(winW.value - 32, (winH.value - 72) * aspect)
    const newW = Math.max(220, Math.min(maxW, Math.round(startW + dx)))
    currentWidth.value = newW
  }

  const onPointerUp = () => {
    isResizing.value = false
    if (cleanupResize) {
      cleanupResize()
      cleanupResize = null
    }
  }

  window.addEventListener('pointermove', onPointerMove)
  window.addEventListener('pointerup', onPointerUp)
  window.addEventListener('pointercancel', onPointerUp)

  cleanupResize = () => {
    window.removeEventListener('pointermove', onPointerMove)
    window.removeEventListener('pointerup', onPointerUp)
    window.removeEventListener('pointercancel', onPointerUp)
  }
}

onUnmounted(() => {
  if (cleanupDrag) cleanupDrag()
  if (cleanupResize) cleanupResize()
})
</script>

<template>
  <div
    class="fixed flex flex-col rounded-xl border bg-black shadow-2xl overflow-hidden ring-1 ring-border/50 transition-shadow select-none"
    :class="{ 'ring-primary/60 shadow-primary/10': isDragging || isResizing }"
    :style="{
      left: `${clampedX}px`,
      top: `${clampedY}px`,
      width: `${currentWidth}px`,
      height: `${floatingHeight}px`,
      zIndex: zIndex,
    }"
    @pointerdown="emit('focus')"
  >
    <div class="flex-1 flex flex-col min-h-0 min-w-0" @pointerdown="onHeaderPointerDown">
      <DevicePlayer
        :key="serial"
        :serial="serial"
        embedded
        @close="emit('close')"
        @layout="onLayout"
      />
    </div>

    <!-- 右下角拖动调整大小把手 -->
    <div
      class="absolute bottom-0 right-0 z-30 flex h-6 w-6 cursor-nwse-resize items-end justify-end p-1 text-muted-foreground/50 transition-colors hover:text-primary active:text-primary"
      title="拖拽调整小窗大小"
      @pointerdown.stop="onResizePointerDown"
    >
      <svg class="h-3 w-3" viewBox="0 0 12 12" fill="none">
        <path d="M10 2L2 10M10 6L6 10M10 10L10 10" stroke="currentColor" stroke-width="1.5" stroke-linecap="round" />
      </svg>
    </div>
  </div>
</template>
