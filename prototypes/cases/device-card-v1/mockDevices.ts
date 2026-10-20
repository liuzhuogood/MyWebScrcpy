export interface DeviceTelemetry {
  // 核心硬件与系统 (只读/静态)
  brand: string // ro.product.brand (如 Xiaomi, Samsung, Google)
  manufacturer: string // ro.product.manufacturer
  model: string // ro.product.model (如 23127PN0CC)
  androidRelease: string // ro.build.version.release (如 14)
  sdkVersion: number // ro.build.version.sdk (如 34)
  cpuAbi: string // ro.product.cpu.abi (如 arm64-v8a)
  buildDisplayId?: string // ro.build.display.id

  // 实时状态与遥测 (动态)
  batteryLevel: number // dumpsys battery level (0-100)
  batteryCharging: boolean // 充电中
  batteryTempCelsius: number // dumpsys battery temp (如 36.5)
  cpuTempCelsius?: number // thermal zone temp (如 42.0)
  cpuTempSupported: boolean // 是否开放/支持获取 CPU 温度
  cpuUsagePercent: number // 0-100
  ramUsedGb: number
  ramTotalGb: number

  // 前台与网络
  currentApp: string
  currentPackage: string
  resolution: string
  ip?: string
  transportSpeed?: string
}

export interface PrototypeDevice {
  serial: string
  state: 'device' | 'connecting' | 'unauthorized' | 'offline'
  source: 'auto' | 'manual'
  alias?: string // 用户自定义别名，若空则卡片默认展示 brand
  telemetry: DeviceTelemetry
}

export const MOCK_DEVICES: PrototypeDevice[] = [
  {
    serial: '23127PN0CC_USB',
    state: 'device',
    source: 'auto',
    alias: '', // 留空，默认显示品牌名 Xiaomi
    telemetry: {
      brand: 'Xiaomi',
      manufacturer: 'Xiaomi',
      model: '23127PN0CC (小米 14)',
      androidRelease: '14',
      sdkVersion: 34,
      cpuAbi: 'arm64-v8a',
      buildDisplayId: 'UKQ1.230804.001',
      batteryLevel: 85,
      batteryCharging: true, // 充电中 -> 绿色
      batteryTempCelsius: 35.8, // 常温 (<38°C) -> 绿色良好
      cpuTempCelsius: 41.5, // CPU 常温 (<48°C) -> 绿色常温
      cpuTempSupported: true,
      cpuUsagePercent: 28,
      ramUsedGb: 6.2,
      ramTotalGb: 12,
      currentApp: '快手极速版',
      currentPackage: 'com.kuaishou.nebula',
      resolution: '1200 × 2670',
      transportSpeed: 'USB 3.2 Gen1',
    },
  },
  {
    serial: '192.168.1.188:5555',
    state: 'device',
    source: 'manual',
    alias: '客厅测试机',
    telemetry: {
      brand: 'Samsung',
      manufacturer: 'samsung',
      model: 'SM-S9280 (Galaxy S24 Ultra)',
      androidRelease: '14',
      sdkVersion: 34,
      cpuAbi: 'arm64-v8a',
      buildDisplayId: 'UP1A.231005.007',
      batteryLevel: 18, // 低电量 (<=20%) -> 琥珀黄预警！
      batteryCharging: false,
      batteryTempCelsius: 39.2, // 温升 (38~42°C) -> 琥珀黄预警！
      cpuTempSupported: false, // 模拟未开放 CPU 温度
      cpuUsagePercent: 35,
      ramUsedGb: 5.8,
      ramTotalGb: 12,
      currentApp: '系统桌面',
      currentPackage: 'com.sec.android.app.launcher',
      resolution: '1440 × 3120',
      ip: '192.168.1.188',
    },
  },
  {
    serial: 'Pixel_8_Pro_Dev',
    state: 'device',
    source: 'auto',
    alias: '', // 留空，默认显示品牌名 Google
    telemetry: {
      brand: 'Google',
      manufacturer: 'Google',
      model: 'Pixel 8 Pro',
      androidRelease: '15',
      sdkVersion: 35,
      cpuAbi: 'arm64-v8a',
      buildDisplayId: 'AP2A.240805.005',
      batteryLevel: 8, // 极低电量 (<=10%) -> 警戒红呼吸闪烁！
      batteryCharging: false,
      batteryTempCelsius: 43.5, // 高热过热 (>=42°C) -> 警戒红警报！
      cpuTempCelsius: 59.8, // CPU 警戒 (>=58°C) -> 警戒红！
      cpuTempSupported: true,
      cpuUsagePercent: 88, // 高负载
      ramUsedGb: 10.2,
      ramTotalGb: 12,
      currentApp: '微信视频通话',
      currentPackage: 'com.tencent.mm',
      resolution: '1344 × 2992',
      transportSpeed: 'USB 3.0',
    },
  },
  {
    serial: 'emulator-5554',
    state: 'device',
    source: 'auto',
    alias: '',
    telemetry: {
      brand: 'Android Emulator',
      manufacturer: 'Google',
      model: 'sdk_gphone64_x86_64',
      androidRelease: '13',
      sdkVersion: 33,
      cpuAbi: 'x86_64',
      buildDisplayId: 'TE1A.220922.010',
      batteryLevel: 100, // 充满电
      batteryCharging: true,
      batteryTempCelsius: 28.0,
      cpuTempSupported: false,
      cpuUsagePercent: 8,
      ramUsedGb: 2.1,
      ramTotalGb: 4,
      currentApp: '控制台主页',
      currentPackage: 'com.mywebscrcpy.desktop',
      resolution: '1080 × 2400',
    },
  },
  {
    serial: '192.168.1.199:5555',
    state: 'offline',
    source: 'manual',
    alias: '',
    telemetry: {
      brand: 'OnePlus',
      manufacturer: 'OnePlus',
      model: 'PJZ110 (一加 12)',
      androidRelease: '14',
      sdkVersion: 34,
      cpuAbi: 'arm64-v8a',
      batteryLevel: 0,
      batteryCharging: false,
      batteryTempCelsius: 0,
      cpuTempSupported: false,
      cpuUsagePercent: 0,
      ramUsedGb: 0,
      ramTotalGb: 16,
      currentApp: '待机离线',
      currentPackage: 'offline',
      resolution: '1440 × 3168',
      ip: '192.168.1.199',
    },
  },
]
