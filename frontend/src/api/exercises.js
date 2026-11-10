import { http } from '@/api/http'

const authed = { __requiresAuth: true }

const data = (resp) => resp.data.data

export const exercisesApi = {
  list: (params) => http.get('/exercises', { ...authed, params }).then(data),
  get: (id) => http.get(`/exercises/${id}`, authed).then(data),
  filters: () => http.get('/exercises/filters', authed).then(data),
}
