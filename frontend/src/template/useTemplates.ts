import { computed, ref } from 'vue'
import {
  addTemplateImage,
  createTemplate,
  deleteTemplate,
  importTemplates,
  listTemplates,
  updateTemplate,
  type TemplateItem,
  type TemplateScope,
} from '@/api/templates'

/** 模板资产管理状态（列表/筛选/CRUD/图片管理/导入导出） */
export function useTemplates(serial: string) {
  const all = ref<TemplateItem[]>([])
  const loading = ref(true)
  const error = ref('')
  const scope = ref<TemplateScope>('all')
  const busyId = ref<string | null>(null)

  const filtered = computed(() => {
    if (scope.value === 'device')
      return all.value.filter(t => t.serial === serial)
    if (scope.value === 'global')
      return all.value.filter(t => t.serial === 'global')
    return all.value
  })

  async function load() {
    loading.value = true
    error.value = ''
    try {
      const data = await listTemplates(serial)
      all.value = Array.isArray(data.templates) ? data.templates : []
    }
    catch (e) {
      error.value = e instanceof Error ? e.message : '加载失败'
    }
    finally {
      loading.value = false
    }
  }

  async function toggleEnabled(t: TemplateItem) {
    busyId.value = t.id
    try {
      const updated = await updateTemplate(t.id, { enabled: !t.enabled })
      replace(updated)
    }
    finally {
      busyId.value = null
    }
  }

  async function saveThreshold(t: TemplateItem, threshold: number) {
    busyId.value = t.id
    try {
      const updated = await updateTemplate(t.id, { threshold })
      replace(updated)
    }
    finally {
      busyId.value = null
    }
  }

  async function switchScope(t: TemplateItem) {
    const target = t.serial === 'global' ? serial : 'global'
    busyId.value = t.id
    try {
      const updated = await updateTemplate(t.id, { serial: target })
      replace(updated)
    }
    finally {
      busyId.value = null
    }
  }

  function replace(updated: TemplateItem) {
    const i = all.value.findIndex(t => t.id === updated.id)
    if (i >= 0)
      all.value[i] = updated
  }

  async function remove(id: string) {
    busyId.value = id
    try {
      await deleteTemplate(id)
      all.value = all.value.filter(t => t.id !== id)
    }
    finally {
      busyId.value = null
    }
  }

  async function uploadSingle(image: File, targetSerial: string) {
    const form = new FormData()
    form.append('image', image)
    form.append('serial', targetSerial)
    form.append('threshold', '0.88')
    const name = image.name.replace(/\.[^/.]+$/, '')
    if (name)
      form.append('name', name)
    await createTemplate(form)
    await load()
  }

  async function addImage(id: string, image: File) {
    busyId.value = id
    try {
      await addTemplateImage(id, image)
      await load()
    }
    finally {
      busyId.value = null
    }
  }

  async function importZip(file: File, targetSerial: string): Promise<number> {
    const data = await importTemplates(file, targetSerial)
    await load()
    return data.imported ?? 0
  }

  return {
    all, loading, error, scope, busyId, filtered,
    load, toggleEnabled, saveThreshold, switchScope, remove,
    uploadSingle, addImage, importZip,
  }
}
