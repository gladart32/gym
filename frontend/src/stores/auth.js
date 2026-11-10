import { defineStore } from 'pinia'

import { authApi } from '@/api/auth'
import { useExercisesStore } from '@/stores/exercises'

const REFRESH_MARGIN_MS = 10_000

export const useAuthStore = defineStore('auth', {
  state: () => ({
    accessToken: null,
    tokenExpiresAt: 0,
    user: null,
    booted: false,
    refreshInFlight: null,
  }),

  getters: {
    isAuthenticated: (state) => state.accessToken !== null,
  },

  actions: {
    applyTokenData(data) {
      this.accessToken = data.access_token
      this.tokenExpiresAt = Date.now() + data.expires_in * 1000
      this.user = data.user
    },

    async login(email, password) {
      this.applyTokenData(await authApi.login(email, password))
    },

    // single-flight: параллельные запросы ждут один и тот же refresh
    async ensureFreshToken({ silent = false } = {}) {
      if (this.accessToken && Date.now() < this.tokenExpiresAt - REFRESH_MARGIN_MS) {
        return this.accessToken
      }
      if (!this.refreshInFlight) {
        this.refreshInFlight = this.refresh(silent)
          .catch((error) => {
            this.clearSession()
            throw error
          })
          .finally(() => {
            this.refreshInFlight = null
          })
      }
      return this.refreshInFlight
    },

    async refresh(silent = false) {
      this.applyTokenData(await authApi.refresh(silent))
      return this.accessToken
    },

    // перед принудительным refresh после 401 (access мог ещё не истечь, но сессия отозвана)
    forceRefresh() {
      this.accessToken = null
      this.tokenExpiresAt = 0
    },

    clearSession() {
      this.accessToken = null
      this.tokenExpiresAt = 0
      this.user = null
      // кэш упражнений (избранное, мой инвентарь) — пользователь-специфичный
      useExercisesStore().$reset()
    },

    async logout() {
      try {
        await authApi.logout()
      } catch {
        // сессия могла уже умереть — локально выходим в любом случае
      }
      this.clearSession()
    },

    async boot() {
      if (this.booted) return
      this.booted = true
      try {
        await this.ensureFreshToken({ silent: true })
      } catch {
        // нет живой сессии — нормальная ситуация; редирект решает guard
      }
    },
  },
})
