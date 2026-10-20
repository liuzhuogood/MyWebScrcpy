export interface DeviceTelemetry {
  fps: number
  tempCelsius: number
  batteryPercent: number
  batteryCharging: boolean
  latencyMs: number
  memoryUsageMb: number
  totalMemoryMb: number
  resolution: string
  androidVersion: string
  currentApp: string
  currentPackage: string
  ip?: string
  wifiSsid?: string
  transportSpeed?: string
}

export interface CyberDevice {
  serial: string
  name: string
  model: string
  source: 'auto' | 'manual'
  state: 'device' | 'connecting' | 'unauthorized' | 'offline'
  telemetry: DeviceTelemetry
}

export const INITIAL_CYBER_DEVICES: CyberDevice[] = [
  {
    serial: '192.168.1.108:5555',
    name: '主力测试机 · 小米 14 Ultra',
    model: 'Xiaomi 14 Ultra',
    source: 'manual',
    state: 'device',
    telemetry: {
      fps: 120,
      tempCelsius: 34.2,
      batteryPercent: 88,
      batteryCharging: true,
      latencyMs: 11,
      memoryUsageMb: 6140,
      totalMemoryMb: 16384,
      resolution: '1440 × 3200',
      androidVersion: 'Android 14 (HyperOS 1.0)',
      currentApp: '快手极速版',
      currentPackage: 'com.kuaishou.nebula',
      ip: '192.168.1.108',
      wifiSsid: 'MATRIX_LAB_5G',
    },
  },
  {
    serial: 'R5CR20XXXXX',
    name: '海外运营机 · 三星 S24 Ultra',
    model: 'Samsung Galaxy S24 Ultra',
    source: 'auto',
    state: 'device',
    telemetry: {
      fps: 60,
      tempCelsius: 31.8,
      batteryPercent: 95,
      batteryCharging: false,
      latencyMs: 4,
      memoryUsageMb: 5210,
      totalMemoryMb: 12288,
      resolution: '1440 × 3120',
      androidVersion: 'Android 14 (One UI 6.1)',
      currentApp: 'TikTok',
      currentPackage: 'com.zhiliaoapp.musically',
      transportSpeed: 'USB 3.2 Gen2 (10 Gbps)',
    },
  },
  {
    serial: 'emulator-5554',
    name: '云端开发模拟器 · Pixel 8 Pro',
    model: 'Google Pixel 8 Pro',
    source: 'auto',
    state: 'device',
    telemetry: {
      fps: 60,
      tempCelsius: 28.5,
      batteryPercent: 100,
      batteryCharging: true,
      latencyMs: 1,
      memoryUsageMb: 3420,
      totalMemoryMb: 8192,
      resolution: '1344 × 2992',
      androidVersion: 'Android 15 (VanillaIceCream)',
      currentApp: '自动化测试套件',
      currentPackage: 'com.android.test.runner',
      transportSpeed: 'PCIe 虚拟直通通道',
    },
  },
  {
    serial: '192.168.1.115:5555',
    name: '自动化挂机 · 一加 12',
    model: 'OnePlus 12',
    source: 'manual',
    state: 'device',
    telemetry: {
      fps: 90,
      tempCelsius: 38.1,
      batteryPercent: 54,
      batteryCharging: false,
      latencyMs: 18,
      memoryUsageMb: 7890,
      totalMemoryMb: 16384,
      resolution: '1440 × 3168',
      androidVersion: 'Android 14 (OxygenOS 14)',
      currentApp: '微信',
      currentPackage: 'com.tencent.mm',
      ip: '192.168.1.115',
      wifiSsid: 'MATRIX_LAB_5G',
    },
  },
  {
    serial: '09151FDD40003R',
    name: '离线备用机 · Pixel 7',
    model: 'Google Pixel 7',
    source: 'auto',
    state: 'offline',
    telemetry: {
      fps: 0,
      tempCelsius: 24.0,
      batteryPercent: 42,
      batteryCharging: false,
      latencyMs: 0,
      memoryUsageMb: 0,
      totalMemoryMb: 8192,
      resolution: '1080 × 2400',
      androidVersion: 'Android 13',
      currentApp: '休眠待机',
      currentPackage: 'android.os.idle',
    },
  },
  {
    serial: '192.168.1.120:5555',
    name: '待授权设备 · Vivo X100 Pro',
    model: 'vivo X100 Pro',
    source: 'manual',
    state: 'unauthorized',
    telemetry: {
      fps: 0,
      tempCelsius: 30.5,
      batteryPercent: 76,
      batteryCharging: true,
      latencyMs: 25,
      memoryUsageMb: 0,
      totalMemoryMb: 16384,
      resolution: '1260 × 2800',
      androidVersion: 'Android 14 (OriginOS 4)',
      currentApp: 'USB 调试指纹确认',
      currentPackage: 'com.android.systemui',
      ip: '192.168.1.120',
      wifiSsid: 'MATRIX_LAB_2.4G',
    },
  },
]
