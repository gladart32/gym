import { defineStore } from 'pinia'
import { toast } from 'vue-sonner'

import { exercisesApi } from '@/api/exercises'
// Циклический импорт с auth.js и profile.js безопасен: сторы вызываются
// только внутри actions в рантайме.
import { useAuthStore } from '@/stores/auth'
import { useProfileStore } from '@/stores/profile'

export const PER_PAGE = 20

const SEARCH_DEBOUNCE_MS = 400

let qTimer = null

function parseIdsParam(value) {
  if (!value) return []
  return String(value)
    .split(',')
    .map(Number)
    .filter((n) => Number.isInteger(n) && n > 0)
}

// Кэш раздела «Упражнения»: фильтры, справочники и список переживают уход
// из раздела, чтобы не перезагружать данные и заново не грузить картинки.
export const useExercisesStore = defineStore('exercises', {
  state: () => ({
    q: '',
    qDebounced: '',
    sort: 'name',
    page: 1,
    bodyPartIds: [],
    equipmentIds: [],
    muscleIds: [],
    trackingTypes: [],
    myEquipment: false,
    favoritesOnly: false,

    dictionaries: null,
    hasUserEquipment: false,
    trackingTypeNames: {},

    items: [],
    total: 0,
    pages: 0,
    loading: false,
    metaLoading: true,

    initialized: false,
    lastParamsKey: '',
    requestSeq: 0,
  }),

  getters: {
    activeFilterCount: (state) =>
      state.bodyPartIds.length +
      state.equipmentIds.length +
      state.muscleIds.length +
      state.trackingTypes.length +
      (state.myEquipment ? 1 : 0) +
      (state.favoritesOnly ? 1 : 0),

    hasActiveFilters: (state) => state.activeFilterCount > 0 || state.q.trim() !== '',

    rangeFrom: (state) => (state.total === 0 ? 0 : (state.page - 1) * PER_PAGE + 1),
    rangeTo: (state) => Math.min(state.page * PER_PAGE, state.total),

    showSkeleton: (state) => state.metaLoading || (state.loading && state.items.length === 0),
  },

  actions: {
    setQ(value) {
      this.q = value
      clearTimeout(qTimer)
      qTimer = setTimeout(() => {
        this.qDebounced = this.q
        this.onFiltersChanged()
      }, SEARCH_DEBOUNCE_MS)
    },

    // URL — источник правды при полной загрузке страницы; при возврате
    // в раздел в рамках сессии стейт не перезаписывается пустым query.
    initFromUrl(query) {
      if (this.initialized) return
      this.initialized = true

      if (typeof query.q === 'string') {
        this.q = query.q
        this.qDebounced = query.q
      }
      if (query.sort === 'newest' || query.sort === 'name') this.sort = query.sort
      const p = Number(query.page)
      if (Number.isInteger(p) && p > 0) this.page = p
      this.bodyPartIds = parseIdsParam(query.bp)
      this.equipmentIds = parseIdsParam(query.eq)
      this.muscleIds = parseIdsParam(query.mu)
      if (typeof query.tt === 'string') {
        this.trackingTypes = query.tt.split(',').filter((v) => v)
      }
      this.myEquipment = query.my === '1'
      this.favoritesOnly = query.fav === '1'
    },

    listParams(instant) {
      const search = (instant ? this.q : this.qDebounced).trim()
      const params = { page: this.page, per_page: PER_PAGE, sort: this.sort }
      if (search) params.q = search
      if (this.bodyPartIds.length) params.body_part_ids = this.bodyPartIds.join(',')
      if (this.equipmentIds.length) params.equipment_ids = this.equipmentIds.join(',')
      if (this.muscleIds.length) params.muscle_ids = this.muscleIds.join(',')
      if (this.trackingTypes.length) params.tracking_types = this.trackingTypes.join(',')
      if (this.myEquipment) params.my_equipment = 1
      if (this.favoritesOnly) params.favorites_only = 1
      return params
    },

    async loadList({ instant = false } = {}) {
      // Без живой сессии не ходим в сеть: иначе clearSession() → $reset() →
      // watcher'ы ExercisesView → loadList → 401 → refresh → clearSession → цикл.
      if (!useAuthStore().isAuthenticated) return
      const params = this.listParams(instant)
      const key = JSON.stringify(params)
      // параметры не изменились — список уже в кэше, перезапрос не нужен
      if (key === this.lastParamsKey) return
      this.lastParamsKey = key

      const seq = ++this.requestSeq
      this.loading = true
      try {
        const data = await exercisesApi.list(params)
        if (seq !== this.requestSeq) return
        this.items = data.items ?? []
        this.total = data.total ?? 0
        this.pages = data.pages ?? 0
        if (this.page > this.pages && this.pages > 0) {
          this.page = this.pages
          return
        }
      } catch (error) {
        if (seq !== this.requestSeq) return
        this.lastParamsKey = ''
        toast.error(error.message)
      } finally {
        if (seq === this.requestSeq) this.loading = false
      }
    },

    async loadMeta() {
      if (!useAuthStore().isAuthenticated) return
      if (this.dictionaries) return
      this.metaLoading = true
      try {
        // инвентарь берём из кэша profile store (или один запрос с дедупом)
        const [filtersData, selectedEquipment] = await Promise.all([
          exercisesApi.filters(),
          useProfileStore().ensureEquipment(),
        ])
        this.dictionaries = filtersData
        this.trackingTypeNames = Object.fromEntries(
          (filtersData.tracking_types ?? []).map((tt) => [tt.value, tt.name]),
        )
        this.hasUserEquipment = selectedEquipment.length > 0
      } catch (error) {
        toast.error(error.message)
      } finally {
        this.metaLoading = false
      }
    },

    onFiltersChanged() {
      if (this.page === 1) {
        this.loadList()
      } else {
        this.page = 1
      }
    },

    resetFilters() {
      clearTimeout(qTimer)
      this.q = ''
      this.qDebounced = ''
      this.bodyPartIds = []
      this.equipmentIds = []
      this.muscleIds = []
      this.trackingTypes = []
      this.myEquipment = false
      this.favoritesOnly = false
      this.sort = 'name'
      this.page = 1
    },

    async toggleFavorite(exercise) {
      const target = !exercise.is_favorite
      exercise.is_favorite = target
      try {
        if (target) {
          await exercisesApi.addFavorite(exercise.id)
        } else {
          await exercisesApi.removeFavorite(exercise.id)
          if (this.favoritesOnly) {
            this.lastParamsKey = ''
            await this.loadList()
          }
        }
      } catch (error) {
        exercise.is_favorite = !target
        toast.error(error.message)
      }
    },

    // Инвалидация кэшированных данных с сохранением фильтров:
    // при следующем входе в раздел мета и список загрузятся заново.
    invalidateCache() {
      clearTimeout(qTimer)
      this.dictionaries = null
      this.hasUserEquipment = false
      this.trackingTypeNames = {}
      this.items = []
      this.total = 0
      this.pages = 0
      this.lastParamsKey = ''
      this.metaLoading = true
    },
  },
})
