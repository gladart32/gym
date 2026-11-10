import { http } from '@/api/http'

const authed = { __requiresAuth: true }

const data = (resp) => resp.data.data

export const profileApi = {
  get: () => http.get('/profile', authed).then(data),
  update: (payload) => http.put('/profile', payload, authed).then(data),
  measurements: () => http.get('/profile/measurements', authed).then(data),
  upsertMeasurement: (payload) => http.post('/profile/measurements', payload, authed).then(data),
  deleteMeasurement: (id) => http.delete(`/profile/measurements/${id}`, authed),
  equipment: () => http.get('/profile/equipment', authed).then(data),
  replaceEquipment: (ids) =>
    http.put('/profile/equipment', { equipment_ids: ids }, authed).then(data),
}
