<script setup lang="ts">
import type { Device } from '@/api/devices'
import { ContextMenu, ContextMenuTrigger, ContextMenuContent, ContextMenuItem, ContextMenuSeparator } from '@/components/ui/context-menu'
defineProps<{ device: Device, mockEnabled?: boolean }>()
const emit = defineEmits<{ action: [action: 'hide' | 'delete' | 'open' | 'detail'] }>()
</script>
<template>
  <ContextMenu>
    <ContextMenuTrigger as-child><slot /></ContextMenuTrigger>
    <ContextMenuContent class="z-[10000] w-36">
      <ContextMenuItem :disabled="device.state !== 'device' || mockEnabled" @select="emit('action', 'open')">打开小窗</ContextMenuItem>
      <ContextMenuItem :disabled="device.state !== 'device' || mockEnabled" @select="emit('action', 'detail')">进入详情</ContextMenuItem>
      <ContextMenuSeparator />
      <ContextMenuItem @select="emit('action', 'hide')">隐藏</ContextMenuItem>
      <ContextMenuItem :disabled="device.source !== 'manual' || mockEnabled" class="text-destructive" @select="emit('action', 'delete')">删除</ContextMenuItem>
    </ContextMenuContent>
  </ContextMenu>
</template>
