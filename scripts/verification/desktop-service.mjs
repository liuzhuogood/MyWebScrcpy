import { spawn, execFileSync } from 'node:child_process'
import { mkdtemp, readFile, rm } from 'node:fs/promises'
import os from 'node:os'
import path from 'node:path'
import assert from 'node:assert/strict'
import { once } from 'node:events'

const executable = process.argv[2] ?? path.join('desktop', 'binaries', `mywebscrcpy-${execFileSync('rustc', ['--print', 'host-tuple'], { encoding: 'utf8' }).trim()}${process.platform === 'win32' ? '.exe' : ''}`)
const directory = await mkdtemp(path.join(os.tmpdir(), 'mywebscrcpy-service-test-'))
const readyFile = path.join(directory, 'ready.json')
const child = spawn(path.resolve(executable), ['--no-gui', '--listen', '127.0.0.1:0', '--data-dir', directory, '--desktop-ready', readyFile], {
  stdio: ['pipe', 'ignore', 'pipe'],
  env: { ...process.env, MYWEBSCRCPY_ADB: path.join(directory, 'missing-adb'), TLS_CERT: '', TLS_KEY: '', PORT: '', SCRIPTS_DIR: path.join(directory, 'scripts'), TEMPLATES_DIR: path.join(directory, 'templates') },
})
let errorLog = ''
child.stderr.on('data', data => { errorLog = (errorLog + data.toString()).slice(-8192) })
try {
  let ready
  const deadline = Date.now() + 15000
  while (Date.now() < deadline) {
    if (child.exitCode !== null) throw new Error(`Service exited: ${errorLog}`)
    try { ready = JSON.parse(await readFile(readyFile, 'utf8')); break }
    catch { await new Promise(resolve => setTimeout(resolve, 50)) }
  }
  assert(ready, 'service readiness timeout')
  assert.equal(ready.pid, child.pid)
  assert.match(ready.url, /^http:\/\/127\.0\.0\.1:\d+$/)
  const authenticated = { Authorization: `Bearer ${ready.token}` }
  const request = (endpoint, init) => fetch(ready.url + endpoint, init)
  const savePaths = async value => {
    const deadline = Date.now() + 5000
    for (;;) {
      const response = await request('/api/dependencies/paths', {
        method: 'PUT', headers: authenticated, body: JSON.stringify(value),
      })
      const body = await response.text()
      if (response.status !== 409 || body.trim() !== '依赖正在使用，请稍后重试') {
        return { status: response.status, body }
      }
      assert(Date.now() < deadline, `ADB 检测持续占用依赖: ${body}\n${errorLog}`)
      await new Promise(resolve => setTimeout(resolve, 50))
    }
  }
  assert.equal((await request('/api/desktop/health')).status, 401)
  assert.equal((await request('/api/desktop/shutdown', { method: 'POST' })).status, 401)
  assert.equal((await request('/api/desktop/health', { headers: authenticated })).status, 200)
  const dependencies = await (await request('/api/dependencies')).json()
  assert.equal(dependencies.configurable, true)
  assert.deepEqual(dependencies.tools.map(tool => tool.id), ['adb', 'python', 'scrcpy-server'])
  assert.equal((await request('/api/dependencies/paths', { method: 'PUT', body: '{}' })).status, 401)
  const invalid = await savePaths({ adb: 'relative/adb', python: '' })
  assert.equal(invalid.status, 400, invalid.body)
  const saved = await savePaths({ adb: '', python: '' })
  assert.equal(saved.status, 204, saved.body)
  assert.equal((await request('/api/version/upgrade', { method: 'POST' })).status, 409)
  const page = await request('/dependencies')
  assert.equal(page.status, 200)
  assert.match(await page.text(), /<div id="app">/)
  const exited = once(child, 'exit')
  assert.equal((await request('/api/desktop/shutdown', { method: 'POST', headers: authenticated })).status, 202)
  const timeout = setTimeout(() => child.kill(), 10000)
  const [code] = await exited
  clearTimeout(timeout)
  assert.equal(code, 0, 'owned service should shut down cleanly')
  console.log('PASS: loopback readiness, private control, dependency paths, desktop update guard, SPA route, clean shutdown')
}
finally {
  child.stdin.end()
  if (child.exitCode === null) child.kill()
  await rm(directory, { recursive: true, force: true, maxRetries: 10, retryDelay: 200 })
}
