<script setup>
import { computed } from 'vue'
import { ChevronDown, X } from '@lucide/vue'
import { useI18n } from 'vue-i18n'

import { Button } from '@/components/ui/button'
import {
  DropdownMenu,
  DropdownMenuCheckboxItem,
  DropdownMenuContent,
  DropdownMenuTrigger,
} from '@/components/ui/dropdown-menu'

const { t } = useI18n()

const props = defineProps({
  label: { type: String, required: true },
  // [{ id, name }], id — число или строка (tracking type value)
  items: { type: Array, required: true },
})

const selected = defineModel('selected', { type: Array, required: true })

const clearLabel = computed(
  () => `${props.label} — ${t('programs.pickerClearFilter')}`,
)

function isChecked(item) {
  return selected.value.includes(item.id)
}

function toggle(item, checked) {
  selected.value = checked
    ? [...selected.value, item.id]
    : selected.value.filter((v) => v !== item.id)
}

function clear() {
  selected.value = []
}
</script>

<template>
  <div class="flex items-center gap-0.5">
    <DropdownMenu>
      <DropdownMenuTrigger as-child>
        <Button variant="outline" size="sm" class="gap-1.5 font-normal">
          <span class="max-w-36 truncate">{{ label }}</span>
          <span
            v-if="selected.length"
            class="bg-secondary text-secondary-foreground rounded-full px-1.5 text-xs leading-4 tabular-nums"
          >
            {{ selected.length }}
          </span>
          <ChevronDown class="text-muted-foreground size-3.5 shrink-0" aria-hidden="true" />
        </Button>
      </DropdownMenuTrigger>
      <DropdownMenuContent align="start" class="w-56">
        <DropdownMenuCheckboxItem
          v-for="item in items"
          :key="item.id"
          :model-value="isChecked(item)"
          @select.prevent
          @update:model-value="toggle(item, $event)"
        >
          <span class="truncate">{{ item.name }}</span>
        </DropdownMenuCheckboxItem>
        <div
          v-if="items.length === 0"
          class="text-muted-foreground py-4 text-center text-sm"
        >
          {{ t('programs.pickerNoOptions') }}
        </div>
      </DropdownMenuContent>
    </DropdownMenu>
    <Button
      v-if="selected.length"
      variant="ghost"
      size="icon-sm"
      class="text-muted-foreground size-6"
      :aria-label="clearLabel"
      :title="clearLabel"
      @click="clear"
    >
      <X class="size-3.5" aria-hidden="true" />
    </Button>
  </div>
</template>
