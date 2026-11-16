<script setup>
// Проведение тренировки: восстановление активной сессии из БД
// (переживает F5), экраны подхода/отдыха/итогов, wake-lock,
// чтобы экран не гас во время тренировки.
import { computed, onBeforeUnmount, onMounted, watch } from 'vue'
import { useRouter } from 'vue-router'
import { useWakeLock } from '@vueuse/core'

import CurrentExerciseCard from '@/components/session/CurrentExerciseCard.vue'
import RestCard from '@/components/session/RestCard.vue'
import SessionHeader from '@/components/session/SessionHeader.vue'
import SetList from '@/components/session/SetList.vue'
import SummaryScreen from '@/components/session/SummaryScreen.vue'
import TimeSetCountdown from '@/components/session/TimeSetCountdown.vue'
import { Skeleton } from '@/components/ui/skeleton'
import useWorkoutSessionStore from '@/stores/workoutSession'

const router = useRouter()
const store = useWorkoutSessionStore()

const { isSupported, request, release } = useWakeLock()

const showWorkout = computed(() => store.phase === 'workout')

watch(
  () => store.phase,
  async (phase) => {
    if (!isSupported.value) return
    if (phase === 'workout') {
      try {
        await request('screen')
      } catch {
        // wake-lock не критичен
      }
    } else {
      release()
    }
  },
)

onMounted(async () => {
  const ok = await store.resume()
  if (!ok) {
    router.replace({ name: 'dashboard' })
  }
})

onBeforeUnmount(() => {
  release()
})
</script>

<template>
  <div class="flex min-h-dvh flex-col">
    <SessionHeader v-if="store.hasActive" />

    <main class="flex flex-1 flex-col gap-4 pb-8">
      <template v-if="store.phase === 'loading'">
        <div class="mx-auto flex w-full max-w-3xl flex-col gap-4 px-4 pt-4">
          <Skeleton class="h-56 w-full rounded-xl" />
          <Skeleton class="h-40 w-full rounded-xl" />
        </div>
      </template>

      <template v-else-if="showWorkout">
        <CurrentExerciseCard />

        <RestCard v-if="store.activity === 'rest'" />
        <TimeSetCountdown
          v-else-if="store.isTimeExercise && store.activity === 'set'"
        />
        <SetList v-else />
      </template>

      <SummaryScreen v-else-if="store.phase === 'summary' && store.hasActive" />
    </main>
  </div>
</template>
