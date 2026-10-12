import { defineStore } from 'pinia'
import { ref } from 'vue'
import { checkUpdate, startUpgrade, subscribeUpgradeStream, getUpgradeStatus, isDesktopUpdate, upgradeDesktop, type CheckResult, type UpgradeProgress } from '@/api/updater'

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
    if (isUpgrading.value || !checkResult.value?.can_upgrade) return
    isUpgrading.value = true
    errorMessage.value = null

    progress.value = { state: 'downloading', percent: 0, downloaded: 0, total: 0, speed: '' }
    if (isDesktopUpdate()) {
      try { await upgradeDesktop(value => { progress.value = value }) }
      catch (err: any) { errorMessage.value = String(err); progress.value.state = 'error'; isUpgrading.value = false }
      return
    }

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
        // A controlled restart closes SSE; status polling verifies the final result.
        if (!isUpgrading.value && progress.value.state !== 'success') errorMessage.value = err?.message || '连接断开'
      }
    )

    try {
      await startUpgrade()
      const started = Date.now()
      const poll = async () => {
        if (!isUpgrading.value) return
        try {
          const value = await getUpgradeStatus()
          progress.value = value
          if (value.state === 'success' || value.state === 'error') {
            isUpgrading.value = false
            unsubscribeStream?.()
            unsubscribeStream = null
            if (value.state === 'error') errorMessage.value = value.error || '更新失败'
            return
          }
        } catch { /* Service can be unavailable during restart. */ }
        if (Date.now() - started > 20 * 60_000) {
          isUpgrading.value = false
          errorMessage.value = '未确认更新结果，请检查服务状态'
          progress.value.state = 'error'
          unsubscribeStream?.()
          unsubscribeStream = null
          return
        }
        setTimeout(poll, 1000)
      }
      setTimeout(poll, 1000)
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
