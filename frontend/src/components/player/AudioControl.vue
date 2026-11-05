<script setup lang="ts">
import { Volume2, VolumeX } from 'lucide-vue-next'
import { Button } from '@/components/ui/button'
import { Popover, PopoverContent, PopoverTrigger } from '@/components/ui/popover'
import { Slider } from '@/components/ui/slider'

defineProps<{ ready: boolean, muted: boolean, volume: number, title: string }>()
const emit = defineEmits<{ toggle: [], volume: [value: number], 'update:open': [open: boolean] }>()
function updateVolume(values: number[] | undefined) {
  if (values?.[0] !== undefined) emit('volume', values[0] / 100)
}
</script>
<template>
  <Popover :modal="false" @update:open="emit('update:open', $event)">
    <PopoverTrigger as-child>
      <Button variant="ghost" size="icon" :disabled="!ready" :title="title" aria-label="音量控制">
        <VolumeX v-if="muted" class="h-5 w-5" /><Volume2 v-else class="h-5 w-5" />
      </Button>
    </PopoverTrigger>
    <PopoverContent class="w-60" align="end">
      <div class="flex items-center gap-3">
        <Button variant="ghost" size="icon" :aria-label="muted ? '播放声音' : '静音'" @click="emit('toggle')">
          <VolumeX v-if="muted" class="h-4 w-4" /><Volume2 v-else class="h-4 w-4" />
        </Button>
        <Slider :model-value="[Math.round(volume * 100)]" :min="0" :max="100" :step="1" aria-label="投屏音量" @update:model-value="updateVolume" />
        <span class="w-9 shrink-0 text-right text-xs tabular-nums">{{ Math.round(volume * 100) }}%</span>
      </div>
    </PopoverContent>
  </Popover>
</template>
