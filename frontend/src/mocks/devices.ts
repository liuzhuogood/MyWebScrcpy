import type { Device } from '@/api/devices'

/** 无后端演示用的假设备原型，与真实 Device 契约一致 */
interface MockTemplate {
  brand: string
  manufacturer: string
  model: string
  product: string
  device: string
  android_release: string
  sdk_version: number
  cpu_temp_supported: boolean
}

const MOCK_TEMPLATES: MockTemplate[] = [
  { brand: 'Google', manufacturer: 'Google', model: 'Pixel 8', product: 'husky', device: 'husky', android_release: '15', sdk_version: 35, cpu_temp_supported: true },
  { brand: 'Xiaomi', manufacturer: 'Xiaomi', model: '23127PN0CC (小米 14)', product: 'houji', device: 'houji', android_release: '14', sdk_version: 34, cpu_temp_supported: true },
  { brand: 'Android Emulator', manufacturer: 'Google', model: 'sdk_gphone64_arm64', product: 'sdk_gphone64_arm64', device: 'emu64a', android_release: '13', sdk_version: 33, cpu_temp_supported: false },
  { brand: 'Samsung', manufacturer: 'samsung', model: 'SM-S9280 (Galaxy S24)', product: 'dm3q', device: 'dm3q', android_release: '14', sdk_version: 34, cpu_temp_supported: false },
  { brand: 'OnePlus', manufacturer: 'OnePlus', model: 'PJZ110 (一加 12)', product: 'waffle', device: 'waffle', android_release: '14', sdk_version: 34, cpu_temp_supported: false },
  { brand: 'vivo', manufacturer: 'vivo', model: 'V2309A (X100)', product: 'PD2309', device: 'PD2309', android_release: '14', sdk_version: 34, cpu_temp_supported: true },
  { brand: 'OPPO', manufacturer: 'OPPO', model: 'PJD110 (Find X7)', product: 'OP5929L1', device: 'OP5929L1', android_release: '14', sdk_version: 34, cpu_temp_supported: false },
  { brand: 'Redmi', manufacturer: 'Xiaomi', model: '2312DRA50C (K70)', product: 'dizi', device: 'dizi', android_release: '14', sdk_version: 34, cpu_temp_supported: true },
  { brand: 'HUAWEI', manufacturer: 'HUAWEI', model: 'ALN-AL00 (Mate 60)', product: 'ALN', device: 'HWALN', android_release: '12', sdk_version: 31, cpu_temp_supported: false },
  { brand: 'honor', manufacturer: 'honor', model: 'BVL-AN16 (Magic6)', product: 'BVL', device: 'HWBVL', android_release: '14', sdk_version: 34, cpu_temp_supported: false },
]

const MOCK_COUNT = 500

// 确定性伪随机（seed 固定，列表稳定不抖动），避免每次渲染 serial/电量跳变
function seededRand(seed: number): () => number {
  let s = seed >>> 0
  return () => {
    s = (s * 1664525 + 1013904223) >>> 0
    return s / 4294967296
  }
}

function buildMockDevices(): Device[] {
  const rand = seededRand(20261022)
  const devices: Device[] = []
  const ipPrefixes = ['10.0.0', '192.168.1', '172.16.8', '10.1.2']
  for (let i = 0; i < MOCK_COUNT; i++) {
    const t = MOCK_TEMPLATES[i % MOCK_TEMPLATES.length]
    const isEmulator = t.product === 'sdk_gphone64_arm64'
    const offline = !isEmulator && rand() < 0.12
    const serial = isEmulator
      ? 'emulator-' + String(5554 + i * 2)
      : ipPrefixes[i % ipPrefixes.length] + '.' + String((i % 250) + 2) + ':5555'
    devices.push({
      serial,
      state: offline ? 'offline' : 'device',
      source: 'auto',
      brand: t.brand,
      manufacturer: t.manufacturer,
      model: t.model,
      product: t.product,
      device: t.device,
      transport_id: String(i + 1),
      android_release: t.android_release,
      sdk_version: t.sdk_version,
      cpu_abi: 'arm64-v8a',
      battery_level: offline ? 0 : Math.floor(10 + rand() * 90),
      battery_charging: !offline && rand() < 0.4,
      battery_temp: offline ? 0 : Math.round((28 + rand() * 12) * 10) / 10,
      ...(t.cpu_temp_supported && !offline
        ? { cpu_temp: Math.round((38 + rand() * 10) * 10) / 10, cpu_temp_supported: true }
        : { cpu_temp_supported: false }),
      ...(!offline ? {
        screen_on: (i % 4 !== 1),
        screen_locked: (i % 4 === 1),
        foreground_app_name: ['微信', '快手极速版', '哔哩哔哩', '系统设置', '网易云音乐', '抖音'][i % 6],
        foreground_pkg: ['com.tencent.mm', 'com.kuaishou.nebula', 'tv.danmaku.bili', 'com.android.settings', 'com.netease.cloudmusic', 'com.ss.android.ugc.aweme'][i % 6],
        foreground_app_icon: ['wechat', 'kuaishou', 'bilibili', 'settings', 'neteasemusic', 'douyin'][i % 6],
      } : {}),
    })
  }
  return devices
}

/** 无后端演示用的假设备，与真实 Device 契约一致 */
export const MOCK_DEVICES: Device[] = buildMockDevices()
