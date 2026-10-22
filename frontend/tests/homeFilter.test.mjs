import assert from 'node:assert/strict'
import test from 'node:test'
import {
  filterOnlineDevices,
  parseShowOfflinePreference,
} from '../src/composables/useHomeFilter.ts'

// 模拟 DeviceStore 的原有搜索与显示名逻辑以验证组合正交行为
function mockDisplayName(d, aliases = {}) {
  return d.alias?.trim() || aliases[d.serial]?.trim() || d.brand?.trim() || d.model || 'Android'
}

function mockStoreFilter(devices, { keyword = '', sourceFilter = 'all', aliases = {} } = {}) {
  let list = devices
  if (sourceFilter !== 'all') {
    list = list.filter(d => (d.source || 'auto') === sourceFilter)
  }
  const k = keyword.trim().toLowerCase()
  if (!k)
    return list
  return list.filter(d =>
    [mockDisplayName(d, aliases), d.serial, d.brand, d.model, d.product].filter(Boolean)
      .join(' ').toLowerCase().includes(k),
  )
}

test('parseShowOfflinePreference parses boolean string accurately', () => {
  assert.equal(parseShowOfflinePreference('true'), true)
  assert.equal(parseShowOfflinePreference('false'), false)
  assert.equal(parseShowOfflinePreference(''), false)
  assert.equal(parseShowOfflinePreference(null), false)
  assert.equal(parseShowOfflinePreference('1'), false)
})

test('filterOnlineDevices strictly filters state===device when showOffline is false', () => {
  const sample = [
    { serial: 'd1', state: 'device' },
    { serial: 'd2', state: 'offline' },
    { serial: 'd3', state: 'unauthorized' },
    { serial: 'd4', state: 'device' },
  ]

  const onlyOnline = filterOnlineDevices(sample, false)
  assert.deepEqual(onlyOnline.map(d => d.serial), ['d1', 'd4'])
})

test('filterOnlineDevices preserves all states when showOffline is true', () => {
  const sample = [
    { serial: 'd1', state: 'device' },
    { serial: 'd2', state: 'offline' },
    { serial: 'd3', state: 'unauthorized' },
  ]

  const all = filterOnlineDevices(sample, true)
  assert.deepEqual(all.map(d => d.serial), ['d1', 'd2', 'd3'])
})

test('regression: searches brand "vivo" and combines with showOffline correctly', () => {
  const devices = [
    { serial: 'vivo-001', brand: 'vivo', model: 'V2183A', product: 'PD2183', state: 'device', source: 'auto' },
    { serial: 'vivo-002', brand: 'vivo', model: 'iQOO Neo9', product: 'PD2338', state: 'offline', source: 'manual' },
    { serial: 'oppo-001', brand: 'OPPO', model: 'Find X6', product: 'PHFY10', state: 'device', source: 'auto' },
  ]

  // 1. 用户输入 "vivo"，默认仅显示在线设备 -> 命中 vivo-001
  const vivoFiltered = mockStoreFilter(devices, { keyword: 'vivo' })
  const defaultOnline = filterOnlineDevices(vivoFiltered, false)
  assert.deepEqual(defaultOnline.map(d => d.serial), ['vivo-001'])

  // 2. 用户输入 "vivo"，开启显示离线 -> 命中 vivo-001 和 vivo-002
  const withOffline = filterOnlineDevices(vivoFiltered, true)
  assert.deepEqual(withOffline.map(d => d.serial), ['vivo-001', 'vivo-002'])

  // 3. 用户输入型号 "V2183"，默认在线 -> vivo-001
  const modelFiltered = mockStoreFilter(devices, { keyword: 'v2183' })
  assert.deepEqual(filterOnlineDevices(modelFiltered, false).map(d => d.serial), ['vivo-001'])
})

test('regression: searches custom displayName/alias and combines with showOffline', () => {
  const devices = [
    { serial: 'dev-1', brand: 'Xiaomi', model: '23116PN5BC', state: 'device', source: 'auto' },
    { serial: 'dev-2', brand: 'OnePlus', model: 'PJZ110', state: 'offline', source: 'manual' },
  ]
  const aliases = {
    'dev-1': '客厅测试机',
    'dev-2': '备用机一号',
  }

  // 1. 搜别名 "客厅"，在线 -> dev-1
  const res1 = mockStoreFilter(devices, { keyword: '客厅', aliases })
  assert.deepEqual(filterOnlineDevices(res1, false).map(d => d.serial), ['dev-1'])

  // 2. 搜别名 "备用机"，默认在线 -> 空（因为 dev-2 离线）
  const res2 = mockStoreFilter(devices, { keyword: '备用机', aliases })
  assert.deepEqual(filterOnlineDevices(res2, false).map(d => d.serial), [])

  // 3. 搜别名 "备用机"，开离线 -> dev-2
  assert.deepEqual(filterOnlineDevices(res2, true).map(d => d.serial), ['dev-2'])
})
