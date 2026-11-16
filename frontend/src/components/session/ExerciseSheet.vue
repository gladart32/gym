<script setup>
// Список упражнений тренировки (bottom sheet): переход к любому
// упражнению, добавление подхода, пропуск и замена.
import { computed, ref } from 'vue'
import { useI18n } from 'vue-i18n'
import { Ban, Plus, Replace } from '@lucide/vue'
import { toast } from 'vue-sonner'

import ExercisePickerDialog from '@/components/programs/ExercisePickerDialog.vue'
import { Badge } from '@/components/ui/badge'
import { Button } from '@/components/ui/button'
import {
  Sheet,
  SheetContent,
  SheetDescription,
  SheetHeader,
  SheetTitle,
} from '@/components/ui/sheet'
import useWorkoutSessionStore from '@/stores/workoutSession'

const { t } = useI18n()
const store = useWorkoutSessionStore()

const open = defineModel('open', { type: Boolean, default: false })

const pickerOpen = ref(false)
const replaceTarget = ref(null)

const exercises = computed(() => store.session?.exercises ?? [])

function doneCount(exercise) {
  return exercise.sets.filter((s) => s.completed_at).length
}

function goTo(exerciseIdx) {
  const exercise = exercises.value[exerciseIdx]
  if (!exercise || exercise.is_skipped || exercise.sets.length === 0) return
  const nextIdx = exercise.sets.findIndex((s) => !s.completed_at)
  store.jumpTo(exerciseIdx, nextIdx === -1 ? 0 : nextIdx)
  open.value = false
}

async function addSet(exercise) {
  const created = await store.addSet(exercise.id)
  if (created) toast.success(t('session.toast.setAdded'))
}

async function skip(exercise) {
  const ok = await store.skipExerciseById(exercise.id)
  if (ok) toast.info(t('session.toast.exerciseSkipped'))
}

function openReplace(exercise) {
  replaceTarget.value = exercise
  pickerOpen.value = true
}

async function onPick(catalogExercise) {
  pickerOpen.value = false
  if (!catalogExercise?.id) return
  if (replaceTarget.value) {
    const ok = await store.replaceExerciseById(replaceTarget.value.id, catalogExercise.id)
    if (ok) toast.success(t('session.toast.exerciseReplaced'))
    replaceTarget.value = null
  } else {
    const created = await store.addExercise(catalogExercise.id)
    if (created) toast.success(t('session.toast.exerciseAdded'))
  }
}
</script>

<template>
  <Sheet v-model:open="open">
    <SheetContent side="bottom" class="mx-auto max-h-[80dvh] max-w-3xl overflow-y-auto rounded-t-xl">
      <SheetHeader>
        <SheetTitle>{{ $t('session.list.title') }}</SheetTitle>
        <SheetDescription class="sr-only">{{ $t('session.list.title') }}</SheetDescription>
      </SheetHeader>

      <ul class="flex flex-col gap-2 pb-4">
        <li
          v-for="(exercise, idx) in exercises"
          :key="exercise.id"
          class="bg-card flex flex-col gap-2 rounded-lg border p-3"
          :class="{ 'border-primary': idx === store.exerciseIdx }"
        >
          <div class="flex min-w-0 items-center gap-2">
            <button
              type="button"
              class="flex min-w-0 flex-1 flex-col items-start text-left"
              @click="goTo(idx)"
            >
              <span class="truncate text-sm font-medium">{{ exercise.name }}</span>
              <span class="text-muted-foreground text-xs tabular-nums">
                {{ doneCount(exercise) }}/{{ exercise.sets.length }}
              </span>
            </button>
            <Badge v-if="exercise.is_skipped" variant="outline">
              {{ $t('session.exercise.skipped') }}
            </Badge>
            <Badge v-if="exercise.replaced" variant="secondary">
              {{ $t('session.exercise.replaced') }}
            </Badge>
            <div class="flex shrink-0 items-center gap-1">
              <Button variant="ghost" size="icon" class="size-8" :aria-label="$t('session.exercise.addSet')" @click="addSet(exercise)">
                <Plus />
              </Button>
              <Button variant="ghost" size="icon" class="size-8" :aria-label="$t('session.exercise.replace')" @click="openReplace(exercise)">
                <Replace />
              </Button>
              <Button
                variant="ghost"
                size="icon"
                class="text-muted-foreground size-8"
                :disabled="exercise.is_skipped"
                :aria-label="$t('session.exercise.skip')"
                @click="skip(exercise)"
              >
                <Ban />
              </Button>
            </div>
          </div>
        </li>
      </ul>
    </SheetContent>
  </Sheet>

  <ExercisePickerDialog v-model:open="pickerOpen" @select="onPick" />
</template>
