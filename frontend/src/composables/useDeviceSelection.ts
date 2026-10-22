import { computed, ref, watch, type Ref } from 'vue'
import type { Device } from '../api/devices.ts'
import { dedupeSerials, filterSelectableDevices } from './useDeviceBatch.ts'

export function useDeviceSelection(
  currentScopeDevices: Ref<Device[]>,
  isMockEnabled: Ref<boolean> = ref(false),
) {
  const selectedSerials = ref<string[]>([])

  // 当前作用域下允许被选中的设备：
  // 必须是当前页/当前列表的设备，排除掉离线/未授权设备；当处于 Mock 模式时不可做真机批操作
  const selectableDevices = computed(() => {
    if (isMockEnabled.value) {
      return []
    }
    return filterSelectableDevices(currentScopeDevices.value)
  })

  // 当作用域列表改变（显隐/翻页/掉线/Mock开启）时，自动剔除已经不在可选范围内的设备
  watch(
    [selectableDevices, isMockEnabled],
    () => {
      const allowed = new Set(selectableDevices.value.map(d => d.serial))
      selectedSerials.value = selectedSerials.value.filter(s => allowed.has(s))
    },
    { immediate: true },
  )

  const isAllSelected = computed(() => {
    if (selectableDevices.value.length === 0)
      return false
    return selectableDevices.value.every(d => selectedSerials.value.includes(d.serial))
  })

  const selectedDevices = computed(() => {
    return selectableDevices.value.filter(d => selectedSerials.value.includes(d.serial))
  })

  function isSelected(serial: string): boolean {
    return selectedSerials.value.includes(serial)
  }

  function toggleSelect(serial: string) {
    if (isMockEnabled.value)
      return

    if (selectedSerials.value.includes(serial)) {
      selectedSerials.value = selectedSerials.value.filter(s => s !== serial)
    }
    else {
      const isSelectable = selectableDevices.value.some(d => d.serial === serial)
      if (isSelectable) {
        selectedSerials.value = dedupeSerials([...selectedSerials.value, serial])
      }
    }
  }

  function selectAll() {
    if (isMockEnabled.value) {
      selectedSerials.value = []
      return
    }

    if (isAllSelected.value) {
      selectedSerials.value = []
    }
    else {
      selectedSerials.value = selectableDevices.value.map(d => d.serial)
    }
  }

  function clearSelection() {
    selectedSerials.value = []
  }

  // 发起前再重校验一次，确保不携带任何已掉线或隐藏设备
  function getValidatedTargets(): string[] {
    const allowed = new Set(selectableDevices.value.map(d => d.serial))
    const valid = selectedSerials.value.filter(s => allowed.has(s))
    selectedSerials.value = valid
    return valid
  }

  return {
    selectedSerials,
    selectableDevices,
    selectedDevices,
    isAllSelected,
    isSelected,
    toggleSelect,
    selectAll,
    clearSelection,
    getValidatedTargets,
  }
}
