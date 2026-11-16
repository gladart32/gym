<script setup>
// Подходы текущего упражнения: редактор факта активного подхода
// (вес/повторы/время/дистанция — по tracking_type), кнопки
// «Начать/Закончить подход» и таблица всех подходов с планом и фактом.
import { computed } from 'vue'
import { useI18n } from 'vue-i18n'
import { Check, Minus, Play, Plus, Square } from '@lucide/vue'

import { Button } from '@/components/ui/button'
import { Input } from '@/components/ui/input'
import useWorkoutSessionStore from '@/stores/workoutSession'

const { t } = useI18n()
const store = useWorkoutSessionStore()

const exercise = computed(() => store.currentExercise)

// Поля факта активного подхода по tracking_type
const fields = computed(() =>
  store.factFields.map((key) => {
    const meta = {
      weightKg: { key: 'weightKg', label: t('session.set.weight'), step: 2.5, min: -300, max: 500 },
      reps: { key: 'reps', label: t('session.set.reps'), step: 1, min: 0, max: 2000 },
      durationSec: { key: 'durationSec', label: t('session.set.duration'), step: 5, min: 0, max: 86400 },
      distanceM: { key: 'distanceM', label: t('session.set.distance'), step: 100, min: 0, max: 1000000 },
    }[key]
    return meta
  }),
)

const isRunning = computed(() => store.activity === 'set')

function changeFact(field, direction) {
  const meta = fields.value.find((f) => f.key === field)
  if (!meta) return
  const current = Number(store.fact[field]) || 0
  let next = current + direction * meta.step
  if (meta.min > next) next = meta.min
  // плавающая точка — округляем до шага
  store.fact[field] = Math.round(next * 100) / 100
}

// «12×40», «15», «60 сек», «1000 м» — компактное отображение плана/факта
function valueLabel(set, trackingType, planned) {
  const weight = planned ? set.planned_weight_kg : set.weight_kg
  const reps = planned ? set.planned_reps : set.reps
  const duration = planned ? set.planned_duration_sec : set.duration_sec
  const distance = planned ? set.planned_distance_m : set.distance_m
  switch (trackingType) {
    case 'weight_reps': {
      if (weight == null && reps == null) return '—'
      return weight != null ? `${reps ?? '—'}×${weight}` : `${reps ?? '—'}`
    }
    case 'reps':
      return reps ?? '—'
    case 'time':
      return duration != null ? `${duration} ${t('session.units.secShort')}` : '—'
    case 'distance_time':
      return distance != null || duration != null
        ? `${distance ?? '—'} ${t('session.units.mShort')} / ${duration != null ? formatShort(duration) : '—'}`
        : '—'
    default:
      return '—'
  }
}

function formatShort(sec) {
  const s = Math.floor(Number(sec) || 0)
  const m = Math.floor(s / 60)
  const rest = s % 60
  return `${m}:${String(rest).padStart(2, '0')}`
}
</script>

<template>
  <div v-if="exercise" class="mx-auto w-full max-w-3xl px-4">
    <div class="flex flex-col gap-4">
      <!-- Активный подход: факт + управление -->
      <div v-if="store.currentSet && !store.currentSet.completed_at" class="bg-card rounded-xl border p-4">
        <p class="mb-3 text-sm font-semibold">
          {{ $t('session.set.title', { n: store.currentSet.set_number }) }}
          <span v-if="exercise.rest_after_sec != null || store.currentSet.planned_rest_sec != null" class="text-muted-foreground font-normal">
            ·
            {{ $t('session.set.rest') }}:
            {{ store.currentSet.planned_rest_sec ?? exercise.rest_after_sec ?? 0 }}
          </span>
        </p>

        <div class="grid grid-cols-2 gap-3 sm:grid-cols-4">
          <div v-for="meta in fields" :key="meta.key" class="flex flex-col gap-1.5">
            <label class="text-muted-foreground text-xs">{{ meta.label }}</label>
            <div class="flex items-center gap-1">
              <Button variant="outline" size="icon" class="size-8 shrink-0" @click="changeFact(meta.key, -1)">
                <Minus />
              </Button>
              <Input
                class="h-8 text-center tabular-nums"
                type="number"
                inputmode="decimal"
                :model-value="store.fact[meta.key]"
                @update:model-value="(v) => (store.fact[meta.key] = v === '' || v == null ? null : Number(v))"
              />
              <Button variant="outline" size="icon" class="size-8 shrink-0" @click="changeFact(meta.key, +1)">
                <Plus />
              </Button>
            </div>
          </div>
        </div>

        <Button
          v-if="!isRunning"
          class="mt-4 h-12 w-full text-base"
          size="lg"
          @click="store.beginSet"
        >
          <Play />
          {{ $t('session.set.start') }}
        </Button>
        <Button
          v-else
          class="mt-4 h-12 w-full text-base"
          size="lg"
          @click="store.finishSet"
        >
          <Square />
          {{ $t('session.set.finish') }}
        </Button>
      </div>

      <!-- Все подходы упражнения -->
      <div class="bg-card overflow-hidden rounded-xl border">
        <div class="text-muted-foreground border-b px-4 py-2 text-xs font-medium tracking-wide uppercase">
          {{ $t('session.set.plan') }} → {{ $t('session.set.fact') }}
        </div>
        <ul>
          <li
            v-for="(set, i) in exercise.sets"
            :key="set.id"
            class="flex items-center gap-3 border-b px-4 py-2.5 last:border-0"
            :class="{ 'bg-primary/5': i === store.setIdx }"
          >
            <button
              type="button"
              class="flex min-w-0 flex-1 items-center gap-3 text-left"
              @click="!set.completed_at && store.jumpTo(store.exerciseIdx, i)"
            >
              <span class="text-muted-foreground w-6 shrink-0 text-sm tabular-nums">{{ set.set_number }}</span>
              <span class="text-muted-foreground truncate text-sm tabular-nums">
                {{ valueLabel(set, exercise.tracking_type, true) }}
              </span>
              <span class="truncate text-sm font-medium tabular-nums">
                {{ valueLabel(set, exercise.tracking_type, false) }}
              </span>
            </button>

            <span
              v-if="set.completed_at"
              class="flex size-7 shrink-0 cursor-pointer items-center justify-center rounded-full bg-green-600 text-white"
              role="button"
              tabindex="0"
              :aria-label="$t('session.set.completed')"
              :title="$t('session.set.completed')"
              @click.stop="store.uncheckSet(store.exerciseIdx, i)"
              @keydown.enter.stop="store.uncheckSet(store.exerciseIdx, i)"
            >
              <Check class="size-4" />
            </span>
            <span
              v-else
              class="border-muted-foreground/40 size-7 shrink-0 rounded-full border-2 border-dashed"
            />
          </li>
          <li v-if="exercise.sets.length === 0" class="text-muted-foreground px-4 py-6 text-center text-sm">
            {{ $t('session.set.empty') }}
          </li>
        </ul>
      </div>
    </div>
  </div>
</template>
