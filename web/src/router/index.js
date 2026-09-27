import { createRouter, createWebHistory } from 'vue-router'

const routes = [
  { path: '/login', name: 'login', component: () => import('../views/Login.vue') },
  { path: '/', name: 'lobby', component: () => import('../views/Lobby.vue') },
  { path: '/room/:code', name: 'room', component: () => import('../views/Room.vue') },
  { path: '/replay/:id', name: 'replay', component: () => import('../views/Replay.vue') },
  { path: '/puzzle', name: 'puzzle', component: () => import('../views/Puzzle.vue') },
  { path: '/admin', name: 'admin', component: () => import('../views/Admin.vue'), meta: { requiresAdmin: true } }
]

const router = createRouter({
  history: createWebHistory(),
  routes
})

router.beforeEach((to, from, next) => {
  if (to.meta.requiresAdmin) {
    const role = localStorage.getItem('xq_role')
    if (role !== 'admin') {
      next('/')
      return
    }
  }
  next()
})

export default router
