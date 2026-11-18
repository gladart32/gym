import { http } from '@/api/http'

const authed = { __requiresAuth: true }

const data = (resp) => resp.data.data

export const statsApi = {
  // Сводка для главной: счётчики, серия, активность за 12 недель, последняя тренировка
  dashboard: () => http.get('/stats/dashboard', authed).then(data),
}
