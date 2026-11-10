<script setup>
import { onMounted, onUnmounted, ref, watch } from 'vue'
import { storeToRefs } from 'pinia'
import { RotateCcw, Search, SlidersHorizontal } from '@lucide/vue'
import { useRoute, useRouter } from 'vue-router'

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
import { PER_PAGE, useExercisesStore } from '@/stores/exercises'

const route = useRoute()
const router = useRouter()
const store = useExercisesStore()

const {
  q,
  qDebounced,
  sort,
  page,
  bodyPartIds,
  equipmentIds,
  muscleIds,
  trackingTypes,
  myEquipment,
  favoritesOnly,
  dictionaries,
  hasUserEquipment,
  trackingTypeNames,
  items,
  total,
  pages,
  loading,
} = storeToRefs(store)

const { activeFilterCount, hasActiveFilters, appliedRevision, rangeFrom, rangeTo, showSkeleton } =
  storeToRefs(store)

const mobileFiltersOpen = ref(false)
const detailOpen = ref(false)
const detailExerciseId = ref(null)

function syncUrl() {
  const query = {}
  if (qDebounced.value.trim()) query.q = qDebounced.value.trim()
  if (bodyPartIds.value.length) query.bp = bodyPartIds.value.join(',')
  if (equipmentIds.value.length) query.eq = equipmentIds.value.join(',')
  if (muscleIds.value.length) query.mu = muscleIds.value.join(',')
  if (trackingTypes.value.length) query.tt = trackingTypes.value.join(',')
  if (myEquipment.value) query.my = '1'
  if (favoritesOnly.value) query.fav = '1'
  if (sort.value !== 'name') query.sort = sort.value
  if (page.value !== 1) query.page = String(page.value)
  router.replace({ query })
}

function onPageSelected(value) {
  page.value = value
  window.scrollTo({ top: 0, behavior: 'smooth' })
}

function openExercise(exercise) {
  detailExerciseId.value = exercise.id
  detailOpen.value = true
}

// Поиск (q/qDebounced) не в watcher: debounce и загрузку запускает store.setQ
// Сортировка — одиночное осознанное действие, применяется мгновенно
// (applyFiltersNow также «догоняет» отложенные чекбоксы и гасит пилюлю)
watch(sort, () => store.applyFiltersNow())

watch(
  [bodyPartIds, equipmentIds, muscleIds, trackingTypes, myEquipment, favoritesOnly],
  () => {
    store.scheduleFiltersApply()
  },
)

watch(page, () => {
  if (store.pendingFilters) {
    store.applyFiltersNow()
  } else {
    store.loadList()
  }
})

// URL отражает применённые фильтры: onFiltersChanged инкрементит appliedRevision
watch([page, appliedRevision], syncUrl)

onMounted(() => {
  store.initFromUrl(route.query)
  store.loadMeta()
  store.loadList({ instant: true })
})

onUnmounted(() => {
  store.cancelPendingApply()
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
          <Input
            :model-value="q"
            :placeholder="$t('exercises.searchPlaceholder')"
            class="pl-8"
            @update:model-value="store.setQ"
          />
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
          @click="store.resetFilters"
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
          v-model:favorites-only="favoritesOnly"
          :dictionaries="dictionaries"
          :show-my-equipment="hasUserEquipment"
          :has-active-filters="hasActiveFilters"
          @reset="store.resetFilters"
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
            @click="store.resetFilters"
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
              @toggle-favorite="store.toggleFavorite"
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
            @update:page="onPageSelected"
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
          v-model:favorites-only="favoritesOnly"
          :dictionaries="dictionaries"
          :show-my-equipment="hasUserEquipment"
          :has-active-filters="hasActiveFilters"
          @reset="store.resetFilters"
        />
      </SheetContent>
    </Sheet>

    <ExerciseDetailDialog
      v-model:open="detailOpen"
      :exercise-id="detailExerciseId"
      :tracking-type-names="trackingTypeNames"
    />

    <!-- Teleport в body: предки main создают stacking context, из-за которого
         пилюля пряталась под мобильным Sheet'ом -->
    <Teleport to="body">
      <Transition
        enter-active-class="transition duration-200 ease-out"
        enter-from-class="translate-y-2 opacity-0"
        enter-to-class="translate-y-0 opacity-100"
        leave-active-class="transition duration-150 ease-in"
        leave-from-class="opacity-100"
        leave-to-class="opacity-0"
      >
        <div
          v-if="store.pendingFilters"
          class="bg-background fixed bottom-4 left-1/2 z-[60] flex -translate-x-1/2 items-center gap-2.5 rounded-full border py-1.5 pr-1.5 pl-4 text-sm shadow-md"
        >
          <span class="text-muted-foreground whitespace-nowrap">
            {{ $t('exercises.pendingFilters') }}
          </span>
          <span
            class="bg-muted text-foreground flex size-6 items-center justify-center rounded-full text-xs font-medium tabular-nums"
          >
            {{ store.applyCountdown }}
          </span>
          <Button size="sm" class="h-7 rounded-full px-3" @click="store.applyFiltersNow">
            {{ $t('exercises.applyFilters') }}
          </Button>
        </div>
      </Transition>
    </Teleport>
  </div>
</template>
