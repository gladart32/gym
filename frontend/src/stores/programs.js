import { defineStore } from 'pinia'
import { toast } from 'vue-sonner'

import { programsApi } from '@/api/programs'
// Циклический импорт с auth.js безопасен: сторы вызываются
// только внутри actions в рантайме.
import { useAuthStore } from '@/stores/auth'

// Серверный максимум: секции списка отдают как можно больше за раз,
// пагинация внутри секции нужна только при 50+ тренировках.
export const PER_PAGE = 50

const SEARCH_DEBOUNCE_MS = 600

let qTimer = null

// Кэш раздела «Тренировки» (бэкенд-сущность programs): списки секций
// и детали карточек переживают уход из раздела, чтобы не перезагружать данные.
const useProgramsStore = defineStore('programs', {
  state: () => ({
    q: '',
    qDebounced: '',
    sort: 'newest',

    mine: { items: [], total: 0, page: 1, pages: 0, loading: false },
    ready: { items: [], total: 0, page: 1, pages: 0, loading: false },

    initialized: false,
    lastMineKey: '',
    lastReadyKey: '',
    // отдельный счётчик на секцию: два параллельных loadList (mine+ready)
    // не должны отбрасывать ответы друг друга
    seq: { mine: 0, ready: 0 },
    appliedRevision: 0,

    // детали тренировок по id; мутации (create/update/copy) обновляют кэш
    details: {},
  }),

  getters: {
    hasActiveFilters: (state) => state.qDebounced.trim() !== '' || state.sort !== 'newest',
    mineShowSkeleton: (state) => state.mine.loading && state.mine.items.length === 0,
    readyShowSkeleton: (state) => state.ready.loading && state.ready.items.length === 0,
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
    },

    listParams() {
      const params = { per_page: PER_PAGE, sort: this.sort }
      const search = this.qDebounced.trim()
      if (search) params.q = search
      return params
    },

    // Изменение поиска/сортировки: страницы секций сбрасываются,
    // инкремент ревизии синхронизирует URL с применёнными фильтрами.
    onFiltersChanged() {
      this.appliedRevision += 1
      this.mine.page = 1
      this.ready.page = 1
      this.lastMineKey = ''
      this.lastReadyKey = ''
      return Promise.all([this.loadList('mine'), this.loadList('ready')])
    },

    async loadList(scope) {
      // Без живой сессии не ходим в сеть: иначе clearSession() → $reset() →
      // 401 → refresh → clearSession → цикл.
      if (!useAuthStore().isAuthenticated) return
      const section = this[scope]
      const params = { ...this.listParams(), scope, page: section.page }
      const key = JSON.stringify(params)
      const cacheKey = scope === 'mine' ? 'lastMineKey' : 'lastReadyKey'
      // параметры не изменились — список уже в кэше, перезапрос не нужен
      if (key === this[cacheKey]) return
      this[cacheKey] = key

      const seq = ++this.seq[scope]
      section.loading = true
      try {
        const data = await programsApi.list(params)
        if (seq !== this.seq[scope]) return
        section.items = data.items ?? []
        section.total = data.total ?? 0
        section.pages = data.pages ?? 0
      } catch (error) {
        if (seq !== this.seq[scope]) return
        this[cacheKey] = ''
        toast.error(error.message)
      } finally {
        if (seq === this.seq[scope]) section.loading = false
      }
    },

    async changePage(scope, value) {
      this[scope].page = value
      window.scrollTo({ top: 0, behavior: 'smooth' })
      return this.loadList(scope)
    },

    resetFilters() {
      clearTimeout(qTimer)
      this.q = ''
      this.qDebounced = ''
      this.sort = 'newest'
      return this.onFiltersChanged()
    },

    async loadDetail(id) {
      if (this.details[id]) return this.details[id]
      try {
        const detail = await programsApi.get(id)
        this.details[detail.id] = detail
        return detail
      } catch (error) {
        toast.error(error.message)
        return null
      }
    },

    // create/update/copy возвращают Detail — кладём в кэш, чтобы
    // детальная страница и редактор открылись без повторного запроса.
    cacheDetail(detail) {
      if (detail?.id) this.details[detail.id] = detail
    },

    async saveProgram(id, body) {
      try {
        const detail = id ? await programsApi.update(id, body) : await programsApi.create(body)
        this.cacheDetail(detail)
        // структура/шапка изменились — списки перезагрузим при следующем входе
        this.lastMineKey = ''
        this.lastReadyKey = ''
        return detail
      } catch (error) {
        toast.error(error.message)
        return null
      }
    },

    async copyProgram(id) {
      try {
        const detail = await programsApi.copy(id)
        this.cacheDetail(detail)
        this.lastMineKey = ''
        return detail
      } catch (error) {
        toast.error(error.message)
        return null
      }
    },

    async removeProgram(id) {
      try {
        await programsApi.remove(id)
        delete this.details[id]
        this.lastMineKey = ''
        this.lastReadyKey = ''
        return true
      } catch (error) {
        toast.error(error.message)
        return false
      }
    },
  },
})
export default useProgramsStore
