// Continuation of ui-fix-3.mjs in the same task-owned Ego page.
const task = await taskSpace('MyWebScrcpy 修复恢复验证')
const page = task.page('p1')
const assert = (await import('node:assert/strict')).default
const fs = await import('node:fs/promises')
const output = '/Users/liuzhuo/code/MyWebScrcpy/.cache/verification/fix-3'
await page.click('[role=dialog] [data-slot=dialog-close]')
await page.evaluate(() => {
  const original = window.fetch
  window.__discoveryJobs = []
  window.fetch = async (url, init = {}) => {
    const response = await original(url, init)
    if (String(url).startsWith('/api/device-discovery/') && response.ok) {
      const body = await response.clone().json()
      if (body.id) window.__discoveryJobs.push({id:body.id, kind:body.kind, state:body.state, method:init.method ?? 'GET'})
    }
    return response
  }
})
await page.click('text="设备"')
await page.click('text="添加"')
await page.waitForFunction(() => {
  const dialog = document.querySelector('[role=dialog]')
  return dialog?.getBoundingClientRect().height === 620 && !!dialog.querySelector('img[src^="data:image/png"]')
})
const initial = await page.evaluate(() => ({
  width:document.querySelector('[role=dialog]').getBoundingClientRect().width,
  height:document.querySelector('[role=dialog]').getBoundingClientRect().height,
  preloaded:!!document.querySelector('[role=dialog] img'),
}))
const dimensions = []
for (const tab of ['扫码配对','范围扫描','地址','扫码配对','范围扫描']) {
  await page.click(`text="${tab}"`)
  dimensions.push(await page.evaluate(() => {
    const rect=document.querySelector('[role=dialog]').getBoundingClientRect()
    return {width:rect.width,height:rect.height,y:document.querySelector('[role=tab]').getBoundingClientRect().y}
  }))
}
assert.ok(dimensions.every(d => d.width === initial.width && d.height === initial.height && d.y === dimensions[0].y))
const defaults = await page.evaluate(() => ({
  start:[...document.querySelectorAll('input[aria-label^="起始"]')].map(e=>e.value).join('.'),
  end:[...document.querySelectorAll('input[aria-label^="结束"]')].map(e=>e.value).join('.'),
}))
const networks = JSON.parse((await page.fetch('/api/device-discovery/networks')).body)
assert.ok(networks.some(n=>n.start === defaults.start && n.end === defaults.end))
async function fillRange(start,end) {
  for (const [label,value] of [['起始',start],['结束',end]]) {
    for (const [index,part] of value.split('.').entries()) await page.fill(`input[aria-label="${label} IP 第 ${index+1} 段"]`, part)
  }
}
await fillRange('127.0.0.1','127.0.0.1')
await page.click('text="添加范围"')
await fillRange('127.0.0.2','127.0.0.2')
await page.click('text="添加范围"')
await page.click('button[aria-label="删除范围 127.0.0.1-127.0.0.1"]')
await fillRange('127.0.0.1','127.0.0.1')
await page.click('text="添加范围"')
await page.fill('#scan-ports', '1')
await page.click('text="开始扫描"')
await page.waitForFunction(() => [...document.querySelectorAll('[role=status]')].some(e=>e.textContent.includes('已结束 2/2')))
const scan = await page.evaluate(() => window.__fix3.calls.filter(c=>c.url==='/api/device-discovery/scan').map(c=>JSON.parse(c.body)))
assert.deepEqual(scan.at(-1).ranges, ['127.0.0.2-127.0.0.2','127.0.0.1-127.0.0.1'])
await page.click('text="扫码配对"')
assert.equal(await page.evaluate(() => !!document.querySelector('[role=dialog] img')), true, 'scan must preserve QR lease')
await page.click('text="停止"')
await page.waitForFunction(() => !document.querySelector('[role=dialog] img'))
await page.click('text="重新生成"')
await page.waitForFunction(() => !!document.querySelector('[role=dialog] img'))
await page.click('text="取消"')
await page.waitForFunction(() => window.__discoveryJobs.filter(j=>j.kind==='pair'&&j.method==='DELETE').length >= 2)
const jobs = await page.evaluate(() => window.__discoveryJobs)
// Lock marker is driven by the real Device contract, with no live mock streams.
await page.click('text="视图"')
console.log((await page.snapshot()).split('\n').filter(l=>/button|手机|卡片/.test(l)).slice(-25).join('\n'))
await fs.writeFile(output+'/ui-discovery.json', JSON.stringify({ok:true,initial,dimensions,defaults,networks,scan,jobs},null,2)+'\n')
console.log({ok:true,report:output+'/ui-discovery.json'})
