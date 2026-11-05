import { spawn, execFileSync } from 'node:child_process'
import { createHash } from 'node:crypto'
import { existsSync, mkdirSync, readFileSync, writeFileSync, rmSync } from 'node:fs'
import os from 'node:os'
import path from 'node:path'
import { fileURLToPath } from 'node:url'

const root = path.resolve(path.dirname(fileURLToPath(import.meta.url)), '..')
const directory = path.join(root, '.cache/services/desktop-dev')
const recordFile = path.join(directory, 'process.json')
const buildFile = path.join(directory, 'build.json')
const marker = path.join(directory, 'window-ready')
const logFile = path.join(os.homedir(), 'Library/Application Support/com.mywebscrcpy.desktop/desktop.log')
const appBundle = path.join(root, 'desktop/target/release/bundle/macos/MyWebScrcpy.app')
const executable = path.join(appBundle, 'Contents/MacOS/mywebscrcpy-gui')
const endpointFile = path.join(os.homedir(), 'Library/Application Support/com.mywebscrcpy.desktop/desktop-endpoint.json')
const sleep = ms => new Promise(resolve => setTimeout(resolve, ms))

function identity(pid) {
  if (!Number.isInteger(pid) || pid <= 1) return ''
  try {
    return execFileSync('ps', ['-p', String(pid), '-o', 'lstart=', '-o', 'args='], {
      encoding: 'utf8', env: { ...process.env, LC_ALL: 'C' }, stdio: ['ignore', 'pipe', 'ignore'],
    }).trim()
  } catch { return '' }
}

function parentPID(pid) {
  try { return Number(execFileSync('ps', ['-p', String(pid), '-o', 'ppid='], { encoding: 'utf8' }).trim()) }
  catch { return 0 }
}

function readJSON(file) {
  return existsSync(file) ? JSON.parse(readFileSync(file, 'utf8')) : undefined
}

function sourceHash() {
  const files = execFileSync('git', ['ls-files', '-z', '--cached', '--others', '--exclude-standard'], { cwd: root, encoding: 'utf8' })
    .split('\0').filter(file => /^(frontend\/|desktop\/|internal\/|web\/|configs\/|packages\/).*|\.go$|^go\.(mod|sum)$|^VERSION$|^scripts\/(build-desktop|sync-version|start-desktop)\.(mjs|sh)$/.test(file))
  const hash = createHash('sha256')
  for (const file of [...new Set(files)].sort()) {
    hash.update(file + '\0')
    hash.update(existsSync(path.join(root, file)) ? readFileSync(path.join(root, file)) : '<deleted>')
  }
  return hash.digest('hex')
}

async function endpoint() {
  const value = readJSON(endpointFile)
  if (!value || !/^http:\/\/127\.0\.0\.1:\d+$/.test(value.url) || !identity(value.pid)) return undefined
  try {
    const response = await fetch(value.url + '/api/openapi.json', { signal: AbortSignal.timeout(1500) })
    return response.ok && (await response.json()).info.title === 'MyWebScrcpy API' ? value : undefined
  } catch { return undefined }
}

async function run(command, args, timeout = 600000, env = process.env) {
  const child = spawn(command, args, { cwd: root, stdio: 'inherit', detached: true, env })
  let hardStop
  function terminate() {
    if (!child.pid) return
    try { process.kill(-child.pid, 'SIGTERM') } catch (error) { if (error.code !== 'ESRCH') throw error }
    hardStop = setTimeout(() => {
      try { process.kill(-child.pid, 'SIGKILL') } catch (error) { if (error.code !== 'ESRCH') console.error(error.message) }
    }, 5000)
  }
  process.once('SIGINT', terminate)
  process.once('SIGTERM', terminate)
  const timer = setTimeout(terminate, timeout)
  try {
    await new Promise((resolve, reject) => {
      child.once('error', reject)
      child.once('exit', (code, signal) => code === 0 ? resolve() : reject(new Error(`${command} 失败（${signal ?? code}）`)))
    })
  } finally {
    clearTimeout(timer)
    clearTimeout(hardStop)
    process.removeListener('SIGINT', terminate)
    process.removeListener('SIGTERM', terminate)
  }
}

async function main() {
  if (process.platform !== 'darwin' || process.arch !== 'arm64') throw new Error('此快捷脚本用于 macOS Apple Silicon 开发环境')
  if (process.argv.length !== 2) throw new Error('用法: node scripts/start-desktop.mjs')
  mkdirSync(directory, { recursive: true })
  const lock = path.join(directory, 'launch.lock')
  try { mkdirSync(lock) } catch (error) {
    if (error.code !== 'EEXIST') throw error
    const owner = readJSON(path.join(lock, 'owner.json'))
    if (!owner || identity(owner.pid) === owner.identity) throw new Error('另一个桌面启动命令正在执行')
    rmSync(lock, { recursive: true })
    mkdirSync(lock)
  }
  writeFileSync(path.join(lock, 'owner.json'), JSON.stringify({ pid: process.pid, identity: identity(process.pid) }))
  try {
    const hash = sourceHash()
    const previous = readJSON(recordFile)
    if (previous && identity(previous.pid) === previous.identity) {
      if (previous.hash === hash) {
        const ready = await endpoint()
        if (ready && parentPID(ready.pid) === previous.pid) {
          await run('open', ['-a', appBundle, '--args', '--gui'], 10000)
          console.log(`[运行中] 当前源码桌面开发版: ${ready.url}（PID ${previous.pid}）`)
          return
        }
      }
      console.log('[重启] 停止当前工作区拥有的旧桌面开发实例')
      process.kill(previous.pid, 'SIGTERM')
      const stopDeadline = Date.now() + 10000
      while (identity(previous.pid) === previous.identity && Date.now() < stopDeadline) await sleep(100)
      if (identity(previous.pid) === previous.identity) throw new Error('旧桌面进程未按时退出，请从应用菜单退出后重试')
      // The sidecar exits when its GUI parent closes stdin.
      while (await endpoint() && Date.now() < stopDeadline) await sleep(100)
    }
    if (await endpoint()) throw new Error('已有其他桌面实例运行，请先从 MyWebScrcpy 菜单退出后重试')
    if (readJSON(buildFile)?.hash !== hash || !existsSync(executable)) {
      console.log('[构建] 当前工作区桌面开发版')
      // Use the official App bundle so macOS can identify and activate its window.
      await run(process.execPath, ['scripts/build-desktop.mjs', '--no-bundle'])
      await run('pnpm', ['--dir', 'desktop', 'exec', 'tauri', 'bundle', '--bundles', 'app'])
      if (sourceHash() !== hash) throw new Error('构建期间源码发生变更，请重试')
      writeFileSync(buildFile, JSON.stringify({ hash }))
    }
    rmSync(marker, { force: true })
    // LaunchServices opens and activates this exact workspace App, never /Applications.
    await run('open', ['-n', '-a', appBundle, '--args', '--launcher-ready', marker], 10000)
    let record
    const deadline = Date.now() + 30000
    try {
      while (Date.now() < deadline) {
        if (existsSync(marker)) {
          const ready = await endpoint()
          const guiPID = ready ? parentPID(ready.pid) : 0
          const guiIdentity = identity(guiPID)
          if (ready && guiIdentity.includes(executable)) {
            record = { pid: guiPID, identity: guiIdentity, hash }
            writeFileSync(recordFile, JSON.stringify(record), { mode: 0o600 })
            console.log(`[已启动] 当前源码桌面开发版: ${ready.url}（PID ${record.pid}）`)
            console.log(`[日志] ${logFile}`)
            return
          }
        }
        await sleep(250)
      }
      throw new Error(`桌面窗口或服务启动超时，请查看 ${logFile}`)
    } catch (error) {
      if (record && identity(record.pid) === record.identity) process.kill(record.pid, 'SIGTERM')
      throw error
    }
  } finally { rmSync(lock, { recursive: true, force: true }) }
}

main().catch(error => { console.error(`[桌面启动失败] ${error.message}`); process.exitCode = 1 })
