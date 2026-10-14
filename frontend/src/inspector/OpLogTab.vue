<script setup lang="ts">
import { Check, Copy, Trash2 } from 'lucide-vue-next'
import { onMounted, ref } from 'vue'
import { Button } from '@/components/ui/button'
import { ScrollArea } from '@/components/ui/scroll-area'
import { clearOps, loadOps, opClearing, opEntries, opError, opLoadFailed, opLoading, opMode } from './opLog'

const copied = ref<number | null>(null)
onMounted(() => { void loadOps() })

async function copyEntry(i: number, text: string) {
  try {
    await navigator.clipboard.writeText(text)
    copied.value = i
    setTimeout(() => {
      if (copied.value === i)
        copied.value = null
    }, 1200)
  }
  catch { /* 忽略 */ }
}
</script>

<template>
  <div class="flex h-full flex-col gap-2 p-3">
    <div class="flex gap-1.5">
      <Button size="sm" :variant="opMode === 'desc' ? 'secondary' : 'ghost'" @click="opMode = 'desc'">描述</Button>
      <Button size="sm" :variant="opMode === 'adb' ? 'secondary' : 'ghost'" @click="opMode = 'adb'">ADB 命令</Button>
      <Button size="sm" variant="ghost" class="ml-auto" :disabled="!opEntries.length || opClearing || opLoading" @click="clearOps()">
        <Trash2 class="h-3.5 w-3.5" />清空
      </Button>
    </div>
    <div v-if="opError" class="flex items-center gap-2 text-xs text-destructive">
      <span>{{ opError }}</span>
      <Button v-if="opLoadFailed" size="sm" variant="ghost" :disabled="opLoading || opClearing" @click="loadOps()">重试</Button>
    </div>
    <ScrollArea v-if="opEntries.length" class="min-h-0 flex-1 rounded-md border">
      <ul class="divide-y font-mono text-xs">
        <li v-for="(entry, i) in opEntries" :key="entry.id" class="flex items-start gap-2 p-2">
          <span class="shrink-0 text-muted-foreground">{{ entry.time }}</span>
          <span class="min-w-0 flex-1 break-all">{{ opMode === 'desc' ? entry.desc : entry.adb }}</span>
          <Button
            variant="ghost" size="icon" class="h-6 w-6 shrink-0" aria-label="复制这条记录"
            @click="copyEntry(i, opMode === 'desc' ? entry.desc : entry.adb)"
          >
            <Check v-if="copied === i" class="h-3.5 w-3.5 text-green-600" />
            <Copy v-else class="h-3.5 w-3.5" />
          </Button>
        </li>
      </ul>
    </ScrollArea>
    <p v-else-if="opLoading" class="text-xs text-muted-foreground">加载中…</p>
    <p v-else-if="!opError" class="text-xs text-muted-foreground">暂无操作记录。在投屏画面上点击、滑动或按键后会显示在这里。</p>
  </div>
</template>
