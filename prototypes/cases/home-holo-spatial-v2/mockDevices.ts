export interface DeviceSpatialTelemetry {
  fps: number
  tempCelsius: number
  batteryPercent: number
  batteryCharging: boolean
  latencyMs: number
  resolution: string
  androidVersion: string
  currentApp: string
  currentPackage: string
  ip?: string
  transportSpeed?: string
  aspectRatio: string
}

export interface SpatialDevice {
  serial: string
  name: string
  model: string
  source: 'auto' | 'manual'
  state: 'device' | 'connecting' | 'unauthorized' | 'offline'
  telemetry: DeviceSpatialTelemetry
}

export const INITIAL_SPATIAL_DEVICES: SpatialDevice[] = [
  {
    serial: '192.168.1.108:5555',
    name: '小米 14 Ultra (摄影旗舰)',
    model: 'Xiaomi 14 Ultra',
    source: 'manual',
    state: 'device',
    telemetry: {
      fps: 120,
      tempCelsius: 33.8,
      batteryPercent: 88,
      batteryCharging: true,
      latencyMs: 11,
      resolution: '1440 × 3200',
      androidVersion: 'Android 14',
      currentApp: '快手极速版',
      currentPackage: 'com.kuaishou.nebula',
      ip: '192.168.1.108',
      aspectRatio: '20:9',
    },
  },
  {
    serial: 'R5CR20XXXXX',
    name: '三星 S24 Ultra (AI 生产力)',
    model: 'Samsung Galaxy S24 Ultra',
    source: 'auto',
    state: 'device',
    telemetry: {
      fps: 60,
      tempCelsius: 31.4,
      batteryPercent: 95,
      batteryCharging: false,
      latencyMs: 4,
      resolution: '1440 × 3120',
      androidVersion: 'Android 14',
      currentApp: 'TikTok Global',
      currentPackage: 'com.zhiliaoapp.musically',
      transportSpeed: 'USB 3.2 Gen2',
      aspectRatio: '19.5:9',
    },
  },
  {
    serial: 'emulator-5554',
    name: 'Pixel 8 Pro (云端测试机)',
    model: 'Google Pixel 8 Pro',
    source: 'auto',
    state: 'device',
    telemetry: {
      fps: 60,
      tempCelsius: 28.0,
      batteryPercent: 100,
      batteryCharging: true,
      latencyMs: 1,
      resolution: '1344 × 2992',
      androidVersion: 'Android 15',
      currentApp: '自动化用例套件',
      currentPackage: 'com.android.test',
      transportSpeed: 'PCIe 虚拟直通',
      aspectRatio: '20:9',
    },
  },
  {
    serial: '192.168.1.115:5555',
    name: '一加 12 (高性能测试组)',
    model: 'OnePlus 12',
    source: 'manual',
    state: 'device',
    telemetry: {
      fps: 90,
      tempCelsius: 37.5,
      batteryPercent: 54,
      batteryCharging: false,
      latencyMs: 16,
      resolution: '1440 × 3168',
      androidVersion: 'Android 14',
      currentApp: '微信',
      currentPackage: 'com.tencent.mm',
      ip: '192.168.1.115',
      aspectRatio: '19.8:9',
    },
  },
  {
    serial: '09151FDD40003R',
    name: 'Pixel 7 (离线备用)',
    model: 'Google Pixel 7',
    source: 'auto',
    state: 'offline',
    telemetry: {
      fps: 0,
      tempCelsius: 24.5,
      batteryPercent: 42,
      batteryCharging: false,
      latencyMs: 0,
      resolution: '1080 × 2400',
      androidVersion: 'Android 13',
      currentApp: '未运行',
      currentPackage: 'none',
      aspectRatio: '20:9',
    },
  },
  {
    serial: '192.168.1.120:5555',
    name: 'Vivo X100 Pro (等待授权)',
    model: 'vivo X100 Pro',
    source: 'manual',
    state: 'unauthorized',
    telemetry: {
      fps: 0,
      tempCelsius: 30.2,
      batteryPercent: 76,
      batteryCharging: true,
      latencyMs: 22,
      resolution: '1260 × 2800',
      androidVersion: 'Android 14',
      currentApp: 'RSA 授权弹窗',
      currentPackage: 'com.android.systemui',
      ip: '192.168.1.120',
      aspectRatio: '20:9',
    },
  },
]
