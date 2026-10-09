<script setup lang="ts">
import { ChevronDown, ChevronRight } from 'lucide-vue-next'
import { useInspector } from './useInspector'

defineProps<{
  node: Element
  depth: number
  collapsed: Set<Element>
  toggle: (node: Element) => void
  summary: (node: Element) => string
}>()

const ins = useInspector()

function childrenOf(node: Element): Element[] {
  return [...node.children]
}
</script>

<template>
  <li>
    <div
      class="flex cursor-pointer items-start gap-1 rounded px-1 py-0.5 hover:bg-accent"
      :class="{ 'bg-accent': ins.selectedNode.value === node }"
      :style="{ paddingLeft: `${depth * 12 + 4}px` }"
      :title="summary(node)"
      @click="ins.selectNode(ins.selectedNode.value === node ? null : node); ins.testXpath()"
    >
      <button
        v-if="childrenOf(node).length" type="button"
        class="shrink-0 rounded p-0.5 hover:bg-muted"
        :aria-expanded="!collapsed.has(node)"
        :aria-label="collapsed.has(node) ? '展开子节点' : '收起子节点'"
        @click.stop="toggle(node)"
      >
        <ChevronRight v-if="collapsed.has(node)" class="h-3 w-3" />
        <ChevronDown v-else class="h-3 w-3" />
      </button>
      <span v-else class="w-4 shrink-0" />
      <span class="break-all">
        <span class="text-primary">{{ node.tagName }}</span>
        <template v-for="name in ['class', 'resource-id', 'text', 'bounds']" :key="name">
          <span v-if="node.getAttribute(name)" class="ml-1">
            <span class="text-muted-foreground">{{ name }}=</span><span class="text-green-700 dark:text-green-400">{{ JSON.stringify(node.getAttribute(name)) }}</span>
          </span>
        </template>
      </span>
    </div>
    <ul v-if="childrenOf(node).length && !collapsed.has(node)">
      <XmlTreeNode
        v-for="(child, i) in childrenOf(node)" :key="`${child.tagName}-${i}`"
        :node="child" :depth="depth + 1"
        :collapsed="collapsed" :toggle="toggle" :summary="summary"
      />
    </ul>
  </li>
</template>
