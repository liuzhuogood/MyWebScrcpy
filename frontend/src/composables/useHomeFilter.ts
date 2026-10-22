import { ref, watch } from 'vue'

export const SHOW_OFFLINE_DEVICES_KEY = 'home_show_offline_devices'

export function parseShowOfflinePreference(raw: string | null): boolean {
  return raw === 'true'
}

/**
 * 离线状态正交过滤：
 * 不重新实现搜索或来源规则，直接基于已完成筛选的设备列表做状态约束。
 * - showOffline 为 false 时：严格保留 state === 'device' 的在线可用设备；
 * - showOffline 为 true 时：包含所有状态的设备（如 offline、unauthorized、connecting 等）。
 */
export function filterOnlineDevices<T extends { state?: string }>(
  devices: T[],
  showOffline = false,
): T[] {
  if (showOffline) {
    return devices
  }
  return devices.filter(d => d.state === 'device')
}

export function useHomeFilter() {
  const initialShowOffline = parseShowOfflinePreference(localStorage.getItem(SHOW_OFFLINE_DEVICES_KEY))
  const showOffline = ref<boolean>(initialShowOffline)

  watch(showOffline, (val) => {
    try {
      localStorage.setItem(SHOW_OFFLINE_DEVICES_KEY, String(val))
    }
    catch {
      // 忽略存储异常
    }
  })

  function toggleShowOffline(forceVal?: boolean) {
    if (typeof forceVal === 'boolean') {
      showOffline.value = forceVal
    }
    else {
      showOffline.value = !showOffline.value
    }
  }

  return {
    showOffline,
    toggleShowOffline,
    filterOnlineDevices,
  }
}
