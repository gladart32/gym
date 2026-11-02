import axios from 'axios'

import { i18n } from '@/i18n'

export const http = axios.create({
  baseURL: import.meta.env.VITE_API_URL || '/api/v1',
  timeout: 15000,
})

http.interceptors.request.use((config) => {
  config.headers['Accept-Language'] = i18n.global.locale.value
  return config
})

http.interceptors.response.use(
  (response) => response,
  (error) => {
    const message = error.response?.data?.message ?? error.message
    return Promise.reject(new Error(message))
  },
)
