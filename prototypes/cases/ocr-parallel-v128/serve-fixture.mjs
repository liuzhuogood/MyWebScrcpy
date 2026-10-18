import { createRequire } from 'node:module'
import path from 'node:path'
import { fileURLToPath } from 'node:url'

const caseDir = path.dirname(fileURLToPath(import.meta.url))
const frontendDir = path.resolve(caseDir, '../../../frontend')
const require = createRequire(path.join(frontendDir, 'package.json'))
const { createServer, loadConfigFromFile, mergeConfig } = await import(require.resolve('vite'))
const loaded = await loadConfigFromFile({ command: 'serve', mode: 'development' }, path.join(frontendDir, 'vite.prototype.config.ts'))
if (!loaded) throw new Error('原型 Vite 配置不存在')

const server = await createServer(mergeConfig(loaded.config, {
  configFile: false,
  root: path.join(caseDir, 'verification'),
  cacheDir: path.join(frontendDir, 'node_modules/.vite-ocr-fixture'),
  server: { host: '127.0.0.1', port: 5176, strictPort: true },
  plugins: [{
    name: 'ocr-parallel-fixture',
    configureServer(vite) {
      vite.middlewares.use(async (req, res, next) => {
        if (!req.url?.startsWith('/api/')) return next()
        res.setHeader('Content-Type', 'application/json')
        // 不转发到真实后端；未列出的接口明确失败。
        if (req.url !== '/api/vision/ocr' || req.method !== 'POST') {
          res.statusCode = 404
          res.end(JSON.stringify({ error: 'fixture 接口不存在' }))
          return
        }
        try {
          const chunks = []
          let size = 0
          for await (const chunk of req) {
            size += chunk.length
            if (size > 2 * 1024 * 1024) throw new Error('fixture 上传超过限制')
            chunks.push(chunk)
          }
          const form = await new Response(Buffer.concat(chunks), { headers: { 'Content-Type': req.headers['content-type'] } }).formData()
          const engine = form.get('engine')
          if (!['auto', 'parallel', 'indraocr', 'macocr', 'paddleocr'].includes(engine)) throw new Error('无效的引擎模式')
          if (form.get('serial') !== 'ocr-parallel-fixture' || form.get('mark') !== 'false') throw new Error('非隔离请求')
          console.info('[OCR fixture] engine=%s mark=false', engine)
          res.end(JSON.stringify({
            status: 'matched', matched: true, text: '设置',
            boxes: [{ text: '设置', confidence: 0.99, x: 0.1, y: 0.07, w: 0.2, h: 0.04 }],
            engine: engine === 'auto' ? 'indraocr' : engine, elapsed_ms: 48,
            samples: 1, engines_used: engine === 'parallel' ? ['indraocr', 'macocr', 'paddleocr'] : [engine === 'auto' ? 'indraocr' : engine],
          }))
        }
        catch (error) {
          console.error('[OCR fixture] 请求失败', error)
          res.statusCode = 400
          res.end(JSON.stringify({ error: error.message }))
        }
      })
    },
  }],
}))
await server.listen()
server.printUrls()
for (const signal of ['SIGINT', 'SIGTERM']) {
  process.once(signal, async () => { await server.close(); process.exit(0) })
}
