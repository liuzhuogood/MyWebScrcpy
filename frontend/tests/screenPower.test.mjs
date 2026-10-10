import assert from 'node:assert/strict'
import test from 'node:test'
import { createWakeUnlockTrigger, pressPowerWithUnlock, requestScreenUnlock } from '../src/scrcpy/screenPower.ts'

function mockRequests(t, replies) {
  const calls = []
  t.mock.method(console, 'info', () => {})
  t.mock.method(console, 'warn', () => {})
  t.mock.method(globalThis, 'fetch', async (url, init = {}) => {
    calls.push({ url, ...init, body: init.body ? JSON.parse(init.body) : undefined })
    assert.ok(replies.length, 'unexpected request')
    const reply = replies.shift()
    if (typeof reply === 'function')
      return reply()
    return Response.json(reply.data, { status: reply.status ?? 200 })
  })
  return calls
}

const screen = screenOn => ({ data: { screenOn } })
const completed = { data: { status: 'completed', exit_code: 0 } }

test('disabled setting preserves the original power key without making requests', async (t) => {
  const calls = mockRequests(t, [])
  let presses = 0
  const result = await pressPowerWithUnlock('device:5555', false, () => presses++, new AbortController().signal)
  assert.equal(result, false)
  assert.equal(presses, 1)
  assert.equal(calls.length, 0)
})

test('an already lit screen uses the original power key and does not request unlock', async (t) => {
  const calls = mockRequests(t, [screen(true)])
  let presses = 0
  assert.equal(await pressPowerWithUnlock('device:5555', true, () => presses++, new AbortController().signal), false)
  assert.equal(presses, 1)
  assert.equal(calls.length, 1)
})

test('wakes the exact device, waits for a lit screen, then requests keyguard dismissal', async (t) => {
  const calls = mockRequests(t, [screen(false), completed, screen(false), screen(true), completed])
  assert.equal(await pressPowerWithUnlock('device:5555', true, () => assert.fail('must not toggle power'), new AbortController().signal), true)
  const commands = calls.filter(call => call.method === 'POST')
  assert.deepEqual(commands.map(call => call.body.args), [
    ['shell', 'input', 'keyevent', 'KEYCODE_WAKEUP'],
    ['shell', 'wm', 'dismiss-keyguard'],
  ])
  assert.ok(calls.every(call => call.url.endsWith('serial=device%3A5555')))
  assert.ok(commands.every(call => call.body.async === false && call.body.timeout_ms === 5000))
})

test('missing screenOn is rejected without guessing or issuing commands', async (t) => {
  const calls = mockRequests(t, [{ data: { state: 'OFF' } }])
  await assert.rejects(pressPowerWithUnlock('device', true, () => assert.fail(), new AbortController().signal), /无法确认设备屏幕状态/)
  assert.equal(calls.length, 1)
})

test('screen-state errors stop the operation even if screenOn is present', async (t) => {
  const calls = mockRequests(t, [{ data: { screenOn: false, error: 'device offline' } }])
  await assert.rejects(pressPowerWithUnlock('device', true, () => assert.fail(), new AbortController().signal), /无法确认设备屏幕状态/)
  assert.equal(calls.length, 1)
})

test('a failed wake command does not send any unlock command', async (t) => {
  const calls = mockRequests(t, [screen(false), { status: 503, data: { status: 'failed', error: 'device offline' } }])
  await assert.rejects(pressPowerWithUnlock('device', true, () => assert.fail(), new AbortController().signal), /503/)
  assert.equal(calls.length, 2)
})

test('a non-completed command in an HTTP 200 response is treated as failure', async (t) => {
  const calls = mockRequests(t, [screen(false), { data: { status: 'running' } }])
  await assert.rejects(pressPowerWithUnlock('device', true, () => assert.fail(), new AbortController().signal), /设备命令未完成/)
  assert.equal(calls.length, 2)
})

test('a failed keyguard dismissal is reported rather than claiming success', async (t) => {
  mockRequests(t, [screen(false), completed, screen(true), { data: { status: 'failed', exit_code: 1 } }])
  await assert.rejects(pressPowerWithUnlock('device', true, () => assert.fail(), new AbortController().signal), /设备命令未完成/)
})

test('a device that stays asleep never receives an unlock command', async (t) => {
  const calls = mockRequests(t, [screen(false), completed, ...Array.from({ length: 12 }, () => screen(false))])
  await assert.rejects(pressPowerWithUnlock('device', true, () => assert.fail(), new AbortController().signal), /屏幕尚未点亮/)
  assert.equal(calls.filter(call => call.method === 'POST').length, 1)
})

test('aborting after the wake request prevents subsequent screen and unlock requests', async (t) => {
  const controller = new AbortController()
  const calls = mockRequests(t, [screen(false), () => {
    controller.abort(new Error('player closed'))
    return Response.json(completed.data)
  }])
  await assert.rejects(pressPowerWithUnlock('device', true, () => assert.fail(), controller.signal), { name: 'AbortError' })
  assert.equal(calls.length, 2)
})

test('opening an already lit device requests unlock once without toggling power', async (t) => {
  const calls = mockRequests(t, [completed])
  const trigger = createWakeUnlockTrigger(() => requestScreenUnlock('device', new AbortController().signal))
  await trigger.observe(true, true)
  await trigger.observe(true, true)
  assert.equal(calls.length, 1)
  assert.deepEqual(calls[0].body.args, ['shell', 'wm', 'dismiss-keyguard'])
})

test('a screen that wakes independently gets a new unlock attempt', async () => {
  let attempts = 0
  const trigger = createWakeUnlockTrigger(async () => { attempts++ })
  await trigger.observe(false, true)
  assert.equal(attempts, 0)
  await trigger.observe(true, true)
  await trigger.observe(true, true)
  await trigger.observe(false, true)
  await trigger.observe(true, true)
  assert.equal(attempts, 2)
})

test('enabling the setting on a lit device immediately attempts unlock', async () => {
  let attempts = 0
  const trigger = createWakeUnlockTrigger(async () => { attempts++ })
  await trigger.observe(true, false)
  assert.equal(attempts, 0)
  await trigger.observe(true, true)
  assert.equal(attempts, 1)
})

test('disabled automatic detection never attempts unlock', async () => {
  const trigger = createWakeUnlockTrigger(async () => assert.fail('setting is disabled'))
  await trigger.observe(false, false)
  await trigger.observe(true, false)
  await trigger.observe(true, false)
})

test('concurrent screen observations do not duplicate a pending unlock', async () => {
  let attempts = 0
  let finish
  const trigger = createWakeUnlockTrigger(() => {
    attempts++
    return new Promise(resolve => { finish = resolve })
  })
  const pending = trigger.observe(true, true)
  await trigger.observe(false, true)
  await trigger.observe(true, true)
  assert.equal(attempts, 1)
  finish()
  await pending
})

test('an unlock error is not retried every poll, but a later wake can try again', async () => {
  let attempts = 0
  const trigger = createWakeUnlockTrigger(async () => {
    attempts++
    if (attempts === 1)
      throw new Error('command failed')
  })
  await assert.rejects(trigger.observe(true, true), /command failed/)
  await trigger.observe(true, true)
  assert.equal(attempts, 1)
  await trigger.observe(false, true)
  await trigger.observe(true, true)
  assert.equal(attempts, 2)
})

test('changing the setting resets the attempt for the next confirmed screen state', async () => {
  let attempts = 0
  const trigger = createWakeUnlockTrigger(async () => { attempts++ })
  await trigger.observe(true, true)
  trigger.reset()
  await trigger.observe(true, true)
  assert.equal(attempts, 2)
})
