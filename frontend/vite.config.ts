import path from 'node:path'
import tailwindcss from '@tailwindcss/vite'
import vue from '@vitejs/plugin-vue'
import { defineConfig } from 'vite'

const backendURL = process.env.MYWEBSCRCPY_BACKEND_URL ?? 'http://localhost:8080'

// https://vite.dev/config/
export default defineConfig({
  base: '/',
  build: {
    outDir: '../web/dist',
    emptyOutDir: true,
  },
  plugins: [vue(), tailwindcss()],
  resolve: {
    alias: {
      '@': path.resolve(import.meta.dirname, './src'),
    },
  },
  server: {
    port: 5173,
    proxy: {
      '/api': {
        target: backendURL,
        changeOrigin: true,
        ws: true,
      },
      '/api-docs.html': { target: backendURL },
      '/api-docs-automation.html': { target: backendURL },
      '/automation-api-docs.html': { target: backendURL },
      // 投屏 / 终端 / Vision 均走 WebSocket，必须显式代理 ws
      '/ws': {
        target: backendURL,
        changeOrigin: true,
        ws: true,
      },
    },
  },
})
