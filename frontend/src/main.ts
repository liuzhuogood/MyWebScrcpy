import { createPinia } from 'pinia'
import { createApp } from 'vue'
import { isTauri } from '@tauri-apps/api/core'
import './style.css'
import App from './App.vue'
import './composables/useTheme'
import router from './router'

if (typeof window !== 'undefined') {
  const isDesktop = Boolean(
    isTauri() ||
    (window as any).__MYWEBSCRCPY_DESKTOP_TOKEN__ ||
    (window as any).__TAURI_INTERNALS__ ||
    (window as any).__TAURI__
  )
  if (isDesktop) {
    document.documentElement.classList.add('is-desktop')
  }
}

const app = createApp(App)
app.use(createPinia())
app.use(router)
app.mount('#app')
