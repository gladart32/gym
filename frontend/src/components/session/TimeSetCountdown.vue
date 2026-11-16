<script setup>
// Обратный отсчёт подхода для time/distance_time упражнений:
// по окончании подход отмечается автоматически (в ручном режиме
// можно закончить раньше).
import { computed } from 'vue'
import { Flag } from '@lucide/vue'

import { Button } from '@/components/ui/button'
import useWorkoutSessionStore from '@/stores/workoutSession'
import { formatClock } from '@/utils/format'

const store = useWorkoutSessionStore()

const clock = computed(() => formatClock(store.setLeft))
const total = computed(() => {
  const set = store.currentSet
  const duration = set?.duration_sec ?? set?.planned_duration_sec
  return Number(duration) || 0
})

const percent = computed(() =>
  total.value > 0 ? Math.min(100, Math.round(((total.value - store.setLeft) / total.value) * 100)) : 0,
)

const isAuto = computed(() => store.mode === 'auto')
</script>

<template>
  <div class="mx-auto w-full max-w-3xl px-4">
    <div class="flex flex-col items-center gap-4 rounded-xl border py-8">
      <p class="text-muted-foreground text-sm font-medium tracking-wide uppercase">
        {{ $t('session.set.countdown') }}
      </p>

      <p class="text-6xl font-bold tabular-nums sm:text-7xl" aria-live="polite">
        {{ clock }}
      </p>

      <div class="bg-muted h-2 w-56 overflow-hidden rounded-full">
        <div class="bg-primary h-full transition-all duration-1000 ease-linear" :style="{ width: `${percent}%` }" />
      </div>

      <p class="text-muted-foreground text-sm">
        {{
          isAuto
            ? $t('session.mode.autoHint')
            : $t('session.mode.manualHint')
        }}
      </p>

      <Button v-if="!isAuto" variant="outline" size="sm" @click="store.finishSet">
        <Flag />
        {{ $t('session.set.finish') }}
      </Button>
    </div>
  </div>
</template>
