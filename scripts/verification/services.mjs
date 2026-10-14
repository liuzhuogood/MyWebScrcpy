import assert from 'node:assert/strict'
import { execFile, execFileSync } from 'node:child_process'
import { mkdtemp, readFile, writeFile, rm } from 'node:fs/promises'
import net from 'node:net'
import os from 'node:os'
import path from 'node:path'
import { fileURLToPath } from 'node:url'
import { promisify } from 'node:util'

const execute = promisify(execFile)
const root = path.resolve(path.dirname(fileURLToPath(import.meta.url)), '../..')
const script = path.join(root, 'scripts/services.mjs')
const directory = await mkdtemp(path.join(os.tmpdir(), 'mywebscrcpy-services-'))
const listen = server => new Promise(resolve => server.listen(0, '127.0.0.1', resolve))
const close = server => new Promise(resolve => server.close(resolve))
const blocker = net.createServer()
await listen(blocker)
const backendPort = blocker.address().port
const frontendReservation = net.createServer()
await listen(frontendReservation)
const frontendPort = frontendReservation.address().port
await close(frontendReservation)
const env = {
  ...process.env,
  MYWEBSCRCPY_SERVICE_DIR: path.join(directory, 'services'),
  MYWEBSCRCPY_SERVICE_DATA_DIR: path.join(directory, 'data'),
  MYWEBSCRCPY_ADB: path.join(directory, 'missing-adb'),
  MYWEBSCRCPY_BACKEND_PORT: String(backendPort),
  MYWEBSCRCPY_FRONTEND_PORT: String(frontendPort),
}
async function command(action, target = 'all', expected = 0, selectedEnv = env) {
  try {
    const result = await execute(process.execPath, [script, action, target], { cwd: directory, env: selectedEnv, timeout: 120000 })
    assert.equal(expected, 0, `expected failure: ${action}`)
    return result.stdout
  } catch (error) {
    if (expected === 0 || error.code !== expected) throw error
    return error.stderr
  }
}
const record = name => readFile(path.join(env.MYWEBSCRCPY_SERVICE_DIR, `${name}.json`), 'utf8').then(JSON.parse)
async function assertProxy() {
  const response = await fetch(`http://127.0.0.1:${frontendPort}/api/openapi.json`, { signal: AbortSignal.timeout(3000) })
  assert.equal(response.status, 200)
  assert.equal((await response.json()).info.title, 'MyWebScrcpy API')
  const docs = await fetch(`http://127.0.0.1:${frontendPort}/api-docs.html`, { signal: AbortSignal.timeout(3000) })
  assert.equal(docs.status, 200)
}
async function assertPortClosed(port) {
  await new Promise((resolve, reject) => {
    const connection = net.connect({ host: '127.0.0.1', port })
    connection.on('connect', () => { connection.destroy(); reject(new Error(`port ${port} remains open`)) })
    connection.on('error', resolve)
  })
}

let passed = false
try {
  assert.match(await command('start', 'all', 1), /已被占用/)
  assert(blocker.listening, 'unrelated listener was stopped')
  await close(blocker)
  assert.match(await command('start'), /已启动.*frontend/)
  const originalBackend = await record('backend')
  const originalFrontend = await record('frontend')
  await new Promise(resolve => setTimeout(resolve, 1000))
  assert.match(await command('status'), /运行中.*backend/)
  await assertProxy()
  await command('start')
  assert.equal((await record('backend')).pid, originalBackend.pid, 'duplicate backend')
  assert.equal((await record('frontend')).pid, originalFrontend.pid, 'duplicate frontend')
  await command('stop', 'frontend')
  await assertPortClosed(frontendPort)
  assert.match(await command('status', 'backend'), /运行中/)
  await command('start', 'frontend')
  const frontend = await record('frontend')
  const restartEnv = { ...env }
  delete restartEnv.MYWEBSCRCPY_BACKEND_PORT
  delete restartEnv.MYWEBSCRCPY_FRONTEND_PORT
  delete restartEnv.MYWEBSCRCPY_SERVICE_DATA_DIR
  await command('restart', 'backend', 0, restartEnv)
  assert.notEqual((await record('backend')).pid, originalBackend.pid)
  assert.equal((await record('backend')).dataDir, originalBackend.dataDir, 'restart changed the data directory')
  assert.equal((await record('frontend')).pid, frontend.pid, 'backend restart stopped frontend')
  await assertProxy()
  assert.match(await command('logs', 'backend'), /退出 code=0 signal=null/, 'backend did not close gracefully')
  await command('stop')
  await assertPortClosed(frontendPort)
  await assertPortClosed(backendPort)
  await command('stop')
  await command('status', 'all', 1)
  // An unrelated process with a stale record must never receive a signal.
  const currentIdentity = execFileSync('ps', ['-p', String(process.pid), '-o', 'lstart=', '-o', 'pgid=', '-o', 'args='], { encoding: 'utf8', env: { ...process.env, LC_ALL: 'C' } }).trim()
  await writeFile(path.join(env.MYWEBSCRCPY_SERVICE_DIR, 'backend.json'), JSON.stringify({ ...originalBackend, pid: process.pid, identity: currentIdentity + ' stale' }))
  await command('stop', 'backend')
  passed = true
  console.log('PASS: real frontend/backend startup, proxy, repeated start, separate control, graceful restart, complete stop, occupied-port and stale-PID protection')
} finally {
  if (blocker.listening) await close(blocker)
  await command('stop').catch(error => console.error('cleanup failed:', error.message))
  if (passed) await rm(directory, { recursive: true, force: true })
  else console.error(`Verification logs: ${directory}`)
}
