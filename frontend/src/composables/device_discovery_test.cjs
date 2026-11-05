const fs = require('node:fs');
const path = require('node:path');
const vm = require('node:vm');
const assert = require('node:assert/strict');
const { createRequire } = require('node:module');
const req = createRequire(path.resolve(__dirname, '../../package.json'));
const ts = req('typescript'), vue = req('vue');
const settle = () => new Promise(r => setImmediate(r));
function fixture() {
 const timers=[], unmounts=[], stops=[], calls=[]; let release, hold=false;
 let nextState='running';
 const module={exports:{}};
 const code=ts.transpileModule(fs.readFileSync(path.join(__dirname,'useDeviceDiscovery.ts'),'utf8'),{compilerOptions:{module:ts.ModuleKind.CommonJS,target:ts.ScriptTarget.ES2022,esModuleInterop:true}}).outputText;
 vm.runInNewContext(code,{
  module,exports:module.exports,AbortController,console:{warn(){}},setTimeout: fn=>{timers.push(fn);return fn},clearTimeout:fn=>{const index=timers.indexOf(fn);if(index>=0)timers.splice(index,1)},
  require(name){
   if(name==='vue')return {...vue,onUnmounted:fn=>unmounts.push(fn)};
   if(name==='qrcode')return {toDataURL:async payload=>'image:'+payload};
   if(name==='@/api/discovery')return {
    startPairing:async()=>{calls.push('pair');if(hold)await new Promise(r=>release=r);return {id:'pair-1',kind:'pair',state:'running',qr:'WIFI:T:ADB;S:session;P:secret;;',found:[]}},
    startScan:async body=>{calls.push(body);return {id:'scan-1',kind:'scan',state:'running',found:[]}},
    pollDiscovery:async id=>({id,kind:id.startsWith('pair')?'pair':'scan',state:nextState,found:[]}),
    stopDiscovery:async id=>{stops.push(id)},
   };
   throw new Error(name);
  }
 });
 return {use:module.exports.useDiscoverySession,timers,stops,calls,unmounts,hold:()=>hold=true,release:()=>release(),state:value=>nextState=value};
}
(async()=>{
 const f=fixture(),open=vue.ref(true);let completed=0;
 const pair=f.use(open,'pair',async()=>{completed++});await settle();
 assert.ok(pair.qrImage.value.includes('WIFI:T:ADB;'),'opening dialog must preload real QR payload');
 const scan=f.use(open,'scan',async()=>{});await scan.start({ranges:['127.0.0.1-127.0.0.1'],ports:[5555]});await settle();
 assert.equal(pair.running.value,true,'scan must not destroy pairing session');
 f.state('failed');f.timers.shift()();await settle();
 assert.equal(pair.qrImage.value,'','terminal poll must remove expired QR');assert.equal(completed,1);
 open.value=false;await vue.nextTick();await settle();assert.ok(f.stops.includes('scan-1'));
 const stale=fixture(),visible=vue.ref(false);stale.hold();const session=stale.use(visible,'pair',async()=>{});
 visible.value=true;await vue.nextTick();await settle();visible.value=false;await vue.nextTick();stale.release();await settle();
 assert.equal(session.job.value,null);assert.equal(session.qrImage.value,'');assert.ok(stale.stops.includes('pair-1'),'late-created sessions must be stopped');
 for(const unmount of [...f.unmounts,...stale.unmounts])unmount();
 console.log('PASS: QR preload, independent scan, expiry, close, stale-create cleanup');
})().catch(error=>{console.error(error);process.exitCode=1});
