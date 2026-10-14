import { spawn, spawnSync, execFileSync } from 'node:child_process'
import { existsSync, mkdirSync, openSync, closeSync, readFileSync, writeFileSync, renameSync, rmSync, statSync } from 'node:fs'
import net from 'node:net'
import path from 'node:path'
import { fileURLToPath } from 'node:url'

const script = fileURLToPath(import.meta.url)
const root = path.dirname(path.dirname(script))
const stateDir = path.resolve(process.env.MYWEBSCRCPY_SERVICE_DIR ?? path.join(root, '.cache/services'))
const names = ['backend', 'frontend']
const sleep = ms => new Promise(resolve => setTimeout(resolve, ms))
const recordPath = name => path.join(stateDir, `${name}.json`)
const logPath = name => path.join(stateDir, `${name}.log`)

function identity(pid) {
  try {
    // Start time and process group prevent signalling a reused PID.
    return execFileSync('ps', ['-p', String(pid), '-o', 'lstart=', '-o', 'pgid=', '-o', 'args='], {
      encoding: 'utf8', env: { ...process.env, LC_ALL: 'C' }, stdio: ['ignore', 'pipe', 'ignore'],
    }).trim()
  } catch { return '' }
}

function alive(record) { return identity(record.pid) === record.identity }

function readRecord(name) {
  const file = recordPath(name)
  if (!existsSync(file)) return undefined
  const record = JSON.parse(readFileSync(file, 'utf8'))
  if (record.schema !== 1 || record.root !== root || record.name !== name || !Number.isInteger(record.pid) || record.pid <= 1 || !record.identity || !/^http:\/\/127\.0\.0\.1:\d+$/.test(record.url) || (name === 'backend' && (typeof record.dataDir !== 'string' || !path.isAbsolute(record.dataDir)))) {
    throw new Error(`${file} 的进程记录无效`)
  }
  return record
}

function writeJSON(file, value) {
  const temporary = `${file}.${process.pid}.tmp`
  writeFileSync(temporary, JSON.stringify(value, null, 2), { mode: 0o600 })
  renameSync(temporary, file)
}

function acquireLock() {
  mkdirSync(stateDir, { recursive: true })
  const directory = path.join(stateDir, 'command.lock')
  try { mkdirSync(directory) }
  catch (error) {
    if (error.code !== 'EEXIST') throw error
    const ownerFile = path.join(directory, 'owner.json')
    if (existsSync(ownerFile)) {
      const owner = JSON.parse(readFileSync(ownerFile, 'utf8'))
      if (alive(owner)) throw new Error('另一个服务管理命令正在执行')
    } else if (Date.now() - statSync(directory).mtimeMs < 15000) {
      throw new Error('另一个服务管理命令正在初始化')
    }
    rmSync(directory, { recursive: true })
    mkdirSync(directory)
  }
  writeJSON(path.join(directory, 'owner.json'), { pid: process.pid, identity: identity(process.pid) })
  return () => rmSync(directory, { recursive: true, force: true })
}

function run(command, args) {
  return new Promise((resolve, reject) => {
    const child = spawn(command, args, { cwd: root, stdio: 'inherit' })
    child.on('error', reject)
    child.on('exit', (code, signal) => code === 0 ? resolve() : reject(new Error(`${command} 失败（${signal ?? code}）`)))
  })
}

function port(value, fallback) {
  const selected = value === undefined ? String(fallback) : value
  if (!/^\d+$/.test(selected) || Number(selected) < 1 || Number(selected) > 65535) throw new Error(`端口无效: ${selected}`)
  return Number(selected)
}

function configuration() {
  const previousBackend = readRecord('backend')
  const previousFrontend = readRecord('frontend')
  const backend = port(process.env.MYWEBSCRCPY_BACKEND_PORT, previousBackend ? previousBackend.url.split(':')[2] : 8080)
  const frontend = port(process.env.MYWEBSCRCPY_FRONTEND_PORT, previousFrontend ? previousFrontend.url.split(':')[2] : 5173)
  if (backend === frontend) throw new Error('前端与后端端口不能相同')
  return {
    backend: { port: backend, url: `http://127.0.0.1:${backend}`, dataDir: path.resolve(process.env.MYWEBSCRCPY_SERVICE_DATA_DIR ?? previousBackend?.dataDir ?? root) },
    frontend: { port: frontend, url: `http://127.0.0.1:${frontend}` },
  }
}

async function requireFreePort(value) {
  const server = net.createServer()
  await new Promise((resolve, reject) => {
    server.once('error', () => reject(new Error(`端口 ${value} 已被占用；不会停止已有进程`)))
    server.listen(value, '127.0.0.1', () => server.close(resolve))
  })
}

async function healthy(name, url) {
  try {
    const response = await fetch(url + (name === 'backend' ? '/api/openapi.json' : '/'), { signal: AbortSignal.timeout(1500) })
    if (!response.ok) return false
    if (name === 'backend') return (await response.json()).info.title === 'MyWebScrcpy API'
    return (await response.text()).includes('/src/main.ts')
  } catch { return false }
}

async function requireFrontendDependencies() {
  if (!existsSync(path.join(root, 'frontend/node_modules/vite/package.json'))) {
    console.log('[准备] 安装前端锁定依赖')
    await run('pnpm', ['--dir', 'frontend', 'install', '--frozen-lockfile'])
  }
}

async function startOne(name, config) {
  const existing = readRecord(name)
  if (existing && alive(existing)) {
    if (existing.url !== config[name].url) throw new Error(`${name} 正在使用 ${existing.url}，请先停止后再修改端口`)
    if (!await healthy(name, existing.url)) throw new Error(`${name} 进程存在但服务不可用，请查看 ${logPath(name)}`)
    console.log(`[运行中] ${name}: ${existing.url}`)
    return false
  }
  rmSync(recordPath(name), { force: true })
  await requireFreePort(config[name].port)
  let command, args
  if (name === 'backend') {
    if (!existsSync(path.join(root, 'web/dist/index.html'))) {
      await requireFrontendDependencies()
      await run('pnpm', ['--dir', 'frontend', 'build'])
    }
    command = path.join(stateDir, 'mywebscrcpy')
    console.log('[构建] Go 后端')
    await run('go', ['build', '-o', command, '.'])
    args = ['--no-gui', '--listen', `127.0.0.1:${config.backend.port}`, '--data-dir', config.backend.dataDir]
  } else {
    await requireFrontendDependencies()
    command = 'pnpm'
    args = ['--dir', 'frontend', 'dev', '--host', '127.0.0.1', '--port', String(config.frontend.port), '--strictPort']
  }
  const log = openSync(logPath(name), 'a', 0o600)
  writeFileSync(log, `\n[Services] ${new Date().toISOString()} 启动 ${name}\n`)
  // A persistent group leader owns all children, including the package manager
  // and Vite. It remains alive until descendants exit or the stop timeout fires.
  const child = spawn(process.execPath, [script, '__run', JSON.stringify({ name, command, args })], {
    cwd: root, detached: true, stdio: ['ignore', log, log],
    env: { ...process.env, TLS_CERT: '', TLS_KEY: '', MYWEBSCRCPY_BACKEND_URL: config.backend.url },
  })
  closeSync(log)
  await new Promise((resolve, reject) => { child.once('spawn', resolve); child.once('error', reject) })
  child.unref()
  const fingerprint = identity(child.pid)
  if (!fingerprint) throw new Error(`${name} 启动失败，请查看 ${logPath(name)}`)
  const record = { schema: 1, name, root, pid: child.pid, identity: fingerprint, url: config[name].url, ...(name === 'backend' ? { dataDir: config.backend.dataDir } : {}) }
  writeJSON(recordPath(name), record)
  try {
    const deadline = Date.now() + 30000
    while (Date.now() < deadline) {
      if (!alive(record)) throw new Error(`${name} 启动时退出，请查看 ${logPath(name)}`)
      if (await healthy(name, record.url)) {
        console.log(`[已启动] ${name}: ${record.url}（PID ${record.pid}）`)
        return true
      }
      await sleep(250)
    }
    throw new Error(`${name} 启动超时，请查看 ${logPath(name)}`)
  } catch (error) { await stopOne(name); throw error }
}

async function stopOne(name) {
  const record = readRecord(name)
  if (!record || !alive(record)) {
    rmSync(recordPath(name), { force: true })
    console.log(`[已停止] ${name}`)
    return
  }
  console.log(`[停止] ${name}（PID ${record.pid}）`)
  // The supervisor broadcasts once. Sending to the whole group here too would
  // deliver two SIGTERMs and interrupt Go's deferred shutdown cleanup.
  process.kill(record.pid, 'SIGTERM')
  const deadline = Date.now() + 12000
  while (alive(record) && Date.now() < deadline) await sleep(100)
  if (alive(record)) {
    console.log(`[强制停止] ${name}`)
    process.kill(-record.pid, 'SIGKILL')
    for (let attempt = 0; attempt < 20 && alive(record); attempt++) await sleep(100)
  }
  if (alive(record)) throw new Error(`${name} 尚未退出，保留进程记录`)
  rmSync(recordPath(name), { force: true })
  console.log(`[已停止] ${name}`)
}

function groupMembers() {
  const snapshot = spawnSync('ps', ['-axo', 'pid=,pgid=,stat='], { encoding: 'utf8' })
  if (snapshot.status !== 0) throw new Error('读取服务进程组失败')
  return snapshot.stdout.trim().split('\n').filter(line => {
    const [pid, group, state] = line.trim().split(/\s+/)
    return Number(group) === process.pid && Number(pid) !== process.pid && Number(pid) !== snapshot.pid && !state.startsWith('Z')
  }).length
}

async function supervise() {
  const spec = JSON.parse(process.argv[3])
  let stopping = false
  let exitCode = 0
  const terminate = () => {
    if (stopping) return
    stopping = true
    process.kill(-process.pid, 'SIGTERM')
    const deadline = Date.now() + 10000
    const monitor = setInterval(() => {
      if (groupMembers() === 0) { clearInterval(monitor); process.exit(exitCode) }
      if (Date.now() >= deadline) process.kill(-process.pid, 'SIGKILL')
    }, 100)
  }
  process.on('SIGTERM', terminate)
  process.on('SIGINT', terminate)
  const child = spawn(spec.command, spec.args, { cwd: root, stdio: 'inherit' })
  child.on('error', error => { console.error(`[Services] ${spec.name}: ${error.message}`); exitCode = 1; terminate() })
  child.on('exit', (code, signal) => {
    console.log(`[Services] ${spec.name} 退出 code=${code} signal=${signal}`)
    if (!stopping) exitCode = code ?? 1
    terminate()
  })
}

async function main() {
  const action = process.argv[2] ?? 'help'
  const target = process.argv[3] ?? 'all'
  if (action === 'help' || action === '--help' || action === '-h') {
    console.log('用法: ./scripts/services.sh <start|stop|restart|status|logs> [all|backend|frontend]\n默认前端 http://127.0.0.1:5173，后端 http://127.0.0.1:8080。')
    return
  }
  if (process.platform === 'win32') throw new Error('此脚本用于 macOS/Linux')
  if (!['start', 'stop', 'restart', 'status', 'logs'].includes(action) || !['all', ...names].includes(target) || process.argv.length > 4) throw new Error('命令无效，使用 ./scripts/services.sh help 查看用法')
  const selected = target === 'all' ? names : [target]
  if (action === 'logs') {
    for (const name of selected) {
      console.log(`[日志] ${logPath(name)}`)
      if (existsSync(logPath(name))) {
        const file = openSync(logPath(name), 'r')
        try {
          const size = statSync(logPath(name)).size
          const buffer = Buffer.alloc(Math.min(size, 32768))
          const { readSync } = await import('node:fs')
          readSync(file, buffer, 0, buffer.length, size - buffer.length)
          console.log(buffer.toString('utf8').split('\n').slice(-80).join('\n'))
        } finally { closeSync(file) }
      }
    }
    return
  }
  if (action === 'status') {
    for (const name of selected) {
      const record = readRecord(name)
      const running = record && alive(record)
      const ready = running && await healthy(name, record.url)
      console.log(ready ? `[运行中] ${name}: ${record.url}（PID ${record.pid}）` : running ? `[异常] ${name}: 进程存在但服务不可用（PID ${record.pid}）` : `[未运行] ${name}`)
      if (!ready) process.exitCode = 1
    }
    return
  }
  const release = acquireLock()
  try {
    const config = action === 'stop' ? undefined : configuration()
    if (action === 'stop' || action === 'restart') for (const name of [...selected].reverse()) await stopOne(name)
    if (action === 'stop') return
    // Detect conflicts before starting either half of an all-service request.
    for (const name of selected) {
      const record = readRecord(name)
      if (!record || !alive(record)) await requireFreePort(config[name].port)
    }
    const started = []
    try {
      for (const name of selected) if (await startOne(name, config)) started.push(name)
    } catch (error) {
      for (const name of started.reverse()) await stopOne(name)
      throw error
    }
    console.log(`[日志目录] ${stateDir}`)
  } finally { release() }
}

try { if (process.argv[2] === '__run') await supervise(); else await main() }
catch (error) { console.error(`[Services] ${error.message}`); process.exitCode = 1 }
