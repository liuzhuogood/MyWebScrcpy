export interface LogLine { seq: number; time: string; stream: 'stdout' | 'stderr' | 'system'; message: string }
export interface DeviceRun {
  id: string; device: string; serial: string; status: string; duration: string;
  logs: LogLine[]; recording: 'completed' | 'disabled' | 'failed' | 'expired' | 'recording';
  report: 'completed' | 'disabled' | 'failed' | 'pending';
}
export interface Batch { id: string; time: string; trigger: string; status: string; runs: DeviceRun[] }
const lines: LogLine[] = [
  { seq: 1, time: '09:00:00.012', stream: 'system', message: '任务启动，录制已开始' },
  { seq: 2, time: '09:00:00.036', stream: 'stdout', message: '步骤 1：唤醒设备' },
  { seq: 3, time: '09:00:02.180', stream: 'stdout', message: '步骤 2：打开应用' },
  { seq: 4, time: '09:00:06.412', stream: 'stdout', message: '步骤 3：等待签到按钮，重试 1/3' },
  { seq: 5, time: '09:00:08.504', stream: 'stdout', message: '步骤 3：找到签到按钮，点击' },
  { seq: 6, time: '09:00:09.136', stream: 'stdout', message: '步骤 4：签到成功' },
  { seq: 7, time: '09:00:12.015', stream: 'system', message: '进程已退出，退出码 0；录制已完成' },
]
export const batches: Batch[] = [
  { id: 'dispatch-12', time: '10月12日 09:00', trigger: '定时', status: '部分异常', runs: [
    { id: 'run-pixel-12', device: 'Pixel 8', serial: '3A281F', status: '已完成', duration: '12 秒', logs: lines, recording: 'completed', report: 'completed' },
    { id: 'run-redmi-12', device: 'Redmi K70', serial: '82ACF4', status: '失败', duration: '120 秒', logs: [
      { seq: 1, time: '09:00:00.018', stream: 'system', message: '任务启动，录制失败：设备已有录制' },
      { seq: 2, time: '09:00:02.330', stream: 'stdout', message: '步骤 2：打开应用' },
      { seq: 3, time: '09:01:58.103', stream: 'stderr', message: '等待签到按钮超时' },
      { seq: 4, time: '09:02:00.015', stream: 'system', message: '任务超时，进程已终止' },
    ], recording: 'failed', report: 'failed' },
  ] },
  { id: 'dispatch-11', time: '10月11日 09:00', trigger: '定时', status: '已完成', runs: [
    { id: 'run-pixel-11', device: 'Pixel 8', serial: '3A281F', status: '已完成', duration: '15 秒', logs: lines, recording: 'disabled', report: 'disabled' },
  ] },
  { id: 'dispatch-05', time: '10月05日 09:00', trigger: '手动', status: '已跳过', runs: [
    { id: 'run-pixel-05', device: 'Pixel 8', serial: '3A281F', status: '已跳过', duration: '—', logs: [{ seq: 1, time: '09:00:00.012', stream: 'system', message: '目标设备离线，未启动进程' }], recording: 'disabled', report: 'completed' },
  ] },
]
export const documentSource = `---
version: 1.0.0
schedule: "0 9 * * *"
timeout_sec: 120
enabled: true
recording:
  enabled: true
  record_audio: false
report:
  enabled: true
  prompt: |
    总结每台设备的签到结果、失败原因和重试情况。
    结论引用日志行号；无法确认时说明证据不足。
---

# 每日签到

## 步骤
1. 唤醒设备
2. 打开应用
3. 等待签到按钮，最多重试 3 次
4. 点击签到，检查结果
`
