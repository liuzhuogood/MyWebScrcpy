<script setup lang="ts">
import { FitAddon } from '@xterm/addon-fit'
import { Terminal } from '@xterm/xterm'
import { computed, onMounted, onUnmounted, ref } from 'vue'
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
  <aside class="relative flex h-full min-h-0 w-full flex-col overflow-hidden rounded-xl border bg-[#18202d] text-[#e8edf7]" aria-label="ADB Shell 终端">
    <header class="flex h-11 shrink-0 items-center gap-2 border-b border-[#303b4d] px-3">
      <span class="h-2 w-2 rounded-full" :class="connected ? 'bg-green-500' : failed ? 'bg-red-500' : 'bg-gray-500'" />
      <strong class="text-sm">ADB Shell</strong>
      <div class="ml-auto flex gap-2">
        <Button variant="ghost" size="sm" class="text-[#e8edf7] hover:bg-[#31405a]" @click="terminal?.clear()">清屏</Button>
        <Button variant="ghost" size="sm" class="text-[#e8edf7] hover:bg-[#31405a]" @click="emit('close')">关闭</Button>
      </div>
    </header>
    <div ref="terminalEl" class="min-h-0 flex-1 p-2" />
    <div v-if="failed && shell.errorMessage" class="absolute inset-0 flex items-center justify-center bg-[#18202d]/90">
      <div class="text-center text-sm"><p>{{ shell.errorMessage }}</p><Button class="mt-3" size="sm" @click="shell.retry()">重新连接</Button></div>
    </div>
  </aside>
</template>
