<script setup lang="ts">
import { FitAddon } from '@xterm/addon-fit'
import { Terminal } from '@xterm/xterm'
import { computed, onMounted, onUnmounted, ref } from 'vue'
import SidePanelHeader from '@/components/layout/SidePanelHeader.vue'
import { Button } from '@/components/ui/button'
import { useAdbShell } from './useAdbShell'
import '@xterm/xterm/css/xterm.css'

const props = defineProps<{ serial: string }>()
const emit = defineEmits<{ close: [] }>()
const terminalEl = ref<HTMLElement | null>(null)
const shell = useAdbShell(props.serial)
const connected = computed(() => shell.status.value === 'connected')
const failed = computed(() => shell.status.value === 'error')
let terminal: Terminal | undefined
let fit: FitAddon | undefined
let observer: ResizeObserver | undefined

function fitTerminal() {
  if (!terminal || !fit) return
  try { fit.fit(); shell.resize(terminal.cols, terminal.rows) } catch { /* teardown */ }
}

onMounted(() => {
  terminal = new Terminal({ cursorBlink: true, convertEol: false, scrollback: 5000, fontSize: 13, theme: { background: '#18202d', foreground: '#e8edf7' } })
  fit = new FitAddon()
  terminal.loadAddon(fit)
  terminal.open(terminalEl.value!)
  terminal.onData(shell.input)
  shell.configure({ output: data => terminal?.write(data), resize: fitTerminal })
  observer = new ResizeObserver(fitTerminal)
  observer.observe(terminalEl.value!)
  fitTerminal()
  shell.connect()
})
onUnmounted(() => { shell.close(); observer?.disconnect(); terminal?.dispose(); terminal = undefined })
</script>

<template>
  <aside class="relative flex h-full min-h-0 w-full flex-col overflow-hidden bg-[#18202d] text-[#e8edf7]" aria-label="ADB Shell 终端">
    <SidePanelHeader
      class="border-[#303b4d] text-[#e8edf7]"
      title="ADB Shell"
      button-class="text-[#e8edf7] hover:bg-[#31405a]"
      @close="emit('close')"
    >
      <template #title>
        <div class="flex min-w-0 items-center gap-2">
          <span class="h-2 w-2 shrink-0 rounded-full" :class="connected ? 'bg-green-500' : failed ? 'bg-red-500' : 'bg-gray-500'" />
          <strong class="truncate text-sm font-semibold">ADB Shell</strong>
        </div>
      </template>
      <template #actions>
        <Button variant="ghost" size="sm" class="h-7 px-2 text-[#e8edf7] hover:bg-[#31405a]" @click="terminal?.clear()">清屏</Button>
      </template>
    </SidePanelHeader>
    <div ref="terminalEl" class="min-h-0 flex-1 p-2" />
    <div v-if="failed && shell.errorMessage" class="absolute inset-0 flex items-center justify-center bg-[#18202d]/90">
      <div class="text-center text-sm"><p>{{ shell.errorMessage }}</p><Button class="mt-3" size="sm" @click="shell.retry()">重新连接</Button></div>
    </div>
  </aside>
</template>
