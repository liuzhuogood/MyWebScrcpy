import { spawn } from 'node:child_process'
import { once } from 'node:events'
import { mkdtemp, readFile, rm } from 'node:fs/promises'
import os from 'node:os'
import path from 'node:path'
import assert from 'node:assert/strict'

const binary = process.argv[2]
assert.ok(binary, 'Usage: node scripts/verification/player-windows.mjs <native-verify GUI binary>')
const directory = await mkdtemp(path.join(os.tmpdir(), 'mywebscrcpy-players-'))
const report = process.argv[3] ? path.resolve(process.argv[3]) : path.join(directory, 'report.json')
// A previous successful report must never satisfy this run's native watchdog.
await rm(report, { force: true })
const child = spawn(path.resolve(binary), [], {
  env: {
    ...process.env,
    MYWEBSCRCPY_NATIVE_REPORT: report,
    MYWEBSCRCPY_VERIFY_PLAYERS: '1',
    MYWEBSCRCPY_DESKTOP_PORT: '21989',
  },
  stdio: ['ignore', 'ignore', 'pipe'],
})
let errors = ''
child.stderr.on('data', data => { errors = (errors + data).slice(-8192) })
const timer = setTimeout(() => child.kill(), 80000)
try {
  const [code] = await once(child, 'exit')
  assert.equal(code, 0, errors)
  const result = JSON.parse(await readFile(report, 'utf8')).player_windows
  for (const key of ['ok', 'mainProtected', 'zeroAndHalfOpacity', 'transparentBody', 'clickThroughIPC', 'rangeGuard', 'independentRecovery', 'reopenRecovery', 'closeReopen', 'menuRecovery'])
    assert.equal(result[key], true, `${key}: ${JSON.stringify(result)}\n${errors}`)
  if (process.env.MYWEBSCRCPY_VERIFY_HOTKEY) assert.equal(result.globalHotkey, true, JSON.stringify(result))
  console.log('PASS: native player IPC, 0%/50% rendering, range guard, main-window guard, independent recovery and reopening')
  console.log(JSON.stringify(result))
}
finally {
  clearTimeout(timer)
  if (child.exitCode === null) child.kill()
  await rm(directory, { recursive: true, force: true })
}
