<script setup lang="ts">
import { Download, Upload } from 'lucide-vue-next'
import type { DropKind } from '@/composables/useDeviceFileDrop'

defineProps<{
  visible: boolean
  dragKind: DropKind | null
}>()
</script>

<template>
  <Transition
    enter-active-class="transition duration-150 ease-out"
    enter-from-class="opacity-0 scale-98"
    enter-to-class="opacity-100 scale-100"
    leave-active-class="transition duration-100 ease-in"
    leave-from-class="opacity-100 scale-100"
    leave-to-class="opacity-0 scale-98"
  >
    <div
      v-if="visible"
      class="pointer-events-none absolute inset-0 z-50 flex flex-col items-center justify-center bg-black/70 backdrop-blur-md p-4 text-center border-2 border-dashed border-primary/80 rounded-2xl"
    >
      <div
        class="flex h-12 w-12 items-center justify-center rounded-2xl bg-primary text-primary-foreground shadow-lg"
      >
        <Upload v-if="dragKind === 'apk'" class="h-6 w-6 animate-bounce" />
        <Download v-else class="h-6 w-6 animate-bounce" />
      </div>
      <p class="mt-3 text-sm font-semibold text-white">
        {{ dragKind === 'apk' ? '松开以安装应用' : '松开以保存到 Downloads' }}
      </p>
    </div>
  </Transition>
</template>
