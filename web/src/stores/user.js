import { defineStore } from 'pinia'

export const useUserStore = defineStore('user', {
  state: () => ({
    token: localStorage.getItem('xq_token') || '',
    username: localStorage.getItem('xq_name') || '',
    role: localStorage.getItem('xq_role') || 'user',
    stats: null
  }),
  getters: {
    isLogin: (s) => !!s.token,
    isAdmin: (s) => s.role === 'admin'
  },
  actions: {
    setAuth(data) {
      this.token = data.token
      this.username = data.username
      this.role = data.role || 'user'
      this.stats = data.stats || null
      localStorage.setItem('xq_token', data.token)
      localStorage.setItem('xq_name', data.username)
      localStorage.setItem('xq_role', this.role)
    },
    async authenticate(mode, username, password, email = '', code = '') {
      const res = await fetch(`/api/${mode}`, {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({ username, password, email, code })
      })
      const data = await res.json()
      if (!res.ok) throw new Error(data.error || '请求失败')
      this.setAuth(data)
    },
    async fetchMe() {
      if (!this.token) return
      const res = await fetch('/api/me', {
        headers: { Authorization: `Bearer ${this.token}` }
      })
      if (!res.ok) {
        this.logout()
        return
      }
      const data = await res.json()
      this.username = data.username
      this.role = data.role || 'user'
      this.stats = data.stats
      localStorage.setItem('xq_role', this.role)
    },
    async recentGames() {
      const res = await fetch('/api/games/recent', {
        headers: { Authorization: `Bearer ${this.token}` }
      })
      if (!res.ok) return []
      const data = await res.json()
      return data.games || []
    },
    logout() {
      this.token = ''
      this.username = ''
      this.role = 'user'
      this.stats = null
      localStorage.removeItem('xq_token')
      localStorage.removeItem('xq_name')
      localStorage.removeItem('xq_role')
    }
  }
})
