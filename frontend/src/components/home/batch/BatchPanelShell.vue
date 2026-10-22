<script setup lang="ts">
import { SlidersHorizontal, X } from 'lucide-vue-next'
import { onMounted, onUnmounted } from 'vue'

defineProps<{
  title?: string
}>()

const emit = defineEmits<{
  (e: 'close'): void
}>()

function handleKeyDown(e: KeyboardEvent) {
  if (e.key === 'Escape') {
    emit('close')
  }
}

onMounted(() => {
  window.addEventListener('keydown', handleKeyDown)
})

onUnmounted(() => {
  window.removeEventListener('keydown', handleKeyDown)
})
</script>

<template>
  <aside
    aria-label="批量操作面板"
    class="flex flex-col rounded-3xl border border-slate-200/90 bg-white/95 dark:border-white/15 dark:bg-[#0e121d]/95 backdrop-blur-3xl shadow-xl overflow-hidden select-none"
  >
    <!-- 头部：标题与关闭按钮 -->
    <div class="flex items-center justify-between border-b border-slate-200/80 px-4 py-3 dark:border-white/10 shrink-0">
      <div class="flex items-center gap-2">
        <SlidersHorizontal class="h-4 w-4 text-violet-600 dark:text-violet-400" />
        <span class="font-bold text-sm text-slate-900 dark:text-white">{{ title || '批量操作面板' }}</span>
      </div>
      <button
        type="button"
        class="p-1 rounded-lg text-slate-400 hover:text-slate-900 dark:hover:text-white cursor-pointer transition-colors"
        title="关闭面板 (Esc)"
        aria-label="关闭批量操作面板"
        @click="emit('close')"
      >
        <X class="h-4 w-4" />
      </button>
    </div>

    <!-- 内容区 -->
    <div class="p-4 space-y-4 overflow-y-auto max-h-[calc(100vh-14rem)] flex-1 overscroll-contain">
      <slot />
    </div>
  </aside>
</template>
