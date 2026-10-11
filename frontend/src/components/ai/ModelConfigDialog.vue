<script setup lang="ts">
import { onMounted, ref, watch } from 'vue'
import {
  AlertCircle,
  Bot,
  CheckCircle2,
  Key,
  Layers,
  Loader2,
  Save,
  Server,
  Zap,
} from 'lucide-vue-next'
import { Button } from '@/components/ui/button'
import {
  Dialog,
  DialogContent,
  DialogDescription,
  DialogFooter,
  DialogHeader,
  DialogTitle,
} from '@/components/ui/dialog'
import { Input } from '@/components/ui/input'
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from '@/components/ui/select'
import { getAIConfig, saveAIConfig, testAIConnection, type AIModelConfig } from '@/api/ai'

const props = defineProps<{
  open: boolean
}>()

const emit = defineEmits<{
  (e: 'update:open', val: boolean): void
  (e: 'saved'): void
}>()

const form = ref<AIModelConfig>({
  provider: 'deepseek',
  base_url: 'https://api.deepseek.com/v1',
  api_key: '',
  model: 'deepseek-chat',
})

const isLoading = ref(false)
const isSaving = ref(false)
const isTesting = ref(false)
const testResult = ref<{ success: boolean, msg: string } | null>(null)

// 常用服务商预设
const PRESETS: Record<string, { base_url: string, model: string }> = {
  deepseek: {
    base_url: 'https://api.deepseek.com/v1',
    model: 'deepseek-chat',
  },
  openai: {
    base_url: 'https://api.openai.com/v1',
    model: 'gpt-4o-mini',
  },
  ollama: {
    base_url: 'http://localhost:11434/v1',
    model: 'qwen2.5:7b',
  },
  custom: {
    base_url: 'https://api.openai-proxy.org/v1',
    model: 'gpt-4o',
  },
}

function handleProviderChange(val: any) {
  const provider = String(val)
  form.value.provider = provider as any
  if (PRESETS[provider]) {
    form.value.base_url = PRESETS[provider].base_url
    form.value.model = PRESETS[provider].model
  }
}

async function loadConfig() {
  isLoading.value = true
  try {
    const data = await getAIConfig()
    form.value = {
      provider: data.provider || 'deepseek',
      base_url: data.base_url || 'https://api.deepseek.com/v1',
      api_key: data.api_key || '',
      model: data.model || 'deepseek-chat',
    }
  } catch (err) {
    console.error('加载 AI 模型配置失败', err)
  } finally {
    isLoading.value = false
  }
}

async function handleTest() {
  isTesting.value = true
  testResult.value = null
  try {
    const res = await testAIConnection(form.value)
    testResult.value = { success: true, msg: res.message || '连接成功！' }
  } catch (err: any) {
    testResult.value = { success: false, msg: err?.message || '连接失败，请检查 Base URL 与 API Key' }
  } finally {
    isTesting.value = false
  }
}

async function handleSave() {
  isSaving.value = true
  try {
    await saveAIConfig(form.value)
    emit('saved')
    emit('update:open', false)
  } catch (err: any) {
    alert(err?.message || '保存配置失败')
  } finally {
    isSaving.value = false
  }
}

watch(() => props.open, (val) => {
  if (val) {
    loadConfig()
    testResult.value = null
  }
})

onMounted(() => {
  if (props.open) {
    loadConfig()
  }
})
</script>

<template>
  <Dialog :open="open" @update:open="emit('update:open', $event)">
    <DialogContent class="sm:max-w-[540px]">
      <DialogHeader>
        <DialogTitle class="flex items-center gap-2">
          <Bot class="h-5 w-5 text-primary" />
          Pi 引擎 · AI 模型配置 (OpenAI 兼容)
        </DialogTitle>
        <DialogDescription>
          配置驱动 Pi 助手的大语言模型。支持 DeepSeek、OpenAI、Ollama 本地模型及兼容 OpenAI 协议的任何聚合中转站。
        </DialogDescription>
      </DialogHeader>

      <div class="space-y-4 py-2">
        <!-- 提供商预设选择 -->
        <div class="space-y-1.5">
          <label class="text-xs font-semibold flex items-center gap-1.5 text-foreground">
            <Layers class="h-3.5 w-3.5 text-primary" />
            模型提供商 (Provider)
          </label>
          <Select :model-value="form.provider" @update:model-value="handleProviderChange">
            <SelectTrigger class="h-8 text-xs">
              <SelectValue placeholder="选择模型服务商" />
            </SelectTrigger>
            <SelectContent>
              <SelectItem value="deepseek">DeepSeek (官方推荐，高性价比)</SelectItem>
              <SelectItem value="openai">OpenAI (官方接口)</SelectItem>
              <SelectItem value="ollama">Ollama (本地私有模型，无需 API Key)</SelectItem>
              <SelectItem value="custom">自定义 / OneAPI / 聚合中转</SelectItem>
            </SelectContent>
          </Select>
        </div>

        <!-- Base URL -->
        <div class="space-y-1.5">
          <label class="text-xs font-semibold flex items-center justify-between text-foreground">
            <span class="flex items-center gap-1.5">
              <Server class="h-3.5 w-3.5 text-muted-foreground" />
              接口基地址 (Base URL)
            </span>
            <span class="text-[10px] text-muted-foreground">需包含 /v1 路径</span>
          </label>
          <Input v-model="form.base_url" placeholder="例如 https://api.deepseek.com/v1" class="h-8 text-xs font-mono" />
        </div>

        <!-- API Key -->
        <div class="space-y-1.5">
          <label class="text-xs font-semibold flex items-center justify-between text-foreground">
            <span class="flex items-center gap-1.5">
              <Key class="h-3.5 w-3.5 text-muted-foreground" />
              API Key (秘钥)
            </span>
            <span v-if="form.provider === 'ollama'" class="text-[10px] text-emerald-600 dark:text-emerald-400">本地 Ollama 可留空</span>
          </label>
          <Input v-model="form.api_key" type="password" placeholder="sk-..." class="h-8 text-xs font-mono" />
        </div>

        <!-- 模型名称 -->
        <div class="space-y-1.5">
          <label class="text-xs font-semibold flex items-center justify-between text-foreground">
            <span class="flex items-center gap-1.5">
              <Zap class="h-3.5 w-3.5 text-muted-foreground" />
              模型标识 (Model)
            </span>
            <span class="text-[10px] text-muted-foreground">如 deepseek-chat、gpt-4o、qwen2.5:7b</span>
          </label>
          <Input v-model="form.model" placeholder="输入要调用的模型名" class="h-8 text-xs font-mono" />
        </div>

        <!-- 测试连通性反馈 -->
        <div v-if="testResult" class="p-2.5 rounded text-xs flex items-start gap-2" :class="testResult.success ? 'bg-emerald-50 text-emerald-900 border border-emerald-200 dark:bg-emerald-950/40 dark:text-emerald-300' : 'bg-destructive/10 text-destructive border border-destructive/20'">
          <CheckCircle2 v-if="testResult.success" class="h-4 w-4 shrink-0 text-emerald-600 mt-0.5" />
          <AlertCircle v-else class="h-4 w-4 shrink-0 text-destructive mt-0.5" />
          <div class="flex-1 leading-relaxed break-all">{{ testResult.msg }}</div>
        </div>
      </div>

      <DialogFooter class="flex sm:justify-between items-center w-full">
        <Button size="sm" variant="outline" class="h-8 text-xs gap-1.5" :disabled="isTesting || !form.base_url || !form.model" @click="handleTest">
          <Loader2 v-if="isTesting" class="h-3.5 w-3.5 animate-spin" />
          <Zap v-else class="h-3.5 w-3.5 text-amber-500" />
          {{ isTesting ? '正在测试...' : '测试连通性' }}
        </Button>

        <div class="flex items-center gap-2">
          <Button variant="ghost" size="sm" class="h-8 text-xs" @click="emit('update:open', false)">取消</Button>
          <Button size="sm" class="h-8 text-xs gap-1.5" :disabled="isSaving" @click="handleSave">
            <Loader2 v-if="isSaving" class="h-3.5 w-3.5 animate-spin" />
            <Save v-else class="h-3.5 w-3.5" />
            保存配置
          </Button>
        </div>
      </DialogFooter>
    </DialogContent>
  </Dialog>
</template>
