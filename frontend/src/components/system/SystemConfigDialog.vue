<script setup lang="ts">
import { ref, watch } from 'vue'
import { Loader2, Monitor, Moon, Sun } from 'lucide-vue-next'
import { Badge } from '@/components/ui/badge'
import { Button } from '@/components/ui/button'
import { Input } from '@/components/ui/input'
import { Dialog, DialogContent, DialogHeader, DialogTitle } from '@/components/ui/dialog'
import { Tabs, TabsContent, TabsList, TabsTrigger } from '@/components/ui/tabs'
import { theme } from '@/composables/useTheme'
import { useDesktopConfig } from '@/composables/useDesktopConfig'
import { useServiceConfig } from '@/composables/useServiceConfig'
import DependencyPanel from '@/components/dependencies/DependencyPanel.vue'
import MatchingServicePanel from './MatchingServicePanel.vue'
import OcrServicePanel from './OcrServicePanel.vue'
import PlayerSettingsContent from './PlayerSettingsContent.vue'

const props = defineProps<{ initialTab?: 'general' | 'player' | 'theme' | 'service' | 'dependencies' | 'matching' | 'ocr' }>()

const open = defineModel<boolean>('open', { default: false })
const tab = ref('general')
const desktopConfig = useDesktopConfig()
const serviceConfig = useServiceConfig()
const themes = [{ id: 'auto' as const, name: '系统', icon: Monitor }, { id: 'light' as const, name: '明亮', icon: Sun }, { id: 'dark' as const, name: '黑暗', icon: Moon }]

watch(open, (value) => {
  if (value) {
    if (props.initialTab === 'matching' || props.initialTab === 'ocr') {
      tab.value = 'dependencies'
    }
    else if (props.initialTab) {
      tab.value = props.initialTab === 'theme' ? 'general' : props.initialTab
    }
    void desktopConfig.load()
    void serviceConfig.load()
  }
})

async function handleServiceSave() {
  if (desktopConfig.isDesktop.value && !desktopConfig.config.value?.is_overridden) {
    await desktopConfig.save()
  }
  await serviceConfig.save()
}
</script>

<template>
  <Dialog v-model:open="open">
    <DialogContent class="flex h-[min(720px,90dvh)] flex-col gap-0 overflow-hidden p-0 sm:max-w-2xl" :aria-describedby="undefined">
      <DialogHeader class="shrink-0 border-b px-5 py-4"><DialogTitle>系统设置</DialogTitle></DialogHeader>
      <Tabs v-model="tab" class="flex min-h-0 flex-1 flex-col gap-0">
        <div class="shrink-0 px-5 pt-4">
          <TabsList class="grid h-auto w-full grid-cols-4">
            <TabsTrigger value="general">通用设置</TabsTrigger>
            <TabsTrigger value="player">投屏设置</TabsTrigger>
            <TabsTrigger value="service">服务</TabsTrigger>
            <TabsTrigger value="dependencies">依赖</TabsTrigger>
          </TabsList>
        </div>
        <div v-if="tab !== 'dependencies'" class="min-h-0 flex-1 overflow-y-auto px-5 py-5 [scrollbar-gutter:stable]">
          <TabsContent value="general" class="m-0 min-h-56 space-y-4">
            <p class="text-sm font-medium">外观</p>
            <div class="grid grid-cols-3 gap-3" role="group" aria-label="主题">
              <Button v-for="item in themes" :key="item.id" variant="outline" class="h-24 flex-col gap-3" :class="theme === item.id ? 'border-primary bg-primary/5 text-primary' : ''" :aria-pressed="theme === item.id" @click="theme = item.id"><component :is="item.icon" class="size-6" />{{ item.name }}</Button>
            </div>
          </TabsContent>
          <TabsContent value="player" class="m-0"><PlayerSettingsContent /></TabsContent>
          <TabsContent value="service" class="m-0 space-y-4">
            <!-- 桌面端口 -->
            <div class="space-y-3 rounded-lg border p-4">
              <div class="flex items-center justify-between gap-3">
                <div>
                  <label for="desktop-port" class="text-sm font-medium">服务端口</label>
                  <p class="mt-1 text-xs text-muted-foreground">桌面内部服务与外部调用的本地回环端口（默认 1989）</p>
                </div>
                <Badge variant="outline">{{ desktopConfig.config.value?.effective ? `运行中 · ${desktopConfig.config.value.effective}` : '默认 1989' }}</Badge>
              </div>
              <div class="space-y-2">
                <Input
                  id="desktop-port"
                  v-model="desktopConfig.draftPort.value"
                  type="number"
                  min="1"
                  max="65535"
                  autocomplete="off"
                  :disabled="desktopConfig.saving.value || desktopConfig.config.value?.is_overridden"
                  class="w-40"
                />
                <p v-if="desktopConfig.config.value?.is_overridden" class="text-xs text-muted-foreground">
                  受环境变量覆盖 ({{ desktopConfig.config.value?.env_port }})
                </p>
                <p class="text-xs text-muted-foreground">重启生效</p>
              </div>
            </div>

            <!-- 任务工作目录 -->
            <div class="space-y-3 rounded-lg border p-4">
              <div>
                <label for="task-root-dir" class="text-sm font-medium">任务工作目录</label>
                <p class="mt-1 text-xs text-muted-foreground">外部任务记录与产出的存放根目录（默认 ~/.mywebscrcppy/tasks/）</p>
              </div>
              <Input
                id="task-root-dir"
                v-model="serviceConfig.config.value.task_root_dir"
                placeholder="~/.mywebscrcppy/tasks/"
                autocomplete="off"
                class="w-full font-mono text-xs"
              />
            </div>

            <!-- AI 视觉决策服务 -->
            <div class="space-y-3 rounded-lg border p-4">
              <div>
                <label class="text-sm font-medium">AI 视觉决策服务</label>
                <p class="mt-1 text-xs text-muted-foreground">配置兼容官方视觉协议的 Chat Completions 服务，用于页面状态决策分析</p>
              </div>
              <div class="space-y-3 text-xs">
                <div>
                  <label for="ai-endpoint" class="text-muted-foreground block mb-1">接口完整地址 (Chat Completions)</label>
                  <Input
                    id="ai-endpoint"
                    v-model="serviceConfig.config.value.ai_chat_endpoint"
                    placeholder="https://api.openai.com/v1/chat/completions"
                    class="w-full font-mono text-xs"
                  />
                </div>
                <div>
                  <label for="ai-model" class="text-muted-foreground block mb-1">模型名称</label>
                  <Input
                    id="ai-model"
                    v-model="serviceConfig.config.value.ai_model"
                    placeholder="gpt-4o"
                    class="w-full font-mono text-xs"
                  />
                </div>
                <div>
                  <label for="ai-key" class="text-muted-foreground block mb-1">API 密钥</label>
                  <Input
                    id="ai-key"
                    v-model="serviceConfig.apiKeyDraft.value"
                    type="password"
                    :placeholder="serviceConfig.config.value.ai_has_key ? `${serviceConfig.config.value.ai_api_key_masked} (留空保持原密钥)` : '输入 API Key'"
                    class="w-full font-mono text-xs"
                  />
                </div>
                <div class="w-36">
                  <label for="ai-timeout" class="text-muted-foreground block mb-1">请求超时 (秒)</label>
                  <Input
                    id="ai-timeout"
                    v-model="serviceConfig.config.value.ai_timeout_seconds"
                    type="number"
                    min="5"
                    max="120"
                    class="h-8"
                  />
                </div>
              </div>
            </div>
          </TabsContent>
        </div>
        <TabsContent value="dependencies" force-mount v-show="tab === 'dependencies'" class="m-0 flex min-h-0 flex-1 flex-col overflow-hidden">
          <DependencyPanel :active="open && tab === 'dependencies'" class="flex-1">
            <MatchingServicePanel v-if="open && tab === 'dependencies'" />
            <OcrServicePanel v-if="open && tab === 'dependencies'" />
            <template #actions><Button variant="ghost" size="sm" @click="open = false">关闭</Button></template>
          </DependencyPanel>
        </TabsContent>
      </Tabs>
      <footer v-if="tab !== 'dependencies'" class="flex shrink-0 items-center gap-3 border-t px-5 py-3">
        <p v-if="tab === 'service' && (desktopConfig.error.value || serviceConfig.error.value)" role="alert" class="flex-1 text-xs text-destructive">
          {{ desktopConfig.error.value || serviceConfig.error.value }}
        </p>
        <p v-else-if="tab === 'service' && (desktopConfig.message.value || serviceConfig.message.value)" role="status" class="flex-1 text-xs text-muted-foreground">
          {{ desktopConfig.message.value || serviceConfig.message.value }}
        </p>
        <div class="ml-auto flex items-center gap-2">
          <Button variant="ghost" size="sm" @click="open = false">关闭</Button>
          <Button
            v-if="tab === 'service'"
            size="sm"
            :disabled="desktopConfig.saving.value || serviceConfig.saving.value"
            @click="handleServiceSave"
          >
            <Loader2 v-if="desktopConfig.saving.value || serviceConfig.saving.value" class="size-3.5 animate-spin mr-1" />
            保存
          </Button>
        </div>
      </footer>
    </DialogContent>
  </Dialog>
</template>
