<script setup>
// Итоги тренировки: длительность, выполнено подходов, тоннаж.
// «Сохранить» — complete, «Отменить» — cancel.
import { computed, ref } from 'vue'
import { useI18n } from 'vue-i18n'
import { CheckCircle2, Dumbbell, Timer, XCircle } from '@lucide/vue'
import { useRouter } from 'vue-router'
import { toast } from 'vue-sonner'

import { Button } from '@/components/ui/button'
import {
  Dialog,
  DialogContent,
  DialogDescription,
  DialogFooter,
  DialogHeader,
  DialogTitle,
} from '@/components/ui/dialog'
import useWorkoutSessionStore from '@/stores/workoutSession'
import { formatClock } from '@/utils/format'

const { t } = useI18n()
const router = useRouter()
const store = useWorkoutSessionStore()

const cancelOpen = ref(false)

const hasSets = computed(() => store.totals.total > 0)
const volumeLabel = computed(() => Math.round(store.totals.volume).toLocaleString())

function goHome() {
  store.reset()
  router.replace({ name: 'dashboard' })
}

async function save() {
  const ok = await store.complete()
  if (!ok) return
  toast.success(t('session.toast.saved'))
  goHome()
}

async function cancelWorkout() {
  cancelOpen.value = false
  const ok = await store.cancel()
  if (!ok) return
  toast.success(t('session.toast.cancelled'))
  goHome()
}
</script>

<template>
  <div class="mx-auto flex w-full max-w-3xl flex-col gap-6 px-4 py-8">
    <div class="flex flex-col items-center gap-2 text-center">
      <CheckCircle2 class="text-green-600 size-12" />
      <h1 class="text-2xl font-bold">{{ $t('session.summary.title') }}</h1>
      <p class="text-muted-foreground text-sm">{{ $t('session.summary.subtitle') }}</p>
    </div>

    <div v-if="hasSets" class="grid grid-cols-3 gap-3">
      <div class="bg-card flex flex-col items-center gap-1 rounded-xl border p-4">
        <Timer class="text-muted-foreground size-5" />
        <p class="text-xl font-bold tabular-nums">{{ formatClock(store.totalSeconds) }}</p>
        <p class="text-muted-foreground text-xs">{{ $t('session.summary.duration') }}</p>
      </div>
      <div class="bg-card flex flex-col items-center gap-1 rounded-xl border p-4">
        <CheckCircle2 class="text-muted-foreground size-5" />
        <p class="text-xl font-bold tabular-nums">{{ store.totals.completed }}/{{ store.totals.total }}</p>
        <p class="text-muted-foreground text-xs">{{ $t('session.summary.setsDone') }}</p>
      </div>
      <div class="bg-card flex flex-col items-center gap-1 rounded-xl border p-4">
        <Dumbbell class="text-muted-foreground size-5" />
        <p class="text-xl font-bold tabular-nums">{{ volumeLabel }}</p>
        <p class="text-muted-foreground text-xs">{{ $t('session.summary.volume') }}</p>
      </div>
    </div>
    <p v-else class="text-muted-foreground rounded-lg border border-dashed py-10 text-center text-sm">
      {{ $t('session.summary.empty') }}
    </p>

    <div class="flex flex-col gap-2">
      <Button class="h-12 text-base" size="lg" :disabled="store.saving" @click="save">
        {{ $t('session.summary.saveAndExit') }}
      </Button>
      <Button variant="outline" class="text-destructive" :disabled="store.saving" @click="cancelOpen = true">
        <XCircle />
        {{ $t('session.summary.cancelWorkout') }}
      </Button>
    </div>

    <Dialog v-model:open="cancelOpen">
      <DialogContent class="sm:max-w-md">
        <DialogHeader>
          <DialogTitle>{{ $t('session.summary.cancelWorkout') }}</DialogTitle>
          <DialogDescription>{{ $t('session.workout.cancelConfirm') }}</DialogDescription>
        </DialogHeader>
        <DialogFooter>
          <Button variant="outline" @click="cancelOpen = false">
            {{ $t('common.back') }}
          </Button>
          <Button variant="destructive" :disabled="store.saving" @click="cancelWorkout">
            {{ $t('session.workout.cancel') }}
          </Button>
        </DialogFooter>
      </DialogContent>
    </Dialog>
  </div>
</template>
