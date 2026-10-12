<script setup lang="ts">
import { History, X } from 'lucide-vue-next'
import { Button } from '@/components/ui/button'
import { Dialog, DialogClose, DialogContent, DialogTitle } from '@/components/ui/dialog'
import ExecutionRecords from './ExecutionRecords.vue'

defineProps<{ scriptId?: number, initialTask: string }>()
const open = defineModel<boolean>('open', { required: true })
</script>

<template>
  <Dialog v-model:open="open">
    <DialogContent
      class="flex h-[calc(100dvh-1rem)] w-[calc(100vw-1rem)] max-w-none flex-col gap-0 overflow-hidden p-0 sm:h-[85dvh] sm:max-h-[860px] sm:w-[calc(100vw-4rem)] sm:max-w-[1200px]"
      :show-close-button="false"
      :aria-describedby="undefined"
    >
      <header class="flex shrink-0 items-center gap-2 border-b px-4 py-3">
        <History class="size-4 text-muted-foreground" />
        <DialogTitle class="flex-1 text-sm font-medium">执行记录</DialogTitle>
        <DialogClose as-child><Button variant="ghost" size="icon-sm" aria-label="关闭执行记录"><X class="size-4" /></Button></DialogClose>
      </header>
      <ExecutionRecords :script-id="scriptId" :active="open" :initial-task="initialTask" />
    </DialogContent>
  </Dialog>
</template>
