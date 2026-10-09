<script setup lang="ts">
import { Check, Copy } from 'lucide-vue-next'
import { ref } from 'vue'
import { Button } from '@/components/ui/button'
import { useInspector } from './useInspector'

const ins = useInspector()
const copied = ref<string | null>(null)

async function copy(label: string, value: string) {
  try {
    if (navigator.clipboard && window.isSecureContext)
      await navigator.clipboard.writeText(value)
    else
      throw new Error('clipboard 不可用')
    copied.value = label
    setTimeout(() => {
      if (copied.value === label)
        copied.value = null
    }, 1200)
  }
  catch {
    // 降级：textarea + execCommand
    const area = document.createElement('textarea')
    area.value = value
    area.style.position = 'fixed'
    area.style.opacity = '0'
    document.body.append(area)
    area.select()
    try {
      if (document.execCommand('copy')) {
        copied.value = label
        setTimeout(() => {
          if (copied.value === label)
            copied.value = null
        }, 1200)
      }
    }
    catch { /* 忽略 */ }
    area.remove()
  }
}

function rows(): { label: string, value: string }[] {
  const d = ins.pageInfo.value
  if (!d)
    return []
  const list = [
    { label: '包名', value: d.package_name || '—' },
    { label: 'Activity', value: d.activity || '—' },
    { label: '组件', value: d.component || '—' },
    { label: '窗口', value: d.window_name || '—' },
  ]
  if (d.display) {
    list.push({ label: '分辨率', value: `${d.display.width} × ${d.display.height}` })
    if (Number.isFinite(d.display.density_dpi) && (d.display.density_dpi ?? 0) > 0)
      list.push({ label: 'DPI', value: String(d.display.density_dpi) })
    list.push({ label: '方向', value: `${d.display.rotation ?? '—'}°` })
  }
  if (d.fetched_at)
    list.push({ label: '获取时间', value: new Date(d.fetched_at).toLocaleString() })
  return list
}
</script>

<template>
  <div class="space-y-2 p-3">
    <Button size="sm" variant="outline" :disabled="ins.pageLoading.value" @click="ins.loadPageInfo()">刷新信息</Button>
    <p v-if="ins.pageLoading.value" class="text-xs text-muted-foreground">正在获取页面信息…</p>
    <p v-else-if="ins.pageError.value" class="text-xs text-destructive">{{ ins.pageError.value }}</p>
    <div v-else-if="ins.pageInfo.value" class="space-y-1.5">
      <div v-for="row in rows()" :key="row.label" class="rounded-md border p-2">
        <p class="text-[11px] text-muted-foreground">{{ row.label }}</p>
        <div class="flex items-start gap-1.5">
          <p class="min-w-0 flex-1 break-all font-mono text-xs">{{ row.value }}</p>
          <Button
            variant="ghost" size="icon" class="h-6 w-6 shrink-0"
            :aria-label="`复制${row.label}`" @click="copy(row.label, row.value)"
          >
            <Check v-if="copied === row.label" class="h-3.5 w-3.5 text-green-600" />
            <Copy v-else class="h-3.5 w-3.5" />
          </Button>
        </div>
      </div>
    </div>
    <p v-else class="text-xs text-muted-foreground">尚未获取</p>
  </div>
</template>
