<script setup>
import { computed, ref } from 'vue'
import { ChevronDown, Info } from '@lucide/vue'
import { useI18n } from 'vue-i18n'

import { Checkbox } from '@/components/ui/checkbox'
import { Label } from '@/components/ui/label'
import { ScrollArea } from '@/components/ui/scroll-area'
import { Switch } from '@/components/ui/switch'
import { Tooltip, TooltipContent, TooltipProvider, TooltipTrigger } from '@/components/ui/tooltip'

const { t } = useI18n()

const props = defineProps({
  dictionaries: { type: Object, required: true },
  showMyEquipment: { type: Boolean, default: false },
})

const bodyPartIds = defineModel('bodyPartIds', { type: Array, required: true })
const equipmentIds = defineModel('equipmentIds', { type: Array, required: true })
const muscleIds = defineModel('muscleIds', { type: Array, required: true })
const trackingTypes = defineModel('trackingTypes', { type: Array, required: true })
const myEquipment = defineModel('myEquipment', { type: Boolean, required: true })
const favoritesOnly = defineModel('favoritesOnly', { type: Boolean, required: true })

// Секции свёрнуты по умолчанию, кроме «Часть тела»; состояние локальное.
const collapsed = ref({ bodyPart: false, equipment: true, muscle: true, trackingType: true })

const sections = computed(() => [
  {
    key: 'bodyPart',
    label: t('exercises.bodyPart'),
    items: props.dictionaries.body_parts ?? [],
    model: bodyPartIds,
    scroll: false,
  },
  {
    key: 'equipment',
    label: t('exercises.equipment'),
    items: props.dictionaries.equipment ?? [],
    model: equipmentIds,
    scroll: true,
  },
  {
    key: 'muscle',
    label: t('exercises.muscle'),
    items: props.dictionaries.muscles ?? [],
    model: muscleIds,
    scroll: true,
  },
  {
    key: 'trackingType',
    label: t('exercises.trackingType'),
    items: (props.dictionaries.tracking_types ?? []).map((tt) => ({
      id: tt.value,
      name: tt.name,
    })),
    model: trackingTypes,
    scroll: false,
  },
])

function toggle(model, value) {
  model.value = model.value.includes(value)
    ? model.value.filter((v) => v !== value)
    : [...model.value, value]
}

function toggleSection(key) {
  collapsed.value[key] = !collapsed.value[key]
}
</script>

<template>
  <div class="flex flex-col gap-5">
    <div class="flex items-center gap-2">
      <Switch id="favorites-only" v-model="favoritesOnly" />
      <Label for="favorites-only" class="cursor-pointer">{{ t('exercises.favorites') }}</Label>
      <TooltipProvider :delay-duration="200">
        <Tooltip>
          <TooltipTrigger as-child>
            <Info class="text-muted-foreground size-4 shrink-0" aria-hidden="true" />
          </TooltipTrigger>
          <TooltipContent class="max-w-64 text-wrap">
            {{ t('exercises.favoritesHint') }}
          </TooltipContent>
        </Tooltip>
      </TooltipProvider>
    </div>

    <div v-if="showMyEquipment" class="flex items-center gap-2">
      <Switch id="my-equipment" v-model="myEquipment" />
      <Label for="my-equipment" class="cursor-pointer">{{ t('exercises.myEquipment') }}</Label>
      <TooltipProvider :delay-duration="200">
        <Tooltip>
          <TooltipTrigger as-child>
            <Info class="text-muted-foreground size-4 shrink-0" aria-hidden="true" />
          </TooltipTrigger>
          <TooltipContent class="max-w-64 text-wrap">
            {{ t('exercises.myEquipmentHint') }}
          </TooltipContent>
        </Tooltip>
      </TooltipProvider>
    </div>

    <section v-for="section in sections" :key="section.key" class="flex flex-col gap-2">
      <button
        type="button"
        class="hover:bg-muted/50 flex w-full cursor-pointer items-center justify-between rounded px-1 py-0.5"
        :aria-expanded="!collapsed[section.key]"
        @click="toggleSection(section.key)"
      >
        <Label class="cursor-pointer">{{ section.label }}</Label>
        <ChevronDown
          class="text-muted-foreground size-4 shrink-0 transition-transform"
          :class="{ 'rotate-180': !collapsed[section.key] }"
          aria-hidden="true"
        />
      </button>
      <div v-show="!collapsed[section.key]" class="flex flex-col gap-2">
        <ScrollArea v-if="section.scroll" class="-mr-2 h-56 pr-2">
          <div class="flex flex-col gap-0.5">
            <label
              v-for="item in section.items"
              :key="item.id"
              class="hover:bg-muted/50 flex cursor-pointer items-center gap-2 rounded px-1 py-1 text-sm"
            >
              <Checkbox
                :model-value="section.model.value.includes(item.id)"
                @update:model-value="toggle(section.model, item.id)"
              />
              <span class="truncate">{{ item.name }}</span>
            </label>
          </div>
        </ScrollArea>
        <div v-else class="flex flex-col gap-0.5">
          <label
            v-for="item in section.items"
            :key="item.id"
            class="hover:bg-muted/50 flex cursor-pointer items-center gap-2 rounded px-1 py-1 text-sm"
          >
            <Checkbox
              :model-value="section.model.value.includes(item.id)"
              @update:model-value="toggle(section.model, item.id)"
            />
            <span class="truncate">{{ item.name }}</span>
          </label>
        </div>
      </div>
    </section>
  </div>
</template>
