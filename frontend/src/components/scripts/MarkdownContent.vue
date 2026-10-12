<script setup lang="ts">
import { computed } from 'vue'
import type { TemplateItem } from '@/api/templates'
import { renderDocument, renderMarkdown } from '@/scripts/document'

const props = withDefaults(defineProps<{ content: string, assets?: TemplateItem[], document?: boolean }>(), {
  assets: () => [],
  document: false,
})
const html = computed(() => props.document ? renderDocument(props.content, props.assets) : renderMarkdown(props.content, props.assets))
</script>
<template>
  <article class="markdown-content" v-html="html" />
</template>
<style scoped>
.markdown-content { font-size: 14px; line-height: 1.8; overflow-wrap: anywhere; min-width: 0; }
.markdown-content :deep(h1) { font-size: 24px; font-weight: 600; margin: 0 0 24px; }
.markdown-content :deep(h2) { font-size: 18px; font-weight: 600; margin: 24px 0 12px; }
.markdown-content :deep(h3), .markdown-content :deep(h4), .markdown-content :deep(h5), .markdown-content :deep(h6) { font-weight: 600; margin: 20px 0 8px; }
.markdown-content :deep(p) { margin: 12px 0; }
.markdown-content :deep(ul) { list-style: disc; padding-left: 24px; }
.markdown-content :deep(ol) { list-style: decimal; padding-left: 24px; }
.markdown-content :deep(li) { margin: 8px 0; }
.markdown-content :deep(img) { max-width: 100%; max-height: 240px; object-fit: contain; border: 1px solid var(--border); border-radius: 8px; margin: 12px 0; }
.markdown-content :deep(pre) { white-space: pre; overflow-x: auto; background: var(--muted); padding: 16px; border-radius: 8px; }
.markdown-content :deep(pre code) { padding: 0; background: transparent; }
.markdown-content :deep(code) { font-size: 12px; background: var(--muted); padding: 2px 4px; border-radius: 3px; }
.markdown-content :deep(table) { display: block; max-width: 100%; overflow-x: auto; border-collapse: collapse; margin: 16px 0; }
.markdown-content :deep(th), .markdown-content :deep(td) { border: 1px solid var(--border); padding: 6px 12px; }
.markdown-content :deep(blockquote) { border-left: 3px solid var(--border); padding-left: 16px; color: var(--muted-foreground); }
.markdown-content :deep(a) { color: var(--primary); text-decoration: underline; }
.markdown-content :deep(.template-unavailable) { color: var(--destructive); }
.markdown-content :deep(> :first-child) { margin-top: 0; }
.markdown-content :deep(> :last-child) { margin-bottom: 0; }
</style>
