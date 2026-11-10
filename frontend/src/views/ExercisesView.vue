<script setup>
import { computed, onMounted, ref, watch } from 'vue'
import { RotateCcw, Search, SlidersHorizontal } from '@lucide/vue'
import { refDebounced } from '@vueuse/core'
import { useRoute, useRouter } from 'vue-router'
import { toast } from 'vue-sonner'

import { exercisesApi } from '@/api/exercises'
import { profileApi } from '@/api/profile'
import ExerciseCard from '@/components/exercises/ExerciseCard.vue'
import ExerciseDetailDialog from '@/components/exercises/ExerciseDetailDialog.vue'
import ExerciseFiltersPanel from '@/components/exercises/ExerciseFiltersPanel.vue'
import { Badge } from '@/components/ui/badge'
import { Button } from '@/components/ui/button'
import { Input } from '@/components/ui/input'
import {
  Pagination,
  PaginationContent,
  PaginationEllipsis,
  PaginationItem,
  PaginationNext,
  PaginationPrevious,
} from '@/components/ui/pagination'
import {
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from '@/components/ui/select'
import {
  Sheet,
  SheetContent,
  SheetDescription,
  SheetHeader,
  SheetTitle,
} from '@/components/ui/sheet'
import { Skeleton } from '@/components/ui/skeleton'

const PER_PAGE = 20

const route = useRoute()
const router = useRouter()

const q = ref('')
const qDebounced = refDebounced(q, 400)
const sort = ref('name')
const page = ref(1)
const bodyPartIds = ref([])
const equipmentIds = ref([])
const muscleIds = ref([])
const trackingTypes = ref([])
const myEquipment = ref(false)

const dictionaries = ref(null)
const hasUserEquipment = ref(false)
const trackingTypeNames = ref({})

const items = ref([])
const total = ref(0)
const pages = ref(0)
const loading = ref(false)
const metaLoading = ref(true)

const mobileFiltersOpen = ref(false)
const detailOpen = ref(false)
const detailExerciseId = ref(null)

const activeFilterCount = computed(
  () =>
    bodyPartIds.value.length +
    equipmentIds.value.length +
    muscleIds.value.length +
    trackingTypes.value.length +
    (myEquipment.value ? 1 : 0),
)

const hasActiveFilters = computed(() => activeFilterCount.value > 0 || q.value.trim() !== '')

const rangeFrom = computed(() => (total.value === 0 ? 0 : (page.value - 1) * PER_PAGE + 1))
const rangeTo = computed(() => Math.min(page.value * PER_PAGE, total.value))

const showSkeleton = computed(
  () => metaLoading.value || (loading.value && items.value.length === 0),
)

let requestSeq = 0
let lastParamsKey = ''

function listParams(instant) {
  const search = (instant ? q.value : qDebounced.value).trim()
  const params = { page: page.value, per_page: PER_PAGE, sort: sort.value }
  if (search) params.q = search
  if (bodyPartIds.value.length) params.body_part_ids = bodyPartIds.value.join(',')
  if (equipmentIds.value.length) params.equipment_ids = equipmentIds.value.join(',')
  if (muscleIds.value.length) params.muscle_ids = muscleIds.value.join(',')
  if (trackingTypes.value.length) params.tracking_types = trackingTypes.value.join(',')
  if (myEquipment.value) params.my_equipment = 1
  return params
}

async function loadList({ instant = false } = {}) {
  const params = listParams(instant)
  const key = JSON.stringify(params)
  if (key === lastParamsKey) return
  lastParamsKey = key

  const seq = ++requestSeq
  loading.value = true
  try {
    const data = await exercisesApi.list(params)
    if (seq !== requestSeq) return
    items.value = data.items ?? []
    total.value = data.total ?? 0
    pages.value = data.pages ?? 0
    if (page.value > pages.value && pages.value > 0) {
      page.value = pages.value
      return
    }
  } catch (error) {
    if (seq !== requestSeq) return
    lastParamsKey = ''
    toast.error(error.message)
  } finally {
    if (seq === requestSeq) loading.value = false
  }
}

function syncUrl() {
  const query = {}
  if (qDebounced.value.trim()) query.q = qDebounced.value.trim()
  if (bodyPartIds.value.length) query.bp = bodyPartIds.value.join(',')
  if (equipmentIds.value.length) query.eq = equipmentIds.value.join(',')
  if (muscleIds.value.length) query.mu = muscleIds.value.join(',')
  if (trackingTypes.value.length) query.tt = trackingTypes.value.join(',')
  if (myEquipment.value) query.my = '1'
  if (sort.value !== 'name') query.sort = sort.value
  if (page.value !== 1) query.page = String(page.value)
  router.replace({ query })
}

function resetFilters() {
  q.value = ''
  bodyPartIds.value = []
  equipmentIds.value = []
  muscleIds.value = []
  trackingTypes.value = []
  myEquipment.value = false
  sort.value = 'name'
  page.value = 1
}

function onFiltersChanged() {
  if (page.value === 1) {
    loadList()
  } else {
    page.value = 1
  }
}

initFromUrl()

watch([qDebounced, bodyPartIds, equipmentIds, muscleIds, trackingTypes, myEquipment, sort], () => {
  onFiltersChanged()
})

watch(page, () => loadList())

watch(
  [qDebounced, bodyPartIds, equipmentIds, muscleIds, trackingTypes, myEquipment, sort, page],
  syncUrl,
)

function parseIdsParam(value) {
  if (!value) return []
  return String(value)
    .split(',')
    .map(Number)
    .filter((n) => Number.isInteger(n) && n > 0)
}

function initFromUrl() {
  const query = route.query
  if (typeof query.q === 'string') q.value = query.q
  if (query.sort === 'newest' || query.sort === 'name') sort.value = query.sort
  const p = Number(query.page)
  if (Number.isInteger(p) && p > 0) page.value = p
  bodyPartIds.value = parseIdsParam(query.bp)
  equipmentIds.value = parseIdsParam(query.eq)
  muscleIds.value = parseIdsParam(query.mu)
  if (typeof query.tt === 'string') {
    trackingTypes.value = query.tt.split(',').filter((v) => v)
  }
  myEquipment.value = query.my === '1'
}

async function loadMeta() {
  metaLoading.value = true
  try {
    const [filtersData, equipmentData] = await Promise.all([
      exercisesApi.filters(),
      profileApi.equipment(),
    ])
    dictionaries.value = filtersData
    trackingTypeNames.value = Object.fromEntries(
      (filtersData.tracking_types ?? []).map((tt) => [tt.value, tt.name]),
    )
    hasUserEquipment.value = (equipmentData.selected_ids ?? []).length > 0
  } catch (error) {
    toast.error(error.message)
  } finally {
    metaLoading.value = false
  }
}

function openExercise(exercise) {
  detailExerciseId.value = exercise.id
  detailOpen.value = true
}

onMounted(() => {
  loadMeta()
  loadList({ instant: true })
})
</script>

<template>
  <div class="flex flex-col gap-6">
    <h1 class="text-3xl font-bold">{{ $t('nav.exercises') }}</h1>

    <div class="flex flex-col gap-3">
      <div class="flex flex-wrap items-center gap-2">
        <div class="relative min-w-52 flex-1">
          <Search
            class="text-muted-foreground pointer-events-none absolute top-1/2 left-2.5 size-4 -translate-y-1/2"
          />
          <Input v-model="q" :placeholder="$t('exercises.searchPlaceholder')" class="pl-8" />
        </div>
        <Select v-model="sort">
          <SelectTrigger class="w-44">
            <SelectValue :placeholder="$t('exercises.sort')" />
          </SelectTrigger>
          <SelectContent>
            <SelectItem value="name">{{ $t('exercises.sort_name') }}</SelectItem>
            <SelectItem value="newest">{{ $t('exercises.sort_newest') }}</SelectItem>
          </SelectContent>
        </Select>
        <Button
          variant="outline"
          class="lg:hidden"
          :aria-label="$t('exercises.filters')"
          @click="mobileFiltersOpen = true"
        >
          <SlidersHorizontal />
          {{ $t('exercises.filters') }}
          <Badge v-if="activeFilterCount > 0" variant="secondary" class="ml-1">
            {{ activeFilterCount }}
          </Badge>
        </Button>
      </div>

      <div class="flex min-h-8 items-center justify-between gap-2">
        <p v-if="total > 0" class="text-muted-foreground text-sm">
          {{ $t('exercises.showingRange', { from: rangeFrom, to: rangeTo, total }) }}
        </p>
        <p v-else class="text-muted-foreground text-sm">&nbsp;</p>
        <Button
          v-if="hasActiveFilters"
          variant="ghost"
          size="sm"
          class="text-muted-foreground"
          @click="resetFilters"
        >
          <RotateCcw />
          {{ $t('exercises.resetFilters') }}
        </Button>
      </div>
    </div>

    <div class="flex items-start gap-6">
      <aside class="top-4 hidden w-64 shrink-0 rounded-lg border p-4 lg:sticky lg:block">
        <div v-if="!dictionaries" class="flex flex-col gap-3">
          <Skeleton class="h-4 w-1/2" />
          <Skeleton class="h-4 w-full" />
          <Skeleton class="h-4 w-3/4" />
          <Skeleton class="h-4 w-2/3" />
        </div>
        <ExerciseFiltersPanel
          v-else
          v-model:body-part-ids="bodyPartIds"
          v-model:equipment-ids="equipmentIds"
          v-model:muscle-ids="muscleIds"
          v-model:tracking-types="trackingTypes"
          v-model:my-equipment="myEquipment"
          :dictionaries="dictionaries"
          :show-my-equipment="hasUserEquipment"
        />
      </aside>

      <div class="flex min-w-0 flex-1 flex-col gap-4">
        <div v-if="showSkeleton" class="grid grid-cols-2 gap-3 sm:grid-cols-3 xl:grid-cols-4">
          <div v-for="i in 8" :key="i" class="flex flex-col gap-2 rounded-lg border p-3">
            <Skeleton class="aspect-square w-full rounded-md" />
            <Skeleton class="h-4 w-3/4" />
            <Skeleton class="h-4 w-1/2" />
          </div>
        </div>

        <div
          v-else-if="items.length === 0"
          class="flex flex-col items-center gap-2 rounded-lg border border-dashed py-16"
        >
          <p class="font-medium">{{ $t('exercises.notFoundTitle') }}</p>
          <p class="text-muted-foreground text-sm">{{ $t('exercises.notFoundHint') }}</p>
          <Button
            v-if="hasActiveFilters"
            variant="outline"
            size="sm"
            class="mt-2"
            @click="resetFilters"
          >
            <RotateCcw />
            {{ $t('exercises.resetFilters') }}
          </Button>
        </div>

        <template v-else>
          <div
            class="grid grid-cols-2 gap-3 transition-opacity sm:grid-cols-3 xl:grid-cols-4"
            :class="{ 'pointer-events-none opacity-60': loading }"
          >
            <ExerciseCard
              v-for="exercise in items"
              :key="exercise.id"
              :exercise="exercise"
              @open="openExercise"
            />
          </div>

          <Pagination
            v-if="pages > 1"
            v-slot="{ page: currentPage }"
            :page="page"
            :items-per-page="PER_PAGE"
            :total="total"
            :sibling-count="1"
            show-edges
            class="justify-center"
            @update:page="(value) => (page = value)"
          >
            <PaginationContent v-slot="{ items: pageItems }">
              <PaginationPrevious>
                <span class="hidden sm:block">{{ $t('exercises.prevPage') }}</span>
              </PaginationPrevious>
              <template v-for="(item, index) in pageItems" :key="index">
                <PaginationItem
                  v-if="item.type === 'page'"
                  :value="item.value"
                  :is-active="item.value === currentPage"
                >
                  {{ item.value }}
                </PaginationItem>
                <PaginationEllipsis v-else :index="index" />
              </template>
              <PaginationNext>
                <span class="hidden sm:block">{{ $t('exercises.nextPage') }}</span>
              </PaginationNext>
            </PaginationContent>
          </Pagination>
        </template>
      </div>
    </div>

    <Sheet v-model:open="mobileFiltersOpen">
      <SheetContent side="left" class="flex w-80 flex-col gap-4 overflow-y-auto p-4">
        <SheetHeader class="p-0 text-left">
          <SheetTitle>{{ $t('exercises.filters') }}</SheetTitle>
          <SheetDescription>{{ $t('exercises.description') }}</SheetDescription>
        </SheetHeader>
        <div v-if="!dictionaries" class="flex flex-col gap-3">
          <Skeleton class="h-4 w-1/2" />
          <Skeleton class="h-4 w-full" />
          <Skeleton class="h-4 w-3/4" />
        </div>
        <ExerciseFiltersPanel
          v-else
          v-model:body-part-ids="bodyPartIds"
          v-model:equipment-ids="equipmentIds"
          v-model:muscle-ids="muscleIds"
          v-model:tracking-types="trackingTypes"
          v-model:my-equipment="myEquipment"
          :dictionaries="dictionaries"
          :show-my-equipment="hasUserEquipment"
        />
        <Button
          v-if="hasActiveFilters"
          variant="outline"
          size="sm"
          class="w-fit"
          @click="resetFilters"
        >
          <RotateCcw />
          {{ $t('exercises.resetFilters') }}
        </Button>
      </SheetContent>
    </Sheet>

    <ExerciseDetailDialog
      v-model:open="detailOpen"
      :exercise-id="detailExerciseId"
      :tracking-type-names="trackingTypeNames"
    />
  </div>
</template>
