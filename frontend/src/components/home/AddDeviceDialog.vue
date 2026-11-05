<script setup lang="ts">
import { computed, onUnmounted, ref, watch } from 'vue'
import { toast } from 'vue-sonner'
import { X } from 'lucide-vue-next'
import { useDeviceStore } from '@/stores/devices'
import { Dialog, DialogContent, DialogFooter, DialogHeader, DialogTitle } from '@/components/ui/dialog'
import { Tabs, TabsList, TabsTrigger } from '@/components/ui/tabs'
import { Button } from '@/components/ui/button'
import { Input } from '@/components/ui/input'
import { Switch } from '@/components/ui/switch'
import Ipv4Input from './Ipv4Input.vue'
import { fetchScanNetworks } from '@/api/discovery'
import { useDiscoverySession } from '@/composables/useDeviceDiscovery'
import { formatScanRange } from '@/utils/scanRanges'

const props = defineProps<{ open: boolean }>()
const emit = defineEmits<{ 'update:open': [value: boolean] }>()
const store = useDeviceStore()
const mode = ref('address')
const host = ref(''), port = ref('5555'), alias = ref('')
const start = ref(['', '', '', '']), end = ref(['', '', '', ''])
const ranges = ref<string[]>([])
const ports = ref('5555'), continuous = ref(false), interval = ref(30)
const error = ref(''), saving = ref(false)
const openRef = computed(() => props.open)
const pair = useDiscoverySession(openRef, 'pair', () => store.load(true))
const scan = useDiscoverySession(openRef, 'scan', () => store.load(true))
const active = computed(() => mode.value === 'qr' ? pair : scan)
const busy = computed(() => mode.value === 'address' ? saving.value : active.value.busy.value)
const running = computed(() => active.value.running.value)
const activeError = computed(() => error.value || (mode.value === 'address' ? '' : active.value.error.value))
const job = computed(() => mode.value === 'address' ? null : active.value.job.value)
const status = computed(() => {
  const next = job.value
  if (!next) return ''
  if (next.kind === 'pair') return next.state === 'completed' ? '已连接' : running.value ? '等待手机扫码' : '已停止'
  return `${next.state === 'waiting' ? '等待下一轮' : running.value ? '扫描中' : '已结束'} ${next.tried}/${next.total} · 已连接 ${next.found.length} 台`
})
let generation = 0
let networkRequest: AbortController | undefined
function report(cause: unknown) { error.value = cause instanceof Error ? cause.message : '操作失败'; console.warn('[device-discovery]', error.value) }
function addRange() {
  error.value = ''
  try {
    if (ranges.value.length >= 8) throw new Error('最多添加 8 个范围')
    const value = formatScanRange(start.value, end.value)
    if (ranges.value.includes(value)) throw new Error('该范围已添加')
    ranges.value = [...ranges.value, value]
  }
  catch (cause) { report(cause) }
}
function submitDiscovery() {
  error.value = ''
  if (mode.value === 'qr') { void pair.start(); return }
  try {
    const values = ports.value.split(/[,，\s]+/).filter(Boolean)
    if (!values.length || values.length > 8 || values.some(v => !/^\d+$/.test(v) || +v < 1 || +v > 65535) || new Set(values.map(Number)).size !== values.length) throw new Error('请输入 1 至 8 个不重复的有效端口')
    if (continuous.value && (!Number.isInteger(+interval.value) || +interval.value < 10 || +interval.value > 3600)) throw new Error('扫描间隔须为 10 至 3600 秒')
    // Added ranges are the submitted set; when none are added, scan the editor.
    const inputs = ranges.value.length ? ranges.value : [formatScanRange(start.value, end.value)]
    void scan.start({ ranges: inputs, ports: values.map(Number), continuous: continuous.value, interval_seconds: +interval.value })
  }
  catch (cause) { report(cause) }
}
async function save() {
  if (saving.value) return
  if (!host.value.trim()) { error.value = '请输入设备 IP 或主机名'; return }
  if (!/^\d+$/.test(port.value) || +port.value < 1 || +port.value > 65535) { error.value = '端口须为 1 至 65535'; return }
  const current = generation
  saving.value = true; error.value = ''
  try {
    await store.addDevice(`${host.value.trim()}:${port.value}`, alias.value.trim())
    if (current === generation && props.open) { emit('update:open', false); toast.success('已添加网络设备') }
  }
  catch (cause) { if (current === generation) report(cause) }
  finally { if (current === generation) saving.value = false }
}
watch(openRef, async open => {
  const current = ++generation
  networkRequest?.abort()
  saving.value = false
  if (!open) return
  host.value = ''; port.value = '5555'; alias.value = ''; error.value = ''; mode.value = 'address'
  ranges.value = []; start.value = ['', '', '', '']; end.value = ['', '', '', '']
  const request = new AbortController(); networkRequest = request
  try {
    const networks = await fetchScanNetworks(request.signal)
    if (current !== generation || !props.open) return
    const network = networks[0]
    if (network) { start.value = network.start.split('.'); end.value = network.end.split('.') }
  }
  catch (cause) { if (current === generation && !request.signal.aborted) report(cause) }
}, { immediate: true })
watch(mode, () => { error.value = '' })
onUnmounted(() => { generation++; networkRequest?.abort() })
</script>

<template>
  <Dialog :open="open" @update:open="emit('update:open', $event)">
    <DialogContent class="flex h-[620px] max-h-[calc(100dvh-2rem)] w-[480px] max-w-[calc(100vw-2rem)] flex-col sm:max-w-[480px]" :aria-describedby="undefined">
      <DialogHeader><DialogTitle>添加 Android 设备</DialogTitle></DialogHeader>
      <Tabs v-model="mode" class="shrink-0">
        <TabsList class="grid w-full grid-cols-3">
          <TabsTrigger value="address">地址</TabsTrigger><TabsTrigger value="qr">扫码配对</TabsTrigger><TabsTrigger value="scan">范围扫描</TabsTrigger>
        </TabsList>
      </Tabs>
      <div class="min-h-0 flex-1 overflow-y-auto">
        <form v-show="mode === 'address'" id="add-android-device" class="space-y-4" @submit.prevent="save">
          <div class="grid grid-cols-3 gap-3">
            <div class="col-span-2 space-y-1.5"><label for="add-host" class="text-sm">IP 地址</label><Input id="add-host" v-model="host" :disabled="saving" /></div>
            <div class="space-y-1.5"><label for="add-port" class="text-sm">端口</label><Input id="add-port" v-model="port" inputmode="numeric" :disabled="saving" /></div>
          </div>
          <div class="space-y-1.5"><label for="add-alias" class="text-sm">别名（可选）</label><Input id="add-alias" v-model="alias" :disabled="saving" /></div>
        </form>
        <div v-show="mode === 'qr'" class="flex flex-col items-center gap-3">
          <div class="flex h-60 w-60 items-center justify-center"><img v-if="pair.qrImage.value" :src="pair.qrImage.value" width="240" height="240" alt="Android 无线调试配对二维码" /><span v-else-if="pair.busy.value" role="status" class="text-sm text-muted-foreground">正在生成…</span></div>
          <p class="text-sm text-muted-foreground">手机无线调试 → 使用二维码配对设备</p>
        </div>
        <form v-show="mode === 'scan'" class="space-y-3" @submit.prevent="submitDiscovery">
          <Ipv4Input v-model="start" label="起始" :disabled="scan.running.value || scan.busy.value" />
          <Ipv4Input v-model="end" label="结束" :disabled="scan.running.value || scan.busy.value" />
          <div class="flex justify-end"><Button type="button" variant="outline" size="sm" :disabled="scan.running.value || scan.busy.value" @click="addRange">添加范围</Button></div>
          <ul v-if="ranges.length" class="max-h-24 overflow-y-auto space-y-1">
            <li v-for="(value, index) in ranges" :key="value" class="flex items-center justify-between gap-2 text-xs font-mono"><span>{{ value }}</span><Button type="button" variant="ghost" size="icon" class="h-6 w-6" :aria-label="`删除范围 ${value}`" :disabled="scan.running.value || scan.busy.value" @click="ranges.splice(index, 1)"><X class="h-3 w-3" /></Button></li>
          </ul>
          <div class="flex items-center gap-3"><label for="scan-ports" class="w-10 shrink-0 text-sm">端口</label><Input id="scan-ports" v-model="ports" placeholder="5555,5556" :disabled="scan.running.value || scan.busy.value" /></div>
          <div class="flex items-center justify-between"><label for="scan-continuous" class="text-sm">持续扫描</label><Switch id="scan-continuous" v-model="continuous" :disabled="scan.running.value || scan.busy.value" /></div>
          <div v-if="continuous" class="flex items-center justify-between gap-3"><label for="scan-interval" class="shrink-0 text-sm">间隔（秒）</label><Input id="scan-interval" v-model="interval" type="number" min="10" max="3600" :disabled="scan.running.value || scan.busy.value" /></div>
        </form>
        <p v-if="activeError" role="alert" class="mt-3 text-sm text-destructive">{{ activeError }}</p>
        <p v-if="status" role="status" class="mt-3 text-sm text-muted-foreground">{{ status }}</p>
        <ul v-if="job?.found.length" class="max-h-24 overflow-auto text-sm font-mono"><li v-for="address in job.found" :key="address">{{ address }}</li></ul>
      </div>
      <DialogFooter class="shrink-0">
        <Button variant="outline" @click="emit('update:open', false)">取消</Button>
        <Button v-if="mode === 'address'" type="submit" form="add-android-device" class="bg-blue-600 text-white hover:bg-blue-700" :disabled="busy">保存</Button>
        <Button v-else-if="running" variant="outline" @click="active.stop()">停止</Button>
        <Button v-else class="bg-blue-600 text-white hover:bg-blue-700" :disabled="busy" @click="submitDiscovery">{{ mode === 'qr' ? '重新生成' : '开始扫描' }}</Button>
      </DialogFooter>
    </DialogContent>
  </Dialog>
</template>
