<script setup>
import { computed, ref, watch } from 'vue'
import { storeToRefs } from 'pinia'
import { Info, Plus, RotateCcw, Search } from '@lucide/vue'
import { toast } from 'vue-sonner'

import { exercisesApi } from '@/api/exercises'
import MultiSelectFilter from '@/components/programs/MultiSelectFilter.vue'
import { useExercisesStore } from '@/stores/exercises'
import { Badge } from '@/components/ui/badge'
import { Button } from '@/components/ui/button'
import {
  Dialog,
  DialogContent,
  DialogDescription,
  DialogHeader,
  DialogTitle,
} from '@/components/ui/dialog'
import { Input } from '@/components/ui/input'
import { Label } from '@/components/ui/label'
import { Skeleton } from '@/components/ui/skeleton'
import { Switch } from '@/components/ui/switch'
import { Tooltip, TooltipContent, TooltipProvider, TooltipTrigger } from '@/components/ui/tooltip'

const open = defineModel('open', { type: Boolean, default: false })

const emit = defineEmits(['select'])

const exercisesStore = useExercisesStore()
const { dictionaries, hasUserEquipment } = storeToRefs(exercisesStore)

const q = ref('')
const bodyPartIds = ref([])
const equipmentIds = ref([])
const muscleIds = ref([])
const trackingTypes = ref([])
const favoritesOnly = ref(false)
const myEquipment = ref(false)

const items = ref([])
const total = ref(0)
const loading = ref(false)

// Значение tracking type — строка; справочник приходит как [{ value, name }]
const trackingTypeItems = computed(() =>
  (dictionaries.value?.tracking_types ?? []).map((tt) => ({ id: tt.value, name: tt.name })),
)

const hasActiveFilters = computed(
  () =>
    q.value.trim() !== '' ||
    bodyPartIds.value.length > 0 ||
    equipmentIds.value.length > 0 ||
    muscleIds.value.length > 0 ||
    trackingTypes.value.length > 0 ||
    favoritesOnly.value ||
    myEquipment.value,
)

let qTimer = null
let searchTimer = null
let requestSeq = 0

async function search() {
  const seq = ++requestSeq
  loading.value = true
  try {
    const params = { per_page: 30, sort: 'name' }
    const query = q.value.trim()
    if (query) params.q = query
    if (bodyPartIds.value.length) params.body_part_ids = bodyPartIds.value.join(',')
    if (equipmentIds.value.length) params.equipment_ids = equipmentIds.value.join(',')
    if (muscleIds.value.length) params.muscle_ids = muscleIds.value.join(',')
    if (trackingTypes.value.length) params.tracking_types = trackingTypes.value.join(',')
    if (favoritesOnly.value) params.favorites_only = 1
    if (myEquipment.value) params.my_equipment = 1
    const data = await exercisesApi.list(params)
    if (seq !== requestSeq) return
    items.value = data.items ?? []
    total.value = data.total ?? 0
  } catch (error) {
    if (seq !== requestSeq) return
    toast.error(error.message)
  } finally {
    if (seq === requestSeq) loading.value = false
  }
}

// Любой ввод (поиск/фильтры) — один отложенный запрос вместо запроса на клик
function scheduleSearch(delay = 300) {
  clearTimeout(searchTimer)
  searchTimer = setTimeout(search, delay)
}

function resetFilters() {
  q.value = ''
  bodyPartIds.value = []
  equipmentIds.value = []
  muscleIds.value = []
  trackingTypes.value = []
  favoritesOnly.value = false
  myEquipment.value = false
  clearTimeout(qTimer)
  scheduleSearch(0)
}

watch(open, (value) => {
  if (!value) return
  // справочники фильтров (кэшируются в store, грузятся один раз за сессию)
  exercisesStore.loadMeta()
  if (items.value.length === 0) search()
})

watch(q, () => {
  clearTimeout(qTimer)
  qTimer = setTimeout(search, 400)
})

watch(
  [bodyPartIds, equipmentIds, muscleIds, trackingTypes, favoritesOnly, myEquipment],
  () => scheduleSearch(),
)

function pick(exercise) {
  emit('select', exercise)
}
</script>

<template>
  <Dialog v-model:open="open">
    <DialogContent class="sm:max-w-2xl">
      <DialogHeader>
        <DialogTitle>{{ $t('programs.pickerTitle') }}</DialogTitle>
        <DialogDescription>{{ $t('programs.pickerHint') }}</DialogDescription>
      </DialogHeader>

      <div class="flex flex-col gap-3">
        <div class="relative">
          <Search
            class="text-muted-foreground pointer-events-none absolute top-1/2 left-2.5 size-4 -translate-y-1/2"
          />
          <Input
            v-model="q"
            :placeholder="$t('exercises.searchPlaceholder')"
            class="pl-8"
          />
        </div>

        <div class="flex flex-wrap items-center gap-1.5">
          <MultiSelectFilter
            v-model:selected="bodyPartIds"
            :label="$t('exercises.bodyPart')"
            :items="dictionaries?.body_parts ?? []"
          />
          <MultiSelectFilter
            v-model:selected="equipmentIds"
            :label="$t('exercises.equipment')"
            :items="dictionaries?.equipment ?? []"
          />
          <MultiSelectFilter
            v-model:selected="muscleIds"
            :label="$t('exercises.muscle')"
            :items="dictionaries?.muscles ?? []"
          />
          <MultiSelectFilter
            v-model:selected="trackingTypes"
            :label="$t('exercises.trackingType')"
            :items="trackingTypeItems"
          />
        </div>

        <div class="flex flex-wrap items-center gap-x-4 gap-y-2">
          <div class="flex items-center gap-2">
            <Switch id="picker-favorites" v-model="favoritesOnly" />
            <Label for="picker-favorites" class="cursor-pointer text-sm">
              {{ $t('exercises.favorites') }}
            </Label>
            <TooltipProvider :delay-duration="200">
              <Tooltip>
                <TooltipTrigger as-child>
                  <Info class="text-muted-foreground size-4 shrink-0" aria-hidden="true" />
                </TooltipTrigger>
                <TooltipContent class="max-w-64 text-wrap">
                  {{ $t('exercises.favoritesHint') }}
                </TooltipContent>
              </Tooltip>
            </TooltipProvider>
          </div>

          <div v-if="hasUserEquipment" class="flex items-center gap-2">
            <Switch id="picker-my-equipment" v-model="myEquipment" />
            <Label for="picker-my-equipment" class="cursor-pointer text-sm">
              {{ $t('exercises.myEquipment') }}
            </Label>
            <TooltipProvider :delay-duration="200">
              <Tooltip>
                <TooltipTrigger as-child>
                  <Info class="text-muted-foreground size-4 shrink-0" aria-hidden="true" />
                </TooltipTrigger>
                <TooltipContent class="max-w-64 text-wrap">
                  {{ $t('exercises.myEquipmentHint') }}
                </TooltipContent>
              </Tooltip>
            </TooltipProvider>
          </div>

          <div class="ml-auto flex items-center gap-2">
            <p v-if="total > 0" class="text-muted-foreground text-sm">
              {{ $t('exercises.found', { total }) }}
            </p>
            <Button
              v-if="hasActiveFilters"
              variant="ghost"
              size="sm"
              class="text-muted-foreground h-7"
              @click="resetFilters"
            >
              <RotateCcw />
              {{ $t('programs.resetFilters') }}
            </Button>
          </div>
        </div>
      </div>

      <div class="max-h-96 overflow-y-auto" :class="{ 'pointer-events-none opacity-60': loading }">
        <div v-if="loading && items.length === 0" class="flex flex-col gap-2 py-1">
          <Skeleton v-for="i in 5" :key="i" class="h-12 w-full" />
        </div>

        <p
          v-else-if="items.length === 0"
          class="text-muted-foreground py-8 text-center text-sm"
        >
          {{ $t('programs.pickerEmpty') }}
        </p>

        <template v-else>
          <button
            v-for="exercise in items"
            :key="exercise.id"
            type="button"
            class="hover:bg-muted flex w-full items-center gap-3 rounded-lg p-2 text-left transition-colors"
            @click="pick(exercise)"
          >
            <div
              class="bg-muted flex size-12 shrink-0 items-center justify-center overflow-hidden rounded-md"
            >
              <img
                v-if="exercise.media?.gif_url ?? exercise.media?.thumbnail_url"
                :src="exercise.media.gif_url ?? exercise.media.thumbnail_url"
                :alt="exercise.name"
                loading="lazy"
                decoding="async"
                class="size-full object-contain"
                @error="$event.target.style.visibility = 'hidden'"
              />
            </div>
            <div class="flex min-w-0 flex-1 flex-col gap-1">
              <p class="line-clamp-1 text-sm font-medium">{{ exercise.name }}</p>
              <div class="flex flex-wrap gap-1">
                <Badge variant="secondary" class="text-xs">{{ exercise.body_part.name }}</Badge>
                <Badge variant="outline" class="text-xs">{{ exercise.equipment.name }}</Badge>
              </div>
            </div>
            <Button variant="ghost" size="icon-sm" :aria-label="$t('programs.pickerAdd')">
              <Plus />
            </Button>
          </button>
        </template>
      </div>
    </DialogContent>
  </Dialog>
</template>
