<script setup lang="ts">
import { ref, watch } from 'vue'
import { Blend, MousePointer2 } from 'lucide-vue-next'
import { Button } from '@/components/ui/button'
import { Popover, PopoverContent, PopoverTrigger } from '@/components/ui/popover'
import { Slider } from '@/components/ui/slider'
import { Switch } from '@/components/ui/switch'
import {
  AlertDialog, AlertDialogAction, AlertDialogCancel, AlertDialogContent,
  AlertDialogDescription, AlertDialogFooter, AlertDialogHeader, AlertDialogTitle,
} from '@/components/ui/alert-dialog'

const props = withDefaults(defineProps<{
  opacity: number
  clickThrough: boolean
  borderless?: boolean
  shortcut: string | null
  busy: boolean
}>(), { borderless: false })
const emit = defineEmits<{
  opacity: [value: number]
  clickThrough: [enabled: boolean]
  borderless: [enabled: boolean]
  restore: []
  activeChange: [active: boolean]
}>()
const confirmOpen = ref(false)
const popoverOpen = ref(false)
const value = ref([props.opacity])
watch([popoverOpen, confirmOpen], ([p, c]) => {
  emit('activeChange', p || c)
})
watch(() => props.opacity, next => { value.value = [next] })
watch(() => props.busy, busy => { if (!busy) value.value = [props.opacity] })

function updateOpacity(next: number[] | undefined) {
  if (!next || next[0] === undefined) return
  value.value = next
  emit('opacity', next[0])
}

function enableClickThrough() {
  // Close the portalled controls before the whole native window ignores input.
  popoverOpen.value = false
  emit('clickThrough', true)
}
</script>

<template>
  <Popover v-model:open="popoverOpen">
    <PopoverTrigger as-child>
      <Button variant="ghost" size="icon" title="窗口设置" aria-label="窗口设置">
        <Blend class="h-4 w-4" />
      </Button>
    </PopoverTrigger>
    <PopoverContent align="end" class="z-[9999] w-64 space-y-4">
      <div class="flex items-center justify-between text-sm">
        <span>透明度</span><span class="tabular-nums">{{ value[0] }}%</span>
      </div>
      <Slider :model-value="value" :min="0" :max="100" :step="1" :disabled="!shortcut" aria-label="小窗透明度" @update:model-value="updateOpacity" />
      <p v-if="shortcut" class="text-xs text-muted-foreground">{{ shortcut }} 恢复小窗</p>
      <p v-else class="text-xs text-destructive">恢复快捷键不可用</p>
      <div class="flex items-center justify-between gap-4 text-sm">
        <span>无边框</span>
        <Switch :model-value="borderless" :disabled="busy" aria-label="无边框" @update:model-value="emit('borderless', !!$event)" />
      </div>
      <div class="flex items-center justify-between gap-2">
        <Button variant="outline" size="sm" :disabled="busy || !shortcut" @click="clickThrough ? emit('clickThrough', false) : (confirmOpen = true)">
          <MousePointer2 class="h-4 w-4" />{{ clickThrough ? '解除穿透' : '鼠标穿透' }}
        </Button>
        <Button variant="ghost" size="sm" :disabled="busy || (opacity === 100 && !clickThrough && !borderless)" @click="emit('restore')">恢复</Button>
      </div>
    </PopoverContent>
  </Popover>
  <AlertDialog v-model:open="confirmOpen">
    <AlertDialogContent class="z-[10000]">
      <AlertDialogHeader>
        <AlertDialogTitle>开启鼠标穿透？</AlertDialogTitle>
        <AlertDialogDescription>点击将传递到下方应用。按 {{ shortcut }} 或在主窗口点击“恢复小窗”可恢复。</AlertDialogDescription>
      </AlertDialogHeader>
      <AlertDialogFooter>
        <AlertDialogCancel>取消</AlertDialogCancel>
        <AlertDialogAction :disabled="busy || !shortcut" @click="enableClickThrough">开启穿透</AlertDialogAction>
      </AlertDialogFooter>
    </AlertDialogContent>
  </AlertDialog>
</template>
