<script setup lang="ts">
import { computed, ref, watch } from 'vue'
import {
  Activity,
  AlertCircle,
  Bot,
  Calendar,
  Check,
  CheckCircle2,
  Clock,
  Code2,
  Copy,
  Database,
  Eye,
  FileCode,
  FileEdit,
  Image as ImageIcon,
  Play,
  Plus,
  RefreshCw,
  Search,
  Send,
  Settings2,
  Smartphone,
  Sparkles,
  Square,
  User,
} from 'lucide-vue-next'
import { Badge } from '@/components/ui/badge'
import { Button } from '@/components/ui/button'
import { Card, CardContent, CardHeader, CardTitle } from '@/components/ui/card'
import {
  Dialog,
  DialogContent,
  DialogDescription,
  DialogFooter,
  DialogHeader,
  DialogTitle,
} from '@/components/ui/dialog'
import { Input } from '@/components/ui/input'
import { ScrollArea } from '@/components/ui/scroll-area'
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from '@/components/ui/select'

interface ScriptItem {
  id: string
  name: string
  content: string
  deviceSerial: string
  updatedAt: string
  compiledVersion: string
  compiledPython: string
}

// 任务管理器运行实例
interface RunningTask {
  id: string
  scriptId: string
  scriptName: string
  deviceSerial: string
  pid: number
  status: 'running' | 'stopping' | 'completed' | 'failed'
  startedAt: string
  elapsedSec: number
  timeoutSec: number
}

interface TemplateAsset {
  id: string
  name: string
  threshold: number
  previewUrl: string
  description: string
}

interface ChatMessage {
  id: string
  role: 'user' | 'assistant'
  content: string
  time: string
  actionType?: 'generate' | 'adjust'
}

// 对话历史
const chatMessages = ref<ChatMessage[]>([
  {
    id: 'm1',
    role: 'assistant',
    content: '你好！我是 Markdown 脚本助手。你可以直接向我描述自动化需求来「自动生成文档」，或者对当前文档提出修改意见进行「调整文档」。',
    time: '15:28',
  },
])
const inputPrompt = ref('')
const isAiResponding = ref(false)

function sendAiPrompt() {
  const prompt = inputPrompt.value.trim()
  if (!prompt || isAiResponding.value) return

  const now = new Date()
  const timeStr = `${String(now.getHours()).padStart(2, '0')}:${String(now.getMinutes()).padStart(2, '0')}`

  chatMessages.value.push({
    id: `m_${Date.now()}`,
    role: 'user',
    content: prompt,
    time: timeStr,
  })
  inputPrompt.value = ''
  isAiResponding.value = true

  setTimeout(() => {
    isAiResponding.value = false
    const lower = prompt.toLowerCase()

    if (lower.includes('生成') || lower.includes('新建') || lower.includes('写一个') || lower.includes('小红书') || lower.includes('抖音')) {
      // 场景 1: 自动生成新文档
      const generatedMd = `---
name: 社交平台内容巡检与点赞
version: 1.0.0
schedule: "0 */2 * * *"
target_device: auto
timeout_sec: 150
enabled: true
---

# 自动巡检流程

## 第一步：唤醒并打开应用
- 唤醒手机屏幕
- 向上滑动屏幕解锁
- 启动应用包名 \`com.xingin.xhs\`
- 等待 3 秒直到推荐页加载完毕

## 第二步：寻找推荐内容并互动
- 向下滑动浏览两条推荐帖子
- 匹配并点击页面点赞图标：
  ![点赞红心按钮](template://tmpl_sign_btn?threshold=0.82)
- 等待 1.5 秒确认点赞生效
`
      editorContent.value = generatedMd
      chatMessages.value.push({
        id: `m_${Date.now() + 1}`,
        role: 'assistant',
        content: '已为您自动生成了「社交平台内容巡检与点赞」文档，已配置初始版本 1.0.0 与每两小时巡检一次的周期，并自动关联了点赞图标模板。您可以在中间预览区直接核对。',
        time: timeStr,
        actionType: 'generate',
      })
    } else {
      // 场景 2: 调整/润色现有文档
      if (prompt.includes('重试') || prompt.includes('异常') || prompt.includes('等待')) {
        editorContent.value = editorContent.value.replace(
          /## 第二步：打开微信主界面/g,
          '## 第二步：打开微信主界面\n- 若应用未响应，自动重试 1 次\n- 增加屏幕加载等待至 5 秒',
        )
        chatMessages.value.push({
          id: `m_${Date.now() + 1}`,
          role: 'assistant',
          content: '已根据您的要求调整文档：在「打开微信主界面」步骤中增加了 1 次自动重试机制与 5 秒加载等待。',
          time: timeStr,
          actionType: 'adjust',
        })
      } else if (prompt.includes('版本') || prompt.includes('升级')) {
        editorContent.value = editorContent.value.replace(/version:\s*1\.2\.0/g, 'version: 1.2.1')
        chatMessages.value.push({
          id: `m_${Date.now() + 1}`,
          role: 'assistant',
          content: '已将版本号升级至 1.2.1，右侧判定状态已变更为待重新生成 Python 脚本。',
          time: timeStr,
          actionType: 'adjust',
        })
      } else {
        editorContent.value += `\n\n## 补充步骤：安全退出\n- 按返回键返回桌面\n- 锁定屏幕\n`
        chatMessages.value.push({
          id: `m_${Date.now() + 1}`,
          role: 'assistant',
          content: `已为您在文档末尾追加了针对「${prompt}」的补充流程，包含返回桌面与锁屏指令。`,
          time: timeStr,
          actionType: 'adjust',
        })
      }
    }
  }, 800)
}

// 快速填入示例 Prompt
function fillPrompt(text: string) {
  inputPrompt.value = text
  sendAiPrompt()
}

// 模拟任务管理器活跃列表
const activeTasks = ref<RunningTask[]>([
  {
    id: 'task_run_101',
    scriptId: 's_01',
    scriptName: '微信自动签到任务',
    deviceSerial: '10.0.0.30:5555',
    pid: 89412,
    status: 'running',
    startedAt: '15:32:10',
    elapsedSec: 42,
    timeoutSec: 180,
  },
])
const taskManagerDialogOpen = ref(false)

// 任务管理器操作：终止进程
function killTask(task: RunningTask) {
  task.status = 'stopping'
  setTimeout(() => {
    activeTasks.value = activeTasks.value.filter(t => t.id !== task.id)
  }, 400)
}

// 模拟立即运行脚本（带设备级锁检查）
const runNotice = ref<{ type: 'locked' | 'success', text: string } | null>(null)
function executeScriptWithLock() {
  const targetDevice = parsedMeta.value.targetDevice === 'auto' ? '10.0.0.30:5555' : parsedMeta.value.targetDevice
  const existingRunning = activeTasks.value.find(t => t.deviceSerial === targetDevice && t.status === 'running')

  if (existingRunning) {
    runNotice.value = {
      type: 'locked',
      text: `设备 [${targetDevice}] 当前正被任务「${existingRunning.scriptName}」(PID: ${existingRunning.pid}) 占用中，已触发互斥保护！请先等待其完成或在任务管理器中结束。`,
    }
    return
  }

  // 无占用：生成新 PID 并注册到任务管理器
  const randomPid = Math.floor(10000 + Math.random() * 80000)
  const newTask: RunningTask = {
    id: `task_${Date.now()}`,
    scriptId: currentScript.value?.id || 's_new',
    scriptName: currentScript.value?.name || '自动化脚本',
    deviceSerial: targetDevice,
    pid: randomPid,
    status: 'running',
    startedAt: new Date().toLocaleTimeString(),
    elapsedSec: 0,
    timeoutSec: parsedMeta.value.timeoutSec,
  }
  activeTasks.value.unshift(newTask)
  runNotice.value = {
    type: 'success',
    text: `成功在设备 [${targetDevice}] 启动进程 (PID: ${randomPid})，已注册至任务管理器！`,
  }
  setTimeout(() => {
    runNotice.value = null
  }, 4000)
}
const templateAssets = ref<TemplateAsset[]>([
  {
    id: 'tmpl_wechat_tab',
    name: '微信主页底部消息按钮',
    threshold: 0.85,
    previewUrl: 'data:image/svg+xml;utf8,<svg xmlns="http://www.w3.org/2000/svg" width="64" height="64" viewBox="0 0 24 24" fill="none" stroke="%2310b981" stroke-width="2"><path d="M21 15a2 2 0 0 1-2 2H7l-4 4V5a2 2 0 0 1 2-2h14a2 2 0 0 1 2 2z"/></svg>',
    description: '匹配底部绿色消息 Tab 高亮图标',
  },
  {
    id: 'tmpl_sign_btn',
    name: '签到打卡按钮',
    threshold: 0.80,
    previewUrl: 'data:image/svg+xml;utf8,<svg xmlns="http://www.w3.org/2000/svg" width="64" height="64" viewBox="0 0 24 24" fill="none" stroke="%233b82f6" stroke-width="2"><circle cx="12" cy="12" r="10"/><path d="m9 12 2 2 4-4"/></svg>',
    description: '页面中部圆形打卡橙色/蓝色按钮',
  },
  {
    id: 'tmpl_confirm_modal',
    name: '弹窗确认按钮',
    threshold: 0.90,
    previewUrl: 'data:image/svg+xml;utf8,<svg xmlns="http://www.w3.org/2000/svg" width="64" height="64" viewBox="0 0 24 24" fill="none" stroke="%23f59e0b" stroke-width="2"><rect width="18" height="18" x="3" y="3" rx="2"/><path d="m9 12 2 2 4-4"/></svg>',
    description: '通用对话框右下角主操作按钮',
  },
])

// 初始示例 Markdown 内容
const sampleMarkdown1 = `---
name: 微信自动签到任务
version: 1.2.0
schedule: "*/30 * * * *"
target_device: auto
timeout_sec: 180
enabled: true
---

# 业务操作流程

## 第一步：唤醒并解锁手机
- 唤醒屏幕并保持常亮
- 从屏幕底部 (500, 1800) 上滑至 (500, 600) 解锁

## 第二步：打开微信主界面
- 启动应用包名 \`com.tencent.mm\`
- 等待 3 秒等待主页面加载
- 匹配并点击底部消息图标：
  ![微信消息图标](template://tmpl_wechat_tab?threshold=0.85)

## 第三步：进入服务号完成打卡
- 点击页面上方搜索框或第一条置顶
- 匹配并点击打卡按钮：
  ![签到打卡按钮](template://tmpl_sign_btn?threshold=0.80)
- 等待 1 秒确认签到成功
`

const samplePython1 = `# -*- coding: utf-8 -*-
"""
Generated by MyWebScrcpy Markdown Skill
Script: 微信自动签到任务
Version: 1.2.0
Generated At: 2026-10-11 15:30:00
Schedule: */30 * * * *
"""
import time
from mywebscrcpy.device import Device
from mywebscrcpy.vision import Matcher

def run(serial: str = "auto") -> bool:
    dev = Device.connect(serial)
    matcher = Matcher(dev)
    
    # 步骤 1: 唤醒并解锁手机
    print("[Step 1] 唤醒屏幕并滑动解锁...")
    dev.wake_up()
    dev.swipe(500, 1800, 500, 600, duration_ms=300)
    time.sleep(1.0)
    
    # 步骤 2: 打开微信主界面
    print("[Step 2] 启动 com.tencent.mm...")
    dev.start_app("com.tencent.mm")
    time.sleep(3.0)
    
    # 模板匹配: tmpl_wechat_tab
    print("[Step 2.1] 寻找并点击模板: tmpl_wechat_tab (阈值: 0.85)")
    target = matcher.wait_and_click("tmpl_wechat_tab", threshold=0.85, timeout=10.0)
    if not target:
        raise RuntimeError("未定位到微信消息图标")
        
    # 步骤 3: 匹配打卡按钮
    print("[Step 3] 寻找并点击模板: tmpl_sign_btn (阈值: 0.80)")
    signed = matcher.wait_and_click("tmpl_sign_btn", threshold=0.80, timeout=10.0)
    if not signed:
        raise RuntimeError("未定位到签到按钮")
        
    print("[Success] 脚本执行完成")
    return True

if __name__ == "__main__":
    run()
`

const sampleMarkdown2 = `---
name: 游戏每日福利巡检
# 未填写 version，将自动使用文件时间戳作为版本号
schedule: "0 9 * * *"
target_device: 10.0.0.30:5555
timeout_sec: 240
enabled: true
---

# 巡检流程

## 启动游戏
- 启动应用 \`com.sample.game\`
- 等待 5 秒完成冷启动

## 确认弹窗
- 检测是否有日常弹窗并关闭：
  ![弹窗确认按钮](template://tmpl_confirm_modal?threshold=0.90)
`

// 脚本列表
const scripts = ref<ScriptItem[]>([
  {
    id: 's_01',
    name: '微信自动签到任务',
    content: sampleMarkdown1,
    deviceSerial: 'auto',
    updatedAt: '2026-10-11 15:30:00',
    compiledVersion: '1.2.0',
    compiledPython: samplePython1,
  },
  {
    id: 's_02',
    name: '游戏每日福利巡检',
    content: sampleMarkdown2,
    deviceSerial: '10.0.0.30:5555',
    updatedAt: '2026-10-11 16:15:22',
    compiledVersion: '',
    compiledPython: '',
  },
])

// 当前选中的脚本 ID
const currentScriptId = ref<string>('s_01')
const currentScript = computed(() => scripts.value.find(s => s.id === currentScriptId.value) || scripts.value[0])

// 当前正在编辑的 Markdown 源码
const editorContent = ref(currentScript.value?.content || '')

// 切换脚本时同步编辑内容
watch(currentScriptId, () => {
  if (currentScript.value) {
    editorContent.value = currentScript.value.content
  }
})

// 正则解析器逻辑：提取版本号、定时周期、存储方式等
const parsedMeta = computed(() => {
  const text = editorContent.value

  // 1. 版本号正则: version: x.y.z
  const versionMatch = text.match(/(?:^|\n)version\s*:\s*["']?([a-zA-Z0-9._-]+)["']?/i)
  const hasVersionInMd = !!versionMatch
  const version = versionMatch ? versionMatch[1] : `ts_${currentScript.value?.updatedAt.replace(/[- :]/g, '') || '20261011160000'}`

  // 2. 定时周期: schedule: "..."
  const scheduleMatch = text.match(/(?:^|\n)schedule\s*:\s*["']?([^"'\r\n]+)["']?/i)
  const schedule = scheduleMatch ? scheduleMatch[1] : '手动运行'

  // 3. 目标设备
  const targetDeviceMatch = text.match(/(?:^|\n)target_device\s*:\s*["']?([^"'\r\n]+)["']?/i)
  const targetDevice = targetDeviceMatch ? targetDeviceMatch[1] : 'auto'

  // 4. 超时时间
  const timeoutMatch = text.match(/(?:^|\n)timeout_sec\s*:\s*(\d+)/i)
  const timeoutSec = timeoutMatch ? Number.parseInt(timeoutMatch[1], 10) : 120

  // 5. 提取模板引用: ![name](template://id?threshold=0.8)
  const templateRegex = /!\[(.*?)\]\((template:\/\/([^?\s)]+)(?:\?threshold=([0-9.]+))?\))/g
  const matchedTemplates: Array<{ alt: string, id: string, threshold: number }> = []
  let match: RegExpExecArray | null
  // eslint-disable-next-line no-cond-assign
  while ((match = templateRegex.exec(text)) !== null) {
    matchedTemplates.push({
      alt: match[1] || match[3],
      id: match[3],
      threshold: match[4] ? Number.parseFloat(match[4]) : 0.85,
    })
  }

  // 人类可读周期翻译
  let scheduleHuman = '未配置定时'
  if (schedule === '*/30 * * * *')
    scheduleHuman = '每 30 分钟执行一次'
  else if (schedule === '0 9 * * *')
    scheduleHuman = '每天早晨 09:00 准时执行'
  else if (schedule && schedule !== '手动运行')
    scheduleHuman = `Cron: ${schedule}`

  return {
    version,
    hasVersionInMd,
    schedule,
    scheduleHuman,
    targetDevice,
    timeoutSec,
    matchedTemplates,
  }
})

// 判断当前版本是否已修改、是否需要重新生成 Python 脚本
const isUpToDate = computed(() => {
  if (!currentScript.value)
    return false
  if (!currentScript.value.compiledPython)
    return false
  return currentScript.value.compiledVersion === parsedMeta.value.version
})

// 结构化预览解析（段落与步骤）
interface ParsedSection {
  title: string
  steps: Array<{
    text: string
    isTemplate: boolean
    tmplId?: string
    tmplAlt?: string
    threshold?: number
  }>
}

const previewSections = computed<ParsedSection[]>(() => {
  const lines = editorContent.value.split('\n')
  const sections: ParsedSection[] = []
  let currentSec: ParsedSection = { title: '总览流程', steps: [] }

  for (const line of lines) {
    const trimmed = line.trim()
    if (trimmed.startsWith('# ')) {
      currentSec = { title: trimmed.replace(/^#\s+/, ''), steps: [] }
      sections.push(currentSec)
    }
    else if (trimmed.startsWith('## ')) {
      currentSec = { title: trimmed.replace(/^##\s+/, ''), steps: [] }
      sections.push(currentSec)
    }
    else if (trimmed.startsWith('- ')) {
      const stepText = trimmed.replace(/^- \s*/, '')
      const tmplMatch = stepText.match(/!\[(.*?)\]\((template:\/\/([^?\s)]+)(?:\?threshold=([0-9.]+))?\))/)
      if (tmplMatch) {
        currentSec.steps.push({
          text: stepText,
          isTemplate: true,
          tmplAlt: tmplMatch[1],
          tmplId: tmplMatch[3],
          threshold: tmplMatch[4] ? Number.parseFloat(tmplMatch[4]) : 0.85,
        })
      }
      else {
        currentSec.steps.push({
          text: stepText,
          isTemplate: false,
        })
      }
    }
  }

  return sections.filter(s => s.steps.length > 0 || s.title !== '总览流程')
})

// 模板选择弹窗控制
const templateDialogOpen = ref(false)
const templateSearch = ref('')
const filteredAssets = computed(() => {
  return templateAssets.value.filter(a =>
    a.name.toLowerCase().includes(templateSearch.value.trim().toLowerCase())
    || a.id.toLowerCase().includes(templateSearch.value.trim().toLowerCase()),
  )
})

function insertTemplate(asset: TemplateAsset) {
  const insertText = `\n- 匹配并点击目标：\n  ![${asset.name}](template://${asset.id}?threshold=${asset.threshold.toFixed(2)})\n`
  editorContent.value += insertText
  templateDialogOpen.value = false
}

// 模拟调用专用技能生成稳定的 Python 脚本
const isGenerating = ref(false)
function triggerSkillGeneration() {
  isGenerating.value = true
  setTimeout(() => {
    isGenerating.value = false
    const now = new Date()
    const timeStr = `${now.getFullYear()}-${String(now.getMonth() + 1).padStart(2, '0')}-${String(now.getDate()).padStart(2, '0')} ${String(now.getHours()).padStart(2, '0')}:${String(now.getMinutes()).padStart(2, '0')}:${String(now.getSeconds()).padStart(2, '0')}`

    // 自动根据步骤构建 Python 代码
    const stepsCode = previewSections.value.flatMap(sec => sec.steps.map(s => {
      if (s.isTemplate && s.tmplId) {
        return `    # 步骤: ${s.tmplAlt || s.tmplId}\n    matcher.wait_and_click("${s.tmplId}", threshold=${s.threshold || 0.85}, timeout=10.0)`
      }
      return `    # 步骤: ${s.text}\n    dev.log("${s.text.replace(/"/g, '\\"')}")`
    })).join('\n')

    const generated = `# -*- coding: utf-8 -*-
"""
Generated by MyWebScrcpy Markdown Skill
Script: ${currentScript.value?.name}
Version: ${parsedMeta.value.version}
Generated At: ${timeStr}
Schedule: ${parsedMeta.value.schedule}
"""
import time
from mywebscrcpy.device import Device
from mywebscrcpy.vision import Matcher

def run(serial: str = "${parsedMeta.value.targetDevice}") -> bool:
    dev = Device.connect(serial)
    matcher = Matcher(dev)
    print(f"正在执行 ${currentScript.value?.name} (版本: ${parsedMeta.value.version})...")

${stepsCode || '    print("无具体执行指令")'}

    print("[Success] 脚本执行完毕")
    return True

if __name__ == "__main__":
    run()
`
    if (currentScript.value) {
      currentScript.value.content = editorContent.value
      currentScript.value.compiledVersion = parsedMeta.value.version
      currentScript.value.compiledPython = generated
    }
  }, 600)
}

// 保存当前修改
function saveChanges() {
  if (currentScript.value) {
    currentScript.value.content = editorContent.value
    currentScript.value.updatedAt = new Date().toLocaleTimeString()
  }
}

// 快捷插入步骤
function insertStep(type: 'wait' | 'swipe' | 'tap') {
  if (type === 'wait') {
    editorContent.value += '\n- 等待 2 秒\n'
  }
  else if (type === 'swipe') {
    editorContent.value += '\n- 从屏幕底部 (500, 1600) 向上滑动至 (500, 600)\n'
  }
  else if (type === 'tap') {
    editorContent.value += '\n- 点击屏幕坐标 (540, 960)\n'
  }
}

// 复制 Python 代码
const copied = ref(false)
function copyPythonCode() {
  if (currentScript.value?.compiledPython) {
    navigator.clipboard.writeText(currentScript.value.compiledPython)
    copied.value = true
    setTimeout(() => {
      copied.value = false
    }, 2000)
  }
}
</script>

<template>
  <div class="space-y-6">
    <!-- 顶部状态与标题 -->
    <header class="flex flex-col gap-4 md:flex-row md:items-center md:justify-between border-b pb-4">
      <div>
        <div class="flex items-center gap-2">
          <h1 class="text-2xl font-bold tracking-tight">
            Markdown 自动化脚本管理
          </h1>
          <Badge variant="outline" class="text-xs">
            v1.16 原型
          </Badge>
        </div>
        <p class="text-sm text-muted-foreground mt-1">
          基于对话交互自动生成与调整 Markdown 文档，支持模板匹配与稳定 Python 脚本编译。
        </p>
      </div>

      <!-- 右上角快捷操作 -->
      <div class="flex items-center gap-2">
        <Select v-model="currentScriptId">
          <SelectTrigger class="w-[180px]">
            <SelectValue placeholder="选择脚本" />
          </SelectTrigger>
          <SelectContent>
            <SelectItem v-for="s in scripts" :key="s.id" :value="s.id">
              {{ s.name }}
            </SelectItem>
          </SelectContent>
        </Select>

        <Button variant="outline" class="relative" @click="taskManagerDialogOpen = true">
          <Activity class="h-4 w-4 mr-1 text-emerald-500" />
          任务管理器
          <Badge v-if="activeTasks.length > 0" variant="default" class="ml-1.5 h-4 px-1 text-[10px] bg-emerald-600">
            {{ activeTasks.length }}
          </Badge>
        </Button>

        <Button variant="outline" @click="templateDialogOpen = true">
          <ImageIcon class="h-4 w-4 mr-1 text-blue-500" />
          模板库管理
        </Button>
        <Button @click="saveChanges">
          <Check class="h-4 w-4 mr-1" />
          保存变更
        </Button>
      </div>
    </header>

    <!-- 四栏/四区核心工作台：[栏1: AI 对话助手 (生成与调整文档)] | [栏2: Markdown 源码] | [栏3: 实时预览] | [栏4: Python 产物] -->
    <div class="grid grid-cols-1 xl:grid-cols-12 gap-5 items-start">
      <!-- ================= 栏 1 (宽3/12): AI 对话助手 (自动生成与调整文档) ================= -->
      <div class="xl:col-span-3 space-y-4">
        <Card class="h-[760px] flex flex-col border-primary/20 shadow-sm">
          <CardHeader class="py-3 px-4 border-b flex flex-row items-center justify-between space-y-0 bg-primary/5">
            <div class="flex items-center gap-2">
              <Bot class="h-4 w-4 text-primary" />
              <CardTitle class="text-sm font-medium">
                AI 文档助手
              </CardTitle>
            </div>
            <Badge variant="secondary" class="text-[10px] px-1.5 py-0 h-5">
              生成 / 调整
            </Badge>
          </CardHeader>

          <!-- 快捷提示词标签 -->
          <div class="p-2.5 border-b bg-muted/20 space-y-1.5">
            <div class="text-[11px] text-muted-foreground flex items-center gap-1">
              <Sparkles class="h-3 w-3 text-amber-500" />
              <span>常用快捷指令：</span>
            </div>
            <div class="flex flex-wrap gap-1">
              <button
                class="text-[10px] bg-background hover:bg-muted border rounded px-1.5 py-0.5 transition-colors text-left"
                @click="fillPrompt('自动生成一个小红书点赞脚本')"
              >
                ✨ 自动生成小红书脚本
              </button>
              <button
                class="text-[10px] bg-background hover:bg-muted border rounded px-1.5 py-0.5 transition-colors text-left"
                @click="fillPrompt('给微信打开步骤增加异常重试和等待时间')"
              >
                🛠️ 调整：增加重试与等待
              </button>
              <button
                class="text-[10px] bg-background hover:bg-muted border rounded px-1.5 py-0.5 transition-colors text-left"
                @click="fillPrompt('升级当前文档版本号为 1.2.1')"
              >
                🆙 调整：升级版本号
              </button>
            </div>
          </div>

          <!-- 对话消息列表 -->
          <CardContent class="p-0 flex-1 flex flex-col justify-between overflow-hidden">
            <ScrollArea class="flex-1 p-3">
              <div class="space-y-3">
                <div
                  v-for="msg in chatMessages"
                  :key="msg.id"
                  class="flex gap-2"
                  :class="msg.role === 'user' ? 'justify-end' : 'justify-start'"
                >
                  <div
                    v-if="msg.role === 'assistant'"
                    class="w-6 h-6 rounded-full bg-primary/10 flex items-center justify-center shrink-0 mt-0.5"
                  >
                    <Bot class="h-3.5 w-3.5 text-primary" />
                  </div>

                  <div
                    class="max-w-[85%] rounded-lg px-2.5 py-2 text-xs leading-relaxed"
                    :class="msg.role === 'user' ? 'bg-primary text-primary-foreground' : 'bg-muted/60 border'"
                  >
                    <div>{{ msg.content }}</div>
                    <div
                      class="text-[9px] mt-1 text-right opacity-60"
                      :class="msg.role === 'user' ? 'text-primary-foreground' : 'text-muted-foreground'"
                    >
                      {{ msg.time }}
                    </div>
                  </div>

                  <div
                    v-if="msg.role === 'user'"
                    class="w-6 h-6 rounded-full bg-muted flex items-center justify-center shrink-0 mt-0.5"
                  >
                    <User class="h-3.5 w-3.5 text-foreground" />
                  </div>
                </div>

                <div v-if="isAiResponding" class="flex items-center gap-2 text-xs text-muted-foreground py-1">
                  <Bot class="h-3.5 w-3.5 animate-spin text-primary" />
                  <span>正在分析并修改 Markdown 文档...</span>
                </div>
              </div>
            </ScrollArea>

            <!-- 底部输入框 -->
            <div class="p-2.5 border-t bg-background">
              <form class="flex items-center gap-1.5" @submit.prevent="sendAiPrompt">
                <Input
                  v-model="inputPrompt"
                  placeholder="输入要求，如「自动生成...」或「修改步骤...」"
                  class="h-8 text-xs flex-1"
                  :disabled="isAiResponding"
                />
                <Button size="sm" type="submit" class="h-8 w-8 p-0 shrink-0" :disabled="!inputPrompt.trim() || isAiResponding">
                  <Send class="h-3.5 w-3.5" />
                </Button>
              </form>
              <div class="text-[10px] text-muted-foreground mt-1 text-center">
                支持自然语言自动生成或就地调整文档
              </div>
            </div>
          </CardContent>
        </Card>
      </div>

      <!-- ================= 栏 2 (宽3/12): Markdown 源码编辑区 ================= -->
      <div class="xl:col-span-3 space-y-4">
        <Card class="h-[760px] flex flex-col">
          <CardHeader class="py-3 px-4 border-b flex flex-row items-center justify-between space-y-0">
            <div class="flex items-center gap-2">
              <FileEdit class="h-4 w-4 text-primary" />
              <CardTitle class="text-sm font-medium">
                Markdown 源码
              </CardTitle>
            </div>
            <div class="flex items-center gap-1">
              <Button size="sm" variant="ghost" class="h-7 text-[11px] px-1.5" @click="insertStep('wait')">
                +等待
              </Button>
              <Button size="sm" variant="ghost" class="h-7 text-[11px] px-1.5" @click="insertStep('swipe')">
                +滑动
              </Button>
              <Button size="sm" variant="ghost" class="h-7 text-[11px] px-1.5" @click="insertStep('tap')">
                +点击
              </Button>
              <Button size="sm" variant="secondary" class="h-7 text-[11px] px-1.5" @click="templateDialogOpen = true">
                <Plus class="h-3 w-3 mr-0.5" />模板
              </Button>
            </div>
          </CardHeader>

          <CardContent class="p-0 flex-1 flex flex-col">
            <textarea
              v-model="editorContent"
              class="w-full flex-1 p-3 font-mono text-[11px] leading-relaxed resize-none bg-muted/20 focus:outline-none focus:ring-1 focus:ring-ring border-0"
              placeholder="在此编写 Markdown 脚本..."
              spellcheck="false"
            />
            <div class="px-3 py-2 bg-muted/40 border-t text-[11px] text-muted-foreground flex items-center justify-between">
              <span>YAML Front-Matter</span>
              <span>{{ editorContent.length }} 字符</span>
            </div>
          </CardContent>
        </Card>
      </div>

      <!-- ================= 栏 3 (宽3/12): Markdown 直观预览与交互区 ================= -->
      <div class="xl:col-span-3 space-y-4">
        <Card class="h-[760px] flex flex-col">
          <CardHeader class="py-3 px-4 border-b flex flex-row items-center justify-between space-y-0">
            <div class="flex items-center gap-2">
              <Eye class="h-4 w-4 text-emerald-500" />
              <CardTitle class="text-sm font-medium">
                Markdown 渲染预览
              </CardTitle>
            </div>
            <Badge variant="secondary" class="text-xs">
              {{ previewSections.length }} 阶段
            </Badge>
          </CardHeader>

          <!-- 顶部提取指标指示牌 -->
          <div class="p-2.5 bg-muted/20 border-b grid grid-cols-2 gap-2 text-xs">
            <div class="flex items-center gap-1.5 bg-background p-1.5 rounded border">
              <Calendar class="h-3.5 w-3.5 text-blue-500 shrink-0" />
              <div class="truncate">
                <div class="text-[9px] text-muted-foreground">提取版本</div>
                <div class="font-mono text-[11px] font-medium truncate flex items-center gap-1">
                  {{ parsedMeta.version }}
                  <span v-if="!parsedMeta.hasVersionInMd" class="text-[9px] text-amber-500">(时间戳)</span>
                </div>
              </div>
            </div>

            <div class="flex items-center gap-1.5 bg-background p-1.5 rounded border">
              <Clock class="h-3.5 w-3.5 text-purple-500 shrink-0" />
              <div class="truncate">
                <div class="text-[9px] text-muted-foreground">定时周期</div>
                <div class="font-medium text-[11px] truncate text-emerald-600 dark:text-emerald-400">
                  {{ parsedMeta.scheduleHuman }}
                </div>
              </div>
            </div>

            <div class="flex items-center gap-1.5 bg-background p-1.5 rounded border">
              <Database class="h-3.5 w-3.5 text-amber-500 shrink-0" />
              <div class="truncate">
                <div class="text-[9px] text-muted-foreground">存储介质</div>
                <div class="font-medium text-[11px]">
                  服务端 SQLite (纯集中管理)
                </div>
              </div>
            </div>

            <div class="flex items-center gap-1.5 bg-background p-1.5 rounded border">
              <Settings2 class="h-3.5 w-3.5 text-slate-500 shrink-0" />
              <div class="truncate">
                <div class="text-[9px] text-muted-foreground">关联设备</div>
                <div class="font-mono text-[11px] truncate">
                  {{ parsedMeta.targetDevice }}
                </div>
              </div>
            </div>
          </div>

          <!-- 步骤渲染列表 -->
          <ScrollArea class="flex-1 p-3">
            <div class="space-y-4">
              <div v-for="(sec, idx) in previewSections" :key="idx" class="space-y-2">
                <h3 class="text-xs font-semibold flex items-center gap-1.5 text-foreground/90">
                  <span class="w-1.5 h-3 bg-primary rounded-full inline-block" />
                  {{ sec.title }}
                </h3>

                <div class="space-y-1.5 pl-2.5 border-l-2 border-muted ml-0.5">
                  <div
                    v-for="(step, sIdx) in sec.steps"
                    :key="sIdx"
                    class="text-[11px] p-2 rounded-md transition-colors"
                    :class="step.isTemplate ? 'bg-blue-50/60 dark:bg-blue-950/20 border border-blue-200/50 dark:border-blue-900/50' : 'bg-muted/40'"
                  >
                    <!-- 模板匹配动作步骤 -->
                    <div v-if="step.isTemplate" class="flex items-center gap-1.5">
                      <div class="p-1 rounded bg-blue-100 dark:bg-blue-900/40 text-blue-600 dark:text-blue-400 shrink-0">
                        <ImageIcon class="h-3.5 w-3.5" />
                      </div>
                      <div class="flex-1 min-w-0">
                        <div class="font-medium text-blue-700 dark:text-blue-300 truncate">
                          {{ step.tmplAlt || step.tmplId }}
                        </div>
                        <div class="text-[9px] text-muted-foreground flex items-center gap-1.5 mt-0.5">
                          <span><code>{{ step.tmplId }}</code></span>
                          <span class="text-amber-600 dark:text-amber-400">阈值: {{ step.threshold }}</span>
                        </div>
                      </div>
                      <Badge variant="outline" class="text-[9px] px-1 py-0 h-4 shrink-0">
                        视觉
                      </Badge>
                    </div>

                    <!-- 普通文字控制步骤 -->
                    <div v-else class="flex items-start gap-1.5">
                      <div class="w-1.5 h-1.5 rounded-full bg-muted-foreground/60 mt-1 shrink-0" />
                      <span class="text-foreground/80 leading-relaxed">{{ step.text }}</span>
                    </div>
                  </div>
                </div>
              </div>

              <div v-if="previewSections.length === 0" class="py-12 text-center text-muted-foreground text-xs">
                暂无流程步骤，请在左侧编辑或使用 AI 生成文档。
              </div>
            </div>
          </ScrollArea>
        </Card>
      </div>

      <!-- ================= 栏 4 (宽3/12): 编译后的 Python 脚本产物 ================= -->
      <div class="xl:col-span-3 space-y-4">
        <Card class="h-[760px] flex flex-col">
          <CardHeader class="py-3 px-4 border-b flex flex-row items-center justify-between space-y-0">
            <div class="flex items-center gap-2">
              <Code2 class="h-4 w-4 text-purple-500" />
              <CardTitle class="text-sm font-medium">
                稳定 Python 产物
              </CardTitle>
            </div>
            <div class="flex items-center gap-1">
              <Button size="sm" variant="ghost" class="h-7 text-xs px-2" @click="copyPythonCode">
                <Check v-if="copied" class="h-3.5 w-3.5 text-emerald-500 mr-1" />
                <Copy v-else class="h-3.5 w-3.5 mr-1" />
                {{ copied ? '已复制' : '复制' }}
              </Button>
            </div>
          </CardHeader>

          <!-- 版本比对与缓存提示横幅 -->
          <div class="p-2.5 border-b text-xs transition-colors" :class="isUpToDate ? 'bg-emerald-50 dark:bg-emerald-950/20' : 'bg-amber-50 dark:bg-amber-950/20'">
            <div class="flex items-start gap-1.5">
              <CheckCircle2 v-if="isUpToDate" class="h-3.5 w-3.5 text-emerald-600 dark:text-emerald-400 shrink-0 mt-0.5" />
              <AlertCircle v-else class="h-3.5 w-3.5 text-amber-600 dark:text-amber-400 shrink-0 mt-0.5" />
              <div class="flex-1 min-w-0">
                <div class="font-medium text-[11px]" :class="isUpToDate ? 'text-emerald-800 dark:text-emerald-300' : 'text-amber-800 dark:text-amber-300'">
                  {{ isUpToDate ? '版本一致 · 复用稳定脚本' : '检测到版本变化 · 待生成' }}
                </div>
                <div class="text-[10px] text-muted-foreground mt-0.5 truncate">
                  <span v-if="isUpToDate">版本 ({{ currentScript?.compiledVersion }}) 未变，直接复用。</span>
                  <span v-else>提取版本 [{{ parsedMeta.version }}]，需编译。</span>
                </div>
              </div>
            </div>

            <!-- 生成触发按钮 -->
            <div class="mt-2 space-y-1.5">
              <div class="grid grid-cols-2 gap-1.5">
                <Button
                  size="sm"
                  class="h-7 text-xs font-normal"
                  :variant="isUpToDate ? 'outline' : 'default'"
                  :disabled="isGenerating"
                  @click="triggerSkillGeneration"
                >
                  <RefreshCw v-if="isGenerating" class="h-3 w-3 mr-1 animate-spin" />
                  <Sparkles v-else class="h-3 w-3 mr-1 text-amber-400" />
                  {{ isGenerating ? '编译中...' : (isUpToDate ? '重新生成' : '触发生成') }}
                </Button>

                <Button
                  size="sm"
                  variant="secondary"
                  class="h-7 text-xs font-normal"
                  :disabled="!currentScript?.compiledPython"
                  @click="executeScriptWithLock"
                >
                  <Play class="h-3 w-3 mr-1 text-emerald-500" />
                  立即运行(托管)
                </Button>
              </div>

              <!-- 执行提示与锁冲突预警 -->
              <div v-if="runNotice" class="p-1.5 rounded text-[10px] leading-tight flex items-start gap-1" :class="runNotice.type === 'locked' ? 'bg-destructive/15 text-destructive border border-destructive/20' : 'bg-emerald-100 text-emerald-800 dark:bg-emerald-950/40 dark:text-emerald-300'">
                <AlertCircle v-if="runNotice.type === 'locked'" class="h-3 w-3 shrink-0 mt-0.5" />
                <CheckCircle2 v-else class="h-3 w-3 shrink-0 mt-0.5" />
                <span class="flex-1">{{ runNotice.text }}</span>
              </div>
            </div>
          </div>

          <!-- Python 产物代码预览 -->
          <CardContent class="p-0 flex-1 flex flex-col bg-muted/20">
            <ScrollArea class="flex-1 p-2.5">
              <pre v-if="currentScript?.compiledPython" class="font-mono text-[11px] text-foreground/80 leading-relaxed whitespace-pre select-text"><code>{{ currentScript.compiledPython }}</code></pre>
              <div v-else class="py-24 text-center text-muted-foreground text-xs space-y-2">
                <FileCode class="h-8 w-8 mx-auto text-muted-foreground/40" />
                <div>尚未生成 Python 脚本</div>
                <p class="text-[10px]">点击上方按钮生成</p>
              </div>
            </ScrollArea>
          </CardContent>
        </Card>
      </div>
    </div>

    <!-- ================= 模板资产管理与插入弹窗 ================= -->
    <Dialog :open="templateDialogOpen" @update:open="templateDialogOpen = $event">
      <DialogContent class="sm:max-w-[560px]">
        <DialogHeader>
          <DialogTitle class="flex items-center gap-2">
            <ImageIcon class="h-5 w-5 text-blue-500" />
            模板资产库 (用于视觉匹配)
          </DialogTitle>
          <DialogDescription>
            选择现有识别模板插入到当前 Markdown 中，或调整匹配阈值。
          </DialogDescription>
        </DialogHeader>

        <div class="space-y-4 py-2">
          <div class="flex items-center gap-2">
            <Search class="h-4 w-4 text-muted-foreground ml-2" />
            <Input v-model="templateSearch" placeholder="搜索模板名称或 ID..." class="h-8 text-xs" />
          </div>

          <div class="space-y-2 max-h-[320px] overflow-y-auto pr-1">
            <div
              v-for="asset in filteredAssets"
              :key="asset.id"
              class="flex items-center justify-between p-2.5 rounded-lg border bg-card hover:bg-muted/40 transition-colors gap-3"
            >
              <div class="w-10 h-10 rounded border bg-muted/30 flex items-center justify-center shrink-0 overflow-hidden" v-html="asset.previewUrl" />
              <div class="flex-1 min-w-0">
                <div class="text-xs font-medium truncate">
                  {{ asset.name }}
                </div>
                <div class="text-[10px] text-muted-foreground flex items-center gap-2 mt-0.5">
                  <code>{{ asset.id }}</code>
                  <span class="text-blue-600 dark:text-blue-400">推荐阈值: {{ asset.threshold }}</span>
                </div>
              </div>
              <Button size="sm" variant="secondary" class="h-7 text-xs" @click="insertTemplate(asset)">
                <Plus class="h-3.5 w-3.5 mr-1" />插入
              </Button>
            </div>
          </div>
        </div>

        <DialogFooter>
          <Button variant="outline" @click="templateDialogOpen = false">
            关闭
          </Button>
        </DialogFooter>
      </DialogContent>
    </Dialog>

    <!-- ================= 任务管理器 (生命周期管理与进程锁) 弹窗 ================= -->
    <Dialog :open="taskManagerDialogOpen" @update:open="taskManagerDialogOpen = $event">
      <DialogContent class="sm:max-w-[680px]">
        <DialogHeader>
          <DialogTitle class="flex items-center gap-2">
            <Activity class="h-5 w-5 text-emerald-500" />
            任务管理器 (设备执行锁与进程监控)
          </DialogTitle>
          <DialogDescription>
            每台手机同一时间仅允许运行一个任务。所有活动进程 PID 登记在此，支持随时强制结束任务释放设备锁。
          </DialogDescription>
        </DialogHeader>

        <div class="py-2 space-y-3">
          <div class="flex items-center justify-between text-xs text-muted-foreground px-1">
            <span>活跃执行任务: {{ activeTasks.length }} 个</span>
            <span>状态说明：执行中即对目标设备施加独占排他锁</span>
          </div>

          <div v-if="activeTasks.length > 0" class="border rounded-lg divide-y">
            <div
              v-for="task in activeTasks"
              :key="task.id"
              class="p-3 flex items-center justify-between gap-4 hover:bg-muted/30 transition-colors"
            >
              <div class="flex items-start gap-3 min-w-0 flex-1">
                <div class="p-2 rounded bg-emerald-100 text-emerald-700 dark:bg-emerald-950/60 dark:text-emerald-400 shrink-0 mt-0.5">
                  <Smartphone class="h-4 w-4" />
                </div>
                <div class="min-w-0 flex-1">
                  <div class="flex items-center gap-2">
                    <span class="text-xs font-semibold truncate">{{ task.scriptName }}</span>
                    <Badge variant="outline" class="text-[10px] px-1 py-0 border-emerald-500 text-emerald-600">
                      PID: {{ task.pid }}
                    </Badge>
                  </div>
                  <div class="text-[11px] text-muted-foreground flex flex-wrap items-center gap-x-3 gap-y-1 mt-1">
                    <span>目标设备: <code class="font-mono">{{ task.deviceSerial }}</code></span>
                    <span>启动时间: {{ task.startedAt }}</span>
                    <span>运行时长: {{ task.elapsedSec }}s / {{ task.timeoutSec }}s</span>
                  </div>
                </div>
              </div>

              <!-- 操作区：结束任务 -->
              <div class="shrink-0 flex items-center gap-2">
                <Button
                  size="sm"
                  variant="destructive"
                  class="h-7 text-xs px-2.5"
                  :disabled="task.status === 'stopping'"
                  @click="killTask(task)"
                >
                  <Square class="h-3 w-3 mr-1 fill-current" />
                  {{ task.status === 'stopping' ? '正在终止...' : '结束进程' }}
                </Button>
              </div>
            </div>
          </div>

          <div v-else class="py-12 border rounded-lg text-center text-muted-foreground text-xs space-y-2">
            <CheckCircle2 class="h-8 w-8 mx-auto text-emerald-500/40" />
            <div>当前没有正在执行的后台任务</div>
            <p class="text-[11px]">所有连接设备均处于空闲就绪状态，可随时发起调度</p>
          </div>
        </div>

        <DialogFooter>
          <Button variant="outline" @click="taskManagerDialogOpen = false">
            关闭
          </Button>
        </DialogFooter>
      </DialogContent>
    </Dialog>
  </div>
</template>
