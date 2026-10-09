<script setup lang="ts">
import { Pencil, Smartphone } from 'lucide-vue-next'
import { computed, nextTick, ref } from 'vue'
import type { ComponentPublicInstance } from 'vue'
import { useRouter } from 'vue-router'
import { Badge } from '@/components/ui/badge'
import { Button } from '@/components/ui/button'
import { Card, CardContent } from '@/components/ui/card'
import { Input } from '@/components/ui/input'
import type { Device } from '@/api/devices'
import { useDeviceStore } from '@/stores/devices'

const props = defineProps<{ device: Device }>()
const store = useDeviceStore()
const router = useRouter()

const isEditing = computed(() => store.editingSerial === props.device.serial)
const draft = ref('')
// Input 是 shadcn 封装组件：ref 拿到的是组件实例，用 $el 取其根 input 元素
const inputRef = ref<ComponentPublicInstance | null>(null)

async function startEdit(e: Event) {
  e.stopPropagation()
  store.editingSerial = props.device.serial
  draft.value = store.aliases[props.device.serial] || props.device.model || ''
  await nextTick()
  const el = inputRef.value?.$el as HTMLInputElement | undefined
  el?.focus()
  el?.select()
}

function save() {
  store.saveAlias(props.device.serial, draft.value, props.device.model || '未知设备')
  store.editingSerial = null
}

function cancel() {
  store.editingSerial = null
}

function open() {
  if (isEditing.value)
    return
  router.push({ name: 'player', query: { serial: props.device.serial } })
}
</script>

<template>
  <Card class="group cursor-pointer transition-colors hover:border-primary/50" @click="open">
    <CardContent class="flex items-start gap-3 p-4">
      <span class="flex h-10 w-10 shrink-0 items-center justify-center rounded-lg bg-muted">
        <Smartphone class="h-5 w-5 text-muted-foreground" />
      </span>
      <div class="min-w-0 flex-1">
        <template v-if="!isEditing">
          <div class="flex items-center gap-2">
            <p class="truncate font-medium">{{ store.displayName(device) }}</p>
            <Badge v-if="store.aliases[device.serial]" variant="secondary">{{ device.model || '未知设备' }}</Badge>
          </div>
          <p class="mt-1 truncate font-mono text-xs text-muted-foreground">
            {{ device.serial }}{{ device.product ? ` · ${device.product}` : '' }}
          </p>
        </template>
        <template v-else>
          <div class="flex gap-1.5" @click.stop>
            <Input
              ref="inputRef"
              v-model="draft"
              maxlength="20"
              placeholder="输入设备名称"
              aria-label="设备名称"
              class="h-8"
              @keydown.enter.prevent="save"
              @keydown.esc.prevent="cancel"
            />
            <Button size="sm" @click="save">保存</Button>
            <Button size="sm" variant="ghost" @click="cancel">取消</Button>
          </div>
        </template>
      </div>
      <Button
        v-if="!isEditing"
        variant="ghost"
        size="icon"
        class="h-8 w-8 opacity-0 group-hover:opacity-100 focus-visible:opacity-100"
        :aria-label="`重命名 ${store.displayName(device)}`"
        @click="startEdit"
      >
        <Pencil class="h-4 w-4" />
      </Button>
    </CardContent>
  </Card>
</template>
