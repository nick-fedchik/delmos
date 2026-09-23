import { defineStore } from 'pinia'

import { api, clearCsrfToken, setCsrfToken } from '@/api/client'
import type { LoginResponse, UserView } from '@/api/types'

interface SessionState {
  user: UserView | null
  checked: boolean
}

export const useSessionStore = defineStore('session', {
  state: (): SessionState => ({ user: null, checked: false }),
  getters: {
    isAuthenticated: (state) => state.user !== null,
  },
  actions: {
    hasPermission(this: SessionState, key: string): boolean {
      return this.user?.permissions.includes(key) ?? false
    },

    async login(login: string, password: string): Promise<void> {
      const response = await api.post<LoginResponse>('/auth/login', { login, password })
      setCsrfToken(response.csrf_token)
      this.user = response.user
      this.checked = true
    },

    async logout(): Promise<void> {
      try {
        await api.post('/auth/logout')
      } finally {
        clearCsrfToken()
        this.user = null
      }
    },

    // restore перевіряє чинність сесії при перезавантаженні сторінки (cookie
    // зберігається браузером, але CSRF-токен — лише в пам'яті процесу).
    async restore(): Promise<void> {
      try {
        this.user = await api.get<UserView>('/auth/session')
      } catch {
        this.user = null
      } finally {
        this.checked = true
      }
    },
  },
})
