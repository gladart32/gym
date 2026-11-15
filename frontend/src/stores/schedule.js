import { defineStore } from 'pinia'
import { toast } from 'vue-sonner'

import { programsApi } from '@/api/programs'
import { scheduleApi } from '@/api/schedule'
// Циклический импорт с auth.js безопасен: сторы вызываются
// только внутри actions в рантайме.
import { useAuthStore } from '@/stores/auth'

// Стор раздела «Календарь»: активные программы пользователя
// и план на выбранную неделю (пн–вс).
const useScheduleStore = defineStore('schedule', {
  state: () => ({
    // активные программы с правилами (GET /schedule)
    programs: [],
    programsLoading: false,
    programsLoaded: false,

    // план на неделю (GET /schedule/week)
    // anchorDate — якорь недели в формате YYYY-MM-DD ('' = текущая)
    week: null,
    anchorDate: '',
    weekLoading: false,
  }),

  getters: {
    hasPrograms: (state) => state.programs.length > 0,
  },

  actions: {
    async load() {
      if (!useAuthStore().isAuthenticated) return
      this.programsLoading = true
      try {
        this.programs = await scheduleApi.get()
        this.programsLoaded = true
      } catch (error) {
        toast.error(error.message)
      } finally {
        this.programsLoading = false
      }
    },

    async loadWeek(date) {
      if (!useAuthStore().isAuthenticated) return
      this.weekLoading = true
      this.anchorDate = date ?? ''
      try {
        this.week = await scheduleApi.week(this.anchorDate)
      } catch (error) {
        toast.error(error.message)
      } finally {
        this.weekLoading = false
      }
    },

    async refresh() {
      return Promise.all([this.load(), this.loadWeek(this.anchorDate)])
    },

    // Выбор программы (старт с указанной даты) — после перезагружаем расписание
    async selectProgram(programId, startDate) {
      const body = startDate ? { start_date: startDate } : {}
      try {
        await programsApi.select(programId, body)
        await this.refresh()
        return true
      } catch (error) {
        toast.error(error.message)
        return false
      }
    },

    // Правка правила: ответ сервера заменяет правило в сторе
    async updateRule(rule, body) {
      try {
        const updated = await scheduleApi.updateRule(rule.id, body)
        this.replaceRule(updated)
        return true
      } catch (error) {
        toast.error(error.message)
        return false
      }
    },

    // Удаление правила: исчезает из карточки программы и из плана недели
    async removeRule(ruleId) {
      try {
        await scheduleApi.removeRule(ruleId)
        for (const program of this.programs) {
          const index = program.rules.findIndex((r) => r.id === ruleId)
          if (index !== -1) {
            program.rules.splice(index, 1)
            break
          }
        }
        await this.loadWeek(this.anchorDate)
        return true
      } catch (error) {
        toast.error(error.message)
        return false
      }
    },

    // Остановка активной программы по id самой программы
    async stopProgram(programId) {
      const active = this.programs.find((p) => p.program_id === programId)
      if (!active) return false
      return this.deactivate(active.id)
    },

    async deactivate(userProgramId) {
      try {
        await scheduleApi.deactivate(userProgramId)
        this.programs = this.programs.filter((p) => p.id !== userProgramId)
        await this.loadWeek(this.anchorDate)
        return true
      } catch (error) {
        toast.error(error.message)
        return false
      }
    },

    replaceRule(updated) {
      for (const program of this.programs) {
        const index = program.rules.findIndex((r) => r.id === updated.id)
        if (index !== -1) {
          program.rules.splice(index, 1, updated)
          return
        }
      }
    },

    reset() {
      this.$reset()
    },
  },
})

export default useScheduleStore
