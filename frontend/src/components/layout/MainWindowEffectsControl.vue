<script setup lang="ts">
import { ref, watch } from 'vue'
import { Blend, MousePointer2, Pin, PinOff } from 'lucide-vue-next'
import { Button } from '@/components/ui/button'
import { Popover, PopoverContent, PopoverTrigger } from '@/components/ui/popover'
import { Slider } from '@/components/ui/slider'
import { Switch } from '@/components/ui/switch'
import {
  AlertDialog, AlertDialogAction, AlertDialogCancel, AlertDialogContent,
  AlertDialogDescription, AlertDialogFooter, AlertDialogHeader, AlertDialogTitle,
} from '@/components/ui/alert-dialog'
import { useMainWindowEffects } from '@/composables/useMainWindowEffects'

const effects = useMainWindowEffects()
const confirmOpen = ref(false)
const popoverOpen = ref(false)
const opacityValue = ref([effects.state.value.opacity])

watch(() => effects.state.value.opacity, next => {
  opacityValue.value = [next]
})

function updateOpacity(next: number[] | undefined) {
  if (!next || next[0] === undefined) return
  opacityValue.value = next
  void effects.setOpacity(next[0])
}

function enableClickThrough() {
  popoverOpen.value = false
  void effects.setClickThrough(true)
}
</script>

<template>
  <div v-if="effects.native" class="flex items-center gap-1">
    <!-- 置顶按钮 -->
    <Button
      variant="ghost"
      size="icon"
      :title="effects.state.value.alwaysOnTop ? '取消主窗口置顶' : '保持主窗口置顶'"
      :aria-label="effects.state.value.alwaysOnTop ? '取消主窗口置顶' : '保持主窗口置顶'"
      :class="{ 'text-primary bg-primary/10': effects.state.value.alwaysOnTop }"
      :disabled="effects.busy.value"
      class="cursor-pointer"
      @click="effects.toggleAlwaysOnTop()"
    >
      <PinOff v-if="effects.state.value.alwaysOnTop" class="h-4 w-4" />
      <Pin v-else class="h-4 w-4" />
    </Button>

    <!-- 透明与穿透设置 Popover -->
    <Popover v-model:open="popoverOpen">
      <PopoverTrigger as-child>
        <Button
          variant="ghost"
          size="icon"
          title="主窗口效果"
          aria-label="主窗口效果"
          :class="{ 'text-primary bg-primary/10': effects.state.value.opacity < 100 || effects.state.value.clickThrough }"
          class="cursor-pointer"
        >
          <Blend class="h-4 w-4" />
        </Button>
      </PopoverTrigger>
      <PopoverContent align="end" class="z-[9999] w-64 space-y-4">
        <div class="flex items-center justify-between text-sm">
          <span>主窗口透明度</span>
          <span class="tabular-nums font-mono text-xs">{{ opacityValue[0] }}%</span>
        </div>
        <Slider
          :model-value="opacityValue"
          :min="0"
          :max="100"
          :step="1"
          aria-label="主窗口透明度"
          @update:model-value="updateOpacity"
        />
        <div class="flex items-center gap-1.5 flex-wrap">
          <Button
            v-for="op in [100, 10, 5, 2, 1]"
            :key="op"
            variant="outline"
            size="sm"
            class="h-6 px-1.5 text-xs font-mono"
            :class="{ 'border-primary text-primary bg-primary/10': opacityValue[0] === op }"
            @click="updateOpacity([op])"
          >
            {{ op }}%
          </Button>
        </div>
        <div class="flex items-center justify-between gap-4 text-sm pt-1">
          <span>无边框</span>
          <Switch
            :model-value="effects.state.value.borderless"
            :disabled="effects.busy.value"
            aria-label="主窗口无边框"
            @update:model-value="effects.setBorderless(!!$event)"
          />
        </div>
        <p v-if="effects.shortcut.value" class="text-xs text-muted-foreground">
          按 {{ effects.shortcut.value }} 或菜单“窗口 → 恢复主窗口”可恢复
        </p>
        <p v-else class="text-xs text-muted-foreground">
          可通过菜单“窗口 → 恢复主窗口”恢复
        </p>
        <div class="flex items-center justify-between gap-2 pt-1 border-t">
          <Button
            variant="outline"
            size="sm"
            :disabled="effects.busy.value"
            @click="effects.state.value.clickThrough ? effects.setClickThrough(false) : (confirmOpen = true)"
          >
            <MousePointer2 class="h-4 w-4 mr-1" />
            {{ effects.state.value.clickThrough ? '解除穿透' : '鼠标穿透' }}
          </Button>
          <Button
            variant="ghost"
            size="sm"
            :disabled="effects.busy.value || (effects.state.value.opacity === 100 && !effects.state.value.clickThrough && !effects.state.value.alwaysOnTop && !effects.state.value.borderless)"
            @click="effects.restore()"
          >
            恢复默认
          </Button>
        </div>
      </PopoverContent>
    </Popover>

    <!-- 鼠标穿透二次确认弹窗 -->
    <AlertDialog v-model:open="confirmOpen">
      <AlertDialogContent class="z-[10000]">
        <AlertDialogHeader>
          <AlertDialogTitle>开启主窗口鼠标穿透？</AlertDialogTitle>
          <AlertDialogDescription>
            开启后鼠标点击将穿透主窗口传递给下方应用。
            {{ effects.shortcut.value ? `可按 ${effects.shortcut.value} 恢复，或` : '' }}通过系统菜单栏“窗口 → 恢复主窗口”恢复。
          </AlertDialogDescription>
        </AlertDialogHeader>
        <AlertDialogFooter>
          <AlertDialogCancel>取消</AlertDialogCancel>
          <AlertDialogAction :disabled="effects.busy.value" @click="enableClickThrough">
            开启穿透
          </AlertDialogAction>
        </AlertDialogFooter>
      </AlertDialogContent>
    </AlertDialog>
  </div>
</template>
