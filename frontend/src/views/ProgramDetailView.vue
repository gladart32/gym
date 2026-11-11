<script setup>
import { computed, onMounted, ref, watch } from 'vue'
import { storeToRefs } from 'pinia'
import { ArrowLeft, Copy, Pencil, Trash2 } from '@lucide/vue'
import { useI18n } from 'vue-i18n'
import { useRoute, useRouter } from 'vue-router'
import { toast } from 'vue-sonner'

import { Badge } from '@/components/ui/badge'
import { Button } from '@/components/ui/button'
import {
  Card,
  CardContent,
  CardDescription,
  CardHeader,
  CardTitle,
} from '@/components/ui/card'
import {
  Dialog,
  DialogContent,
  DialogDescription,
  DialogFooter,
  DialogHeader,
  DialogTitle,
} from '@/components/ui/dialog'
import { Skeleton } from '@/components/ui/skeleton'
import { useExercisesStore } from '@/stores/exercises'
import useProgramsStore from '@/stores/programs'

const { t } = useI18n()
const route = useRoute()
const router = useRouter()
const store = useProgramsStore()
const exercisesStore = useExercisesStore()

const { trackingTypeNames } = storeToRefs(exercisesStore)

const detail = ref(null)
const loading = ref(true)
const deleteOpen = ref(false)
const deleting = ref(false)

const programId = computed(() => Number(route.params.id))

const canEdit = computed(() => detail.value?.can_edit ?? false)
const canCopy = computed(() => detail.value?.is_ready && !detail.value?.can_edit)

// Набор колонок таблицы подходов зависит от типа записи упражнения каталога
function columnsFor(trackingType) {
  switch (trackingType) {
    case 'weight_reps':
      return ['weight_kg', 'reps', 'rest_sec']
    case 'reps':
      return ['reps', 'rest_sec']
    case 'time':
      return ['duration_sec', 'rest_sec']
    case 'distance_time':
      return ['distance_m', 'duration_sec', 'rest_sec']
    default:
      return ['reps', 'rest_sec']
  }
}

function columnLabel(field) {
  return t(`programs.fields.${field}`)
}

function formatValue(set, field) {
  const value = set[field]
  if (value == null) return '—'
  if (field === 'rest_sec') return t('programs.restSeconds', value)
  if (field === 'duration_sec') return t('programs.durationSeconds', value)
  return value
}

function exerciseColumns(trackingType) {
  return columnsFor(trackingType)
}

async function copyProgram() {
  const copied = await store.copyProgram(programId.value)
  if (copied) {
    toast.success(t('programs.copySuccess'))
    router.replace({ name: 'program-detail', params: { id: copied.id } })
  }
}

async function removeProgram() {
  deleting.value = true
  const ok = await store.removeProgram(programId.value)
  deleting.value = false
  deleteOpen.value = false
  if (ok) {
    toast.success(t('programs.deleteSuccess'))
    router.replace({ name: 'workouts' })
  }
}

onMounted(async () => {
  // имена типов записи берём из словаря каталога (кэшируется)
  exercisesStore.loadMeta()
  detail.value = await store.loadDetail(programId.value)
  loading.value = false
})

// Роут-рекорд переиспользуется при смене :id (например, после «Копировать
// себе» → replace на копию) — перезагружаем дерево тренировки
watch(programId, async (id, prev) => {
  if (id === prev) return
  loading.value = true
  deleteOpen.value = false
  detail.value = await store.loadDetail(id)
  loading.value = false
})
</script>

<template>
  <div class="flex flex-col gap-6">
    <div class="flex items-center justify-between gap-3">
      <Button variant="ghost" size="sm" class="text-muted-foreground" @click="router.back()">
        <ArrowLeft />
        {{ $t('programs.backToList') }}
      </Button>
      <div v-if="detail" class="flex flex-wrap items-center gap-2">
        <Button v-if="canEdit" variant="outline" @click="router.push({ name: 'program-edit', params: { id: detail.id } })">
          <Pencil />
          {{ $t('programs.edit') }}
        </Button>
        <Button v-if="canCopy" variant="outline" @click="copyProgram">
          <Copy />
          {{ $t('programs.copyToSelf') }}
        </Button>
        <Button v-if="canEdit" variant="destructive" @click="deleteOpen = true">
          <Trash2 />
          {{ $t('programs.delete') }}
        </Button>
      </div>
    </div>

    <div v-if="loading" class="flex flex-col gap-4">
      <Skeleton class="h-8 w-1/2" />
      <Skeleton class="h-4 w-1/3" />
      <Skeleton class="h-40 w-full" />
      <Skeleton class="h-64 w-full" />
    </div>

    <div
      v-else-if="!detail"
      class="flex flex-col items-center gap-2 rounded-lg border border-dashed py-16"
    >
      <p class="font-medium">{{ $t('programs.notFoundTitle') }}</p>
      <p class="text-muted-foreground text-sm">{{ $t('programs.notFoundHint') }}</p>
      <Button variant="outline" size="sm" class="mt-2" @click="router.push({ name: 'workouts' })">
        {{ $t('programs.backToList') }}
      </Button>
    </div>

    <template v-else>
      <div class="flex flex-col gap-2">
        <div class="flex flex-wrap items-center gap-2">
          <h1 class="text-2xl font-bold">{{ detail.name }}</h1>
          <Badge v-if="detail.is_ready && !detail.is_published" variant="secondary">
            {{ $t('programs.draft') }}
          </Badge>
          <Badge v-if="detail.copied_from_id" variant="outline">
            {{ $t('programs.copyBadge') }}
          </Badge>
        </div>
        <p v-if="detail.description" class="text-muted-foreground text-sm whitespace-pre-line">
          {{ detail.description }}
        </p>
        <div class="flex flex-wrap items-center gap-1">
          <Badge v-if="detail.level" variant="outline">
            {{ $t(`profile.level_${detail.level}`) }}
          </Badge>
          <span v-if="detail.author_name" class="text-muted-foreground text-sm">
            {{ detail.author_name }}
          </span>
        </div>
      </div>

      <Card v-for="day in detail.days" :key="day.id">
        <CardHeader>
          <CardTitle>{{ day.name }}</CardTitle>
          <CardDescription>
            {{ $t('programs.exercisesCount', day.exercises.length) }}
          </CardDescription>
        </CardHeader>
        <CardContent class="flex flex-col gap-3">
          <p
            v-if="day.exercises.length === 0"
            class="text-muted-foreground rounded-lg border border-dashed py-8 text-center text-sm"
          >
            {{ $t('programs.emptyDay') }}
          </p>

          <div
            v-for="exercise in day.exercises"
            :key="exercise.id"
            class="flex flex-col gap-3 rounded-lg border p-3 sm:flex-row sm:items-start"
          >
            <div
              class="bg-muted flex aspect-square size-24 shrink-0 items-center justify-center overflow-hidden rounded-md sm:size-40"
            >
              <img
                v-if="exercise.gif_url || exercise.thumbnail_url"
                :src="exercise.gif_url || exercise.thumbnail_url"
                :alt="exercise.name"
                loading="lazy"
                decoding="async"
                class="size-full object-contain"
                @error="$event.target.style.display = 'none'"
              />
            </div>

            <div class="flex min-w-0 flex-1 flex-col gap-2">
              <div class="flex flex-wrap items-center gap-1">
                <p class="text-sm font-medium">{{ exercise.name }}</p>
                <Badge variant="outline">{{ exercise.equipment }}</Badge>
                <Badge v-if="trackingTypeNames[exercise.tracking_type]" variant="secondary">
                  {{ trackingTypeNames[exercise.tracking_type] }}
                </Badge>
              </div>

              <div class="overflow-x-auto">
                <table class="w-full min-w-64 text-sm">
                  <thead>
                    <tr class="text-muted-foreground border-b text-left text-xs">
                      <th class="py-1.5 pr-3 font-normal">{{ $t('programs.setNumber') }}</th>
                      <th
                        v-for="field in exerciseColumns(exercise.tracking_type)"
                        :key="field"
                        class="py-1.5 pr-3 font-normal"
                      >
                        {{ columnLabel(field) }}
                      </th>
                    </tr>
                  </thead>
                  <tbody>
                    <tr v-for="set in exercise.sets" :key="set.id" class="border-b last:border-0">
                      <td class="py-1.5 pr-3 tabular-nums">{{ set.set_number }}</td>
                      <td
                        v-for="field in exerciseColumns(exercise.tracking_type)"
                        :key="field"
                        class="py-1.5 pr-3 tabular-nums"
                      >
                        {{ formatValue(set, field) }}
                      </td>
                    </tr>
                    <tr v-if="exercise.sets.length === 0">
                      <td :colspan="exerciseColumns(exercise.tracking_type).length + 1" class="text-muted-foreground py-2 text-sm">
                        {{ $t('programs.noSets') }}
                      </td>
                    </tr>
                  </tbody>
                </table>
              </div>
            </div>
          </div>
        </CardContent>
      </Card>

      <Dialog v-model:open="deleteOpen">
        <DialogContent class="sm:max-w-md">
          <DialogHeader>
            <DialogTitle>{{ $t('programs.deleteConfirmTitle') }}</DialogTitle>
            <DialogDescription>{{ $t('programs.deleteConfirmHint') }}</DialogDescription>
          </DialogHeader>
          <DialogFooter>
            <Button variant="outline" @click="deleteOpen = false">
              {{ $t('programs.cancel') }}
            </Button>
            <Button variant="destructive" :disabled="deleting" @click="removeProgram">
              {{ $t('programs.delete') }}
            </Button>
          </DialogFooter>
        </DialogContent>
      </Dialog>
    </template>
  </div>
</template>
