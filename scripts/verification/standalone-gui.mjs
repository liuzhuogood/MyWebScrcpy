import { spawn } from 'node:child_process'
import { mkdtemp, readFile, readdir, rm } from 'node:fs/promises'
import os from 'node:os'
import path from 'node:path'
import assert from 'node:assert/strict'
import { once } from 'node:events'

const executable = path.resolve(process.argv[2])
assert.deepEqual(await readdir(path.dirname(executable)), [path.basename(executable)], 'GUI 测试目录只能包含单个 exe')
const directory = await mkdtemp(path.join(os.tmpdir(), 'mywebscrcpy-standalone-test-'))
const readyFile = path.join(directory, 'ready.json')
const cache = path.join(directory, 'local-app-data')
const child = spawn(executable, ['--no-gui', '--listen', '127.0.0.1:0', '--data-dir', directory, '--desktop-ready', readyFile], {
  stdio: ['pipe', 'ignore', 'pipe'],
  env: { ...process.env, LOCALAPPDATA: cache, TLS_CERT: '', TLS_KEY: '', PORT: '' },
})
let errorLog = ''
child.stderr.on('data', data => { errorLog = (errorLog + data).slice(-8192) })
try {
  let ready
  const deadline = Date.now() + 20000
  while (Date.now() < deadline) {
    if (child.exitCode !== null) throw new Error(`GUI exited: ${errorLog}`)
    try { ready = JSON.parse(await readFile(readyFile, 'utf8')); break }
    catch { await new Promise(resolve => setTimeout(resolve, 50)) }
  }
  assert(ready, '独立 GUI 内嵌服务未就绪')
  assert.notEqual(ready.pid, child.pid, '就绪服务必须由 GUI 启动')
  const runtime = path.join(cache, 'com.mywebscrcpy.desktop', 'runtime')
  const hashes = await readdir(runtime)
  assert.equal(hashes.length, 1)
  assert.match(hashes[0], /^[a-f0-9]{64}$/)
  assert((await readFile(path.join(runtime, hashes[0], 'mywebscrcpy.exe'))).length > 1000000)
  const headers = { Authorization: `Bearer ${ready.token}` }
  assert.equal((await fetch(ready.url + '/api/desktop/health', { headers })).status, 200)
  const exited = once(child, 'exit')
  assert.equal((await fetch(ready.url + '/api/desktop/shutdown', { method: 'POST', headers })).status, 202)
  const timeout = setTimeout(() => child.kill(), 10000)
  try { assert.equal((await exited)[0], 0) } finally { clearTimeout(timeout) }
  console.log('PASS: standalone GUI embeds, extracts, starts and stops its own service')
} finally {
  child.stdin.end()
  if (child.exitCode === null) child.kill()
  await rm(directory, { recursive: true, force: true, maxRetries: 10, retryDelay: 200 })
}
