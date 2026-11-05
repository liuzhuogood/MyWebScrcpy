import test from 'node:test'
import assert from 'node:assert/strict'
import { mkdtempSync, mkdirSync, writeFileSync, rmSync } from 'node:fs'
import { tmpdir } from 'node:os'
import path from 'node:path'
import { execFileSync } from 'node:child_process'
import { releaseNotes } from './prepare-release-notes.mjs'

function repo(t) {
 const root=mkdtempSync(path.join(tmpdir(),'release-notes-'))
 t.after(()=>rmSync(root,{recursive:true,force:true}))
 const git=(...args)=>execFileSync('git',args,{cwd:root,stdio:'pipe'}).toString()
 git('init');git('config','user.email','test@example.invalid');git('config','user.name','release test')
 writeFileSync(path.join(root,'base.txt'),'base');git('add','base.txt');git('commit','-m','initial');git('tag','v1.0.0')
 return {root,git}
}
test('uses the exact committed release document, never a later planning feature',t=>{
 const {root,git}=repo(t)
 for (const [dir,text] of [['v1.0.1-fix','# v1.0.1 版本说明\n\n已实现：修正连接状态，验证与已知限制在此记录。\n'],['v1.35.0-feature','# 未来规划，尚未实现。']]) {
  mkdirSync(path.join(root,'docs/versions',dir),{recursive:true});writeFileSync(path.join(root,'docs/versions',dir,'version.md'),text)
 }
 git('add','docs');git('commit','-m','fix(连接)：修正状态')
 const result=releaseNotes({root,tag:'v1.0.1'})
 assert.match(result.text,/已实现/);assert.doesNotMatch(result.text,/未来规划/)
 writeFileSync(path.join(root,result.source),'')
 assert.match(releaseNotes({root,tag:'v1.0.1'}).text,/已实现/)
 assert.throws(()=>releaseNotes({root,tag:'v1.0.1',workingTree:true}),/为空/)
})
test('missing document produces factual categorized notes from the release baseline',t=>{
 const {root,git}=repo(t)
 writeFileSync(path.join(root,'new.txt'),'actual change');git('add','new.txt');git('commit','-m','feat(连接)：支持范围扫描')
 const result=releaseNotes({root,tag:'v1.0.1',baseTag:'v1.0.0'})
 assert.match(result.text,/## 新增/);assert.match(result.text,/支持范围扫描/);assert.match(result.text,/new.txt/)
 assert.match(result.text,/不能从提交记录推断/)
 assert.throws(()=>releaseNotes({root,tag:'v1.0.0',target:'v1.0.0',baseTag:'v1.0.0'}),/没有可生成/)
})
