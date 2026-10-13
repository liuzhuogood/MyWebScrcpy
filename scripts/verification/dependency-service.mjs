import { spawn } from 'node:child_process'
import { mkdtemp, readFile, rm, stat } from 'node:fs/promises'
import os from 'node:os'
import path from 'node:path'
import assert from 'node:assert/strict'
import { once } from 'node:events'

const executable = process.argv[2]
if (!executable) throw new Error('需要待验证的主程序路径')
const directory = await mkdtemp(path.join(os.tmpdir(), 'mywebscrcpy-dependency-api-'))
const readyFile = path.join(directory, 'ready.json')
const child = spawn(path.resolve(executable), ['--no-gui', '--listen', '127.0.0.1:0', '--data-dir', directory, '--desktop-ready', readyFile], {
  stdio: ['pipe', 'ignore', 'pipe'],
  env: { ...process.env, MYWEBSCRCPY_ADB: path.join(directory, 'missing-adb'), TLS_CERT: '', TLS_KEY: '' },
})
let log = ''
child.stderr.on('data', data => { log = (log + data.toString()).slice(-8192) })
try {
  let ready
  const deadline = Date.now() + 15000
  while (Date.now() < deadline) {
    if (child.exitCode !== null) throw new Error(log)
    try { ready = JSON.parse(await readFile(readyFile, 'utf8')); break }
    catch { await new Promise(resolve => setTimeout(resolve, 50)) }
  }
  assert(ready, '缺依赖仍应启动配置页')
  const auth = { 'Content-Type': 'application/json', Authorization: `Bearer ${ready.token}` }
  const request = (endpoint, init) => fetch(ready.url + endpoint, init)
  assert.equal((await request('/')).status, 200)
  assert.equal((await request('/api/devices')).status, 503)
  const initial = await (await request('/api/dependencies')).json()
  assert.deepEqual(initial.managed.map(tool => tool.id), ['pi-agent', 'adb'])
  assert(initial.managed.every(tool => !tool.installed))
  for (const [endpoint, method] of [['/api/dependencies/config', 'PUT'], ['/api/dependencies/install', 'POST'], ['/api/dependencies/installed/pi-agent', 'DELETE'], ['/api/dependencies/detect', 'POST']]) {
    assert.equal((await request(endpoint, { method, headers: { 'Content-Type': 'application/json' }, body: method === 'DELETE' ? undefined : '{}' })).status, 401)
  }
  assert.equal((await request('/api/dependencies/install', { method: 'POST', headers: auth, body: JSON.stringify({ id: 'adb', accept_license: false }) })).status, 409)
  assert.equal((await request('/api/dependencies/install', { method: 'POST', headers: auth, body: JSON.stringify({ id: 'unknown', accept_license: false }) })).status, 409)
  const invalid = structuredClone(initial.config)
  invalid.pi = { source: 'system', path: path.join(directory, 'missing-pi'), fd: '', rg: '' }
  assert.equal((await request('/api/dependencies/config', { method: 'PUT', headers: auth, body: JSON.stringify(invalid) })).status, 409)
  assert.deepEqual((await (await request('/api/dependencies')).json()).config, initial.config)
  await assert.rejects(stat(path.join(directory, 'dependencies', 'config.json')))
  assert.equal((await request('/api/dependencies/install', { method: 'POST', headers: auth, body: JSON.stringify({ id: 'pi-agent', accept_license: false, url: 'https://evil.example/archive.zip' }) })).status, 400)
  assert.equal((await request('/api/dependencies/install', { method: 'POST', headers: auth, body: JSON.stringify({ id: 'pi-agent', accept_license: false }) })).status, 202)
  assert.equal((await request('/api/dependencies/install/pi-agent', { method: 'DELETE', headers: auth })).status, 204)
  for (let i = 0; i < 100; i++) {
    const task = await (await request('/api/dependencies/install/pi-agent')).json()
    if (task.stage === 'cancelled') break
    await new Promise(resolve => setTimeout(resolve, 50))
  }
  assert.equal((await (await request('/api/dependencies/install/pi-agent')).json()).stage, 'cancelled')
  assert.equal((await request('/api/dependencies/installed/pi-agent', { method: 'DELETE', headers: auth })).status, 204)
  await assert.rejects(stat(path.join(directory, 'dependencies', 'pi-agent')))
  const exited = once(child, 'exit')
  assert.equal((await request('/api/desktop/shutdown', { method: 'POST', headers: auth })).status, 202)
  const [code] = await exited
  assert.equal(code, 0)
  console.log('PASS: dependency-free startup, private mutations, license gate, strict contracts, atomic rejection, cancellation and owned cleanup')
}
finally {
  child.stdin.end()
  if (child.exitCode === null) child.kill()
  await rm(directory, { recursive: true, force: true, maxRetries: 10, retryDelay: 200 })
}
