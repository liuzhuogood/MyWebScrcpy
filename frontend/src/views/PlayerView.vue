<script setup lang="ts">
import { computed, onUnmounted, watch } from 'vue'
import { useRoute } from 'vue-router'
import { isTauri } from '@tauri-apps/api/core'
import { getCurrentWindow } from '@tauri-apps/api/window'
import DevicePlayer from '@/components/player/DevicePlayer.vue'

const route = useRoute()
const serial = computed(() => typeof route.query.serial === 'string' ? route.query.serial : '')
const embedded = computed(() => route.query.embedded === '1' || route.query.embedded === 'true')

watch(embedded, value => {
  document.documentElement.classList.toggle('desktop-player', value && isTauri() && getCurrentWindow().label.startsWith('player-'))
}, { immediate: true })
onUnmounted(() => document.documentElement.classList.remove('desktop-player'))

async function handleClose() {
  if (isTauri()) {
    try {
      await getCurrentWindow().close()
    }
    catch (e) {
      console.warn('关闭窗口失败', e)
    }
  }
}
</script>

<template>
  <div class="h-full w-full overflow-hidden">
    <DevicePlayer
      :key="serial"
      :serial="serial"
      :embedded="embedded"
      @close="handleClose"
    />
  </div>
</template>
