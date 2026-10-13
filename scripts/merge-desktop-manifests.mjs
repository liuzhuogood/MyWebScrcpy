import { readdirSync, readFileSync, writeFileSync } from 'node:fs'
import path from 'node:path'
const root = process.argv[2]
const targets = process.argv[3] ? process.argv[3].split(',') : ['darwin-aarch64','darwin-x86_64','windows-x86_64','linux-x86_64']
const version = readFileSync('VERSION','utf8').trim()
function files(dir) { return readdirSync(dir,{withFileTypes:true}).flatMap(item=>item.isDirectory()?files(path.join(dir,item.name)):[path.join(dir,item.name)]) }
const platforms = {}
for (const file of files(root).filter(file=>path.basename(file).startsWith('desktop-manifest-'))) {
 const manifest = JSON.parse(readFileSync(file,'utf8'))
 if (manifest.version !== version) throw new Error(`${file} 版本不一致`)
 for (const [target,value] of Object.entries(manifest.platforms)) {
  if (platforms[target]) throw new Error(`重复平台 ${target}`)
  platforms[target]=value
 }
}
for (const target of targets) {
 if (!platforms[target]) throw new Error(`缺少 ${target} 更新包`)
}
for (const target of Object.keys(platforms)) {
 if (!targets.includes(target)) throw new Error(`非发布目标 ${target}`)
}
const notes = process.env.RELEASE_NOTES_FILE ? readFileSync(process.env.RELEASE_NOTES_FILE, 'utf8') : ''
writeFileSync('desktop-latest.json',JSON.stringify({version,notes,pub_date:new Date().toISOString(),platforms},null,2))
