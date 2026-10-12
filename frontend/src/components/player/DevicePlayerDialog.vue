<script setup lang="ts">
import { computed, defineAsyncComponent, ref, watch } from 'vue'
import { useWindowSize } from '@vueuse/core'
import { Dialog, DialogContent, DialogTitle } from '@/components/ui/dialog'
import type DevicePlayerComponent from './DevicePlayer.vue'

const props = defineProps<{ serial: string }>()
const open = defineModel<boolean>('open', { required: true })
const DevicePlayer = defineAsyncComponent(() => import('./DevicePlayer.vue'))
const player = ref<InstanceType<typeof DevicePlayerComponent> | null>(null)
const { width: viewportWidth, height: viewportHeight } = useWindowSize()
const layout = ref({ width: 0, height: 0, side: null as 'capture' | 'tools' | null })
watch(() => props.serial, () => { layout.value = { width: 0, height: 0, side: null } })
const dialogStyle = computed(() => {
  const height = viewportWidth.value < 640 ? viewportHeight.value : Math.floor(viewportHeight.value * 0.92)
  const maxWidth = viewportWidth.value < 640 ? viewportWidth.value : viewportWidth.value - 32
  const screenWidth = layout.value.height > 0 ? (height - 56) * layout.value.width / layout.value.height : 360
  const sideWidth = layout.value.side === 'capture' ? 288 : layout.value.side === 'tools' ? 428 : 0
  const width = layout.value.side && viewportWidth.value <= 820 ? maxWidth : Math.max(320, screenWidth) + sideWidth
  return { width: `${Math.min(maxWidth, Math.ceil(width) + 2)}px`, height: `${height}px` }
})
</script>

<template>
  <Dialog v-model:open="open">
    <DialogContent
      data-player-dialog
      class="flex max-w-none flex-col gap-0 overflow-hidden rounded-none border-0 p-0 sm:max-w-none sm:rounded-xl sm:border"
      :style="dialogStyle"
      :show-close-button="false"
      :aria-describedby="undefined"
      @escape-key-down="player?.handleEscape($event)"
    >
      <DialogTitle class="sr-only">手机操控 {{ serial }}</DialogTitle>
      <DevicePlayer v-if="open && serial" :key="serial" ref="player" :serial="serial" embedded @close="open = false" @layout="layout = $event" />
    </DialogContent>
  </Dialog>
</template>
