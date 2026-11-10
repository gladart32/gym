<script setup>
import { reactiveOmit } from '@vueuse/core'
import { cn } from 'cn'
import { PaginationList } from 'reka-ui'

const props = defineProps({
  asChild: { type: Boolean, required: false },
  as: { type: null, required: false },
  class: {
    type: [Boolean, null, String, Object, Array],
    required: false,
    skipCheck: true,
  },
})

const delegatedProps = reactiveOmit(props, 'class')
</script>

<template>
  <PaginationList
    v-slot="slotProps"
    data-slot="pagination-content"
    v-bind="delegatedProps"
    :class="cn('flex items-center gap-0.5', props.class)"
  >
    <slot v-bind="slotProps" />
  </PaginationList>
</template>
