<script setup lang="ts">
import { computed, ref } from 'vue'
import {
  AlertTriangle,
  CheckCircle2,
  Download,
  FileText,
  Folder,
  Image as ImageIcon,
  RefreshCw,
  Settings2,
  Sparkles,
  Terminal,
  Video,
} from 'lucide-vue-next'
import { Button } from '@/components/ui/button'
import { Badge } from '@/components/ui/badge'
import { Input } from '@/components/ui/input'
import { Tabs, TabsContent, TabsList, TabsTrigger } from '@/components/ui/tabs'
import {
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from '@/components/ui/select'
import {
  Dialog,
  DialogContent,
  DialogDescription,
  DialogFooter,
  DialogHeader,
  DialogTitle,
} from '@/components/ui/dialog'

interface OutputItem {
  id: string
  category: 'report' | 'log' | 'screenshot' | 'video'
  format: string
  path: string
  required: boolean
  exists: boolean
  size?: string
  files?: string[]
}

interface MockTask {
  id: string
  name: string
  device_serial: string
  source: string
  status: 'pending' | 'running' | 'paused' | 'success' | 'failed' | 'cancelled'
  business_result: string
  outputs_complete: boolean
  goal: string
  success_condition: string
  skill_dir: string
  updated_at: string
  is_stale?: boolean
  outputs: OutputItem[]
  report_content?: string
  logs: string[]
  actions: { timestamp: string; action: string; result: string }[]
}

const initialTasks: MockTask[] = [
  {
    id: 'task-20261017-001',
    name: '快手金币与签到守护',
    device_serial: 'pixel-8-001',
    source: 'codex',
    status: 'success',
    business_result: '金币领取成功，连续打卡维持',
    outputs_complete: true,
    goal: '进入快手极速版福利中心，观看视频领取3轮金币并核对余额变动',
    success_condition: '金币浮窗完成进度且余额增加',
    skill_dir: '/Users/liuzhuo/.codex/skills/kuaishou-short-video',
    updated_at: '2026-10-17 16:25:10',
    outputs: [
      { id: 'report', category: 'report', format: 'markdown', path: 'report.md', required: true, exists: true, size: '2.4 KB' },
      { id: 'log', category: 'log', format: 'text/utf-8', path: 'logs/task.log', required: true, exists: true, size: '18.2 KB' },
      { id: 'shots', category: 'screenshot', format: 'image/png', path: 'screenshots/', required: false, exists: true, files: ['shot-01.png', 'shot-02.png'] },
      { id: 'video', category: 'video', format: 'video/mp4', path: 'videos/recording.mp4', required: false, exists: true, size: '4.8 MB' },
    ],
    report_content: '# 快手极速版执行报告\n\n- **目标**：完成 3 轮短视频金币打卡\n- **业务结果**：成功领取 1800 金币\n- **耗时**：85 秒\n- **异常排查**：第 2 轮遇到弹窗遮挡，通过通用技能 `尝试返回` 恢复。',
    logs: [
      '[16:23:45] [INFO] 检查设备状态: pixel-8-001 连接正常',
      '[16:23:48] [INFO] 启动应用 com.kuaishou.nebula',
      '[16:23:55] [INFO] 观测页面: 发现福利中心入口',
      '[16:24:20] [INFO] 第 1 轮金币已领取 (+600)',
      '[16:24:50] [INFO] 第 2 轮遇到活动悬浮窗，调用通用返回键消除',
      '[16:25:05] [INFO] 第 3 轮完成，写入 state.json',
    ],
    actions: [
      { timestamp: '16:23:48', action: '打开应用 com.kuaishou.nebula', result: '成功' },
      { timestamp: '16:24:00', action: '拟人滑动看视频 (间隔 8s)', result: '成功' },
      { timestamp: '16:24:52', action: '发送 KEYCODE_ESCAPE 消除弹窗', result: '弹窗已消失' },
    ],
  },
  {
    id: 'task-20261017-002',
    name: '钉钉日常打卡',
    device_serial: 'redmi-k70-002',
    source: 'external-cron',
    status: 'success',
    business_result: '打卡成功已记录',
    outputs_complete: false,
    goal: '打开钉钉考勤页面并自动打卡，保存验证录屏',
    success_condition: '打卡成功弹窗并生成录屏归档',
    skill_dir: '/Users/liuzhuo/.codex/skills/dingtalk-attendance',
    updated_at: '2026-10-17 09:05:30',
    outputs: [
      { id: 'report', category: 'report', format: 'markdown', path: 'report.md', required: true, exists: true, size: '1.1 KB' },
      { id: 'log', category: 'log', format: 'text/utf-8', path: 'logs/task.log', required: true, exists: true, size: '8.4 KB' },
      { id: 'video', category: 'video', format: 'video/mp4', path: 'videos/recording.mp4', required: true, exists: false },
    ],
    report_content: '# 钉钉打卡报告\n\n考勤打卡成功，但设备由于录屏编码器冲突导致 `videos/recording.mp4` 缺失。业务结果有效，产出不完整。',
    logs: [
      '[09:04:10] [INFO] 唤醒设备并解锁',
      '[09:04:25] [INFO] 发现打卡按钮，已点击',
      '[09:04:30] [WARN] 视频编码异常，无法写入 videos/recording.mp4',
      '[09:05:30] [INFO] 任务结束',
    ],
    actions: [
      { timestamp: '09:04:12', action: '解锁屏幕', result: '成功' },
      { timestamp: '09:04:25', action: '点击考勤打卡', result: '打卡成功' },
    ],
  },
  {
    id: 'task-20261017-003',
    name: '长视频浏览巡检',
    device_serial: 'pixel-8-001',
    source: 'codex',
    status: 'running',
    business_result: '正在浏览中 (3/10)',
    outputs_complete: false,
    goal: '持续观察短视频播放流，检测黑屏与异常卡死',
    success_condition: '完成 10 轮上滑并上报总状态',
    skill_dir: '/Users/liuzhuo/.codex/skills/my-scrcpy-common',
    updated_at: '2026-10-17 16:32:00',
    outputs: [
      { id: 'report', category: 'report', format: 'markdown', path: 'report.md', required: true, exists: false },
      { id: 'log', category: 'log', format: 'text/utf-8', path: 'logs/task.log', required: true, exists: true, size: '4.2 KB' },
    ],
    logs: [
      '[16:30:10] [INFO] 任务启动',
      '[16:31:00] [INFO] 正在以 12 秒随机停留浏览视频...',
    ],
    actions: [
      { timestamp: '16:30:15', action: '拟人化上滑', result: '视频播放正常' },
      { timestamp: '16:31:30', action: '拟人化上滑', result: '视频播放正常' },
    ],
  },
  {
    id: 'task-20261016-099',
    name: '旧异常归档任务',
    device_serial: 'unknown-device',
    source: 'manual',
    status: 'failed',
    business_result: '中断且未更新',
    outputs_complete: false,
    is_stale: true,
    goal: '旧执行环境任务',
    success_condition: '未知',
    skill_dir: '/opt/legacy-skills',
    updated_at: '2026-10-16 11:20:00',
    outputs: [
      { id: 'log', category: 'log', format: 'text/utf-8', path: 'logs/task.log', required: true, exists: true, size: '0.8 KB' },
    ],
    logs: ['[11:18:00] 异常断电中断'],
    actions: [],
  },
]

// 界面状态
const tasks = ref<MockTask[]>([...initialTasks])
const scanning = ref(false)
const lastScanTime = ref('2026-10-17 16:30:00')
const taskRootDir = ref('~/.mywebscrcppy/tasks/')
const selectedTaskId = ref<string | null>(initialTasks[0]!.id)
const detailTab = ref<'outputs' | 'report' | 'logs' | 'actions'>('outputs')
const deviceFilter = ref('all')
const statusFilter = ref('all')

// 设置弹窗状态
const configOpen = ref(false)
const aiEndpoint = ref('https://api.openai.com/v1/chat/completions')
const aiModel = ref('gpt-4o-mini')
const aiApiKey = ref('sk-proj-**********************')
const aiTimeout = ref(30)
const configSaved = ref(false)

const filteredTasks = computed(() => {
  return tasks.value.filter((t) => {
    if (deviceFilter.value !== 'all' && t.device_serial !== deviceFilter.value) return false
    if (statusFilter.value !== 'all' && t.status !== statusFilter.value) return false
    return true
  })
})

const currentTask = computed(() => {
  return tasks.value.find(t => t.id === selectedTaskId.value) || null
})

const scanStats = computed(() => {
  const total = tasks.value.length
  const incomplete = tasks.value.filter(t => !t.outputs_complete && t.status === 'success').length
  const running = tasks.value.filter(t => t.status === 'running').length
  return { total, incomplete, running }
})

// 主动扫描刷新
function triggerScan() {
  if (scanning.value) return
  scanning.value = true
  setTimeout(() => {
    lastScanTime.value = new Date().toLocaleTimeString('zh-CN', { hour12: false })
    scanning.value = false
  }, 600)
}

// 局部详情刷新（不扫描根目录）
const refreshingDetail = ref(false)
function refreshCurrentTask() {
  if (!currentTask.value || refreshingDetail.value) return
  refreshingDetail.value = true
  setTimeout(() => {
    refreshingDetail.value = false
  }, 400)
}

function saveAiConfig() {
  configSaved.value = true
  setTimeout(() => {
    configSaved.value = false
    configOpen.value = false
  }, 500)
}
</script>

<template>
  <div class="flex flex-col gap-5 p-4 sm:p-6 max-w-7xl mx-auto">
    <!-- 顶栏与控制 -->
    <header class="flex flex-wrap items-center justify-between gap-4 border-b pb-4">
      <div class="flex items-center gap-3">
        <div class="flex size-10 items-center justify-center rounded-lg bg-primary/10 text-primary">
          <Folder class="size-5" />
        </div>
        <div>
          <h1 class="text-xl font-bold tracking-tight">任务运行中心</h1>
          <p class="text-xs text-muted-foreground">
            工作目录：<span class="font-mono text-foreground">{{ taskRootDir }}</span> ·
            上次扫描：{{ lastScanTime }} · 共 {{ scanStats.total }} 个任务
          </p>
        </div>
      </div>

      <div class="flex items-center gap-2">
        <Button
          variant="default"
          size="sm"
          :disabled="scanning"
          @click="triggerScan"
        >
          <RefreshCw class="size-3.5 mr-1.5" :class="{ 'animate-spin': scanning }" />
          {{ scanning ? '扫描中...' : '扫描刷新' }}
        </Button>
        <Button variant="outline" size="sm" @click="configOpen = true">
          <Settings2 class="size-3.5 mr-1.5" />
          系统与 AI 设置
        </Button>
      </div>
    </header>

    <!-- 过滤条与提示 -->
    <div class="flex flex-wrap items-center justify-between gap-3 bg-muted/40 p-3 rounded-lg border">
      <div class="flex flex-wrap items-center gap-2">
        <span class="text-xs font-medium text-muted-foreground mr-1">过滤：</span>
        <Select v-model="deviceFilter">
          <SelectTrigger class="h-8 w-36 text-xs">
            <SelectValue placeholder="设备筛选" />
          </SelectTrigger>
          <SelectContent>
            <SelectItem value="all">全部设备</SelectItem>
            <SelectItem value="pixel-8-001">Pixel 8</SelectItem>
            <SelectItem value="redmi-k70-002">Redmi K70</SelectItem>
          </SelectContent>
        </Select>

        <Select v-model="statusFilter">
          <SelectTrigger class="h-8 w-32 text-xs">
            <SelectValue placeholder="状态筛选" />
          </SelectTrigger>
          <SelectContent>
            <SelectItem value="all">全部状态</SelectItem>
            <SelectItem value="running">运行中</SelectItem>
            <SelectItem value="success">成功</SelectItem>
            <SelectItem value="failed">失败</SelectItem>
          </SelectContent>
        </Select>
      </div>

      <div class="flex items-center gap-3 text-xs text-muted-foreground">
        <span v-if="scanStats.incomplete > 0" class="flex items-center gap-1 text-amber-600 dark:text-amber-400 font-medium">
          <AlertTriangle class="size-3.5" />
          {{ scanStats.incomplete }} 个任务业务成功但产出不完整
        </span>
        <span class="hidden sm:inline">无后台轮询，仅在点击“扫描刷新”时重构索引</span>
      </div>
    </div>

    <!-- 主展示区：任务列表 + 任务详情 -->
    <div class="grid grid-cols-1 lg:grid-cols-12 gap-5 items-start">
      <!-- 任务列表 (5列) -->
      <div class="lg:col-span-5 space-y-2">
        <div class="text-xs font-medium text-muted-foreground px-1 mb-1">
          任务列表 ({{ filteredTasks.length }})
        </div>

        <div
          v-for="task in filteredTasks"
          :key="task.id"
          class="p-3.5 rounded-lg border cursor-pointer transition-all hover:border-primary/50 text-left"
          :class="selectedTaskId === task.id ? 'border-primary bg-primary/5 shadow-xs' : 'bg-card'"
          @click="selectedTaskId = task.id"
        >
          <div class="flex items-start justify-between gap-2">
            <div class="font-medium text-sm line-clamp-1">{{ task.name }}</div>
            <div class="flex items-center gap-1.5 shrink-0">
              <Badge v-if="task.status === 'success' && task.outputs_complete" variant="default" class="bg-emerald-600 text-xs">
                全部完成
              </Badge>
              <Badge v-else-if="task.status === 'success' && !task.outputs_complete" variant="outline" class="border-amber-500 text-amber-600 text-xs">
                业务成功 · 产出不完整
              </Badge>
              <Badge v-else-if="task.status === 'running'" variant="secondary" class="text-xs animate-pulse">
                运行中
              </Badge>
              <Badge v-else variant="destructive" class="text-xs">
                {{ task.status }}
              </Badge>
            </div>
          </div>

          <p class="text-xs text-muted-foreground mt-1 line-clamp-1 font-mono">
            {{ task.id }} · {{ task.device_serial }}
          </p>

          <div class="mt-2.5 flex items-center justify-between text-xs text-muted-foreground border-t pt-2">
            <span>更新：{{ task.updated_at }}</span>
            <span v-if="task.is_stale" class="text-amber-500 font-medium">信息陈旧</span>
            <span v-else class="text-emerald-600 dark:text-emerald-400">{{ task.business_result }}</span>
          </div>
        </div>
      </div>

      <!-- 任务详情抽屉/卡片 (7列) -->
      <div v-if="currentTask" class="lg:col-span-7 rounded-lg border bg-card p-4 space-y-4">
        <div class="flex items-start justify-between gap-3 border-b pb-3">
          <div>
            <div class="flex items-center gap-2">
              <h2 class="text-base font-bold">{{ currentTask.name }}</h2>
              <Badge variant="outline" class="font-mono text-[11px]">{{ currentTask.id }}</Badge>
            </div>
            <p class="text-xs text-muted-foreground mt-0.5">
              设备：<span class="font-medium text-foreground">{{ currentTask.device_serial }}</span> ·
              技能：<span class="font-mono">{{ currentTask.skill_dir }}</span>
            </p>
          </div>

          <Button
            variant="ghost"
            size="sm"
            class="h-8 text-xs shrink-0"
            :disabled="refreshingDetail"
            @click="refreshCurrentTask"
          >
            <RefreshCw class="size-3.5 mr-1" :class="{ 'animate-spin': refreshingDetail }" />
            刷新详情
          </Button>
        </div>

        <!-- 目标与结果 -->
        <div class="grid grid-cols-1 sm:grid-cols-2 gap-3 text-xs bg-muted/30 p-3 rounded-md border">
          <div>
            <span class="text-muted-foreground font-medium">任务目标：</span>
            <p class="mt-0.5 font-normal">{{ currentTask.goal }}</p>
          </div>
          <div>
            <span class="text-muted-foreground font-medium">业务结果：</span>
            <p class="mt-0.5 font-normal text-emerald-600 dark:text-emerald-400">{{ currentTask.business_result }}</p>
          </div>
        </div>

        <!-- 详情选项卡 -->
        <Tabs v-model="detailTab" class="w-full">
          <TabsList class="grid grid-cols-4 h-9">
            <TabsTrigger value="outputs" class="text-xs">
              产出清单 ({{ currentTask.outputs.length }})
            </TabsTrigger>
            <TabsTrigger value="report" class="text-xs">报告预览</TabsTrigger>
            <TabsTrigger value="logs" class="text-xs">日志流</TabsTrigger>
            <TabsTrigger value="actions" class="text-xs">操作历史</TabsTrigger>
          </TabsList>

          <!-- 声明产出卡片列表 -->
          <TabsContent value="outputs" class="space-y-3 pt-3">
            <div class="text-xs text-muted-foreground mb-1">
              按 <span class="font-mono">task.json.outputs</span> 严格匹配，真实校验工作目录产出：
            </div>

            <div
              v-for="out in currentTask.outputs"
              :key="out.id"
              class="flex items-center justify-between p-3 rounded-lg border text-xs"
              :class="out.exists ? 'bg-background' : 'bg-destructive/5 border-destructive/30'"
            >
              <div class="flex items-center gap-2.5">
                <FileText v-if="out.category === 'report'" class="size-4 text-blue-500" />
                <Terminal v-else-if="out.category === 'log'" class="size-4 text-amber-500" />
                <ImageIcon v-else-if="out.category === 'screenshot'" class="size-4 text-emerald-500" />
                <Video v-else class="size-4 text-purple-500" />

                <div>
                  <div class="font-mono font-medium">{{ out.path }}</div>
                  <div class="text-muted-foreground text-[11px]">
                    类别: {{ out.category }} · 格式: {{ out.format }} ·
                    {{ out.required ? '必需产出' : '可选产出' }}
                  </div>
                </div>
              </div>

              <div class="flex items-center gap-2">
                <span v-if="out.exists" class="font-mono text-muted-foreground">{{ out.size || '已生成' }}</span>
                <Badge v-if="out.exists" variant="secondary" class="text-[11px] bg-emerald-50 text-emerald-700 dark:bg-emerald-950 dark:text-emerald-300">
                  有效
                </Badge>
                <Badge v-else variant="destructive" class="text-[11px]">
                  缺失
                </Badge>
              </div>
            </div>
          </TabsContent>

          <!-- 报告预览 -->
          <TabsContent value="report" class="pt-3">
            <div v-if="currentTask.report_content" class="p-3 bg-muted/20 border rounded-lg text-xs leading-relaxed whitespace-pre-wrap font-sans">
              {{ currentTask.report_content }}
            </div>
            <div v-else class="text-center py-8 text-xs text-muted-foreground">
              未声明或未生成 report.md
            </div>
          </TabsContent>

          <!-- 日志流 -->
          <TabsContent value="logs" class="pt-3 space-y-2">
            <div class="flex justify-between items-center text-xs">
              <span class="text-muted-foreground">工作目录: {{ currentTask.skill_dir }}/logs/task.log</span>
              <Button variant="outline" size="sm" class="h-7 text-xs">
                <Download class="size-3 mr-1" />
                下载完整日志
              </Button>
            </div>
            <div class="bg-zinc-950 text-zinc-100 p-3 rounded-lg font-mono text-[11px] leading-5 max-h-60 overflow-y-auto">
              <div v-for="(l, i) in currentTask.logs" :key="i">{{ l }}</div>
            </div>
          </TabsContent>

          <!-- 操作记录 -->
          <TabsContent value="actions" class="pt-3 space-y-2">
            <div class="text-xs text-muted-foreground">actions.jsonl 追加式记录：</div>
            <div class="border rounded-md overflow-hidden text-xs">
              <table class="w-full text-left">
                <thead class="bg-muted text-muted-foreground border-b font-medium">
                  <tr>
                    <th class="p-2">时间</th>
                    <th class="p-2">动作</th>
                    <th class="p-2">结果</th>
                  </tr>
                </thead>
                <tbody>
                  <tr v-for="(act, i) in currentTask.actions" :key="i" class="border-b last:border-none">
                    <td class="p-2 font-mono text-muted-foreground">{{ act.timestamp }}</td>
                    <td class="p-2 font-medium">{{ act.action }}</td>
                    <td class="p-2">{{ act.result }}</td>
                  </tr>
                </tbody>
              </table>
            </div>
          </TabsContent>
        </Tabs>
      </div>
    </div>

    <!-- 系统与 AI 设置 Dialog (原型展示) -->
    <Dialog v-model:open="configOpen">
      <DialogContent class="sm:max-w-lg">
        <DialogHeader>
          <DialogTitle>系统设置 · 服务与 AI 配置</DialogTitle>
          <DialogDescription class="text-xs">
            配置任务工作根目录与视觉 AI 模型端点。API 密钥在服务端受控存储，读接口不回传明文，日志严格脱敏。
          </DialogDescription>
        </DialogHeader>

        <div class="space-y-4 py-2 text-xs">
          <!-- 任务目录 -->
          <div class="space-y-1.5 border rounded-lg p-3 bg-muted/20">
            <label class="font-medium">任务工作根目录</label>
            <Input v-model="taskRootDir" class="h-8 font-mono text-xs" />
            <p class="text-[11px] text-muted-foreground">
              默认 ~/.mywebscrcppy/tasks/。修改后清空旧索引，等待下次手动扫描。
            </p>
          </div>

          <!-- AI 配置 -->
          <div class="space-y-3 border rounded-lg p-3">
            <div class="flex items-center gap-1.5 font-medium text-foreground">
              <Sparkles class="size-4 text-primary" />
              <span>视觉决策模型 (Chat Completions)</span>
            </div>

            <div class="space-y-1">
              <label class="text-muted-foreground">端点地址 (Base URL / Chat Completions)</label>
              <Input v-model="aiEndpoint" placeholder="https://api.openai.com/v1/chat/completions" class="h-8 font-mono text-xs" />
            </div>

            <div class="grid grid-cols-2 gap-3">
              <div class="space-y-1">
                <label class="text-muted-foreground">模型名称</label>
                <Input v-model="aiModel" placeholder="gpt-4o-mini" class="h-8 font-mono text-xs" />
              </div>
              <div class="space-y-1">
                <label class="text-muted-foreground">超时预算 (秒)</label>
                <Input v-model="aiTimeout" type="number" class="h-8 font-mono text-xs" />
              </div>
            </div>

            <div class="space-y-1">
              <label class="text-muted-foreground">API 密钥 (服务端加密存储，不明文回显)</label>
              <Input v-model="aiApiKey" type="password" class="h-8 font-mono text-xs" />
            </div>
          </div>
        </div>

        <DialogFooter>
          <Button variant="outline" size="sm" @click="configOpen = false">取消</Button>
          <Button size="sm" @click="saveAiConfig">
            <CheckCircle2 v-if="configSaved" class="size-3.5 mr-1 text-emerald-500" />
            {{ configSaved ? '已保存' : '保存设置' }}
          </Button>
        </DialogFooter>
      </DialogContent>
    </Dialog>
  </div>
</template>
