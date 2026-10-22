import assert from 'node:assert/strict'
import test from 'node:test'
import { ref, nextTick } from 'vue'

// Polyfill localStorage for node environment before loading production composables
const storageMap = new Map()
if (typeof globalThis.localStorage === 'undefined') {
  globalThis.localStorage = {
    getItem: (k) => storageMap.get(k) ?? null,
    setItem: (k, v) => storageMap.set(k, String(v)),
    removeItem: (k) => storageMap.delete(k),
    clear: () => storageMap.clear(),
  }
}

import {
  buildStableNumberedDevices,
  normalizeDashSize,
  parseHiddenSerials,
  isDeviceOnlineForStream,
  useHomePhoneView,
} from '../src/composables/useHomePhoneView.ts'

import { useDeviceSelection } from '../src/composables/useDeviceSelection.ts'

test('buildStableNumberedDevices handles 0 devices without generating dummy entries', () => {
  const result = buildStableNumberedDevices([])
  assert.equal(result.length, 0)
})

test('buildStableNumberedDevices formats 2-digit zero-padded numbers up to actual device count', () => {
  const devices = [
    { serial: 'device-01', model: 'M1', state: 'device' },
    { serial: 'device-02', model: 'M2', state: 'device' },
    { serial: 'device-10', model: 'M10', state: 'device' },
  ]
  const result = buildStableNumberedDevices(devices)
  assert.equal(result.length, 3)
  assert.equal(result[0].label, '01')
  assert.equal(result[0].number, 1)
  assert.equal(result[0].serial, 'device-01')
  assert.equal(result[1].label, '02')
  assert.equal(result[1].number, 2)
  assert.equal(result[2].label, '03')
  assert.equal(result[2].number, 3)
  assert.equal(result[2].serial, 'device-10')
})

test('buildStableNumberedDevices ensures stable number mapping even when poll order shifts', () => {
  const listA = [
    { serial: '192.168.1.102:5555', state: 'device' },
    { serial: '192.168.1.101:5555', state: 'device' },
    { serial: '192.168.1.103:5555', state: 'device' },
  ]
  const listB = [
    { serial: '192.168.1.103:5555', state: 'device' },
    { serial: '192.168.1.102:5555', state: 'device' },
    { serial: '192.168.1.101:5555', state: 'device' },
  ]

  const mappedA = buildStableNumberedDevices(listA)
  const mappedB = buildStableNumberedDevices(listB)

  // 无论输入数组顺序如何颠倒，序列号与编号的映射必须 100% 恒定
  assert.equal(mappedA[0].serial, '192.168.1.101:5555')
  assert.equal(mappedA[0].label, '01')
  assert.equal(mappedB[0].serial, '192.168.1.101:5555')
  assert.equal(mappedB[0].label, '01')

  assert.equal(mappedA[1].serial, '192.168.1.102:5555')
  assert.equal(mappedA[1].label, '02')
  assert.equal(mappedB[1].serial, '192.168.1.102:5555')
  assert.equal(mappedB[1].label, '02')

  assert.equal(mappedA[2].serial, '192.168.1.103:5555')
  assert.equal(mappedA[2].label, '03')
  assert.equal(mappedB[2].serial, '192.168.1.103:5555')
  assert.equal(mappedB[2].label, '03')
})

test('normalizeDashSize safely ensures valid enum value, fallbacks to small', () => {
  assert.equal(normalizeDashSize('small'), 'small')
  assert.equal(normalizeDashSize('medium'), 'medium')
  assert.equal(normalizeDashSize('large'), 'large')
  assert.equal(normalizeDashSize('fit'), 'fit')
  assert.equal(normalizeDashSize('unknown_size'), 'small')
  assert.equal(normalizeDashSize(null), 'small')
  assert.equal(normalizeDashSize(123), 'small')
})

test('parseHiddenSerials safely parses local storage payload', () => {
  assert.deepEqual(parseHiddenSerials(null), [])
  assert.deepEqual(parseHiddenSerials(''), [])
  assert.deepEqual(parseHiddenSerials('{ "not": "array" }'), [])
  assert.deepEqual(parseHiddenSerials('broken json'), [])
  assert.deepEqual(parseHiddenSerials('["d1", "d2", 123]'), ['d1', 'd2'])
})

test('isDeviceOnlineForStream excludes mock devices and non-device states', () => {
  assert.equal(isDeviceOnlineForStream({ serial: 'd1', state: 'device' }, false), true)
  assert.equal(isDeviceOnlineForStream({ serial: 'd1', state: 'device' }, true), false)
  assert.equal(isDeviceOnlineForStream({ serial: 'd2', state: 'offline' }, false), false)
  assert.equal(isDeviceOnlineForStream({ serial: 'd3', state: 'unauthorized' }, false), false)
  assert.equal(isDeviceOnlineForStream({ serial: 'd4', state: 'unknown' }, false), false)
  assert.equal(isDeviceOnlineForStream(null, false), false)
})

test('production useHomePhoneView: toggle visibility and showAllDevices recovery', () => {
  storageMap.clear()
  const rawDevices = ref([
    { serial: 'dev-1', state: 'device' },
    { serial: 'dev-2', state: 'device' },
    { serial: 'dev-3', state: 'device' },
  ])
  const phone = useHomePhoneView(() => rawDevices.value)

  assert.equal(phone.visibleDevices.value.length, 3)
  assert.equal(phone.hiddenSerials.value.length, 0)

  // 单机编号点击隐藏
  phone.toggleDeviceVisibility('dev-2')
  assert.deepEqual(phone.hiddenSerials.value, ['dev-2'])
  assert.equal(phone.visibleDevices.value.length, 2)
  assert.deepEqual(phone.visibleDevices.value.map(d => d.serial), ['dev-1', 'dev-3'])

  // 再次点击恢复
  phone.toggleDeviceVisibility('dev-2')
  assert.deepEqual(phone.hiddenSerials.value, [])
  assert.equal(phone.visibleDevices.value.length, 3)

  // 全部隐藏后恢复全显
  phone.toggleDeviceVisibility('dev-1')
  phone.toggleDeviceVisibility('dev-2')
  phone.toggleDeviceVisibility('dev-3')
  assert.equal(phone.visibleDevices.value.length, 0)

  phone.showAllDevices()
  assert.equal(phone.visibleDevices.value.length, 3)
  assert.deepEqual(phone.hiddenSerials.value, [])
})

test('production integration: showOnlySelected with useDeviceSelection preserves selection without recursive clearing', async () => {
  storageMap.clear()
  const rawDevices = ref([
    { serial: 'dev-1', state: 'device' },
    { serial: 'dev-2', state: 'device' },
    { serial: 'dev-3', state: 'device' },
  ])
  const phone = useHomePhoneView(() => rawDevices.value)
  const isMock = ref(false)
  const selection = useDeviceSelection(phone.visibleDevices, isMock)

  // 勾选 dev-2
  selection.toggleSelect('dev-2')
  assert.deepEqual(selection.selectedSerials.value, ['dev-2'])

  // 触发只显已选
  phone.showOnlySelected(selection.selectedSerials.value)
  await nextTick()

  // dev-1 和 dev-3 被隐藏，dev-2 仍然可见
  assert.equal(phone.visibleDevices.value.length, 1)
  assert.equal(phone.visibleDevices.value[0].serial, 'dev-2')
  // 关键验证：已选 dev-2 必须完好保留，严禁发生递归清空！
  assert.deepEqual(selection.selectedSerials.value, ['dev-2'])
  assert.deepEqual(selection.getValidatedTargets(), ['dev-2'])

  // 当无任何设备被选中时，调用 showOnlySelected 状态不受影响
  selection.clearSelection()
  assert.equal(selection.selectedSerials.value.length, 0)
  const hiddenBefore = [...phone.hiddenSerials.value]
  phone.showOnlySelected([])
  assert.deepEqual(phone.hiddenSerials.value, hiddenBefore)
})

test('production integration: offline / drop / mock changes strictly exclude devices from selection', async () => {
  storageMap.clear()
  const rawDevices = ref([
    { serial: 'dev-online', state: 'device' },
    { serial: 'dev-offline', state: 'offline' },
    { serial: 'dev-unauth', state: 'unauthorized' },
  ])
  const phone = useHomePhoneView(() => rawDevices.value)
  const isMock = ref(false)
  const selection = useDeviceSelection(phone.visibleDevices, isMock)

  // 仅在线 device 可选
  assert.equal(selection.selectableDevices.value.length, 1)
  assert.equal(selection.selectableDevices.value[0].serial, 'dev-online')

  // 勾选在线机
  selection.toggleSelect('dev-online')
  assert.deepEqual(selection.selectedSerials.value, ['dev-online'])

  // 尝试勾选离线机被忽略
  selection.toggleSelect('dev-offline')
  assert.deepEqual(selection.selectedSerials.value, ['dev-online'])

  // 设备掉线重校验：dev-online 状态变为 offline 时自动移出选中
  rawDevices.value = [
    { serial: 'dev-online', state: 'offline' },
    { serial: 'dev-offline', state: 'offline' },
  ]
  await nextTick()
  assert.deepEqual(selection.selectedSerials.value, [])
  assert.deepEqual(selection.getValidatedTargets(), [])

  // 开启 Mock 模式时强制清空且不可选中
  rawDevices.value = [{ serial: 'dev-mock', state: 'device' }]
  await nextTick()
  selection.toggleSelect('dev-mock')
  assert.deepEqual(selection.selectedSerials.value, ['dev-mock'])

  isMock.value = true
  await nextTick()
  assert.deepEqual(selection.selectedSerials.value, [])
  assert.deepEqual(selection.selectableDevices.value, [])
})
