<script setup>
import { computed, onMounted, watch } from 'vue'
import { storeToRefs } from 'pinia'
import { Plus, RotateCcw, Search } from '@lucide/vue'
import { useRoute, useRouter } from 'vue-router'

import ProgramCard from '@/components/programs/ProgramCard.vue'
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
import { Skeleton } from '@/components/ui/skeleton'
import useProgramsStore, { PER_PAGE } from '@/stores/programs'

const route = useRoute()
const router = useRouter()
const store = useProgramsStore()

const { q, sort, mine, ready, hasActiveFilters, appliedRevision } = storeToRefs(store)

const sections = computed(() => [
  {
    scope: 'mine',
    titleKey: 'programs.mineTitle',
    descriptionKey: 'programs.mineDescription',
    data: mine.value,
    showSkeleton: store.mineShowSkeleton,
  },
  {
    scope: 'ready',
    titleKey: 'programs.readyTitle',
    descriptionKey: 'programs.readyDescription',
    data: ready.value,
    showSkeleton: store.readyShowSkeleton,
  },
])

function openProgram(program) {
  router.push({ name: 'program-detail', params: { id: program.id } })
}

function createProgram() {
  router.push({ name: 'program-create' })
}

// URL отражает применённые фильтры: onFiltersChanged инкрементит appliedRevision
watch(appliedRevision, () => {
  const query = {}
  if (store.qDebounced.trim()) query.q = store.qDebounced.trim()
  if (store.sort !== 'newest') query.sort = store.sort
  router.replace({ query })
})

watch(sort, () => store.onFiltersChanged())

onMounted(() => {
  store.initFromUrl(route.query)
  store.loadList('mine')
  store.loadList('ready')
})
</script>

<template>
  <div class="flex flex-col gap-6">
    <div class="flex flex-wrap items-start justify-between gap-3">
      <div class="flex flex-col gap-1">
        <h1 class="text-3xl font-bold">{{ $t('nav.workouts') }}</h1>
        <p class="text-muted-foreground text-sm">{{ $t('programs.description') }}</p>
      </div>
      <Button @click="createProgram">
        <Plus />
        {{ $t('programs.create') }}
      </Button>
    </div>

    <div class="flex flex-wrap items-center gap-2">
      <div class="relative min-w-52 flex-1">
        <Search
          class="text-muted-foreground pointer-events-none absolute top-1/2 left-2.5 size-4 -translate-y-1/2"
        />
        <Input
          :model-value="q"
          :placeholder="$t('programs.searchPlaceholder')"
          class="pl-8"
          @update:model-value="store.setQ"
        />
      </div>
      <Select v-model="sort">
        <SelectTrigger class="w-44">
          <SelectValue :placeholder="$t('programs.sort')" />
        </SelectTrigger>
        <SelectContent>
          <SelectItem value="newest">{{ $t('programs.sort_newest') }}</SelectItem>
          <SelectItem value="name">{{ $t('programs.sort_name') }}</SelectItem>
        </SelectContent>
      </Select>
      <Button
        v-if="hasActiveFilters"
        variant="ghost"
        size="sm"
        class="text-muted-foreground"
        @click="store.resetFilters"
      >
        <RotateCcw />
        {{ $t('programs.resetFilters') }}
      </Button>
    </div>

    <section v-for="s in sections" :key="s.scope" class="flex flex-col gap-3">
      <div class="flex items-center gap-2">
        <h2 class="text-xl font-semibold">{{ $t(s.titleKey) }}</h2>
        <Badge v-if="s.data.total > 0 && !s.showSkeleton" variant="secondary">
          {{ s.data.total }}
        </Badge>
      </div>
      <p class="text-muted-foreground text-sm">{{ $t(s.descriptionKey) }}</p>

      <div v-if="s.showSkeleton" class="grid grid-cols-1 gap-3 sm:grid-cols-2 xl:grid-cols-3">
        <div v-for="i in 3" :key="i" class="flex flex-col gap-2 rounded-lg border p-4">
          <Skeleton class="h-5 w-3/4" />
          <Skeleton class="h-4 w-full" />
          <Skeleton class="h-4 w-1/2" />
          <Skeleton class="h-4 w-1/3" />
        </div>
      </div>

      <div
        v-else-if="s.data.items.length === 0"
        class="flex flex-col items-center gap-2 rounded-lg border border-dashed py-12"
      >
        <template v-if="hasActiveFilters">
          <p class="font-medium">{{ $t('programs.notFoundTitle') }}</p>
          <p class="text-muted-foreground text-sm">{{ $t('programs.notFoundHint') }}</p>
          <Button variant="outline" size="sm" class="mt-2" @click="store.resetFilters">
            <RotateCcw />
            {{ $t('programs.resetFilters') }}
          </Button>
        </template>
        <template v-else-if="s.scope === 'mine'">
          <p class="font-medium">{{ $t('programs.mineEmptyTitle') }}</p>
          <p class="text-muted-foreground text-sm">{{ $t('programs.mineEmptyHint') }}</p>
          <Button size="sm" class="mt-2" @click="createProgram">
            <Plus />
            {{ $t('programs.create') }}
          </Button>
        </template>
        <template v-else>
          <p class="font-medium">{{ $t('programs.readyEmptyTitle') }}</p>
          <p class="text-muted-foreground text-sm">{{ $t('programs.readyEmptyHint') }}</p>
        </template>
      </div>

      <template v-else>
        <div class="grid grid-cols-1 gap-3 sm:grid-cols-2 xl:grid-cols-3">
          <ProgramCard
            v-for="program in s.data.items"
            :key="program.id"
            :program="program"
            @open="openProgram"
          />
        </div>

        <Pagination
          v-if="s.data.pages > 1"
          v-slot="{ page: currentPage }"
          :page="s.data.page"
          :items-per-page="PER_PAGE"
          :total="s.data.total"
          :sibling-count="1"
          show-edges
          class="justify-center"
          @update:page="store.changePage(s.scope, $event)"
        >
          <PaginationContent v-slot="{ items: pageItems }">
            <PaginationPrevious>
              <span class="hidden sm:block">{{ $t('programs.prevPage') }}</span>
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
              <span class="hidden sm:block">{{ $t('programs.nextPage') }}</span>
            </PaginationNext>
          </PaginationContent>
        </Pagination>
      </template>
    </section>
  </div>
</template>
