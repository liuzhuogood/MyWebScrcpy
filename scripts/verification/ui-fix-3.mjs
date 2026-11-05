// Run with ego-browser nodejs < scripts/verification/ui-fix-3.mjs.
// Uses the task-owned page and isolated Vite/Go ports, never the user's app.
const task = await taskSpace('MyWebScrcpy 修复恢复验证')
const page = task.page('p1')
const assert = (await import('node:assert/strict')).default
const fs = await import('node:fs/promises')
const output = '/Users/liuzhuo/code/MyWebScrcpy/.cache/verification/fix-3'
await page.goto('http://127.0.0.1:5190/')
await page.evaluate(async () => {
  const { useDeviceStore } = await import('/src/stores/devices.ts')
  const store = useDeviceStore()
  store.stopPolling()
  store.mockEnabled = false
  store.keyword = ''
  const fixture = [
    { serial: 'test-a', state: 'device', source: 'manual', model: 'A', screen_on: true, screen_locked: false },
    { serial: 'test-b', state: 'offline', source: 'auto', model: 'B' },
    { serial: 'test-c', state: 'device', source: 'manual', model: 'C', screen_on: false, screen_locked: true },
  ]
  window.__fix3 = { fixture, calls: [], wsCalls: [] }
  const original = window.fetch.bind(window)
  window.fetch = async (url, init = {}) => {
    const input = String(url)
    window.__fix3.calls.push({ url: input, method: init.method ?? 'GET', body: init.body })
    if (input === '/api/devices') return Response.json(window.__fix3.fixture)
    if (input.startsWith('/api/devices/test-') && init.method === 'DELETE') {
      const serial = decodeURIComponent(input.slice('/api/devices/'.length))
      window.__fix3.fixture = window.__fix3.fixture.filter(d => d.serial !== serial)
      return Response.json({ ok: true })
    }
    if (input.startsWith('/api/adb/commands?serial=test-')) return Response.json({ status: 'completed', exit_code: 0 })
    if (input.startsWith('/api/screen-state?serial=test-')) return Response.json({ screenOn: false, screenLocked: true })
    return original(url, init)
  }
  const NativeWebSocket = window.WebSocket
  window.WebSocket = class {
    static OPEN = 1; static CONNECTING = 0; static CLOSED = 3
    constructor(url) {
      if (!String(url).includes('serial=test-')) return new NativeWebSocket(url)
      this.readyState = 0
      this.timer = setTimeout(() => {
        this.readyState = 1
        this.onopen?.(new Event('open'))
        this.onmessage?.(new MessageEvent('message', {data: JSON.stringify({type:'meta', codec:'h264', width:1080, height:2400})}))
      }, 30)
    }
    send(data) { window.__fix3.wsCalls.push(data.byteLength ?? 0) }
    close() { clearTimeout(this.timer); this.readyState = 3 }
  }
  await store.load(true)
  store.reorder(['test-a','test-b','test-c'])
})
await page.waitForSelector('[data-home-device-card][data-device-serial="test-c"]')
// Full-list mapping must retain an offline 02 while the online card remains 03.
await page.click('text="设备"')
const initial = await page.evaluate(() => ({
  card: document.querySelector('[data-home-device-card][data-device-serial="test-c"]').innerText,
  number: document.querySelector('[data-device-grid] [data-device-serial="test-c"]').innerText,
}))
assert.match(initial.card, /^03/)
assert.equal(initial.number, '03')
await page.click('[data-device-grid] [data-device-serial="test-b"]')
await page.click('[data-device-grid] [data-device-serial="test-c"]')
assert.equal(await page.evaluate(() => document.querySelector('[data-home-device-card][data-device-serial="test-c"]').classList.contains('ring-2')), true)
assert.equal(await page.evaluate(() => document.querySelector('[data-home-device-card][data-device-serial="test-a"]').classList.contains('ring-2')), false)
await page.click('text="只显示已选"')
assert.deepEqual(await page.evaluate(() => [...document.querySelectorAll('[data-home-device-card]')].map(e => e.dataset.deviceSerial)), ['test-c'])
await page.click('text="显示全部"')
await page.click('button[aria-label="关闭"]')
await page.dragAndDrop('[data-device-serial="test-c"] [data-sort-handle]', '[data-device-serial="test-a"] [data-sort-handle]')
const reordered = await page.evaluate(() => [...document.querySelectorAll('[data-home-device-card]')].map(e => ({serial:e.dataset.deviceSerial, text:e.innerText, selected:e.classList.contains('ring-2')})))
assert.equal(reordered[0].serial, 'test-c')
assert.match(reordered[0].text, /^01/)
assert.equal(reordered[0].selected, true)
assert.deepEqual(JSON.parse(await page.evaluate(() => localStorage.getItem('home_device_order'))), ['test-c','test-b','test-a'])
// Both menu surfaces use serial, independently of the current number.
await page.click('[data-home-device-card][data-device-serial="test-c"]', {button:'right'})
await page.click('text="隐藏"')
await page.waitForSelector('[data-home-device-card][data-device-serial="test-c"]', {state:'hidden'})
await page.click('text="设备"')
await page.click('text="显示全部"')
assert.equal(await page.evaluate(() => !!document.querySelector('[data-home-device-card][data-device-serial="test-c"]')), false)
await page.click('text="显示隐藏设备"')
await page.waitForSelector('[data-home-device-card][data-device-serial="test-c"]')
await page.click('[data-device-grid] [data-device-serial="test-c"]', {button:'right'})
assert.ok((await page.snapshot()).includes('进入详情'))
await page.keyboard.press('Escape')
await page.click('[data-device-grid] [data-device-serial="test-c"]')
await page.click('button[aria-label="关闭"]')
await page.click('text="操作"')
await page.click('text="锁屏"')
await page.waitForFunction(() => window.__fix3.calls.some(c => c.body?.includes('KEYCODE_SLEEP')))
const lockCalls = await page.evaluate(() => window.__fix3.calls.filter(c => c.body?.includes('KEYCODE_SLEEP')))
assert.equal(lockCalls.length, 1)
assert.match(lockCalls[0].url, /serial=test-c$/)
assert.ok((await page.snapshot()).includes('锁屏'))
await page.click('[data-home-device-card][data-device-serial="test-c"] [data-sort-handle]')
assert.ok((await page.snapshot()).includes('锁屏'), 'outside click must retain panel')
await page.click('button[aria-label="关闭"]')
await page.click('[data-home-device-card][data-device-serial="test-a"]', {button:'right'})
await page.click('text="删除"')
await page.waitForSelector('[data-home-device-card][data-device-serial="test-a"]', {state:'hidden'})
assert.equal(await page.evaluate(() => window.__fix3.calls.filter(c => c.method === 'DELETE' && c.url === '/api/devices/test-a').length), 1)
// Restore A in the fixture, not in the user's backend.
await page.evaluate(async () => { const {useDeviceStore}=await import('/src/stores/devices.ts');window.__fix3.fixture.push({serial:'test-a',state:'device',model:'A',source:'manual'});useDeviceStore().devices=[...window.__fix3.fixture] })
await page.waitForSelector('[data-home-device-card][data-device-serial="test-a"]')
const afterInsert = await page.evaluate(async () => {
 const { useDeviceStore } = await import('/src/stores/devices.ts')
 return { dom:[...document.querySelectorAll('[data-home-device-card]')].map(e=>e.dataset.deviceSerial), data:useDeviceStore().filtered.filter(d=>d.state==='device').map(d=>d.serial) }
})
assert.deepEqual(afterInsert.dom, afterInsert.data, 'reinsertion after sorting must preserve visual order')
await page.click('button[aria-label="系统设置"]')
const shared = await page.evaluate(async () => {
  const { effectScope } = await import('/node_modules/.vite/deps/vue.js')
  const { usePlayerSettings } = await import('/src/composables/usePlayerSettings.ts')
  const first = effectScope(), second = effectScope()
  const a = first.run(() => usePlayerSettings()), b = second.run(() => usePlayerSettings())
  const equal = a.bitrate === b.bitrate && a.homePhoneScale === b.homePhoneScale
  first.stop(); second.stop(); return equal
})
assert.equal(shared, true)
await page.evaluate(() => {
  window.__settingsNodes = [...document.querySelectorAll('[role=dialog] [data-slot=select-trigger]')]
  window.__settingsSamples=[]
  document.addEventListener('pointerdown', e => {
    const tab=e.target.closest('[role=tab]');if(!tab)return
    const start=performance.now()
    requestAnimationFrame(()=>requestAnimationFrame(()=>window.__settingsSamples.push({tab:tab.textContent,ms:performance.now()-start})))
  }, {capture:true})
})
await page.cdp('Emulation.setCPUThrottlingRate', {rate:4})
try {
  for (const tab of ['投屏设置','通用设置','投屏设置','服务','投屏设置','依赖','服务','投屏设置']) await page.click(`text="${tab}"`)
} finally { await page.cdp('Emulation.setCPUThrottlingRate', {rate:1}) }
await page.waitForFunction(() => window.__settingsSamples.length >= 8)
const settings = await page.evaluate(() => ({samples:window.__settingsSamples, retained:window.__settingsNodes.every(e=>e.isConnected), noFrameToggle:!document.querySelector('[role=dialog] [aria-label*="无边框"]')}))
assert.equal(settings.retained, true)
assert.equal(settings.noFrameToggle, true)
await page.screenshot({path:output+'/settings.png'})
// Keep the page and server available for remaining device-discovery and layout checks.
await fs.writeFile(output+'/ui-home-settings.json', JSON.stringify({ok:true,initial,reordered,afterInsert,lockCalls,shared,settings},null,2)+'\n')
console.log({ok:true,report:output+'/ui-home-settings.json',settings})
console.log((await page.snapshot()).split('\n').filter(l => /tab |投屏设置|通用设置/.test(l)).join('\n'))
