<script setup lang="ts">
import { computed, onUnmounted, ref, watch } from 'vue'
import QRCode from 'qrcode'
import { toast } from 'vue-sonner'
import { useDeviceStore } from '@/stores/devices'
import { Dialog, DialogContent, DialogFooter, DialogHeader, DialogTitle } from '@/components/ui/dialog'
import { Tabs, TabsList, TabsTrigger } from '@/components/ui/tabs'
import { Button } from '@/components/ui/button'
import { Input } from '@/components/ui/input'
import { Switch } from '@/components/ui/switch'
import { pollDiscovery, startPairing, startScan, stopDiscovery, type DiscoveryJob } from '@/api/discovery'

const props = defineProps<{ open: boolean }>()
const emit = defineEmits<{ 'update:open': [value: boolean] }>()
const store = useDeviceStore()
const mode = ref('address')
const host = ref('')
const port = ref('5555')
const alias = ref('')
const range = ref('')
const ports = ref('5555')
const continuous = ref(false)
const interval = ref(30)
const error = ref('')
const busy = ref(false)
const qrImage = ref('')
const job = ref<DiscoveryJob | null>(null)
const running = computed(() => job.value?.state === 'running' || job.value?.state === 'waiting')
const status = computed(() => {
  if (!job.value) return ''
  if (job.value.state === 'failed') return job.value.error
  if (job.value.kind === 'pair') return job.value.state === 'completed' ? '已连接' : running.value ? '等待手机扫码' : '已停止'
  return `${job.value.state === 'waiting' ? '等待下一轮' : running.value ? '扫描中' : '已结束'} ${job.value.tried}/${job.value.total} · 已连接 ${job.value.found.length} 台`
})
let generation = 0
let controller: AbortController | undefined
let timer: ReturnType<typeof setTimeout> | undefined

function report(errorValue: unknown) {
  error.value = errorValue instanceof Error ? errorValue.message : '操作失败'
  console.warn('[device-discovery]', error.value)
}
async function cancelJob() {
  const previous = job.value
  generation++
  controller?.abort()
  controller = undefined
  clearTimeout(timer)
  job.value = null
  qrImage.value = ''
  busy.value = false
  if (previous && (previous.state === 'running' || previous.state === 'waiting')) {
    try { await stopDiscovery(previous.id) }
    catch (cause) { console.warn('[device-discovery] Stop failed; server lease will expire', cause) }
  }
}
async function poll(id: string, current: number, signal: AbortSignal) {
  if (current !== generation || !props.open) return
  try {
    const next = await pollDiscovery(id, signal)
    if (current !== generation || !props.open) return
    job.value = next
    if (next.state === 'running' || next.state === 'waiting')
      timer = setTimeout(() => void poll(id, current, signal), 1000)
    else {
      qrImage.value = ''
      await store.load(true)
    }
  }
  catch (cause) {
    if (current !== generation || signal.aborted) return
    report(cause)
    // End the lease immediately on a failed poll instead of leaving hidden work.
    await cancelJob()
  }
}
async function submitDiscovery() {
  if (busy.value || running.value) return
  await cancelJob()
  const current = generation
  const request = new AbortController()
  controller = request
  busy.value = true
  error.value = ''
  try {
    let next: DiscoveryJob
    if (mode.value === 'qr') next = await startPairing(request.signal)
    else {
      const values = ports.value.split(/[,，\s]+/).filter(Boolean)
      if (!values.length || values.some(value => !/^\d+$/.test(value))) throw new Error('请输入端口，多个端口用逗号分隔')
      next = await startScan({ range: range.value.trim(), ports: values.map(Number), continuous: continuous.value, interval_seconds: Number(interval.value) }, request.signal)
    }
    if (current !== generation || !props.open) {
      await stopDiscovery(next.id)
      return
    }
    job.value = next
    if (mode.value === 'qr') {
      if (!next.qr) throw new Error('配对二维码未生成')
      const image = await QRCode.toDataURL(next.qr, { width: 240, margin: 2, errorCorrectionLevel: 'M' })
      if (current !== generation || !props.open) return
      qrImage.value = image
    }
    timer = setTimeout(() => void poll(next.id, current, request.signal), 500)
  }
  catch (cause) {
    if (current === generation && !request.signal.aborted) {
      report(cause)
      await cancelJob()
    }
  }
  finally { if (current === generation) busy.value = false }
}
async function save() {
  if (busy.value) return
  if (!host.value.trim()) { error.value = '请输入设备 IP 或主机名'; return }
  if (!/^\d+$/.test(port.value) || Number(port.value) < 1 || Number(port.value) > 65535) { error.value = '端口须为 1 至 65535'; return }
  const current = generation
  busy.value = true
  error.value = ''
  try {
    await store.addDevice(`${host.value.trim()}:${port.value}`, alias.value.trim())
    if (current === generation && props.open) {
      emit('update:open', false)
      toast.success('已添加网络设备')
    }
  }
  catch (cause) { if (current === generation) report(cause) }
  finally { if (current === generation) busy.value = false }
}
watch(() => props.open, open => {
  if (!open) { void cancelJob(); return }
  host.value = ''; port.value = '5555'; alias.value = ''; error.value = ''; mode.value = 'address'
})
watch(mode, () => { error.value = ''; void cancelJob() })
onUnmounted(() => { void cancelJob() })
</script>

<template>
  <Dialog :open="open" @update:open="emit('update:open', $event)">
    <DialogContent class="sm:max-w-md" :aria-describedby="undefined">
      <DialogHeader><DialogTitle>添加 Android 设备</DialogTitle></DialogHeader>
      <Tabs v-model="mode">
        <TabsList class="grid w-full grid-cols-3">
          <TabsTrigger value="address">地址</TabsTrigger><TabsTrigger value="qr">扫码配对</TabsTrigger><TabsTrigger value="scan">范围扫描</TabsTrigger>
        </TabsList>
      </Tabs>
      <form v-if="mode === 'address'" id="add-android-device" class="space-y-4" @submit.prevent="save">
        <div class="grid grid-cols-3 gap-3">
          <div class="col-span-2 space-y-1.5"><label for="add-host" class="text-sm">IP 地址</label><Input id="add-host" v-model="host" placeholder="192.168.1.100" :disabled="busy" /></div>
          <div class="space-y-1.5"><label for="add-port" class="text-sm">端口</label><Input id="add-port" v-model="port" inputmode="numeric" :disabled="busy" /></div>
        </div>
        <div class="space-y-1.5"><label for="add-alias" class="text-sm">别名（可选）</label><Input id="add-alias" v-model="alias" :disabled="busy" /></div>
      </form>
      <div v-else-if="mode === 'qr'" class="flex flex-col items-center gap-3">
        <img v-if="qrImage" :src="qrImage" width="240" height="240" alt="Android 无线调试配对二维码" />
        <p class="text-sm text-muted-foreground">手机无线调试 → 使用二维码配对设备</p>
      </div>
      <form v-else class="space-y-4" @submit.prevent="submitDiscovery">
        <div class="space-y-1.5"><label for="scan-range" class="text-sm">IP 范围</label><Input id="scan-range" v-model="range" placeholder="192.168.1.100到200" :disabled="running || busy" /></div>
        <div class="space-y-1.5"><label for="scan-ports" class="text-sm">端口</label><Input id="scan-ports" v-model="ports" placeholder="5555,5556" :disabled="running || busy" /></div>
        <div class="flex items-center justify-between"><label for="scan-continuous" class="text-sm">持续扫描</label><Switch id="scan-continuous" v-model="continuous" :disabled="running || busy" /></div>
        <div v-if="continuous" class="flex items-center justify-between gap-3"><label for="scan-interval" class="shrink-0 text-sm">间隔（秒）</label><Input id="scan-interval" v-model="interval" type="number" min="10" max="3600" :disabled="running || busy" /></div>
      </form>
      <p v-if="error" role="alert" class="text-sm text-destructive">{{ error }}</p>
      <p v-if="status" role="status" class="text-sm text-muted-foreground">{{ status }}</p>
      <ul v-if="job?.found.length" class="max-h-28 overflow-auto text-sm font-mono"><li v-for="address in job.found" :key="address">{{ address }}</li></ul>
      <DialogFooter>
        <Button variant="outline" @click="emit('update:open', false)">取消</Button>
        <Button v-if="mode === 'address'" type="submit" form="add-android-device" class="bg-blue-600 text-white hover:bg-blue-700" :disabled="busy">保存</Button>
        <Button v-else-if="running" variant="outline" @click="cancelJob">停止</Button>
        <Button v-else class="bg-blue-600 text-white hover:bg-blue-700" :disabled="busy" @click="submitDiscovery">{{ mode === 'qr' ? '生成二维码' : '开始扫描' }}</Button>
      </DialogFooter>
    </DialogContent>
  </Dialog>
</template>
