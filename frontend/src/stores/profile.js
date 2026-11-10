import { defineStore } from 'pinia'
import { toast } from 'vue-sonner'

import { profileApi } from '@/api/profile'
// Циклический импорт с auth.js и exercises.js безопасен: сторы вызываются
// только внутри actions в рантайме.
import { useAuthStore } from '@/stores/auth'
import { useExercisesStore } from '@/stores/exercises'

// Ин-флайт промисы на сессию: дедупликация параллельных вызовов одного
// ресурса (view + фоновый refresh после обновления токена — один запрос).
let inFlight = {}

// Номер «сессии» стора: $reset() при logout инкрементит его, и ответы
// запросов, прилетевшие после сброса, не трогают состояние чужими данными.
let sessionSeq = 0

export const useProfileStore = defineStore('profile', {
  state: () => ({
    // null = ещё не загружено; {} = профиль заведён, но не заполнен
    profile: null,
    currentMeasurement: null,
    // полный список, бэк отдаёт ORDER BY measured_on DESC
    measurements: null,
    equipmentItems: null,
    selectedEquipment: [],

    profileLoading: false,
    measurementsLoading: false,
    equipmentLoading: false,

    initialized: false,
  }),

  actions: {
    // Разовая загрузка при логине/рефреше токена. Повторные вызовы — no-op:
    // дальше кэш живёт до $reset() или force-перезапроса.
    async bootstrap() {
      if (this.initialized || !useAuthStore().isAuthenticated) return
      this.initialized = true
      await Promise.all([this.loadProfile(), this.loadMeasurements(), this.loadEquipment()])
    },

    // Хук из auth.refresh(): первый рефреш токена — bootstrap, дальше —
    // тихое фоновое обновление данных пользователя (F5 и истёкший access).
    onAuthRefreshed() {
      if (!this.initialized) {
        return this.bootstrap()
      }
      this.loadProfile({ force: true, silent: true })
      this.loadMeasurements({ force: true, silent: true })
      this.loadEquipment({ force: true, silent: true })
    },

    // Кэширующие загрузки. force — перезапросить поверх кэша; silent —
    // фоновый режим: без тостов и без включения loading (список не мигает).
    async loadProfile({ force = false, silent = false } = {}) {
      if (!useAuthStore().isAuthenticated) return
      if (!force && this.profile !== null) return
      if (inFlight.profile) return inFlight.profile

      if (this.profile === null) this.profileLoading = true
      const session = sessionSeq
      inFlight.profile = profileApi
        .get()
        .then((data) => {
          if (session !== sessionSeq) return
          // профиль может быть не заполнен (null от бэка) — храним {},
          // чтобы null оставался маркером «не загружено»
          this.profile = data?.profile ?? {}
          this.currentMeasurement = data?.current_measurement ?? null
        })
        .catch((error) => {
          if (session === sessionSeq && !silent) toast.error(error.message)
        })
        .finally(() => {
          inFlight.profile = null
          if (session === sessionSeq) this.profileLoading = false
        })
      return inFlight.profile
    },

    async loadMeasurements({ force = false, silent = false } = {}) {
      if (!useAuthStore().isAuthenticated) return
      if (!force && this.measurements !== null) return
      if (inFlight.measurements) return inFlight.measurements

      if (this.measurements === null) this.measurementsLoading = true
      const session = sessionSeq
      inFlight.measurements = profileApi
        .measurements()
        .then((data) => {
          if (session !== sessionSeq) return
          this.measurements = Array.isArray(data) ? data : []
        })
        .catch((error) => {
          if (session === sessionSeq && !silent) toast.error(error.message)
        })
        .finally(() => {
          inFlight.measurements = null
          if (session === sessionSeq) this.measurementsLoading = false
        })
      return inFlight.measurements
    },

    async loadEquipment({ force = false, silent = false } = {}) {
      if (!useAuthStore().isAuthenticated) return
      if (!force && this.equipmentItems !== null) return
      if (inFlight.equipment) return inFlight.equipment

      if (this.equipmentItems === null) this.equipmentLoading = true
      const session = sessionSeq
      inFlight.equipment = profileApi
        .equipment()
        .then((data) => {
          if (session !== sessionSeq) return
          this.equipmentItems = data?.items ?? []
          this.selectedEquipment = data?.selected_ids ?? []
        })
        .catch((error) => {
          if (session === sessionSeq && !silent) toast.error(error.message)
        })
        .finally(() => {
          inFlight.equipment = null
          if (session === sessionSeq) this.equipmentLoading = false
        })
      return inFlight.equipment
    },

    // Для exercises.js: кэш инвентаря или один запрос (с дедупликацией).
    async ensureEquipment() {
      await this.loadEquipment()
      return this.selectedEquipment
    },

    async saveProfile(payload) {
      const session = sessionSeq
      try {
        const data = await profileApi.update(payload)
        if (session !== sessionSeq) return false
        this.profile = data?.profile ?? {}
        return true
      } catch (error) {
        if (session === sessionSeq) toast.error(error.message)
        return false
      }
    },

    async addMeasurement(payload) {
      const session = sessionSeq
      try {
        const record = await profileApi.upsertMeasurement(payload)
        if (session !== sessionSeq) return false
        this.applyMeasurement(record)
        return true
      } catch (error) {
        if (session === sessionSeq) toast.error(error.message)
        return false
      }
    },

    // Upsert перезаписывает запись за дату целиком: заменяем/вставляем её
    // в список с сохранением сортировки DESC и пересчитываем текущий замер
    // без похода в сеть — current_measurement это просто самый свежий.
    applyMeasurement(record) {
      if (!record) return
      const list = [...(this.measurements ?? [])]
      const date = String(record.measured_on).slice(0, 10)
      const idx = list.findIndex((m) => String(m.measured_on).slice(0, 10) === date)
      if (idx >= 0) list.splice(idx, 1)
      list.push(record)
      // RFC3339 с тайзонами — сравниваем через Date, не по строке
      list.sort((a, b) => Date.parse(b.measured_on) - Date.parse(a.measured_on))
      this.measurements = list
      this.currentMeasurement = list[0] ?? null
    },

    async deleteMeasurement(id) {
      const session = sessionSeq
      try {
        await profileApi.deleteMeasurement(id)
        if (session !== sessionSeq) return false
        this.measurements = (this.measurements ?? []).filter((m) => m.id !== id)
        this.currentMeasurement = this.measurements[0] ?? null
        return true
      } catch (error) {
        if (session === sessionSeq) toast.error(error.message)
        return false
      }
    },

    async saveEquipment(ids) {
      const session = sessionSeq
      try {
        const data = await profileApi.replaceEquipment(ids)
        if (session !== sessionSeq) return false
        this.selectedEquipment = data?.selected_ids ?? []
        // инвентарь влияет на фильтры и выдачу упражнений — сбрасываем кэш раздела
        useExercisesStore().invalidateCache()
        return true
      } catch (error) {
        if (session === sessionSeq) toast.error(error.message)
        return false
      }
    },

    // Сброс при logout / смерти сессии: кэш юзер-специфичный.
    $reset() {
      sessionSeq += 1
      inFlight = {}
      this.profile = null
      this.currentMeasurement = null
      this.measurements = null
      this.equipmentItems = null
      this.selectedEquipment = []
      this.profileLoading = false
      this.measurementsLoading = false
      this.equipmentLoading = false
      this.initialized = false
    },
  },
})
