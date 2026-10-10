<script setup lang="ts">
import { Download, Globe, ImagePlus, Minus, Plus, RefreshCw, Smartphone, Trash2, Upload } from 'lucide-vue-next'
import { ref } from 'vue'
import { toast } from 'vue-sonner'
import {
  AlertDialog,
  AlertDialogAction,
  AlertDialogCancel,
  AlertDialogContent,
  AlertDialogDescription,
  AlertDialogFooter,
  AlertDialogHeader,
  AlertDialogTitle,
} from '@/components/ui/alert-dialog'
import { Badge } from '@/components/ui/badge'
import { Button } from '@/components/ui/button'
import { Card, CardContent } from '@/components/ui/card'
import { Empty, EmptyDescription, EmptyHeader, EmptyTitle } from '@/components/ui/empty'
import { Skeleton } from '@/components/ui/skeleton'
import { Switch } from '@/components/ui/switch'
import SidePanelHeader from '@/components/layout/SidePanelHeader.vue'
import { exportUrl, templateImageDownloadUrl, templateImageUrl, type TemplateItem } from '@/api/templates'
import { useTemplates } from './useTemplates'

const props = defineProps<{ serial: string }>()
const emit = defineEmits<{ close: [] }>()

const t = useTemplates(props.serial)
const deleteTarget = ref<TemplateItem | null>(null)
// 删除 id 单独存放：Action 点击会先关弹窗（清空 deleteTarget）再调 onDelete
const deleteId = ref<string | null>(null)
const actionMsg = ref('')
const actionMsgError = ref(false)

function note(msg: string, isError: boolean) {
  actionMsg.value = msg
  actionMsgError.value = isError
  if (!isError)
    toast.success(msg)
  else toast.error(msg)
}
const uploadInput = ref<HTMLInputElement | null>(null)
const importInput = ref<HTMLInputElement | null>(null)
const addImageInput = ref<HTMLInputElement | null>(null)
const addImageTarget = ref<string | null>(null)

function targetSerial(): string {
  return t.scope.value === 'global' ? 'global' : props.serial
}

async function stepThreshold(item: TemplateItem, delta: number) {
  const v = Math.min(0.99, Math.max(0.05, Math.round(((item.threshold ?? 0.8) + delta) * 100) / 100))
  try {
    await t.saveThreshold(item, v)
    note(`阈值已更新为 ${v}`, false)
  }
  catch (e) {
    note(e instanceof Error ? e.message : '更新失败', true)
  }
}

async function onUploadPicked(e: Event) {
  const file = (e.target as HTMLInputElement).files?.[0]
  if (!file)
    return
  try {
    await t.uploadSingle(file, targetSerial())
    toast.success('模板已上传')
  }
  catch (err) {
    toast.error(err instanceof Error ? err.message : '上传失败')
  }
  finally {
    if (uploadInput.value)
      uploadInput.value.value = ''
  }
}

async function onImportPicked(e: Event) {
  const file = (e.target as HTMLInputElement).files?.[0]
  if (!file)
    return
  try {
    const n = await t.importZip(file, targetSerial())
    toast.success(`成功导入 ${n} 个模板`)
  }
  catch (err) {
    toast.error(err instanceof Error ? err.message : '导入失败')
  }
  finally {
    if (importInput.value)
      importInput.value.value = ''
  }
}

function pickAddImage(id: string) {
  addImageTarget.value = id
  addImageInput.value?.click()
}

async function onAddImagePicked(e: Event) {
  const file = (e.target as HTMLInputElement).files?.[0]
  const id = addImageTarget.value
  if (!file || !id)
    return
  try {
    await t.addImage(id, file)
    toast.success('参考图已添加')
  }
  catch {
    toast.error('添加参考图失败，请重试')
  }
  finally {
    if (addImageInput.value)
      addImageInput.value.value = ''
    addImageTarget.value = null
  }
}

async function onDelete() {
  const id = deleteId.value
  deleteId.value = null
  if (!id)
    return
  try {
    await t.remove(id)
    note('模板已删除', false)
  }
  catch (e) {
    note(e instanceof Error ? e.message : '删除失败', true)
  }
  finally {
    deleteTarget.value = null
  }
}

function askDelete(item: TemplateItem) {
  deleteTarget.value = item
  deleteId.value = item.id
}

void t.load()
</script>

<template>
  <section class="flex h-full min-h-0 flex-col overflow-hidden bg-card" aria-label="模板管理">
    <SidePanelHeader title="模板管理" @close="emit('close')">
      <template #actions>
        <Button size="sm" variant="ghost" class="h-7 px-2" aria-label="刷新模板列表" @click="t.load()">
          <RefreshCw class="h-3.5 w-3.5" />
        </Button>
      </template>
    </SidePanelHeader>

    <div class="flex min-h-0 flex-1 flex-col gap-3 overflow-hidden p-3">
      <div class="flex flex-wrap items-center gap-1.5">
        <div class="flex gap-1" role="group" aria-label="模板范围筛选">
          <Button size="sm" :variant="t.scope.value === 'all' ? 'secondary' : 'ghost'" @click="t.scope.value = 'all'">全部</Button>
          <Button size="sm" :variant="t.scope.value === 'device' ? 'secondary' : 'ghost'" @click="t.scope.value = 'device'">设备</Button>
          <Button size="sm" :variant="t.scope.value === 'global' ? 'secondary' : 'ghost'" @click="t.scope.value = 'global'">全局</Button>
        </div>
        <div class="ml-auto flex gap-1.5">
          <Button size="sm" variant="outline" @click="uploadInput?.click()"><Upload class="h-3.5 w-3.5" />上传单张</Button>
          <Button size="sm" variant="outline" as-child>
            <a :href="exportUrl(serial, t.scope.value)" target="_blank" rel="noopener"><Download class="h-3.5 w-3.5" />导出 ZIP</a>
          </Button>
          <Button size="sm" variant="outline" @click="importInput?.click()"><Plus class="h-3.5 w-3.5" />导入 ZIP</Button>
        </div>
        <input ref="uploadInput" type="file" accept="image/png,image/jpeg" class="hidden" @change="onUploadPicked" />
        <input ref="importInput" type="file" accept=".zip" class="hidden" @change="onImportPicked" />
        <input ref="addImageInput" type="file" accept="image/png,image/jpeg" class="hidden" @change="onAddImagePicked" />
      </div>

    <div class="min-h-0 flex-1 overflow-auto">
      <p v-if="actionMsg" class="mb-2 text-xs" :class="actionMsgError ? 'text-destructive' : 'text-green-700 dark:text-green-400'">{{ actionMsg }}</p>
      <div v-if="t.loading.value" class="grid gap-3 sm:grid-cols-2">
        <Skeleton v-for="i in 4" :key="i" class="h-28 rounded-xl" />
      </div>
      <Empty v-else-if="t.error.value">
        <EmptyHeader>
          <EmptyTitle>模板加载失败</EmptyTitle>
          <EmptyDescription>{{ t.error.value }}</EmptyDescription>
        </EmptyHeader>
        <Button size="sm" variant="outline" @click="t.load()">重试</Button>
      </Empty>
      <Empty v-else-if="t.filtered.value.length === 0">
        <EmptyHeader>
          <EmptyTitle>暂无模板</EmptyTitle>
          <EmptyDescription>可上传图片，或从截图选区存为模板。</EmptyDescription>
        </EmptyHeader>
      </Empty>
      <div v-else class="grid gap-3 sm:grid-cols-2">
        <Card v-for="item in t.filtered.value" :key="item.id" :class="{ 'opacity-60': !item.enabled }">
          <CardContent class="flex gap-3 p-3">
            <img
              :src="templateImageUrl(item.id, item.updated_at)" :alt="item.name"
              class="h-16 w-16 shrink-0 rounded-md border object-contain" loading="lazy"
            />
            <div class="min-w-0 flex-1">
              <div class="flex items-center gap-1.5">
                <p class="truncate text-sm font-medium" :title="item.name">{{ item.name }}</p>
                <Badge variant="secondary" class="shrink-0 text-[10px]">{{ item.serial === 'global' ? '全局' : '设备' }}</Badge>
              </div>
              <p class="mt-0.5 font-mono text-[11px] text-muted-foreground">
                {{ item.width || 0 }}×{{ item.height || 0 }} · {{ item.image_count || 1 }} 图{{ item.scales?.length ? ` · 尺度${item.scales.join(',')}` : '' }}
              </p>
              <div class="mt-1.5 flex items-center gap-1.5">
                <Switch
                  :model-value="item.enabled" :disabled="t.busyId.value === item.id"
                  :aria-label="`${item.name} 启用开关`" @update:model-value="t.toggleEnabled(item)"
                />
                <div class="flex items-center gap-0.5" role="group" :aria-label="`${item.name} 匹配阈值`">
                  <Button variant="ghost" size="icon" class="h-7 w-6" aria-label="降低阈值" @click="stepThreshold(item, -0.05)">
                    <Minus class="h-3 w-3" />
                  </Button>
                  <span class="w-9 text-center font-mono text-xs" :title="`当前阈值 ${item.threshold}`">{{ item.threshold }}</span>
                  <Button variant="ghost" size="icon" class="h-7 w-6" aria-label="提高阈值" @click="stepThreshold(item, 0.05)">
                    <Plus class="h-3 w-3" />
                  </Button>
                </div>
                <Button
                  variant="ghost" size="icon" class="h-7 w-7"
                  :title="item.serial === 'global' ? '转为当前设备模板' : '转为全局模板'"
                  :aria-label="item.serial === 'global' ? '转为当前设备模板' : '转为全局模板'"
                  @click="t.switchScope(item)"
                >
                  <Globe v-if="item.serial !== 'global'" class="h-3.5 w-3.5" />
                  <Smartphone v-else class="h-3.5 w-3.5" />
                </Button>
                <Button variant="ghost" size="icon" class="h-7 w-7" title="添加参考图" :aria-label="`为${item.name}添加参考图`" @click="pickAddImage(item.id)">
                  <ImagePlus class="h-3.5 w-3.5" />
                </Button>
                <Button variant="ghost" size="icon" class="h-7 w-7" title="下载原图" :aria-label="`下载${item.name}原图`" as-child>
                  <a :href="templateImageDownloadUrl(item.id)" :download="`${item.name}.png`"><Download class="h-3.5 w-3.5" /></a>
                </Button>
                <Button
                  variant="ghost" size="icon" class="h-7 w-7 text-destructive" title="删除模板" :aria-label="`删除模板${item.name}`"
                  @click="askDelete(item)"
                >
                  <Trash2 class="h-3.5 w-3.5" />
                </Button>
              </div>
            </div>
          </CardContent>
        </Card>
      </div>
    </div>
    </div>

    <AlertDialog :open="deleteTarget !== null" @update:open="(v) => { if (!v) deleteTarget = null }">
      <AlertDialogContent>
        <AlertDialogHeader>
          <AlertDialogTitle>删除模板「{{ deleteTarget?.name }}」？</AlertDialogTitle>
          <AlertDialogDescription>关联磁盘文件一并删除，不可撤销。</AlertDialogDescription>
        </AlertDialogHeader>
        <AlertDialogFooter>
          <AlertDialogCancel>取消</AlertDialogCancel>
          <AlertDialogAction class="bg-destructive text-destructive-foreground hover:bg-destructive/90" @click="onDelete">删除</AlertDialogAction>
        </AlertDialogFooter>
      </AlertDialogContent>
    </AlertDialog>
  </section>
</template>
