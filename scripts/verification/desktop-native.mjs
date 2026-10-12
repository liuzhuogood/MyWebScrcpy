import { spawn } from 'node:child_process'
import { mkdtemp, readFile, rm } from 'node:fs/promises'
import os from 'node:os'
import path from 'node:path'
import { once } from 'node:events'
import assert from 'node:assert/strict'
const directory=await mkdtemp(path.join(os.tmpdir(),'mywebscrcpy-native-test-'))
const report=path.join(directory,'report.json')
const child=spawn(path.resolve('desktop/target/release/mywebscrcpy-gui'),[],{env:{...process.env,MYWEBSCRCPY_NATIVE_REPORT:report},stdio:['ignore','ignore','pipe']})
let errors='';child.stderr.on('data',data=>{errors=(errors+data.toString()).slice(-4096)})
const timer=setTimeout(()=>child.kill(),55000)
try {
 const [code]=await once(child,'exit')
 assert.equal(code,0,errors)
 const result=JSON.parse(await readFile(report,'utf8'))
 for (const key of ['tauri','updater_invokable','mounted','dependency_heading','close_hides','restore_visible']) assert.equal(result[key],true,`${key}: ${JSON.stringify(result)}`)
 assert.equal(result.updater_error,'')
 console.log('PASS: native WebView, updater IPC, private control, menu, hide/restore and clean App exit')
} finally {clearTimeout(timer);if(child.exitCode===null)child.kill();await rm(directory,{recursive:true,force:true})}
