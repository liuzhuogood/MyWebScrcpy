const fs=require('node:fs');const vm=require('node:vm');const assert=require('node:assert/strict');const ts=require('typescript');
const moduleObject={exports:{}};const decoders=[];const drawn=[];
class FakeDecoder {
 constructor(callbacks){this.callbacks=callbacks;this.state='unconfigured';this.chunks=[];decoders.push(this)}
 configure(){this.state='configured'} reset(){this.state='unconfigured'} close(){this.state='closed'}
 decode(chunk){this.chunks.push(chunk)}
}
vm.runInNewContext(ts.transpileModule(fs.readFileSync(__dirname+'/decoder.ts','utf8'),{compilerOptions:{module:ts.ModuleKind.CommonJS,target:ts.ScriptTarget.ES2022}}).outputText,{
 module:moduleObject,exports:moduleObject.exports,console:{log(){},warn(){},error(){}},performance:{now:()=>2000},VideoDecoder:FakeDecoder,EncodedVideoChunk:class{constructor(args){Object.assign(this,args)}},Uint8Array,DataView,
});
const Decoder=moduleObject.exports.ScrcpyDecoder;
const canvas={width:160,height:96,getContext:()=>({drawImage(frame){drawn.push(frame.timestamp)}})};
const d=new Decoder(canvas);let notifications=0;d.onDecodedFrame=()=>notifications++;
const packet=(kind,pts)=>{const data=new Uint8Array(13);data[0]=kind;new DataView(data.buffer).setBigUint64(1,BigInt(pts));data.set([1,2,3,4],9);return data};
const output=(decoder,pts)=>{const frame={timestamp:pts,closed:false,close(){this.closed=true}};decoder.callbacks.output(frame);assert.equal(frame.closed,true,'all VideoFrames must be released')};
d.configure('h264',160,96,'same-session');d.handleFrame(packet(0,0));d.handleFrame(packet(1,100));output(decoders.at(-1),100);output(decoders.at(-1),500);
const previous=decoders.at(-1);d.configure('h264',160,96,'same-session');d.handleFrame(packet(0,0));d.handleFrame(packet(1,100));output(decoders.at(-1),100);output(decoders.at(-1),300);output(decoders.at(-1),500);
assert.deepEqual(drawn,[100,500,500],'cached key/deltas from the same source must not rewind the presented image');
output(previous,700);assert.deepEqual(drawn,[100,500,500],'late output from a closed decoder cannot overwrite the new one');
d.configure('h264',160,96,'new-session');d.handleFrame(packet(0,0));d.handleFrame(packet(2,1));assert.equal(decoders.at(-1).chunks.length,0,'wait for key after config');d.handleFrame(packet(1,1));output(decoders.at(-1),1);assert.equal(drawn.at(-1),1,'a genuinely new stream may restart PTS');
d.handleFrame(packet(0,0));d.handleFrame(packet(1,0));output(decoders.at(-1),0);assert.equal(drawn.at(-1),0,'in-stream encoder/replay reset must remain supported');
const active=decoders.at(-1);d.close();output(active,900);assert.equal(drawn.at(-1),0);assert.equal(notifications,5);
console.log('PASS: monotonic presentation per session, stale decoder disposal, new epochs, wait-for-key and VideoFrame release');
