<script setup lang="ts">
import { computed, onMounted, onUnmounted, ref } from 'vue'
import { Check, Copy, Loader2, Play, RefreshCw, Square } from 'lucide-vue-next'
import { toast } from 'vue-sonner'
import { Badge } from '@/components/ui/badge'
import { Button } from '@/components/ui/button'
import { Card, CardContent, CardDescription, CardHeader, CardTitle } from '@/components/ui/card'
import { Input } from '@/components/ui/input'
import {
  getOCRStatus,
  saveOCRConfig,
  startPaddleOCR,
  startIndraOCR,
  stopIndraOCR,
  stopPaddleOCR,
  testOCRConnection,
  type OCRStatusResponse,
} from '@/api/ocr'

const status = ref<OCRStatusResponse | null>(null)
const loading = ref(false)
const busy = ref(false)
const indraBusy = ref(false)
const indraPreparing = computed(() => status.value?.indra_status === 'installing' || status.value?.indra_status === 'starting')
const saving = ref(false)
const preparing = computed(() => status.value?.paddle_status === 'installing' || status.value?.paddle_status === 'starting')
let pollTimer: ReturnType<typeof setTimeout> | undefined
let disposed = false
let statusEpoch = 0

const macUrl = ref('')
const macAuth = ref('')
const paddleUrl = ref('')
const indraUrl = ref('')
const engine = ref('auto')

const testStatus = ref<{ mac?: 'success' | 'failed' | 'testing', paddle?: 'success' | 'failed' | 'testing', indra?: 'success' | 'failed' | 'testing' }>({})
const testMsg = ref<{ mac?: string, paddle?: string, indra?: string }>({})
const copied = ref(false)

async function loadStatus() {
  loading.value = true
  try {
    const res = await getOCRStatus()
    status.value = res
    macUrl.value = res.macocr_url
    paddleUrl.value = res.paddleocr_url
    indraUrl.value = res.indraocr_url
    engine.value = res.engine || 'auto'
  }
  catch (e) {
    toast.error(e instanceof Error ? e.message : '获取 OCR 状态失败')
  }
  finally {
    loading.value = false
  }
}

async function togglePaddle() {
  statusEpoch++
  busy.value = true
  try {
    if (status.value?.paddle_running) {
      status.value = await stopPaddleOCR()
      toast.success('PaddleOCR 服务已停止')
    }
    else {
      status.value = await startPaddleOCR()
      toast.success('PaddleOCR 服务已启动')
    }
  }
  catch (e) {
    toast.error(e instanceof Error ? e.message : '操作失败')
  }
  finally {
    statusEpoch++
    busy.value = false
  }
}

async function toggleIndra() {
  statusEpoch++
  indraBusy.value = true
  try {
    status.value = status.value?.indra_running ? await stopIndraOCR() : await startIndraOCR()
    toast.success(status.value.indra_running ? 'Indra OCR 服务已启动' : 'Indra OCR 服务已停止')
  }
  catch (e) { toast.error(e instanceof Error ? e.message : '操作失败') }
  finally { statusEpoch++; indraBusy.value = false }
}

async function testIndra() {
  testStatus.value.indra = 'testing'
  testMsg.value.indra = ''
  try {
    const res = await testOCRConnection(indraUrl.value)
    testStatus.value.indra = 'success'
    testMsg.value.indra = `响应正常 (${res.elapsed_ms}ms)`
  }
  catch (e) {
    testStatus.value.indra = 'failed'
    testMsg.value.indra = e instanceof Error ? e.message : '连接失败'
  }
}

async function save() {
  saving.value = true
  try {
    status.value = await saveOCRConfig({
      macocr_url: macUrl.value.trim(),
      macocr_auth: macAuth.value.trim(),
      paddleocr_url: paddleUrl.value.trim(),
      indraocr_url: indraUrl.value.trim(),
      engine: engine.value,
    })
    toast.success('OCR 配置已保存')
  }
  catch (e) {
    toast.error(e instanceof Error ? e.message : '保存失败')
  }
  finally {
    saving.value = false
  }
}

async function testPaddle() {
  const url = paddleUrl.value || status.value?.paddleocr_url || 'http://127.0.0.1:8031/upload'
  testStatus.value.paddle = 'testing'
  testMsg.value.paddle = ''
  try {
    const res = await testOCRConnection(url)
    testStatus.value.paddle = 'success'
    testMsg.value.paddle = `响应正常 (${res.elapsed_ms}ms)`
  }
  catch (e) {
    testStatus.value.paddle = 'failed'
    testMsg.value.paddle = e instanceof Error ? e.message : '连接失败'
  }
}

async function testMac() {
  testStatus.value.mac = 'testing'
  testMsg.value.mac = ''
  try {
    const res = await testOCRConnection(macUrl.value, macAuth.value)
    testStatus.value.mac = 'success'
    testMsg.value.mac = `响应正常 (${res.elapsed_ms}ms)`
  }
  catch (e) {
    testStatus.value.mac = 'failed'
    testMsg.value.mac = e instanceof Error ? e.message : '连接失败'
  }
}

function copyAddress(text: string) {
  navigator.clipboard.writeText(text).then(() => {
    copied.value = true
    toast.success('已复制服务地址')
    setTimeout(() => { copied.value = false }, 1500)
  })
}

onMounted(() => {
  void loadStatus()
  async function poll() {
    const epoch = statusEpoch
    try {
      const current = await getOCRStatus()
      if (!disposed && epoch === statusEpoch) status.value = current
    }
    catch { /* Keep the last confirmed status; explicit operations report errors. */ }
    if (!disposed) pollTimer = setTimeout(poll, 2500)
  }
  pollTimer = setTimeout(poll, 2500)
})
onUnmounted(() => { disposed = true; if (pollTimer) clearTimeout(pollTimer) })
</script>

<template>
  <div class="space-y-4">
    <!-- Indra OCR 本地服务 -->
    <Card>
      <CardHeader class="pb-3">
        <div class="flex flex-wrap items-center justify-between gap-3">
          <div class="min-w-0 flex-1 basis-48">
            <CardTitle class="text-sm font-semibold flex flex-wrap items-center gap-2">
              Indra OCR 本地服务
              <Badge :variant="!indraPreparing && (status?.indra_status === 'failed' || (!status?.indra_installed && !status?.indra_running)) ? 'destructive' : (status?.indra_ready ? 'default' : 'secondary')" class="text-xs">
                {{ status?.indra_status === 'installing' ? '准备依赖中' : (status?.indra_status === 'starting' ? '启动中' : (status?.indra_ready ? `运行中 (PID: ${status.indra_pid})` : (status?.indra_status === 'failed' ? '启动失败' : (!status?.indra_installed ? '未安装' : '已停止')))) }}
              </Badge>
            </CardTitle>
            <p v-if="!status?.indra_running && status?.indra_error" class="text-xs text-destructive mt-1 [overflow-wrap:anywhere]">
              {{ status.indra_error }}
            </p>
          </div>
          <Button
            size="sm"
            class="shrink-0"
            :variant="status?.indra_running ? 'destructive' : 'default'"
            :disabled="indraBusy || loading || indraPreparing"
            @click="toggleIndra"
          >
            <Loader2 v-if="indraBusy" class="mr-1.5 h-3.5 w-3.5 animate-spin" />
            <Square v-else-if="status?.indra_running" class="mr-1.5 h-3.5 w-3.5 fill-current" />
            <Play v-else class="mr-1.5 h-3.5 w-3.5 fill-current" />
            {{ status?.indra_running ? '停止服务' : '启动服务' }}
          </Button>
        </div>
      </CardHeader>

      <CardContent class="space-y-3 pt-0">
        <!-- 本机服务地址展示与复制 -->
        <div class="flex flex-wrap items-center justify-between gap-3 rounded-lg border bg-muted/30 p-2.5">
          <div class="min-w-0 flex-1 basis-48">
            <div class="text-[11px] text-muted-foreground">本机服务地址 (INDRAOCR_URL)：</div>
            <div class="font-mono text-xs font-semibold mt-0.5 [overflow-wrap:anywhere]">
              {{ status?.indraocr_url || `http://127.0.0.1:${status?.indra_port || 8032}/upload` }}
            </div>
          </div>
          <div class="flex shrink-0 items-center gap-2">
            <Button
              size="sm"
              variant="outline"
              class="h-7 text-xs"
              @click="copyAddress(status?.indraocr_url || `http://127.0.0.1:${status?.indra_port || 8032}/upload`)"
            >
              <Check v-if="copied" class="mr-1 h-3 w-3 text-emerald-500" />
              <Copy v-else class="mr-1 h-3 w-3" />
              {{ copied ? '已复制' : '复制地址' }}
            </Button>
            <Button
              size="sm"
              variant="ghost"
              class="h-7 text-xs"
              :disabled="testStatus.indra === 'testing'"
              @click="testIndra"
            >
              <RefreshCw v-if="testStatus.indra === 'testing'" class="mr-1 h-3 w-3 animate-spin" />
              测试
            </Button>
          </div>
        </div>
        <p v-if="testMsg.indra" class="text-xs [overflow-wrap:anywhere]" :class="testStatus.indra === 'success' ? 'text-emerald-500' : 'text-destructive'">
          {{ testMsg.indra }}
        </p>
      </CardContent>
    </Card>

    <!-- PaddleOCR 本地服务 -->
    <Card>
      <CardHeader class="pb-3">
        <div class="flex flex-wrap items-center justify-between gap-3">
          <div class="min-w-0 flex-1 basis-48">
            <CardTitle class="text-sm font-semibold flex flex-wrap items-center gap-2">
              PaddleOCR 本地服务
              <Badge :variant="!preparing && (status?.paddle_status === 'failed' || (!status?.paddle_installed && !status?.paddle_running)) ? 'destructive' : (status?.paddle_ready ? 'default' : 'secondary')" class="text-xs">
                {{ status?.paddle_status === 'installing' ? '准备依赖中' : (status?.paddle_status === 'starting' ? '启动中' : (status?.paddle_ready ? `运行中 (PID: ${status.paddle_pid})` : (status?.paddle_status === 'failed' ? '启动失败' : (!status?.paddle_installed ? '未安装' : '已停止')))) }}
              </Badge>
            </CardTitle>
            <p v-if="!status?.paddle_running && status?.paddle_error" class="text-xs text-destructive mt-1 [overflow-wrap:anywhere]">
              {{ status.paddle_error }}
            </p>
          </div>
          <Button
            size="sm"
            class="shrink-0"
            :variant="status?.paddle_running ? 'destructive' : 'default'"
            :disabled="busy || loading || preparing"
            @click="togglePaddle"
          >
            <Loader2 v-if="busy" class="mr-1.5 h-3.5 w-3.5 animate-spin" />
            <Square v-else-if="status?.paddle_running" class="mr-1.5 h-3.5 w-3.5 fill-current" />
            <Play v-else class="mr-1.5 h-3.5 w-3.5 fill-current" />
            {{ status?.paddle_running ? '停止服务' : '启动服务' }}
          </Button>
        </div>
      </CardHeader>

      <CardContent class="space-y-3 pt-0">
        <!-- 本机服务地址展示与复制 -->
        <div class="flex flex-wrap items-center justify-between gap-3 rounded-lg border bg-muted/30 p-2.5">
          <div class="min-w-0 flex-1 basis-48">
            <div class="text-[11px] text-muted-foreground">本机服务地址 (PADDLEOCR_URL)：</div>
            <div class="font-mono text-xs font-semibold mt-0.5 [overflow-wrap:anywhere]">
              {{ status?.paddleocr_url || `http://127.0.0.1:${status?.paddle_port || 8031}/upload` }}
            </div>
          </div>
          <div class="flex shrink-0 items-center gap-2">
            <Button
              size="sm"
              variant="outline"
              class="h-7 text-xs"
              @click="copyAddress(status?.paddleocr_url || `http://127.0.0.1:${status?.paddle_port || 8031}/upload`)"
            >
              <Check v-if="copied" class="mr-1 h-3 w-3 text-emerald-500" />
              <Copy v-else class="mr-1 h-3 w-3" />
              {{ copied ? '已复制' : '复制地址' }}
            </Button>
            <Button
              size="sm"
              variant="ghost"
              class="h-7 text-xs"
              :disabled="testStatus.paddle === 'testing'"
              @click="testPaddle"
            >
              <RefreshCw v-if="testStatus.paddle === 'testing'" class="mr-1 h-3 w-3 animate-spin" />
              测试
            </Button>
          </div>
        </div>
        <p v-if="testMsg.paddle" class="text-xs [overflow-wrap:anywhere]" :class="testStatus.paddle === 'success' ? 'text-emerald-500' : 'text-destructive'">
          {{ testMsg.paddle }}
        </p>
      </CardContent>
    </Card>

    <!-- OCR 引擎模式与远程地址配置 -->
    <Card>
      <CardHeader class="pb-3">
        <CardTitle class="text-sm font-semibold">OCR 引擎与远程服务配置</CardTitle>
        <CardDescription class="text-xs">
          配置全局默认 OCR 引擎类型，及各引擎的服务地址。
        </CardDescription>
      </CardHeader>
      <CardContent class="space-y-4 pt-0">
        <!-- 引擎模式选择 -->
        <div class="space-y-1.5">
          <label class="text-xs font-medium">默认 OCR 引擎模式</label>
          <div class="grid grid-cols-2 gap-2 sm:grid-cols-4">
            <Button
              size="sm"
              :variant="engine === 'auto' ? 'default' : 'outline'"
              class="text-xs"
              @click="engine = 'auto'"
            >
              自动降级 (auto)
            </Button>
            <Button size="sm" :variant="engine === 'parallel' ? 'default' : 'outline'" class="text-xs" @click="engine = 'parallel'">
              并行
            </Button>
            <Button size="sm" :variant="engine === 'indraocr' ? 'default' : 'outline'" class="text-xs" @click="engine = 'indraocr'">
              Indra OCR
            </Button>
            <Button
              size="sm"
              :variant="engine === 'macocr' ? 'default' : 'outline'"
              class="text-xs"
              @click="engine = 'macocr'"
            >
              macOCR
            </Button>
            <Button
              size="sm"
              :variant="engine === 'paddleocr' ? 'default' : 'outline'"
              class="text-xs"
              @click="engine = 'paddleocr'"
            >
              PaddleOCR
            </Button>
          </div>
          <p class="text-[11px] text-muted-foreground">
            自动模式：Indra OCR → macOCR → PaddleOCR，不可用时依次切换。
          </p>
        </div>

        <div class="space-y-1.5 pt-2 border-t">
          <label class="text-xs font-medium">INDRAOCR_URL</label>
          <Input v-model="indraUrl" placeholder="http://127.0.0.1:8032/upload" class="font-mono text-xs" />
        </div>

        <!-- MACOCR_URL 配置 -->
        <div class="space-y-1.5 pt-2 border-t">
          <div class="flex items-center justify-between">
            <label class="text-xs font-medium">MACOCR_URL</label>
            <Button
              size="sm"
              variant="link"
              class="h-auto p-0 text-xs"
              :disabled="testStatus.mac === 'testing'"
              @click="testMac"
            >
              <RefreshCw v-if="testStatus.mac === 'testing'" class="mr-1 h-3 w-3 animate-spin" />
              测试连接
            </Button>
          </div>
          <Input v-model="macUrl" placeholder="http://10.0.0.11:8000/upload" class="font-mono text-xs" />
          <p v-if="testMsg.mac" class="text-xs" :class="testStatus.mac === 'success' ? 'text-emerald-500' : 'text-destructive'">
            {{ testMsg.mac }}
          </p>
        </div>

        <!-- MACOCR_AUTH 配置 -->
        <div class="space-y-1.5">
          <label class="text-xs font-medium">MACOCR_AUTH (可选认证凭据)</label>
          <Input v-model="macAuth" type="password" placeholder="用户名:密码 或 Bearer Token" class="font-mono text-xs" />
        </div>

        <!-- PADDLEOCR_URL 覆盖配置 -->
        <div class="space-y-1.5 pt-2 border-t">
          <label class="text-xs font-medium">PADDLEOCR_URL (服务地址覆盖)</label>
          <Input v-model="paddleUrl" placeholder="http://127.0.0.1:8031/upload" class="font-mono text-xs" />
          <p class="text-[11px] text-muted-foreground">
            留空时默认使用本机启动地址；如局域网有独立 GPU 机器可填入远程地址。
          </p>
        </div>

        <!-- 保存按钮 -->
        <div class="flex justify-end pt-2">
          <Button size="sm" :disabled="saving" @click="save">
            <Loader2 v-if="saving" class="mr-1.5 h-3.5 w-3.5 animate-spin" />
            保存配置
          </Button>
        </div>
      </CardContent>
    </Card>
  </div>
</template>
