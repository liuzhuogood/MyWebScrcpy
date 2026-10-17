import { createRouter, createWebHistory } from 'vue-router'
import routes from './routes'

const router = createRouter({
  history: createWebHistory(import.meta.env.BASE_URL),
  routes,
  scrollBehavior(to, from, savedPosition) {
    if (to.path === from.path)
      return false
    if (to.path.startsWith('/ai-integration')) {
      return savedPosition || { top: 0 }
    }
  },
})

router.afterEach((to) => {
  const title = to.meta.title as string | undefined
  if (title)
    document.title = title
})

export default router
