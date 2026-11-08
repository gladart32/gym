import axios from 'axios'

import { i18n } from '@/i18n'
import router from '@/router'
import { useAuthStore } from '@/stores/auth'

export const http = axios.create({
  baseURL: import.meta.env.VITE_API_URL || '/api/v1',
  timeout: 15000,
})

const REFRESH_URL = '/auth/refresh'

http.interceptors.request.use(async (config) => {
  config.headers['Accept-Language'] = i18n.global.locale.value
  if (config.__requiresAuth) {
    const auth = useAuthStore()
    const token = await auth.ensureFreshToken()
    config.headers.Authorization = `Bearer ${token}`
  }
  return config
})

http.interceptors.response.use(
  (response) => response,
  async (error) => {
    const { config, response } = error
    const status = response?.status

    // refresh сам вернул 401: сессия мертва — чистим стор; на логин уводим
    // только если это не тихий boot-refresh (редирект решает guard роутера)
    if (status === 401 && config?.url === REFRESH_URL) {
      useAuthStore().clearSession()
      if (!config.__silentAuth) {
        redirectToLogin()
      }
    } else if (status === 401 && config?.headers?.Authorization && !config.__retried) {
      // 401 на запросе с Bearer: один повтор после single-flight refresh
      const auth = useAuthStore()
      auth.forceRefresh()
      try {
        const token = await auth.ensureFreshToken()
        config.__retried = true
        config.headers.Authorization = `Bearer ${token}`
        return http(config)
      } catch (refreshError) {
        redirectToLogin()
        const message = refreshError.response?.data?.message ?? refreshError.message
        return Promise.reject(new Error(message))
      }
    }

    // сетевая ошибка или сбой сервера: без сырых axios-строк
    const isServerError = !response || response.status >= 500
    const message = isServerError
      ? i18n.global.t('errors.serverUnavailable')
      : (response?.data?.message ?? error.message)
    return Promise.reject(new Error(message))
  },
)

function redirectToLogin() {
  const current = router.currentRoute.value
  if (current.name !== 'login') {
    router.push({ name: 'login', query: { redirect: current.fullPath } })
  }
}
