import { useStorage } from '@vueuse/core'

export type OpenMode = 'floating' | 'page'
export type FloatingSize = 'compact' | 'normal' | 'large'
/** 点击/滑动注入模式：sdk = scrcpy 注入（默认），adb = adb shell input，hid = UHID 虚拟触摸屏 */
export type InjectMode = 'sdk' | 'adb' | 'hid'

export function usePlayerSettings() {
  const openMode = useStorage<OpenMode>('player-open-mode', 'floating')
  const floatingSize = useStorage<FloatingSize>('player-floating-size', 'normal')
  const borderless = useStorage<boolean>('player-borderless', false)
  const autoAudio = useStorage<boolean>('player-auto-audio', false)
  const bitrate = useStorage<number>('player-bitrate', 4000000)
  const maxFps = useStorage<number>('player-max-fps', 30)
  const injectMode = useStorage<InjectMode>('player-inject-mode', 'sdk')

  // 同步复用现有键值，确保全面向后兼容
  const autoUnlockOnWake = useStorage<boolean>('auto-unlock-on-wake', true)
  const clipboardSync = useStorage<boolean>('clipboard-sync', true)
  const touchVisible = useStorage<boolean>('show-touch-overlay', true)
  const boxesVisible = useStorage<boolean>('show-template-boxes', true)

  return {
    openMode,
    floatingSize,
    borderless,
    autoAudio,
    bitrate,
    maxFps,
    injectMode,
    autoUnlockOnWake,
    clipboardSync,
    touchVisible,
    boxesVisible,
  }
}
