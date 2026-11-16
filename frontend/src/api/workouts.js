import { http } from '@/api/http'

const authed = { __requiresAuth: true }

const data = (resp) => resp.data.data

export const workoutsApi = {
  // Старт тренировки из дня программы (снапшот плана): {user_program_id, program_day_id}
  start: (body) => http.post('/workouts/start', body, authed).then(data),
  // Активная (незавершённая) тренировка или null — точка восстановления после F5
  active: () => http.get('/workouts/active', authed).then(data),
  // Замена факта подхода (все поля факта + completed true/false/undefined)
  updateSet: (id, body) => http.patch(`/workouts/sets/${id}`, body, authed).then(data),
  // Добавить подход по ходу
  addSet: (exerciseId) => http.post(`/workouts/exercises/${exerciseId}/sets`, {}, authed).then(data),
  // Пропуск упражнения или замена (exercise_id)
  updateExercise: (id, body) => http.patch(`/workouts/exercises/${id}`, body, authed).then(data),
  // Добавить упражнение по ходу: {exercise_id, rest_after_sec?}
  addExercise: (body) => http.post('/workouts/exercises', body, authed).then(data),
  // Завершить тренировку
  complete: () => http.post('/workouts/complete', {}, authed).then(data),
  // Отменить тренировку
  cancel: () => http.post('/workouts/cancel', {}, authed).then(data),
}
