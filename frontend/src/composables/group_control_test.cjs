const fs = require('node:fs');
const path = require('node:path');
const vm = require('node:vm');
const assert = require('node:assert/strict');
const ts = require('typescript');
const vue = require('vue');
const code = ts.transpileModule(fs.readFileSync(path.join(__dirname, 'useGroupControl.ts'), 'utf8'), {
  compilerOptions: { module: ts.ModuleKind.CommonJS, target: ts.ScriptTarget.ES2022 },
}).outputText;
const scrcpyModules = new Map();
function loadScrcpy(name) {
  if (scrcpyModules.has(name)) return scrcpyModules.get(name);
  const module = {exports:{}};
  vm.runInNewContext(ts.transpileModule(fs.readFileSync(path.join(__dirname,'../scrcpy',name+'.ts'),'utf8'), {
    compilerOptions:{module:ts.ModuleKind.CommonJS,target:ts.ScriptTarget.ES2022},
  }).outputText,{module,exports:module.exports,Uint8Array,DataView,require:name=>loadScrcpy(name.replace('./',''))});
  scrcpyModules.set(name,module.exports);return module.exports;
}
const buses = new Set();
class Bus {
  constructor() { buses.add(this); }
  postMessage(data) { for (const other of buses) if (other !== this) other.onmessage?.({ data: structuredClone(data) }); }
  close() { buses.delete(this); }
}
class Socket {
  static OPEN = 1;
  static sockets = [];
  constructor(url) { this.url=url; this.readyState=0; this.bufferedAmount=0; this.sent=[]; Socket.sockets.push(this); }
  ready(width=720,height=1600) { this.readyState=1; this.onmessage?.({data:JSON.stringify({type:'meta',width,height})}); }
  send(bytes) { this.sent.push(bytes); }
  close() { this.readyState=3; this.onclose?.(); }
}
function realm(browser = false, timerOverrides = {}) {
  const module = { exports: {} }; const unload=[];
  const context = {
    module, exports:module.exports, Uint8Array, ArrayBuffer, performance: { now: () => 0 },
    console:{warn(){},error(){}}, location:{protocol:'http:',host:'localhost'},
    setTimeout,clearTimeout,setInterval,clearInterval,
    require(name) {
      if(name==='vue')return vue;
      if(name==='../scrcpy/controlSender')return loadScrcpy('controlSender');
      throw Error(name);
    },
  };
  Object.assign(context,timerOverrides);
  if(browser)Object.assign(context,{window:{addEventListener(type,cb){unload.push(cb)},setTimeout:context.setTimeout,clearTimeout:context.clearTimeout},BroadcastChannel:Bus,WebSocket:Socket});
  vm.runInNewContext(code,context);
  return {...module.exports, unload(){unload.forEach(cb=>cb())}};
}
function touch(x=540,y=1200) {
  const b=new Uint8Array(32); const v=new DataView(b.buffer);
  b[0]=2;b[1]=2;v.setInt32(10,x);v.setInt32(14,y);v.setUint16(18,1080);v.setUint16(20,2400);return b;
}
function channel(serial, output, width=720,height=1600) {
  return {serial,send:b=>output.push(b),isReady:()=>true,getDeviceSize:()=>({width,height})};
}
const mod=realm(); const gc=mod.useGroupControl();
const input=touch();const outputs=[];const duplicates=[];
gc.registerChannel(channel('target',outputs));gc.registerChannel(channel('target',duplicates));
gc.setTargets(['target']);gc.setActive(true);
gc.broadcastControl('unselected-source',input,{width:1080,height:2400});
assert.equal(outputs.length,1,'unselected source synchronizes even with one selected target');
assert.equal(duplicates.length,0,'multiple views of one target never duplicate input');
let v=new DataView(outputs[0].buffer);assert.equal(v.getInt32(10),360);assert.equal(v.getInt32(14),800);
const bound=mod.transformTouchPacket(touch(1080,2400),{width:1080,height:2400},{width:720,height:1600});
v=new DataView(bound.buffer);assert.equal(v.getInt32(10),719);assert.equal(v.getInt32(14),1599);
assert.equal(new DataView(input.buffer).getInt32(10),540,'source packet stays unchanged');
const scroll=new Uint8Array(21);scroll[0]=3;const sv=new DataView(scroll.buffer);sv.setInt32(1,540);sv.setInt32(5,1200);
const mapped=mod.transformScrollPacket(scroll,{width:1080,height:2400},{width:720,height:1600});
assert.equal(new DataView(mapped.buffer).getInt32(1),360);
assert.equal(new DataView(mapped.buffer).getInt32(5),800);
gc.registerChannel({...channel('failed',[]),send(){throw Error('failed')}});
gc.setTargets(['failed','target']);gc.broadcastControl('unselected-source',new Uint8Array([0,1]));
assert.equal(outputs.length,2,'one failing target never blocks other targets');
gc.broadcastControl('target',input,{width:1080,height:2400});assert.equal(outputs.length,2,'source is never echoed');
gc.setActive(false);gc.broadcastControl('source',input);assert.equal(outputs.length,2);

// Real page realms share state and input through the same BroadcastChannel contract.
const main=realm(true);const owner=main.useGroupControl();const received=[];
owner.registerChannel(channel('target',received));owner.setTargets(['target']);owner.setActive(true);
const player=realm(true);const source=player.useGroupControl();
assert.equal(source.isActive.value,true,'a newly opened player discovers existing group state');
source.broadcastControl('not-selected',input,{width:1080,height:2400});assert.equal(received.length,1);
source.broadcastControl('not-selected',JSON.stringify({type:'input.hid',action:'tap',x:.5,y:.5}));
assert.equal(JSON.parse(received[1]).type,'input.hid','normalized HID gestures also synchronize');
const unregisterSource=source.registerChannel(channel('not-selected',[]));
const down=touch();down[1]=0;source.broadcastControl('not-selected',down,{width:1080,height:2400});
const heldCount=received.length;unregisterSource();
assert.equal(received.length,heldCount+1,'closing a source releases its held target input');
assert.equal(received.at(-1)[1],1,'source shutdown sends UP');
owner.setTargets([]);const stopped=received.length;source.broadcastControl('not-selected',input,{width:1080,height:2400});assert.equal(received.length,stopped);
owner.setTargets(['target']);owner.setActive(true);main.unload();assert.equal(source.isActive.value,false,'coordinator exit disables group');player.unload();

// Background control connections use bounded startup and never subscribe to video.
Socket.sockets=[];const bg=realm(true);const control=bg.useGroupControl();
control.setTargets(['a','b','c','d','e','f']);control.setActive(true);
assert.equal(Socket.sockets.length,4,'at most four connections start concurrently');
assert.ok(Socket.sockets.every(s=>s.url.includes('control_only=1')));
Socket.sockets[0].ready();assert.equal(Socket.sockets.length,5,'successful metadata frees one startup slot');
const first=Socket.sockets[0];const third=Socket.sockets[2];third.ready();
control.broadcastControl('external',input,{width:1080,height:2400});
assert.equal(first.sent.length,1);assert.equal(third.sent.length,1,'unready peers do not block ready peers');
control.broadcastControl('external',down,{width:1080,height:2400});
first.bufferedAmount=5000;
control.broadcastControl('external',input,{width:1080,height:2400});
assert.equal(first.readyState,3,'a 4KiB interactive backlog closes only the congested target');
assert.equal(first.sent.at(-1)[1],1,'congested target attempts to release its held touch');
assert.equal(third.readyState,1,'another ready target remains connected');
assert.equal(third.sent.at(-1)[1],2,'healthy target continues receiving the same input');
control.setActive(false);assert.ok(Socket.sockets.every(s=>s.readyState===3),'group shutdown closes every socket');bg.unload();
// Independent outages restore their retry budget; consecutive failed attempts remain bounded.
Socket.sockets=[];const timers=new Set();
const retryRealm=realm(true,{
  setTimeout(fn,delay){const timer={fn,delay};timers.add(timer);return timer},
  clearTimeout(timer){timers.delete(timer)},
});
const retryControl=retryRealm.useGroupControl();retryControl.setTargets(['recover']);retryControl.setActive(true);
function retryTick(){const timer=[...timers].find(t=>t.delay===1000);assert.ok(timer,'retry timer exists');timers.delete(timer);timer.fn()}
for(let i=0;i<4;i++){
  Socket.sockets.at(-1).ready();Socket.sockets.at(-1).close();retryTick();
  assert.equal(Socket.sockets.length,i+2,'successful reconnect resets the next outage budget');
}
Socket.sockets.at(-1).ready();
const startFailures=Socket.sockets.length;
for(let i=0;i<3;i++){Socket.sockets.at(-1).close();retryTick()}
assert.equal(Socket.sockets.length,startFailures+2,'consecutive failures stop after three attempts');
retryControl.setActive(false);retryRealm.unload();

// A small socket backlog cannot persist indefinitely, even after input stops.
Socket.sockets=[];let clock=0;const backlogTimers=new Set();
const backlogRealm=realm(true,{
  performance:{now:()=>clock},
  setTimeout(fn,delay){const timer={fn,delay};backlogTimers.add(timer);return timer},
  clearTimeout(timer){backlogTimers.delete(timer)},
});
const backlogControl=backlogRealm.useGroupControl();backlogControl.setTargets(['slow']);backlogControl.setActive(true);
const slow=Socket.sockets.at(-1);slow.ready();slow.bufferedAmount=32;
backlogControl.broadcastControl('external',down,{width:1080,height:2400});
const monitor=[...backlogTimers].find(t=>t.delay===16);assert.ok(monitor,'low-volume backlog has a watchdog');
clock=500;backlogTimers.delete(monitor);monitor.fn();
assert.equal(slow.readyState,3,'500ms of persistent backlog recovers a stalled background connection');
assert.equal(slow.sent.at(-1)[1],1,'watchdog releases the held touch before closing');
const oldCount=slow.sent.length;monitor.fn();assert.equal(slow.sent.length,oldCount,'late old timer cannot resend input');
backlogControl.setActive(false);backlogRealm.unload();

const oversized=realm().useGroupControl();assert.equal(oversized.setTargets(Array.from({length:1025},(_,i)=>String(i))),false);oversized.setActive(true);assert.equal(oversized.isActive.value,false);assert.ok(oversized.lastError.value.includes('1024'),'capacity errors are not cleared or reported as successful activation');
console.log('PASS: group control mapping, single-target, cross-window, failure isolation, bounded background connections');
