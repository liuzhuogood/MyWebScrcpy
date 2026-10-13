import { spawn } from 'node:child_process'
import { mkdtemp, readFile, rm } from 'node:fs/promises'
import { once } from 'node:events'
import assert from 'node:assert/strict'
import os from 'node:os'
import path from 'node:path'

const executable = path.resolve(process.argv[2] ?? '.cache/verification/mywebscrcpy')
assert(process.env.MYWEBSCRCPY_MATCHING_TEST_PYTHON, 'set MYWEBSCRCPY_MATCHING_TEST_PYTHON to a Python with vision dependencies')
const directory = await mkdtemp(path.join(os.tmpdir(), 'matching-service-test-'))
let child
let ready
let errorLog = ''
async function boot() {
  const readyFile = path.join(directory, 'ready.json')
  child = spawn(executable, ['--no-gui', '--listen', '127.0.0.1:0', '--data-dir', directory, '--desktop-ready', readyFile], {
    stdio: ['pipe', 'ignore', 'pipe'],
    env: { ...process.env, MYWEBSCRCPY_PYTHON: process.env.MYWEBSCRCPY_MATCHING_TEST_PYTHON, MYWEBSCRCPY_ADB: path.join(directory, 'missing-adb'), PORT: '', SCRIPTS_DIR: path.join(directory, 'scripts'), TEMPLATES_DIR: path.join(directory, 'templates') },
  })
  child.stderr.on('data', data => { errorLog = (errorLog + data).slice(-8192) })
  const deadline = Date.now() + 15000
  for (;;) {
    assert(child.exitCode === null, errorLog)
    try { ready = JSON.parse(await readFile(readyFile, 'utf8')); break }
    catch { assert(Date.now() < deadline, errorLog); await new Promise(resolve => setTimeout(resolve, 50)) }
  }
}
async function request(operation) {
  return fetch(`${ready.url}/api/matching-service/${operation}`, { method: operation === 'status' ? 'GET' : 'POST', signal: AbortSignal.timeout(40000) })
}
async function shutdown() {
  const exited = once(child, 'exit')
  const response = await fetch(`${ready.url}/api/desktop/shutdown`, { method: 'POST', headers: { Authorization: `Bearer ${ready.token}` } })
  assert.equal(response.status, 202)
  const timer = setTimeout(() => child.kill(), 10000)
  const [code] = await exited
  clearTimeout(timer)
  assert.equal(code, 0, errorLog)
}
try {
  await boot()
  let status = await (await request('status')).json()
  assert.equal(status.state, 'stopped'); assert.equal(status.pid, 0)
  status = await (await request('check')).json()
  assert(status.environment.ready, JSON.stringify(status.environment))
  status = await (await request('start')).json()
  assert.equal(status.state, 'running'); assert(status.pid > 0); assert.deepEqual(status.devices, [])
  const pid = status.pid
  assert.equal((await request('start')).status, 409)
  assert.equal((await (await request('status')).json()).pid, pid)
  status = await (await request('stop')).json()
  assert.equal(status.state, 'stopped'); assert.equal(status.pid, 0)
  const running = await (await request('start')).json()
  assert.equal(running.state, 'running'); assert(running.pid > 0)
  await shutdown() // Go shutdown reaps a still-running matcher.
  assert.throws(() => process.kill(running.pid, 0), { code: 'ESRCH' })
  await boot()
  status = await (await request('status')).json()
  assert.equal(status.state, 'stopped'); assert.equal(status.pid, 0)
  await shutdown()
  console.log('PASS: no auto-start, environment, single process, stop, Go shutdown reaping, restart remains stopped')
}
finally {
  if (child?.exitCode === null) { child.stdin.end(); child.kill() }
  await rm(directory, { recursive: true, force: true, maxRetries: 10, retryDelay: 200 })
}
