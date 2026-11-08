import { http } from '@/api/http'

const authed = { __requiresAuth: true }

const data = (resp) => resp.data.data

export const authApi = {
  register: (payload) => http.post('/auth/register', payload).then(data),
  login: (email, password) => http.post('/auth/login', { email, password }).then(data),
  refresh: (silent = false) =>
    http.post('/auth/refresh', undefined, silent ? { __silentAuth: true } : {}).then(data),
  logout: () => http.post('/auth/logout', undefined, authed),
  verifyEmail: (token) => http.post('/auth/verify-email', { token }),
  resendVerification: (email) => http.post('/auth/resend-verification', { email }),
  forgotPassword: (email) => http.post('/auth/forgot-password', { email }),
  resetPassword: (token, password) => http.post('/auth/reset-password', { token, password }),
  sessions: () => http.get('/auth/sessions', authed).then((resp) => data(resp).sessions),
  revokeSession: (id) => http.delete(`/auth/sessions/${id}`, authed),
  revokeAllSessions: () => http.post('/auth/sessions/revoke-all', undefined, authed),
  changePassword: (currentPassword, newPassword) =>
    http.post(
      '/auth/password',
      { current_password: currentPassword, new_password: newPassword },
      authed,
    ),
}
