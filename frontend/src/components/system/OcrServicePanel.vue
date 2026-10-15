<script setup lang="ts">
import { onMounted, ref } from 'vue'
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
  stopPaddleOCR,
  testOCRConnection,
  type OCRStatusResponse,
} from '@/api/ocr'

const status = ref<OCRStatusResponse | null>(null)
const loading = ref(false)
const busy = ref(false)
const saving = ref(false)

const macUrl = ref('')
const macAuth = ref('')
const paddleUrl = ref('')
const engine = ref('auto')

const testStatus = ref<{ mac?: 'success' | 'failed' | 'testing', paddle?: 'success' | 'failed' | 'testing' }>({})
const testMsg = ref<{ mac?: string, paddle?: string }>({})
const copied = ref(false)

async function loadStatus() {
  loading.value = true
  try {
    const res = await getOCRStatus()
    status.value = res
    macUrl.value = res.macocr_url
    paddleUrl.value = res.paddleocr_url
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
    busy.value = false
  }
}

async function save() {
  saving.value = true
  try {
    status.value = await saveOCRConfig({
      macocr_url: macUrl.value.trim(),
      macocr_auth: macAuth.value.trim(),
      paddleocr_url: paddleUrl.value.trim(),
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
})
</script>

<template>
  <div class="space-y-4">
    <!-- PaddleOCR 本地服务 -->
    <Card>
      <CardHeader class="pb-3">
        <div class="flex items-center justify-between">
          <div>
            <CardTitle class="text-sm font-semibold flex items-center gap-2">
              PaddleOCR 本地服务
              <Badge :variant="status?.paddle_status === 'failed' || (!status?.paddle_installed && !status?.paddle_running) ? 'destructive' : (status?.paddle_ready ? 'default' : 'secondary')" class="text-xs">
                {{ status?.paddle_status === 'starting' ? '启动中' : (status?.paddle_ready ? `运行中 (PID: ${status.paddle_pid})` : (!status?.paddle_installed && !status?.paddle_running ? '未安装' : (status?.paddle_status === 'failed' ? '启动失败' : '已停止'))) }}
              </Badge>
            </CardTitle>
            <CardDescription class="text-xs mt-1">
              位于 packages/paddleocr-server，提供本地高速 PP-OCR 文字识别。
            </CardDescription>
            <p v-if="!status?.paddle_running && status?.paddle_error" class="text-xs text-destructive mt-1">
              {{ status.paddle_error }}
            </p>
          </div>
          <Button
            size="sm"
            :variant="status?.paddle_running ? 'destructive' : 'default'"
            :disabled="busy || loading"
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
        <div class="flex items-center justify-between rounded-lg border bg-muted/30 p-2.5">
          <div>
            <div class="text-[11px] text-muted-foreground">本机服务地址 (PADDLEOCR_URL)：</div>
            <div class="font-mono text-xs font-semibold mt-0.5">
              {{ status?.paddleocr_url || `http://127.0.0.1:${status?.paddle_port || 8031}/upload` }}
            </div>
          </div>
          <div class="flex items-center gap-2">
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
        <p v-if="testMsg.paddle" class="text-xs" :class="testStatus.paddle === 'success' ? 'text-emerald-500' : 'text-destructive'">
          {{ testMsg.paddle }}
        </p>
      </CardContent>
    </Card>

    <!-- OCR 引擎模式与远程地址配置 -->
    <Card>
      <CardHeader class="pb-3">
        <CardTitle class="text-sm font-semibold">OCR 引擎与远程服务配置</CardTitle>
        <CardDescription class="text-xs">
          配置全局默认 OCR 引擎类型，及远程 macOCR / PaddleOCR 服务地址。
        </CardDescription>
      </CardHeader>
      <CardContent class="space-y-4 pt-0">
        <!-- 引擎模式选择 -->
        <div class="space-y-1.5">
          <label class="text-xs font-medium">默认 OCR 引擎模式</label>
          <div class="grid grid-cols-3 gap-2">
            <Button
              size="sm"
              :variant="engine === 'auto' ? 'default' : 'outline'"
              class="text-xs"
              @click="engine = 'auto'"
            >
              自动降级 (auto)
            </Button>
            <Button
              size="sm"
              :variant="engine === 'macocr' ? 'default' : 'outline'"
              class="text-xs"
              @click="engine = 'macocr'"
            >
              优先 macOCR
            </Button>
            <Button
              size="sm"
              :variant="engine === 'paddleocr' ? 'default' : 'outline'"
              class="text-xs"
              @click="engine = 'paddleocr'"
            >
              优先 PaddleOCR
            </Button>
          </div>
          <p class="text-[11px] text-muted-foreground">
            自动模式：优先请求远程 macOCR，若不可用或报错自动平滑降级至本地 PaddleOCR。
          </p>
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
