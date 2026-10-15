import assert from 'node:assert/strict'
import { readdirSync, mkdtempSync, mkdirSync, copyFileSync, rmSync, existsSync } from 'node:fs'
import { spawnSync } from 'node:child_process'
import path from 'node:path'
import os from 'node:os'

const [bundle, arch, tag, output] = process.argv.slice(2)
assert.equal(process.platform, 'darwin', 'DMG 必须在 macOS 原生验证')
assert(bundle && output && /^(arm64|amd64)$/.test(arch) && /^v\d+\.\d+\.\d+$/.test(tag), '需要 bundle、架构、版本标签、输出目录')
function run(command, args) {
  const result = spawnSync(command, args, { encoding: 'utf8', timeout: 120000 })
  if (result.error) throw result.error
  assert.equal(result.status, 0, `${command}: ${result.stderr || result.stdout}`)
  return result.stdout.trim()
}
function files(directory) {
  return readdirSync(directory, { withFileTypes: true }).flatMap(item =>
    item.isDirectory() ? files(path.join(directory, item.name)) : [path.join(directory, item.name)])
}
const images = files(bundle).filter(file => file.endsWith('.dmg'))
assert.equal(images.length, 1, '每个架构必须恰好生成一个 DMG')
run('hdiutil', ['verify', images[0]])
const mount = mkdtempSync(path.join(os.tmpdir(), 'mywebscrcpy-dmg-'))
let mounted = false
try {
  run('hdiutil', ['attach', '-readonly', '-nobrowse', '-mountpoint', mount, images[0]])
  mounted = true
  const apps = readdirSync(mount).filter(name => name.endsWith('.app'))
  assert.equal(apps.length, 1, 'DMG 必须包含一个完整 App')
  assert(existsSync(path.join(mount, '解决打开报错指引.txt')), 'DMG 必须包含解决打开报错指引文档')
  const app = path.join(mount, apps[0]), plist = path.join(app, 'Contents', 'Info.plist')
  const version = run('plutil', ['-extract', 'CFBundleShortVersionString', 'raw', '-o', '-', plist])
  assert.equal(version, tag.slice(1), 'App 版本必须与发布标签一致')
  const executable = run('plutil', ['-extract', 'CFBundleExecutable', 'raw', '-o', '-', plist])
  for (const name of [executable, 'mywebscrcpy']) {
    assert.equal(run('lipo', ['-archs', path.join(app, 'Contents', 'MacOS', name)]), arch === 'amd64' ? 'x86_64' : 'arm64', `${name} 架构错误`)
  }
  run('codesign', ['--verify', '--deep', '--strict', app])
  console.log(run(process.execPath, [path.resolve('scripts/verification/desktop-service.mjs'), path.join(app, 'Contents', 'MacOS', 'mywebscrcpy')]))
  console.log(`PASS: ${tag} macOS ${arch} DMG、完整 App、GUI/服务架构和代码签名校验`)
} finally {
  if (mounted) run('hdiutil', ['detach', mount])
  rmSync(mount, { recursive: true, force: true })
}
mkdirSync(output, { recursive: true })
const destination = path.join(output, `mywebscrcpy-${tag}-macos-${arch}-gui.dmg`)
copyFileSync(images[0], destination)
console.log(`GUI installer: ${destination}`)
