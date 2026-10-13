<script setup lang="ts">
import { computed, onUnmounted, reactive, ref } from 'vue'
import { Code2, Play, Square } from 'lucide-vue-next'
import { Button } from '@/components/ui/button'
import { Tabs, TabsContent, TabsList, TabsTrigger } from '@/components/ui/tabs'
import AssistantActivities from '@/components/scripts/AssistantActivities.vue'
import type { AssistantActivity } from '@/scripts/useScriptAssistant'

const document = ref('---\nversion: 1.0.0\n---\n\n# 设备查询\n- 使用 ADB 查询设备型号并打印\n')
const tab = ref('source')
const mode = ref('success')
const busy = ref(false)
const status = ref('')
const source = ref('')
const generatedDocument = ref('')
const activities = reactive<AssistantActivity[]>([])
const valid = computed(() => !!source.value && generatedDocument.value === document.value)
let timer: ReturnType<typeof setTimeout> | undefined
function stop() {
  clearTimeout(timer)
  busy.value = false
  status.value = '已停止生成'
  for (const activity of activities) {
    if (activity.kind === 'thinking') activity.running = false
    else if (activity.status === 'running') activity.status = 'interrupted'
  }
}
function generate() {
  const snapshot = document.value
  busy.value = true
  status.value = '正在生成 Python…'
  activities.splice(0, activities.length,
    { kind: 'thinking', content: '先确认运行接口和当前文档中的操作参数。', running: true },
    { kind: 'tool', id: 'read', name: 'read', args: '{"path":"skill/references/sdk-api.md"}', output: '', status: 'running' })
  timer = setTimeout(() => {
    for (const activity of activities) {
      if (activity.kind === 'thinking') activity.running = false
      else { activity.status = 'completed'; activity.output = '设备命令、坐标约定与异常行为已读取。' }
    }
    busy.value = false
    if (snapshot !== document.value) { status.value = '生成期间任务已修改，请重新生成'; return }
    if (mode.value === 'missing') { status.value = '请补充文档：步骤 2：请提供点击坐标'; return }
    if (mode.value === 'failure') { status.value = '模型连接失败，请重试'; return }
    source.value = "import argparse\nimport os\nfrom mywebscrcpy import Client\n\n\ndef main():\n    parser = argparse.ArgumentParser(description=\"查询设备型号\")\n    parser.add_argument(\"serial\", help=\"ADB 设备序列号\")\n    parser.add_argument(\"--base-url\", default=os.environ.get(\"MYWEBSCRCPY_API_URL\", \"\"))\n    parser.add_argument(\"--token\", default=os.environ.get(\"MYWEBSCRCPY_API_TOKEN\", \"\"))\n    options = parser.parse_args()\n    if not options.base_url:\n        raise SystemExit(\"请通过 --base-url 指定 MyWebScrcpy 服务地址\")\n    headers = {}\n    if options.token:\n        headers[\"Authorization\"] = \"Bearer \" + options.token\n\n    with Client(options.base_url, headers=headers) as client:\n        with client.device(options.serial) as device:\n            print(\"步骤 1：读取设备型号\", flush=True)\n            result = device.adb([\"shell\", \"getprop\", \"ro.product.model\"], timeout=10)\n            print(result.stdout, end=\"\", flush=True)\n\n\nif __name__ == \"__main__\":\n    main()\n"
    generatedDocument.value = snapshot
    tab.value = 'python'
    status.value = 'Python 已生成'
  }, 1800)
}
onUnmounted(() => clearTimeout(timer))
</script>
<template>
  <div class="mx-auto flex h-[calc(100vh-5rem)] max-w-6xl flex-col overflow-hidden rounded-xl border bg-background text-foreground">
    <header class="flex flex-wrap items-center gap-2 border-b p-4">
      <h1 class="mr-auto text-sm font-semibold">设备查询</h1>
      <select v-model="mode" aria-label="生成结果" class="rounded-md border bg-background p-2 text-xs"><option value="success">成功</option><option value="missing">缺少信息</option><option value="failure">请求失败</option></select>
      <Button :disabled="busy" size="sm" variant="outline" @click="generate"><Code2 class="size-3.5" />{{ busy ? '生成中…' : '生成 Python' }}</Button>
      <Button :disabled="busy || !valid" size="sm" @click="status = '已启动 1/1 台设备'"><Play class="size-3.5" />运行</Button>
    </header>
    <div class="grid min-h-0 flex-1 grid-cols-1 overflow-auto lg:grid-cols-[1fr_360px]">
      <Tabs v-model="tab" class="flex min-h-80 flex-col gap-0">
        <TabsContent value="source" class="m-0 min-h-0 flex-1 overflow-hidden"><textarea v-model="document" aria-label="Markdown 源码" class="h-full min-h-0 w-full resize-none bg-background p-5 font-mono text-xs leading-7 outline-none" /></TabsContent>
        <TabsContent value="python" class="m-0 min-h-0 flex-1 overflow-hidden"><pre class="h-full overflow-auto p-5 text-xs leading-7">{{ source || '尚未生成 Python' }}</pre></TabsContent>
        <TabsList class="justify-start rounded-none border-t bg-muted/20 p-2"><TabsTrigger value="source">源码</TabsTrigger><TabsTrigger value="python">Python</TabsTrigger></TabsList>
      </Tabs>
      <aside aria-label="AI助手" class="space-y-4 overflow-auto border-t bg-muted/10 p-4 lg:border-t-0 lg:border-l">
        <h2 class="text-sm font-semibold">AI助手</h2>
        <div class="flex items-center gap-2"><p role="status" class="flex-1 text-sm">{{ status }}</p><Button v-if="busy" size="sm" variant="outline" @click="stop"><Square class="size-3" />停止生成</Button></div>
        <AssistantActivities :activities="activities" />
      </aside>
    </div>
  </div>
</template>
