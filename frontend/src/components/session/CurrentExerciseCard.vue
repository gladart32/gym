<script setup>
// Карточка текущего упражнения: gif-визуализация «как делать»,
// название, инвентарь и быстрые действия по ходу тренировки.
import { computed, ref } from 'vue'
import { useI18n } from 'vue-i18n'
import { ChevronDown, ListPlus, Plus, Repeat, SkipForward } from '@lucide/vue'
import { toast } from 'vue-sonner'

import ExercisePickerDialog from '@/components/programs/ExercisePickerDialog.vue'
import { Badge } from '@/components/ui/badge'
import { Button } from '@/components/ui/button'
import useWorkoutSessionStore from '@/stores/workoutSession'

const { t } = useI18n()
const store = useWorkoutSessionStore()

const sheetOpen = ref(false)
const pickerOpen = ref(false)
const pickerMode = ref('replace') // 'replace' | 'add'

const exercise = computed(() => store.currentExercise)
const total = computed(() => store.session?.exercises?.length ?? 0)

const mediaSrc = computed(() => exercise.value?.gif_url || exercise.value?.thumbnail_url || '')

// Короткая атрибуция медиа («© Gym visual») — конвенция каталога упражнений
const attributionShort = computed(() => {
  const full = exercise.value?.attribution
  if (!full) return ''
  return full.split('—')[0].trim()
})

async function skip() {
  const ok = await store.skipExercise()
  if (ok) toast.info(t('session.toast.exerciseSkipped'))
}

function openReplace() {
  pickerMode.value = 'replace'
  pickerOpen.value = true
}

async function addSet() {
  const ex = exercise.value
  if (!ex) return
  const created = await store.addSet(ex.id)
  if (created) {
    toast.success(t('session.toast.setAdded'))
    // прыгаем на новый подход, чтобы сразу начать его
    store.jumpTo(store.exerciseIdx, ex.sets.length - 1)
  }
}

async function onPick(catalogExercise) {
  pickerOpen.value = false
  if (!catalogExercise?.id) return
  if (pickerMode.value === 'replace') {
    const ok = await store.replaceExercise(catalogExercise.id)
    if (ok) toast.success(t('session.toast.exerciseReplaced'))
  } else {
    const created = await store.addExercise(catalogExercise.id)
    if (created) {
      toast.success(t('session.toast.exerciseAdded'))
      store.jumpTo(store.session.exercises.length - 1, 0)
    }
  }
}
</script>

<template>
  <div v-if="exercise" class="mx-auto w-full max-w-3xl px-4 pt-4">
    <div class="bg-card rounded-xl border">
      <div class="flex flex-col gap-4 p-4 sm:flex-row">
        <div
          class="bg-muted relative flex aspect-square size-32 shrink-0 items-center justify-center overflow-hidden rounded-lg sm:size-44"
        >
          <img
            v-if="mediaSrc"
            :src="mediaSrc"
            :alt="exercise.name"
            class="size-full object-contain"
            @error="$event.target.style.display = 'none'"
          />
          <span
            v-if="mediaSrc && attributionShort"
            class="bg-background/70 text-muted-foreground absolute right-1 bottom-1 rounded px-1 text-[10px] leading-4"
          >
            {{ attributionShort }}
          </span>
        </div>

        <div class="flex min-w-0 flex-1 flex-col gap-2">
          <div class="flex flex-wrap items-center gap-2">
            <span class="text-muted-foreground text-xs">
              {{ $t('session.exercise.of', { current: store.exerciseIdx + 1, total }) }}
            </span>
            <Badge v-if="exercise.is_skipped" variant="outline">
              {{ $t('session.exercise.skipped') }}
            </Badge>
            <Badge v-if="exercise.replaced" variant="secondary">
              {{ $t('session.exercise.replaced') }}
            </Badge>
            <Badge v-if="!exercise.program_exercise_id" variant="secondary">
              {{ $t('session.exercise.added') }}
            </Badge>
          </div>

          <h2 class="text-lg leading-tight font-bold">{{ exercise.name }}</h2>
          <div class="flex flex-wrap items-center gap-1.5">
            <Badge v-if="exercise.equipment" variant="outline">{{ exercise.equipment }}</Badge>
            <Badge v-if="exercise.rest_after_sec" variant="secondary">
              {{ $t('session.exercise.restAfter', { sec: exercise.rest_after_sec }) }}
            </Badge>
          </div>
        </div>
      </div>

      <div class="flex flex-wrap gap-2 border-t px-4 py-3">
        <Button variant="outline" size="sm" @click="skip">
          <SkipForward />
          {{ $t('session.exercise.skip') }}
        </Button>
        <Button variant="outline" size="sm" @click="openReplace">
          <Repeat />
          {{ $t('session.exercise.replace') }}
        </Button>
        <Button variant="outline" size="sm" @click="addSet">
          <Plus />
          {{ $t('session.exercise.addSet') }}
        </Button>
        <Button variant="ghost" size="sm" class="text-muted-foreground" @click="sheetOpen = true">
          <ListPlus />
          <ChevronDown />
        </Button>
      </div>
    </div>
  </div>

  <ExerciseSheet v-model:open="sheetOpen" />

  <ExercisePickerDialog v-model:open="pickerOpen" @select="onPick" />
</template>
