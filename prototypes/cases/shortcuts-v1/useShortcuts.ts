import { ref } from 'vue'

export type ShortcutCategory = 'privacy' | 'player' | 'device'
export type RuntimeEnv = 'desktop' | 'web'
export type PlatformOs = 'mac' | 'windows'

export interface ShortcutItem {
  id: string
  name: string
  description: string
  category: ShortcutCategory
  supportedEnvs: RuntimeEnv[]
  desktopKey: string
  desktopMacKey: string
  webKey: string
  webMacKey: string
  isGlobalDesktop: boolean
  enabled: boolean
}

export interface BossKeyBehavior {
  mode: 'hide_all' | 'fake_work' | 'minimize_tray'
  muteAudio: boolean
}

const DEFAULT_SHORTCUTS: ShortcutItem[] = [
  {
    id: 'boss_key',
    name: '老板键 (一键隐匿)',
    description: '瞬时隐藏全部投屏与窗口，切入安全隐匿状态并静音',
    category: 'privacy',
    supportedEnvs: ['desktop', 'web'],
    desktopKey: 'Alt + Shift + H',
    desktopMacKey: '⌥ ⇧ H',
    webKey: 'Alt + H',
    webMacKey: '⌥ H',
    isGlobalDesktop: true,
    enabled: true,
  },
  {
    id: 'toggle_floating',
    name: '投屏小窗总开关',
    description: '批量显示或隐藏当前所有运行中的画中画小窗',
    category: 'privacy',
    supportedEnvs: ['desktop', 'web'],
    desktopKey: 'Alt + Shift + W',
    desktopMacKey: '⌥ ⇧ W',
    webKey: 'Alt + W',
    webMacKey: '⌥ W',
    isGlobalDesktop: true,
    enabled: true,
  },
  {
    id: 'open_device_n',
    name: '序号快速打开设备投屏',
    description: '按下 Option/Alt + 1~9 直接唤出对应排序的设备小窗，支持卡片拖动调整顺序',
    category: 'player',
    supportedEnvs: ['desktop', 'web'],
    desktopKey: 'Alt + 1~9',
    desktopMacKey: '⌥ 1~9',
    webKey: 'Alt + 1~9',
    webMacKey: '⌥ 1~9',
    isGlobalDesktop: false,
    enabled: true,
  },
  {
    id: 'toggle_toolbar',
    name: '小窗控制条显示与隐藏',
    description: '切换投屏小窗顶栏/操作工具栏/参考悬浮条的折叠与展开',
    category: 'player',
    supportedEnvs: ['desktop', 'web'],
    desktopKey: 'Alt + T',
    desktopMacKey: '⌥ T',
    webKey: 'Alt + T',
    webMacKey: '⌥ T',
    isGlobalDesktop: false,
    enabled: true,
  },
  {
    id: 'restore_effects',
    name: '恢复小窗状态 (仅桌面端)',
    description: '恢复桌面原生小窗透明度至 100%，解除鼠标穿透并呼出主窗口（Web 端无原生窗口穿透属性，故自动隐藏）',
    category: 'player',
    supportedEnvs: ['desktop'], // 仅桌面端有意义
    desktopKey: 'Alt + P',
    desktopMacKey: '⌥ P',
    webKey: 'Alt + P',
    webMacKey: '⌥ P',
    isGlobalDesktop: true,
    enabled: true,
  },
  {
    id: 'toggle_inspector',
    name: '辅助参考与检查器',
    description: '快速展开或收起取色坐标、辅助参考线与 UI XML 检查侧栏',
    category: 'player',
    supportedEnvs: ['desktop', 'web'],
    desktopKey: 'Alt + I',
    desktopMacKey: '⌥ I',
    webKey: 'Alt + I',
    webMacKey: '⌥ I',
    isGlobalDesktop: false,
    enabled: true,
  },
  {
    id: 'device_back',
    name: '设备返回键',
    description: '当前投屏设备注入返回动作',
    category: 'device',
    supportedEnvs: ['desktop', 'web'],
    desktopKey: 'Alt + Backspace',
    desktopMacKey: '⌥ ⌫',
    webKey: 'Alt + Backspace',
    webMacKey: '⌥ ⌫',
    isGlobalDesktop: false,
    enabled: true,
  },
  {
    id: 'device_home',
    name: '设备主屏键',
    description: '当前投屏设备注入返回桌面主屏',
    category: 'device',
    supportedEnvs: ['desktop', 'web'],
    desktopKey: 'Alt + Home',
    desktopMacKey: '⌥ ↖',
    webKey: 'Alt + Home',
    webMacKey: '⌥ ↖',
    isGlobalDesktop: false,
    enabled: true,
  },
  {
    id: 'device_tasks',
    name: '多任务切换',
    description: '呼出当前设备的后台多任务卡片视图',
    category: 'device',
    supportedEnvs: ['desktop', 'web'],
    desktopKey: 'Alt + M',
    desktopMacKey: '⌥ M',
    webKey: 'Alt + M',
    webMacKey: '⌥ M',
    isGlobalDesktop: false,
    enabled: true,
  },
]

// 浏览器高危快捷键列表（网页端如果录制这些需要警示）
const BROWSER_RESERVED_KEYS = [
  'control+w',
  'control+t',
  'control+n',
  'control+r',
  'control+shift+w',
  'control+shift+t',
  'control+q',
  'alt+f4',
]

export function useShortcuts() {
  const currentEnv = ref<RuntimeEnv>('desktop')
  const currentOs = ref<PlatformOs>('mac')
  const shortcuts = ref<ShortcutItem[]>(JSON.parse(JSON.stringify(DEFAULT_SHORTCUTS)))

  const bossBehavior = ref<BossKeyBehavior>({
    mode: 'fake_work',
    muteAudio: true,
  })

  // 录制状态
  const recordingId = ref<string | null>(null)
  const recordedConflictWarning = ref<string | null>(null)

  function getKey(item: ShortcutItem) {
    if (currentEnv.value === 'desktop') {
      return currentOs.value === 'mac' ? item.desktopMacKey : item.desktopKey
    }
    return currentOs.value === 'mac' ? item.webMacKey : item.webKey
  }

  function startRecording(id: string) {
    recordingId.value = id
    recordedConflictWarning.value = null
  }

  function stopRecording() {
    recordingId.value = null
    recordedConflictWarning.value = null
  }

  function updateKey(id: string, newKeyStandard: string, newKeyMac: string) {
    const item = shortcuts.value.find(s => s.id === id)
    if (!item) return

    // 检查冲突
    if (currentEnv.value === 'web') {
      const normalized = newKeyStandard.toLowerCase().replace(/\s+/g, '')
      if (BROWSER_RESERVED_KEYS.includes(normalized)) {
        recordedConflictWarning.value = `组合键 ${newKeyStandard} 与浏览器内置保留动作冲突，可能无法拦截或直接关闭标签！`
        return
      }
    }

    if (currentEnv.value === 'desktop') {
      item.desktopKey = newKeyStandard
      item.desktopMacKey = newKeyMac
    } else {
      item.webKey = newKeyStandard
      item.webMacKey = newKeyMac
    }
    stopRecording()
  }

  function resetToDefaults() {
    shortcuts.value = JSON.parse(JSON.stringify(DEFAULT_SHORTCUTS))
    stopRecording()
  }

  const categoryLabels: Record<ShortcutCategory, { name: string; desc: string }> = {
    privacy: { name: '隐私与全局控制', desc: '包括紧急防窥老板键、小窗一键隐藏与多窗口协同' },
    player: { name: '投屏与小窗控制', desc: '控制工具栏折叠、序号快速投屏与显示效果' },
    device: { name: '设备实体键映射', desc: '在投屏聚焦时注入 Android 系统级导航操作' },
  }

  return {
    currentEnv,
    currentOs,
    shortcuts,
    bossBehavior,
    recordingId,
    recordedConflictWarning,
    getKey,
    startRecording,
    stopRecording,
    updateKey,
    resetToDefaults,
    categoryLabels,
  }
}
