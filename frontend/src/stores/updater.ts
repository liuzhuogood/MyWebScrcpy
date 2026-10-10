import { defineStore } from 'pinia'
import { ref } from 'vue'
import { checkUpdate, startUpgrade, subscribeUpgradeStream, type CheckResult, type UpgradeProgress } from '@/api/updater'

export const useUpdaterStore = defineStore('updater', () => {
  const isChecking = ref(false)
  const isUpgrading = ref(false)
  const checkResult = ref<CheckResult | null>(null)
  const progress = ref<UpgradeProgress>({
    state: 'idle',
    percent: 0,
    downloaded: 0,
    total: 0,
    speed: '0 KB/s',
  })
  const errorMessage = ref<string | null>(null)
  const dialogOpen = ref(false)
  let unsubscribeStream: (() => void) | null = null

  async function check() {
    isChecking.value = true
    errorMessage.value = null
    try {
      const res = await checkUpdate()
      checkResult.value = res
      return res
    } catch (err: any) {
      errorMessage.value = err.message || '检查更新失败'
      return null
    } finally {
      isChecking.value = false
    }
  }

  async function upgrade() {
    if (isUpgrading.value) return
    isUpgrading.value = true
    errorMessage.value = null

    // 先建立 SSE 连接监听
    if (unsubscribeStream) {
      unsubscribeStream()
      unsubscribeStream = null
    }

    unsubscribeStream = subscribeUpgradeStream(
      (p) => {
        progress.value = p
        if (p.state === 'error' && p.error) {
          errorMessage.value = p.error
          isUpgrading.value = false
        }
      },
      (err) => {
        if (progress.value.state !== 'success') {
          errorMessage.value = err?.message || '连接断开'
        }
      }
    )

    try {
      await startUpgrade({
        download_url: checkResult.value?.download_url,
        asset_size: checkResult.value?.asset_size,
      })
    } catch (err: any) {
      errorMessage.value = err.message || '启动更新失败'
      isUpgrading.value = false
      if (unsubscribeStream) {
        unsubscribeStream()
        unsubscribeStream = null
      }
    }
  }

  function openDialog() {
    dialogOpen.value = true
  }

  function closeDialog() {
    if (isUpgrading.value && progress.value.state === 'downloading') {
      // 正在下载时允许隐藏弹窗但不中断
      dialogOpen.value = false
      return
    }
    dialogOpen.value = false
  }

  return {
    isChecking,
    isUpgrading,
    checkResult,
    progress,
    errorMessage,
    dialogOpen,
    check,
    upgrade,
    openDialog,
    closeDialog,
  }
})
