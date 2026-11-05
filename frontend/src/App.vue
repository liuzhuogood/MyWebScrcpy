<script setup lang="ts">
import { onMounted, onUnmounted } from 'vue'
import { useRouter, RouterView } from 'vue-router'
import { isTauri } from '@tauri-apps/api/core'
import { listen, type UnlistenFn } from '@tauri-apps/api/event'
import { Toaster } from '@/components/ui/sonner'

const router = useRouter()
let unlisten: UnlistenFn | undefined

onMounted(async () => {
  if (typeof window !== 'undefined') {
    (window as unknown as { __NAVIGATE_TO_PLAYER__?: (serial: string) => void }).__NAVIGATE_TO_PLAYER__ = (serial: string) => {
      router.push({ name: 'player', query: { serial } })
    }
  }

  if (isTauri()) {
    try {
      unlisten = await listen<string>('navigate-to-player', (event) => {
        if (event.payload) {
          router.push({ name: 'player', query: { serial: event.payload } })
        }
      })
    }
    catch (e) {
      console.warn('[App] 监听导航事件失败:', e)
    }
  }
})

onUnmounted(() => {
  if (unlisten) {
    unlisten()
  }
})
</script>

<template>
  <RouterView />
  <Toaster position="bottom-center" />
</template>
