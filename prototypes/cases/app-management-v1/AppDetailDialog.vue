<script setup lang="ts">
import { computed } from 'vue'
import {
  Calendar,
  Cpu,
  Download,
  FolderOpen,
  HardDrive,
  Layers,
  Play,
  RotateCcw,
  Shield,
  Square,
  Trash2,
} from 'lucide-vue-next'
import { Badge } from '@/components/ui/badge'
import { Button } from '@/components/ui/button'
import {
  Dialog,
  DialogContent,
  DialogDescription,
  DialogFooter,
  DialogHeader,
  DialogTitle,
} from '@/components/ui/dialog'

import type { AppItem } from './types'

const props = defineProps<{
  open: boolean
  app: AppItem | null
}>()

const emit = defineEmits<{
  'update:open': [value: boolean]
  'launch': [packageName: string]
  'stop': [packageName: string]
  'clear': [packageName: string]
  'export': [packageName: string]
  'toggle-status': [packageName: string]
  'uninstall': [app: AppItem]
}>()

const formattedSize = computed(() => {
  if (!props.app) return '0 B'
  const mb = props.app.sizeBytes / (1024 * 1024)
  if (mb >= 1) return `${mb.toFixed(1)} MB`
  return `${(props.app.sizeBytes / 1024).toFixed(1)} KB`
})
</script>

<template>
  <Dialog :open="open" @update:open="$emit('update:open', $event)">
    <DialogContent class="sm:max-w-lg">
      <DialogHeader>
        <div v-if="app" class="flex items-start gap-3">
          <div
            class="flex size-12 shrink-0 items-center justify-center rounded-xl text-white font-semibold text-lg shadow-sm"
            :class="app.iconColor"
          >
            {{ app.name.slice(0, 1) }}
          </div>
          <div class="min-w-0 flex-1 space-y-1">
            <div class="flex items-center gap-2">
              <DialogTitle class="text-base truncate">{{ app.name }}</DialogTitle>
              <Badge :variant="app.type === 'user' ? 'default' : 'secondary'" class="text-xs shrink-0">
                {{ app.type === 'user' ? '用户应用' : '系统内置' }}
              </Badge>
              <Badge :variant="app.status === 'enabled' ? 'outline' : 'destructive'" class="text-xs shrink-0">
                {{ app.status === 'enabled' ? '已启用' : '已停用' }}
              </Badge>
            </div>
            <DialogDescription class="font-mono text-xs text-muted-foreground truncate">
              {{ app.packageName }}
            </DialogDescription>
          </div>
        </div>
      </DialogHeader>

      <div v-if="app" class="space-y-4 py-1 text-xs">
        <!-- 基础元数据网格 -->
        <div class="grid grid-cols-2 gap-2 rounded-lg border bg-muted/20 p-3">
          <div class="space-y-0.5">
            <span class="text-muted-foreground flex items-center gap-1">
              <Layers class="size-3.5" />版本信息
            </span>
            <p class="font-medium font-mono text-foreground">v{{ app.versionName }} ({{ app.versionCode }})</p>
          </div>
          <div class="space-y-0.5">
            <span class="text-muted-foreground flex items-center gap-1">
              <HardDrive class="size-3.5" />占用体积
            </span>
            <p class="font-medium text-foreground">{{ formattedSize }}</p>
          </div>
          <div class="space-y-0.5">
            <span class="text-muted-foreground flex items-center gap-1">
              <Cpu class="size-3.5" />SDK 级别
            </span>
            <p class="font-medium text-foreground">Target {{ app.targetSdk }} / Min {{ app.minSdk }}</p>
          </div>
          <div class="space-y-0.5">
            <span class="text-muted-foreground flex items-center gap-1">
              <Calendar class="size-3.5" />最近更新
            </span>
            <p class="font-medium text-foreground">{{ app.updateTime }}</p>
          </div>
        </div>

        <!-- 安装路径 -->
        <div class="space-y-1.5 rounded-lg border p-3">
          <p class="text-muted-foreground font-medium flex items-center gap-1">
            <FolderOpen class="size-3.5" />APK 真实路径
          </p>
          <p class="font-mono text-xs break-all bg-muted/40 p-1.5 rounded text-foreground select-all">
            {{ app.apkPath }}
          </p>
        </div>

        <!-- 权限声明 -->
        <div class="space-y-1.5 rounded-lg border p-3">
          <div class="flex items-center justify-between">
            <p class="text-muted-foreground font-medium flex items-center gap-1">
              <Shield class="size-3.5" />主要权限声明 ({{ app.permissions.length }})
            </p>
          </div>
          <div class="flex flex-wrap gap-1.5 pt-1 max-h-24 overflow-y-auto">
            <span
              v-for="perm in app.permissions"
              :key="perm"
              class="inline-flex items-center rounded bg-muted px-2 py-0.5 font-mono text-[11px] text-muted-foreground"
            >
              {{ perm.replace('android.permission.', '') }}
            </span>
          </div>
        </div>

        <!-- 快捷操作栏 -->
        <div class="space-y-1.5 pt-1">
          <p class="text-muted-foreground font-medium">应用管理操作</p>
          <div class="flex flex-wrap gap-2">
            <Button size="sm" variant="outline" class="h-8 gap-1" @click="emit('launch', app.packageName)">
              <Play class="size-3.5 text-emerald-500" />
              <span>启动</span>
            </Button>
            <Button size="sm" variant="outline" class="h-8 gap-1" @click="emit('stop', app.packageName)">
              <Square class="size-3.5 text-amber-500" />
              <span>强行停止</span>
            </Button>
            <Button size="sm" variant="outline" class="h-8 gap-1" @click="emit('clear', app.packageName)">
              <RotateCcw class="size-3.5" />
              <span>清除数据</span>
            </Button>
            <Button size="sm" variant="outline" class="h-8 gap-1" @click="emit('export', app.packageName)">
              <Download class="size-3.5" />
              <span>导出 APK</span>
            </Button>
            <Button
              size="sm"
              variant="outline"
              class="h-8 gap-1"
              @click="emit('toggle-status', app.packageName)"
            >
              <span>{{ app.status === 'enabled' ? '停用应用' : '启用应用' }}</span>
            </Button>
            <Button
              size="sm"
              variant="destructive"
              class="h-8 gap-1"
              @click="emit('uninstall', app)"
            >
              <Trash2 class="size-3.5" />
              <span>卸载</span>
            </Button>
          </div>
        </div>
      </div>

      <DialogFooter>
        <Button variant="outline" size="sm" @click="$emit('update:open', false)">关闭</Button>
      </DialogFooter>
    </DialogContent>
  </Dialog>
</template>
