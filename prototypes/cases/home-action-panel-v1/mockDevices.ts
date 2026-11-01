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
  screenOn?: boolean
  screenLocked?: boolean
}

export interface MockDevice {
  serial: string
  state: 'device' | 'connecting' | 'unauthorized' | 'offline'
  source: 'auto' | 'manual'
  alias?: string
  telemetry: DeviceTelemetry
}

export const MOCK_DEVICES: MockDevice[] = [
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
      batteryLevel: 86,
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
      screenOn: true,
      screenLocked: false,
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
      batteryLevel: 62,
      batteryCharging: false,
      batteryTempCelsius: 31.2,
      cpuTempCelsius: 36.4,
      cpuTempSupported: true,
      cpuUsagePercent: 12,
      currentApp: '微信',
      currentPackage: 'com.tencent.mm',
      iconType: 'wechat',
      resolution: '1440 × 3120',
      transportSpeed: 'Wi-Fi 6',
      screenOn: true,
      screenLocked: false,
    },
  },
  {
    serial: 'R58M12ABCDE_USB',
    state: 'device',
    source: 'auto',
    alias: '测试机 Pixel 8',
    telemetry: {
      brand: 'Google',
      manufacturer: 'Google',
      model: 'Pixel 8',
      androidRelease: '14',
      batteryLevel: 45,
      batteryCharging: false,
      batteryTempCelsius: 34.0,
      cpuTempCelsius: 38.0,
      cpuTempSupported: true,
      cpuUsagePercent: 18,
      currentApp: '抖音',
      currentPackage: 'com.ss.android.ugc.aweme',
      iconType: 'douyin',
      resolution: '1080 × 2400',
      transportSpeed: 'USB 2.0',
      screenOn: true,
      screenLocked: false,
    },
  },
  {
    serial: '192.168.1.202:5555',
    state: 'device',
    source: 'manual',
    alias: '卧室内测机 一加 12',
    telemetry: {
      brand: 'OnePlus',
      manufacturer: 'OnePlus',
      model: 'PJD110 (一加 12)',
      androidRelease: '14',
      batteryLevel: 100,
      batteryCharging: true,
      batteryTempCelsius: 29.5,
      cpuTempCelsius: 33.1,
      cpuTempSupported: true,
      cpuUsagePercent: 8,
      currentApp: '已休眠锁屏',
      currentPackage: 'com.android.systemui',
      iconType: 'system',
      resolution: '1440 × 3168',
      transportSpeed: 'Wi-Fi 6E',
      screenOn: false,
      screenLocked: true,
    },
  },
  {
    serial: 'V2309A_USB',
    state: 'device',
    source: 'auto',
    alias: '备用机 vivo X100',
    telemetry: {
      brand: 'vivo',
      manufacturer: 'vivo',
      model: 'V2309A (vivo X100)',
      androidRelease: '14',
      batteryLevel: 78,
      batteryCharging: false,
      batteryTempCelsius: 33.6,
      cpuTempCelsius: 37.2,
      cpuTempSupported: true,
      cpuUsagePercent: 15,
      currentApp: '小红书',
      currentPackage: 'com.xingin.xhs',
      iconType: 'xiaohongshu',
      resolution: '1260 × 2800',
      transportSpeed: 'USB 3.0',
      screenOn: true,
      screenLocked: false,
    },
  },
  {
    serial: '192.168.1.99:5555',
    state: 'offline',
    source: 'manual',
    alias: '离线测试机 红米 K70',
    telemetry: {
      brand: 'Redmi',
      manufacturer: 'Xiaomi',
      model: '2311DRK48C (Redmi K70)',
      androidRelease: '14',
      batteryLevel: 0,
      batteryCharging: false,
      batteryTempCelsius: 0,
      cpuTempSupported: false,
      cpuUsagePercent: 0,
      currentApp: '离线',
      currentPackage: '',
      iconType: 'offline',
      resolution: '1440 × 3200',
      screenOn: false,
      screenLocked: false,
    },
  },
]
