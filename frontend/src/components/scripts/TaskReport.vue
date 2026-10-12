<script setup lang="ts">
import { computed } from 'vue'
import { Marked } from 'marked'
import DOMPurify from 'dompurify'
const props = defineProps<{ content: string }>()
const html = computed(() => {
  const markdown = new Marked({ gfm: true, breaks: true })
  markdown.use({ renderer: { image: () => '', html: () => '', link: token => markdown.parseInline(token.text, { async: false }) } })
  return DOMPurify.sanitize(markdown.parse(props.content, { async: false }), { USE_PROFILES: { html: true }, FORBID_TAGS: ['img', 'iframe', 'audio', 'video', 'a'], FORBID_ATTR: ['src', 'srcset', 'href'] })
})
</script>
<template><article class="task-report" v-html="html" /></template>
<style scoped>
.task-report { font-size: 14px; line-height: 1.85; overflow-wrap: anywhere; }
.task-report :deep(h1), .task-report :deep(h2), .task-report :deep(h3) { font-size: 16px; font-weight: 600; margin: 20px 0 8px; }
.task-report :deep(p) { margin: 8px 0; }.task-report :deep(ul) { list-style: disc; padding-left: 22px; }.task-report :deep(ol) { list-style: decimal; padding-left: 22px; }
.task-report :deep(pre) { overflow: auto; background: var(--muted); padding: 12px; }.task-report :deep(table) { display: block; overflow-x: auto; border-collapse: collapse; }.task-report :deep(td), .task-report :deep(th) { border: 1px solid var(--border); padding: 6px 10px; }
</style>
