<script setup>
import { computed, onMounted } from 'vue'
import { storeToRefs } from 'pinia'
import { CalendarCheck, Dumbbell, Play } from '@lucide/vue'
import { useI18n } from 'vue-i18n'
import { useRouter } from 'vue-router'

import { Badge } from '@/components/ui/badge'
import { Button } from '@/components/ui/button'
import { Card, CardContent, CardDescription, CardHeader, CardTitle } from '@/components/ui/card'
import { Skeleton } from '@/components/ui/skeleton'
import useScheduleStore from '@/stores/schedule'
import useWorkoutSessionStore from '@/stores/workoutSession'
import { todayStr } from '@/utils/date'

const { t, locale } = useI18n()
const router = useRouter()
const scheduleStore = useScheduleStore()
const sessionStore = useWorkoutSessionStore()
const { nextWorkout, nextLoading } = storeToRefs(scheduleStore)

const stats = computed(() => [
  { title: t('dashboard.workouts'), value: '—', description: t('dashboard.workoutsHint') },
  { title: t('dashboard.exercises'), value: '—', description: t('dashboard.exercisesHint') },
  { title: t('dashboard.tonnage'), value: '—', description: t('dashboard.tonnageHint') },
])

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
  // если есть незавершённая тренировка — показываем баннер возврата
  sessionStore.resume()
})
</script>

<template>
  <div class="flex flex-col gap-6">
    <h1 class="text-3xl font-bold">{{ $t('nav.dashboard') }}</h1>

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

    <div class="grid gap-4 sm:grid-cols-3">
      <Card v-for="stat in stats" :key="stat.title">
        <CardHeader>
          <CardDescription>{{ stat.title }}</CardDescription>
          <CardTitle class="text-4xl">{{ stat.value }}</CardTitle>
        </CardHeader>
        <CardContent class="text-muted-foreground text-sm">
          {{ stat.description }}
        </CardContent>
      </Card>
    </div>
  </div>
</template>
