<script setup lang="ts">
import { Bot, Send, Settings2 } from 'lucide-vue-next'
import { Button } from '@/components/ui/button'
import { Input } from '@/components/ui/input'
import MarkdownContent from './MarkdownContent.vue'
import type { ScriptsWorkbench } from '@/scripts/useScriptsWorkbench'
defineProps<{ w: ScriptsWorkbench }>()
</script>
<template>
  <aside aria-label="AI助手" class="script-pane flex min-w-0 flex-col bg-muted/10 lg:border-l" :class="w.mobilePanel === 'assistant' ? '' : 'hidden lg:flex'">
    <div class="flex items-center gap-2 border-b px-4 py-3"><div class="flex size-7 items-center justify-center rounded-lg bg-primary/10"><Bot class="size-4 text-primary" /></div><h2 class="flex-1 text-sm font-semibold">AI助手</h2><Button variant="ghost" size="icon-sm" aria-label="模型配置" @click="w.modelConfigOpen = true"><Settings2 class="size-3.5" /></Button></div>
    <div aria-live="polite" class="min-h-0 flex-1 space-y-5 overflow-auto p-4"><div v-for="(message, index) in w.conversation.messages" :key="index" :class="message.role === 'user' ? 'ml-6 rounded-lg bg-primary/10 p-3' : 'mr-2'" class="min-w-0 text-sm leading-6 break-words"><span v-if="message.role === 'assistant'" class="mb-1 block text-[10px] font-medium text-muted-foreground">AI助手</span><MarkdownContent v-if="message.role === 'assistant'" :content="message.content" :assets="w.assets" /><p v-else class="whitespace-pre-wrap">{{ message.content }}</p></div><template v-if="w.conversation.proposal"><MarkdownContent :content="w.conversation.proposal.content" :assets="w.assets" document class="max-h-80 overflow-auto rounded-lg border bg-muted/30 p-3" /><p v-if="w.content !== w.conversation.proposal.base" class="text-xs text-muted-foreground">文档已有新的修改，请重新生成建议。</p><Button size="sm" variant="outline" class="w-full" :disabled="w.content !== w.conversation.proposal.base" @click="w.applyProposal">应用到文档</Button></template><p v-if="w.conversation.busy" role="status" class="text-xs text-muted-foreground">正在生成建议…</p></div>
    <form class="space-y-2 border-t p-3" @submit.prevent="w.sendPrompt"><label for="assistant-prompt" class="sr-only">文档修改要求</label><Input id="assistant-prompt" v-model="w.conversation.prompt" :disabled="!w.current" placeholder="描述要调整的步骤…" class="text-xs" /><Button type="submit" size="sm" class="w-full text-xs" :disabled="!w.current || !w.conversation.prompt.trim() || w.conversation.busy"><Send class="size-3.5" />发送</Button></form>
  </aside>
</template>
