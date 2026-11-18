import { defineStore } from 'pinia'
import { toast } from 'vue-sonner'

import { statsApi } from '@/api/stats'
import { useAuthStore } from '@/stores/auth'

// Стор сводной статистики главной страницы (GET /stats/dashboard).
// Без кэша: каждый заход на главную тянет свежие данные,
// поэтому после завершения тренировки числа сразу актуальны.
const useStatsStore = defineStore('stats', {
  state: () => ({
    dashboard: null,
    loading: false,
  }),

  actions: {
    async load() {
      if (!useAuthStore().isAuthenticated) return
      this.loading = true
      try {
        this.dashboard = await statsApi.dashboard()
      } catch (error) {
        toast.error(error.message)
      } finally {
        this.loading = false
      }
    },

    reset() {
      this.$reset()
    },
  },
})

export default useStatsStore
