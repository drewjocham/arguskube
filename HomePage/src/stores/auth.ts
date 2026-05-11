import { defineStore } from 'pinia'
import type { AuthProvider, AuthEvent, AuthState } from '../types/auth'

export const useAuthStore = defineStore('auth', {
  state: (): AuthState => ({
    isAuthenticated: false,
    provider: null,
    events: [],
    loading: false,
  }),

  getters: {
    recentEvents: (state) => state.events.slice(-5).reverse(),
  },

  actions: {
    login(provider: AuthProvider) {
      this.loading = true
      const event: AuthEvent = {
        provider,
        action: 'login',
        timestamp: Date.now(),
      }
      this.events.push(event)
      this.isAuthenticated = true
      this.provider = provider
      this.loading = false
    },

    register(provider: AuthProvider) {
      this.loading = true
      const event: AuthEvent = {
        provider,
        action: 'register',
        timestamp: Date.now(),
      }
      this.events.push(event)
      this.isAuthenticated = true
      this.provider = provider
      this.loading = false
    },

    logout() {
      this.isAuthenticated = false
      this.provider = null
    },
  },
})
