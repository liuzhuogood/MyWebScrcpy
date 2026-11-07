const fs=require('node:fs');const vm=require('node:vm');const assert=require('node:assert/strict');const ts=require('typescript');
const moduleObject={exports:{}};const decoders=[];const drawn=[];
let now=2000,nextSchedule=0;const animationFrames=new Map(),timers=new Map();
const flushAnimationFrame=()=>{now+=16;const jobs=[...animationFrames.values()];animationFrames.clear();for(const run of jobs)run(now)};
const flushTimers=()=>{now+=100;const jobs=[...timers.values()];timers.clear();for(const run of jobs)run()};
class FakeDecoder {
 constructor(callbacks){this.callbacks=callbacks;this.state='unconfigured';this.chunks=[];decoders.push(this)}
 configure(){this.state='configured'} reset(){this.state='unconfigured'} close(){this.state='closed'}
 decode(chunk){this.chunks.push(chunk)}
}
vm.runInNewContext(ts.transpileModule(fs.readFileSync(__dirname+'/decoder.ts','utf8'),{compilerOptions:{module:ts.ModuleKind.CommonJS,target:ts.ScriptTarget.ES2022}}).outputText,{
 module:moduleObject,exports:moduleObject.exports,console:{log(){},warn(){},error(){}},performance:{now:()=>now},requestAnimationFrame:run=>{const id=++nextSchedule;animationFrames.set(id,run);return id},cancelAnimationFrame:id=>animationFrames.delete(id),setTimeout:run=>{const id=++nextSchedule;timers.set(id,run);return id},clearTimeout:id=>timers.delete(id),VideoDecoder:FakeDecoder,EncodedVideoChunk:class{constructor(args){Object.assign(this,args)}},Uint8Array,DataView,
});
const Decoder=moduleObject.exports.ScrcpyDecoder;
const canvas={width:160,height:96,getContext:()=>({drawImage(frame){drawn.push(frame.timestamp)}})};
const d=new Decoder(canvas);let notifications=0;d.onDecodedFrame=()=>notifications++;
const packet=(kind,pts)=>{const data=new Uint8Array(13);data[0]=kind;new DataView(data.buffer).setBigUint64(1,BigInt(pts));data.set([1,2,3,4],9);return data};
const output=(decoder,pts,flush=true)=>{const frame={timestamp:pts,closed:false,closeCount:0,close(){this.closed=true;this.closeCount++}};decoder.callbacks.output(frame);if(flush){flushAnimationFrame();assert.equal(frame.closed,true,'all VideoFrames must be released')}return frame};
d.configure('h264',160,96,'same-session');d.handleFrame(packet(0,0));d.handleFrame(packet(1,100));output(decoders.at(-1),100);output(decoders.at(-1),500);
const previous=decoders.at(-1);d.configure('h264',160,96,'same-session');d.handleFrame(packet(0,0));d.handleFrame(packet(1,100));output(decoders.at(-1),100);output(decoders.at(-1),300);output(decoders.at(-1),500);
assert.deepEqual(drawn,[100,500,500],'cached key/deltas from the same source must not rewind the presented image');
output(previous,700);assert.deepEqual(drawn,[100,500,500],'late output from a closed decoder cannot overwrite the new one');
d.configure('h264',160,96,'new-session');d.handleFrame(packet(0,0));d.handleFrame(packet(2,1));assert.equal(decoders.at(-1).chunks.length,0,'wait for key after config');d.handleFrame(packet(1,1));output(decoders.at(-1),1);assert.equal(drawn.at(-1),1,'a genuinely new stream may restart PTS');
d.handleFrame(packet(0,0));d.handleFrame(packet(1,0));output(decoders.at(-1),0);assert.equal(drawn.at(-1),0,'in-stream encoder/replay reset must remain supported');
const active=decoders.at(-1);d.close();output(active,900);assert.equal(drawn.at(-1),0);assert.equal(notifications,5);
console.log('PASS: monotonic presentation per session, stale decoder disposal, new epochs, wait-for-key and VideoFrame release');

// A decoder burst must retain every reference for decoding, but only paint the
// newest completed image at the next display opportunity.
d.configure('h264',160,96,'burst-session');d.handleFrame(packet(0,0));
for(const [kind,pts] of [[1,100],[2,200],[2,300]])d.handleFrame(packet(kind,pts));
assert.equal(decoders.at(-1).chunks.length,3,'encoded reference frames must all be decoded');
const beforeBurst=drawn.length;
const first=output(decoders.at(-1),100,false),second=output(decoders.at(-1),200,false),latest=output(decoders.at(-1),300,false);
assert.equal(drawn.length,beforeBurst,'burst outputs wait for one display opportunity');
assert.equal(first.closed,true);assert.equal(second.closed,true);assert.equal(latest.closed,false);
assert.equal(animationFrames.size,1,'schedule only one animation frame');
assert.equal(timers.size,1,'schedule only one bounded fallback');
flushAnimationFrame();
assert.deepEqual(drawn.slice(beforeBurst),[300],'skip superseded decoded images');
assert.equal(latest.closeCount,1);assert.equal(timers.size,0,'successful presentation cancels fallback');

// No further video packets are needed to display the final still image. A
// background document whose RAF is suspended must also keep progressing.
const lastStill=output(decoders.at(-1),400,false);
flushTimers();
assert.equal(drawn.at(-1),400,'fallback paints final static image without more packets');
assert.equal(lastStill.closeCount,1);assert.equal(animationFrames.size,0);

const pendingNewer=output(decoders.at(-1),600,false),outOfOrder=output(decoders.at(-1),500,false);
assert.equal(outOfOrder.closed,true,'late older decoded image must not replace newer pending image');
flushAnimationFrame();assert.equal(drawn.at(-1),600);assert.equal(pendingNewer.closeCount,1);
const beforeReset=drawn.length;
const resetPending=output(decoders.at(-1),700,false),lateJobs=[...animationFrames.values(),...timers.values()];
d.handleFrame(packet(0,0));
assert.equal(resetPending.closeCount,1,'encoder config releases queued presentation');
d.handleFrame(packet(1,1));const newEpoch=output(decoders.at(-1),1,false);
for(const run of lateJobs)run();
assert.equal(drawn.length,beforeReset,'canceled callback cannot present previous encoder image or flush a new epoch early');
assert.equal(newEpoch.closed,false,'stale scheduled callbacks cannot release the new epoch pending frame');
flushAnimationFrame();assert.equal(drawn.at(-1),1);assert.equal(newEpoch.closeCount,1);

const resizePending=output(decoders.at(-1),20,false);
const resize=new Uint8Array(17);resize[0]=3;const sizeView=new DataView(resize.buffer);sizeView.setUint32(9,96);sizeView.setUint32(13,160);
d.handleFrame(resize);flushAnimationFrame();flushTimers();
assert.equal(resizePending.closeCount,1);assert.equal(drawn.at(-1),1,'rotation cannot paint pending old dimensions');

const closing=output(decoders.at(-1),30,false),closingJobs=[...animationFrames.values(),...timers.values()];
const beforeClose=drawn.length;d.close();for(const run of closingJobs)run();
assert.equal(closing.closeCount,1);assert.equal(drawn.length,beforeClose);
assert.equal(animationFrames.size,0);assert.equal(timers.size,0);
console.log('PASS: latest decoded image wins, encoded dependency chain retained, bounded static fallback and pending frame disposal');
