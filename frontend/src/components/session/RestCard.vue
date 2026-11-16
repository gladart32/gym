<script setup>
// Экран отдыха между подходами: большой обратный отсчёт,
// «что дальше», пропуск и добавление времени.
import { computed } from 'vue'
import { useI18n } from 'vue-i18n'
import { FastForward, Plus } from '@lucide/vue'

import { Button } from '@/components/ui/button'
import useWorkoutSessionStore from '@/stores/workoutSession'
import { formatClock } from '@/utils/format'

const { t } = useI18n()
const store = useWorkoutSessionStore()

const clock = computed(() => formatClock(store.restLeft))
const percent = computed(() =>
  store.restTotal > 0 ? Math.round((store.restLeft / store.restTotal) * 100) : 0,
)

const nextLabel = computed(() => {
  const up = store.upcomingLabel
  if (!up) return ''
  return up.kind === 'set'
    ? t('session.rest.nextSet', { n: up.setNumber })
    : t('session.rest.nextExercise', { name: up.name })
})
</script>

<template>
  <div class="mx-auto w-full max-w-3xl px-4">
    <div class="flex flex-col items-center gap-4 rounded-xl border border-dashed py-8">
      <p class="text-muted-foreground text-sm font-medium tracking-wide uppercase">
        {{ $t('session.rest.title') }}
      </p>

      <p class="text-6xl font-bold tabular-nums sm:text-7xl" aria-live="polite">
        {{ clock }}
      </p>

      <div class="bg-muted h-2 w-56 overflow-hidden rounded-full">
        <div class="bg-primary h-full transition-all duration-1000 ease-linear" :style="{ width: `${percent}%` }" />
      </div>

      <p v-if="nextLabel" class="text-muted-foreground text-sm">{{ nextLabel }}</p>

      <div class="flex gap-2">
        <Button variant="outline" size="sm" @click="store.extendRest">
          <Plus />
          {{ $t('session.rest.extend') }}
        </Button>
        <Button size="sm" @click="store.skipRest">
          <FastForward />
          {{ $t('session.rest.skip') }}
        </Button>
      </div>
    </div>
  </div>
</template>
