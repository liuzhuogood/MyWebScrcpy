#!/usr/bin/env node
import { spawn } from 'node:child_process'
import { mkdirSync, mkdtempSync, rmSync, writeFileSync } from 'node:fs'
import net from 'node:net'
import os from 'node:os'
import path from 'node:path'

// 解析命令行参数
const args = process.argv.slice(2)
function getArg(flag, fallback = '') {
  const idx = args.indexOf(flag)
  return idx !== -1 && args[idx + 1] ? args[idx + 1] : fallback
}
const hasFlag = (flag) => args.includes(flag)

const targetUrl = getArg('--url')
const outputDir = path.resolve(getArg('--out', './artifacts'))
const defaultName = getArg('--name', 'desktop.png')
const width = parseInt(getArg('--width', '1440'), 10)
const height = parseInt(getArg('--height', '900'), 10)
const tabs = getArg('--tabs', '').split(',').map(s => s.trim()).filter(Boolean)
const captureMobile = hasFlag('--mobile')
const captureDark = hasFlag('--dark')

if (!targetUrl) {
  console.error('用法: node capture.mjs --url <URL> [--out <DIR>] [--name <NAME>] [--tabs <tab1,tab2>] [--mobile] [--dark]')
  process.exit(1)
}

mkdirSync(outputDir, { recursive: true })

async function getAvailablePort() {
  return new Promise((resolve, reject) => {
    const srv = net.createServer()
    srv.listen(0, '127.0.0.1', () => {
      const port = srv.address().port
      srv.close(() => resolve(port))
    })
    srv.on('error', reject)
  })
}

const sleep = ms => new Promise(r => setTimeout(r, ms))

async function main() {
  const port = await getAvailablePort()
  const tempProfile = mkdtempSync(path.join(os.tmpdir(), 'proto-capture-'))

  const chromePath = '/Applications/Google Chrome.app/Contents/MacOS/Google Chrome'
  const chrome = spawn(chromePath, [
    '--headless=new',
    `--user-data-dir=${tempProfile}`,
    `--remote-debugging-port=${port}`,
    `--window-size=${width},${height}`,
    '--disable-gpu',
    '--no-first-run',
    '--no-default-browser-check',
    targetUrl,
  ], { stdio: 'ignore' })

  const cleanup = () => {
    try { chrome.kill() } catch (_) {}
    try { rmSync(tempProfile, { recursive: true, force: true }) } catch (_) {}
  }
  process.on('exit', cleanup)
  process.on('SIGINT', () => { cleanup(); process.exit(1) })

  // 等待 Chrome 启动 CDP 端口
  let pageTab = null
  for (let i = 0; i < 20; i++) {
    await sleep(300)
    try {
      const res = await fetch(`http://127.0.0.1:${port}/json`)
      const list = await res.json()
      pageTab = list.find(t => t.type === 'page' && (t.url.includes('5174') || t.url.includes('demo.wskj.work') || t.url === targetUrl))
      if (pageTab) break
    } catch (_) {}
  }

  if (!pageTab) {
    console.error('未能连接至 Chrome 页面')
    cleanup()
    process.exit(1)
  }

  const ws = new WebSocket(pageTab.webSocketDebuggerUrl)
  let msgId = 1
  const pending = new Map()

  ws.onmessage = (event) => {
    const msg = JSON.parse(event.data)
    if (msg.id && pending.has(msg.id)) {
      pending.get(msg.id)(msg.result)
      pending.delete(msg.id)
    }
  }

  await new Promise(resolve => ws.onopen = resolve)

  function send(method, params = {}) {
    return new Promise(resolve => {
      const id = msgId++
      pending.set(id, resolve)
      ws.send(JSON.stringify({ id, method, params }))
    })
  }

  await send('Page.enable')
  await send('Runtime.enable')

  // 等待 Vue 应用挂载
  for (let i = 0; i < 40; i++) {
    const res = await send('Runtime.evaluate', {
      expression: 'document.querySelector("#app")?.children?.length || 0',
      returnByValue: true,
    })
    if (res?.result?.value > 0) break
    await sleep(400)
  }
  await sleep(600)

  async function takeScreenshot(filename) {
    const res = await send('Page.captureScreenshot', { format: 'png' })
    if (res?.data) {
      writeFileSync(path.join(outputDir, filename), Buffer.from(res.data, 'base64'))
      console.log(`[capture] 导出截图: ${filename}`)
    }
  }

  // 1. 默认主视口全景截图
  await takeScreenshot(defaultName)

  // 2. 按需点击各 Tab 截图
  for (const tab of tabs) {
    await send('Runtime.evaluate', {
      expression: `(() => {
        const btn = document.querySelector('[data-tab="${tab}"]');
        if (btn) btn.click();
      })()`,
    })
    await sleep(500)
    await takeScreenshot(`tab-${tab}.png`)
  }

  // 3. 暗色模式截图
  if (captureDark) {
    await send('Runtime.evaluate', {
      expression: `(() => {
        const btn = Array.from(document.querySelectorAll('button')).find(b => b.textContent.includes('暗色') || b.textContent.includes('亮色'));
        if (btn) btn.click();
      })()`,
    })
    await sleep(500)
    await takeScreenshot('dark-mode.png')
  }

  // 4. 移动端小屏截图
  if (captureMobile) {
    await send('Emulation.setDeviceMetricsOverride', {
      width: 390,
      height: 844,
      deviceScaleFactor: 2,
      mobile: true,
    })
    await sleep(500)
    await takeScreenshot('mobile.png')
  }

  ws.close()
  cleanup()
}

main().catch(err => {
  console.error('[capture] 失败:', err.message)
  process.exit(1)
})
