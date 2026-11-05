// Uses only the test-* synthetic WebSocket/HTTP fixtures from ui-fix-3.mjs.
const task = await taskSpace('MyWebScrcpy 修复恢复验证')
const page = task.page('p1')
const assert = (await import('node:assert/strict')).default
const fs = await import('node:fs/promises')
const output = '/Users/liuzhuo/code/MyWebScrcpy/.cache/verification/fix-3'
await page.waitForFunction(() => document.querySelector('canvas[aria-label="设备投屏画面"]')?.width === 1080)
const before = await page.evaluate(() => document.querySelector('canvas[aria-label="设备投屏画面"]').getBoundingClientRect().toJSON())
await page.hover('button[aria-label="展开工具条"]')
await page.waitForSelector('button[aria-label="关闭小窗"]')
const overlay = await page.evaluate(() => {
 const canvas=document.querySelector('canvas[aria-label="设备投屏画面"]')
 const stage=canvas.closest('[data-player-viewport]').getBoundingClientRect()
 const header=document.querySelector('button[aria-label="关闭小窗"]').closest('header')
 return {canvas:canvas.getBoundingClientRect().toJSON(),stage:stage.toJSON(),position:getComputedStyle(header).position,background:getComputedStyle(header).backgroundColor}
})
assert.equal(overlay.position,'absolute')
assert.equal(overlay.canvas.width,before.width)
assert.equal(overlay.canvas.height,before.height)
assert.ok(Math.abs(overlay.canvas.width-overlay.stage.width)<1)
assert.ok(Math.abs(overlay.canvas.height-overlay.stage.height)<1)
await page.screenshot({path:output+'/web-player.png'})
await page.click('button[aria-label="关闭小窗"]')
await page.waitForSelector('canvas[aria-label="设备投屏画面"]', {state:'hidden'})
await page.evaluate(() => {
 const original=window.fetch
 window.fetch=(url,init)=>String(url).startsWith('/api/screen-state?serial=test-')?Promise.resolve(Response.json({screenOn:true,screenLocked:false})):original(url,init)
})
await page.click('[data-home-device-card][data-device-serial="test-c"]',{button:'right'})
await page.click('text="进入详情"')
await page.waitForURL('**/player?serial=test-c')
await page.waitForFunction(()=>document.querySelector('canvas[aria-label="设备投屏画面"]')?.width===1080)
const detail=await page.evaluate(()=>{
 const canvas=document.querySelector('canvas[aria-label="设备投屏画面"]'),c=canvas.getBoundingClientRect(),v=canvas.closest('[data-player-viewport]').getBoundingClientRect()
 return {canvas:c.toJSON(),stage:v.toJSON(),top:c.top-v.top,bottom:v.bottom-c.bottom,ratio:c.width/c.height}
})
assert.ok(detail.top>=32&&detail.bottom>=32, JSON.stringify(detail))
assert.ok(Math.abs(detail.ratio-1080/2400)<.003)
await page.screenshot({path:output+'/detail-player.png'})
await fs.writeFile(output+'/ui-player-layout.json',JSON.stringify({ok:true,syntheticVideo:true,overlay,detail},null,2)+'\n')
console.log({ok:true,overlay,detail})
