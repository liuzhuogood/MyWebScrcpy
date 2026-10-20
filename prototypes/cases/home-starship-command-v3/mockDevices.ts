export interface FleetDevice {
  serial: string
  name: string
  model: string
  source: 'auto' | 'manual'
  state: 'device' | 'connecting' | 'unauthorized' | 'offline'
  fps: number
  tempCelsius: number
  batteryPercent: number
  batteryCharging: boolean
  latencyMs: number
  resolution: string
  dpi: number
  androidVersion: string
  currentApp: string
  currentPackage: string
  currentActivity: string
  ip?: string
  streamBitrateMbps: number
}

export const INITIAL_FLEET_DEVICES: FleetDevice[] = [
  {
    serial: '192.168.1.108:5555',
    name: '战术领航节点 · 小米 14 Ultra',
    model: 'Xiaomi 14 Ultra',
    source: 'manual',
    state: 'device',
    fps: 120,
    tempCelsius: 34.2,
    batteryPercent: 88,
    batteryCharging: true,
    latencyMs: 11,
    resolution: '1440 × 3200',
    dpi: 560,
    androidVersion: 'Android 14 (HyperOS 1.0)',
    currentApp: '快手极速版',
    currentPackage: 'com.kuaishou.nebula',
    currentActivity: 'com.yxcorp.gifshow.HomeActivity',
    ip: '192.168.1.108',
    streamBitrateMbps: 8.5,
  },
  {
    serial: 'R5CR20XXXXX',
    name: '战术二号阵位 · 三星 S24 Ultra',
    model: 'Samsung Galaxy S24 Ultra',
    source: 'auto',
    state: 'device',
    fps: 60,
    tempCelsius: 31.8,
    batteryPercent: 95,
    batteryCharging: false,
    latencyMs: 4,
    resolution: '1440 × 3120',
    dpi: 500,
    androidVersion: 'Android 14 (One UI 6.1)',
    currentApp: 'TikTok Global',
    currentPackage: 'com.zhiliaoapp.musically',
    currentActivity: 'com.ss.android.ugc.aweme.splash.SplashActivity',
    streamBitrateMbps: 12.0,
  },
  {
    serial: 'emulator-5554',
    name: '星际模拟矩阵 · Pixel 8 Pro',
    model: 'Google Pixel 8 Pro',
    source: 'auto',
    state: 'device',
    fps: 60,
    tempCelsius: 28.5,
    batteryPercent: 100,
    batteryCharging: true,
    latencyMs: 1,
    resolution: '1344 × 2992',
    dpi: 480,
    androidVersion: 'Android 15 (VanillaIceCream)',
    currentApp: '自动化测试引擎',
    currentPackage: 'com.android.test.runner',
    currentActivity: 'android.app.Activity',
    streamBitrateMbps: 16.0,
  },
  {
    serial: '192.168.1.115:5555',
    name: '挂机集群三号 · 一加 12',
    model: 'OnePlus 12',
    source: 'manual',
    state: 'device',
    fps: 90,
    tempCelsius: 38.1,
    batteryPercent: 54,
    batteryCharging: false,
    latencyMs: 18,
    resolution: '1440 × 3168',
    dpi: 510,
    androidVersion: 'Android 14 (OxygenOS 14)',
    currentApp: '微信社交终端',
    currentPackage: 'com.tencent.mm',
    currentActivity: 'com.tencent.mm.ui.LauncherUI',
    ip: '192.168.1.115',
    streamBitrateMbps: 6.2,
  },
  {
    serial: '09151FDD40003R',
    name: '离线备用舱 · Pixel 7',
    model: 'Google Pixel 7',
    source: 'auto',
    state: 'offline',
    fps: 0,
    tempCelsius: 24.0,
    batteryPercent: 42,
    batteryCharging: false,
    latencyMs: 0,
    resolution: '1080 × 2400',
    dpi: 420,
    androidVersion: 'Android 13',
    currentApp: '系统休眠',
    currentPackage: 'none',
    currentActivity: 'none',
    streamBitrateMbps: 0,
  },
  {
    serial: '192.168.1.120:5555',
    name: '待入列战位 · Vivo X100 Pro',
    model: 'vivo X100 Pro',
    source: 'manual',
    state: 'unauthorized',
    fps: 0,
    tempCelsius: 30.5,
    batteryPercent: 76,
    batteryCharging: true,
    latencyMs: 25,
    resolution: '1260 × 2800',
    dpi: 450,
    androidVersion: 'Android 14',
    currentApp: 'RSA 授权等待',
    currentPackage: 'com.android.systemui',
    currentActivity: 'com.android.systemui.usb.UsbDebuggingActivity',
    ip: '192.168.1.120',
    streamBitrateMbps: 0,
  },
]
