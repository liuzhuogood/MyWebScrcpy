import { spawnSync } from 'node:child_process'
import { mkdirSync, copyFileSync, chmodSync, writeFileSync, readFileSync } from 'node:fs'
import path from 'node:path'
import { fileURLToPath } from 'node:url'

const root = path.resolve(path.dirname(fileURLToPath(import.meta.url)), '..')
const desktop = path.join(root, 'desktop')
const args = process.argv.slice(2)
const skipFrontend = args.includes('--skip-frontend')
const noBundle = args.includes('--no-bundle')
const checkOnly = args.includes('--check')
const signed = args.includes('--signed')
const verifyNative = args.includes('--verify-native')
for (const arg of args) {
  if (!['--skip-frontend', '--no-bundle', '--check', '--verify-native', '--signed'].includes(arg)) throw new Error(`未知参数: ${arg}`)
}
function run(command, argv, options = {}) {
  const result = spawnSync(command, argv, { cwd: root, stdio: 'inherit', ...options })
  if (result.error) throw result.error
  if (result.status !== 0) throw new Error(`${command} 失败 (${result.status})`)
}
run(process.execPath, ['scripts/sync-version.mjs'])
if (signed && (!process.env.MYWEBSCRCPY_UPDATER_PUBLIC_KEY || !process.env.TAURI_SIGNING_PRIVATE_KEY)) throw new Error('签名发布需要更新公钥和私钥')
if (signed && process.env.MYWEBSCRCPY_UPDATER_PUBLIC_KEY.trim() !== readFileSync(path.join(desktop, 'updater.pub'), 'utf8').trim()) throw new Error('发布公钥与仓库公钥不一致')
const host = spawnSync('rustc', ['--print', 'host-tuple'], { encoding: 'utf8' })
if (host.status !== 0) throw new Error('无法检测 Rust 编译目标')
const target = host.stdout.trim()
const platforms = {
  'aarch64-apple-darwin': ['darwin', 'arm64'],
  'x86_64-apple-darwin': ['darwin', 'amd64'],
  'x86_64-pc-windows-msvc': ['windows', 'amd64'],
  'x86_64-unknown-linux-gnu': ['linux', 'amd64'],
  'aarch64-unknown-linux-gnu': ['linux', 'arm64'],
}
if (!Object.hasOwn(platforms, target)) throw new Error(`未支持的桌面目标: ${target}`)
const [goos, goarch] = platforms[target]
run(process.platform === 'win32' ? 'python' : 'python3', ['scripts/prepare-pi.py', '--goos', goos, '--goarch', goarch])
const executableSuffix = goos === 'windows' ? '.exe' : ''
const pnpm = process.platform === 'win32' ? 'pnpm.cmd' : 'pnpm'
if (!skipFrontend) run(pnpm, ['--dir', 'frontend', 'build'], { shell: process.platform === 'win32' })
const binaries = path.join(desktop, 'binaries')
mkdirSync(binaries, { recursive: true })
const service = path.join(binaries, `mywebscrcpy-${target}${executableSuffix}`)
run('go', ['build', ...(verifyNative ? ['-tags=desktopverify'] : []), '-trimpath', '-ldflags=-s -w', '-o', service, '.'], {
  env: { ...process.env, GOOS: goos, GOARCH: goarch, CGO_ENABLED: '0' },
})
if (checkOnly) {
  run('cargo', ['check', '--locked'], { cwd: desktop })
  process.exit(0)
}
const buildArgs = ['--dir', desktop, 'exec', 'tauri', 'build']
if (noBundle) buildArgs.push('--no-bundle')
if (process.env.DESKTOP_BUNDLES) buildArgs.push('--bundles', process.env.DESKTOP_BUNDLES)
if (goos === 'darwin') buildArgs.push('--config', JSON.stringify({ bundle: { macOS: { signingIdentity: process.env.APPLE_SIGNING_IDENTITY || '-' } } }))
if (signed) buildArgs.push('--config', JSON.stringify({ bundle: { createUpdaterArtifacts: true } }))
if (verifyNative) buildArgs.push('--features', 'native-verify', '--config', JSON.stringify({ identifier: 'com.mywebscrcpy.desktop.verification' }))
// Linux desktop shortcuts invoke the Go entry point to choose GUI or service.
if (goos === 'linux' && !noBundle) {
  const config = {
    bundle: {
      linux: { deb: { files: { '/usr/bin/mywebscrcpy': service }, desktopTemplate: 'linux.desktop' },
        rpm: { files: { '/usr/bin/mywebscrcpy': service }, desktopTemplate: 'linux.desktop' },
        appimage: { files: { '/usr/bin/mywebscrcpy': service } } },
    },
  }
  const configPath = path.join(desktop, 'target', 'launcher.conf.json')
  mkdirSync(path.dirname(configPath), { recursive: true })
  writeFileSync(configPath, JSON.stringify(config))
  buildArgs.push('--config', configPath)
}
const tauriEnv = { ...process.env }
for (const key of ['APPLE_CERTIFICATE','APPLE_CERTIFICATE_PASSWORD','APPLE_SIGNING_IDENTITY','APPLE_ID','APPLE_PASSWORD','APPLE_TEAM_ID']) {
 if (!tauriEnv[key]) delete tauriEnv[key]
}
run(pnpm, buildArgs, { shell: process.platform === 'win32', env: tauriEnv })
// Also deliver a portable folder with a Go entry point independent of GUI libs.
const portable = path.join(desktop, 'target', 'portable', `${goos}-${goarch}`)
mkdirSync(portable, { recursive: true })
copyFileSync(service, path.join(portable, `mywebscrcpy${executableSuffix}`))
copyFileSync(path.join(desktop, 'target', 'release', `mywebscrcpy-gui${executableSuffix}`), path.join(portable, `mywebscrcpy-gui${executableSuffix}`))
if (goos !== 'windows') {
  chmodSync(path.join(portable, 'mywebscrcpy'), 0o755)
  chmodSync(path.join(portable, 'mywebscrcpy-gui'), 0o755)
}
copyFileSync(path.join(desktop, 'README.md'), path.join(portable, 'README.md'))
console.log(`桌面构建完成，便携入口：${portable}`)
