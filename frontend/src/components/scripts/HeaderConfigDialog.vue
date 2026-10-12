<script setup lang="ts">
import { ref } from 'vue'
import { Check, Copy } from 'lucide-vue-next'
import { Button } from '@/components/ui/button'
import { Dialog, DialogContent, DialogDescription, DialogHeader, DialogTitle } from '@/components/ui/dialog'
import { Tabs, TabsContent, TabsList, TabsTrigger } from '@/components/ui/tabs'

const open = defineModel<boolean>('open', { required: true })
const copied = ref(false), copyError = ref('')
const parameters = [
  { name: 'version', default: '按修改时间生成', description: '文档版本，如 1.0.0。修改步骤时建议同步更新版本号。' },
  { name: 'schedule', default: '不定时执行', description: 'Cron 定时表达式。5 段为“分 时 日 月 星期”，6 段在最前面增加“秒”；如 0 9 * * * 表示每天 09:00。' },
  { name: 'timeout_sec', default: '120 秒', description: '执行超时时间，填写正整数，单位为秒。超时后终止本次任务。' },
  { name: 'enabled', default: 'true', description: '定时执行开关，填写 true 或 false；不影响手动运行。需要同时配置 schedule 和目标设备。' },
  { name: 'recording.enabled', default: 'false', description: '任务开始时自动录制，结束时停止。录制失败仍继续执行任务。' },
  { name: 'recording.record_audio', default: 'false', description: '同时录制设备声音，需开启 recording.enabled 且设备支持。' },
  { name: 'report.enabled', default: 'false', description: '每台设备执行结束后生成 AI 报告，使用系统已配置的模型。' },
  { name: 'report.prompt', default: '自动总结执行结果', description: '报告总结要求，最多 4000 字。可用 | 换行填写；需开启 report.enabled。' },
]
const example = `---
version: 1.0.0
schedule: "0 9 * * *"
timeout_sec: 120
enabled: false
recording:
  enabled: false
  record_audio: false
report:
  enabled: false
  prompt: |
    总结执行结果和失败原因。
    引用日志证据，列出需要人工确认的事项。
---

# 步骤流程
- 等待 2 秒
`
async function copyExample() {
  copyError.value = ''
  try { await navigator.clipboard.writeText(example); copied.value = true }
  catch { copyError.value = '复制失败，请手动选择示例。' }
}
</script>

<template>
  <Dialog v-model:open="open">
    <DialogContent class="flex max-h-[90dvh] flex-col overflow-hidden sm:max-w-3xl">
      <DialogHeader class="shrink-0 pr-6">
        <DialogTitle>Markdown 头部配置</DialogTitle>
        <DialogDescription>放在文档最开头，用两行 <code>---</code> 包住；嵌套参数缩进两个空格。</DialogDescription>
      </DialogHeader>
      <Tabs default-value="parameters" class="flex min-h-0 flex-col">
        <TabsList class="shrink-0 self-start"><TabsTrigger value="parameters">参数说明</TabsTrigger><TabsTrigger value="example">完整示例</TabsTrigger></TabsList>
        <TabsContent value="parameters" class="min-h-0 overflow-auto">
          <p class="mb-2 text-xs text-muted-foreground">默认值指省略该参数时的行为。</p>
          <dl class="divide-y">
            <div v-for="parameter in parameters" :key="parameter.name" class="py-3">
              <dt class="flex flex-wrap items-center justify-between gap-2 text-xs"><code class="font-medium">{{ parameter.name }}</code><span class="text-muted-foreground">默认：{{ parameter.default }}</span></dt>
              <dd class="mt-1.5 text-sm leading-6">{{ parameter.description }}</dd>
            </div>
          </dl>
          <p class="mt-3 text-xs leading-6 text-muted-foreground">任务标题与目标设备在“任务配置”中设置；<code>name</code> 仅作标注，<code>target_device</code> 和 <code>target_devices</code> 不再作为头部参数。</p>
          <p class="mt-2 text-xs leading-6 text-muted-foreground">开关使用不带引号的 <code>true</code> / <code>false</code>，同一个参数不能重复填写。录制和报告只读取头部，不读取正文中的示例。</p>
        </TabsContent>
        <TabsContent value="example" class="min-h-0 overflow-auto">
          <div class="mb-3 flex flex-wrap items-center gap-2"><p class="min-w-0 flex-1 text-xs leading-6 text-muted-foreground">示例默认关闭定时、录制和报告，按需改为 <code>true</code>。</p><Button variant="ghost" size="sm" @click="copyExample"><Check v-if="copied" class="size-3.5" /><Copy v-else class="size-3.5" />{{ copied ? '已复制' : '复制示例' }}</Button></div>
          <p v-if="copyError" role="alert" class="mb-2 text-xs text-destructive">{{ copyError }}</p>
          <pre class="rounded-lg bg-muted/40 p-4 font-mono text-xs leading-6 whitespace-pre-wrap break-words"><code>{{ example }}</code></pre>
        </TabsContent>
      </Tabs>
    </DialogContent>
  </Dialog>
</template>
