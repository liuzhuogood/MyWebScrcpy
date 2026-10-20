import { ref } from 'vue'

export type DropKind = 'apk' | 'file'

export function formatFileSize(bytes: number): string {
  if (bytes <= 0) return '0 B'
  const mb = bytes / (1024 * 1024)
  if (mb >= 1) return `${mb.toFixed(1)} MB`
  const kb = bytes / 1024
  if (kb >= 1) return `${kb.toFixed(1)} KB`
  return `${bytes} B`
}

/**
 * 投屏画面文件拖拽公共 Composable。
 * 支持识别 APK 文件并触发安装确认弹窗，或识别其他文件并触发保存至 Downloads 弹窗。
 */
export function useDeviceFileDrop() {
  const isDragging = ref(false)
  const dragKind = ref<DropKind | null>(null)
  const candidateFile = ref<File | null>(null)
  const installDialogOpen = ref(false)
  const saveDialogOpen = ref(false)

  let enterCount = 0

  function onDragEnter(e: DragEvent) {
    e.preventDefault()
    if (!e.dataTransfer?.types.includes('Files')) return
    enterCount++
    isDragging.value = true

    // 探测是否包含 .apk 文件
    let isApk = false
    if (e.dataTransfer.items && e.dataTransfer.items.length > 0) {
      for (let i = 0; i < e.dataTransfer.items.length; i++) {
        const item = e.dataTransfer.items[i]
        if (item.kind === 'file') {
          if (item.type === 'application/vnd.android.package-archive') {
            isApk = true
            break
          }
        }
      }
    }
    dragKind.value = isApk ? 'apk' : 'file'
  }

  function onDragOver(e: DragEvent) {
    e.preventDefault()
    if (e.dataTransfer) {
      e.dataTransfer.dropEffect = 'copy'
    }
    if (!isDragging.value) {
      isDragging.value = true
    }
  }

  function onDragLeave(e: DragEvent) {
    e.preventDefault()
    enterCount--
    if (enterCount <= 0) {
      enterCount = 0
      isDragging.value = false
      dragKind.value = null
    }
  }

  function onDrop(e: DragEvent) {
    e.preventDefault()
    enterCount = 0
    isDragging.value = false
    dragKind.value = null

    const files = e.dataTransfer?.files
    if (!files || files.length === 0) return

    const file = files[0]
    candidateFile.value = file

    if (file.name.toLowerCase().endsWith('.apk')) {
      installDialogOpen.value = true
    }
    else {
      saveDialogOpen.value = true
    }
  }

  return {
    isDragging,
    dragKind,
    candidateFile,
    installDialogOpen,
    saveDialogOpen,
    onDragEnter,
    onDragOver,
    onDragLeave,
    onDrop,
  }
}
