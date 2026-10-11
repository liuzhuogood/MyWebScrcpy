import type { Device } from '@/api/devices'
import type { TemplateItem } from '@/api/templates'

export interface DemoTask {
  id: number
  name: string
  target_devices: string[]
  content: string
  version: string
  schedule: string
  timeout: number
  enabled: boolean
  nextRun: string
  lastResult: string
  compiledPython: string
  compiledContent: string
}

export const devices: Device[] = [
  { serial: 'demo-pixel-01', alias: 'Pixel 8', state: 'device' },
  { serial: 'demo-redmi-02', alias: 'Redmi K70', state: 'device' },
  { serial: 'demo-galaxy-03', alias: 'Galaxy S24', state: 'offline' },
]

export const templates: TemplateItem[] = [
  { id: 'common-confirm', name: '确认按钮', serial: 'global', threshold: 0.85, enabled: true },
  { id: 'common-back', name: '返回按钮', serial: 'global', threshold: 0.90, enabled: true },
  { id: 'pixel-home', name: '应用入口', serial: 'demo-pixel-01', threshold: 0.88, enabled: true },
  { id: 'redmi-home', name: '应用入口', serial: 'demo-redmi-02', threshold: 0.88, enabled: true },
  { id: 'galaxy-home', name: '应用入口', serial: 'demo-galaxy-03', threshold: 0.88, enabled: true },
]

export const initialContent = `---
version: 1.0.0
schedule: "0 9 * * *"
timeout_sec: 120
enabled: true
---

# 每日应用巡检

## 启动应用
- 唤醒设备
- 打开目标应用
- 等待 3 秒

## 确认页面
- 检查页面是否加载完成
![确认按钮](template://common-confirm?threshold=0.85)
- 保存执行结果
`

export const initialTasks: DemoTask[] = [
  { id: 1, name: '每日应用巡检', target_devices: ['demo-pixel-01', 'demo-redmi-02'], content: initialContent, version: '1.0.0', schedule: '每天 09:00', timeout: 120, enabled: true, nextRun: '明天 09:00', lastResult: '2 台设备已完成', compiledPython: '', compiledContent: '' },
  { id: 2, name: '晚间消息检查', target_devices: ['demo-redmi-02'], content: initialContent.replaceAll('每日应用巡检', '晚间消息检查').replace('0 9 * * *', '0 20 * * *'), version: '1.0.0', schedule: '每天 20:00', timeout: 120, enabled: true, nextRun: '今天 20:00', lastResult: '已完成', compiledPython: '', compiledContent: '' },
  { id: 3, name: '页面回归检查', target_devices: ['demo-galaxy-03'], content: initialContent.replaceAll('每日应用巡检', '页面回归检查').replace('enabled: true', 'enabled: false'), version: '1.0.0', schedule: '每天 09:00', timeout: 120, enabled: false, nextRun: '未排期', lastResult: '未运行', compiledPython: '', compiledContent: '' },
]

export function deviceName(serial: string): string {
  const device = devices.find(item => item.serial === serial)
  if (!device || !device.alias) throw new Error(`原型设备配置缺失: ${serial}`)
  return device.alias
}

export function templateOwner(asset: TemplateItem): string {
  return asset.serial === 'global' ? '公共' : deviceName(asset.serial)
}
