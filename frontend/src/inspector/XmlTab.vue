<script setup lang="ts">
import { ref } from 'vue'
import { Button } from '@/components/ui/button'
import { Checkbox } from '@/components/ui/checkbox'
import { Input } from '@/components/ui/input'
import { ScrollArea } from '@/components/ui/scroll-area'
import { useInspector } from './useInspector'
import XmlTreeNode from './XmlTreeNode.vue'

const ins = useInspector()
const collapsed = ref<Set<Element>>(new Set())

function toggle(node: Element) {
  const next = new Set(collapsed.value)
  if (next.has(node))
    next.delete(node)
  else next.add(node)
  collapsed.value = next
}

function summary(node: Element): string {
  const parts = [node.tagName]
  for (const name of ['class', 'resource-id', 'text', 'bounds']) {
    const v = node.getAttribute(name)
    if (v)
      parts.push(`${name}=${v}`)
  }
  return parts.join(' ')
}
</script>

<template>
  <div class="flex h-full flex-col gap-2 p-3">
    <div class="flex flex-wrap gap-1.5">
      <Button size="sm" variant="outline" :disabled="ins.xmlLoading.value" @click="ins.loadXml()">刷新 XML</Button>
      <Button size="sm" variant="outline" :disabled="!ins.xmlDoc.value" @click="ins.toggleShowAll()">
        {{ ins.showAll.value ? '隐藏全部' : '显示全部' }}
      </Button>
      <Button size="sm" variant="ghost" @click="ins.clearXpath()">清除</Button>
    </div>
    <div class="flex gap-1.5">
      <Input
        v-model="ins.xpathText.value"
        placeholder="例如 //*[@resource-id='app:id/login']"
        aria-label="XPath 表达式"
        class="h-8 font-mono text-xs"
        @keydown.enter.prevent="ins.testXpath()"
      />
      <Button size="sm" :disabled="!ins.xmlDoc.value" @click="ins.testXpath()">测试</Button>
    </div>
    <p class="text-xs" :class="ins.xpathError.value ? 'text-destructive' : 'text-muted-foreground'">
      {{ ins.xmlLoading.value ? '正在获取 XML…' : ins.xmlError.value || ins.xpathResult.value }}
    </p>

    <ScrollArea v-if="ins.xmlDoc.value" class="min-h-0 flex-1 rounded-md border">
      <ul class="p-2 font-mono text-xs">
        <XmlTreeNode
          v-for="root in ins.treeRoots.value" :key="root.tagName"
          :node="root" :depth="0"
          :collapsed="collapsed" :toggle="toggle" :summary="summary"
        />
      </ul>
    </ScrollArea>

    <div v-if="ins.selectedTag.value" class="space-y-1 rounded-md border p-2">
      <p class="font-mono text-xs font-semibold">{{ ins.selectedTag.value }}</p>
      <label
        v-for="attr in ins.selectedAttrs.value" :key="attr.name"
        class="flex cursor-pointer items-center gap-1.5 font-mono text-[11px]"
        :class="{ 'opacity-50': attr.disabled }"
      >
        <Checkbox
          :model-value="attr.checked" :disabled="attr.disabled"
          :aria-label="`${attr.name} 参与定位`"
          @update:model-value="ins.toggleAttr(attr.name, $event === true)"
        />
        <span class="text-muted-foreground">{{ attr.name }}=</span>
        <span class="truncate">{{ JSON.stringify(attr.value) }}</span>
      </label>
    </div>
    <p v-if="ins.xmlTime.value" class="text-[11px] text-muted-foreground">{{ ins.xmlTime.value }}</p>
  </div>
</template>
