<script setup lang="ts">
import { ref } from 'vue'
import {
  AlertCircle,
  EyeOff,
  Globe,
  Keyboard,
  Laptop,
  RotateCcw,
  Shield,
  Volume2,
  VolumeX,
} from 'lucide-vue-next'
import { Badge } from '@/components/ui/badge'
import { Button } from '@/components/ui/button'
import { Switch } from '@/components/ui/switch'
import {
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from '@/components/ui/select'
import { useShortcuts, type ShortcutCategory } from './useShortcuts'

const props = defineProps<{
  shortcutState: ReturnType<typeof useShortcuts>
}>()

const s = props.shortcutState
const activeCategory = ref<ShortcutCategory | 'all'>('all')

// 模拟按键录制交互（预设常用键位供快速体验）
const simulatedPresets = [
  { label: 'Ctrl + Alt + B', mac: '⌃ ⌥ B' },
  { label: 'Alt + Q', mac: '⌥ Q' },
  { label: 'Alt + Shift + X', mac: '⌥ ⇧ X' },
  { label: 'Ctrl + W (冲突测试)', mac: '⌃ W (冲突测试)' },
]
</script>

<template>
  <div class="space-y-5 text-foreground">
    <!-- 环境与操作系统视图切换条 -->
    <div class="flex flex-wrap items-center justify-between gap-3 rounded-lg border bg-muted/30 p-3">
      <div class="flex items-center gap-2">
        <span class="text-xs font-medium text-muted-foreground">当前运行环境：</span>
        <div class="flex items-center rounded-md border bg-background p-0.5">
          <button
            type="button"
            class="flex items-center gap-1.5 rounded px-2.5 py-1 text-xs font-medium transition-colors"
            :class="s.currentEnv.value === 'desktop' ? 'bg-primary text-primary-foreground shadow-xs' : 'text-muted-foreground hover:text-foreground'"
            @click="s.currentEnv.value = 'desktop'"
          >
            <Laptop class="size-3.5" />
            桌面客户端 (GUI)
          </button>
          <button
            type="button"
            class="flex items-center gap-1.5 rounded px-2.5 py-1 text-xs font-medium transition-colors"
            :class="s.currentEnv.value === 'web' ? 'bg-primary text-primary-foreground shadow-xs' : 'text-muted-foreground hover:text-foreground'"
            @click="s.currentEnv.value = 'web'"
          >
            <Globe class="size-3.5" />
            Web 网页端
          </button>
        </div>
      </div>

      <div class="flex items-center gap-2">
        <span class="text-xs font-medium text-muted-foreground">按键符号：</span>
        <div class="flex items-center rounded-md border bg-background p-0.5">
          <button
            type="button"
            class="rounded px-2 py-0.5 text-xs font-medium transition-colors"
            :class="s.currentOs.value === 'mac' ? 'bg-secondary text-secondary-foreground font-semibold' : 'text-muted-foreground hover:text-foreground'"
            @click="s.currentOs.value = 'mac'"
          >
            macOS (⌥ ⌘)
          </button>
          <button
            type="button"
            class="rounded px-2 py-0.5 text-xs font-medium transition-colors"
            :class="s.currentOs.value === 'windows' ? 'bg-secondary text-secondary-foreground font-semibold' : 'text-muted-foreground hover:text-foreground'"
            @click="s.currentOs.value = 'windows'"
          >
            Windows (Alt Ctrl)
          </button>
        </div>
      </div>
    </div>

    <!-- 环境特征提示条 -->
    <div
      class="flex items-start gap-3 rounded-lg border p-3.5 text-xs transition-colors"
      :class="s.currentEnv.value === 'desktop' ? 'border-blue-500/20 bg-blue-500/5 text-blue-900 dark:text-blue-200' : 'border-amber-500/20 bg-amber-500/5 text-amber-900 dark:text-amber-200'"
    >
      <div class="mt-0.5 shrink-0">
        <Laptop v-if="s.currentEnv.value === 'desktop'" class="size-4 text-blue-600 dark:text-blue-400" />
        <Globe v-else class="size-4 text-amber-600 dark:text-amber-400" />
      </div>
      <div class="min-w-0 flex-1 space-y-1">
        <div class="flex items-center gap-2 font-medium">
          <span v-if="s.currentEnv.value === 'desktop'">桌面端全局热键机制 (Tauri Native)</span>
          <span v-else>Web 浏览器环境与智能隐藏规则</span>
          <Badge :variant="s.currentEnv.value === 'desktop' ? 'default' : 'outline'" class="h-4.5 px-1.5 text-[10px]">
            {{ s.currentEnv.value === 'desktop' ? '系统全局生效' : '页面作用域 · 已过滤' }}
          </Badge>
        </div>
        <p class="text-[11px] leading-relaxed text-muted-foreground">
          <template v-if="s.currentEnv.value === 'desktop'">
            依托底层系统级注册，即使主程序最小化、处于后台或正在操作其他全屏软件，按下老板键或恢复快捷键也能全局拦截响应。
          </template>
          <template v-else>
            浏览器环境下，桌面专享的系统级窗口穿透恢复等快捷键无实际意义，已自动智能隐藏；核心快捷键已调优为 <code class="rounded bg-muted px-1 font-mono">Alt / Option</code> 组合键，规避浏览器保留快捷键冲突。
          </template>
        </p>
      </div>
    </div>

    <!-- 核心板块 1: 老板键（一键隐匿）独立配置卡片 -->
    <div class="space-y-4 rounded-xl border-2 border-primary/20 bg-gradient-to-b from-primary/[0.03] to-transparent p-4 shadow-xs">
      <div class="flex flex-wrap items-center justify-between gap-3 border-b pb-3">
        <div class="flex items-center gap-2.5">
          <div class="flex size-8 items-center justify-center rounded-lg bg-primary/10 text-primary">
            <Shield class="size-4" />
          </div>
          <div>
            <div class="flex items-center gap-2">
              <h3 class="text-sm font-semibold">老板键 (Boss Key)</h3>
              <Badge variant="secondary" class="h-5 text-[10px]">高优先级</Badge>
            </div>
            <p class="text-xs text-muted-foreground">突发情况一键极速隐匿全部投屏、小窗与操作痕迹</p>
          </div>
        </div>

        <div class="flex items-center gap-2">
          <!-- 录制按钮或当前按键胶囊 -->
          <template v-if="s.recordingId.value === 'boss_key'">
            <div class="flex items-center gap-2 rounded-lg border border-primary bg-primary/10 px-3 py-1.5 animate-pulse">
              <Keyboard class="size-3.5 text-primary" />
              <span class="text-xs font-medium text-primary">请按下键盘组合键...</span>
              <Button size="xs" variant="ghost" class="h-5 px-1.5 text-[10px]" @click="s.stopRecording">取消</Button>
            </div>
          </template>
          <template v-else>
            <button
              type="button"
              class="group flex items-center gap-2 rounded-lg border bg-background px-3 py-1.5 shadow-2xs hover:border-primary transition-colors"
              @click="s.startRecording('boss_key')"
            >
              <span class="text-xs text-muted-foreground group-hover:text-foreground">按键:</span>
              <kbd class="rounded border bg-muted px-2 py-0.5 font-mono text-xs font-semibold shadow-2xs">
                {{ s.getKey(s.shortcuts.value.find(item => item.id === 'boss_key')!) }}
              </kbd>
              <span class="text-[10px] text-muted-foreground underline decoration-dotted group-hover:text-primary">修改</span>
            </button>
          </template>
        </div>
      </div>

      <!-- 录制冲突报警 -->
      <div v-if="s.recordingId.value === 'boss_key' && s.recordedConflictWarning.value" class="flex items-center gap-2 rounded-md border border-destructive/30 bg-destructive/10 p-2 text-xs text-destructive">
        <AlertCircle class="size-3.5 shrink-0" />
        <span>{{ s.recordedConflictWarning.value }}</span>
      </div>

      <!-- 录制模拟体验快捷预设条（供原型评审直接测试） -->
      <div v-if="s.recordingId.value === 'boss_key'" class="flex flex-wrap items-center gap-1.5 rounded-lg border border-dashed p-2 text-xs">
        <span class="text-muted-foreground text-[11px]">快速换绑预设：</span>
        <Button
          v-for="p in simulatedPresets"
          :key="p.label"
          size="xs"
          variant="outline"
          class="h-6 text-[11px]"
          @click="s.updateKey('boss_key', p.label, p.mac)"
        >
          {{ s.currentOs.value === 'mac' ? p.mac : p.label }}
        </Button>
      </div>

      <!-- 老板键行为微调参数 -->
      <div class="grid gap-3 pt-1 sm:grid-cols-2">
        <div class="space-y-1.5">
          <label class="text-xs font-medium text-foreground">触发老板键动作</label>
          <Select v-model="s.bossBehavior.value.mode">
            <SelectTrigger class="h-8 text-xs">
              <SelectValue />
            </SelectTrigger>
            <SelectContent>
              <template v-if="s.currentEnv.value === 'desktop'">
                <SelectItem value="fake_work">一键隐匿主窗口并隐藏所有投屏小窗</SelectItem>
                <SelectItem value="hide_all">极速最小化到系统托盘 (无任务栏常驻)</SelectItem>
                <SelectItem value="minimize_tray">完全退出到后台幽灵运行模式</SelectItem>
              </template>
              <template v-else>
                <SelectItem value="fake_work">一键伪装：切换到工作复盘报表假视图</SelectItem>
                <SelectItem value="hide_all">黑屏待机：切断所有投屏并黑屏静音</SelectItem>
              </template>
            </SelectContent>
          </Select>
        </div>

        <div class="flex items-center justify-between rounded-lg border bg-background/50 p-2.5">
          <div class="space-y-0.5">
            <div class="flex items-center gap-1.5 text-xs font-medium">
              <VolumeX v-if="s.bossBehavior.value.muteAudio" class="size-3.5 text-muted-foreground" />
              <Volume2 v-else class="size-3.5 text-muted-foreground" />
              <span>触发时自动静音</span>
            </div>
            <p class="text-[10px] text-muted-foreground">阻断手机端音频流，避免外放声音暴露</p>
          </div>
          <Switch v-model="s.bossBehavior.value.muteAudio" />
        </div>
      </div>
    </div>

    <!-- 核心板块 2: 快捷键清单列表 -->
    <div class="space-y-3">
      <div class="flex flex-wrap items-center justify-between gap-2">
        <div class="flex items-center gap-2">
          <span class="text-xs font-medium">功能分类：</span>
          <div class="flex items-center gap-1">
            <Button
              size="xs"
              :variant="activeCategory === 'all' ? 'secondary' : 'ghost'"
              class="h-6 text-xs"
              @click="activeCategory = 'all'"
            >
              全部
            </Button>
            <Button
              size="xs"
              :variant="activeCategory === 'privacy' ? 'secondary' : 'ghost'"
              class="h-6 text-xs"
              @click="activeCategory = 'privacy'"
            >
              隐私与全局
            </Button>
            <Button
              size="xs"
              :variant="activeCategory === 'player' ? 'secondary' : 'ghost'"
              class="h-6 text-xs"
              @click="activeCategory = 'player'"
            >
              投屏与控制
            </Button>
            <Button
              size="xs"
              :variant="activeCategory === 'device' ? 'secondary' : 'ghost'"
              class="h-6 text-xs"
              @click="activeCategory = 'device'"
            >
              设备实体键
            </Button>
          </div>
        </div>

        <div class="flex items-center gap-2">
          <span v-if="s.currentEnv.value === 'web'" class="text-[11px] text-muted-foreground flex items-center gap-1">
            <EyeOff class="size-3 text-muted-foreground" />
            <span>已隐藏 1 项桌面端专享快捷键</span>
          </span>
          <Button
            variant="outline"
            size="xs"
            class="h-6 text-xs text-muted-foreground hover:text-foreground"
            @click="s.resetToDefaults"
          >
            <RotateCcw class="size-3 mr-1" />
            重置为默认键位
          </Button>
        </div>
      </div>

      <!-- 快捷键条目卡片 (按当前环境自动过滤掉不支持或无意义项) -->
      <div class="divide-y rounded-xl border bg-card">
        <div
          v-for="item in s.shortcuts.value.filter(sc => (activeCategory === 'all' || sc.category === activeCategory) && sc.supportedEnvs.includes(s.currentEnv.value))"
          :key="item.id"
          class="flex flex-col sm:flex-row sm:items-center justify-between gap-3 p-3.5 hover:bg-muted/30 transition-colors"
        >
          <div class="space-y-1 min-w-0 flex-1">
            <div class="flex items-center gap-2">
              <span class="text-sm font-medium">{{ item.name }}</span>
              <Badge
                v-if="s.currentEnv.value === 'desktop' && item.isGlobalDesktop"
                variant="outline"
                class="h-4.5 px-1.5 text-[10px] text-blue-600 border-blue-500/30 dark:text-blue-400"
              >
                桌面全局
              </Badge>
              <Badge
                v-else
                variant="outline"
                class="h-4.5 px-1.5 text-[10px] text-muted-foreground"
              >
                聚焦生效
              </Badge>
            </div>
            <p class="text-xs text-muted-foreground line-clamp-1">{{ item.description }}</p>
          </div>

          <div class="flex items-center gap-3 shrink-0">
            <!-- 录制状态 -->
            <template v-if="s.recordingId.value === item.id">
              <div class="flex items-center gap-2 rounded-lg border border-primary bg-primary/10 px-2.5 py-1 animate-pulse">
                <Keyboard class="size-3 text-primary" />
                <span class="text-xs text-primary">等待按键...</span>
                <Button size="xs" variant="ghost" class="h-4.5 px-1 text-[10px]" @click="s.stopRecording">取消</Button>
              </div>
            </template>
            <!-- 常态按键显示 -->
            <template v-else>
              <button
                type="button"
                class="group flex items-center gap-1 rounded-md border bg-muted/40 px-2.5 py-1 hover:border-primary hover:bg-background transition-colors"
                :disabled="!item.enabled"
                @click="s.startRecording(item.id)"
              >
                <kbd class="font-mono text-xs font-semibold text-foreground">
                  {{ s.getKey(item) }}
                </kbd>
              </button>
            </template>

            <!-- 独立启用开关 -->
            <Switch
              v-model="item.enabled"
              :aria-label="`启用 ${item.name}`"
            />
          </div>
        </div>
      </div>
    </div>
  </div>
</template>
