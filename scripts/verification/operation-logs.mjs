import assert from 'node:assert/strict'
import { readFile } from 'node:fs/promises'
import { createRequire } from 'node:module'

const require = createRequire(new URL('../../frontend/package.json', import.meta.url))
const ts = require('typescript')
globalThis.__opLogTestVue = require('vue')
const root = new URL('../../frontend/src/', import.meta.url)
const original = { fetch: globalThis.fetch, setTimeout: globalThis.setTimeout, clearTimeout: globalThis.clearTimeout, now: Date.now, warn: console.warn }
const source = async path => ts.transpileModule(await readFile(new URL(path, root), 'utf8'), { compilerOptions: { target: ts.ScriptTarget.ES2022, module: ts.ModuleKind.ESNext } }).outputText
const url = code => `data:text/javascript;base64,${Buffer.from(code).toString('base64')}`
const vueURL = url('export const ref = globalThis.__opLogTestVue.ref')
const apiURL = url(await source('api/operationLogs.ts'))
const storeSource = (await source('inspector/opLogStore.ts')).replace(/(['"])vue\1/g, JSON.stringify(vueURL)).replace(/(['"])@\/api\/operationLogs\1/g, JSON.stringify(apiURL))
const logSource = await source('inspector/opLog.ts')
let testNumber = 0

function deferred() {
  let resolve
  const promise = new Promise(r => { resolve = r })
  return { promise, resolve }
}

async function setup() {
  const timers = new Map()
  const calls = []
  let clock = 1800000000000
  let timerID = 0
  globalThis.setTimeout = (callback, delay) => {
    const id = ++timerID
    timers.set(id, { callback, at: clock + delay })
    return id
  }
  globalThis.clearTimeout = id => timers.delete(id)
  Date.now = () => clock
  console.warn = () => {}
  let responder = async () => ({ ok: true, status: 201, json: async () => ({ saved: 1 }) })
  globalThis.fetch = (path, init) => {
    calls.push({ path, init, body: init.body ? JSON.parse(init.body) : undefined })
    return responder(path, init)
  }
  const storeURL = url(storeSource + `\n// instance ${++testNumber}`)
  const logURL = url(logSource.replace(/(['"])vue\1/g, JSON.stringify(vueURL)).replace(/(['"])\.\/opLogStore\1/g, JSON.stringify(storeURL)))
  const store = await import(storeURL)
  const log = await import(logURL)
  const settle = async () => { for (let i = 0; i < 1000; i++) await Promise.resolve() }
  const advance = async milliseconds => {
    clock += milliseconds
    for (const [id, timer] of [...timers]) {
      if (timer.at <= clock) {
        timers.delete(id)
        timer.callback()
      }
    }
    await settle()
  }
  return { store, log, calls, settle, advance, respond: value => { responder = value }, time: () => clock }
}

try {
  {
    const t = await setup()
    const blocked = deferred()
    t.respond(() => blocked.promise)
    t.log.recordOp('device', 'tap', { x: 10, y: 20 })
    t.log.recordOp('device', 'key', { label: '返回', keycode: 'KEYCODE_BACK' })
    assert.equal(t.calls.length, 0, 'input callback must not start HTTP')
    assert.equal(t.store.opEntries.value.length, 2)
    await t.advance(250)
    assert.equal(t.calls.length, 1)
    assert.equal(t.calls[0].body.items.length, 2)
    assert.equal(t.calls[0].body.items[0].adb_command, "adb -s 'device' shell input tap 10 20")
    t.log.recordOp('device', 'text', { text: 'hello' })
    assert.equal(t.store.opEntries.value.length, 3, 'blocked HTTP must not block input')
    blocked.resolve({ ok: true, status: 201, json: async () => ({ saved: 2 }) })
    await t.settle()
    assert.equal(t.calls.length, 2)
  }
  {
    const t = await setup()
    let failures = 0
    t.respond(async () => ({ ok: ++failures >= 3, status: failures >= 3 ? 201 : 503, json: async () => ({ saved: 1 }) }))
    t.log.recordOp('device', 'longpress', { x: 1, y: 2, duration: 800 })
    await t.advance(250)
    await t.advance(1000)
    await t.advance(2000)
    assert.equal(t.calls.length, 3)
    assert.equal(new Set(t.calls.map(call => call.body.items[0].id)).size, 1, 'retries must preserve IDs')
    assert.equal(t.store.opError.value, '')
  }
  {
    const t = await setup()
    const deletion = deferred()
    t.respond(async (_path, init) => init.method === 'DELETE' ? deletion.promise : ({ ok: true, status: 201, json: async () => ({ saved: 1 }) }))
    t.log.recordOp('device', 'tap', { x: 1, y: 1 })
    const clear = t.store.clearOps()
    await t.advance(250)
    assert.deepEqual(t.calls.map(call => call.init.method), ['POST', 'DELETE'])
    t.log.recordOp('device', 'tap', { x: 2, y: 2 })
    assert.equal(t.store.opEntries.value.length, 2)
    deletion.resolve({ ok: true, status: 200, json: async () => ({ deleted: 1 }) })
    await clear
    await t.settle()
    assert.deepEqual(t.calls.map(call => call.init.method), ['POST', 'DELETE', 'POST'])
    assert.equal(t.store.opEntries.value.length, 1)
    assert.equal(t.store.opEntries.value[0].desc, '点击 (2, 2)')
  }
  {
    const t = await setup()
    const history = deferred()
    t.respond(async (_path, init) => init.method ? ({ ok: true, status: 200, json: async () => ({ deleted: 1 }) }) : history.promise)
    const loading = t.store.loadOps()
    t.log.recordOp('device', 'tap', { x: 1, y: 1 })
    const clear = t.store.clearOps()
    await t.advance(250)
    await clear
    history.resolve({ ok: true, status: 200, json: async () => ({ items: [{ id: 'stale', serial: 'device', type: 'tap', description: 'old', adb_command: 'old', occurred_at: t.time() }], next_before: 0 }) })
    await loading
    assert.equal(t.store.opEntries.value.length, 0, 'stale history must not restore cleared records')
  }
  {
    const t = await setup()
    t.log.recordOp('device', 'tap', { x: 1, y: 1 })
    const live = t.store.opEntries.value[0]
    t.respond(async () => ({ ok: true, status: 200, json: async () => ({ items: [{ id: live.id, serial: 'device', type: 'tap', description: live.desc, adb_command: live.adb, occurred_at: live.occurredAt }, { id: 'previous-session', serial: 'device', type: 'key', description: '返回', adb_command: 'adb shell input keyevent 4', occurred_at: live.occurredAt - 1000 }], next_before: 0 }) }))
    await t.store.loadOps()
    assert.equal(t.store.opEntries.value.length, 2, 'history must merge without duplicates')
    assert.equal(t.store.opEntries.value[0].id, 'previous-session')
  }
  {
    const t = await setup()
    for (let i = 0; i < 1005; i++) t.log.recordOp('device', 'tap', { x: i, y: 1 })
    assert.equal(t.calls.length, 0)
    assert.equal(t.store.opEntries.value.length, 500)
    assert.match(t.store.opError.value, /过多/)
    await t.advance(250)
    assert.equal(t.calls.flatMap(call => call.body.items).length, 1000, 'upload queue must be bounded')
    assert.ok(t.calls.every(call => call.body.items.length <= 50))
  }
  {
    const t = await setup()
    t.respond(async () => ({ ok: false, status: 400 }))
    t.log.recordOp('device', 'tap', { x: 1, y: 1 })
    const clear = t.store.clearOps()
    await t.advance(250)
    await clear
    assert.equal(t.store.opEntries.value.length, 1, 'failed delete must preserve display')
    assert.equal(t.calls.length, 2, 'invalid requests must not be retried')
    assert.match(t.store.opError.value, /清理/)
    const loading = t.store.loadOps()
    await loading
    assert.equal(t.store.opLoadFailed.value, true)
  }
  console.log('PASS: asynchronous batching, blocked-network input, retry IDs, ordered clear, stale history, merge, bounded queue, failure preservation')
}
finally {
  globalThis.fetch = original.fetch
  globalThis.setTimeout = original.setTimeout
  globalThis.clearTimeout = original.clearTimeout
  Date.now = original.now
  console.warn = original.warn
  delete globalThis.__opLogTestVue
}
