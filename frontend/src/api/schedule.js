import { http } from '@/api/http'

const authed = { __requiresAuth: true }

const data = (resp) => resp.data.data

export const scheduleApi = {
  // Активные программы пользователя с их правилами расписания
  get: () => http.get('/schedule', authed).then(data),
  // Правка одного правила (frequency, interval_n, weekdays, once_date, is_enabled)
  updateRule: (id, body) => http.put(`/schedule/rules/${id}`, body, authed).then(data),
  // Деактивация программы пользователя
  deactivate: (userProgramId) => http.delete(`/schedule/programs/${userProgramId}`, authed).then(data),
  // План на неделю (пн–вс), содержащую дату; без даты — текущая
  week: (date) => http.get('/schedule/week', { ...authed, params: date ? { date } : {} }).then(data),
}
