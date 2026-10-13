import { createPinia } from 'pinia'
import { createApp } from 'vue'
import './style.css'
import App from './App.vue'
import './composables/useTheme'
import router from './router'

const app = createApp(App)
app.use(createPinia())
app.use(router)
app.mount('#app')
