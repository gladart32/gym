import { defineStore } from 'pinia'
import { toast } from 'vue-sonner'

import { workoutsApi } from '@/api/workouts'

// Стор раздела «Проведение тренировки»: машина состояний сессии
// (подход → отдых → следующий подход), два режима (manual/auto),
// таймеры и устойчивость к перезагрузке страницы (состояние в БД).
//
// Фазы (phase):
//   'loading'  — идёт запрос start/active
//   'none'     — активной тренировки нет (view сделает редирект)
//   'workout'  — основной экран
//   'summary'  — все подходы отмечены, итоги + сохранение/отмена
// Активность (activity) внутри 'workout':
//   'idle' — ждём «Начать подход» (ручной режим)
//   'set'  — подход выполняется (для time-based — обратный отсчёт setLeft)
//   'rest' — отдых (обратный отсчёт restLeft)

const REST_EXTEND_STEP_SEC = 20
const TIME_TRACKING = ['time', 'distance_time']
const REP_TRACKING = ['weight_reps', 'reps']

// Короткий сигнал через WebAudio (без аудио-файлов).
export function playBeep(freq = 880, durationMs = 200) {
  try {
    const Ctx = window.AudioContext ?? window.webkitAudioContext
    if (!Ctx) return
    const ctx = new Ctx()
    const osc = ctx.createOscillator()
    const gain = ctx.createGain()
    osc.type = 'sine'
    osc.frequency.value = freq
    gain.gain.setValueAtTime(0.15, ctx.currentTime)
    gain.gain.exponentialRampToValueAtTime(0.0001, ctx.currentTime + durationMs / 1000)
    osc.connect(gain)
    gain.connect(ctx.destination)
    osc.start()
    osc.stop(ctx.currentTime + durationMs / 1000)
    osc.onended = () => ctx.close()
  } catch {
    // аудио недоступно — не критично
  }
}

function vibrate(pattern) {
  try {
    navigator.vibrate?.(pattern)
  } catch {
    // вибро не поддерживается
  }
}

const emptyFact = () => ({ reps: null, weightKg: null, durationSec: null, distanceM: null })

const useWorkoutSessionStore = defineStore('workoutSession', {
  state: () => ({
    session: null, // объект workouts/active
    loading: false, // запрос start/active
    saving: false, // запрос complete/cancel
    phase: 'loading',
    mode: 'manual', // 'manual' | 'auto'
    activity: 'idle',

    exerciseIdx: 0,
    setIdx: 0,

    restTotal: 0,
    restLeft: 0,
    setLeft: 0, // отсчёт для time-based подхода

    fact: emptyFact(), // редактируемый факт текущего подхода
    totalSeconds: 0, // прошедшее время тренировки (сек)

    _ticker: null,
  }),

  getters: {
    hasActive: (state) => !!state.session && state.session.status === 'in_progress',

    currentExercise: (state) => state.session?.exercises?.[state.exerciseIdx] ?? null,
    currentSet(state) {
      return this.currentExercise?.sets?.[state.setIdx] ?? null
    },

    isTimeExercise: (state) => TIME_TRACKING.includes(state.currentExercise?.tracking_type),

    // Вводимые поля факта по tracking_type (как setFields в редакторе программ)
    factFields(state) {
      switch (state.currentExercise?.tracking_type) {
        case 'weight_reps':
          return ['weightKg', 'reps']
        case 'reps':
          return ['reps']
        case 'time':
          return ['durationSec']
        case 'distance_time':
          return ['distanceM', 'durationSec']
        default:
          return []
      }
    },

    totals(state) {
      const exercises = state.session?.exercises ?? []
      let total = 0
      let completed = 0
      let volume = 0
      for (const ex of exercises) {
        if (ex.is_skipped) continue
        for (const set of ex.sets) {
          total += 1
          if (!set.completed_at) continue
          completed += 1
          if (set.weight_kg != null && set.reps != null) {
            volume += set.weight_kg * set.reps
          }
        }
      }
      return { total, completed, volume }
    },

    // Упражнение, которое пойдёт после текущего (для «дальше» на экране отдыха)
    upcomingLabel(state) {
      const set = this.currentSet
      if (set) return { kind: 'set', setNumber: set.set_number }
      const next = state.session?.exercises
        ?.slice(state.exerciseIdx + 1)
        .find((ex) => !ex.is_skipped && ex.sets.length > 0)
      return next ? { kind: 'exercise', name: next.name } : null
    },
  },

  actions: {
    // --- жизненный цикл -------------------------------------------------

    // Старт из карточки «Следующая тренировка» / календаря.
    // Если активная тренировка уже есть (409) — продолжаем её.
    async startFromEntry(userProgramId, programDayId) {
      const ok = await this.start(userProgramId, programDayId)
      if (ok) return true
      return this.resume()
    },

    async start(userProgramId, programDayId) {
      this.loading = true
      try {
        this.session = await workoutsApi.start({ user_program_id: userProgramId, program_day_id: programDayId })
        this.beginWorkout()
        return true
      } catch (error) {
        toast.error(error.message)
        return false
      } finally {
        this.loading = false
      }
    },

    // Восстановление после перезагрузки страницы / возврат к тренировке.
    async resume() {
      this.loading = true
      try {
        const session = await workoutsApi.active()
        if (!session) {
          this.session = null
          this.phase = 'none'
          return false
        }
        this.session = session
        this.beginWorkout()
        return true
      } catch (error) {
        toast.error(error.message)
        this.phase = 'none'
        return false
      } finally {
        this.loading = false
      }
    },

    beginWorkout() {
      this.phase = 'workout'
      this.activity = 'idle'
      this.restLeft = 0
      this.setLeft = 0
      this.locateCurrent()
      this.startTicker()
      if (this.mode === 'auto') this.beginSet()
    },

    reset() {
      this.stopTicker()
      this.$reset()
      this.phase = 'loading'
    },

    // --- указатель и факт -------------------------------------------------

    // Первый неотмеченный подход среди не пропущенных упражнений.
    locateCurrent() {
      const exercises = this.session?.exercises ?? []
      for (let e = 0; e < exercises.length; e++) {
        const ex = exercises[e]
        if (ex.is_skipped || ex.sets.length === 0) continue
        for (let i = 0; i < ex.sets.length; i++) {
          if (!ex.sets[i].completed_at) {
            this.exerciseIdx = e
            this.setIdx = i
            this.resetFact()
            return
          }
        }
      }
      // всё отмечено — сразу итоги
      this.exerciseIdx = Math.max(0, exercises.length - 1)
      this.setIdx = 0
      this.phase = 'summary'
    },

    // Следующий неотмеченный подход от текущего указателя.
    advance() {
      const exercises = this.session?.exercises ?? []
      for (let e = this.exerciseIdx; e < exercises.length; e++) {
        const ex = exercises[e]
        if (ex.is_skipped || ex.sets.length === 0) continue
        const from = e === this.exerciseIdx ? this.setIdx + 1 : 0
        for (let i = from; i < ex.sets.length; i++) {
          if (!ex.sets[i].completed_at) {
            this.exerciseIdx = e
            this.setIdx = i
            this.resetFact()
            return true
          }
        }
      }
      return false
    },

    resetFact() {
      const ex = this.currentExercise
      const set = this.currentSet
      if (!ex || !set) {
        this.fact = emptyFact()
        return
      }
      const fact = {
        reps: set.reps ?? set.planned_reps ?? null,
        weightKg: set.weight_kg ?? set.planned_weight_kg ?? null,
        durationSec: set.duration_sec ?? set.planned_duration_sec ?? null,
        distanceM: set.distance_m ?? set.planned_distance_m ?? null,
      }
      // вес/повторы переносятся с предыдущего выполненного подхода
      if (REP_TRACKING.includes(ex.tracking_type) && this.setIdx > 0) {
        const prev = ex.sets[this.setIdx - 1]
        if (prev?.completed_at) {
          if (fact.reps == null) fact.reps = prev.reps
          if (fact.weightKg == null) fact.weightKg = prev.weight_kg
        }
      }
      this.fact = fact
    },

    // --- подходы и отдых ---------------------------------------------------

    beginSet() {
      if (this.phase !== 'workout' || !this.currentSet) return
      this.activity = 'set'
      const set = this.currentSet
      const duration = set.duration_sec ?? set.planned_duration_sec
      this.setLeft = this.isTimeExercise && duration ? Number(duration) : 0
    },

    // «Закончить подход»: сохраняем факт + отметку, дальше отдых/следующий.
    async finishSet() {
      const ex = this.currentExercise
      const set = this.currentSet
      if (!ex || !set || this.activity !== 'set') return

      const body = {
        reps: this.fact.reps ?? null,
        weight_kg: this.fact.weightKg ?? null,
        duration_sec: this.fact.durationSec ?? null,
        distance_m: this.fact.distanceM ?? null,
        completed: true,
      }
      try {
        await workoutsApi.updateSet(set.id, body)
      } catch (error) {
        toast.error(error.message)
        return
      }
      set.reps = body.reps
      set.weight_kg = body.weight_kg
      set.duration_sec = body.duration_sec
      set.distance_m = body.distance_m
      set.completed_at = new Date().toISOString()

      this.activity = 'idle'
      playBeep(880)
      vibrate(150)
      this.proceedAfterSet(ex, this.setIdx)
    },

    // Отдых и переход, исходя из только что выполненного подхода.
    proceedAfterSet(ex, setIdx) {
      const rest = this.restAfterSet(ex, setIdx)
      if (!this.advance()) {
        this.phase = 'summary'
        playBeep(1200, 400)
        vibrate([300, 150, 300])
        return
      }
      if (rest > 0) {
        this.activity = 'rest'
        this.restTotal = rest
        this.restLeft = rest
      } else if (this.mode === 'auto') {
        this.beginSet()
      }
    },

    restAfterSet(ex, setIdx) {
      const set = ex.sets[setIdx]
      if (!set) return 0
      if (setIdx + 1 < ex.sets.length) return set.planned_rest_sec ?? 0
      // подход последний в упражнении: отдых после упражнения, если есть следующее
      const exercises = this.session?.exercises ?? []
      const hasTail = exercises
        .slice(this.exerciseIdx + 1)
        .some((item) => !item.is_skipped && item.sets.length > 0)
      return hasTail ? (ex.rest_after_sec ?? 0) : 0
    },

    skipRest() {
      if (this.activity !== 'rest') return
      this.restLeft = 0
      this.activity = 'idle'
      if (this.mode === 'auto') this.beginSet()
    },

    extendRest() {
      if (this.activity !== 'rest') return
      this.restLeft += REST_EXTEND_STEP_SEC
      this.restTotal += REST_EXTEND_STEP_SEC
    },

    // Тик каждую секунду: общий таймер + отсчёты подхода/отдыха.
    startTicker() {
      this.stopTicker()
      this.updateTotal()
      this._ticker = setInterval(() => {
        this.updateTotal()
        if (this.phase !== 'workout') return
        if (this.activity === 'set' && this.setLeft > 0) {
          this.setLeft -= 1
          if (this.setLeft === 0) {
            vibrate(200)
            this.finishSet()
          }
        } else if (this.activity === 'rest' && this.restLeft > 0) {
          this.restLeft -= 1
          if (this.restLeft > 0 && this.restLeft <= 3) playBeep(660, 120)
          if (this.restLeft === 0) {
            playBeep(980, 300)
            vibrate([300, 150, 300])
            this.activity = 'idle'
            if (this.mode === 'auto') this.beginSet()
          }
        }
      }, 1000)
    },

    stopTicker() {
      if (this._ticker) {
        clearInterval(this._ticker)
        this._ticker = null
      }
    },

    updateTotal() {
      if (!this.session?.started_at) return
      const started = new Date(this.session.started_at).getTime()
      if (Number.isNaN(started)) return
      this.totalSeconds = Math.max(0, Math.floor((Date.now() - started) / 1000))
    },

    // --- отклонения от плана -------------------------------------------------

    async skipExercise() {
      const ex = this.currentExercise
      if (!ex) return false
      return this.skipExerciseById(ex.id)
    },

    // Пропуск любого упражнения сессии по workout_exercises.id.
    async skipExerciseById(exerciseId) {
      const exIdx = this.session?.exercises?.findIndex((item) => item.id === exerciseId) ?? -1
      if (exIdx === -1) return false
      try {
        await workoutsApi.updateExercise(exerciseId, { is_skipped: true })
        this.session.exercises[exIdx].is_skipped = true
        // пропустили текущее — двигаемся дальше
        if (exIdx === this.exerciseIdx) {
          this.activity = 'idle'
          this.restLeft = 0
          if (!this.advance()) {
            this.phase = 'summary'
          } else if (this.mode === 'auto') {
            this.beginSet()
          }
        }
        return true
      } catch (error) {
        toast.error(error.message)
        return false
      }
    },

    async replaceExercise(exerciseId) {
      return this.replaceExerciseById(this.currentExercise?.id, exerciseId)
    },

    // Замена любого упражнения сессии другим упражнением каталога.
    async replaceExerciseById(targetId, catalogExerciseId) {
      if (!targetId) return false
      const exIdx = this.session?.exercises?.findIndex((item) => item.id === targetId) ?? -1
      if (exIdx === -1) return false
      try {
        const updated = await workoutsApi.updateExercise(targetId, { exercise_id: catalogExerciseId })
        this.session.exercises.splice(exIdx, 1, updated)
        if (exIdx === this.exerciseIdx) {
          this.activity = 'idle'
          this.setLeft = 0
          this.resetFact()
          if (this.mode === 'auto') this.beginSet()
        }
        return true
      } catch (error) {
        toast.error(error.message)
        return false
      }
    },

    async addExercise(exerciseId, restAfterSec = null) {
      try {
        const created = await workoutsApi.addExercise({
          exercise_id: exerciseId,
          rest_after_sec: restAfterSec,
        })
        this.session.exercises.push(created)
        return created
      } catch (error) {
        toast.error(error.message)
        return null
      }
    },

    async addSet(exerciseId) {
      const ex = this.session?.exercises?.find((item) => item.id === exerciseId)
      if (!ex) return null
      try {
        const created = await workoutsApi.addSet(exerciseId)
        ex.sets.push(created)
        return created
      } catch (error) {
        toast.error(error.message)
        return null
      }
    },

    // Прыжок к любому подходу из списка упражнений.
    jumpTo(exerciseIdx, setIdx) {
      if (this.phase !== 'workout') return
      this.exerciseIdx = exerciseIdx
      this.setIdx = setIdx
      this.activity = 'idle'
      this.restLeft = 0
      this.resetFact()
      if (this.mode === 'auto') this.beginSet()
    },

    // Снять отметку выполнения (исправление по ходу или после завершения).
    async uncheckSet(exerciseIdx, setIdx) {
      const set = this.session?.exercises?.[exerciseIdx]?.sets?.[setIdx]
      if (!set?.completed_at) return
      try {
        await workoutsApi.updateSet(set.id, {
          reps: set.reps ?? null,
          weight_kg: set.weight_kg ?? null,
          duration_sec: set.duration_sec ?? null,
          distance_m: set.distance_m ?? null,
          completed: false,
        })
        set.completed_at = null
        if (exerciseIdx === this.exerciseIdx && setIdx === this.setIdx) {
          this.activity = 'idle'
          this.setLeft = 0
          this.resetFact()
        }
        if (this.phase === 'summary') this.phase = 'workout'
      } catch (error) {
        toast.error(error.message)
      }
    },

    // --- завершение -----------------------------------------------------------

    async complete() {
      if (!this.session) return false
      this.saving = true
      try {
        await workoutsApi.complete()
        this.stopTicker()
        this.session.status = 'completed'
        this.session.finished_at = new Date().toISOString()
        return true
      } catch (error) {
        toast.error(error.message)
        return false
      } finally {
        this.saving = false
      }
    },

    async cancel() {
      if (!this.session) return false
      this.saving = true
      try {
        await workoutsApi.cancel()
        this.stopTicker()
        this.session.status = 'cancelled'
        return true
      } catch (error) {
        toast.error(error.message)
        return false
      } finally {
        this.saving = false
      }
    },
  },
})

export default useWorkoutSessionStore
