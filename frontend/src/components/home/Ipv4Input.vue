<script setup lang="ts">
import { Input } from '@/components/ui/input'
const props = defineProps<{ modelValue: string[], label: string, disabled?: boolean }>()
const emit = defineEmits<{ 'update:modelValue': [value: string[]] }>()
function update(index: number, value: string | number) {
  const next = [...props.modelValue]
  next[index] = String(value).replace(/\D/g, '').slice(0, 3)
  emit('update:modelValue', next)
}
</script>
<template>
  <div class="flex items-center gap-1.5">
    <span class="w-10 shrink-0 text-sm">{{ label }}</span>
    <template v-for="(_, index) in 4" :key="index">
      <span v-if="index" aria-hidden="true">.</span>
      <Input :model-value="modelValue[index]" :aria-label="`${label} IP 第 ${index + 1} 段`" inputmode="numeric" maxlength="3" class="min-w-0 px-1 text-center font-mono" :disabled="disabled" @update:model-value="update(index, $event)" />
    </template>
  </div>
</template>
