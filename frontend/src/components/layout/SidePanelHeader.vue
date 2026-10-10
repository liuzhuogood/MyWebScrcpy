<script setup lang="ts">
import { X } from 'lucide-vue-next'
import { Button } from '@/components/ui/button'

defineProps<{
  title?: string
  ariaLabel?: string
  buttonClass?: string
}>()

const emit = defineEmits<{
  close: []
}>()
</script>

<template>
  <header class="flex h-11 shrink-0 items-center gap-2 border-b px-3">
    <Button
      variant="ghost"
      size="icon"
      class="h-7 w-7 shrink-0"
      :class="buttonClass || 'text-muted-foreground hover:text-foreground'"
      :aria-label="ariaLabel || (title ? `关闭${title}` : '关闭面板')"
      @click="emit('close')"
    >
      <X class="h-4 w-4" />
    </Button>
    <slot name="title">
      <strong v-if="title" class="truncate text-sm font-semibold">{{ title }}</strong>
    </slot>
    <div v-if="$slots.actions" class="ml-auto flex items-center gap-1.5">
      <slot name="actions" />
    </div>
  </header>
</template>
