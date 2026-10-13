import { readFileSync, writeFileSync, readdirSync, renameSync } from 'node:fs'
import path from 'node:path'
const [bundle, target, output, tag] = process.argv.slice(2)
if (!bundle || !target || !output || !tag) throw new Error('需要 bundle、target、output、tag')
const version = readFileSync('VERSION', 'utf8').trim()
if (tag !== version) throw new Error('发布 tag 与 VERSION 不一致')
function files(dir) {
 return readdirSync(dir, { withFileTypes: true }).flatMap(item => item.isDirectory() ? files(path.join(dir,item.name)) : [path.join(dir,item.name)])
}
const signatures = files(bundle).filter(file => file.endsWith('.sig'))
if (signatures.length !== 1) throw new Error(`预期一个更新包签名，实际 ${signatures.length}`)
const signature = signatures[0]
const extensions = { 'darwin-aarch64': '.app.tar.gz', 'darwin-x86_64': '.app.tar.gz', 'windows-x86_64': '-gui-setup.exe', 'linux-x86_64': '.AppImage' }
if (!extensions[target]) throw new Error(`未知更新平台 ${target}`)
const artifact = signature.slice(0,-4)
const name = `mywebscrcpy-${tag}-${target}${extensions[target]}`
const destination = path.join(path.dirname(artifact),name)
// Both macOS architectures otherwise produce the same .app.tar.gz asset name.
renameSync(artifact,destination)
renameSync(signature,destination+'.sig')
writeFileSync(output, JSON.stringify({version:tag,platforms:{[target]:{
 url:`https://github.com/liuzhuogood/MyWebScrcpy/releases/download/${tag}/${encodeURIComponent(name)}`,
 signature:readFileSync(destination+'.sig','utf8').trim(),
}}},null,2))
