interface ScreenState {
  screenOn: boolean | null
  error?: string
}

interface CommandResult {
  status: 'queued' | 'running' | 'completed' | 'failed' | 'timed_out'
  exit_code?: number
  error?: string
}

function ensureActive(signal: AbortSignal): void {
  if (signal.aborted)
    throw new DOMException('请求已取消', 'AbortError')
}

async function screenIsOn(serial: string, signal: AbortSignal): Promise<boolean> {
  ensureActive(signal)
  const response = await fetch(`/api/screen-state?serial=${encodeURIComponent(serial)}`, { signal })
  if (!response.ok)
    throw new Error(`读取屏幕状态失败（${response.status}）`)
  const data: ScreenState = await response.json()
  if (typeof data.screenOn !== 'boolean' || data.error)
    throw new Error('无法确认设备屏幕状态，请稍后重试')
  return data.screenOn
}

async function adbCommand(serial: string, args: string[], signal: AbortSignal): Promise<void> {
  ensureActive(signal)
  const response = await fetch(`/api/adb/commands?serial=${encodeURIComponent(serial)}`, {
    method: 'POST',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify({ args, timeout_ms: 5000, async: false }),
    signal,
  })
  if (!response.ok)
    throw new Error(`设备命令执行失败（${response.status}）`)
  const result: CommandResult = await response.json()
  if (result.status !== 'completed' || result.exit_code !== 0) {
    console.warn('[wake-unlock] ADB command failed', { serial, args, status: result.status, error: result.error })
    throw new Error('设备命令未完成，请稍后重试')
  }
}

function waitForNextCheck(signal: AbortSignal): Promise<void> {
  ensureActive(signal)
  return new Promise((resolve, reject) => {
    const onAbort = () => {
      clearTimeout(timer)
      reject(new DOMException('请求已取消', 'AbortError'))
    }
    const timer = setTimeout(() => {
      signal.removeEventListener('abort', onAbort)
      resolve()
    }, 250)
    signal.addEventListener('abort', onAbort, { once: true })
  })
}

/** 保留电源键原有切换行为，仅在已确认熄屏时使用 ADB 唤醒并尝试解锁。 */
export async function pressPowerWithUnlock(
  serial: string,
  autoUnlock: boolean,
  sendPowerKey: () => void,
  signal: AbortSignal,
): Promise<boolean> {
  ensureActive(signal)
  if (!autoUnlock || await screenIsOn(serial, signal)) {
    ensureActive(signal)
    sendPowerKey()
    return false
  }

  console.info('[wake-unlock] Waking device', { serial })
  await adbCommand(serial, ['shell', 'input', 'keyevent', 'KEYCODE_WAKEUP'], signal)
  for (let attempt = 0; attempt < 12; attempt++) {
    if (await screenIsOn(serial, signal)) {
      console.info('[wake-unlock] Requesting keyguard dismissal', { serial })
      await adbCommand(serial, ['shell', 'wm', 'dismiss-keyguard'], signal)
      // 命令完成只代表已请求解除锁屏，不代表密码锁已通过验证。
      console.info('[wake-unlock] Keyguard dismissal requested', { serial })
      return true
    }
    if (attempt < 11)
      await waitForNextCheck(signal)
  }
  throw new Error('屏幕尚未点亮，未尝试解锁，请稍后重试')
}
