// Continue static-video-browser.mjs in its existing task-owned page.
const page=(await taskSpace('MyWebScrcpy 静止画面回退排查')).page('p1')
const assert=(await import('node:assert/strict')).default
const fs=await import('node:fs/promises')
await page.waitForFunction(()=>window.__staticFrames.length>=10&&performance.now()-window.__staticFrames[9].at>10000,undefined,{timeout:20000})
const steady=await page.evaluate(async()=>({frames:window.__staticFrames,status:await(await fetch('/api/static-video/status')).json()}))
assert.equal(steady.status.connections,1,'healthy static video must not reconnect')
assert.ok(steady.frames.at(-1).pixel[2]>200&&steady.frames.at(-1).pixel[0]<30,'latest blue image must remain')
await page.fetch('/api/static-video/drop',{method:'POST'})
await page.waitForFunction(async()=>{const s=await(await fetch('/api/static-video/status')).json();return s.connections>=2&&window.__staticFrames.length>10},undefined,{timeout:10000})
const recovered=await page.evaluate(()=>window.__staticFrames)
const firstBlue=recovered.findIndex(frame=>frame.pixel[2]>200&&frame.pixel[0]<30)
assert.ok(recovered.slice(firstBlue).every(frame=>frame.pixel[2]>200&&frame.pixel[0]<30),'real reconnect must never present cached old red frames')
// Mute the fixture's received heartbeats only: no image data remains. The
// unchanged watchdog must detect this silent transport and attempt recovery.
await page.evaluate(()=>window.__ignoreHeartbeats=true)
await page.waitForFunction(async()=>{const s=await(await fetch('/api/static-video/status')).json();return s.connections>=3},undefined,{timeout:10000})
await page.evaluate(()=>window.__ignoreHeartbeats=false)
const final=await page.evaluate(async()=>({frames:window.__staticFrames,status:await(await fetch('/api/static-video/status')).json(),player:window.__staticPlayer.statusText.value}))
assert.ok(final.frames.slice(firstBlue).every(frame=>frame.pixel[2]>200&&frame.pixel[0]<30))
await fs.writeFile('/Users/liuzhuo/code/MyWebScrcpy/.cache/verification/static-video/fixed.json',JSON.stringify({ok:true,steady,recovered,final},null,2)+'\n')
console.log({ok:true,healthyConnections:steady.status.connections,frames:final.frames.length,finalConnections:final.status.connections,finalPixel:final.frames.at(-1).pixel})
await page.evaluate(()=>window.__staticApp.unmount())
await page.fetch('/api/static-video/finish',{method:'POST'})
