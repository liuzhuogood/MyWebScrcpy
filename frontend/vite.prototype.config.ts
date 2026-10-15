import { readFileSync } from 'node:fs'
import path from 'node:path'
import tailwindcss from '@tailwindcss/vite'
import vue from '@vitejs/plugin-vue'
import { defineConfig } from 'vite'

const frontendDir = import.meta.dirname
const prototypeDir = path.resolve(frontendDir, '../prototypes')
const manifest = JSON.parse(readFileSync(path.join(frontendDir, 'package.json'), 'utf8')) as {
  dependencies: Record<string, string>
}

export default defineConfig({
  root: prototypeDir,
  publicDir: false,
  envDir: prototypeDir,
  cacheDir: path.join(frontendDir, 'node_modules/.vite-prototypes'),
  plugins: [vue(), tailwindcss()],
  resolve: {
    // 原型在 frontend 外，依赖仍从现有安装读取，不另装一套 Vue。
    alias: [
      { find: '@', replacement: path.join(frontendDir, 'src') },
      ...Object.keys(manifest.dependencies).map(name => ({
        find: name,
        replacement: path.join(frontendDir, 'node_modules', name),
      })),
    ],
  },
  server: {
    host: '127.0.0.1',
    port: 5174,
    strictPort: true,
    allowedHosts: [
      'yen-motorcycles-investigations-league.trycloudflare.com',
      'sellers-farm-modified-cubic.trycloudflare.com',
      'imposed-stop-entrance-cuisine.trycloudflare.com',
    ],
    fs: { allow: [prototypeDir, frontendDir] },
  },
  preview: {
    host: '127.0.0.1',
    port: 5174,
    strictPort: true,
    allowedHosts: [
      'yen-motorcycles-investigations-league.trycloudflare.com',
      'examining-remedy-layer-auditor.trycloudflare.com',
      'imposed-stop-entrance-cuisine.trycloudflare.com',
    ],
  },
  build: { outDir: path.join(prototypeDir, 'dist'), emptyOutDir: true },
})
