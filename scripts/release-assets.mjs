import { readdirSync, readFileSync, mkdirSync, copyFileSync } from 'node:fs'
import path from 'node:path'

const [artifacts, index, output] = process.argv.slice(2)
if (!artifacts || !index || !output) throw new Error('需要 artifacts、index、output')
const version = readFileSync('VERSION', 'utf8').trim()
const windowsInstaller = `mywebscrcpy-${version}-windows-x86_64-gui-setup.exe`
const programs = [
  'mywebscrcpy-linux-amd64-cli',
  'mywebscrcpy-linux-arm64-cli',
  'mywebscrcpy-macos-amd64-cli',
  'mywebscrcpy-macos-arm64-cli',
  'mywebscrcpy-windows-cli-amd64.exe',
  'mywebscrcpy-windows-gui-amd64.exe',
  windowsInstaller,
  `mywebscrcpy-${version}-macos-amd64-gui.dmg`,
  `mywebscrcpy-${version}-macos-arm64-gui.dmg`,
]
function files(directory) {
  return readdirSync(directory, { withFileTypes: true }).flatMap(item =>
    item.isDirectory() ? files(path.join(directory, item.name)) : [path.join(directory, item.name)])
}
const all = files(artifacts)
const selected = programs.map(name => {
  const matches = all.filter(file => path.basename(file) === name)
  if (matches.length !== 1) throw new Error(`${name} 预期一个产物，实际 ${matches.length}`)
  return matches[0]
})
const manifest = JSON.parse(readFileSync(index, 'utf8'))
if (manifest.version !== version || Object.keys(manifest.platforms).join(',') !== 'windows-x86_64') {
  throw new Error('GUI 更新索引必须对应当前版本的 Windows 安装包')
}
const platform = manifest.platforms['windows-x86_64']
const expected = `https://github.com/liuzhuogood/MyWebScrcpy/releases/download/${version}/${windowsInstaller}`
if (platform.url !== expected || !platform.signature) throw new Error('GUI 更新索引地址或签名错误')
mkdirSync(output, { recursive: true })
if (readdirSync(output).length) throw new Error('发布目录必须为空')
for (const file of selected) copyFileSync(file, path.join(output, path.basename(file)))
copyFileSync(index, path.join(output, 'desktop-latest.json'))
console.log(`Release: ${programs.length} 个程序 + GUI 自动更新索引`)
