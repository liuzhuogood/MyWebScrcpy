<script setup lang="ts">
import { nextTick, ref, watch } from 'vue'
import { Bot, Send, Settings2, Square, Loader2 } from 'lucide-vue-next'
import { Button } from '@/components/ui/button'
import { Input } from '@/components/ui/input'
import MarkdownContent from './MarkdownContent.vue'
import AssistantActivities from './AssistantActivities.vue'
import type { ScriptsWorkbench } from '@/scripts/useScriptsWorkbench'
const props = defineProps<{ w: ScriptsWorkbench }>()
const streamBox = ref<HTMLElement>()
const followStream = ref(true)
function onScroll() {
  const box = streamBox.value
  if (box) followStream.value = box.scrollHeight - box.scrollTop - box.clientHeight < 48
}
watch(() => props.w.currentId, () => { followStream.value = true })
watch(() => [props.w.conversation, props.w.generation], async () => {
  if (!followStream.value) return
  await nextTick()
  if (streamBox.value) streamBox.value.scrollTop = streamBox.value.scrollHeight
}, { deep: true })
</script>
<template>
  <aside aria-label="AI助手" class="script-pane flex min-w-0 flex-col bg-muted/10 lg:border-l" :class="w.mobilePanel === 'assistant' ? '' : 'hidden lg:flex'">
    <div class="flex items-center gap-2 border-b px-4 py-3">
      <div class="flex size-7 items-center justify-center rounded-lg bg-primary/10"><Bot class="size-4 text-primary" /></div>
      <h2 class="flex-1 text-sm font-semibold">AI助手</h2>
      <Button variant="ghost" size="icon-sm" aria-label="模型配置" @click="w.modelConfigOpen = true"><Settings2 class="size-3.5" /></Button>
    </div>
    <div ref="streamBox" aria-live="polite" class="min-h-0 flex-1 space-y-5 overflow-auto p-4" @scroll="onScroll">
      <section v-if="w.generation" aria-label="Python 生成进度" class="space-y-3 border-b pb-4">
        <div class="flex items-center gap-2"><p role="status" class="flex-1 text-sm">{{ w.generation.status }}</p><Button v-if="w.generation.busy" size="sm" variant="outline" @click="w.stopGeneration"><Square class="size-3.5" />停止生成</Button></div>
        <AssistantActivities :activities="w.generation.message.activities" />
      </section>
      <div v-for="(message, index) in w.conversation.messages" :key="index" :class="message.role === 'user' ? 'ml-6 rounded-lg bg-primary/10 p-3' : 'mr-2'" class="min-w-0 space-y-3 text-sm leading-6 break-words">
        <AssistantActivities :activities="message.activities" />
        <MarkdownContent v-if="message.role === 'assistant' && message.content" :content="message.content" :assets="w.assets" />
        <p v-else-if="message.content" class="whitespace-pre-wrap">{{ message.content }}</p>
      </div>
      <template v-if="w.conversation.proposal">
        <MarkdownContent :content="w.conversation.proposal.content" :assets="w.assets" document class="max-h-80 overflow-auto rounded-lg border bg-muted/30 p-3" />
        <p v-if="w.content !== w.conversation.proposal.base" class="text-xs text-muted-foreground">文档已有新的修改，请重新生成建议。</p>
        <Button size="sm" variant="outline" class="w-full" :disabled="w.content !== w.conversation.proposal.base" @click="w.applyProposal">应用到文档</Button>
      </template>
      <Loader2 v-if="w.conversation.busy" role="status" aria-label="正在生成建议" class="size-4 animate-spin text-muted-foreground" />
    </div>
    <form class="space-y-2 border-t p-3" @submit.prevent="w.sendPrompt">
      <label for="assistant-prompt" class="sr-only">文档修改要求</label>
      <Input id="assistant-prompt" v-model="w.conversation.prompt" :disabled="!w.current" placeholder="描述要调整的步骤…" class="text-xs" />
      <Button v-if="w.conversation.busy" type="button" size="sm" variant="outline" class="w-full text-xs" @click="w.stopAssistant"><Square class="size-3.5" />停止</Button>
      <Button v-else type="submit" size="sm" class="w-full text-xs" :disabled="!w.current || !w.conversation.prompt.trim()"><Send class="size-3.5" />发送</Button>
    </form>
  </aside>
</template>
