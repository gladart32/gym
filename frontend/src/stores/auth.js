import { defineStore } from 'pinia'

import { authApi } from '@/api/auth'
import { useProfileStore } from '@/stores/profile'
// Циклический импорт с exercises.js, programs.js и profile.js безопасен:
// сторы вызываются только внутри actions в рантайме.
import { useExercisesStore } from '@/stores/exercises'
import useProgramsStore from '@/stores/programs'

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
      // данные профиля грузим сразу и ждём: к моменту навигации стор готов
      await useProfileStore().bootstrap()
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
      // каждый рефреш токена (F5 через boot и фоновое истечение access)
      // обновляет данные пользователя в сторе; не ждём — не тормозим запросы
      useProfileStore().onAuthRefreshed()
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
      // кэши exercises, programs и profile — пользователь-специфичные
      useExercisesStore().$reset()
      useProgramsStore().$reset()
      useProfileStore().$reset()
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
