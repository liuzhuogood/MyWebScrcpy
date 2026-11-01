export interface DeviceTelemetry {
  brand: string
  manufacturer: string
  model: string
  androidRelease: string
  batteryLevel: number
  batteryCharging: boolean
  batteryTempCelsius: number
  cpuTempCelsius?: number
  cpuTempSupported: boolean
  cpuUsagePercent: number
  currentApp: string
  currentPackage: string
  iconType: string
  resolution: string
  transportSpeed?: string
}

export interface MockPhoneDevice {
  serial: string
  state: 'device' | 'connecting' | 'unauthorized' | 'offline'
  source: 'auto' | 'manual'
  alias?: string
  telemetry: DeviceTelemetry
}

export const MOCK_PHONE_DEVICES: MockPhoneDevice[] = [
  {
    serial: '23127PN0CC_USB',
    state: 'device',
    source: 'auto',
    alias: '小米 14 主机',
    telemetry: {
      brand: 'Xiaomi',
      manufacturer: 'Xiaomi',
      model: '23127PN0CC (小米 14)',
      androidRelease: '14',
      batteryLevel: 85,
      batteryCharging: true,
      batteryTempCelsius: 35.8,
      cpuTempCelsius: 41.5,
      cpuTempSupported: true,
      cpuUsagePercent: 28,
      currentApp: '快手极速版',
      currentPackage: 'com.kuaishou.nebula',
      iconType: 'kuaishou',
      resolution: '1200 × 2670',
      transportSpeed: 'USB 3.2 Gen1',
    },
  },
  {
    serial: '192.168.1.188:5555',
    state: 'device',
    source: 'manual',
    alias: '客厅 Galaxy S24',
    telemetry: {
      brand: 'Samsung',
      manufacturer: 'Samsung',
      model: 'SM-S9280 (Galaxy S24 Ultra)',
      androidRelease: '14',
      batteryLevel: 18,
      batteryCharging: false,
      batteryTempCelsius: 39.4,
      cpuTempSupported: false,
      cpuUsagePercent: 35,
      currentApp: '微信',
      currentPackage: 'com.tencent.mm',
      iconType: 'wechat',
      resolution: '1440 × 3120',
      transportSpeed: 'Wi-Fi 6',
    },
  },
  {
    serial: 'Pixel_8_Pro_Dev',
    state: 'device',
    source: 'auto',
    alias: '',
    telemetry: {
      brand: 'Google',
      manufacturer: 'Google',
      model: 'Pixel 8 Pro',
      androidRelease: '15',
      batteryLevel: 58,
      batteryCharging: false,
      batteryTempCelsius: 36.2,
      cpuTempCelsius: 44.0,
      cpuTempSupported: true,
      cpuUsagePercent: 22,
      currentApp: '系统设置',
      currentPackage: 'com.android.settings',
      iconType: 'settings',
      resolution: '1344 × 2992',
      transportSpeed: 'USB 3.0',
    },
  },
  {
    serial: 'OnePlus_12_Game',
    state: 'device',
    source: 'auto',
    alias: '一加 12 性能机',
    telemetry: {
      brand: 'OnePlus',
      manufacturer: 'OnePlus',
      model: 'PJD110 (OnePlus 12)',
      androidRelease: '14',
      batteryLevel: 8,
      batteryCharging: false,
      batteryTempCelsius: 43.8,
      cpuTempCelsius: 61.2,
      cpuTempSupported: true,
      cpuUsagePercent: 88,
      currentApp: '哔哩哔哩',
      currentPackage: 'tv.danmaku.bili',
      iconType: 'bilibili',
      resolution: '1440 × 3168',
      transportSpeed: 'USB 3.0',
    },
  },
  {
    serial: 'Vivo_X100_Camera',
    state: 'device',
    source: 'auto',
    alias: '',
    telemetry: {
      brand: 'vivo',
      manufacturer: 'vivo',
      model: 'V2309A (X100 Pro)',
      androidRelease: '14',
      batteryLevel: 94,
      batteryCharging: true,
      batteryTempCelsius: 32.5,
      cpuTempCelsius: 38.0,
      cpuTempSupported: true,
      cpuUsagePercent: 15,
      currentApp: '网易云音乐',
      currentPackage: 'com.netease.cloudmusic',
      iconType: 'neteasemusic',
      resolution: '1260 × 2800',
      transportSpeed: 'USB 3.2',
    },
  },
  {
    serial: '192.168.1.205:5555',
    state: 'offline',
    source: 'manual',
    alias: '备用离线机',
    telemetry: {
      brand: 'Xiaomi',
      manufacturer: 'Xiaomi',
      model: 'Redmi K60',
      androidRelease: '13',
      batteryLevel: 0,
      batteryCharging: false,
      batteryTempCelsius: 0,
      cpuTempSupported: false,
      cpuUsagePercent: 0,
      currentApp: '待机',
      currentPackage: '',
      iconType: 'standby',
      resolution: '1440 × 3200',
    },
  },
]
