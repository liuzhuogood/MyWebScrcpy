import { spawnSync } from 'node:child_process'
import { readdirSync, mkdtempSync, writeFileSync, rmSync } from 'node:fs'
import path from 'node:path'
import os from 'node:os'
import assert from 'node:assert/strict'
const bundle=path.resolve(process.argv[2] ?? 'desktop/target/release/bundle')
function files(dir){return readdirSync(dir,{withFileTypes:true}).flatMap(item=>item.isDirectory()?files(path.join(dir,item.name)):[path.join(dir,item.name)])}
const signatures=files(bundle).filter(file=>file.endsWith('.sig'))
assert.equal(signatures.length,1,'expected one signed updater artifact')
const deps=path.resolve('desktop/target/release/deps'), entries=readdirSync(deps)
const library=name=>path.join(deps,entries.find(file=>file.startsWith(`lib${name}-`)&&file.endsWith('.rlib')))
const temp=mkdtempSync(path.join(os.tmpdir(),'mywebscrcpy-signature-test-'))
const source=`use base64::{engine::general_purpose::STANDARD,Engine};
fn main(){
 let args:Vec<String>=std::env::args().collect();
 let data=std::fs::read(&args[1]).unwrap();
 let key=std::fs::read_to_string(&args[2]).unwrap();
 let sig=std::fs::read_to_string(&args[3]).unwrap();
 let key=String::from_utf8(STANDARD.decode(key.trim()).unwrap()).unwrap();
 let sig=String::from_utf8(STANDARD.decode(sig.trim()).unwrap()).unwrap();
 let key=minisign_verify::PublicKey::decode(&key).unwrap();
 let sig=minisign_verify::Signature::decode(&sig).unwrap();
 key.verify(&data,&sig,true).unwrap();
 let mut corrupt=data.clone();corrupt[0]^=1;
 assert!(key.verify(&corrupt,&sig,true).is_err());
 println!("PASS: update signature verified; corrupt package rejected");
}`
function run(command,args){const result=spawnSync(command,args,{stdio:'inherit'});assert.equal(result.status,0,`${command} failed`)}
try{
 const file=path.join(temp,'verify.rs'), executable=path.join(temp,process.platform==='win32'?'verify.exe':'verify')
 writeFileSync(file,source)
 run('rustc',['--edition=2021',file,'-L',`dependency=${deps}`,'--extern',`base64=${library('base64')}`,'--extern',`minisign_verify=${library('minisign_verify')}`,'-o',executable])
 run(executable,[signatures[0].slice(0,-4),path.resolve('desktop/updater.pub'),signatures[0]])
}finally{rmSync(temp,{recursive:true,force:true})}
