import { mkdtemp, mkdir, writeFile, readFile, rm, stat } from 'node:fs/promises'
import { spawnSync } from 'node:child_process'
import path from 'node:path'
import os from 'node:os'
import assert from 'node:assert/strict'
const root=process.cwd(), temp=await mkdtemp(path.join(os.tmpdir(),'mywebscrcpy-manifest-test-'))
const version=(await readFile('VERSION','utf8')).trim()
function run(script,args,expected=0){const result=spawnSync(process.execPath,[path.join(root,script),...args],{cwd:temp,encoding:'utf8'});assert.equal(result.status,expected,result.stderr)}
try {
 await writeFile(path.join(temp,'VERSION'),version)
 const targets=['darwin-aarch64','darwin-x86_64','windows-x86_64','linux-x86_64']
 for(const target of targets){
  const directory=path.join(temp,'artifacts','desktop-'+target,'bundle');await mkdir(directory,{recursive:true})
  const file=target.startsWith('darwin')?'MyWebScrcpy.app.tar.gz':target.startsWith('windows')?'setup.exe':'app.AppImage'
  await writeFile(path.join(directory,file),'test bundle');await writeFile(path.join(directory,file+'.sig'),'test signature')
  const manifest=path.join(directory,'desktop-manifest-'+target+'.json')
  run('scripts/desktop-manifest.mjs',[directory,target,manifest,version])
  const value=JSON.parse(await readFile(manifest,'utf8'));const name=decodeURIComponent(new URL(value.platforms[target].url).pathname.split('/').at(-1))
  assert((await stat(path.join(directory,name))).isFile())
 }
 run('scripts/merge-desktop-manifests.mjs',['artifacts'])
 const merged=JSON.parse(await readFile(path.join(temp,'desktop-latest.json'),'utf8'))
 assert.equal(merged.version,version);assert.equal(Object.keys(merged.platforms).length,4)
 assert.notEqual(merged.platforms['darwin-aarch64'].url,merged.platforms['darwin-x86_64'].url)
 await rm(path.join(temp,'artifacts','desktop-linux-x86_64'),{recursive:true})
 run('scripts/merge-desktop-manifests.mjs',['artifacts'],1)
 console.log('PASS: platform filenames, manifest merge, architecture separation and incomplete release rejection')
}finally{await rm(temp,{recursive:true,force:true,maxRetries:10,retryDelay:200})}
