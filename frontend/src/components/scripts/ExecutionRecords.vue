<script setup lang="ts">
import { computed, nextTick, reactive, ref, toRef, watch } from 'vue'
import { ArrowLeft, Download, RefreshCw, Square } from 'lucide-vue-next'
import { Button } from '@/components/ui/button'
import { Badge } from '@/components/ui/badge'
import { Input } from '@/components/ui/input'
import { Tabs, TabsContent, TabsList, TabsTrigger } from '@/components/ui/tabs'
import { downloadArtifact, recordingMedia } from '@/api/taskRuns'
import { killTask } from '@/api/mdScripts'
import { dateTime, taskStatus } from '@/scripts/document'
import { useExecutionRecords } from '@/scripts/useExecutionRecords'
import TaskReport from './TaskReport.vue'
const props = defineProps<{ scriptId?: number, active: boolean, initialTask: string }>()
const r = reactive(useExecutionRecords(toRef(props, 'scriptId'), toRef(props, 'active'), toRef(props, 'initialTask')))
const logContainer = ref<HTMLElement>()
const actionError = ref(''), actionBusy = ref(false), videoError = ref('')
const log = computed(() => r.detail?.artifacts.log)
const recording = computed(() => r.detail?.artifacts.recording)
const reportState = computed(() => r.report?.generation ?? r.detail?.artifacts.report)
const recordingText: Record<string, string> = { unavailable: '该历史执行未保存录制', disabled: '本次执行未开启录制', not_started: '设备未启动，未开始录制', preparing: '准备录制…', recording: '录制中', stopping: '录制正在收尾…', failed: '录制失败', expired: '录制已过期' }
const reportText: Record<string, string> = { unavailable: '该历史执行未保存报告', disabled: '本次执行未开启报告', waiting: '执行结束后生成报告', pending: '报告等待生成', running: '报告生成中…', failed: '报告生成失败' }
async function download(kind: 'logs' | 'recording' | 'report') {
  if (actionBusy.value || !r.selectedTask) return
  const id = r.selectedTask; actionBusy.value = true; actionError.value = ''
  try { await downloadArtifact(id, kind, kind === 'report' ? r.report?.revision : undefined) }
  catch (error) { if (r.selectedTask === id) actionError.value = error instanceof Error ? error.message : '下载失败' }
  finally { actionBusy.value = false }
}
async function stop() {
  if (actionBusy.value) return
  const id = r.selectedTask; actionBusy.value = true; actionError.value = ''
  try { await killTask(id); if (r.selectedTask === id) await r.refreshTask() }
  catch (error) { if (r.selectedTask === id) actionError.value = error instanceof Error ? error.message : '终止失败' }
  finally { actionBusy.value = false }
}
watch(() => r.selectedTask, () => { actionError.value = ''; videoError.value = '' })
watch(() => r.lines, async () => { await nextTick(); if (r.highlighted) document.getElementById(`task-log-${r.selectedTask}-${r.highlighted}`)?.scrollIntoView({ block: 'center' }); else if (r.follow && logContainer.value) logContainer.value.scrollTop = logContainer.value.scrollHeight }, { deep: true })
</script>
<template>
  <section aria-label="执行记录" class="flex min-h-0 min-w-0 flex-1 flex-col">
    <div class="grid min-h-0 flex-1 md:grid-cols-[210px_minmax(0,1fr)]">
      <aside aria-label="执行批次" class="flex min-h-0 flex-col border-r" :class="r.mobileDetail ? 'hidden md:flex' : ''">
        <p v-if="r.listLoading && !r.dispatches.length" role="status" class="p-4 text-sm text-muted-foreground">加载中…</p>
        <div v-else-if="r.listError" role="alert" class="p-4 text-sm"><p class="text-destructive">{{ r.listError }}</p><Button variant="ghost" size="sm" @click="r.loadList()">重试</Button></div>
        <p v-else-if="!r.dispatches.length" class="p-4 text-sm text-muted-foreground">尚无执行记录</p>
        <div class="min-h-0 flex-1 overflow-auto p-2">
          <Button v-for="batch in r.dispatches" :key="batch.id" variant="ghost" class="mb-1 h-auto w-full flex-col items-start gap-1 p-3 text-left" :class="batch.id === r.selectedDispatch ? 'bg-muted' : ''" :aria-pressed="batch.id === r.selectedDispatch" @click="r.selectDispatch(batch.id); r.mobileDetail = true">
            <span class="text-xs">{{ dateTime(batch.started_at) }}</span><span class="flex w-full items-center justify-between gap-1 text-[11px] text-muted-foreground"><span>{{ batch.trigger === 'manual' ? '手动执行' : batch.trigger === 'schedule' ? '定时执行' : '历史执行' }}</span><span>{{ taskStatus[batch.status] ?? batch.status }}</span></span>
          </Button>
          <Button v-if="r.hasMore" variant="ghost" size="sm" class="w-full" :disabled="r.listLoading" @click="r.loadList(true)">更早执行</Button>
        </div>
      </aside>
      <main class="flex min-h-0 min-w-0 flex-col" :class="!r.mobileDetail ? 'hidden md:flex' : ''">
        <div class="flex items-center gap-2 border-b p-2 md:hidden"><Button variant="ghost" size="sm" @click="r.mobileDetail = false"><ArrowLeft class="size-4" />执行列表</Button></div>
        <p v-if="r.detailError" role="alert" class="p-4 text-sm text-destructive">{{ r.detailError }}<Button variant="ghost" size="sm" @click="r.refreshTask()">重试</Button></p>
        <p v-else-if="r.detailLoading && !r.detail" role="status" class="p-4 text-sm text-muted-foreground">加载中…</p>
        <template v-if="r.dispatch">
          <div class="space-y-2 border-b px-4 py-3">
            <div class="flex flex-wrap items-center gap-2 text-xs"><span class="min-w-0 flex-1 break-all font-medium">{{ r.dispatch.snapshot.availability === 'available' ? r.dispatch.snapshot.script_name : '历史执行' }}</span><span class="text-muted-foreground">{{ dateTime(r.dispatch.started_at) }}</span></div>
            <div class="flex flex-wrap items-center gap-2"><label v-if="r.tasks.length" class="flex min-w-0 flex-1 items-center gap-2 text-xs">设备<select v-model="r.selectedTask" aria-label="执行设备" class="min-w-0 max-w-full rounded-md border bg-background px-2 py-1.5 text-xs" @change="r.selectTask(r.selectedTask)"><option v-for="task in r.tasks" :key="task.id" :value="task.id">{{ task.device_serial }}</option></select></label><Badge v-if="r.detail" variant="outline" class="text-[11px]">{{ taskStatus[r.detail.task.status] }}</Badge><Button v-if="r.running" size="sm" variant="ghost" class="text-xs" :disabled="actionBusy" @click="stop"><Square class="size-3.5" />终止</Button></div>
            <p v-if="r.detail?.task.error_msg || r.dispatch.error_msg" class="text-xs text-destructive">{{ r.detail?.task.error_msg || r.dispatch.error_msg }}</p>
          </div>
          <Tabs v-if="r.detail" v-model="r.tab" class="flex min-h-0 flex-1 flex-col gap-0">
            <TabsList class="m-3 shrink-0 self-start"><TabsTrigger value="logs">日志</TabsTrigger><TabsTrigger value="recording">录制</TabsTrigger><TabsTrigger value="report">报告</TabsTrigger></TabsList>
            <TabsContent value="logs" class="m-0 flex min-h-0 flex-1 flex-col data-[state=inactive]:hidden">
              <div v-if="log?.availability === 'available'" class="flex flex-wrap items-center gap-2 border-b px-4 pb-3"><select v-model="r.stream" aria-label="日志来源" class="rounded-md border bg-background p-1.5 text-xs"><option value="all">全部来源</option><option value="stdout">标准输出</option><option value="stderr">错误输出</option><option value="system">系统</option></select><Input v-model="r.search" aria-label="搜索日志" placeholder="搜索日志" maxlength="200" class="h-8 min-w-24 flex-1 text-xs" /><label class="flex items-center gap-1 text-xs"><input v-model="r.follow" type="checkbox">跟随</label><Button size="icon-sm" variant="ghost" aria-label="下载日志" :disabled="actionBusy" @click="download('logs')"><Download class="size-4" /></Button></div>
              <p v-if="log?.availability !== 'available'" class="px-4 py-6 text-sm text-muted-foreground">{{ log?.availability === 'expired' ? '原始日志已过期' : '该历史执行未采集日志' }}</p>
              <template v-else>
                <p v-if="log.truncated || log.capture_state === 'degraded'" role="status" class="px-4 py-2 text-xs text-amber-700 dark:text-amber-400">{{ log.error_msg || '日志达到保存上限，内容不完整' }}</p>
                <p v-if="r.logError" role="alert" class="px-4 py-2 text-xs text-destructive">{{ r.logError }}<Button size="sm" variant="ghost" @click="r.loadLogs()">重试</Button></p>
                <div ref="logContainer" aria-label="日志内容" class="min-h-0 flex-1 overflow-auto px-3 pb-4 font-mono text-[11px] leading-6">
                  <Button v-if="r.older" variant="ghost" size="sm" class="mb-2 w-full text-xs" :disabled="r.logLoading" @click="r.follow = false; r.loadLogs('older')">更早日志</Button>
                  <p v-if="r.logLoading && !r.lines.length" role="status" class="p-3 text-muted-foreground">加载中…</p><p v-else-if="!r.lines.length && !r.logError" class="p-3 text-muted-foreground">{{ r.search || r.stream !== 'all' ? '当前范围无匹配日志' : '等待日志输出' }}</p>
                  <div v-for="line in r.lines" :id="`task-log-${r.selectedTask}-${line.seq}`" :key="`${line.task_id}-${line.seq}`" class="grid grid-cols-[30px_68px_minmax(0,1fr)] gap-x-2 rounded px-1" :class="line.seq === r.highlighted ? 'bg-primary/10 ring-1 ring-primary/30' : ''"><span class="text-right text-muted-foreground">{{ line.seq }}</span><time class="text-muted-foreground">{{ new Date(line.at_ms).toLocaleTimeString('zh-CN', { hour12: false }) }}</time><span class="whitespace-pre-wrap break-all" :class="line.stream === 'stderr' ? 'text-destructive' : line.stream === 'system' ? 'text-muted-foreground' : ''">{{ line.message }}<span v-if="line.continued" class="text-muted-foreground"> ↪</span></span></div>
                </div>
              </template>
            </TabsContent>
            <TabsContent value="recording" class="m-0 min-h-0 flex-1 overflow-auto p-4">
              <template v-if="recording?.state === 'completed'"><video :key="r.selectedTask" :src="recordingMedia(r.selectedTask)" controls preload="metadata" class="mx-auto max-h-[55dvh] w-full max-w-2xl rounded-lg bg-black" @error="videoError = '视频读取失败，请重试'" /><p v-if="videoError" role="alert" class="mt-2 text-sm text-destructive">{{ videoError }}</p><div class="mt-3 flex items-center gap-2"><span v-if="recording.coverage_complete === false" class="text-xs text-muted-foreground">录制未覆盖完整执行</span><Button variant="outline" size="sm" class="ml-auto" :disabled="actionBusy" @click="download('recording')"><Download class="size-4" />下载录制</Button></div></template>
              <p v-else class="py-6 text-sm text-muted-foreground">{{ recordingText[recording?.state ?? 'unavailable'] }}</p><p v-if="recording?.error_msg" class="text-xs text-destructive">{{ recording.error_msg }}</p>
            </TabsContent>
            <TabsContent value="report" class="m-0 min-h-0 flex-1 overflow-auto p-4">
              <div class="mb-3 flex flex-wrap justify-end gap-2"><Button v-if="r.report?.report" variant="ghost" size="sm" :disabled="actionBusy" @click="download('report')"><Download class="size-4" />下载报告</Button><Button v-if="reportState?.can_retry" variant="outline" size="sm" :disabled="r.retrying" @click="r.retryReport"><RefreshCw class="size-3.5" />重新生成</Button></div>
              <p v-if="reportState?.state !== 'completed'" role="status" class="mb-3 text-sm text-muted-foreground">{{ reportText[reportState?.state ?? 'unavailable'] }}</p><p v-if="reportState?.error_msg || r.reportError" role="alert" class="mb-3 text-xs text-destructive">{{ r.reportError || reportState?.error_msg }}</p>
              <TaskReport v-if="r.report?.report" :content="r.report.report.markdown" />
              <div v-if="r.report?.report?.citations.length" class="mt-4 flex flex-wrap items-center gap-2 border-t pt-3"><span class="text-xs text-muted-foreground">日志依据</span><Button v-for="citation in r.report.report.citations" :key="citation.seq" variant="ghost" size="sm" class="h-7 px-2 text-xs" :disabled="r.report.log_availability !== 'available'" @click="r.jumpToLine(citation.seq)">第 {{ citation.seq }} 行</Button><span v-if="r.report.log_availability === 'expired'" class="text-xs text-muted-foreground">原始日志已过期</span></div>
            </TabsContent>
          </Tabs>
          <p v-else-if="!r.detailLoading && !r.tasks.length" class="p-4 text-sm text-muted-foreground">本次执行未启动设备</p>
        </template>
        <p v-else-if="!r.detailLoading && r.dispatches.length" class="p-4 text-sm text-muted-foreground">选择一次执行</p>
        <p v-if="actionError" role="alert" class="border-t p-3 text-xs text-destructive">{{ actionError }}</p>
      </main>
    </div>
  </section>
</template>
