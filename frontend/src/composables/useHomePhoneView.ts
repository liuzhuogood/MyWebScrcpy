import { computed, ref, watch } from 'vue'
import type { Device } from '../api/devices.ts'

export type DashSize = 'small' | 'medium' | 'large' | 'fit'

export const PHONE_HIDDEN_KEY = 'home_phone_hidden_devices'
export const PHONE_DASH_SIZE_KEY = 'home_phone_dash_size'

export const VALID_DASH_SIZES: DashSize[] = ['small', 'medium', 'large', 'fit']

export function normalizeDashSize(val: unknown): DashSize {
  if (typeof val === 'string' && VALID_DASH_SIZES.includes(val as DashSize)) {
    return val as DashSize
  }
  return 'small'
}

export function parseHiddenSerials(raw: string | null): string[] {
  if (!raw)
    return []
  try {
    const parsed = JSON.parse(raw)
    if (Array.isArray(parsed)) {
      return parsed.filter((item): item is string => typeof item === 'string')
    }
    return []
  }
  catch {
    return []
  }
}

export function isDeviceOnlineForStream(
  device: { state?: string } | null | undefined,
  mockEnabled = false,
): boolean {
  if (mockEnabled)
    return false
  return device?.state === 'device'
}

export interface NumberedDevice {
  serial: string
  number: number
  label: string // '01', '02', ...
  device: Device
}

/**
 * 稳定映射设备与编号：
 * 按序列号稳定字典序排序，保证无论 ADB 轮询列表顺序如何变动，设备编号恒定一致。
 * 不伪造设备，有多少真实设备就生成多少编号项。
 */
export function buildStableNumberedDevices(devices: Device[]): NumberedDevice[] {
  const sorted = devices.slice().sort((a, b) => a.serial.localeCompare(b.serial))
  return sorted.map((device, idx) => {
    const num = idx + 1
    const label = num < 10 ? `0${num}` : String(num)
    return {
      serial: device.serial,
      number: num,
      label,
      device,
    }
  })
}

export function useHomePhoneView(allDevices: () => Device[]) {
  const initialDashSize = normalizeDashSize(localStorage.getItem(PHONE_DASH_SIZE_KEY))
  const dashSize = ref<DashSize>(initialDashSize)

  const hiddenSerials = ref<string[]>(parseHiddenSerials(localStorage.getItem(PHONE_HIDDEN_KEY)))

  watch(dashSize, (newVal) => {
    localStorage.setItem(PHONE_DASH_SIZE_KEY, newVal)
  })

  function saveHidden() {
    localStorage.setItem(PHONE_HIDDEN_KEY, JSON.stringify(hiddenSerials.value))
  }

  function isDeviceVisible(serial: string): boolean {
    return !hiddenSerials.value.includes(serial)
  }

  function toggleDeviceVisibility(serial: string) {
    if (hiddenSerials.value.includes(serial)) {
      hiddenSerials.value = hiddenSerials.value.filter(s => s !== serial)
    }
    else {
      hiddenSerials.value = [...hiddenSerials.value, serial]
    }
    saveHidden()
  }

  function showAllDevices() {
    hiddenSerials.value = []
    saveHidden()
  }

  function showOnlySelected(selectedSerials: string[]) {
    if (selectedSerials.length === 0)
      return
    const selectedSet = new Set(selectedSerials)
    hiddenSerials.value = allDevices()
      .filter(d => !selectedSet.has(d.serial))
      .map(d => d.serial)
    saveHidden()
  }

  function setDashSize(sz: DashSize) {
    dashSize.value = normalizeDashSize(sz)
  }

  // 稳定排序编号设备列表
  const numberedDevices = computed(() => {
    return buildStableNumberedDevices(allDevices())
  })

  // 编号字典：serial -> label ('01', '02', etc.)
  const serialToNumberMap = computed(() => {
    const map = new Map<string, string>()
    for (const item of numberedDevices.value) {
      map.set(item.serial, item.label)
    }
    return map
  })

  // 当前可见设备列表
  const visibleDevices = computed(() => {
    return allDevices().filter(d => !hiddenSerials.value.includes(d.serial))
  })

  return {
    dashSize,
    hiddenSerials,
    numberedDevices,
    serialToNumberMap,
    visibleDevices,
    isDeviceVisible,
    toggleDeviceVisibility,
    showAllDevices,
    showOnlySelected,
    setDashSize,
  }
}
