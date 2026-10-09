import type { Device } from '@/api/devices'

/** 无后端演示用的假设备，与真实 Device 契约一致 */
export const MOCK_DEVICES: Device[] = [
  { serial: '10.0.0.30:5555', state: 'device', model: 'Pixel 8', product: 'husky', device: 'husky', transport_id: '1' },
  { serial: '10.0.0.31:5555', state: 'device', model: 'Xiaomi 14', product: 'houji', device: 'houji', transport_id: '2' },
  { serial: 'emulator-5554', state: 'device', model: 'sdk_gphone64_arm64', product: 'sdk_gphone64_arm64', device: 'emu64a', transport_id: '3' },
  { serial: 'RF8M33XXXXX', state: 'device', model: 'Galaxy S24', product: 'dm3q', device: 'dm3q', transport_id: '4' },
  { serial: '192.168.1.50:5555', state: 'offline', model: 'OnePlus 12', product: 'waffle', device: 'waffle', transport_id: '5' },
]
