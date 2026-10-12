<script setup lang="ts">
import { computed, ref } from 'vue'
import { ArrowLeft, ChevronRight, Download, FileText, History, Play, Search, Settings2, Video } from 'lucide-vue-next'
import { Button } from '@/components/ui/button'
import { Badge } from '@/components/ui/badge'
import { Input } from '@/components/ui/input'
import { Checkbox } from '@/components/ui/checkbox'
import { Tabs, TabsContent, TabsList, TabsTrigger } from '@/components/ui/tabs'
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from '@/components/ui/select'
import MarkdownContent from '@/components/scripts/MarkdownContent.vue'
import { batches, documentSource } from './data'

const mode = ref('history')
const view = ref('logs')
const batchId = ref(batches[0]!.id)
const runId = ref(batches[0]!.runs[0]!.id)
const mobileDetail = ref(false)
const keyword = ref('')
const stream = ref('all')
const follow = ref(true)
const highlight = ref(0)
const source = ref(documentSource)
const sourceView = ref('source')
const reportRetried = ref(false)
const position = ref('0')
const currentBatch = computed(() => batches.find(batch => batch.id === batchId.value)!)
const run = computed(() => currentBatch.value.runs.find(item => item.id === runId.value)!)
const filteredLogs = computed(() => run.value.logs.filter(line =>
  (stream.value === 'all' || stream.value === line.stream) && line.message.includes(keyword.value.trim()),
))
const reportMarkdown = computed(() => run.value.status === '已跳过'
  ? '# 执行报告\n\n设备离线，任务未启动。没有设备操作日志，无法确认签到结果。'
  : run.value.status === '失败'
    ? '# 执行报告\n\n## 结果\n任务在等待签到按钮时超时并被终止。\n\n## 异常\n日志记录等待超时，无法确认具体界面原因。录制因已有录制而未启动。\n\n## 建议\n检查应用状态和按钮模板，再单独重试该设备。'
    : '# 执行报告\n\n## 结果\n本次进程正常结束，退出码为 0。日志记录签到成功，尚无额外的页面校验证据。\n\n## 关键过程\n- 唤醒设备并打开应用。\n- 首次等待签到按钮时重试 1 次，随后找到按钮并点击。\n- 09:00:09 日志记录签到成功。\n\n## 建议\n关注后续执行的按钮等待时间；如需确认业务结果，增加签到结果校验。')
function selectBatch(id: string) {
  batchId.value = id
  runId.value = currentBatch.value.runs[0]!.id
  resetFilters()
  mobileDetail.value = true
}
function resetFilters() {
  keyword.value = ''; stream.value = 'all'; highlight.value = 0; reportRetried.value = false; position.value = '0'
}
function selectDevice(value: unknown) {
  if (typeof value === 'string') { runId.value = value; resetFilters() }
}
function jumpToLog(seq: number) {
  view.value = 'logs'; keyword.value = ''; stream.value = 'all'; highlight.value = seq
}
function download(text: string, filename: string) {
  const url = URL.createObjectURL(new Blob([text], { type: 'text/plain;charset=utf-8' }))
  const anchor = document.createElement('a'); anchor.href = url; anchor.download = filename; anchor.click()
  URL.revokeObjectURL(url)
}
</script>

<template>
  <section class="space-y-4">
    <div class="flex flex-wrap items-center gap-3">
      <div class="min-w-0 flex-1"><h1 class="text-xl font-semibold">每日签到</h1><p class="mt-1 text-xs text-muted-foreground">Pixel 8 · Redmi K70</p></div>
      <Button :variant="mode === 'document' ? 'secondary' : 'outline'" size="sm" @click="mode = 'document'"><FileText class="size-4" />文档</Button>
      <Button :variant="mode === 'history' ? 'secondary' : 'outline'" size="sm" @click="mode = 'history'"><History class="size-4" />执行记录</Button>
    </div>

    <div v-if="mode === 'document'" class="grid overflow-hidden rounded-xl border lg:grid-cols-[1fr_260px]">
      <Tabs v-model="sourceView" class="gap-0">
        <TabsContent value="source" class="m-0"><textarea v-model="source" aria-label="Markdown 源码" spellcheck="false" class="h-[530px] w-full resize-none bg-background p-5 font-mono text-xs leading-6 outline-none focus-visible:ring-2 focus-visible:ring-ring" /></TabsContent>
        <TabsContent value="preview" class="m-0 h-[530px] overflow-auto"><MarkdownContent :content="source" document class="p-6" /></TabsContent>
        <TabsList class="h-11 w-full justify-start rounded-none border-t bg-muted/30 px-3"><TabsTrigger value="source">源码</TabsTrigger><TabsTrigger value="preview">预览</TabsTrigger></TabsList>
      </Tabs>
      <aside class="space-y-5 border-t bg-muted/15 p-5 lg:border-t-0 lg:border-l">
        <div class="flex items-center gap-2 text-sm font-medium"><Settings2 class="size-4" />任务配置</div>
        <dl class="space-y-4 text-xs"><div><dt class="text-muted-foreground">执行时间</dt><dd class="mt-1">每天 09:00</dd></div><div><dt class="text-muted-foreground">超时</dt><dd class="mt-1">120 秒</dd></div></dl>
        <p class="text-xs text-muted-foreground">自动录制与执行报告在文档头部配置。</p>
      </aside>
    </div>

    <div v-else class="grid min-h-[580px] overflow-hidden rounded-xl border md:grid-cols-[228px_1fr]">
      <aside :class="mobileDetail ? 'hidden md:block' : ''" class="border-r bg-muted/15">
        <h2 class="border-b px-4 py-3 text-sm font-medium">执行记录</h2>
        <div class="space-y-1 p-2">
          <Button v-for="batch in batches" :key="batch.id" variant="ghost" class="h-auto w-full justify-start px-3 py-3 text-left" :class="batch.id === batchId ? 'bg-muted' : ''" @click="selectBatch(batch.id)">
            <span class="flex min-w-0 flex-1 flex-col gap-2"><span class="font-medium">{{ batch.time }}</span><span class="text-xs text-muted-foreground">{{ batch.trigger }} · {{ batch.status }}</span></span><ChevronRight class="size-3.5 shrink-0 text-muted-foreground" />
          </Button>
        </div>
      </aside>

      <div :class="mobileDetail ? '' : 'hidden md:flex'" class="flex min-w-0 flex-col">
        <div class="flex flex-wrap items-center gap-3 border-b px-4 py-3">
          <Button variant="ghost" size="icon-sm" aria-label="返回执行记录" class="md:hidden" @click="mobileDetail = false"><ArrowLeft class="size-4" /></Button>
          <Select :model-value="runId" @update:model-value="selectDevice"><SelectTrigger aria-label="执行设备" class="w-full max-w-48"><SelectValue /></SelectTrigger><SelectContent><SelectItem v-for="item in currentBatch.runs" :key="item.id" :value="item.id">{{ item.device }}</SelectItem></SelectContent></Select>
          <Badge variant="outline">{{ run.status }}</Badge><span class="ml-auto text-xs text-muted-foreground">{{ run.duration }}</span>
          <p class="w-full text-xs text-muted-foreground">{{ currentBatch.time }} · {{ currentBatch.trigger }} · {{ run.serial }}</p>
        </div>
        <Tabs v-model="view" class="flex flex-1 flex-col gap-0">
          <TabsList class="h-12 w-full justify-start gap-2 rounded-none border-b bg-transparent px-4" aria-label="执行详情"><TabsTrigger value="logs">日志</TabsTrigger><TabsTrigger value="recording">录制</TabsTrigger><TabsTrigger value="report">报告</TabsTrigger></TabsList>

          <TabsContent value="logs" class="m-0 flex flex-1 flex-col">
            <div class="flex flex-wrap items-center gap-2 border-b px-4 py-3">
              <div class="relative min-w-32 flex-1"><Search class="absolute top-2.5 left-2.5 size-3.5 text-muted-foreground" /><Input v-model="keyword" placeholder="搜索日志" aria-label="搜索日志" class="h-8 pl-8 text-xs" /></div>
              <Select v-model="stream"><SelectTrigger aria-label="日志来源" class="h-8 w-28 text-xs"><SelectValue /></SelectTrigger><SelectContent><SelectItem value="all">全部来源</SelectItem><SelectItem value="stdout">标准输出</SelectItem><SelectItem value="stderr">错误输出</SelectItem><SelectItem value="system">任务事件</SelectItem></SelectContent></Select>
              <Button size="icon-sm" variant="ghost" aria-label="下载日志" @click="download(run.logs.map(line => `${line.seq} ${line.time} ${line.stream} ${line.message}`).join('\n'), `${run.id}.log`)"><Download class="size-4" /></Button>
            </div>
            <div class="min-h-64 flex-1 overflow-auto px-4 py-3 font-mono text-xs leading-6">
              <div v-for="line in filteredLogs" :key="line.seq" class="flex gap-3 rounded px-1 py-1" :class="highlight === line.seq ? 'bg-accent ring-1 ring-ring' : ''"><span class="w-4 shrink-0 text-right text-muted-foreground">{{ line.seq }}</span><span class="hidden shrink-0 text-muted-foreground sm:inline">{{ line.time }}</span><span class="w-12 shrink-0 text-[10px] text-muted-foreground">{{ line.stream }}</span><span class="min-w-0 break-words" :class="line.stream === 'stderr' ? 'text-destructive' : ''">{{ line.message }}</span></div>
              <p v-if="!filteredLogs.length" role="status" class="py-12 text-center font-sans text-muted-foreground">没有匹配的日志</p>
            </div>
            <div class="flex items-center justify-between border-t px-4 py-3 text-xs text-muted-foreground"><label class="flex items-center gap-2"><Checkbox v-model="follow" aria-label="跟随最新日志" />跟随最新</label><span>{{ filteredLogs.length }} 行</span></div>
          </TabsContent>

          <TabsContent value="recording" class="m-0 flex-1 p-5">
            <template v-if="run.recording === 'completed'">
              <div class="flex items-center justify-between"><h3 class="text-sm font-medium">任务录制</h3><span class="text-xs text-muted-foreground">12 秒 · 无声音</span></div>
              <div class="mt-4 flex h-72 items-center justify-center rounded-lg bg-muted">
                <div class="flex h-60 w-28 flex-col items-center justify-center rounded-xl border-4 border-foreground/25 bg-background px-2 text-center shadow-sm"><span class="mb-6 text-[10px] text-muted-foreground">每日签到</span><span class="flex size-10 items-center justify-center rounded-full bg-secondary text-xl">✓</span><span class="mt-3 text-xs">签到成功</span><span class="mt-1 text-[10px] text-muted-foreground">连续签到 7 天</span></div>
              </div>
              <label class="mt-4 block text-xs text-muted-foreground" for="recording-position">{{ position }} / 12 秒</label><Input id="recording-position" v-model="position" aria-label="播放位置" type="range" min="0" max="12" class="mt-2 h-8" />
              <div class="mt-3 flex gap-2"><Button size="sm" variant="outline" @click="position = position === '12' ? '0' : '12'"><Play class="size-3.5" />查看片段</Button></div>
            </template>
            <div v-else class="flex min-h-80 flex-col items-center justify-center gap-3 text-center"><Video class="size-8 text-muted-foreground/50" /><p class="text-sm">{{ run.recording === 'disabled' ? '本次未开启录制' : run.recording === 'expired' ? '录制已过期' : '录制失败' }}</p><p v-if="run.recording === 'failed'" class="text-xs text-muted-foreground">设备已有录制，本次未开始任务录制</p></div>
          </TabsContent>

          <TabsContent value="report" class="m-0 flex-1">
            <template v-if="run.report === 'completed' || reportRetried">
              <div class="flex items-center justify-between border-b px-5 py-3"><span class="text-xs text-muted-foreground">AI 总结 · {{ reportRetried ? '刚刚生成' : '09:02 生成' }}</span><Button size="sm" variant="ghost" @click="download(reportMarkdown, `${run.id}-report.md`)"><Download class="size-3.5" />下载</Button></div>
              <MarkdownContent :content="reportMarkdown" class="p-5" />
              <div v-if="run.status !== '已跳过'" class="space-y-2 px-5 pb-5"><p class="text-xs text-muted-foreground">相关日志</p><div class="flex flex-wrap gap-2"><Button v-for="seq in run.status === '失败' ? [3, 4] : [4, 6, 7]" :key="seq" size="sm" variant="outline" @click="jumpToLog(seq)">第 {{ seq }} 行</Button></div></div>
            </template>
            <div v-else class="flex min-h-80 flex-col items-center justify-center gap-3 text-center"><FileText class="size-8 text-muted-foreground/50" /><p class="text-sm">{{ run.report === 'disabled' ? '本次未开启报告' : run.report === 'pending' ? '报告生成中' : '报告生成失败' }}</p><template v-if="run.report === 'failed'"><p class="text-xs text-muted-foreground">模型请求超时</p><Button size="sm" variant="outline" @click="reportRetried = true">重新生成</Button></template></div>
          </TabsContent>
        </Tabs>
      </div>
    </div>
  </section>
</template>
