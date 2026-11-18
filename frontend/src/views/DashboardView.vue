<script setup>
import { computed, onMounted } from 'vue'
import { storeToRefs } from 'pinia'
import { CalendarCheck, Dumbbell, Flame, Play, Trophy } from '@lucide/vue'
import { useI18n } from 'vue-i18n'
import { useRouter } from 'vue-router'

import WeeklyActivityChart from '@/components/dashboard/WeeklyActivityChart.vue'
import { Badge } from '@/components/ui/badge'
import { Button } from '@/components/ui/button'
import { Card, CardContent, CardDescription, CardHeader, CardTitle } from '@/components/ui/card'
import { Skeleton } from '@/components/ui/skeleton'
import useScheduleStore from '@/stores/schedule'
import useStatsStore from '@/stores/stats'
import useWorkoutSessionStore from '@/stores/workoutSession'
import { todayStr } from '@/utils/date'
import { formatClock, formatTonnage } from '@/utils/format'

const { t, locale } = useI18n()
const router = useRouter()
const scheduleStore = useScheduleStore()
const sessionStore = useWorkoutSessionStore()
const statsStore = useStatsStore()
const { nextWorkout, nextLoading } = storeToRefs(scheduleStore)
const { dashboard: stats, loading: statsLoading } = storeToRefs(statsStore)

// «Сегодня» / «Завтра» / «Через N дн.»
const untilLabel = computed(() => {
  const next = nextWorkout.value
  if (!next || next.days_until < 0) return ''
  if (next.days_until === 0) return t('dashboard.today')
  if (next.days_until === 1) return t('dashboard.tomorrow')
  return t('dashboard.inDays', next.days_until)
})

const dateLabel = computed(() => {
  const next = nextWorkout.value
  if (!next || !next.date) return ''
  const [y, m, d] = next.date.split('-').map(Number)
  return new Date(y, m - 1, d).toLocaleDateString(locale.value, {
    weekday: 'long',
    day: 'numeric',
    month: 'long',
  })
})

const tonnageLabel = computed(() =>
  formatTonnage(stats.value?.tonnage_month_kg ?? 0, locale.value, t('dashboard.kg'), t('dashboard.tons')),
)

const lastWorkoutDate = computed(() => {
  const last = stats.value?.last_workout
  if (!last?.date) return ''
  const [y, m, d] = last.date.split('-').map(Number)
  return new Date(y, m - 1, d).toLocaleDateString(locale.value, {
    day: 'numeric',
    month: 'long',
  })
})

const statCards = computed(() => [
  {
    icon: Dumbbell,
    title: t('dashboard.workouts'),
    value: stats.value ? String(stats.value.workouts_month) : '—',
    hint: t('dashboard.workoutsHint', { n: stats.value?.workouts_total ?? 0 }),
  },
  {
    icon: Flame,
    title: t('dashboard.streakTitle'),
    value: stats.value?.streak ? String(stats.value.streak.current) : '—',
    hint: stats.value?.streak
      ? t('dashboard.streakHint', {
          done: stats.value.streak.completed,
          total: stats.value.streak.scheduled,
        })
      : t('dashboard.streakNone'),
  },
  {
    icon: Trophy,
    title: t('dashboard.tonnage'),
    value: stats.value ? tonnageLabel.value : '—',
    hint: t('dashboard.tonnageHint'),
  },
])

// «Начать тренировку» из карточки дня: старт сессии (или возврат к активной)
async function startSession(entry) {
  if (sessionStore.hasActive) {
    router.push({ name: 'workout-session' })
    return
  }
  const ok = await sessionStore.startFromEntry(entry.user_program_id, entry.program_day_id)
  if (ok || sessionStore.hasActive) {
    router.push({ name: 'workout-session' })
  }
}

async function resumeSession() {
  router.push({ name: 'workout-session' })
}

onMounted(() => {
  scheduleStore.loadNext(todayStr())
  statsStore.load()
  // если есть незавершённая тренировка — показываем баннер возврата
  sessionStore.resume()
})
</script>

<template>
  <div class="flex flex-col gap-6">
    <h1 class="text-3xl font-bold text-center sm:text-left">{{ $t('nav.dashboard') }}</h1>

    <Skeleton v-if="nextLoading && !nextWorkout" class="h-40 w-full" />

    <!-- Незавершённая тренировка: баннер возврата -->
    <Card v-else-if="sessionStore.hasActive" class="border-primary">
      <CardContent class="flex items-center gap-3 py-4">
        <Play class="text-primary size-5 shrink-0" />
        <p class="flex-1 text-sm font-medium">{{ $t('session.activeBanner') }}</p>
        <Button size="sm" @click="resumeSession">
          {{ $t('session.goToActive') }}
        </Button>
      </CardContent>
    </Card>

    <Card v-else-if="nextWorkout && nextWorkout.entries.length > 0">
      <CardHeader class="flex flex-row items-start justify-between space-y-0">
        <div class="flex flex-col gap-1">
          <CardDescription class="flex items-center gap-1.5">
            <CalendarCheck class="size-4" />
            {{ $t('dashboard.nextWorkout') }}
          </CardDescription>
          <CardTitle class="first-letter:uppercase">{{ dateLabel }}</CardTitle>
        </div>
        <Badge>{{ untilLabel }}</Badge>
      </CardHeader>
      <CardContent class="flex flex-col gap-2">
        <div
          v-for="entry in nextWorkout.entries"
          :key="`${entry.user_program_id}-${entry.program_day_id}`"
          class="bg-primary/10 flex flex-wrap items-center gap-x-2 gap-y-0.5 rounded-md border p-2.5"
        >
          <div class="flex min-w-0 flex-1 flex-col gap-y-0.5">
            <span class="font-medium">{{ entry.day_name }}</span>
            <span class="text-muted-foreground text-sm">{{ entry.program_name }}</span>
          </div>
          <Button size="sm" :disabled="sessionStore.loading" @click="startSession(entry)">
            <Play />
            {{ $t('session.start') }}
          </Button>
        </div>
        <Button
          variant="ghost"
          size="sm"
          class="self-start"
          @click="router.push({ name: 'calendar' })"
        >
          {{ $t('dashboard.toCalendar') }}
        </Button>
      </CardContent>
    </Card>

    <Card v-else>
      <CardHeader>
        <CardTitle class="flex items-center gap-2">
          <Dumbbell class="size-5" />
          {{ $t('dashboard.noWorkoutsTitle') }}
        </CardTitle>
        <CardDescription>{{ $t('dashboard.noWorkoutsHint') }}</CardDescription>
      </CardHeader>
      <CardContent>
        <Button size="sm" @click="router.push({ name: 'workouts' })">
          {{ $t('dashboard.goToWorkouts') }}
        </Button>
      </CardContent>
    </Card>

    <!-- Счётчики -->
    <div class="grid gap-4 sm:grid-cols-3">
      <Card v-for="stat in statCards" :key="stat.title">
        <CardHeader>
          <CardDescription class="flex items-center gap-1.5">
            <component :is="stat.icon" class="size-4" />
            {{ stat.title }}
          </CardDescription>
          <CardTitle class="text-4xl">{{ stat.value }}</CardTitle>
        </CardHeader>
        <CardContent class="text-muted-foreground text-sm">
          {{ stat.hint }}
        </CardContent>
      </Card>
    </div>

    <!-- Активность за 12 недель -->
    <Card v-if="statsLoading && !stats">
      <CardHeader>
        <Skeleton class="h-5 w-56" />
      </CardHeader>
      <CardContent>
        <Skeleton class="h-56 w-full" />
      </CardContent>
    </Card>
    <Card v-else-if="stats">
      <CardHeader>
        <CardDescription>{{ $t('dashboard.activityTitle') }}</CardDescription>
        <CardTitle>{{ $t('dashboard.activitySubtitle') }}</CardTitle>
      </CardHeader>
      <CardContent>
        <WeeklyActivityChart :weeks="stats.weekly_activity" />
      </CardContent>
    </Card>

    <!-- Последняя тренировка -->
    <Card v-if="stats?.last_workout">
      <CardHeader>
        <CardDescription>{{ $t('dashboard.lastTitle') }}</CardDescription>
        <CardTitle class="first-letter:uppercase">{{ lastWorkoutDate }} · {{ stats.last_workout.name }}</CardTitle>
      </CardHeader>
      <CardContent class="text-muted-foreground flex flex-wrap gap-x-6 gap-y-1 text-sm">
        <span class="flex items-center gap-1.5">
          <Play class="size-4" />
          {{ formatClock(stats.last_workout.duration_sec) }}
        </span>
        <span class="flex items-center gap-1.5">
          <Dumbbell class="size-4" />
          {{ $t('dashboard.lastSets', stats.last_workout.sets_done) }}
          · {{ $t('dashboard.lastExercises', stats.last_workout.exercises_done) }}
        </span>
        <span class="flex items-center gap-1.5">
          <Flame class="size-4" />
          {{ formatTonnage(stats.last_workout.total_volume_kg, locale, t('dashboard.kg'), t('dashboard.tons')) }}
        </span>
      </CardContent>
    </Card>
  </div>
</template>
