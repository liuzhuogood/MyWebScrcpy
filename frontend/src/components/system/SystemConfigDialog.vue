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
import DependencyPanel from '@/components/dependencies/DependencyPanel.vue'
import MatchingServicePanel from './MatchingServicePanel.vue'

const props = defineProps<{ initialTab?: 'theme' | 'service' | 'dependencies' | 'matching' }>()

const open = defineModel<boolean>('open', { default: false })
const tab = ref('theme')
const desktopConfig = useDesktopConfig()
const themes = [{ id: 'auto' as const, name: '系统', icon: Monitor }, { id: 'light' as const, name: '明亮', icon: Sun }, { id: 'dark' as const, name: '黑暗', icon: Moon }]
watch(open, (value) => {
  if (value) {
    if (props.initialTab) tab.value = props.initialTab
    void desktopConfig.load()
  }
})
</script>

<template>
  <Dialog v-model:open="open">
    <DialogContent class="flex max-h-[90dvh] flex-col gap-0 overflow-hidden p-0 sm:max-w-2xl" :aria-describedby="undefined">
      <DialogHeader class="shrink-0 border-b px-5 py-4"><DialogTitle>系统配置</DialogTitle></DialogHeader>
      <Tabs v-model="tab" class="flex min-h-0 flex-1 flex-col gap-0">
        <div class="shrink-0 px-5 pt-4"><TabsList class="grid w-full grid-cols-4"><TabsTrigger value="theme">主题</TabsTrigger><TabsTrigger value="service">服务</TabsTrigger><TabsTrigger value="dependencies">依赖</TabsTrigger><TabsTrigger value="matching">匹配</TabsTrigger></TabsList></div>
        <div v-if="tab !== 'dependencies'" class="min-h-0 overflow-y-auto px-5 py-5">
          <TabsContent value="theme" class="m-0 min-h-56 space-y-4">
            <p class="text-sm font-medium">外观</p>
            <div class="grid grid-cols-3 gap-3" role="group" aria-label="主题">
              <Button v-for="item in themes" :key="item.id" variant="outline" class="h-24 flex-col gap-3" :class="theme === item.id ? 'border-primary bg-primary/5 text-primary' : ''" :aria-pressed="theme === item.id" @click="theme = item.id"><component :is="item.icon" class="size-6" />{{ item.name }}</Button>
            </div>
          </TabsContent>
          <TabsContent value="matching" class="m-0"><MatchingServicePanel v-if="open" /></TabsContent>
          <TabsContent value="service" class="m-0 space-y-4">
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
          </TabsContent>
        </div>
        <TabsContent value="dependencies" force-mount v-show="tab === 'dependencies'" class="m-0 flex min-h-0 flex-1 flex-col overflow-hidden">
          <DependencyPanel :active="open && tab === 'dependencies'" class="flex-1">
            <template #actions><Button variant="ghost" size="sm" @click="open = false">关闭</Button></template>
          </DependencyPanel>
        </TabsContent>
      </Tabs>
      <footer v-if="tab !== 'dependencies'" class="flex shrink-0 items-center gap-3 border-t px-5 py-3">
        <p v-if="tab === 'service' && desktopConfig.error.value" role="alert" class="flex-1 text-xs text-destructive">{{ desktopConfig.error.value }}</p>
        <p v-else-if="tab === 'service' && desktopConfig.message.value" role="status" class="flex-1 text-xs text-muted-foreground">{{ desktopConfig.message.value }}</p>
        <div class="ml-auto flex items-center gap-2">
          <Button variant="ghost" size="sm" @click="open = false">关闭</Button>
          <Button v-if="tab === 'service'" size="sm" :disabled="desktopConfig.saving.value || desktopConfig.config.value?.is_overridden" @click="desktopConfig.save">
            <Loader2 v-if="desktopConfig.saving.value" class="size-3.5 animate-spin" />保存
          </Button>
        </div>
      </footer>
    </DialogContent>
  </Dialog>
</template>
