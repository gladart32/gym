import { http } from '@/api/http'

const authed = { __requiresAuth: true }

const data = (resp) => resp.data.data

export const programsApi = {
  list: (params) => http.get('/programs', { ...authed, params }).then(data),
  get: (id) => http.get(`/programs/${id}`, authed).then(data),
  create: (body) => http.post('/programs', body, authed).then(data),
  update: (id, body) => http.put(`/programs/${id}`, body, authed).then(data),
  remove: (id) => http.delete(`/programs/${id}`, authed).then(data),
  copy: (id) => http.post(`/programs/${id}/copy`, null, authed).then(data),
}
