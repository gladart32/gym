<script setup>
import { computed, onMounted, reactive, ref } from 'vue'
import { ChevronDown, ChevronUp, Plus, Trash2 } from '@lucide/vue'
import { useI18n } from 'vue-i18n'
import { useRoute, useRouter } from 'vue-router'
import { toast } from 'vue-sonner'

import ExercisePickerDialog from '@/components/programs/ExercisePickerDialog.vue'
import { Badge } from '@/components/ui/badge'
import { Button } from '@/components/ui/button'
import { Card, CardContent } from '@/components/ui/card'
import { Input } from '@/components/ui/input'
import { Label } from '@/components/ui/label'
import {
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from '@/components/ui/select'
import { Skeleton } from '@/components/ui/skeleton'
import { Switch } from '@/components/ui/switch'
import { Textarea } from '@/components/ui/textarea'
import { useAuthStore } from '@/stores/auth'
import { useExercisesStore } from '@/stores/exercises'
import useProgramsStore from '@/stores/programs'

// Лимиты синхронизированы с бэкендом (internal/programs/mutations.go)
const MAX_DAYS = 50
const MAX_EXERCISES_PER_DAY = 50
const MAX_SETS_PER_EXERCISE = 50
const MIN_WEIGHT_KG = -300 // отрицательный вес = помощь (жим с противовесом)
const MAX_WEIGHT_KG = 500

const FIELD_KEYS = {
  weightKg: 'weight_kg',
  reps: 'reps',
  durationSec: 'duration_sec',
  distanceM: 'distance_m',
  restSec: 'rest_sec',
}

const { t } = useI18n()
const route = useRoute()
const router = useRouter()
const store = useProgramsStore()
const exercisesStore = useExercisesStore()
const auth = useAuthStore()

const isAdmin = computed(() => auth.user?.role === 'admin')

const editingId = computed(() => {
  const id = Number(route.params.id)
  return Number.isInteger(id) && id > 0 ? id : null
})

const loading = ref(false)
const saving = ref(false)

let uidSeq = 0
const uid = () => ++uidSeq

const form = reactive({
  name: '',
  description: '',
  level: 'none',
  authorName: '',
  isPublished: false,
  days: [],
})

function emptySet() {
  return { uid: uid(), reps: '', weightKg: '', durationSec: '', distanceM: '', restSec: '' }
}

function addDay() {
  if (form.days.length >= MAX_DAYS) {
    toast.error(t('programs.tooManyDays'))
    return
  }
  form.days.push({ uid: uid(), name: `${t('programs.dayDefault')} ${form.days.length + 1}`, exercises: [] })
}

function removeDay(index) {
  form.days.splice(index, 1)
}

function moveItem(list, index, offset) {
  const target = index + offset
  if (target < 0 || target >= list.length) return
  ;[list[index], list[target]] = [list[target], list[index]]
}

const pickerOpen = ref(false)
let pickerDayUid = null

function openPicker(day) {
  pickerDayUid = day.uid
  pickerOpen.value = true
}

function onExercisePicked(exercise) {
  const day = form.days.find((d) => d.uid === pickerDayUid)
  if (!day) return
  if (day.exercises.length >= MAX_EXERCISES_PER_DAY) {
    toast.error(t('programs.tooManyExercises'))
    return
  }
  day.exercises.push({
    uid: uid(),
    exerciseId: exercise.id,
    name: exercise.name,
    equipment: exercise.equipment?.name ?? '',
    trackingType: exercise.tracking_type,
    thumbnailUrl: exercise.media?.thumbnail_url ?? exercise.media?.gif_url ?? '',
    restAfterSec: '',
    sets: [emptySet()],
  })
  toast.success(t('programs.exerciseAdded', { name: exercise.name }))
}

function removeExercise(day, index) {
  day.exercises.splice(index, 1)
}

function addSet(exercise) {
  if (exercise.sets.length >= MAX_SETS_PER_EXERCISE) {
    toast.error(t('programs.tooManySets'))
    return
  }
  exercise.sets.push(emptySet())
}

function removeSet(exercise, index) {
  exercise.sets.splice(index, 1)
}

// Набор полей подхода определяется типом записи упражнения каталога
function setFields(trackingType) {
  switch (trackingType) {
    case 'weight_reps':
      return ['weightKg', 'reps', 'restSec']
    case 'reps':
      return ['reps', 'restSec']
    case 'time':
      return ['durationSec', 'restSec']
    case 'distance_time':
      return ['distanceM', 'durationSec', 'restSec']
    default:
      return ['reps', 'restSec']
  }
}

function fieldLabel(field) {
  return t(`programs.fields.${FIELD_KEYS[field]}`)
}

function fieldAttrs(field) {
  if (field === 'weightKg') {
    return { min: MIN_WEIGHT_KG, max: MAX_WEIGHT_KG, step: 0.5 }
  }
  return { min: 0, step: 1 }
}

function intOrNull(value) {
  if (value === '' || value == null) return null
  const n = Number(value)
  return Number.isFinite(n) ? Math.round(n) : null
}

function floatOrNull(value) {
  if (value === '' || value == null) return null
  const n = Number(value)
  return Number.isFinite(n) ? n : null
}

function validate() {
  const name = form.name.trim()
  if (!name || name.length > 255) {
    toast.error(t('programs.nameRequired'))
    return false
  }
  if (form.days.some((day) => {
    const dayName = day.name.trim()
    return !dayName || dayName.length > 255
  })) {
    toast.error(t('programs.dayNameRequired'))
    return false
  }
  for (const day of form.days) {
    for (const exercise of day.exercises) {
      for (const set of exercise.sets) {
        if (set.weightKg !== '' && set.weightKg != null) {
          const weight = Number(set.weightKg)
          if (!Number.isFinite(weight) || weight < MIN_WEIGHT_KG || weight > MAX_WEIGHT_KG) {
            toast.error(t('programs.weightInvalid'))
            return false
          }
        }
      }
    }
  }
  return true
}

function buildBody() {
  return {
    name: form.name.trim(),
    description: form.description.trim() || null,
    author_name: form.authorName.trim() || null,
    level: form.level === 'none' ? null : form.level,
    is_published: isAdmin.value ? form.isPublished : false,
    days: form.days.map((day) => ({
      name: day.name.trim(),
      exercises: day.exercises.map((exercise) => ({
        exercise_id: exercise.exerciseId,
        rest_after_sec: intOrNull(exercise.restAfterSec),
        sets: exercise.sets.map((set, index) => ({
          set_number: index + 1,
          reps: intOrNull(set.reps),
          weight_kg: floatOrNull(set.weightKg),
          duration_sec: intOrNull(set.durationSec),
          distance_m: intOrNull(set.distanceM),
          rest_sec: intOrNull(set.restSec),
        })),
      })),
    })),
  }
}

async function save() {
  if (!validate()) return
  saving.value = true
  const detail = await store.saveProgram(editingId.value, buildBody())
  saving.value = false
  if (detail) {
    toast.success(t(editingId.value ? 'programs.saveSuccess' : 'programs.createSuccess'))
    router.replace({ name: 'program-detail', params: { id: detail.id } })
  }
}

function cancel() {
  if (editingId.value) {
    router.replace({ name: 'program-detail', params: { id: editingId.value } })
  } else {
    router.replace({ name: 'workouts' })
  }
}

async function loadForEdit() {
  loading.value = true
  const detail = await store.loadDetail(editingId.value)
  if (!detail) {
    // 404 или сеть: тост уже показан в сторе, уводим на список
    loading.value = false
    router.replace({ name: 'workouts' })
    return
  }
  if (!detail.can_edit) {
    toast.error(t('programs.noEditRights'))
    router.replace({ name: 'program-detail', params: { id: detail.id } })
    return
  }
  form.name = detail.name
  form.description = detail.description ?? ''
  form.level = detail.level ?? 'none'
  form.authorName = detail.author_name ?? ''
  form.isPublished = detail.is_published
  form.days = detail.days.map((day) => ({
    uid: uid(),
    name: day.name,
    exercises: day.exercises.map((exercise) => ({
      uid: uid(),
      exerciseId: exercise.exercise_id,
      name: exercise.name,
      equipment: exercise.equipment,
      trackingType: exercise.tracking_type,
      thumbnailUrl: exercise.thumbnail_url || exercise.gif_url || '',
      restAfterSec: exercise.rest_after_sec ?? '',
      sets: exercise.sets.map((set) => ({
        uid: uid(),
        reps: set.reps ?? '',
        weightKg: set.weight_kg ?? '',
        durationSec: set.duration_sec ?? '',
        distanceM: set.distance_m ?? '',
        restSec: set.rest_sec ?? '',
      })),
    })),
  }))
  loading.value = false
}

onMounted(() => {
  exercisesStore.loadMeta()
  if (editingId.value) {
    loadForEdit()
  } else {
    addDay()
  }
})
</script>

<template>
  <div class="flex flex-col gap-6">
    <div class="flex flex-wrap items-center justify-between gap-3">
      <h1 class="text-2xl font-bold">
        {{ editingId ? $t('programs.editTitle') : $t('programs.createTitle') }}
      </h1>
      <div class="flex items-center gap-2">
        <Button variant="outline" :disabled="saving" @click="cancel">
          {{ $t('programs.cancel') }}
        </Button>
        <Button :disabled="saving" @click="save">
          {{ saving ? $t('common.loading') : $t('common.save') }}
        </Button>
      </div>
    </div>

    <div v-if="loading" class="flex flex-col gap-4">
      <Skeleton class="h-40 w-full" />
      <Skeleton class="h-64 w-full" />
    </div>

    <template v-else>
      <Card>
        <CardContent class="grid gap-4 sm:grid-cols-2">
          <div class="flex flex-col gap-1.5 sm:col-span-2">
            <Label for="program-name">{{ $t('programs.name') }} *</Label>
            <Input id="program-name" v-model="form.name" maxlength="255" />
          </div>
          <div class="flex flex-col gap-1.5 sm:col-span-2">
            <Label for="program-description">{{ $t('programs.descriptionLabel') }}</Label>
            <Textarea
              id="program-description"
              v-model="form.description"
              :placeholder="$t('programs.descriptionPlaceholder')"
              class="min-h-20"
            />
          </div>
          <div class="flex flex-col gap-1.5">
            <Label>{{ $t('programs.level') }}</Label>
            <Select v-model="form.level">
              <SelectTrigger class="w-full">
                <SelectValue />
              </SelectTrigger>
              <SelectContent>
                <SelectItem value="none">{{ $t('programs.levelNone') }}</SelectItem>
                <SelectItem value="beginner">{{ $t('profile.level_beginner') }}</SelectItem>
                <SelectItem value="intermediate">{{ $t('profile.level_intermediate') }}</SelectItem>
                <SelectItem value="advanced">{{ $t('profile.level_advanced') }}</SelectItem>
              </SelectContent>
            </Select>
          </div>
          <div v-if="isAdmin" class="flex flex-col gap-1.5">
            <Label for="program-author">{{ $t('programs.authorLabel') }}</Label>
            <Input id="program-author" v-model="form.authorName" maxlength="255" />
          </div>
          <div v-if="isAdmin" class="flex items-center gap-2 sm:col-span-2">
            <Switch id="program-published" v-model="form.isPublished" />
            <Label for="program-published">{{ $t('programs.readyToggle') }}</Label>
          </div>
        </CardContent>
      </Card>

      <Card v-for="(day, dayIndex) in form.days" :key="day.uid">
        <CardContent class="flex flex-col gap-3">
          <div class="flex items-center gap-2">
            <Badge variant="secondary">{{ dayIndex + 1 }}</Badge>
            <Input v-model="day.name" maxlength="255" class="h-8 flex-1 font-medium" />
            <Button
              variant="ghost"
              size="icon-sm"
              :disabled="dayIndex === 0"
              :aria-label="$t('programs.moveUp')"
              @click="moveItem(form.days, dayIndex, -1)"
            >
              <ChevronUp />
            </Button>
            <Button
              variant="ghost"
              size="icon-sm"
              :disabled="dayIndex === form.days.length - 1"
              :aria-label="$t('programs.moveDown')"
              @click="moveItem(form.days, dayIndex, 1)"
            >
              <ChevronDown />
            </Button>
            <Button
              variant="ghost"
              size="icon-sm"
              class="text-destructive"
              :aria-label="$t('programs.removeDay')"
              @click="removeDay(dayIndex)"
            >
              <Trash2 />
            </Button>
          </div>

          <div
            v-if="day.exercises.length === 0"
            class="text-muted-foreground rounded-lg border border-dashed py-6 text-center text-sm"
          >
            {{ $t('programs.emptyDay') }}
          </div>

          <div
            v-for="(exercise, exerciseIndex) in day.exercises"
            :key="exercise.uid"
            class="flex flex-col gap-3 rounded-lg border p-3"
          >
            <div class="flex flex-wrap items-center gap-2">
              <div
                class="bg-muted flex size-12 shrink-0 items-center justify-center overflow-hidden rounded-md"
              >
                <img
                  v-if="exercise.thumbnailUrl"
                  :src="exercise.thumbnailUrl"
                  :alt="exercise.name"
                  loading="lazy"
                  decoding="async"
                  class="size-full object-contain"
                  @error="$event.target.style.visibility = 'hidden'"
                />
              </div>
              <div class="flex min-w-0 flex-1 flex-col gap-1">
                <p class="line-clamp-1 text-sm font-medium">{{ exercise.name }}</p>
                <Badge v-if="exercise.equipment" variant="outline" class="w-fit text-xs">
                  {{ exercise.equipment }}
                </Badge>
              </div>
              <div class="flex items-center gap-1.5">
                <span class="text-muted-foreground text-xs">
                  {{ $t('programs.restAfter') }}
                </span>
                <Input
                  :id="`rest-after-${exercise.uid}`"
                  :model-value="exercise.restAfterSec"
                  type="number"
                  min="0"
                  class="h-7 w-20"
                  @update:model-value="exercise.restAfterSec = $event"
                />
              </div>
              <Button
                variant="ghost"
                size="icon-sm"
                :disabled="exerciseIndex === 0"
                :aria-label="$t('programs.moveUp')"
                @click="moveItem(day.exercises, exerciseIndex, -1)"
              >
                <ChevronUp />
              </Button>
              <Button
                variant="ghost"
                size="icon-sm"
                :disabled="exerciseIndex === day.exercises.length - 1"
                :aria-label="$t('programs.moveDown')"
                @click="moveItem(day.exercises, exerciseIndex, 1)"
              >
                <ChevronDown />
              </Button>
              <Button
                variant="ghost"
                size="icon-sm"
                class="text-destructive"
                :aria-label="$t('programs.removeExercise')"
                @click="removeExercise(day, exerciseIndex)"
              >
                <Trash2 />
              </Button>
            </div>

            <div v-if="exercise.sets.length > 0" class="overflow-x-auto">
              <table class="w-full min-w-80">
                <thead>
                  <tr class="text-muted-foreground text-left text-xs">
                    <th class="py-1 pr-2 font-normal">{{ $t('programs.setNumber') }}</th>
                    <th
                      v-for="field in setFields(exercise.trackingType)"
                      :key="field"
                      class="py-1 pr-2 font-normal"
                    >
                      {{ fieldLabel(field) }}
                    </th>
                    <th class="w-8" />
                  </tr>
                </thead>
                <tbody>
                  <tr v-for="(set, setIndex) in exercise.sets" :key="set.uid">
                    <td class="py-1 pr-2 text-sm tabular-nums">{{ setIndex + 1 }}</td>
                    <td
                      v-for="field in setFields(exercise.trackingType)"
                      :key="field"
                      class="py-1 pr-2"
                    >
                      <Input
                        :model-value="set[field]"
                        type="number"
                        v-bind="fieldAttrs(field)"
                        class="h-7 w-24"
                        @update:model-value="set[field] = $event"
                      />
                    </td>
                    <td class="py-1">
                      <Button
                        variant="ghost"
                        size="icon-sm"
                        class="text-destructive"
                        :aria-label="$t('programs.removeSet')"
                        @click="removeSet(exercise, setIndex)"
                      >
                        <Trash2 />
                      </Button>
                    </td>
                  </tr>
                </tbody>
              </table>
            </div>

            <Button
              variant="outline"
              size="sm"
              class="w-fit"
              @click="addSet(exercise)"
            >
              <Plus />
              {{ $t('programs.addSet') }}
            </Button>
          </div>

          <Button variant="outline" size="sm" class="w-fit" @click="openPicker(day)">
            <Plus />
            {{ $t('programs.addExercise') }}
          </Button>
        </CardContent>
      </Card>

      <Button variant="outline" @click="addDay">
        <Plus />
        {{ $t('programs.addDay') }}
      </Button>

      <div class="flex justify-end gap-2">
        <Button variant="outline" :disabled="saving" @click="cancel">
          {{ $t('programs.cancel') }}
        </Button>
        <Button :disabled="saving" @click="save">
          {{ saving ? $t('common.loading') : $t('common.save') }}
        </Button>
      </div>
    </template>

    <ExercisePickerDialog v-model:open="pickerOpen" @select="onExercisePicked" />
  </div>
</template>
